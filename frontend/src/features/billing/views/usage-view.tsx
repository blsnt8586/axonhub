import { Link } from '@tanstack/react-router';
import { BarChart3 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { BillingSection, EmptyTableRow, StateBadge } from '../shared';
import { formatDate, microsToAmount, type BillingViewProps } from '../utils';

export function UsageChargesView({ data, isLoading, formatCurrency }: BillingViewProps) {
  const { t } = useTranslation();
  return (
    <div className='space-y-4' data-testid='billing-usage-view'>
      <div className='flex justify-end'>
        <Button asChild variant='outline'>
          <Link to='/project/usage-stats'>
            <BarChart3 className='size-4' />
            {t('billing.usage.openAnalytics')}
          </Link>
        </Button>
      </div>
      <BillingSection title={t('billing.usage.title')} description={t('billing.usage.description')}>
        <div className='overflow-x-auto border'>
          <Table className='min-w-[760px]'>
            <TableHeader>
              <TableRow>
                <TableHead>{t('billing.columns.time')}</TableHead>
                <TableHead>{t('billing.columns.model')}</TableHead>
                <TableHead>{t('billing.usage.requestType')}</TableHead>
                <TableHead>{t('billing.columns.status')}</TableHead>
                <TableHead className='text-right'>{t('billing.columns.amount')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(data?.usageBillingRecords.length ?? 0) === 0 && (
                <EmptyTableRow colSpan={5} loading={isLoading} loadingLabel={t('common.loading')} emptyLabel={t('common.noData')} />
              )}
              {data?.usageBillingRecords.map((record) => (
                <TableRow key={record.id}>
                  <TableCell>{formatDate(record.createdAt)}</TableCell>
                  <TableCell className='max-w-64 truncate font-mono text-xs'>{record.modelID}</TableCell>
                  <TableCell>{record.requestType}</TableCell>
                  <TableCell>
                    <StateBadge status={record.status} success={record.status === 'charged'} />
                  </TableCell>
                  <TableCell className='text-right font-mono'>{formatCurrency.format(microsToAmount(record.chargeAmountMicros))}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </BillingSection>
    </div>
  );
}
