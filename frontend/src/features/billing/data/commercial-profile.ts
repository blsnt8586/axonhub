import { useQuery } from '@tanstack/react-query';
import { apiRequest } from '@/lib/api-client';

export type CommercialRequestType = 'chat' | 'image' | 'video' | 'embedding' | 'audio' | 'other';

export interface CommercialProfileFilter {
  from?: string;
  to?: string;
  projectId?: string | number;
  apiKeyId?: string | number;
  modelId?: string;
  requestType?: CommercialRequestType | 'all' | '';
  limit?: number;
}

export interface CommercialProfileUser {
  id: number;
  email: string;
}

export interface CommercialProfileBillingAccount {
  id: number;
  ownerType: string;
  ownerId: number;
  currency: string;
  balanceMicros: number;
  heldBalanceMicros: number;
  creditLimitMicros: number;
  availableMicros: number;
  status: string;
  totalRechargeMicros: number;
}

export interface CommercialProfileTotals {
  totalRechargeMicros: number;
  totalConsumptionMicros: number;
  todayConsumptionMicros: number;
  monthConsumptionMicros: number;
  requestCount: number;
  failureCount: number;
}

export interface CommercialProfileRankedItem {
  id: string;
  name: string;
  chargeAmountMicros: number;
  requestCount: number;
}

export interface CommercialProfileCharge {
  id: number;
  createdAt: string;
  usageLogId: number;
  billingAccountId: number;
  projectId: number;
  projectName: string;
  userId?: number | null;
  apiKeyId?: number | null;
  apiKeyName?: string;
  modelId: string;
  requestType: CommercialRequestType;
  chargeAmountMicros: number;
  costAmountMicros: number;
  currency: string;
  status: string;
  error: string;
}

export interface CommercialProfileRequest {
  id: number;
  createdAt: string;
  projectId: number;
  projectName: string;
  apiKeyId?: number | null;
  apiKeyName?: string;
  modelId: string;
  requestType: CommercialRequestType;
  format: string;
  source: string;
  status: string;
  stream: boolean;
  latencyMs?: number | null;
  firstTokenMs?: number | null;
  reasoningMs?: number | null;
}

export interface CommercialProfileBillingFailure {
  id: number;
  createdAt: string;
  usageLogId: number;
  projectId: number;
  projectName: string;
  apiKeyId?: number | null;
  apiKeyName?: string;
  modelId: string;
  requestType: CommercialRequestType;
  chargeAmountMicros: number;
  currency: string;
  status: string;
  error: string;
}

export interface CommercialProfile {
  user: CommercialProfileUser;
  filter: CommercialProfileFilter & { limit: number };
  billingAccount: CommercialProfileBillingAccount;
  totals: CommercialProfileTotals;
  topModels: CommercialProfileRankedItem[];
  topProjects: CommercialProfileRankedItem[];
  topApiKeys: CommercialProfileRankedItem[];
  recentCharges: CommercialProfileCharge[];
  recentRequests: CommercialProfileRequest[];
  recentBillingFailures: CommercialProfileBillingFailure[];
}

export function useMyCommercialProfile(filter: CommercialProfileFilter = {}) {
  return useQuery({
    queryKey: ['commercial-profile', 'me', normalizeCommercialProfileFilter(filter)],
    queryFn: () => apiRequest<CommercialProfile>(`/admin/account/commercial-profile${commercialProfileQuery(filter)}`, { requireAuth: true }),
  });
}

export function useUserCommercialProfile(userId?: number, filter: CommercialProfileFilter = {}) {
  return useQuery({
    queryKey: ['commercial-profile', 'user', userId, normalizeCommercialProfileFilter(filter)],
    enabled: Boolean(userId),
    queryFn: () => apiRequest<CommercialProfile>(`/admin/users/${userId}/commercial-profile${commercialProfileQuery(filter)}`, { requireAuth: true }),
  });
}

function commercialProfileQuery(filter: CommercialProfileFilter) {
  const params = new URLSearchParams();
  const normalized = normalizeCommercialProfileFilter(filter);
  Object.entries(normalized).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '' || value === 'all') return;
    params.set(key, String(value));
  });
  const query = params.toString();
  return query ? `?${query}` : '';
}

function normalizeCommercialProfileFilter(filter: CommercialProfileFilter) {
  return {
    from: toAPITime(filter.from),
    to: toAPITime(filter.to),
    projectId: emptyToUndefined(filter.projectId),
    apiKeyId: emptyToUndefined(filter.apiKeyId),
    modelId: emptyToUndefined(filter.modelId),
    requestType: filter.requestType && filter.requestType !== 'all' ? filter.requestType : undefined,
    limit: filter.limit,
  };
}

function emptyToUndefined(value: unknown) {
  if (value === undefined || value === null) return undefined;
  const text = String(value).trim();
  return text ? text : undefined;
}

function toAPITime(value?: string) {
  const text = value?.trim();
  if (!text) return undefined;
  if (/^\d{4}-\d{2}-\d{2}$/.test(text)) return text;
  const date = new Date(text);
  if (Number.isNaN(date.getTime())) return text;
  return date.toISOString();
}
