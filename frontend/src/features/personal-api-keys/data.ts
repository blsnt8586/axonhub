import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/lib/api-client';

export type PersonalAPIKeyStatus = 'enabled' | 'disabled' | 'archived';
export type RequestLimitWindow = 'all_time' | 'minute' | 'hour' | 'day';

export interface CommercialLimits {
  enabled: boolean;
  currency?: string;
  totalBudgetMicros?: number | null;
  dailyBudgetMicros?: number | null;
  monthlyBudgetMicros?: number | null;
  singleRequestMaxMicros?: number | null;
  notes?: string | null;
}

export interface PersonalAPIKey {
  id: string;
  projectId: string;
  name: string;
  type: 'user' | 'personal';
  status: PersonalAPIKeyStatus;
  maskedKey: string;
  expiresAt?: string;
  ipAllowlist: string[];
  allowedModelIds: string[];
  requestLimit?: number;
  requestLimitWindow?: RequestLimitWindow;
  commercialLimits?: CommercialLimits;
  createdAt: string;
  updatedAt: string;
}

export interface PersonalAPIKeyModel {
  modelId: string;
  currency?: string;
  price?: { items: Array<{ itemCode: string; pricing: Record<string, unknown> }> };
}

export interface PersonalAPIKeyInput {
  name: string;
  expiresAt?: string | null;
  ipAllowlist: string[];
  allowedModelIds: string[];
  requestLimit?: number | null;
  requestLimitWindow?: RequestLimitWindow;
  commercialLimits?: CommercialLimits | null;
}

function projectQuery(projectId: string, keyId?: string) {
  const params = new URLSearchParams({ projectId });
  if (keyId) params.set('keyId', keyId);
  return params.toString();
}

export function usePersonalAPIKeys(projectId?: string | null) {
  return useQuery({
    queryKey: ['personal-api-keys', projectId],
    queryFn: () => apiRequest<{ apiKeys: PersonalAPIKey[] }>(`/admin/account/api-keys?${projectQuery(projectId!)}`, { requireAuth: true }),
    enabled: Boolean(projectId),
  });
}

export function usePersonalAPIKeyModels(projectId?: string | null) {
  return useQuery({
    queryKey: ['personal-api-key-models', projectId],
    queryFn: () =>
      apiRequest<{ models: PersonalAPIKeyModel[] }>(`/admin/account/api-key-models?${projectQuery(projectId!)}`, {
        requireAuth: true,
      }),
    enabled: Boolean(projectId),
  });
}

export function useCreatePersonalAPIKey(projectId?: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: PersonalAPIKeyInput) =>
      apiRequest<{ apiKey: PersonalAPIKey; secret: string }>(`/admin/account/api-keys?${projectQuery(projectId!)}`, {
        method: 'POST',
        requireAuth: true,
        body: input,
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['personal-api-keys', projectId] }),
  });
}

export function useUpdatePersonalAPIKey(projectId?: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ keyId, input }: { keyId: string; input: Record<string, unknown> }) =>
      apiRequest<PersonalAPIKey>(`/admin/account/api-keys?${projectQuery(projectId!, keyId)}`, {
        method: 'PATCH',
        requireAuth: true,
        body: input,
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['personal-api-keys', projectId] }),
  });
}

export function useRotatePersonalAPIKey(projectId?: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (keyId: string) =>
      apiRequest<{ apiKey: PersonalAPIKey; secret: string }>(`/admin/account/api-keys/rotate?${projectQuery(projectId!, keyId)}`, {
        method: 'POST',
        requireAuth: true,
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['personal-api-keys', projectId] }),
  });
}

export function useArchivePersonalAPIKey(projectId?: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (keyId: string) =>
      apiRequest<void>(`/admin/account/api-keys?${projectQuery(projectId!, keyId)}`, {
        method: 'DELETE',
        requireAuth: true,
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['personal-api-keys', projectId] }),
  });
}
