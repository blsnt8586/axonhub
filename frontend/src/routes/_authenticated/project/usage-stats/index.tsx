import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import UserUsagePage from '@/features/user-usage/usage-page';

function ProtectedUsageStats() {
  return (
    <ProjectGuard>
      <UserUsagePage />
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/usage-stats/')({
  component: ProtectedUsageStats,
});
