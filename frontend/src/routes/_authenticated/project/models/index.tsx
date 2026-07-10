import { createFileRoute } from '@tanstack/react-router';
import { ProjectGuard } from '@/components/project-guard';
import ConsumerModelCatalog from '@/features/model-catalog';

function ProtectedConsumerModels() {
  return (
    <ProjectGuard>
      <ConsumerModelCatalog />
    </ProjectGuard>
  );
}

export const Route = createFileRoute('/_authenticated/project/models/')({
  component: ProtectedConsumerModels,
});
