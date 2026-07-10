import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/lib/api-client';

export interface WorkspaceCapabilities {
  consumeAI: boolean;
  manageOwnAPIKeys: boolean;
  viewOwnUsage: boolean;
  manageMembers: boolean;
  manageRoles: boolean;
  manageSharedKeys: boolean;
}

export interface UserWorkspace {
  id: string;
  name: string;
  description: string;
  status: 'active' | 'archived';
  isOwner: boolean;
  capabilities: WorkspaceCapabilities;
}

export interface UserWorkspaceSettings {
  allowSelfServiceCreation: boolean;
  maxWorkspacesPerUser: number;
}

export interface UserWorkspaceList {
  settings: UserWorkspaceSettings;
  defaultWorkspaceId?: string;
  workspaces: UserWorkspace[];
}

export function useUserWorkspaces() {
  return useQuery({
    queryKey: ['user-workspaces'],
    queryFn: () => apiRequest<UserWorkspaceList>('/admin/account/workspaces', { requireAuth: true }),
  });
}

export function useCreateUserWorkspace() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { name: string; description?: string }) =>
      apiRequest<UserWorkspace>('/admin/account/workspaces', {
        method: 'POST',
        requireAuth: true,
        body: input,
      }),
    onSuccess: () =>
      Promise.all([queryClient.invalidateQueries({ queryKey: ['user-workspaces'] }), queryClient.invalidateQueries({ queryKey: ['me'] })]),
  });
}

export function useWorkspaceSettings() {
  return useQuery({
    queryKey: ['workspace-settings'],
    queryFn: () => apiRequest<UserWorkspaceSettings>('/admin/system/workspaces', { requireAuth: true }),
  });
}

export function useUpdateWorkspaceSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: UserWorkspaceSettings) =>
      apiRequest<UserWorkspaceSettings>('/admin/system/workspaces', {
        method: 'PUT',
        requireAuth: true,
        body: input,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['workspace-settings'] });
      queryClient.invalidateQueries({ queryKey: ['user-workspaces'] });
    },
  });
}
