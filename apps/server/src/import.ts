import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import type Database from 'better-sqlite3';
import type { BroadcastType, CrossCheckStatus, GroundTruthLabel, SenderStatus, SenderType } from '@vts/common';
import { parseCsv } from './csv.js';

const DEFAULT_DATASET_DIR = fileURLToPath(new URL('.', import.meta.url));

export const SENDER_HISTORY_CSV = 'sender_history.csv';
export const BROADCAST_LOG_CSV = 'broadcast_message_log.csv';

const BROADCAST_TYPES: BroadcastType[] = [
  'all_clear',
  'warning',
  'supply_request',
  'situation_report',
  'routine_check',
  'emergency',
];

const CROSS_CHECK_STATUSES: CrossCheckStatus[] = ['verified', 'disputed', 'unconfirmed', 'not_checked'];

const GROUND_TRUTH_LABELS: GroundTruthLabel[] = ['genuine', 'peacock_spoofed', 'outdated', 'unknown'];

const SENDER_STATUSES: SenderStatus[] = ['active', 'gone_quiet', 'suspected_compromised'];

export interface ImportResult {
  stations: number;
  broadcasts: number;
}

interface SenderSeed {
  senderId: string;
  senderType: SenderType;
  location: string;
  firstSeen: string | null;
  lastSeen: string | null;
  totalBroadcasts: number;
  broadcastsVerifiedAccurate: number;
  broadcastsVerifiedFalse: number;
  reliabilityScore: number;
  currentStatus: SenderStatus;
  isDerived: boolean;
}

interface BroadcastSeed {
  broadcastId: string;
  timestamp: string;
  senderId: string;
  location: string;
  broadcastType: BroadcastType;
  messageText: string | null;
  signalStrength: number | null;
  crossCheckStatus: CrossCheckStatus;
  label: GroundTruthLabel;
}

export function resolveDatasetDir(): string {
  if (process.env.DATASET_DIR) return path.resolve(process.env.DATASET_DIR);

  let current = DEFAULT_DATASET_DIR;
  for (let depth = 0; depth < 8; depth += 1) {
    const candidate = path.join(current, 'hackathon', 'dataset');
    if (
      fs.existsSync(path.join(candidate, BROADCAST_LOG_CSV)) &&
      fs.existsSync(path.join(candidate, SENDER_HISTORY_CSV))
    ) {
      return candidate;
    }
    const parent = path.dirname(current);
    if (parent === current) break;
    current = parent;
  }

  return path.join(DEFAULT_DATASET_DIR, 'hackathon', 'dataset');
}

export function importDataset(
  db: Database.Database,
  options: { broadcastCsv?: string; senderCsv?: string } = {},
): ImportResult {
  const datasetDir = resolveDatasetDir();
  const broadcastCsv = options.broadcastCsv ?? path.join(datasetDir, BROADCAST_LOG_CSV);
  const senderCsv = options.senderCsv ?? path.join(datasetDir, SENDER_HISTORY_CSV);

  const broadcastRows = parseCsv(readCsv(broadcastCsv));
  const senderRows = parseCsv(readCsv(senderCsv));
  const broadcastHeader = broadcastRows[0] ?? [];
  const senderHeader = senderRows[0] ?? [];

  const broadcasts = broadcastRows.slice(1).map((row) => mapBroadcastRow(broadcastHeader, row));
  const historySenders = senderRows.slice(1).map((row) => mapSenderRow(senderHeader, row));

  const latestBySender = new Map<string, BroadcastSeed>();
  for (const broadcast of broadcasts) {
    const previous = latestBySender.get(broadcast.senderId);
    if (!previous || broadcast.timestamp >= previous.timestamp) {
      latestBySender.set(broadcast.senderId, broadcast);
    }
  }

  for (const sender of historySenders) {
    sender.location = latestBySender.get(sender.senderId)?.location ?? 'Unknown';
  }

  const knownSenderIds = new Set(historySenders.map((sender) => sender.senderId));
  const derivedSenders = deriveSenderProfiles(broadcasts, knownSenderIds, latestBySender);

  const importTx = db.transaction(() => {
    const insertSender = db.prepare(`
      INSERT OR IGNORE INTO senders
        (sender_id, sender_type, location, first_seen, last_seen, total_broadcasts,
         broadcasts_verified_accurate, broadcasts_verified_false, reliability_score,
         current_status, is_derived)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `);

    const insertBroadcast = db.prepare(`
      INSERT OR IGNORE INTO broadcasts
        (broadcast_id, timestamp, sender_id, location, broadcast_type, message_text,
         signal_strength, cross_check_status, label)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `);

    for (const sender of historySenders) insertSender.run(...senderSeedValues(sender));
    for (const sender of derivedSenders) insertSender.run(...senderSeedValues(sender));

    for (const broadcast of broadcasts) {
      insertBroadcast.run(
        broadcast.broadcastId,
        broadcast.timestamp,
        broadcast.senderId,
        broadcast.location,
        broadcast.broadcastType,
        broadcast.messageText,
        broadcast.signalStrength,
        broadcast.crossCheckStatus,
        broadcast.label,
      );
    }

    db.prepare(`
      UPDATE senders
      SET current_status = CASE
        WHEN (julianday((SELECT MAX(timestamp) FROM broadcasts)) - julianday(last_seen)) * 24 >= 72
          THEN 'gone_quiet'
          ELSE 'active'
      END
      WHERE is_derived = 1
    `).run();
  });

  importTx();

  return { stations: historySenders.length + derivedSenders.length, broadcasts: broadcasts.length };
}

function readCsv(csvPath: string): string {
  if (!fs.existsSync(csvPath)) {
    throw new Error(`Dataset CSV not found at "${csvPath}". Set DATASET_DIR to the dataset folder.`);
  }
  return fs.readFileSync(csvPath, 'utf8');
}

function senderSeedValues(sender: SenderSeed): unknown[] {
  return [
    sender.senderId,
    sender.senderType,
    sender.location,
    sender.firstSeen,
    sender.lastSeen,
    sender.totalBroadcasts,
    sender.broadcastsVerifiedAccurate,
    sender.broadcastsVerifiedFalse,
    sender.reliabilityScore,
    sender.currentStatus,
    sender.isDerived ? 1 : 0,
  ];
}

function deriveSenderProfiles(
  broadcasts: BroadcastSeed[],
  knownSenderIds: Set<string>,
  latestBySender: Map<string, BroadcastSeed>,
): SenderSeed[] {
  const grouped = new Map<string, BroadcastSeed[]>();
  for (const broadcast of broadcasts) {
    const list = grouped.get(broadcast.senderId) ?? [];
    list.push(broadcast);
    grouped.set(broadcast.senderId, list);
  }

  const derived: SenderSeed[] = [];
  for (const [senderId, senderBroadcasts] of grouped) {
    if (knownSenderIds.has(senderId)) continue;

    const latest = latestBySender.get(senderId);
    if (!latest) continue;
    const firstSeen = senderBroadcasts.reduce((a, b) => (a.timestamp <= b.timestamp ? a : b)).timestamp;
    const totalBroadcasts = senderBroadcasts.length;
    const verifiedAccurate = senderBroadcasts.filter((broadcast) => broadcast.label === 'genuine').length;
    const verifiedFalse = senderBroadcasts.filter((broadcast) => broadcast.label === 'peacock_spoofed').length;
    const reliabilityScore = Math.max(
      0,
      Math.min(100, Math.round((verifiedAccurate / totalBroadcasts) * 100)),
    );

    derived.push({
      senderId,
      senderType: deriveSenderType(senderId),
      location: latest.location,
      firstSeen,
      lastSeen: latest.timestamp,
      totalBroadcasts,
      broadcastsVerifiedAccurate: verifiedAccurate,
      broadcastsVerifiedFalse: verifiedFalse,
      reliabilityScore,
      currentStatus: 'active',
      isDerived: true,
    });
  }

  return derived;
}

function deriveSenderType(senderId: string): SenderType {
  if (senderId.startsWith('Outpost-')) return 'robot_outpost';
  if (senderId.startsWith('Mini-Marv-')) return 'junior_scout_group';
  return 'relay_identity';
}

function mapBroadcastRow(header: string[], row: string[]): BroadcastSeed {
  const broadcastId = column(header, row, 'broadcast_id');
  const broadcastType = column(header, row, 'broadcast_type');
  const crossCheckStatus = column(header, row, 'cross_check_status');
  const label = column(header, row, 'label');

  assertMember('broadcast_type', broadcastType, BROADCAST_TYPES, broadcastId);
  assertMember('cross_check_status', crossCheckStatus, CROSS_CHECK_STATUSES, broadcastId);
  assertMember('label', label, GROUND_TRUTH_LABELS, broadcastId);

  const signalRaw = column(header, row, 'signal_strength');
  const signalStrength = signalRaw === '' ? null : Number(signalRaw);
  if (signalStrength !== null && (!Number.isInteger(signalStrength) || signalStrength < 0)) {
    throw new Error(`Invalid signal_strength "${signalRaw}" in broadcast row ${broadcastId}.`);
  }

  return {
    broadcastId,
    timestamp: column(header, row, 'timestamp'),
    senderId: column(header, row, 'sender_id'),
    location: column(header, row, 'location'),
    broadcastType: broadcastType as BroadcastType,
    messageText: emptyToNull(column(header, row, 'message_text')),
    signalStrength,
    crossCheckStatus: crossCheckStatus as CrossCheckStatus,
    label: label as GroundTruthLabel,
  };
}

function mapSenderRow(header: string[], row: string[]): SenderSeed {
  const senderId = column(header, row, 'sender_id');
  const senderType = column(header, row, 'sender_type');
  const currentStatus = column(header, row, 'current_status');

  assertMember('sender_type', senderType, ['robot_outpost', 'relay_identity', 'junior_scout_group'], senderId);
  assertMember('current_status', currentStatus, SENDER_STATUSES, senderId);

  return {
    senderId,
    senderType: senderType as SenderType,
    location: 'Unknown',
    firstSeen: emptyToNull(column(header, row, 'first_seen')),
    lastSeen: emptyToNull(column(header, row, 'last_seen')),
    totalBroadcasts: integerColumn(header, row, 'total_broadcasts', senderId),
    broadcastsVerifiedAccurate: integerColumn(header, row, 'broadcasts_verified_accurate', senderId),
    broadcastsVerifiedFalse: integerColumn(header, row, 'broadcasts_verified_false', senderId),
    reliabilityScore: integerColumn(header, row, 'reliability_score', senderId),
    currentStatus: currentStatus as SenderStatus,
    isDerived: false,
  };
}

function column(header: string[], row: string[], name: string): string {
  const index = header.indexOf(name);
  if (index < 0 || index >= row.length) {
    throw new Error(`Dataset CSV is missing column "${name}".`);
  }
  return row[index] ?? '';
}

function emptyToNull(value: string): string | null {
  return value === '' ? null : value;
}

function integerColumn(header: string[], row: string[], name: string, senderId: string): number {
  const raw = column(header, row, name);
  const value = Number(raw);
  if (!Number.isInteger(value)) {
    throw new Error(`Invalid ${name} "${raw}" in sender history row ${senderId}.`);
  }
  return value;
}

function assertMember<T extends string>(name: string, value: string, allowed: readonly T[], rowId: string): void {
  if (!allowed.includes(value as T)) {
    throw new Error(`Invalid ${name} "${value}" in dataset row ${rowId}.`);
  }
}
