import { after, before, test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import type Database from 'better-sqlite3';
import { createDatabase, getCounts, getStationBroadcasts, getStations } from './db.js';
import { parseCsv } from './csv.js';

const DATASET_DIR = fileURLToPath(new URL('../../../hackathon/dataset/', import.meta.url));
const BROADCAST_CSV = path.join(DATASET_DIR, 'broadcast_message_log.csv');
const SENDER_CSV = path.join(DATASET_DIR, 'sender_history.csv');

let tempDir: string;
let dbPath: string;
const openDbs: Database.Database[] = [];

before(() => {
  tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'silent-outposts-test-'));
  dbPath = path.join(tempDir, 'test.db');
});

after(() => {
  for (const db of openDbs) {
    try {
      db.close();
    } catch {
      // already closed
    }
  }
  fs.rmSync(tempDir, { recursive: true, force: true, maxRetries: 3 });
});

function freshDb() {
  const db = createDatabase(dbPath);
  openDbs.push(db);
  return db;
}

function closeAllDbs() {
  for (const db of openDbs.splice(0)) {
    try {
      db.close();
    } catch {
      // already closed
    }
  }
}

test('imports all 300 broadcasts and 14 stations', () => {
  const db = freshDb();
  const counts = getCounts(db);
  assert.equal(counts.broadcasts, 300);
  assert.equal(counts.stations, 14);
});

test('import is idempotent across restarts', () => {
  freshDb();
  freshDb();
  const counts = getCounts(freshDb());
  assert.equal(counts.broadcasts, 300);
  assert.equal(counts.stations, 14);
});

test('recreating a deleted database restores the full dataset', () => {
  freshDb();
  closeAllDbs();
  for (const suffix of ['', '-wal', '-shm']) {
    fs.rmSync(`${dbPath}${suffix}`, { force: true });
  }
  const counts = getCounts(freshDb());
  assert.equal(counts.broadcasts, 300);
  assert.equal(counts.stations, 14);
});

test('preserves null message_text and signal_strength values', () => {
  const db = freshDb();
  const bc019 = getStationBroadcasts(db, 'Mini-Marv-01')?.find((b) => b.id === 'BC-019');
  const bc026 = getStationBroadcasts(db, 'Marv Mail')?.find((b) => b.id === 'BC-026');
  assert.ok(bc019);
  assert.equal(bc019.signalStrength, null);
  assert.equal(bc019.messageText, 'Attempting contact with Outpost-Delta. No response for 48 hours.');
  assert.ok(bc026);
  assert.equal(bc026.messageText, null);
  assert.equal(bc026.signalStrength, 72);
});

test('preserves quoted CSV fields exactly', () => {
  const db = freshDb();
  const rows = parseCsv(fs.readFileSync(BROADCAST_CSV, 'utf8'));
  const header = rows[0];
  const idIndex = header.indexOf('broadcast_id');
  const senderIndex = header.indexOf('sender_id');
  const messageIndex = header.indexOf('message_text');

  for (const row of rows.slice(1)) {
    const broadcast = getStationBroadcasts(db, row[senderIndex])?.find((b) => b.id === row[idIndex]);
    assert.ok(broadcast, `missing broadcast ${row[idIndex]}`);
    assert.equal(broadcast.messageText, row[messageIndex] === '' ? null : row[messageIndex]);
  }
});

test('derives profiles for sender IDs absent from sender_history.csv', () => {
  const db = freshDb();
  const stations = getStations(db);

  const derived = stations.filter((station) => station.derived);
  assert.deepEqual(
    derived.map((station) => station.senderId).sort(),
    ['Mini-Marv-04', 'Mini-Marv-05', 'Outpost-Epsilon', 'Outpost-Theta', 'Outpost-Zeta'],
  );

  const epsilon = stations.find((station) => station.senderId === 'Outpost-Epsilon');
  assert.ok(epsilon);
  assert.equal(epsilon.senderType, 'robot_outpost');
  assert.equal(epsilon.location, getStationBroadcasts(db, 'Outpost-Epsilon')?.at(-1)?.location);
  assert.equal(epsilon.firstSeen, '2026-07-23 06:30');
  assert.equal(epsilon.lastSeen, '2026-11-19 00:52');
  assert.equal(epsilon.totalBroadcasts, 31);
  assert.equal(epsilon.broadcastsVerifiedAccurate, 31);
  assert.equal(epsilon.broadcastsVerifiedFalse, 0);
  assert.equal(epsilon.reliabilityScore, 100);

  const marv04 = stations.find((station) => station.senderId === 'Mini-Marv-04');
  assert.ok(marv04);
  assert.equal(marv04.senderType, 'junior_scout_group');
  assert.equal(marv04.totalBroadcasts, 9);
  assert.equal(marv04.reliabilityScore, 100);
});

test('derived current_status reflects silence against the dataset max timestamp', () => {
  const db = freshDb();
  const stations = getStations(db);

  const marv05 = stations.find((station) => station.senderId === 'Mini-Marv-05');
  assert.ok(marv05);
  assert.equal(marv05.currentStatus, 'gone_quiet');

  const epsilon = stations.find((station) => station.senderId === 'Outpost-Epsilon');
  assert.ok(epsilon);
  assert.equal(epsilon.currentStatus, 'active');
});

test('sender_history values are imported verbatim', () => {
  const db = freshDb();
  const stations = getStations(db);

  const alpha = stations.find((station) => station.senderId === 'Outpost-Alpha');
  assert.ok(alpha);
  assert.equal(alpha.derived, false);
  assert.equal(alpha.senderType, 'robot_outpost');
  assert.equal(alpha.reliabilityScore, 92);
  assert.equal(alpha.currentStatus, 'active');
  assert.equal(alpha.firstSeen, '2026-07-10');
  assert.equal(alpha.totalBroadcasts, 6);
  assert.equal(alpha.broadcastsVerifiedAccurate, 6);

  const marvMail = stations.find((station) => station.senderId === 'Marv Mail');
  assert.ok(marvMail);
  assert.equal(marvMail.senderType, 'relay_identity');
  assert.equal(marvMail.currentStatus, 'suspected_compromised');
});

test('station broadcast lookups are read-only and 404-able', () => {
  const db = freshDb();
  const broadcasts = getStationBroadcasts(db, 'Outpost-Gamma');
  assert.ok(broadcasts);
  assert.equal(broadcasts.length, 8);
  assert.equal(getStationBroadcasts(db, 'Does-Not-Exist'), null);
});

test('parseCsv handles quotes, escapes, commas and empty fields', () => {
  const rows = parseCsv('a,b,"c,d"\r\n"x""y",,\nz,"",w');
  assert.deepEqual(rows, [
    ['a', 'b', 'c,d'],
    ['x"y', '', ''],
    ['z', '', 'w'],
  ]);
});

test('imports the sender_history.csv header columns', () => {
  const rows = parseCsv(fs.readFileSync(SENDER_CSV, 'utf8'));
  assert.deepEqual(rows[0], [
    'sender_id',
    'sender_type',
    'first_seen',
    'last_seen',
    'total_broadcasts',
    'broadcasts_verified_accurate',
    'broadcasts_verified_false',
    'reliability_score',
    'current_status',
  ]);
});
