import { useTranslation } from 'react-i18next';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { BillingSection, EmptyTableRow, StateBadge } from '../shared';
import { formatDate, microsToAmount, type BillingViewProps } from '../utils';

export function OrdersView({ data, isLoading, formatCurrency }: BillingViewProps) {
  const { t } = useTranslation();
  return (
    <BillingSection title={t('billing.orders.title')} description={t('billing.orders.description')} testId='billing-orders-view'>
      <div className='overflow-x-auto border'>
        <Table className='min-w-[760px]'>
          <TableHeader>
            <TableRow>
              <TableHead>{t('billing.columns.time')}</TableHead>
              <TableHead>{t('billing.columns.orderNo')}</TableHead>
              <TableHead>{t('billing.columns.status')}</TableHead>
              <TableHead>{t('billing.orders.provider')}</TableHead>
              <TableHead className='text-right'>{t('billing.columns.amount')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(data?.paymentOrders.length ?? 0) === 0 && (
              <EmptyTableRow colSpan={5} loading={isLoading} loadingLabel={t('common.loading')} emptyLabel={t('common.noData')} />
            )}
            {data?.paymentOrders.map((order) => (
              <TableRow key={order.id}>
                <TableCell>{formatDate(order.createdAt)}</TableCell>
                <TableCell className='font-mono text-xs'>{order.orderNo}</TableCell>
                <TableCell>
                  <StateBadge status={order.status} success={order.status === 'paid'} />
                </TableCell>
                <TableCell>{order.providerType}</TableCell>
                <TableCell className='text-right font-mono'>
                  {formatCurrency.format(microsToAmount(order.payableAmountMicros || order.amountMicros))}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </BillingSection>
  );
}
