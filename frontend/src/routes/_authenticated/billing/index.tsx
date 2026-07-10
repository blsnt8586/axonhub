import { createFileRoute } from '@tanstack/react-router';
import BillingPage from '@/features/billing';
import type { BillingView } from '@/features/billing/utils';

const billingViews = new Set<BillingView>([
  'wallet',
  'recharge',
  'orders',
  'subscriptions',
  'redeem',
  'affiliate',
  'notifications',
  'ledger',
  'usage',
]);

export const Route = createFileRoute('/_authenticated/billing/')({
  validateSearch: (search: Record<string, unknown>) => ({
    view: billingViews.has(search.view as BillingView) ? (search.view as BillingView) : ('wallet' as const),
  }),
  component: BillingPage,
});
