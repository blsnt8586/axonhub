import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import UserRequestsPage from '@/features/user-usage/requests-page';

function ProtectedProjectRequests() {
  return (
    <ProjectGuard>
      <UserRequestsPage />
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/requests/')({
  validateSearch: (search: Record<string, unknown>) => search,
  component: ProtectedProjectRequests,
});
