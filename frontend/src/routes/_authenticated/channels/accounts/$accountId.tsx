import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import { UpstreamAccountDetailPage } from '@/features/channels/components/upstream-account-detail-page';

function ProtectedUpstreamAccountDetail() {
  return (
    <RouteGuard requiredScopes={['read_channels']} scopeLevel='system'>
      <UpstreamAccountDetailPage />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/channels/accounts/$accountId')({
  component: ProtectedUpstreamAccountDetail,
});
