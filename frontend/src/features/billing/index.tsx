import { FormEvent, useMemo, useState } from 'react';
import { AlertCircle, CreditCard, ExternalLink, Loader2, RefreshCw, Wallet } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { useCreateMyEPayRechargeCheckout, useMyBillingOverview } from './data/billing';

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

export default function BillingPage() {
  const { t, i18n } = useTranslation();
  const [amount, setAmount] = useState('20.00');
  const { data, isLoading, isFetching, error, refetch } = useMyBillingOverview(10);
  const createCheckout = useCreateMyEPayRechargeCheckout();

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
                <Button className='w-full' type='submit' disabled={createCheckout.isPending}>
                  {createCheckout.isPending ? <Loader2 className='size-4 animate-spin' /> : <ExternalLink className='size-4' />}
                  {t('billing.recharge.submit')}
                </Button>
              </form>
            </CardContent>
          </Card>

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
