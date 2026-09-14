import type {
  AuthLoginRequest,
  AuthLoginResponse,
  AuthSessionResponse,
  BroadcastFilters,
  BroadcastsResponse,
  DashboardResponse,
  HealthResponse,
  IncidentSummaryResponse,
  OutpostDetailResponse,
  StationBroadcastsResponse,
  StationAccountsResponse,
  StationsResponse,
  SteganographyConversationResponse,
  SteganographyDecodeRequest,
  SteganographyDecodeResponse,
  SteganographyEncodeRequest,
  SteganographyEncodeResponse,
} from '@vts/common';

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? '';
export const authTokenStorageKey = 'silent-outposts:auth-token';
let currentAuthToken = typeof window === 'undefined'
  ? ''
  : window.sessionStorage.getItem(authTokenStorageKey) ?? '';

export function setAuthToken(token?: string) {
  currentAuthToken = token ?? '';
  if (typeof window === 'undefined') return;
  if (currentAuthToken) {
    window.sessionStorage.setItem(authTokenStorageKey, currentAuthToken);
  } else {
    window.sessionStorage.removeItem(authTokenStorageKey);
  }
}

export function getAuthToken(): string {
  return currentAuthToken;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  if (currentAuthToken && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${currentAuthToken}`);
  }
  const response = await fetch(`${apiBaseUrl}${path}`, { ...init, headers });
  if (!response.ok) {
    const payload = await response.json().catch(() => undefined) as { error?: string } | undefined;
    throw new ApiError(payload?.error || `Request failed with status ${response.status}`, response.status);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export class ApiError extends Error {
  constructor(message: string, readonly status: number) {
    super(message);
    this.name = 'ApiError';
  }
}

export function fetchStationAccounts(): Promise<StationAccountsResponse> {
  return request('/api/auth/stations');
}

export function loginStation(payload: AuthLoginRequest): Promise<AuthLoginResponse> {
  return request('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
}

export function fetchAuthSession(): Promise<AuthSessionResponse> {
  return request('/api/auth/session');
}

export function logoutStation(): Promise<void> {
  return request('/api/auth/logout', { method: 'POST' });
}

export function fetchDashboard(): Promise<DashboardResponse> {
  return request('/api/dashboard');
}

export function fetchHealth(): Promise<HealthResponse> {
  return request('/api/health');
}

export function fetchStations(): Promise<StationsResponse> {
  return request('/api/stations');
}

export function fetchStationBroadcasts(senderId: string): Promise<StationBroadcastsResponse> {
  return request(`/api/stations/${encodeURIComponent(senderId)}/broadcasts`);
}

export function fetchBroadcasts(filters: BroadcastFilters = {}): Promise<BroadcastsResponse> {
  const query = new URLSearchParams();
  if (filters.senderId) query.set('senderId', filters.senderId);
  if (filters.location) query.set('location', filters.location);
  if (filters.type) query.set('type', filters.type);
  if (filters.crossCheckStatus) query.set('crossCheckStatus', filters.crossCheckStatus);
  if (filters.q) query.set('q', filters.q);
  if (filters.page) query.set('page', String(filters.page));
  if (filters.limit) query.set('limit', String(filters.limit));
  const suffix = query.size ? `?${query.toString()}` : '';
  return request(`/api/broadcasts${suffix}`);
}

export function fetchOutpost(senderId: string): Promise<OutpostDetailResponse> {
  return request(`/api/outposts/${encodeURIComponent(senderId)}`);
}

export function fetchIncidentSummary(senderId: string): Promise<IncidentSummaryResponse> {
  return request('/api/ai/incident-summary', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ senderId }),
  });
}

export function fetchSteganographyConversation(
  conversationId: string,
  stationId: string,
): Promise<SteganographyConversationResponse> {
  const query = new URLSearchParams({ stationId });
  return request(`/api/steganography/conversations/${encodeURIComponent(conversationId)}?${query.toString()}`);
}

export function encodeSteganography(payload: SteganographyEncodeRequest): Promise<SteganographyEncodeResponse> {
  return request('/api/steganography/encode', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
}

export function decodeSteganography(payload: SteganographyDecodeRequest): Promise<SteganographyDecodeResponse> {
  return request('/api/steganography/decode', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
}
