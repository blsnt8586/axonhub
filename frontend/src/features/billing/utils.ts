import type { BillingOverview, UserSubscription } from './data/billing';

export type BillingView =
  | 'wallet'
  | 'recharge'
  | 'orders'
  | 'subscriptions'
  | 'redeem'
  | 'affiliate'
  | 'notifications'
  | 'ledger'
  | 'usage';

export type BillingViewProps = {
  data?: BillingOverview;
  isLoading: boolean;
  currency: string;
  formatCurrency: Intl.NumberFormat;
};

export function microsToAmount(value: number) {
  return value / 1_000_000;
}

export function formatDate(value?: string | null) {
  if (!value) return '-';
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

export function normalizeAmount(value: string) {
  const trimmed = value.trim();
  if (!/^\d+(\.\d{1,2})?$/.test(trimmed)) return '';
  const amount = Number(trimmed);
  if (!Number.isFinite(amount) || amount <= 0) return '';
  return amount.toFixed(2);
}

export function usagePercent(subscription: UserSubscription) {
  if (subscription.includedAmountMicros <= 0) return 0;
  return Math.min(100, Math.round((subscription.usedAmountMicros / subscription.includedAmountMicros) * 100));
}
