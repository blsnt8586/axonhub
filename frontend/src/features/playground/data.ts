import { useQuery } from '@tanstack/react-query';
import { apiRequest } from '@/lib/api-client';

export type PlaygroundBlockReason =
  | 'none'
  | 'no_model'
  | 'no_enabled_key'
  | 'key_disabled'
  | 'key_expired'
  | 'key_restricted'
  | 'balance_insufficient'
  | 'account_unavailable'
  | 'rate_limited'
  | 'upstream_unavailable';

export interface PlaygroundAPIKey {
  id: string;
  name: string;
  status: 'enabled' | 'disabled';
  usable: boolean;
  blockReason: 'none' | 'disabled' | 'expired' | 'ip_not_allowed';
  allowedModelIds: string[];
}

export interface PlaygroundModel {
  modelId: string;
  displayName: string;
  modality: string;
  availability: 'available';
  currency?: string;
  price?: { items: Array<{ itemCode: string; pricing: Record<string, unknown> }> };
  priceRule?: { scope: 'global' | 'project'; pattern: string };
}

export interface PlaygroundState {
  projectId: string;
  canSend: boolean;
  blockReason: PlaygroundBlockReason;
  apiKeys: PlaygroundAPIKey[];
  models: PlaygroundModel[];
}

export function usePlaygroundState(projectId?: string | null) {
  return useQuery({
    queryKey: ['user-playground', projectId],
    queryFn: () =>
      apiRequest<PlaygroundState>(`/admin/account/playground?${new URLSearchParams({ projectId: projectId! })}`, {
        requireAuth: true,
      }),
    enabled: Boolean(projectId),
  });
}
