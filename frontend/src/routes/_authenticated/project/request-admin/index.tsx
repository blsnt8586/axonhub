import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import { RouteGuard } from '@/components/route-guard';
import RequestsManagement from '@/features/requests';

function ProtectedRequestAdministration() {
  return (
    <ProjectGuard>
      <RouteGuard requiredScopes={['read_requests']} scopeLevel='any'>
        <RequestsManagement />
      </RouteGuard>
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/request-admin/')({
  validateSearch: (search: Record<string, unknown>) => search,
  component: ProtectedRequestAdministration,
});
