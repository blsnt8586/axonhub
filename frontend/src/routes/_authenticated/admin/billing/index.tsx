import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import AdminBillingPage from '@/features/admin-billing';

function ProtectedAdminBilling() {
  return (
    <RouteGuard requiredScopes={['read_billing']} scopeLevel='system'>
      <AdminBillingPage />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/admin/billing/')({
  component: ProtectedAdminBilling,
});
