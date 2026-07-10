import { useQuery } from '@tanstack/react-query';
import { apiRequest } from '@/lib/api-client';

export type WorkspaceBlockReason =
  | 'none'
  | 'user_inactive'
  | 'project_missing'
  | 'api_key_missing'
  | 'model_unavailable'
  | 'account_unavailable'
  | 'balance_insufficient';

export interface UserWorkspaceSummary {
  user: {
    id: number;
    email: string;
    status: string;
  };
  project?: {
    id: number;
    name: string;
    status: string;
    isOwner: boolean;
  };
  billing: {
    currency: string;
    balanceMicros: number;
    heldBalanceMicros: number;
    creditLimitMicros: number;
    availableMicros: number;
    status: string;
  };
  apiKeys: {
    total: number;
    enabled: number;
  };
  models: {
    availableCount: number;
  };
  subscriptions: {
    activeCount: number;
    usableCoverage: boolean;
  };
  usage: {
    requestCount: number;
    todayConsumptionMicros: number;
  };
  onboarding: {
    canUseAI: boolean;
    blockReason: WorkspaceBlockReason;
  };
}

export function useUserWorkspaceSummary(projectId?: string) {
  return useQuery({
    queryKey: ['user-workspace-summary', projectId],
    queryFn: () => {
      const query = projectId ? `?projectId=${encodeURIComponent(projectId)}` : '';
      return apiRequest<UserWorkspaceSummary>(`/admin/account/workspace-summary${query}`, { requireAuth: true });
    },
  });
}
