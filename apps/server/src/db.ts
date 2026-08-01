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
} from '@vts/common';

const databasePath = path.resolve(process.env.DATABASE_PATH ?? './data/silent-outposts.db');
fs.mkdirSync(path.dirname(databasePath), { recursive: true });
const db = new Database(databasePath);

db.pragma('journal_mode = WAL');
db.pragma('foreign_keys = ON');

db.exec(`
  CREATE TABLE IF NOT EXISTS senders (
    sender_id TEXT PRIMARY KEY,
    sender_type TEXT NOT NULL,
    location TEXT NOT NULL,
    reliability_score INTEGER NOT NULL,
    current_status TEXT NOT NULL
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

const seed = db.transaction(() => {
  const insertSender = db.prepare(`
    INSERT OR IGNORE INTO senders
      (sender_id, sender_type, location, reliability_score, current_status)
    VALUES (?, ?, ?, ?, ?)
  `);

  const senders = [
    ['Outpost-Alpha', 'robot_outpost', 'Sunken Garden', 92, 'active'],
    ['Outpost-Beta', 'robot_outpost', 'Robotics Workshop', 85, 'active'],
    ['Outpost-Gamma', 'robot_outpost', 'Prescott Court', 88, 'active'],
    ['Outpost-Delta', 'robot_outpost', 'Guild Village', 90, 'gone_quiet'],
    ['Mini-Marv-01', 'junior_scout_group', 'Mobile', 88, 'active'],
    ['New Meridian', 'relay_identity', 'Unknown', 5, 'suspected_compromised'],
  ] as const;

  for (const sender of senders) insertSender.run(...sender);

  const insertBroadcast = db.prepare(`
    INSERT OR IGNORE INTO broadcasts
      (broadcast_id, timestamp, sender_id, location, broadcast_type, message_text,
       signal_strength, cross_check_status, label)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
  `);

  const broadcasts = [
    ['BC-003', '2026-07-10 14:05', 'Outpost-Delta', 'Guild Village', 'routine_check', 'Guild Village quiet. Supplies stable.', 55, 'verified', 'genuine'],
    ['BC-010', '2026-07-13 06:45', 'Outpost-Delta', 'Guild Village', 'routine_check', 'Guild Village status unchanged. Low peacock sightings.', 51, 'verified', 'genuine'],
    ['BC-019', '2026-07-16 10:05', 'Mini-Marv-01', 'Guild Village', 'emergency', 'Attempting contact with Outpost-Delta. No response for 48 hours.', null, 'unconfirmed', 'genuine'],
    ['BC-021', '2026-07-17 05:50', 'Outpost-Gamma', 'Prescott Court', 'emergency', 'Urgent: peacock swarm breaching Prescott Court perimeter.', 44, 'not_checked', 'genuine'],
    ['BC-035', '2026-07-22 11:05', 'New Meridian', 'Guild Village', 'all_clear', 'Guild Village confirmed clear. Outpost-Delta concerns overstated.', 87, 'disputed', 'peacock_spoofed'],
    ['BC-036', '2026-07-22 12:00', 'Mini-Marv-01', 'Guild Village', 'emergency', 'Still no contact with Outpost-Delta. The all-clear cannot be confirmed.', null, 'not_checked', 'genuine'],
    ['BC-038', '2026-07-22 12:30', 'Outpost-Alpha', 'Sunken Garden', 'routine_check', 'Scheduled check-in complete.', 61, 'verified', 'genuine'],
    ['BC-039', '2026-07-22 12:45', 'Outpost-Beta', 'Robotics Workshop', 'routine_check', 'Workshop operational. Repairs continuing.', 50, 'verified', 'genuine'],
  ] as const;

  for (const broadcast of broadcasts) insertBroadcast.run(...broadcast);
});

seed();

interface SenderRow {
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

function riskFor(row: SenderRow): RiskLevel {
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

export function getDashboard(): DashboardResponse {
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
  `).all(analysisTimestamp.timestamp) as SenderRow[];

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

export function getIncidentEvidence(senderId: string): Broadcast[] {
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
