import { useMemo } from 'react';
import { createFileRoute } from '@tanstack/react-router';
import { AlertCircle, ArrowLeft, Loader2, RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { PaymentOrder, useMyBillingOverview } from '@/features/billing/data/billing';

export const Route = createFileRoute('/_authenticated/billing/payment-return')({
  component: PaymentReturnPage,
});

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

function statusVariant(status?: PaymentOrder['status']) {
  if (status === 'paid') return 'default';
  if (status === 'failed' || status === 'canceled' || status === 'expired') return 'destructive';
  return 'secondary';
}

function PaymentReturnPage() {
  const { t, i18n } = useTranslation();
  const query = useMemo(() => new URLSearchParams(window.location.search), []);
  const orderNo = query.get('out_trade_no') || query.get('order_no') || '';
  const tradeNo = query.get('trade_no') || '';
  const tradeStatus = query.get('trade_status') || query.get('status') || '';
  const returnedAmount = query.get('money') || query.get('amount') || '';
  const { data, isLoading, isFetching, error, refetch } = useMyBillingOverview(50);

  const order = data?.paymentOrders.find((item) => item.orderNo === orderNo);
  const currency = order?.currency || data?.account.currency || 'CNY';
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';
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

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between gap-4'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('billing.paymentReturn.title')}</h2>
            <p className='text-muted-foreground text-sm'>{t('billing.paymentReturn.description')}</p>
          </div>
          <div className='flex items-center gap-2'>
            <Button variant='outline' size='sm' asChild>
              <a href='/billing'>
                <ArrowLeft className='size-4' />
                {t('billing.paymentReturn.back')}
              </a>
            </Button>
            <Button variant='outline' size='sm' onClick={() => void refetch()} disabled={isFetching}>
              {isFetching ? <Loader2 className='size-4 animate-spin' /> : <RefreshCw className='size-4' />}
              {t('common.refresh')}
            </Button>
          </div>
        </div>
      </Header>

      <Main fixed className='grid gap-4 overflow-auto lg:grid-cols-[360px_1fr]'>
        <div className='space-y-4'>
          <Alert>
            <AlertCircle className='size-4' />
            <AlertTitle>{t('billing.paymentReturn.orderStatus')}</AlertTitle>
            <AlertDescription>{t('billing.paymentReturn.readOnly')}</AlertDescription>
          </Alert>

          <Card className='rounded-lg'>
            <CardHeader>
              <CardTitle className='text-base'>{t('billing.paymentReturn.params')}</CardTitle>
              <CardDescription>{orderNo || '-'}</CardDescription>
            </CardHeader>
            <CardContent className='space-y-3 text-sm'>
              <div className='flex items-center justify-between gap-3'>
                <span className='text-muted-foreground'>{t('billing.columns.orderNo')}</span>
                <span className='max-w-[200px] truncate font-mono text-xs'>{orderNo || '-'}</span>
              </div>
              <div className='flex items-center justify-between gap-3'>
                <span className='text-muted-foreground'>{t('billing.paymentReturn.tradeNo')}</span>
                <span className='max-w-[200px] truncate font-mono text-xs'>{tradeNo || '-'}</span>
              </div>
              <div className='flex items-center justify-between gap-3'>
                <span className='text-muted-foreground'>{t('billing.paymentReturn.tradeStatus')}</span>
                <span className='max-w-[200px] truncate font-mono text-xs'>{tradeStatus || '-'}</span>
              </div>
              <div className='flex items-center justify-between gap-3'>
                <span className='text-muted-foreground'>{t('billing.paymentReturn.returnAmount')}</span>
                <span className='font-mono text-xs'>{returnedAmount || '-'}</span>
              </div>
            </CardContent>
          </Card>
        </div>

        <Card className='rounded-lg'>
          <CardHeader>
            <CardTitle className='text-base'>{t('billing.paymentReturn.orderStatus')}</CardTitle>
            <CardDescription>{order ? order.orderNo : t('billing.paymentReturn.orderNotFound')}</CardDescription>
          </CardHeader>
          <CardContent className='overflow-auto'>
            {error && (
              <Alert variant='destructive' className='mb-4'>
                <AlertCircle className='size-4' />
                <AlertTitle>{t('common.loadError')}</AlertTitle>
                <AlertDescription>{error instanceof Error ? error.message : t('common.errors.unknownError')}</AlertDescription>
              </Alert>
            )}
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('billing.columns.time')}</TableHead>
                  <TableHead>{t('billing.columns.status')}</TableHead>
                  <TableHead>{t('billing.columns.amount')}</TableHead>
                  <TableHead>{t('adminBilling.columns.tradeNo')}</TableHead>
                  <TableHead>{t('adminBilling.columns.expiresAt')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading ? (
                  <TableRow>
                    <TableCell colSpan={5} className='text-muted-foreground h-24 text-center'>
                      {t('common.loading')}
                    </TableCell>
                  </TableRow>
                ) : !order ? (
                  <TableRow>
                    <TableCell colSpan={5} className='text-muted-foreground h-24 text-center'>
                      {t('billing.paymentReturn.orderNotFound')}
                    </TableCell>
                  </TableRow>
                ) : (
                  <TableRow>
                    <TableCell>{formatDate(order.createdAt)}</TableCell>
                    <TableCell>
                      <Badge variant={statusVariant(order.status)}>{order.status}</Badge>
                    </TableCell>
                    <TableCell className='font-mono'>{formatCurrency.format(microsToAmount(order.amountMicros))}</TableCell>
                    <TableCell className='font-mono text-xs'>{order.externalTradeNo || tradeNo || '-'}</TableCell>
                    <TableCell>{formatDate(order.expiresAt)}</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </Main>
    </div>
  );
}
