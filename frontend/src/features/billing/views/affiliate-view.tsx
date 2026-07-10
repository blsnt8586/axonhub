import { type FormEvent, useState } from 'react';
import { Copy, Loader2, Users, Wallet } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useBindAffiliateInvite, useTransferAffiliateRebates } from '../data/billing';
import { BillingSection, EmptyTableRow, StateBadge } from '../shared';
import { formatDate, microsToAmount, type BillingViewProps } from '../utils';

export function AffiliateView({ data, isLoading, formatCurrency }: BillingViewProps) {
  const { t } = useTranslation();
  const [inviteCode, setInviteCode] = useState('');
  const bind = useBindAffiliateInvite();
  const transfer = useTransferAffiliateRebates();
  const summary = data?.affiliateSummary;

  const bindInvite = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!inviteCode.trim()) return toast.error(t('billing.affiliate.invalidInviteCode'));
    try {
      await bind.mutateAsync({ inviteCode: inviteCode.trim() });
      setInviteCode('');
      toast.success(t('billing.affiliate.bindSuccess'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('common.errors.unknownError'));
    }
  };

  const transferRebates = async () => {
    try {
      const result = await transfer.mutateAsync();
      toast.success(
        t('billing.affiliate.transferSuccess', {
          count: result.transferredCount,
          amount: formatCurrency.format(microsToAmount(result.transferredMicros)),
        })
      );
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('common.errors.unknownError'));
    }
  };

  return (
    <div className='space-y-6' data-testid='billing-affiliate-view'>
      <BillingSection title={t('billing.affiliate.title')} description={t('billing.affiliate.description')}>
        <div className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(280px,420px)]'>
          <div className='space-y-4'>
            <div className='border p-4'>
              <p className='text-muted-foreground text-xs'>{t('billing.affiliate.myInviteCode')}</p>
              <div className='mt-2 flex items-center justify-between gap-3'>
                <code className='truncate text-lg font-semibold'>{summary?.profile.inviteCode || '-'}</code>
                <Button
                  size='icon'
                  variant='outline'
                  title={t('billing.affiliate.copy')}
                  onClick={() => {
                    const value = summary?.profile.inviteCode;
                    if (value) {
                      void navigator.clipboard.writeText(value);
                      toast.success(t('billing.affiliate.copied'));
                    }
                  }}
                >
                  <Copy className='size-4' />
                </Button>
              </div>
            </div>
            <div className='grid gap-3 sm:grid-cols-3'>
              <Metric label={t('billing.affiliate.invitees')} value={String(summary?.inviteeCount ?? 0)} />
              <Metric label={t('billing.affiliate.frozen')} value={formatCurrency.format(microsToAmount(summary?.frozenMicros ?? 0))} />
              <Metric
                label={t('billing.affiliate.available')}
                value={formatCurrency.format(microsToAmount(summary?.availableMicros ?? 0))}
              />
            </div>
          </div>
          <div className='space-y-4'>
            {!summary?.invitation && (
              <form className='space-y-3 border p-4' onSubmit={bindInvite}>
                <label className='text-sm font-medium' htmlFor='billing-affiliate-invite'>
                  {t('billing.affiliate.inviteCode')}
                </label>
                <Input
                  id='billing-affiliate-invite'
                  value={inviteCode}
                  onChange={(event) => setInviteCode(event.target.value.toUpperCase())}
                  placeholder={t('billing.affiliate.invitePlaceholder')}
                />
                <Button className='w-full' type='submit' variant='outline' disabled={bind.isPending}>
                  {bind.isPending ? <Loader2 className='size-4 animate-spin' /> : <Users className='size-4' />}
                  {t('billing.affiliate.bind')}
                </Button>
              </form>
            )}
            <Button className='w-full' onClick={transferRebates} disabled={transfer.isPending || (summary?.availableMicros ?? 0) <= 0}>
              {transfer.isPending ? <Loader2 className='size-4 animate-spin' /> : <Wallet className='size-4' />}
              {t('billing.affiliate.transfer')}
            </Button>
          </div>
        </div>
      </BillingSection>

      <div className='grid gap-6 xl:grid-cols-2'>
        <BillingSection title={t('billing.affiliate.inviteesTitle')} description={t('billing.affiliate.inviteesDescription')}>
          <div className='overflow-x-auto border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('billing.columns.time')}</TableHead>
                  <TableHead>{t('billing.affiliate.invitee')}</TableHead>
                  <TableHead>{t('billing.columns.status')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(data?.affiliateInvitations.length ?? 0) === 0 && (
                  <EmptyTableRow colSpan={3} loading={isLoading} loadingLabel={t('common.loading')} emptyLabel={t('common.noData')} />
                )}
                {data?.affiliateInvitations.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>{formatDate(item.createdAt)}</TableCell>
                    <TableCell className='font-mono text-xs'>{item.inviteeUserID}</TableCell>
                    <TableCell>
                      <StateBadge status={item.status} success={item.status === 'active'} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </BillingSection>
        <BillingSection title={t('billing.affiliate.rebatesTitle')} description={t('billing.affiliate.rebatesDescription')}>
          <div className='overflow-x-auto border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('billing.columns.time')}</TableHead>
                  <TableHead>{t('billing.columns.status')}</TableHead>
                  <TableHead className='text-right'>{t('billing.columns.amount')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(data?.affiliateRebates.length ?? 0) === 0 && (
                  <EmptyTableRow colSpan={3} loading={isLoading} loadingLabel={t('common.loading')} emptyLabel={t('common.noData')} />
                )}
                {data?.affiliateRebates.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>{formatDate(item.createdAt)}</TableCell>
                    <TableCell>
                      <StateBadge status={item.status} success={item.status === 'transferred'} />
                    </TableCell>
                    <TableCell className='text-right font-mono'>{formatCurrency.format(microsToAmount(item.amountMicros))}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </BillingSection>
      </div>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className='border p-3'>
      <p className='text-muted-foreground text-xs'>{label}</p>
      <p className='mt-2 font-mono text-lg font-semibold break-words'>{value}</p>
    </div>
  );
}
