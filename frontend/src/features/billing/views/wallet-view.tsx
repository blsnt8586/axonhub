import { Link } from '@tanstack/react-router';
import { Bell, CreditCard, ReceiptText, Wallet } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { BillingSection } from '../shared';
import { microsToAmount, type BillingView, type BillingViewProps } from '../utils';

export function WalletView({ data, isLoading, formatCurrency }: BillingViewProps) {
  const { t } = useTranslation();
  const balance = microsToAmount(data?.account.balanceMicros ?? 0);
  const held = microsToAmount(data?.account.heldBalanceMicros ?? 0);
  const credit = microsToAmount(data?.account.creditLimitMicros ?? 0);
  const available = balance + credit - held;
  const unread = data?.notifications.filter((item) => item.status === 'unread').length ?? 0;

  return (
    <div className='space-y-6' data-testid='billing-wallet-view'>
      <section className='grid gap-4 sm:grid-cols-2 xl:grid-cols-5'>
        <WalletMetric icon={Wallet} label={t('billing.cards.balance')} value={isLoading ? '-' : formatCurrency.format(balance)} />
        <WalletMetric icon={Wallet} label={t('billing.cards.available')} value={isLoading ? '-' : formatCurrency.format(available)} />
        <WalletMetric icon={ReceiptText} label={t('billing.cards.held')} value={isLoading ? '-' : formatCurrency.format(held)} />
        <WalletMetric icon={CreditCard} label={t('billing.cards.credit')} value={isLoading ? '-' : formatCurrency.format(credit)} />
        <div className='border p-4'>
          <p className='text-muted-foreground text-xs font-medium'>{t('billing.cards.status')}</p>
          <div className='mt-3 flex flex-wrap items-center gap-2'>
            <Badge variant={data?.account.status === 'active' ? 'default' : 'destructive'}>{data?.account.status || '-'}</Badge>
            <span className='text-muted-foreground text-xs'>{data?.account.currency || '-'}</span>
          </div>
        </div>
      </section>

      <BillingSection title={t('billing.wallet.actionsTitle')} description={t('billing.wallet.actionsDescription')}>
        <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
          <ActionLink
            view='recharge'
            icon={CreditCard}
            title={t('billing.recharge.title')}
            description={t('billing.recharge.description')}
          />
          <ActionLink
            view='subscriptions'
            icon={ReceiptText}
            title={t('billing.subscriptions.plansTitle')}
            description={t('billing.subscriptions.activeDescription')}
          />
          <ActionLink view='ledger' icon={Wallet} title={t('billing.ledger.title')} description={t('billing.ledger.description')} />
          <ActionLink
            view='notifications'
            icon={Bell}
            title={t('billing.notifications.title')}
            description={t('billing.wallet.unread', { count: unread })}
          />
        </div>
      </BillingSection>

      <BillingSection title={t('billing.wallet.recentTitle')} description={t('billing.wallet.recentDescription')}>
        <div className='divide-y border'>
          {(data?.ledgerTransactions ?? []).slice(0, 5).map((transaction) => (
            <div key={transaction.id} className='flex flex-wrap items-center justify-between gap-3 p-3 text-sm'>
              <div>
                <p className='font-medium'>{transaction.type}</p>
                <p className='text-muted-foreground text-xs'>{transaction.memo || transaction.status}</p>
              </div>
              <span className='font-mono tabular-nums'>
                {transaction.direction === 'debit' ? '-' : '+'}
                {formatCurrency.format(microsToAmount(transaction.amountMicros))}
              </span>
            </div>
          ))}
          {!isLoading && (data?.ledgerTransactions.length ?? 0) === 0 && (
            <p className='text-muted-foreground p-8 text-center text-sm'>{t('common.noData')}</p>
          )}
        </div>
      </BillingSection>
    </div>
  );
}

function WalletMetric({ icon: Icon, label, value }: { icon: typeof Wallet; label: string; value: string }) {
  return (
    <div className='border p-4'>
      <p className='text-muted-foreground flex items-center gap-2 text-xs font-medium'>
        <Icon className='size-4' />
        {label}
      </p>
      <p className='mt-3 font-mono text-xl font-semibold break-words tabular-nums'>{value}</p>
    </div>
  );
}

function ActionLink({
  view,
  icon: Icon,
  title,
  description,
}: {
  view: BillingView;
  icon: typeof Wallet;
  title: string;
  description: string;
}) {
  return (
    <Button asChild variant='outline' className='h-auto min-h-24 items-start justify-start p-4 text-left whitespace-normal'>
      <Link to='/billing' search={{ view }}>
        <Icon className='mt-0.5 size-4 shrink-0' />
        <span>
          <span className='block font-medium'>{title}</span>
          <span className='text-muted-foreground mt-1 block text-xs font-normal'>{description}</span>
        </span>
      </Link>
    </Button>
  );
}
