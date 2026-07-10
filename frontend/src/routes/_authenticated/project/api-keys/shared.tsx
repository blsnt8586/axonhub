import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import ApiKeys from '@/features/apikeys';

function SharedProjectAPIKeys() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_api_keys']} scopeLevel='project'>
        <ApiKeys />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/api-keys/shared')({ component: SharedProjectAPIKeys });
