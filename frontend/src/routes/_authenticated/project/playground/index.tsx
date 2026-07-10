import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import Playground from '@/features/playground';

function ProtectedPlayground() {
  return (
    <ProjectGuard>
      <Playground />
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/playground/')({
  component: ProtectedPlayground,
});
