import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import { UpstreamAccountMonitoringPage } from '@/features/channels/components/upstream-account-monitoring-page';

function ProtectedUpstreamAccountMonitoring() {
  return (
    <RouteGuard requiredScopes={['read_channels']} scopeLevel='system'>
      <UpstreamAccountMonitoringPage />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/channels/accounts/')({
  component: ProtectedUpstreamAccountMonitoring,
});
