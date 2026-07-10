import { useTranslation } from 'react-i18next';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { BillingSection, EmptyTableRow } from '../shared';
import { formatDate, microsToAmount, type BillingViewProps } from '../utils';

export function LedgerView({ data, isLoading, formatCurrency }: BillingViewProps) {
  const { t } = useTranslation();
  return (
    <BillingSection title={t('billing.ledger.title')} description={t('billing.ledger.description')} testId='billing-ledger-view'>
      <div className='overflow-x-auto border'>
        <Table className='min-w-[760px]'>
          <TableHeader>
            <TableRow>
              <TableHead>{t('billing.columns.time')}</TableHead>
              <TableHead>{t('billing.columns.type')}</TableHead>
              <TableHead>{t('billing.columns.status')}</TableHead>
              <TableHead>{t('billing.ledger.memo')}</TableHead>
              <TableHead className='text-right'>{t('billing.columns.amount')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(data?.ledgerTransactions.length ?? 0) === 0 && (
              <EmptyTableRow colSpan={5} loading={isLoading} loadingLabel={t('common.loading')} emptyLabel={t('common.noData')} />
            )}
            {data?.ledgerTransactions.map((transaction) => (
              <TableRow key={transaction.id}>
                <TableCell>{formatDate(transaction.createdAt)}</TableCell>
                <TableCell>{transaction.type}</TableCell>
                <TableCell>{transaction.status}</TableCell>
                <TableCell className='max-w-72 truncate'>{transaction.memo || '-'}</TableCell>
                <TableCell className='text-right font-mono'>
                  {transaction.direction === 'debit' ? '-' : '+'}
                  {formatCurrency.format(microsToAmount(transaction.amountMicros))}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </BillingSection>
  );
}
