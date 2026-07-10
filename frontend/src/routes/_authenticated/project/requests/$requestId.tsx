import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import UserRequestDetailPage from '@/features/user-usage/request-detail';

function ProtectedRequestDetail() {
  return (
    <ProjectGuard>
      <UserRequestDetailPage />
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/requests/$requestId')({
  validateSearch: (search: Record<string, unknown>) => ({ scope: search.scope === 'project' ? ('project' as const) : ('mine' as const) }),
  component: ProtectedRequestDetail,
});
