import type { DashboardResponse, IncidentSummaryResponse } from '@vts/common';

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? '';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, init);
  if (!response.ok) throw new Error(`Request failed with status ${response.status}`);
  return response.json() as Promise<T>;
}

export function fetchDashboard(): Promise<DashboardResponse> {
  return request('/api/dashboard');
}

export function fetchIncidentSummary(senderId: string): Promise<IncidentSummaryResponse> {
  return request('/api/ai/incident-summary', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ senderId }),
  });
}

