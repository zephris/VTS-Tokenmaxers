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
export type SenderType = 'robot_outpost' | 'relay_identity' | 'junior_scout_group';
export type SenderStatus = 'active' | 'gone_quiet' | 'suspected_compromised';
export type CrossCheckStatus = 'verified' | 'disputed' | 'unconfirmed' | 'not_checked';
export type RiskLevel = 'low' | 'medium' | 'high' | 'critical';
export type EncryptionStatus = 'plain' | 'encrypted';
export type SenderProfileSource = 'dataset' | 'derived';

export interface Broadcast {
  id: string;
  timestamp: string;
  senderId: string;
  location: string;
  type: BroadcastType;
  messageText: string | null;
  carrierText: string | null;
  encryptionStatus: EncryptionStatus;
  signalStrength: number | null;
  crossCheckStatus: CrossCheckStatus;
}

export interface StationSummary {
  senderId: string;
  senderType: SenderType;
  location: string;
  lastBroadcastAt: string;
  lastMessagePreview: string | null;
  broadcastCount: number;
  reliabilityScore: number | null;
  status: SenderStatus;
  profileSource: SenderProfileSource;
}

export interface StationsResponse {
  analysisTimestamp: string;
  stations: StationSummary[];
}

export interface StationBroadcastsResponse {
  station: StationSummary;
  broadcasts: Broadcast[];
}

export interface StationAccountSummary {
  stationId: string;
  username: string;
  senderType: SenderType;
  location: string;
}

export interface StationAccountsResponse {
  accounts: StationAccountSummary[];
}

export interface AuthAccount {
  stationId: string;
  username: string;
}

export const authLoginRequestSchema = z.object({
  username: z.string().trim().min(1).max(100),
  password: z.string().min(1).max(160),
});

export type AuthLoginRequest = z.infer<typeof authLoginRequestSchema>;

export interface AuthLoginResponse {
  account: AuthAccount;
  token: string;
  expiresAt: string;
}

export interface AuthSessionResponse {
  account: AuthAccount;
  expiresAt: string;
}

export interface OutpostSummary {
  senderId: string;
  location: string;
  lastSeen: string;
  hoursSilent: number;
  reliabilityScore: number;
  status: SenderStatus;
  riskLevel: RiskLevel;
  expectedCadenceHours: number;
  silenceRatio: number;
  unresolvedEmergencyCount: number;
  contradictionCount: number;
  riskReasons: RiskReason[];
  riskScore: number;
  broadcastCount: number;
}

export interface RiskReason {
  code: string;
  message: string;
  severity: RiskLevel;
  evidenceIds: string[];
}

export interface DashboardResponse {
  analysisTimestamp: string;
  outposts: OutpostSummary[];
  recentBroadcasts: Broadcast[];
}

export interface BroadcastFilters {
  senderId?: string;
  location?: string;
  type?: BroadcastType;
  crossCheckStatus?: CrossCheckStatus;
  q?: string;
  page?: number;
  limit?: number;
}

export interface Pagination {
  page: number;
  limit: number;
  total: number;
  totalPages: number;
}

export interface BroadcastsResponse {
  broadcasts: Broadcast[];
  pagination: Pagination;
}

export interface OutpostDetail extends OutpostSummary {
  broadcastCount: number;
}

export interface OutpostDetailResponse {
  analysisTimestamp: string;
  outpost: OutpostDetail;
  evidenceTimeline: Broadcast[];
}

export interface HealthResponse {
  ok: boolean;
  mode: 'data-only' | 'full';
  data: {
    stationCount: number;
    broadcastCount: number;
  };
  steganography: {
    configured: boolean;
    algorithm: string;
    modelFingerprint?: string;
  };
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

export interface SteganographyRecord {
  index: number;
  from: string;
  senderSequence: number;
  carrierText: string;
  broadcastType?: BroadcastType;
  createdAt?: string;
}

export interface SteganographyConversationResponse {
  conversationId: string;
  stationId: string;
  records: SteganographyRecord[];
  syncCode: string;
  algorithm: string;
  modelFingerprint?: string;
}

const steganographyBaseSchema = z.object({
  conversationId: z.string().trim().min(1).max(200),
  stationId: z.string().trim().min(1).max(100),
  sender: z.string().trim().min(1).max(100),
  secretPhrase: z.string().min(16).max(160),
});

export const steganographyEncodeRequestSchema = steganographyBaseSchema.extend({
  plaintext: z.string().min(1).max(4000),
  broadcastType: z.enum(broadcastTypes).optional(),
});

export type SteganographyEncodeRequest = z.infer<typeof steganographyEncodeRequestSchema>;

export const steganographyDecodeRequestSchema = steganographyBaseSchema.extend({
  carrierText: z.string().min(1).max(32_000),
  broadcastType: z.enum(broadcastTypes).optional(),
});

export type SteganographyDecodeRequest = z.infer<typeof steganographyDecodeRequestSchema>;

export interface SteganographyEncodeResponse extends SteganographyConversationResponse {
  sender: string;
  carrierText: string;
  record: SteganographyRecord;
}

export interface SteganographyDecodeResponse extends SteganographyConversationResponse {
  sender: string;
  carrierText: string;
  plaintext: string;
  record: SteganographyRecord;
}
