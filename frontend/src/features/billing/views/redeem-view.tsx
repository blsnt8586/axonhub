import { type FormEvent, useState } from 'react';
import { Loader2, Ticket } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useRedeemCode } from '../data/billing';
import { BillingSection, EmptyTableRow, StateBadge } from '../shared';
import { formatDate, microsToAmount, type BillingViewProps } from '../utils';

export function RedeemView({ data, isLoading, formatCurrency }: BillingViewProps) {
  const { t } = useTranslation();
  const [code, setCode] = useState('');
  const redeem = useRedeemCode();

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!code.trim()) return toast.error(t('billing.redeem.invalidCode'));
    try {
      await redeem.mutateAsync({ code: code.trim() });
      setCode('');
      toast.success(t('billing.redeem.success'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('common.errors.unknownError'));
    }
  };

  return (
    <div className='space-y-6' data-testid='billing-redeem-view'>
      <div className='max-w-2xl'>
        <BillingSection title={t('billing.redeem.title')} description={t('billing.redeem.description')}>
          <form className='flex flex-col gap-3 sm:flex-row' onSubmit={submit}>
            <Input
              id='billing-redeem-code'
              value={code}
              onChange={(event) => setCode(event.target.value.toUpperCase())}
              placeholder={t('billing.redeem.codePlaceholder')}
            />
            <Button type='submit' disabled={redeem.isPending}>
              {redeem.isPending ? <Loader2 className='size-4 animate-spin' /> : <Ticket className='size-4' />}
              {t('billing.redeem.submit')}
            </Button>
          </form>
        </BillingSection>
      </div>
      <BillingSection title={t('billing.redeem.historyTitle')} description={t('billing.redeem.historyDescription')}>
        <div className='overflow-x-auto border'>
          <Table className='min-w-[720px]'>
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
              {(data?.redeemCodes.length ?? 0) === 0 && (
                <EmptyTableRow colSpan={5} loading={isLoading} loadingLabel={t('common.loading')} emptyLabel={t('common.noData')} />
              )}
              {data?.redeemCodes.map((item) => (
                <TableRow key={item.id}>
                  <TableCell>{formatDate(item.usedAt || item.createdAt)}</TableCell>
                  <TableCell className='font-mono text-xs'>{item.code}</TableCell>
                  <TableCell>
                    <StateBadge status={item.status} success={item.status === 'used'} />
                  </TableCell>
                  <TableCell>{formatDate(item.expiresAt)}</TableCell>
                  <TableCell className='text-right font-mono'>{formatCurrency.format(microsToAmount(item.amountMicros))}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </BillingSection>
    </div>
  );
}
