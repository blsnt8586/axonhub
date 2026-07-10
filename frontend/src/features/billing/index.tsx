import { useMemo } from 'react';
import { useNavigate, useSearch } from '@tanstack/react-router';
import {
  Bell,
  ChartNoAxesCombined,
  CreditCard,
  Gift,
  HandCoins,
  Loader2,
  PackageCheck,
  ReceiptText,
  RefreshCw,
  ShoppingCart,
  Wallet,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { useMyBillingOverview } from './data/billing';
import { type BillingView } from './utils';
import { AffiliateView } from './views/affiliate-view';
import { LedgerView } from './views/ledger-view';
import { NotificationsView } from './views/notifications-view';
import { OrdersView } from './views/orders-view';
import { RechargeView } from './views/recharge-view';
import { RedeemView } from './views/redeem-view';
import { SubscriptionsView } from './views/subscriptions-view';
import { UsageChargesView } from './views/usage-view';
import { WalletView } from './views/wallet-view';

const views: Array<{ value: BillingView; icon: typeof Wallet; label: string }> = [
  { value: 'wallet', icon: Wallet, label: 'billing.views.wallet' },
  { value: 'recharge', icon: CreditCard, label: 'billing.views.recharge' },
  { value: 'orders', icon: ShoppingCart, label: 'billing.views.orders' },
  { value: 'subscriptions', icon: PackageCheck, label: 'billing.views.subscriptions' },
  { value: 'redeem', icon: Gift, label: 'billing.views.redeem' },
  { value: 'affiliate', icon: HandCoins, label: 'billing.views.affiliate' },
  { value: 'notifications', icon: Bell, label: 'billing.views.notifications' },
  { value: 'ledger', icon: ReceiptText, label: 'billing.views.ledger' },
  { value: 'usage', icon: ChartNoAxesCombined, label: 'billing.views.usage' },
];

export default function BillingPage() {
  const { t, i18n } = useTranslation();
  const search = useSearch({ from: '/_authenticated/billing/' });
  const navigate = useNavigate({ from: '/billing' });
  const activeView = search.view;
  const overview = useMyBillingOverview(20);
  const currency = overview.data?.account.currency || 'CNY';
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';
  const formatCurrency = useMemo(
    () =>
      new Intl.NumberFormat(locale, {
        style: 'currency',
        currency,
        currencyDisplay: 'narrowSymbol',
        minimumFractionDigits: 2,
        maximumFractionDigits: 6,
      }),
    [currency, locale]
  );
  const props = { data: overview.data, isLoading: overview.isLoading, currency, formatCurrency };

  return (
    <div className='flex flex-1 flex-col overflow-hidden' data-testid='billing-self-service'>
      <Header fixed>
        <div className='flex min-w-0 flex-1 items-center justify-between gap-4'>
          <div className='min-w-0'>
            <h1 className='truncate text-xl font-bold'>{t('billing.title')}</h1>
            <p className='text-muted-foreground truncate text-sm'>{t('billing.description')}</p>
          </div>
          <Button variant='outline' size='sm' onClick={() => overview.refetch()} disabled={overview.isFetching}>
            {overview.isFetching ? <Loader2 className='size-4 animate-spin' /> : <RefreshCw className='size-4' />}
            <span className='hidden sm:inline'>{t('common.refresh')}</span>
          </Button>
        </div>
      </Header>

      <Main fixed className='flex min-w-0 flex-col gap-5 overflow-auto'>
        {overview.error && (
          <Alert variant='destructive'>
            <AlertTitle>{t('common.loadError')}</AlertTitle>
            <AlertDescription>
              {overview.error instanceof Error ? overview.error.message : t('common.errors.unknownError')}
            </AlertDescription>
          </Alert>
        )}
        <nav className='min-w-0 overflow-x-auto border-b' aria-label={t('billing.views.navigation')} data-testid='billing-view-navigation'>
          <div className='flex min-w-max gap-1 pb-2'>
            {views.map(({ value, icon: Icon, label }) => (
              <Button
                key={value}
                data-testid={`billing-view-${value}-tab`}
                size='sm'
                variant={activeView === value ? 'default' : 'ghost'}
                aria-pressed={activeView === value}
                onClick={() => navigate({ search: { view: value } })}
              >
                <Icon className='size-4' />
                {t(label)}
              </Button>
            ))}
          </div>
        </nav>

        {activeView === 'wallet' && <WalletView {...props} />}
        {activeView === 'recharge' && <RechargeView {...props} />}
        {activeView === 'orders' && <OrdersView {...props} />}
        {activeView === 'subscriptions' && <SubscriptionsView {...props} />}
        {activeView === 'redeem' && <RedeemView {...props} />}
        {activeView === 'affiliate' && <AffiliateView {...props} />}
        {activeView === 'notifications' && <NotificationsView {...props} />}
        {activeView === 'ledger' && <LedgerView {...props} />}
        {activeView === 'usage' && <UsageChargesView {...props} />}
      </Main>
    </div>
  );
}
