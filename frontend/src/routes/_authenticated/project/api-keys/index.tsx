import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import PersonalAPIKeys from '@/features/personal-api-keys';

function ProtectedProjectApiKeys() {
  return (
    <ProjectGuard>
      <PersonalAPIKeys />
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/api-keys/')({
  component: ProtectedProjectApiKeys,
});
