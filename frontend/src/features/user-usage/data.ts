import { useQuery } from '@tanstack/react-query';
import { apiRequest } from '@/lib/api-client';
import { getTokenFromStorage } from '@/stores/authStore';

export type UserUsageScope = 'mine' | 'project';

export interface UserRequestAPIKey {
  id: string;
  name: string;
}

export interface UserRequestItem {
  id: string;
  createdAt: string;
  updatedAt: string;
  apiKey?: UserRequestAPIKey;
  source: string;
  modelId: string;
  status: 'pending' | 'processing' | 'completed' | 'failed' | 'canceled';
  format: string;
  stream: boolean;
  latencyMs?: number;
  firstTokenLatencyMs?: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  chargeAmountMicros: number;
  currency?: string;
  billingRecordId?: string;
}

export interface UserRequestPage {
  items: UserRequestItem[];
  total: number;
  offset: number;
  limit: number;
}

export interface UserBillingProjection {
  billingRecordId: string;
  ledgerTransactionId?: string;
  chargeAmountMicros: number;
  currency: string;
  status: string;
  priceReferenceId: string;
  priceSnapshot?: Record<string, unknown>;
  chargeItems: Array<Record<string, unknown>>;
}

export interface UserRequestDetail extends UserRequestItem {
  requestBody?: unknown;
  responseBody?: unknown;
  billing?: UserBillingProjection;
}

export interface UserUsageSeriesPoint {
  bucketStart: string;
  requestCount: number;
  successCount: number;
  errorCount: number;
  totalTokens: number;
  chargeAmountMicros: number;
}

export interface UserUsageModelTotal {
  modelId: string;
  requestCount: number;
  totalTokens: number;
  chargeAmountMicros: number;
}

export interface UserUsageSummary {
  scope: UserUsageScope;
  projectId: string;
  from: string;
  to: string;
  granularity: 'hour' | 'day';
  currency: string;
  requestCount: number;
  successCount: number;
  errorCount: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  chargeAmountMicros: number;
  series: UserUsageSeriesPoint[];
  models: UserUsageModelTotal[];
}

export interface UserRequestFilters {
  projectId?: string | null;
  scope: UserUsageScope;
  offset?: number;
  limit?: number;
  status?: string;
  modelId?: string;
  from?: string;
  to?: string;
}

function endpoint(scope: UserUsageScope, resource: 'requests' | 'usage') {
  if (scope === 'project') return resource === 'requests' ? '/admin/account/project-requests' : '/admin/account/project-usage';
  return resource === 'requests' ? '/admin/account/requests' : '/admin/account/usage';
}

function requestParams(filters: UserRequestFilters) {
  const params = new URLSearchParams({ projectId: filters.projectId! });
  if (filters.offset !== undefined) params.set('offset', String(filters.offset));
  if (filters.limit !== undefined) params.set('limit', String(filters.limit));
  if (filters.status) params.set('status', filters.status);
  if (filters.modelId?.trim()) params.set('modelId', filters.modelId.trim());
  if (filters.from) params.set('from', filters.from);
  if (filters.to) params.set('to', filters.to);
  return params;
}

export function useUserRequests(filters: UserRequestFilters) {
  return useQuery({
    queryKey: ['user-requests', filters],
    queryFn: () =>
      apiRequest<UserRequestPage>(`${endpoint(filters.scope, 'requests')}?${requestParams(filters)}`, { requireAuth: true }),
    enabled: Boolean(filters.projectId),
  });
}

export function useUserRequest(projectId: string | null | undefined, requestId: string, scope: UserUsageScope) {
  return useQuery({
    queryKey: ['user-request', projectId, requestId, scope],
    queryFn: () => {
      const params = new URLSearchParams({ projectId: projectId! });
      return apiRequest<UserRequestDetail>(`${endpoint(scope, 'requests')}/${encodeURIComponent(requestId)}?${params}`, {
        requireAuth: true,
      });
    },
    enabled: Boolean(projectId && requestId),
    retry: false,
  });
}

export function useUserUsage(projectId: string | null | undefined, scope: UserUsageScope, from: string, to: string) {
  return useQuery({
    queryKey: ['user-usage', projectId, scope, from, to],
    queryFn: () => {
      const params = new URLSearchParams({ projectId: projectId!, from, to });
      return apiRequest<UserUsageSummary>(`${endpoint(scope, 'usage')}?${params}`, { requireAuth: true });
    },
    enabled: Boolean(projectId && from && to),
  });
}

export async function downloadUserRequests(filters: UserRequestFilters) {
  const exportEndpoint = `${endpoint(filters.scope, 'requests')}/export?${requestParams(filters)}`;
  const response = await fetch(exportEndpoint, {
    headers: { Authorization: `Bearer ${getTokenFromStorage()}` },
  });
  if (!response.ok) {
    let message = `HTTP ${response.status}: ${response.statusText}`;
    try {
      const body = await response.json();
      message = body?.error?.message || body?.message || message;
    } catch {
      // Keep the HTTP status when the response is not JSON.
    }
    throw new Error(message);
  }
  const blob = await response.blob();
  const disposition = response.headers.get('Content-Disposition') || '';
  const filename = disposition.match(/filename="?([^";]+)"?/i)?.[1] || `requests-${filters.scope}.csv`;
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}
