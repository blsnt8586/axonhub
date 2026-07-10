interface SelectableWorkspace {
  id: string;
  status: string;
}

export function resolveSelectedWorkspaceId(
  workspaces: SelectableWorkspace[],
  storedWorkspaceId: string | null,
  defaultWorkspaceId?: string | null
): string | null {
  const activeWorkspaces = workspaces.filter((workspace) => workspace.status === 'active');
  if (storedWorkspaceId && activeWorkspaces.some((workspace) => workspace.id === storedWorkspaceId)) {
    return storedWorkspaceId;
  }
  if (defaultWorkspaceId && activeWorkspaces.some((workspace) => workspace.id === defaultWorkspaceId)) {
    return defaultWorkspaceId;
  }
  return activeWorkspaces[0]?.id ?? null;
}
