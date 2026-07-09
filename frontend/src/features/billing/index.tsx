import { FormEvent, useMemo, useState } from 'react';
import { AlertCircle, CreditCard, ExternalLink, Loader2, PackageCheck, RefreshCw, ShieldCheck, Ticket, Wallet } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Progress } from '@/components/ui/progress';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import {
  type PromoQuote,
  type SubscriptionPlan,
  type UserSubscription,
  useCreateMyEPayRechargeCheckout,
  useMyBillingOverview,
  usePurchaseSubscriptionPlan,
  useQuoteRechargePromo,
  useQuoteSubscriptionPromo,
  useRedeemCode,
} from './data/billing';

function microsToAmount(value: number) {
  return value / 1_000_000;
}

function formatDate(value?: string | null) {
  if (!value) {
    return '-';
  }
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

function normalizeAmount(value: string) {
  const trimmed = value.trim();
  if (!/^\d+(\.\d{1,2})?$/.test(trimmed)) {
    return '';
  }
  const amount = Number(trimmed);
  if (!Number.isFinite(amount) || amount <= 0) {
    return '';
  }
  return amount.toFixed(2);
}

function usagePercent(subscription: UserSubscription) {
  if (subscription.includedAmountMicros <= 0) {
    return 0;
  }
  return Math.min(100, Math.round((subscription.usedAmountMicros / subscription.includedAmountMicros) * 100));
}

function PromoQuoteSummary({
  quote,
  fallbackMicros,
  formatCurrency,
}: {
  quote: PromoQuote | null;
  fallbackMicros: number;
  currency: string;
  formatCurrency: Intl.NumberFormat;
}) {
  const { t } = useTranslation();
  const original = quote?.originalAmountMicros ?? fallbackMicros;
  const discount = quote?.discountAmountMicros ?? 0;
  const payable = quote?.payableAmountMicros ?? original;

  return (
    <div className='rounded-md border p-3 text-sm'>
      <div className='flex justify-between gap-3'>
        <span className='text-muted-foreground'>{t('billing.promo.original')}</span>
        <span className='font-mono'>{formatCurrency.format(microsToAmount(original))}</span>
      </div>
      <div className='mt-1 flex justify-between gap-3'>
        <span className='text-muted-foreground'>{t('billing.promo.discount')}</span>
        <span className='font-mono'>-{formatCurrency.format(microsToAmount(discount))}</span>
      </div>
      <div className='mt-2 flex justify-between gap-3 border-t pt-2 font-medium'>
        <span>{t('billing.promo.payable')}</span>
        <span className='font-mono'>{formatCurrency.format(microsToAmount(payable))}</span>
      </div>
    </div>
  );
}

export default function BillingPage() {
  const { t, i18n } = useTranslation();
  const [amount, setAmount] = useState('20.00');
  const [rechargePromoCode, setRechargePromoCode] = useState('');
  const [rechargeQuote, setRechargeQuote] = useState<PromoQuote | null>(null);
  const [subscriptionPromoCode, setSubscriptionPromoCode] = useState('');
  const [redeemCode, setRedeemCode] = useState('');
  const { data, isLoading, isFetching, error, refetch } = useMyBillingOverview(10);
  const createCheckout = useCreateMyEPayRechargeCheckout();
  const quoteRechargePromo = useQuoteRechargePromo();
  const quoteSubscriptionPromo = useQuoteSubscriptionPromo();
  const redeemCodeMutation = useRedeemCode();
  const purchaseSubscriptionPlan = usePurchaseSubscriptionPlan();

  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';
  const currency = data?.account.currency || 'CNY';
  const formatCurrency = useMemo(
    () =>
      new Intl.NumberFormat(locale, {
        style: 'currency',
        currency,
        currencyDisplay: 'narrowSymbol',
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      }),
    [currency, locale]
  );

  const balance = data ? microsToAmount(data.account.balanceMicros) : 0;
  const held = data ? microsToAmount(data.account.heldBalanceMicros) : 0;
  const creditLimit = data ? microsToAmount(data.account.creditLimitMicros) : 0;
  const available = balance + creditLimit - held;

  async function handleRecharge(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalized = normalizeAmount(amount);
    if (!normalized) {
      toast.error(t('billing.recharge.invalidAmount'));
      return;
    }

    try {
      const checkout = await createCheckout.mutateAsync({
        amount: normalized,
        currency,
        subject: t('billing.recharge.subject'),
        promoCode: rechargePromoCode.trim() || undefined,
      });
      if (!checkout.url) {
        toast.error(t('billing.recharge.missingCheckoutUrl'));
        return;
      }
      window.location.assign(checkout.url);
    } catch (err) {
      const message = err instanceof Error ? err.message : t('common.errors.unknownError');
      toast.error(message);
    }
  }

  async function handleQuoteRecharge() {
    const normalized = normalizeAmount(amount);
    if (!normalized) {
      toast.error(t('billing.recharge.invalidAmount'));
      return;
    }
    try {
      const quote = await quoteRechargePromo.mutateAsync({ amount: normalized, currency, promoCode: rechargePromoCode.trim() || undefined });
      setRechargeQuote(quote);
    } catch (err) {
      setRechargeQuote(null);
      const message = err instanceof Error ? err.message : t('common.errors.unknownError');
      toast.error(message);
    }
  }

  async function handleRedeem(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const code = redeemCode.trim();
    if (!code) {
      toast.error(t('billing.redeem.invalidCode'));
      return;
    }

    try {
      await redeemCodeMutation.mutateAsync({ code });
      toast.success(t('billing.redeem.success'));
      setRedeemCode('');
    } catch (err) {
      const message = err instanceof Error ? err.message : t('common.errors.unknownError');
      toast.error(message);
    }
  }

  async function handlePurchasePlan(plan: SubscriptionPlan) {
    let quote: PromoQuote | null = null;
    if (subscriptionPromoCode.trim()) {
      try {
        quote = await quoteSubscriptionPromo.mutateAsync({ planId: plan.id, promoCode: subscriptionPromoCode.trim() });
      } catch (err) {
        const message = err instanceof Error ? err.message : t('common.errors.unknownError');
        toast.error(message);
        return;
      }
    }
    const original = quote?.originalAmountMicros ?? plan.priceMicros;
    const discount = quote?.discountAmountMicros ?? 0;
    const payable = quote?.payableAmountMicros ?? plan.priceMicros;
    if (!window.confirm(t('billing.subscriptions.purchaseConfirm', {
      name: plan.name,
      amount: formatCurrency.format(microsToAmount(payable)),
      original: formatCurrency.format(microsToAmount(original)),
      discount: formatCurrency.format(microsToAmount(discount)),
    }))) {
      return;
    }

    try {
      await purchaseSubscriptionPlan.mutateAsync({ planId: plan.id, promoCode: subscriptionPromoCode.trim() || undefined });
      toast.success(t('billing.subscriptions.purchaseSuccess'));
    } catch (err) {
      const message = err instanceof Error ? err.message : t('common.errors.unknownError');
      toast.error(message);
    }
  }

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between gap-4'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('billing.title')}</h2>
            <p className='text-muted-foreground text-sm'>{t('billing.description')}</p>
          </div>
          <Button variant='outline' size='sm' onClick={() => void refetch()} disabled={isFetching}>
            {isFetching ? <Loader2 className='size-4 animate-spin' /> : <RefreshCw className='size-4' />}
            {t('common.refresh')}
          </Button>
        </div>
      </Header>

      <Main fixed className='flex flex-col gap-4 overflow-auto'>
        {error && (
          <Alert variant='destructive'>
            <AlertCircle className='size-4' />
            <AlertTitle>{t('common.loadError')}</AlertTitle>
            <AlertDescription>{error instanceof Error ? error.message : t('common.errors.unknownError')}</AlertDescription>
          </Alert>
        )}

        <div className='grid gap-4 md:grid-cols-5'>
          <Card className='rounded-lg'>
            <CardHeader className='pb-2'>
              <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
                <Wallet className='size-4' />
                {t('billing.cards.balance')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='font-mono text-2xl font-semibold'>{isLoading ? '-' : formatCurrency.format(balance)}</div>
              <p className='text-muted-foreground mt-1 text-xs'>{t('billing.cards.balanceHint')}</p>
            </CardContent>
          </Card>

          <Card className='rounded-lg'>
            <CardHeader className='pb-2'>
              <CardTitle className='text-muted-foreground text-sm font-medium'>{t('billing.cards.available')}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className='font-mono text-2xl font-semibold'>{isLoading ? '-' : formatCurrency.format(available)}</div>
              <p className='text-muted-foreground mt-1 text-xs'>{t('billing.cards.availableHint')}</p>
            </CardContent>
          </Card>

          <Card className='rounded-lg'>
            <CardHeader className='pb-2'>
              <CardTitle className='text-muted-foreground text-sm font-medium'>{t('billing.cards.held')}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className='font-mono text-2xl font-semibold'>{isLoading ? '-' : formatCurrency.format(held)}</div>
              <p className='text-muted-foreground mt-1 text-xs'>{t('billing.cards.heldHint')}</p>
            </CardContent>
          </Card>

          <Card className='rounded-lg'>
            <CardHeader className='pb-2'>
              <CardTitle className='text-muted-foreground text-sm font-medium'>{t('billing.cards.credit')}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className='font-mono text-2xl font-semibold'>{isLoading ? '-' : formatCurrency.format(creditLimit)}</div>
              <p className='text-muted-foreground mt-1 text-xs'>{t('billing.cards.creditHint')}</p>
            </CardContent>
          </Card>

          <Card className='rounded-lg'>
            <CardHeader className='pb-2'>
              <CardTitle className='text-muted-foreground text-sm font-medium'>{t('billing.cards.status')}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className='flex items-center gap-2'>
                <Badge variant={data?.account.status === 'active' ? 'default' : 'destructive'}>{data?.account.status || '-'}</Badge>
                <span className='text-muted-foreground text-sm'>{data?.account.ownerType || '-'}</span>
              </div>
              <p className='text-muted-foreground mt-2 text-xs'>{t('billing.cards.statusHint')}</p>
            </CardContent>
          </Card>
        </div>

        <div className='grid gap-4 lg:grid-cols-[360px_1fr]'>
          <div className='space-y-4'>
            <Card className='rounded-lg'>
              <CardHeader>
                <CardTitle className='flex items-center gap-2 text-base'>
                  <CreditCard className='size-4' />
                  {t('billing.recharge.title')}
                </CardTitle>
                <CardDescription>{t('billing.recharge.description')}</CardDescription>
              </CardHeader>
              <CardContent>
                <form className='space-y-4' onSubmit={handleRecharge}>
                  <div className='space-y-2'>
                    <label className='text-sm font-medium' htmlFor='billing-recharge-amount'>
                      {t('billing.recharge.amount')}
                    </label>
                    <Input
                      id='billing-recharge-amount'
                      inputMode='decimal'
                      value={amount}
                      onChange={(event) => setAmount(event.target.value)}
                      placeholder='20.00'
                      aria-invalid={amount.trim() !== '' && !normalizeAmount(amount)}
                    />
                    <p className='text-muted-foreground text-xs'>{t('billing.recharge.amountHint', { currency })}</p>
                  </div>
                  <div className='space-y-2'>
                    <label className='text-sm font-medium' htmlFor='billing-recharge-promo'>
                      {t('billing.promo.code')}
                    </label>
                    <div className='flex gap-2'>
                      <Input
                        id='billing-recharge-promo'
                        value={rechargePromoCode}
                        onChange={(event) => {
                          setRechargePromoCode(event.target.value.toUpperCase());
                          setRechargeQuote(null);
                        }}
                        placeholder={t('billing.promo.placeholder')}
                        autoComplete='off'
                      />
                      <Button type='button' variant='outline' onClick={() => void handleQuoteRecharge()} disabled={quoteRechargePromo.isPending}>
                        {quoteRechargePromo.isPending ? <Loader2 className='size-4 animate-spin' /> : <Ticket className='size-4' />}
                        {t('billing.promo.apply')}
                      </Button>
                    </div>
                  </div>
                  <PromoQuoteSummary
                    quote={rechargeQuote}
                    fallbackMicros={normalizeAmount(amount) ? Math.round(Number(normalizeAmount(amount)) * 1_000_000) : 0}
                    currency={currency}
                    formatCurrency={formatCurrency}
                  />
                  <Button className='w-full' type='submit' disabled={createCheckout.isPending}>
                    {createCheckout.isPending ? <Loader2 className='size-4 animate-spin' /> : <ExternalLink className='size-4' />}
                    {t('billing.recharge.submit')}
                  </Button>
                </form>
              </CardContent>
            </Card>

            <Card className='rounded-lg'>
              <CardHeader>
                <CardTitle className='flex items-center gap-2 text-base'>
                  <Ticket className='size-4' />
                  {t('billing.redeem.title')}
                </CardTitle>
                <CardDescription>{t('billing.redeem.description')}</CardDescription>
              </CardHeader>
              <CardContent>
                <form className='space-y-4' onSubmit={handleRedeem}>
                  <div className='space-y-2'>
                    <label className='text-sm font-medium' htmlFor='billing-redeem-code'>
                      {t('billing.redeem.code')}
                    </label>
                    <Input
                      id='billing-redeem-code'
                      value={redeemCode}
                      onChange={(event) => setRedeemCode(event.target.value)}
                      placeholder={t('billing.redeem.codePlaceholder')}
                      autoComplete='off'
                    />
                  </div>
                  <Button className='w-full' type='submit' disabled={redeemCodeMutation.isPending}>
                    {redeemCodeMutation.isPending ? <Loader2 className='size-4 animate-spin' /> : <Ticket className='size-4' />}
                    {t('billing.redeem.submit')}
                  </Button>
                </form>
              </CardContent>
            </Card>
          </div>

          <Card className='rounded-lg'>
            <CardHeader>
              <CardTitle className='text-base'>{t('billing.orders.title')}</CardTitle>
              <CardDescription>{t('billing.orders.description')}</CardDescription>
            </CardHeader>
            <CardContent className='overflow-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('billing.columns.time')}</TableHead>
                    <TableHead>{t('billing.columns.orderNo')}</TableHead>
                    <TableHead>{t('billing.columns.status')}</TableHead>
                    <TableHead className='text-right'>{t('billing.columns.amount')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(data?.paymentOrders ?? []).length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={4} className='text-muted-foreground h-24 text-center'>
                        {isLoading ? t('common.loading') : t('common.noData')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    data?.paymentOrders.map((order) => (
                      <TableRow key={order.id}>
                        <TableCell>{formatDate(order.createdAt)}</TableCell>
                        <TableCell className='font-mono text-xs'>{order.orderNo}</TableCell>
                        <TableCell>
                          <Badge variant={order.status === 'paid' ? 'default' : 'secondary'}>{order.status}</Badge>
                        </TableCell>
                        <TableCell className='text-right font-mono'>{formatCurrency.format(microsToAmount(order.amountMicros))}</TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>

        <div className='grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(360px,520px)]'>
          <Card className='rounded-lg'>
            <CardHeader>
              <CardTitle className='flex items-center gap-2 text-base'>
                <PackageCheck className='size-4' />
                {t('billing.subscriptions.plansTitle')}
              </CardTitle>
              <CardDescription>{t('billing.subscriptions.plansDescription')}</CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              <div className='grid gap-2 md:max-w-sm'>
                <label className='text-sm font-medium' htmlFor='billing-subscription-promo'>{t('billing.promo.subscriptionCode')}</label>
                <Input
                  id='billing-subscription-promo'
                  value={subscriptionPromoCode}
                  onChange={(event) => setSubscriptionPromoCode(event.target.value.toUpperCase())}
                  placeholder={t('billing.promo.placeholder')}
                  autoComplete='off'
                />
              </div>
              <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-3'>
              {(data?.availableSubscriptionPlans ?? []).length === 0 ? (
                <div className='text-muted-foreground rounded-md border p-6 text-center text-sm md:col-span-2 xl:col-span-3'>
                  {isLoading ? t('common.loading') : t('common.noData')}
                </div>
              ) : (
                data?.availableSubscriptionPlans.map((plan) => (
                  <div key={plan.id} className='flex min-h-[220px] flex-col rounded-md border p-4'>
                    <div className='flex items-start justify-between gap-3'>
                      <div className='min-w-0'>
                        <div className='truncate text-base font-semibold'>{plan.name}</div>
                        <div className='text-muted-foreground mt-1 line-clamp-2 text-sm'>{plan.description || t('billing.subscriptions.noDescription')}</div>
                      </div>
                      <Badge variant='secondary'>{plan.period}</Badge>
                    </div>
                    <div className='mt-4 font-mono text-2xl font-semibold'>{formatCurrency.format(microsToAmount(plan.priceMicros))}</div>
                    <div className='text-muted-foreground mt-1 text-xs'>
                      {t('billing.subscriptions.periodDays', { days: plan.periodDays })}
                    </div>
                    <div className='mt-4 space-y-2 text-sm'>
                      <div className='flex justify-between gap-3'>
                        <span className='text-muted-foreground'>{t('billing.subscriptions.included')}</span>
                        <span className='font-mono'>
                          {plan.includedAmountMicros > 0 ? formatCurrency.format(microsToAmount(plan.includedAmountMicros)) : t('billing.subscriptions.unlimited')}
                        </span>
                      </div>
                      <div className='flex justify-between gap-3'>
                        <span className='text-muted-foreground'>{t('billing.subscriptions.scope')}</span>
                        <span className='max-w-[150px] truncate text-right font-mono text-xs'>
                          {plan.supportedModelIds.length > 0 ? plan.supportedModelIds.join(', ') : t('billing.subscriptions.allModels')}
                        </span>
                      </div>
                    </div>
                    <Button className='mt-auto w-full' onClick={() => void handlePurchasePlan(plan)} disabled={purchaseSubscriptionPlan.isPending || quoteSubscriptionPromo.isPending}>
                      {purchaseSubscriptionPlan.isPending || quoteSubscriptionPromo.isPending ? <Loader2 className='size-4 animate-spin' /> : <ShieldCheck className='size-4' />}
                      {t('billing.subscriptions.purchase')}
                    </Button>
                  </div>
                ))
              )}
              </div>
            </CardContent>
          </Card>

          <Card className='rounded-lg'>
            <CardHeader>
              <CardTitle className='flex items-center gap-2 text-base'>
                <ShieldCheck className='size-4' />
                {t('billing.subscriptions.activeTitle')}
              </CardTitle>
              <CardDescription>{t('billing.subscriptions.activeDescription')}</CardDescription>
            </CardHeader>
            <CardContent className='space-y-3'>
              {(data?.userSubscriptions ?? []).length === 0 ? (
                <div className='text-muted-foreground rounded-md border p-6 text-center text-sm'>{isLoading ? t('common.loading') : t('common.noData')}</div>
              ) : (
                data?.userSubscriptions.map((subscription) => (
                  <div key={subscription.id} className='rounded-md border p-4'>
                    <div className='flex items-start justify-between gap-3'>
                      <div className='min-w-0'>
                        <div className='truncate font-medium'>{subscription.plan?.name || t('billing.subscriptions.planSnapshot')}</div>
                        <div className='text-muted-foreground text-xs'>{formatDate(subscription.startsAt)} - {formatDate(subscription.expiresAt)}</div>
                      </div>
                      <Badge variant={subscription.status === 'active' ? 'default' : 'secondary'}>{subscription.status}</Badge>
                    </div>
                    <div className='mt-4 space-y-2'>
                      <div className='flex justify-between text-sm'>
                        <span className='text-muted-foreground'>{t('billing.subscriptions.used')}</span>
                        <span className='font-mono'>
                          {formatCurrency.format(microsToAmount(subscription.usedAmountMicros))} /{' '}
                          {subscription.includedAmountMicros > 0 ? formatCurrency.format(microsToAmount(subscription.includedAmountMicros)) : t('billing.subscriptions.unlimited')}
                        </span>
                      </div>
                      <Progress value={usagePercent(subscription)} />
                      <div className='text-muted-foreground flex justify-between gap-3 text-xs'>
                        <span>{t('billing.subscriptions.resetAt')}: {formatDate(subscription.resetAt)}</span>
                        <span>{subscription.allowWalletFallback ? t('billing.subscriptions.walletFallbackOn') : t('billing.subscriptions.walletFallbackOff')}</span>
                      </div>
                    </div>
                  </div>
                ))
              )}
            </CardContent>
          </Card>
        </div>

        <Card className='rounded-lg'>
          <CardHeader>
            <CardTitle className='text-base'>{t('billing.redeem.historyTitle')}</CardTitle>
            <CardDescription>{t('billing.redeem.historyDescription')}</CardDescription>
          </CardHeader>
          <CardContent className='overflow-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('billing.columns.time')}</TableHead>
                  <TableHead>{t('billing.columns.code')}</TableHead>
                  <TableHead>{t('billing.columns.status')}</TableHead>
                  <TableHead>{t('billing.columns.expiresAt')}</TableHead>
                  <TableHead className='text-right'>{t('billing.columns.amount')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(data?.redeemCodes ?? []).length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className='text-muted-foreground h-24 text-center'>
                      {isLoading ? t('common.loading') : t('common.noData')}
                    </TableCell>
                  </TableRow>
                ) : (
                  data?.redeemCodes.map((code) => (
                    <TableRow key={code.id}>
                      <TableCell>{formatDate(code.usedAt || code.createdAt)}</TableCell>
                      <TableCell className='font-mono text-xs'>{code.code}</TableCell>
                      <TableCell>
                        <Badge variant={code.status === 'used' ? 'default' : 'secondary'}>{code.status}</Badge>
                      </TableCell>
                      <TableCell>{formatDate(code.expiresAt)}</TableCell>
                      <TableCell className='text-right font-mono'>{formatCurrency.format(microsToAmount(code.amountMicros))}</TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <div className='grid gap-4 lg:grid-cols-2'>
          <Card className='rounded-lg'>
            <CardHeader>
              <CardTitle className='text-base'>{t('billing.ledger.title')}</CardTitle>
              <CardDescription>{t('billing.ledger.description')}</CardDescription>
            </CardHeader>
            <CardContent className='overflow-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('billing.columns.time')}</TableHead>
                    <TableHead>{t('billing.columns.type')}</TableHead>
                    <TableHead className='text-right'>{t('billing.columns.amount')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(data?.ledgerTransactions ?? []).length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={3} className='text-muted-foreground h-24 text-center'>
                        {isLoading ? t('common.loading') : t('common.noData')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    data?.ledgerTransactions.map((tx) => (
                      <TableRow key={tx.id}>
                        <TableCell>{formatDate(tx.createdAt)}</TableCell>
                        <TableCell>{tx.type}</TableCell>
                        <TableCell className='text-right font-mono'>
                          {tx.direction === 'debit' ? '-' : '+'}
                          {formatCurrency.format(microsToAmount(tx.amountMicros))}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          <Card className='rounded-lg'>
            <CardHeader>
              <CardTitle className='text-base'>{t('billing.usage.title')}</CardTitle>
              <CardDescription>{t('billing.usage.description')}</CardDescription>
            </CardHeader>
            <CardContent className='overflow-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('billing.columns.time')}</TableHead>
                    <TableHead>{t('billing.columns.model')}</TableHead>
                    <TableHead>{t('billing.columns.status')}</TableHead>
                    <TableHead className='text-right'>{t('billing.columns.amount')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(data?.usageBillingRecords ?? []).length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={4} className='text-muted-foreground h-24 text-center'>
                        {isLoading ? t('common.loading') : t('common.noData')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    data?.usageBillingRecords.map((record) => (
                      <TableRow key={record.id}>
                        <TableCell>{formatDate(record.createdAt)}</TableCell>
                        <TableCell className='max-w-[220px] truncate font-mono text-xs'>{record.modelID}</TableCell>
                        <TableCell>
                          <Badge variant={record.status === 'charged' ? 'default' : 'secondary'}>{record.status}</Badge>
                        </TableCell>
                        <TableCell className='text-right font-mono'>
                          {formatCurrency.format(microsToAmount(record.chargeAmountMicros))}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      </Main>
    </div>
  );
}
