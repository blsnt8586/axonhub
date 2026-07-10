import { useEffect, useMemo, useState } from 'react';
import { Activity, CircleCheck, CircleX, Coins, Sigma } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useRoutePermissions } from '@/hooks/useRoutePermissions';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useUserUsage, type UserUsageScope } from './data';
import { ScopeSwitch } from './shared';
import { canViewProjectUsage, formatMoney } from './utils';

type Range = '24h' | '7d' | '30d' | '90d';

export default function UserUsagePage() {
  const { t } = useTranslation();
  const projectId = useSelectedProjectId();
  const { isOwner, isProjectOwner, systemScopes, projectScopes } = useRoutePermissions();
  const canViewProject = canViewProjectUsage(isOwner, isProjectOwner, systemScopes, projectScopes);
  const [scope, setScope] = useState<UserUsageScope>('mine');
  const [range, setRange] = useState<Range>('30d');
  const dates = useMemo(() => usageRange(range), [range]);
  const usage = useUserUsage(projectId, scope, dates.from, dates.to);

  useEffect(() => {
    if (!canViewProject && scope === 'project') setScope('mine');
  }, [canViewProject, scope]);

  const chartData = useMemo(
    () =>
      (usage.data?.series || []).map((point) => ({
        ...point,
        label: new Intl.DateTimeFormat(undefined, {
          month: 'short',
          day: 'numeric',
          ...(usage.data?.granularity === 'hour' ? { hour: '2-digit' as const } : {}),
        }).format(new Date(point.bucketStart)),
        charge: point.chargeAmountMicros / 1_000_000,
      })),
    [usage.data]
  );

  return (
    <div className='flex flex-1 flex-col gap-6 p-4 md:p-6' data-testid='user-usage-page'>
      <header className='flex flex-wrap items-start justify-between gap-4'>
        <div>
          <h1 className='text-2xl font-semibold'>{t('userUsage.usage.title')}</h1>
          <p className='text-muted-foreground mt-1 text-sm'>{t('userUsage.usage.description')}</p>
        </div>
        <ScopeSwitch
          value={scope}
          onChange={setScope}
          canViewProject={canViewProject}
          mineLabel={t('userUsage.scope.mine')}
          projectLabel={t('userUsage.scope.project')}
        />
      </header>

      <div className='flex flex-wrap gap-2' data-testid='usage-range-selector'>
        {(['24h', '7d', '30d', '90d'] as Range[]).map((value) => (
          <Button key={value} size='sm' variant={range === value ? 'default' : 'outline'} aria-pressed={range === value} onClick={() => setRange(value)}>
            {t(`userUsage.usage.ranges.${value}`)}
          </Button>
        ))}
      </div>

      {usage.isError && (
        <Alert variant='destructive'>
          <AlertTitle>{t('userUsage.usage.loadFailed')}</AlertTitle>
          <AlertDescription>{usage.error instanceof Error ? usage.error.message : t('userUsage.usage.loadFailed')}</AlertDescription>
        </Alert>
      )}

      {usage.isLoading ? (
        <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-5'>{[0, 1, 2, 3, 4].map((item) => <Skeleton key={item} className='h-28' />)}</div>
      ) : (
        <>
          <section className='grid gap-4 md:grid-cols-2 xl:grid-cols-5' data-testid='user-usage-summary'>
            <Metric icon={Activity} label={t('userUsage.usage.metrics.requests')} value={(usage.data?.requestCount || 0).toLocaleString()} />
            <Metric icon={CircleCheck} label={t('userUsage.usage.metrics.success')} value={(usage.data?.successCount || 0).toLocaleString()} />
            <Metric icon={CircleX} label={t('userUsage.usage.metrics.errors')} value={(usage.data?.errorCount || 0).toLocaleString()} />
            <Metric icon={Sigma} label={t('userUsage.usage.metrics.tokens')} value={(usage.data?.totalTokens || 0).toLocaleString()} />
            <Metric icon={Coins} label={t('userUsage.usage.metrics.spend')} value={formatMoney(usage.data?.chargeAmountMicros || 0, usage.data?.currency)} />
          </section>

          <section className='min-w-0 border p-4' data-testid='user-usage-chart'>
            <div className='mb-5 flex flex-wrap items-baseline justify-between gap-2'>
              <h2 className='font-semibold'>{t('userUsage.usage.chart.title')}</h2>
              <span className='text-muted-foreground text-xs'>{t(`userUsage.usage.chart.${usage.data?.granularity || 'day'}`)}</span>
            </div>
            {chartData.length ? (
              <div className='h-80 w-full'>
                <ResponsiveContainer width='100%' height='100%'>
                  <AreaChart data={chartData} margin={{ top: 8, right: 12, left: -16, bottom: 0 }}>
                    <defs>
                      <linearGradient id='requestCountFill' x1='0' y1='0' x2='0' y2='1'>
                        <stop offset='5%' stopColor='var(--chart-1)' stopOpacity={0.35} />
                        <stop offset='95%' stopColor='var(--chart-1)' stopOpacity={0.03} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid strokeDasharray='3 3' vertical={false} />
                    <XAxis dataKey='label' tickLine={false} axisLine={false} minTickGap={28} fontSize={11} />
                    <YAxis tickLine={false} axisLine={false} allowDecimals={false} fontSize={11} />
                    <Tooltip
                      formatter={(value) => [Number(value).toLocaleString(), t('userUsage.usage.metrics.requests')]}
                      contentStyle={{ background: 'var(--popover)', border: '1px solid var(--border)', borderRadius: 6 }}
                    />
                    <Area type='monotone' dataKey='requestCount' stroke='var(--chart-1)' fill='url(#requestCountFill)' strokeWidth={2} />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <div className='text-muted-foreground flex h-80 items-center justify-center text-sm'>{t('userUsage.usage.empty')}</div>
            )}
          </section>

          <section className='space-y-4'>
            <div>
              <h2 className='font-semibold'>{t('userUsage.usage.models.title')}</h2>
              <p className='text-muted-foreground mt-1 text-sm'>{t('userUsage.usage.models.description')}</p>
            </div>
            <div className='min-w-0 overflow-x-auto border'>
              <table className='w-full min-w-[620px] text-sm'>
                <thead className='bg-muted/50 border-b text-left'>
                  <tr>
                    <th className='px-4 py-3 font-medium'>{t('userUsage.usage.models.model')}</th>
                    <th className='px-4 py-3 text-right font-medium'>{t('userUsage.usage.metrics.requests')}</th>
                    <th className='px-4 py-3 text-right font-medium'>{t('userUsage.usage.metrics.tokens')}</th>
                    <th className='px-4 py-3 text-right font-medium'>{t('userUsage.usage.metrics.spend')}</th>
                  </tr>
                </thead>
                <tbody className='divide-y'>
                  {usage.data?.models.map((model) => (
                    <tr key={model.modelId}>
                      <td className='px-4 py-3 font-medium'>{model.modelId || '-'}</td>
                      <td className='px-4 py-3 text-right tabular-nums'>{model.requestCount.toLocaleString()}</td>
                      <td className='px-4 py-3 text-right tabular-nums'>{model.totalTokens.toLocaleString()}</td>
                      <td className='px-4 py-3 text-right tabular-nums'>{formatMoney(model.chargeAmountMicros, usage.data?.currency)}</td>
                    </tr>
                  ))}
                  {usage.data?.models.length === 0 && (
                    <tr><td colSpan={4} className='text-muted-foreground px-4 py-12 text-center'>{t('userUsage.usage.empty')}</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </section>
        </>
      )}
    </div>
  );
}

function Metric({ icon: Icon, label, value }: { icon: typeof Activity; label: string; value: string }) {
  return (
    <div className='border p-4'>
      <div className='text-muted-foreground flex items-center gap-2 text-xs font-medium'><Icon className='h-4 w-4' />{label}</div>
      <p className='mt-3 break-words text-xl font-semibold tabular-nums'>{value}</p>
    </div>
  );
}

function usageRange(range: Range) {
  const to = new Date();
  const from = new Date(to);
  if (range === '24h') from.setHours(from.getHours() - 24);
  else if (range === '7d') from.setDate(from.getDate() - 7);
  else if (range === '30d') from.setDate(from.getDate() - 30);
  else from.setDate(from.getDate() - 90);
  return { from: from.toISOString(), to: to.toISOString() };
}
