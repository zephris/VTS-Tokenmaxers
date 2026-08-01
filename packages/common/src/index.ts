import { z } from 'zod';

export const broadcastTypes = [
  'all_clear',
  'warning',
  'supply_request',
  'situation_report',
  'routine_check',
  'emergency',
] as const;

export type BroadcastType = (typeof broadcastTypes)[number];
export type SenderStatus = 'active' | 'gone_quiet' | 'suspected_compromised';
export type CrossCheckStatus = 'verified' | 'disputed' | 'unconfirmed' | 'not_checked';
export type GroundTruthLabel = 'genuine' | 'peacock_spoofed' | 'outdated' | 'unknown';
export type RiskLevel = 'low' | 'medium' | 'high' | 'critical';

export interface Broadcast {
  id: string;
  timestamp: string;
  senderId: string;
  location: string;
  type: BroadcastType;
  messageText: string | null;
  signalStrength: number | null;
  crossCheckStatus: CrossCheckStatus;
  label?: GroundTruthLabel;
}

export interface OutpostSummary {
  senderId: string;
  location: string;
  lastSeen: string;
  hoursSilent: number;
  reliabilityScore: number;
  status: SenderStatus;
  riskLevel: RiskLevel;
}

export interface DashboardResponse {
  analysisTimestamp: string;
  outposts: OutpostSummary[];
  recentBroadcasts: Broadcast[];
}

export const incidentSummaryRequestSchema = z.object({
  senderId: z.string().trim().min(1).max(100),
});

export type IncidentSummaryRequest = z.infer<typeof incidentSummaryRequestSchema>;

export interface IncidentSummaryResponse {
  senderId: string;
  summary: string;
  source: 'ai' | 'fallback';
  evidenceIds: string[];
}

export type SenderType = 'robot_outpost' | 'relay_identity' | 'junior_scout_group';

export interface Station {
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
  derived: boolean;
}

export interface StationsResponse {
  stations: Station[];
}

export interface StationBroadcastsResponse {
  senderId: string;
  broadcasts: Broadcast[];
}

export type ServerMode = 'full' | 'dataset-only';

export interface HealthResponse {
  ok: boolean;
  mode: ServerMode;
  counts: {
    stations: number;
    broadcasts: number;
  };
}

