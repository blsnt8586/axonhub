import type { ReactNode } from 'react';
import { useMemo, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { IconAlertTriangle, IconArrowRight, IconRefresh, IconServer } from '@tabler/icons-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { UpstreamAccountMonitoringFilter, useUpstreamAccountMonitoring } from '../data/channels';
import type { UpstreamAccountMonitoringSummary } from '../data/schema';

const percent = (value: number) => `${(value * 100).toFixed(1)}%`;
const micros = (value: number) => (value / 1_000_000).toFixed(4);
const compact = (value: number | null | undefined) => (value == null ? '-' : Math.round(value).toLocaleString());

function statusClass(status: string) {
  if (status === 'active') return 'border-emerald-200 bg-emerald-50 text-emerald-700';
  if (status === 'error') return 'border-red-200 bg-red-50 text-red-700';
  if (status === 'disabled') return 'border-amber-200 bg-amber-50 text-amber-700';
  return 'border-slate-200 bg-slate-50 text-slate-600';
}

function buildFilter(form: FilterForm): UpstreamAccountMonitoringFilter {
  return {
    from: toIso(form.from),
    to: toIso(form.to),
    providerType: form.providerType === 'all' ? undefined : form.providerType,
    status: form.status === 'all' ? undefined : form.status,
    modelID: form.modelID.trim() || undefined,
  };
}

function toIso(value: string) {
  if (!value) return undefined;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toISOString();
}

type FilterForm = {
  from: string;
  to: string;
  providerType: string;
  status: string;
  modelID: string;
};

const defaultFilter: FilterForm = {
  from: '',
  to: '',
  providerType: 'all',
  status: 'all',
  modelID: '',
};

export function UpstreamAccountMonitoringPage() {
  const [form, setForm] = useState<FilterForm>(defaultFilter);
  const filter = useMemo(() => buildFilter(form), [form]);
  const query = useUpstreamAccountMonitoring(filter);
  const rows = query.data || [];
  const totals = useMemo(
    () =>
      rows.reduce(
        (acc, row) => {
          acc.accounts += 1;
          acc.requests += row.requestCount;
          acc.errors += row.errorCount;
          acc.rateLimits += row.rateLimitCount;
          acc.serverErrors += row.serverErrorCount;
          acc.userCharge += row.userChargeMicros;
          acc.margin += row.grossMarginMicros;
          if (row.status !== 'active' || !row.schedulable) acc.unhealthy += 1;
          return acc;
        },
        { accounts: 0, requests: 0, errors: 0, rateLimits: 0, serverErrors: 0, userCharge: 0, margin: 0, unhealthy: 0 }
      ),
    [rows]
  );

  return (
    <>
      <Header />
      <Main fixed>
        <div className='mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
          <div>
            <h1 className='flex items-center gap-2 text-xl font-semibold'>
              <IconServer className='h-5 w-5' />
              Upstream accounts
            </h1>
            <p className='text-muted-foreground mt-1 text-sm'>Account-pool usage, errors, cost, and switching visibility.</p>
          </div>
          <Button variant='outline' onClick={() => query.refetch()} disabled={query.isFetching}>
            <IconRefresh className='mr-2 h-4 w-4' />
            Refresh
          </Button>
        </div>

        <div className='grid gap-3 md:grid-cols-4 xl:grid-cols-7'>
          <Metric label='Accounts' value={totals.accounts.toLocaleString()} />
          <Metric label='Unhealthy' value={totals.unhealthy.toLocaleString()} tone={totals.unhealthy > 0 ? 'warning' : 'normal'} />
          <Metric label='Requests' value={totals.requests.toLocaleString()} />
          <Metric label='Errors' value={totals.errors.toLocaleString()} tone={totals.errors > 0 ? 'danger' : 'normal'} />
          <Metric label='429' value={totals.rateLimits.toLocaleString()} tone={totals.rateLimits > 0 ? 'warning' : 'normal'} />
          <Metric label='Charge' value={micros(totals.userCharge)} />
          <Metric label='Margin' value={micros(totals.margin)} tone={totals.margin < 0 ? 'danger' : 'normal'} />
        </div>

        <Card className='mt-4'>
          <CardHeader className='pb-3'>
            <CardTitle className='text-sm'>Filters</CardTitle>
          </CardHeader>
          <CardContent className='grid gap-3 md:grid-cols-5'>
            <Field label='From'>
              <Input
                type='datetime-local'
                value={form.from}
                onChange={(event) => setForm((prev) => ({ ...prev, from: event.target.value }))}
              />
            </Field>
            <Field label='To'>
              <Input type='datetime-local' value={form.to} onChange={(event) => setForm((prev) => ({ ...prev, to: event.target.value }))} />
            </Field>
            <Field label='Provider'>
              <Input
                value={form.providerType === 'all' ? '' : form.providerType}
                placeholder='openai'
                onChange={(event) => setForm((prev) => ({ ...prev, providerType: event.target.value || 'all' }))}
              />
            </Field>
            <Field label='Status'>
              <Select value={form.status} onValueChange={(value) => setForm((prev) => ({ ...prev, status: value }))}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>All</SelectItem>
                  <SelectItem value='active'>Active</SelectItem>
                  <SelectItem value='disabled'>Disabled</SelectItem>
                  <SelectItem value='error'>Error</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Field label='Model'>
              <Input
                value={form.modelID}
                placeholder='gpt-4o'
                onChange={(event) => setForm((prev) => ({ ...prev, modelID: event.target.value }))}
              />
            </Field>
          </CardContent>
        </Card>

        <Card className='mt-4 min-h-0'>
          <CardHeader className='pb-3'>
            <CardTitle className='text-sm'>Account health</CardTitle>
          </CardHeader>
          <CardContent>
            {query.isLoading ? (
              <div className='text-muted-foreground py-10 text-center text-sm'>Loading account monitoring...</div>
            ) : query.isError ? (
              <div className='text-destructive flex items-center justify-center gap-2 py-10 text-sm'>
                <IconAlertTriangle className='h-4 w-4' />
                Failed to load account monitoring.
              </div>
            ) : rows.length === 0 ? (
              <div className='text-muted-foreground py-10 text-center text-sm'>No upstream account usage matched the filters.</div>
            ) : (
              <div className='overflow-x-auto'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Account</TableHead>
                      <TableHead>Channel</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead className='text-right'>Requests</TableHead>
                      <TableHead className='text-right'>Success</TableHead>
                      <TableHead className='text-right'>Errors</TableHead>
                      <TableHead className='text-right'>429</TableHead>
                      <TableHead className='text-right'>5xx</TableHead>
                      <TableHead className='text-right'>Latency</TableHead>
                      <TableHead className='text-right'>Charge</TableHead>
                      <TableHead className='text-right'>Margin</TableHead>
                      <TableHead className='text-right'>Quota</TableHead>
                      <TableHead className='text-right'>Detail</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {rows.map((row) => (
                      <AccountRow key={row.accountID} row={row} />
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>
      </Main>
    </>
  );
}

function AccountRow({ row }: { row: UpstreamAccountMonitoringSummary }) {
  return (
    <TableRow>
      <TableCell className='font-medium'>
        <div>{row.accountName}</div>
        {row.lastErrorMessage && <div className='text-muted-foreground max-w-[220px] truncate text-xs'>{row.lastErrorMessage}</div>}
      </TableCell>
      <TableCell>
        <div>{row.channelName || row.channelID}</div>
        <div className='text-muted-foreground text-xs'>{row.providerType || '-'}</div>
      </TableCell>
      <TableCell>
        <Badge variant='outline' className={statusClass(row.status)}>
          {row.schedulable ? row.status : `${row.status} paused`}
        </Badge>
      </TableCell>
      <TableCell className='text-right'>{row.requestCount.toLocaleString()}</TableCell>
      <TableCell className='text-right'>{percent(row.successRate)}</TableCell>
      <TableCell className='text-right'>{percent(row.errorRate)}</TableCell>
      <TableCell className='text-right'>{row.rateLimitCount.toLocaleString()}</TableCell>
      <TableCell className='text-right'>{row.serverErrorCount.toLocaleString()}</TableCell>
      <TableCell className='text-right'>{compact(row.averageLatencyMs)} ms</TableCell>
      <TableCell className='text-right'>{micros(row.userChargeMicros)}</TableCell>
      <TableCell className='text-right'>{micros(row.grossMarginMicros)}</TableCell>
      <TableCell className='text-right'>
        {row.quotaLimitMicros > 0 ? `${micros(row.quotaUsedMicros)} / ${micros(row.quotaLimitMicros)}` : micros(row.quotaUsedMicros)}
      </TableCell>
      <TableCell className='text-right'>
        <Button asChild variant='outline' size='sm'>
          <Link to='/channels/accounts/$accountId' params={{ accountId: row.accountID }}>
            <IconArrowRight className='h-4 w-4' />
          </Link>
        </Button>
      </TableCell>
    </TableRow>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className='grid gap-1.5'>
      <Label className='text-xs'>{label}</Label>
      {children}
    </div>
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
