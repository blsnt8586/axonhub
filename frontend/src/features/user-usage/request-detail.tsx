import { Link, useParams, useSearch } from '@tanstack/react-router';
import { ArrowLeft, Copy } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useSelectedProjectId } from '@/stores/projectStore';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useUserRequest, type UserUsageScope } from './data';
import { StatusBadge } from './shared';
import { formatDate, formatLatency, formatMoney } from './utils';

export default function UserRequestDetailPage() {
  const { t } = useTranslation();
  const projectId = useSelectedProjectId();
  const { requestId } = useParams({ from: '/_authenticated/project/requests/$requestId' });
  const search = useSearch({ from: '/_authenticated/project/requests/$requestId' });
  const scope: UserUsageScope = search.scope === 'project' ? 'project' : 'mine';
  const request = useUserRequest(projectId, requestId, scope);

  if (request.isLoading) {
    return <div className='space-y-4 p-4 md:p-6'><Skeleton className='h-10 w-64' /><Skeleton className='h-48 w-full' /><Skeleton className='h-72 w-full' /></div>;
  }

  if (request.isError || !request.data) {
    return (
      <div className='space-y-4 p-4 md:p-6'>
        <Button asChild variant='ghost'><Link to='/project/requests' search={{}}><ArrowLeft className='h-4 w-4' />{t('userUsage.detail.back')}</Link></Button>
        <Alert variant='destructive'>
          <AlertTitle>{t('userUsage.detail.loadFailed')}</AlertTitle>
          <AlertDescription>{request.error instanceof Error ? request.error.message : t('userUsage.detail.notFound')}</AlertDescription>
        </Alert>
      </div>
    );
  }

  const item = request.data;
  return (
    <div className='flex flex-1 flex-col gap-6 p-4 md:p-6' data-testid='user-request-detail'>
      <header className='flex flex-wrap items-start justify-between gap-4'>
        <div className='min-w-0'>
          <Button asChild variant='ghost' className='mb-2 -ml-3'>
            <Link to='/project/requests' search={{}}><ArrowLeft className='h-4 w-4' />{t('userUsage.detail.back')}</Link>
          </Button>
          <h1 className='text-2xl font-semibold'>{t('userUsage.detail.title')}</h1>
          <code className='text-muted-foreground mt-1 block break-all text-xs'>{item.id}</code>
        </div>
        <StatusBadge status={item.status} />
      </header>

      <section className='grid gap-x-8 gap-y-5 border-y py-5 sm:grid-cols-2 lg:grid-cols-4'>
        <Detail label={t('userUsage.detail.fields.time')} value={formatDate(item.createdAt)} />
        <Detail label={t('userUsage.detail.fields.model')} value={item.modelId || '-'} />
        <Detail label={t('userUsage.detail.fields.key')} value={item.apiKey?.name || '-'} />
        <Detail label={t('userUsage.detail.fields.source')} value={item.source || '-'} />
        <Detail label={t('userUsage.detail.fields.format')} value={item.format || '-'} />
        <Detail label={t('userUsage.detail.fields.mode')} value={item.stream ? t('userUsage.detail.streaming') : t('userUsage.detail.nonStreaming')} />
        <Detail label={t('userUsage.detail.fields.tokens')} value={item.totalTokens.toLocaleString()} />
        <Detail label={t('userUsage.detail.fields.latency')} value={formatLatency(item.latencyMs)} />
      </section>

      <section className='grid min-w-0 gap-6 xl:grid-cols-2'>
        <JSONPanel title={t('userUsage.detail.requestBody')} value={item.requestBody} />
        <JSONPanel title={t('userUsage.detail.responseBody')} value={item.responseBody} />
      </section>

      <section className='space-y-4' data-testid='user-request-billing'>
        <div>
          <h2 className='text-lg font-semibold'>{t('userUsage.detail.billing.title')}</h2>
          <p className='text-muted-foreground mt-1 text-sm'>{t('userUsage.detail.billing.description')}</p>
        </div>
        {item.billing ? (
          <div className='grid gap-x-8 gap-y-5 border p-4 sm:grid-cols-2 lg:grid-cols-3'>
            <Detail label={t('userUsage.detail.billing.charge')} value={formatMoney(item.billing.chargeAmountMicros, item.billing.currency)} />
            <Detail label={t('userUsage.detail.billing.status')} value={<Badge variant='secondary'>{item.billing.status}</Badge>} />
            <Detail label={t('userUsage.detail.billing.priceReference')} value={item.billing.priceReferenceId || '-'} mono />
            <Detail label={t('userUsage.detail.billing.record')} value={item.billing.billingRecordId || '-'} mono />
            <Detail label={t('userUsage.detail.billing.ledger')} value={item.billing.ledgerTransactionId || '-'} mono />
            <Detail label={t('userUsage.detail.billing.items')} value={String(item.billing.chargeItems?.length || 0)} />
            <div className='min-w-0 sm:col-span-2 lg:col-span-3'>
              <JSONPanel title={t('userUsage.detail.billing.priceSnapshot')} value={item.billing.priceSnapshot} compact />
            </div>
          </div>
        ) : (
          <div className='text-muted-foreground border border-dashed p-6 text-sm'>{t('userUsage.detail.billing.empty')}</div>
        )}
      </section>
    </div>
  );
}

function Detail({ label, value, mono = false }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className='min-w-0'>
      <dt className='text-muted-foreground text-xs font-medium'>{label}</dt>
      <dd className={`mt-1 break-words text-sm ${mono ? 'font-mono text-xs' : 'font-medium'}`}>{value}</dd>
    </div>
  );
}

function JSONPanel({ title, value, compact = false }: { title: string; value: unknown; compact?: boolean }) {
  const { t } = useTranslation();
  const content = value === undefined || value === null ? '' : JSON.stringify(value, null, 2);
  const copy = async () => {
    await navigator.clipboard.writeText(content);
    toast.success(t('userUsage.detail.copied'));
  };
  return (
    <div className='min-w-0 border' data-testid={`user-request-json-${title}`}>
      <div className='bg-muted/40 flex h-11 items-center justify-between border-b px-3'>
        <h2 className='text-sm font-medium'>{title}</h2>
        <Button size='icon' variant='ghost' onClick={copy} disabled={!content} title={t('userUsage.detail.copy')}>
          <Copy className='h-4 w-4' /><span className='sr-only'>{t('userUsage.detail.copy')}</span>
        </Button>
      </div>
      <pre className={`overflow-auto p-4 font-mono text-xs leading-5 ${compact ? 'max-h-64' : 'min-h-64 max-h-[34rem]'}`}>
        {content || t('userUsage.detail.emptyBody')}
      </pre>
    </div>
  );
}
