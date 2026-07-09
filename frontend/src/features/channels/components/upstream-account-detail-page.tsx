import { Link, useParams } from '@tanstack/react-router';
import { IconArrowLeft, IconRefresh, IconServer } from '@tabler/icons-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { useUpstreamAccountMonitoringDetail } from '../data/channels';

const micros = (value: number) => (value / 1_000_000).toFixed(4);
const percent = (value: number) => `${(value * 100).toFixed(1)}%`;
const shortTime = (value?: string | null) => (value ? new Date(value).toLocaleString() : '-');

function statusClass(status: string) {
  if (status === 'active' || status === 'completed') return 'border-emerald-200 bg-emerald-50 text-emerald-700';
  if (status === 'failed' || status === 'error') return 'border-red-200 bg-red-50 text-red-700';
  if (status === 'disabled') return 'border-amber-200 bg-amber-50 text-amber-700';
  return 'border-slate-200 bg-slate-50 text-slate-600';
}

export function UpstreamAccountDetailPage() {
  const { accountId } = useParams({ from: '/_authenticated/channels/accounts/$accountId' });
  const query = useUpstreamAccountMonitoringDetail(accountId);
  const detail = query.data;
  const summary = detail?.summary;

  return (
    <>
      <Header />
      <Main fixed>
        <div className='mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
          <div>
            <Button asChild variant='ghost' size='sm' className='mb-2 px-0'>
              <Link to='/channels/accounts'>
                <IconArrowLeft className='mr-2 h-4 w-4' />
                Accounts
              </Link>
            </Button>
            <h1 className='flex items-center gap-2 text-xl font-semibold'>
              <IconServer className='h-5 w-5' />
              {summary?.accountName || 'Upstream account'}
            </h1>
            <p className='text-muted-foreground mt-1 text-sm'>
              {summary ? `${summary.channelName || summary.channelID} · ${summary.providerType || '-'}` : accountId}
            </p>
          </div>
          <Button variant='outline' onClick={() => query.refetch()} disabled={query.isFetching}>
            <IconRefresh className='mr-2 h-4 w-4' />
            Refresh
          </Button>
        </div>

        {query.isLoading ? (
          <div className='text-muted-foreground py-10 text-center text-sm'>Loading account detail...</div>
        ) : query.isError || !detail || !summary ? (
          <div className='text-destructive py-10 text-center text-sm'>Failed to load account detail.</div>
        ) : (
          <div className='space-y-4'>
            <div className='grid gap-3 md:grid-cols-4 xl:grid-cols-8'>
              <Metric label='Status' value={summary.schedulable ? summary.status : `${summary.status} paused`} />
              <Metric label='Requests' value={summary.requestCount.toLocaleString()} />
              <Metric label='Success' value={percent(summary.successRate)} />
              <Metric label='Errors' value={percent(summary.errorRate)} tone={summary.errorCount > 0 ? 'danger' : 'normal'} />
              <Metric
                label='429'
                value={summary.rateLimitCount.toLocaleString()}
                tone={summary.rateLimitCount > 0 ? 'warning' : 'normal'}
              />
              <Metric label='Latency' value={`${Math.round(summary.averageLatencyMs || 0)} ms`} />
              <Metric label='Charge' value={micros(summary.userChargeMicros)} />
              <Metric label='Margin' value={micros(summary.grossMarginMicros)} tone={summary.grossMarginMicros < 0 ? 'danger' : 'normal'} />
            </div>

            <div className='grid gap-4 xl:grid-cols-3'>
              <Card>
                <CardHeader className='pb-3'>
                  <CardTitle className='text-sm'>Quota and cooldown</CardTitle>
                </CardHeader>
                <CardContent className='space-y-2 text-sm'>
                  <KeyValue label='Quota used' value={micros(summary.quotaUsedMicros)} />
                  <KeyValue label='Quota limit' value={summary.quotaLimitMicros > 0 ? micros(summary.quotaLimitMicros) : 'Unlimited'} />
                  <KeyValue label='Last used' value={shortTime(summary.lastUsedAt)} />
                  <KeyValue label='Rate limit reset' value={shortTime(summary.rateLimitResetAt)} />
                  <KeyValue label='Overload until' value={shortTime(summary.overloadUntil)} />
                  <KeyValue label='Cooldown until' value={shortTime(summary.cooldownUntil)} />
                  <KeyValue label='Reason' value={summary.cooldownReason || summary.lastErrorMessage || '-'} />
                </CardContent>
              </Card>

              <Card className='xl:col-span-2'>
                <CardHeader className='pb-3'>
                  <CardTitle className='text-sm'>Usage trend</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className='overflow-x-auto'>
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Bucket</TableHead>
                          <TableHead className='text-right'>Requests</TableHead>
                          <TableHead className='text-right'>Errors</TableHead>
                          <TableHead className='text-right'>Tokens</TableHead>
                          <TableHead className='text-right'>Charge</TableHead>
                          <TableHead className='text-right'>Margin</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {detail.usageTrend.length === 0 ? (
                          <TableRow>
                            <TableCell colSpan={6} className='text-muted-foreground py-6 text-center text-sm'>
                              No aggregate usage.
                            </TableCell>
                          </TableRow>
                        ) : (
                          detail.usageTrend.map((point) => (
                            <TableRow key={point.bucketStart}>
                              <TableCell>{shortTime(point.bucketStart)}</TableCell>
                              <TableCell className='text-right'>{point.requestCount.toLocaleString()}</TableCell>
                              <TableCell className='text-right'>{point.errorCount.toLocaleString()}</TableCell>
                              <TableCell className='text-right'>{point.totalTokens.toLocaleString()}</TableCell>
                              <TableCell className='text-right'>{micros(point.userChargeMicros)}</TableCell>
                              <TableCell className='text-right'>{micros(point.grossMarginMicros)}</TableCell>
                            </TableRow>
                          ))
                        )}
                      </TableBody>
                    </Table>
                  </div>
                </CardContent>
              </Card>
            </div>

            <Card>
              <CardHeader className='pb-3'>
                <CardTitle className='text-sm'>Recent executions</CardTitle>
              </CardHeader>
              <CardContent>
                <div className='overflow-x-auto'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Time</TableHead>
                        <TableHead>Request</TableHead>
                        <TableHead>Model</TableHead>
                        <TableHead>Status</TableHead>
                        <TableHead className='text-right'>HTTP</TableHead>
                        <TableHead className='text-right'>Latency</TableHead>
                        <TableHead className='text-right'>Retry</TableHead>
                        <TableHead>Error</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {detail.recentExecutions.map((execution) => (
                        <TableRow key={execution.id}>
                          <TableCell>{shortTime(execution.createdAt)}</TableCell>
                          <TableCell>
                            <Button asChild variant='link' className='h-auto p-0 font-mono text-xs'>
                              <Link to='/requests/$requestId' params={{ requestId: execution.requestID }}>
                                {execution.requestID}
                              </Link>
                            </Button>
                          </TableCell>
                          <TableCell>{execution.modelID}</TableCell>
                          <TableCell>
                            <Badge variant='outline' className={statusClass(execution.status)}>
                              {execution.status}
                            </Badge>
                          </TableCell>
                          <TableCell className='text-right'>{execution.responseStatusCode || '-'}</TableCell>
                          <TableCell className='text-right'>
                            {execution.metricsLatencyMs ? `${execution.metricsLatencyMs} ms` : '-'}
                          </TableCell>
                          <TableCell className='text-right'>{execution.upstreamAccountRetryCount || 0}</TableCell>
                          <TableCell className='max-w-[260px] truncate'>{execution.errorMessage || '-'}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='pb-3'>
                <CardTitle className='text-sm'>Switch history</CardTitle>
              </CardHeader>
              <CardContent>
                <div className='overflow-x-auto'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Time</TableHead>
                        <TableHead>From</TableHead>
                        <TableHead>To</TableHead>
                        <TableHead>Model</TableHead>
                        <TableHead>Reason</TableHead>
                        <TableHead className='text-right'>Code</TableHead>
                        <TableHead className='text-right'>Latency</TableHead>
                        <TableHead>Error</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {detail.switchHistory.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={8} className='text-muted-foreground py-6 text-center text-sm'>
                            No switch history.
                          </TableCell>
                        </TableRow>
                      ) : (
                        detail.switchHistory.map((item) => (
                          <TableRow key={item.id}>
                            <TableCell>{shortTime(item.createdAt)}</TableCell>
                            <TableCell>{item.fromAccount?.name || item.fromAccountID || '-'}</TableCell>
                            <TableCell>{item.toAccount?.name || item.toAccountID || '-'}</TableCell>
                            <TableCell>{item.modelID || '-'}</TableCell>
                            <TableCell>{item.reason}</TableCell>
                            <TableCell className='text-right'>{item.errorCode || '-'}</TableCell>
                            <TableCell className='text-right'>{item.latencyMs ? `${item.latencyMs} ms` : '-'}</TableCell>
                            <TableCell className='max-w-[260px] truncate'>{item.errorMessage || '-'}</TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </div>
              </CardContent>
            </Card>
          </div>
        )}
      </Main>
    </>
  );
}

function Metric({ label, value, tone = 'normal' }: { label: string; value: string; tone?: 'normal' | 'warning' | 'danger' }) {
  const valueClass = tone === 'warning' ? 'text-amber-700' : tone === 'danger' ? 'text-red-700' : 'text-foreground';
  return (
    <div className='bg-background rounded-md border px-3 py-2'>
      <div className='text-muted-foreground text-xs'>{label}</div>
      <div className={`mt-1 text-lg font-semibold ${valueClass}`}>{value}</div>
    </div>
  );
}

function KeyValue({ label, value }: { label: string; value: string }) {
  return (
    <div className='flex items-start justify-between gap-4 border-b py-1.5 last:border-0'>
      <span className='text-muted-foreground'>{label}</span>
      <span className='max-w-[60%] text-right font-medium break-words'>{value}</span>
    </div>
  );
}
