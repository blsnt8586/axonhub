import { useState } from 'react';
import { Loader2, ShieldCheck } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Progress } from '@/components/ui/progress';
import { usePurchaseSubscriptionPlan, useQuoteSubscriptionPromo, type SubscriptionPlan } from '../data/billing';
import { BillingSection } from '../shared';
import { formatDate, microsToAmount, usagePercent, type BillingViewProps } from '../utils';

export function SubscriptionsView({ data, isLoading, formatCurrency }: BillingViewProps) {
  const { t } = useTranslation();
  const [promoCode, setPromoCode] = useState('');
  const purchase = usePurchaseSubscriptionPlan();
  const quotePromo = useQuoteSubscriptionPromo();

  const purchasePlan = async (plan: SubscriptionPlan) => {
    try {
      const quote = promoCode.trim() ? await quotePromo.mutateAsync({ planId: plan.id, promoCode: promoCode.trim() }) : null;
      const original = quote?.originalAmountMicros ?? plan.priceMicros;
      const discount = quote?.discountAmountMicros ?? 0;
      const payable = quote?.payableAmountMicros ?? plan.priceMicros;
      if (
        !window.confirm(
          t('billing.subscriptions.purchaseConfirm', {
            name: plan.name,
            amount: formatCurrency.format(microsToAmount(payable)),
            original: formatCurrency.format(microsToAmount(original)),
            discount: formatCurrency.format(microsToAmount(discount)),
          })
        )
      )
        return;
      await purchase.mutateAsync({ planId: plan.id, promoCode: promoCode.trim() || undefined });
      toast.success(t('billing.subscriptions.purchaseSuccess'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('common.errors.unknownError'));
    }
  };

  return (
    <div className='space-y-6' data-testid='billing-subscriptions-view'>
      <BillingSection title={t('billing.subscriptions.plansTitle')} description={t('billing.subscriptions.plansDescription')}>
        <div className='mb-5 max-w-sm space-y-2'>
          <label className='text-sm font-medium' htmlFor='billing-subscription-promo'>
            {t('billing.promo.subscriptionCode')}
          </label>
          <Input
            id='billing-subscription-promo'
            value={promoCode}
            onChange={(event) => setPromoCode(event.target.value.toUpperCase())}
            placeholder={t('billing.promo.placeholder')}
          />
        </div>
        <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
          {data?.availableSubscriptionPlans.map((plan) => (
            <article key={plan.id} className='flex min-h-64 flex-col border p-4'>
              <div className='flex items-start justify-between gap-3'>
                <div className='min-w-0'>
                  <h3 className='truncate font-semibold'>{plan.name}</h3>
                  <p className='text-muted-foreground mt-1 line-clamp-2 text-sm'>
                    {plan.description || t('billing.subscriptions.noDescription')}
                  </p>
                </div>
                <Badge variant='secondary'>{plan.period}</Badge>
              </div>
              <p className='mt-5 font-mono text-2xl font-semibold'>{formatCurrency.format(microsToAmount(plan.priceMicros))}</p>
              <dl className='mt-4 space-y-2 text-sm'>
                <Row label={t('billing.subscriptions.periodDays', { days: plan.periodDays })} value='' />
                <Row
                  label={t('billing.subscriptions.included')}
                  value={
                    plan.includedAmountMicros > 0
                      ? formatCurrency.format(microsToAmount(plan.includedAmountMicros))
                      : t('billing.subscriptions.unlimited')
                  }
                />
                <Row
                  label={t('billing.subscriptions.scope')}
                  value={plan.supportedModelIds.length ? plan.supportedModelIds.join(', ') : t('billing.subscriptions.allModels')}
                />
              </dl>
              <Button className='mt-auto' onClick={() => purchasePlan(plan)} disabled={purchase.isPending || quotePromo.isPending}>
                {purchase.isPending || quotePromo.isPending ? (
                  <Loader2 className='size-4 animate-spin' />
                ) : (
                  <ShieldCheck className='size-4' />
                )}
                {t('billing.subscriptions.purchase')}
              </Button>
            </article>
          ))}
          {!isLoading && (data?.availableSubscriptionPlans.length ?? 0) === 0 && (
            <p className='text-muted-foreground border border-dashed p-8 text-center text-sm md:col-span-2 xl:col-span-3'>
              {t('common.noData')}
            </p>
          )}
        </div>
      </BillingSection>

      <BillingSection title={t('billing.subscriptions.activeTitle')} description={t('billing.subscriptions.activeDescription')}>
        <div className='grid gap-4 lg:grid-cols-2'>
          {data?.userSubscriptions.map((subscription) => (
            <article key={subscription.id} className='border p-4'>
              <div className='flex items-start justify-between gap-3'>
                <div>
                  <h3 className='font-medium'>{subscription.plan?.name || t('billing.subscriptions.planSnapshot')}</h3>
                  <p className='text-muted-foreground mt-1 text-xs'>
                    {formatDate(subscription.startsAt)} - {formatDate(subscription.expiresAt)}
                  </p>
                </div>
                <Badge variant={subscription.status === 'active' ? 'default' : 'secondary'}>{subscription.status}</Badge>
              </div>
              <div className='mt-5 flex justify-between gap-3 text-sm'>
                <span className='text-muted-foreground'>{t('billing.subscriptions.used')}</span>
                <span className='font-mono'>
                  {formatCurrency.format(microsToAmount(subscription.usedAmountMicros))} /{' '}
                  {subscription.includedAmountMicros > 0
                    ? formatCurrency.format(microsToAmount(subscription.includedAmountMicros))
                    : t('billing.subscriptions.unlimited')}
                </span>
              </div>
              <Progress className='mt-2' value={usagePercent(subscription)} />
              <p className='text-muted-foreground mt-3 text-xs'>
                {t('billing.subscriptions.resetAt')}: {formatDate(subscription.resetAt)}
              </p>
            </article>
          ))}
          {!isLoading && (data?.userSubscriptions.length ?? 0) === 0 && (
            <p className='text-muted-foreground border border-dashed p-8 text-center text-sm lg:col-span-2'>{t('common.noData')}</p>
          )}
        </div>
      </BillingSection>
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className='flex justify-between gap-3'>
      <dt className='text-muted-foreground'>{label}</dt>
      <dd className='max-w-44 truncate text-right font-mono text-xs'>{value}</dd>
    </div>
  );
}
