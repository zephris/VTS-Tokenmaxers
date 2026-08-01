import fs from 'node:fs';
import path from 'node:path';
import Database from 'better-sqlite3';
import type {
  Broadcast,
  CrossCheckStatus,
  DashboardResponse,
  GroundTruthLabel,
  OutpostSummary,
  RiskLevel,
  SenderStatus,
  Station,
  SenderType,
} from '@vts/common';
import { importDataset } from './import.js';

const SCHEMA_VERSION = 2;

interface SenderRow {
  sender_id: string;
  sender_type: SenderType;
  location: string;
  first_seen: string | null;
  last_seen: string | null;
  total_broadcasts: number;
  broadcasts_verified_accurate: number;
  broadcasts_verified_false: number;
  reliability_score: number;
  current_status: SenderStatus;
  is_derived: number;
}

interface SenderSummaryRow {
  sender_id: string;
  location: string;
  last_seen: string;
  hours_silent: number;
  reliability_score: number;
  current_status: SenderStatus;
}

interface BroadcastRow {
  broadcast_id: string;
  timestamp: string;
  sender_id: string;
  location: string;
  broadcast_type: Broadcast['type'];
  message_text: string | null;
  signal_strength: number | null;
  cross_check_status: CrossCheckStatus;
  label: GroundTruthLabel;
}

export function createDatabase(databasePath?: string): Database.Database {
  const resolved = path.resolve(databasePath ?? process.env.DATABASE_PATH ?? './data/silent-outposts.db');
  fs.mkdirSync(path.dirname(resolved), { recursive: true });
  const db = new Database(resolved);

  db.pragma('journal_mode = WAL');
  db.pragma('foreign_keys = ON');

  const version = db.pragma('user_version', { simple: true }) as number;
  if (version < SCHEMA_VERSION) {
    db.exec('DROP TABLE IF EXISTS broadcasts; DROP TABLE IF EXISTS senders;');
  }

  db.exec(`
    CREATE TABLE IF NOT EXISTS senders (
      sender_id TEXT PRIMARY KEY,
      sender_type TEXT NOT NULL,
      location TEXT NOT NULL,
      first_seen TEXT,
      last_seen TEXT,
      total_broadcasts INTEGER NOT NULL DEFAULT 0,
      broadcasts_verified_accurate INTEGER NOT NULL DEFAULT 0,
      broadcasts_verified_false INTEGER NOT NULL DEFAULT 0,
      reliability_score INTEGER NOT NULL,
      current_status TEXT NOT NULL,
      is_derived INTEGER NOT NULL DEFAULT 0
    );

    CREATE TABLE IF NOT EXISTS broadcasts (
      broadcast_id TEXT PRIMARY KEY,
      timestamp TEXT NOT NULL,
      sender_id TEXT NOT NULL,
      location TEXT NOT NULL,
      broadcast_type TEXT NOT NULL,
      message_text TEXT,
      signal_strength INTEGER,
      cross_check_status TEXT NOT NULL,
      label TEXT NOT NULL,
      FOREIGN KEY (sender_id) REFERENCES senders(sender_id)
    );
  `);
  db.pragma(`user_version = ${SCHEMA_VERSION}`);

  try {
    importDataset(db);
  } catch (error) {
    db.close();
    throw error;
  }

  return db;
}

function riskFor(row: SenderSummaryRow): RiskLevel {
  if (row.current_status === 'gone_quiet' || row.hours_silent >= 72) return 'critical';
  if (row.hours_silent >= 36) return 'high';
  if (row.hours_silent >= 18) return 'medium';
  return 'low';
}

function mapBroadcast(row: BroadcastRow): Broadcast {
  return {
    id: row.broadcast_id,
    timestamp: row.timestamp,
    senderId: row.sender_id,
    location: row.location,
    type: row.broadcast_type,
    messageText: row.message_text,
    signalStrength: row.signal_strength,
    crossCheckStatus: row.cross_check_status,
  };
}

export function getStations(db: Database.Database): Station[] {
  const rows = db.prepare(`
    SELECT
      sender_id, sender_type, location, first_seen, last_seen, total_broadcasts,
      broadcasts_verified_accurate, broadcasts_verified_false, reliability_score,
      current_status, is_derived
    FROM senders
    ORDER BY sender_id
  `).all() as SenderRow[];

  return rows.map((row) => ({
    senderId: row.sender_id,
    senderType: row.sender_type,
    location: row.location,
    firstSeen: row.first_seen,
    lastSeen: row.last_seen,
    totalBroadcasts: row.total_broadcasts,
    broadcastsVerifiedAccurate: row.broadcasts_verified_accurate,
    broadcastsVerifiedFalse: row.broadcasts_verified_false,
    reliabilityScore: row.reliability_score,
    currentStatus: row.current_status,
    derived: row.is_derived === 1,
  }));
}

export function getStationBroadcasts(
  db: Database.Database,
  senderId: string,
): Broadcast[] | null {
  const sender = db.prepare('SELECT 1 FROM senders WHERE sender_id = ?').get(senderId);
  if (!sender) return null;

  const rows = db.prepare(`
    SELECT * FROM broadcasts
    WHERE sender_id = ?
    ORDER BY timestamp ASC
  `).all(senderId) as BroadcastRow[];

  return rows.map(mapBroadcast);
}

export function getCounts(db: Database.Database): { stations: number; broadcasts: number } {
  const stations = db.prepare('SELECT COUNT(*) AS count FROM senders').get() as { count: number };
  const broadcasts = db.prepare('SELECT COUNT(*) AS count FROM broadcasts').get() as { count: number };
  return { stations: stations.count, broadcasts: broadcasts.count };
}

export function getDashboard(db: Database.Database): DashboardResponse {
  const analysisTimestamp = db
    .prepare('SELECT MAX(timestamp) AS timestamp FROM broadcasts')
    .get() as { timestamp: string };

  const senderRows = db.prepare(`
    SELECT
      s.sender_id,
      s.location,
      MAX(b.timestamp) AS last_seen,
      ROUND((julianday(?) - julianday(MAX(b.timestamp))) * 24, 1) AS hours_silent,
      s.reliability_score,
      s.current_status
    FROM senders s
    JOIN broadcasts b ON b.sender_id = s.sender_id
    WHERE s.sender_type = 'robot_outpost'
    GROUP BY s.sender_id
    ORDER BY hours_silent DESC
  `).all(analysisTimestamp.timestamp) as SenderSummaryRow[];

  const outposts: OutpostSummary[] = senderRows.map((row) => ({
    senderId: row.sender_id,
    location: row.location,
    lastSeen: row.last_seen,
    hoursSilent: row.hours_silent,
    reliabilityScore: row.reliability_score,
    status: row.current_status,
    riskLevel: riskFor(row),
  }));

  const recentRows = db.prepare(`
    SELECT * FROM broadcasts
    ORDER BY timestamp DESC
    LIMIT 20
  `).all() as BroadcastRow[];

  return {
    analysisTimestamp: analysisTimestamp.timestamp,
    outposts,
    recentBroadcasts: recentRows.map(mapBroadcast),
  };
}

export function getIncidentEvidence(db: Database.Database, senderId: string): Broadcast[] {
  const sender = db.prepare('SELECT location FROM senders WHERE sender_id = ?').get(senderId) as
    | { location: string }
    | undefined;

  if (!sender) return [];

  const rows = db.prepare(`
    SELECT * FROM broadcasts
    WHERE sender_id = ? OR location = ?
    ORDER BY timestamp ASC
    LIMIT 30
  `).all(senderId, sender.location) as BroadcastRow[];

  return rows.map(mapBroadcast);
}
