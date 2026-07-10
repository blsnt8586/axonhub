import { useEffect, useMemo, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { ChevronLeft, ChevronRight, Download, Eye, Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useRoutePermissions } from '@/hooks/useRoutePermissions';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { downloadUserRequests, useUserRequests, type UserRequestFilters, type UserUsageScope } from './data';
import { ScopeSwitch, StatusBadge } from './shared';
import { canViewProjectUsage, formatDate, formatLatency, formatMoney } from './utils';

const PAGE_SIZE = 20;

export default function UserRequestsPage() {
  const { t } = useTranslation();
  const projectId = useSelectedProjectId();
  const { isOwner, isProjectOwner, systemScopes, projectScopes } = useRoutePermissions();
  const canViewProject = canViewProjectUsage(isOwner, isProjectOwner, systemScopes, projectScopes);
  const [scope, setScope] = useState<UserUsageScope>('mine');
  const [offset, setOffset] = useState(0);
  const [modelInput, setModelInput] = useState('');
  const [modelId, setModelId] = useState('');
  const [status, setStatus] = useState('all');
  const [fromInput, setFromInput] = useState('');
  const [toInput, setToInput] = useState('');
  const [from, setFrom] = useState<string>();
  const [to, setTo] = useState<string>();
  const [exporting, setExporting] = useState(false);

  useEffect(() => {
    if (!canViewProject && scope === 'project') setScope('mine');
  }, [canViewProject, scope]);

  useEffect(() => setOffset(0), [projectId, scope]);

  const filters = useMemo<UserRequestFilters>(
    () => ({
      projectId,
      scope,
      offset,
      limit: PAGE_SIZE,
      modelId,
      status: status === 'all' ? undefined : status,
      from,
      to,
    }),
    [from, modelId, offset, projectId, scope, status, to]
  );
  const requests = useUserRequests(filters);
  const lastPage = Math.max(1, Math.ceil((requests.data?.total || 0) / PAGE_SIZE));
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;

  const applyFilters = () => {
    try {
      const parsedFrom = fromInput ? new Date(fromInput).toISOString() : undefined;
      const parsedTo = toInput ? new Date(toInput).toISOString() : undefined;
      if (parsedFrom && parsedTo && new Date(parsedTo) <= new Date(parsedFrom)) {
        toast.error(t('userUsage.requests.filters.invalidRange'));
        return;
      }
      setModelId(modelInput.trim());
      setFrom(parsedFrom);
      setTo(parsedTo);
      setOffset(0);
    } catch {
      toast.error(t('userUsage.requests.filters.invalidRange'));
    }
  };

  const exportCSV = async () => {
    setExporting(true);
    try {
      await downloadUserRequests({ ...filters, offset: undefined, limit: undefined });
      toast.success(t('userUsage.requests.exported'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('userUsage.requests.exportFailed'));
    } finally {
      setExporting(false);
    }
  };

  return (
    <div className='flex flex-1 flex-col gap-6 p-4 md:p-6' data-testid='user-requests-page'>
      <header className='flex flex-wrap items-start justify-between gap-4'>
        <div>
          <h1 className='text-2xl font-semibold'>{t('userUsage.requests.title')}</h1>
          <p className='text-muted-foreground mt-1 text-sm'>{t('userUsage.requests.description')}</p>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <ScopeSwitch
            value={scope}
            onChange={setScope}
            canViewProject={canViewProject}
            mineLabel={t('userUsage.scope.mine')}
            projectLabel={t('userUsage.scope.project')}
          />
          <Button variant='outline' onClick={exportCSV} disabled={exporting || !projectId} data-testid='export-user-requests'>
            <Download className='h-4 w-4' />
            {t('userUsage.requests.export')}
          </Button>
        </div>
      </header>

      <section className='grid gap-3 border-y py-4 md:grid-cols-[minmax(180px,1fr)_160px_190px_190px_auto] md:items-end'>
        <div className='space-y-1.5'>
          <Label htmlFor='request-model-filter'>{t('userUsage.requests.filters.model')}</Label>
          <Input
            id='request-model-filter'
            value={modelInput}
            onChange={(event) => setModelInput(event.target.value)}
            onKeyDown={(event) => event.key === 'Enter' && applyFilters()}
          />
        </div>
        <div className='space-y-1.5'>
          <Label>{t('userUsage.requests.filters.status')}</Label>
          <Select value={status} onValueChange={(value) => { setStatus(value); setOffset(0); }}>
            <SelectTrigger data-testid='request-status-filter'><SelectValue /></SelectTrigger>
            <SelectContent>
              {['all', 'pending', 'processing', 'completed', 'failed', 'canceled'].map((value) => (
                <SelectItem key={value} value={value}>{t(`userUsage.requests.status.${value}`)}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className='space-y-1.5'>
          <Label htmlFor='request-from-filter'>{t('userUsage.requests.filters.from')}</Label>
          <Input id='request-from-filter' type='datetime-local' value={fromInput} onChange={(event) => setFromInput(event.target.value)} />
        </div>
        <div className='space-y-1.5'>
          <Label htmlFor='request-to-filter'>{t('userUsage.requests.filters.to')}</Label>
          <Input id='request-to-filter' type='datetime-local' value={toInput} onChange={(event) => setToInput(event.target.value)} />
        </div>
        <Button onClick={applyFilters} data-testid='apply-request-filters'>
          <Search className='h-4 w-4' />
          {t('userUsage.requests.filters.apply')}
        </Button>
      </section>

      {requests.isError && (
        <Alert variant='destructive'>
          <AlertTitle>{t('userUsage.requests.loadFailed')}</AlertTitle>
          <AlertDescription>{requests.error instanceof Error ? requests.error.message : t('userUsage.requests.loadFailed')}</AlertDescription>
        </Alert>
      )}

      {requests.isLoading ? (
        <div className='space-y-3'><Skeleton className='h-12 w-full' /><Skeleton className='h-72 w-full' /></div>
      ) : (
        <div className='min-w-0 overflow-x-auto border' data-testid='user-requests-table'>
          <table className='w-full min-w-[860px] text-sm'>
            <thead className='bg-muted/50 border-b text-left'>
              <tr>
                <th className='px-4 py-3 font-medium'>{t('userUsage.requests.columns.time')}</th>
                <th className='px-4 py-3 font-medium'>{t('userUsage.requests.columns.model')}</th>
                <th className='px-4 py-3 font-medium'>{t('userUsage.requests.columns.key')}</th>
                <th className='px-4 py-3 font-medium'>{t('userUsage.requests.columns.status')}</th>
                <th className='px-4 py-3 text-right font-medium'>{t('userUsage.requests.columns.tokens')}</th>
                <th className='px-4 py-3 text-right font-medium'>{t('userUsage.requests.columns.charge')}</th>
                <th className='px-4 py-3 text-right font-medium'>{t('userUsage.requests.columns.latency')}</th>
                <th className='w-12 px-4 py-3'><span className='sr-only'>{t('userUsage.requests.columns.actions')}</span></th>
              </tr>
            </thead>
            <tbody className='divide-y'>
              {requests.data?.items.map((item) => (
                <tr key={item.id} data-testid={`user-request-row-${item.id}`}>
                  <td className='whitespace-nowrap px-4 py-3'>{formatDate(item.createdAt)}</td>
                  <td className='max-w-56 truncate px-4 py-3 font-medium' title={item.modelId}>{item.modelId || '-'}</td>
                  <td className='max-w-44 truncate px-4 py-3' title={item.apiKey?.name}>{item.apiKey?.name || '-'}</td>
                  <td className='px-4 py-3'><StatusBadge status={item.status} /></td>
                  <td className='px-4 py-3 text-right tabular-nums'>{item.totalTokens.toLocaleString()}</td>
                  <td className='px-4 py-3 text-right tabular-nums'>{formatMoney(item.chargeAmountMicros, item.currency)}</td>
                  <td className='px-4 py-3 text-right tabular-nums'>{formatLatency(item.latencyMs)}</td>
                  <td className='px-4 py-3 text-right'>
                    <Button asChild size='icon' variant='ghost' title={t('userUsage.requests.view')}>
                      <Link to='/project/requests/$requestId' params={{ requestId: item.id }} search={{ scope }}>
                        <Eye className='h-4 w-4' /><span className='sr-only'>{t('userUsage.requests.view')}</span>
                      </Link>
                    </Button>
                  </td>
                </tr>
              ))}
              {requests.data?.items.length === 0 && (
                <tr><td colSpan={8} className='text-muted-foreground px-4 py-16 text-center'>{t('userUsage.requests.empty')}</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <footer className='flex flex-wrap items-center justify-between gap-3'>
        <p className='text-muted-foreground text-sm'>{t('userUsage.requests.total', { count: requests.data?.total || 0 })}</p>
        <div className='flex items-center gap-2'>
          <Button size='icon' variant='outline' disabled={currentPage <= 1} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
            <ChevronLeft className='h-4 w-4' /><span className='sr-only'>{t('userUsage.requests.previous')}</span>
          </Button>
          <span className='min-w-20 text-center text-sm tabular-nums'>{currentPage} / {lastPage}</span>
          <Button size='icon' variant='outline' disabled={currentPage >= lastPage} onClick={() => setOffset(offset + PAGE_SIZE)}>
            <ChevronRight className='h-4 w-4' /><span className='sr-only'>{t('userUsage.requests.next')}</span>
          </Button>
        </div>
      </footer>
    </div>
  );
}
