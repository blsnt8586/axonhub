import type { RouteConfig, ScopeLevel } from './route-permission';

export interface RouteAccessContext {
  systemScopes: string[];
  projectScopes: string[];
  isSystemOwner: boolean;
  isProjectOwner: boolean;
}

export function canAccessRoute(
  routeConfig: Pick<RouteConfig, 'requiredScopes' | 'requireProjectOwner' | 'scopeLevel'>,
  groupScopeLevel: ScopeLevel | undefined,
  context: RouteAccessContext
): boolean {
  if (routeConfig.requireProjectOwner && !context.isProjectOwner) {
    return false;
  }

  const requiredScopes = routeConfig.requiredScopes ?? [];
  if (requiredScopes.length === 0 || context.isSystemOwner) {
    return true;
  }

  const scopeLevel = routeConfig.scopeLevel ?? groupScopeLevel ?? 'any';
  if ((scopeLevel === 'project' || scopeLevel === 'any') && context.isProjectOwner) {
    return true;
  }

  const scopes =
    scopeLevel === 'system'
      ? context.systemScopes
      : scopeLevel === 'project'
        ? context.projectScopes
        : [...context.systemScopes, ...context.projectScopes];

  return scopes.includes('*') || requiredScopes.some((scope) => scopes.includes(scope));
}
