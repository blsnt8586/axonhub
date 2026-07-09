import { useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { Link } from '@tanstack/react-router';
import { IconArchive, IconCheck, IconPlayerPlay, IconPlus, IconRefresh, IconServer, IconShieldLock } from '@tabler/icons-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import { useChannels } from '../context/channels-context';
import {
  useArchiveUpstreamAccount,
  useArchiveUpstreamAccountPool,
  useCreateUpstreamAccount,
  useCreateUpstreamAccountPool,
  useTestUpstreamAccount,
  useUpdateUpstreamAccount,
  useUpdateUpstreamAccountPool,
  useUpstreamAccountPools,
  useUpstreamAccounts,
} from '../data/channels';
import { UpstreamAccount, UpstreamAccountPool } from '../data/schema';

type AccountForm = {
  id?: string;
  name: string;
  poolID: string;
  credentialType: 'api_key' | 'oauth' | 'custom';
  credential: string;
  status: 'active' | 'disabled' | 'archived' | 'error';
  schedulable: boolean;
  priority: string;
  weight: string;
  concurrencyLimit: string;
  rateMultiplier: string;
  quotaLimitMicros: string;
  quotaUsedMicros: string;
  expiresAt: string;
  rateLimitResetAt: string;
  overloadUntil: string;
  cooldownUntil: string;
  cooldownReason: string;
};

type PoolForm = {
  id?: string;
  name: string;
  status: 'enabled' | 'disabled' | 'archived';
  priority: string;
  modelPatterns: string;
  projectIDs: string;
  remark: string;
};

const emptyAccountForm: AccountForm = {
  name: '',
  poolID: 'none',
  credentialType: 'api_key',
  credential: '',
  status: 'active',
  schedulable: true,
  priority: '0',
  weight: '100',
  concurrencyLimit: '0',
  rateMultiplier: '1',
  quotaLimitMicros: '0',
  quotaUsedMicros: '0',
  expiresAt: '',
  rateLimitResetAt: '',
  overloadUntil: '',
  cooldownUntil: '',
  cooldownReason: '',
};

const emptyPoolForm: PoolForm = {
  name: '',
  status: 'enabled',
  priority: '0',
  modelPatterns: '',
  projectIDs: '',
  remark: '',
};

function splitList(value: string) {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
}

function splitIntList(value: string) {
  return splitList(value)
    .map((item) => Number.parseInt(item, 10))
    .filter((item) => Number.isFinite(item) && item > 0);
}

function intValue(value: string, fallback = 0) {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function floatValue(value: string, fallback = 1) {
  const parsed = Number.parseFloat(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function optionalIso(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const date = new Date(trimmed);
  return Number.isNaN(date.getTime()) ? trimmed : date.toISOString();
}

function accountFormFromAccount(account: UpstreamAccount): AccountForm {
  return {
    id: account.id,
    name: account.name,
    poolID: account.poolID || 'none',
    credentialType: account.credentialType,
    credential: '',
    status: account.status,
    schedulable: account.schedulable,
    priority: String(account.priority),
    weight: String(account.weight),
    concurrencyLimit: String(account.concurrencyLimit),
    rateMultiplier: String(account.rateMultiplier),
    quotaLimitMicros: String(account.quotaLimitMicros),
    quotaUsedMicros: String(account.quotaUsedMicros),
    expiresAt: account.expiresAt || '',
    rateLimitResetAt: account.rateLimitResetAt || '',
    overloadUntil: account.overloadUntil || '',
    cooldownUntil: account.cooldownUntil || '',
    cooldownReason: account.cooldownReason || '',
  };
}

function poolFormFromPool(pool: UpstreamAccountPool): PoolForm {
  return {
    id: pool.id,
    name: pool.name,
    status: pool.status,
    priority: String(pool.priority),
    modelPatterns: (pool.modelPatterns || []).join(', '),
    projectIDs: (pool.projectIds || []).join(', '),
    remark: pool.remark || '',
  };
}

function statusBadgeClass(status: string) {
  if (status === 'active' || status === 'enabled') return 'border-emerald-200 bg-emerald-50 text-emerald-700';
  if (status === 'error') return 'border-red-200 bg-red-50 text-red-700';
  if (status === 'disabled') return 'border-amber-200 bg-amber-50 text-amber-700';
  return 'border-slate-200 bg-slate-50 text-slate-600';
}

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ChannelsUpstreamAccountsDialog({ open, onOpenChange }: Props) {
  const { currentRow, setOpen } = useChannels();
  const channelID = currentRow?.id || '';
  const [accountForm, setAccountForm] = useState<AccountForm>(emptyAccountForm);
  const [poolForm, setPoolForm] = useState<PoolForm>(emptyPoolForm);

  const poolsQuery = useUpstreamAccountPools(channelID, { enabled: open && !!channelID });
  const accountsQuery = useUpstreamAccounts(channelID, { enabled: open && !!channelID });
  const createPool = useCreateUpstreamAccountPool();
  const updatePool = useUpdateUpstreamAccountPool();
  const archivePool = useArchiveUpstreamAccountPool();
  const createAccount = useCreateUpstreamAccount();
  const updateAccount = useUpdateUpstreamAccount();
  const archiveAccount = useArchiveUpstreamAccount();
  const testAccount = useTestUpstreamAccount();

  const pools = poolsQuery.data || [];
  const accounts = accountsQuery.data || [];
  const accountPoolName = useMemo(() => new Map(pools.map((pool) => [pool.id, pool.name])), [pools]);
  const accountHealth = useMemo(
    () => ({
      total: accounts.length,
      eligible: accounts.filter((account) => account.eligibleNow).length,
      cooling: accounts.filter((account) => account.cooldownUntil || account.rateLimitResetAt || account.overloadUntil).length,
      error: accounts.filter((account) => account.status === 'error').length,
      disabled: accounts.filter((account) => account.status === 'disabled' || !account.schedulable).length,
    }),
    [accounts]
  );
  const busy =
    createPool.isPending ||
    updatePool.isPending ||
    archivePool.isPending ||
    createAccount.isPending ||
    updateAccount.isPending ||
    archiveAccount.isPending ||
    testAccount.isPending;

  const handleClose = () => {
    setOpen(null);
    onOpenChange(false);
    setAccountForm(emptyAccountForm);
    setPoolForm(emptyPoolForm);
  };

  const handleSavePool = async () => {
    if (!currentRow || !poolForm.name.trim()) return;
    const input = {
      name: poolForm.name.trim(),
      status: poolForm.status,
      priority: intValue(poolForm.priority),
      modelPatterns: splitList(poolForm.modelPatterns),
      projectIDs: splitIntList(poolForm.projectIDs),
      remark: poolForm.remark.trim() || undefined,
      clearRemark: poolForm.id ? poolForm.remark.trim() === '' : undefined,
    };
    if (poolForm.id) {
      await updatePool.mutateAsync({ id: poolForm.id, input });
    } else {
      await createPool.mutateAsync({ channelID: currentRow.id, ...input });
    }
    setPoolForm(emptyPoolForm);
  };

  const handleSaveAccount = async () => {
    if (!currentRow || !accountForm.name.trim()) return;
    const credentialValue = accountForm.credential.trim();
    const credentials =
      credentialValue.length > 0
        ? {
            [accountForm.credentialType === 'api_key' ? 'apiKey' : accountForm.credentialType === 'oauth' ? 'oauth' : 'rawJson']:
              credentialValue,
          }
        : undefined;

    const input = {
      poolID: accountForm.poolID === 'none' ? undefined : accountForm.poolID,
      name: accountForm.name.trim(),
      credentialType: accountForm.credentialType,
      credentials,
      status: accountForm.status,
      schedulable: accountForm.schedulable,
      priority: intValue(accountForm.priority),
      weight: intValue(accountForm.weight, 100),
      concurrencyLimit: intValue(accountForm.concurrencyLimit),
      rateMultiplier: floatValue(accountForm.rateMultiplier),
      expiresAt: optionalIso(accountForm.expiresAt),
      quotaLimitMicros: intValue(accountForm.quotaLimitMicros),
      quotaUsedMicros: intValue(accountForm.quotaUsedMicros),
      rateLimitResetAt: optionalIso(accountForm.rateLimitResetAt),
      overloadUntil: optionalIso(accountForm.overloadUntil),
      cooldownUntil: optionalIso(accountForm.cooldownUntil),
      cooldownReason: accountForm.cooldownReason.trim() || undefined,
    };

    if (accountForm.id) {
      await updateAccount.mutateAsync({
        id: accountForm.id,
        input: {
          ...input,
          clearPool: accountForm.poolID === 'none',
          clearExpiresAt: accountForm.expiresAt.trim() === '',
          clearRateLimitResetAt: accountForm.rateLimitResetAt.trim() === '',
          clearOverloadUntil: accountForm.overloadUntil.trim() === '',
          clearCooldownUntil: accountForm.cooldownUntil.trim() === '',
          clearCooldownReason: accountForm.cooldownReason.trim() === '',
        },
      });
    } else {
      if (!credentials) return;
      await createAccount.mutateAsync({ channelID: currentRow.id, ...input, credentials });
    }
    setAccountForm(emptyAccountForm);
  };

  const handleRecoverAccount = async (account: UpstreamAccount) => {
    await updateAccount.mutateAsync({
      id: account.id,
      input: {
        status: 'active',
        schedulable: true,
        clearErrorMessage: true,
        clearRateLimitResetAt: true,
        clearOverloadUntil: true,
        clearCooldownUntil: true,
        clearCooldownReason: true,
      },
    });
  };

  if (!currentRow) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[90vh] flex-col p-0 sm:max-w-6xl'>
        <DialogHeader className='border-b px-6 py-4 text-left'>
          <DialogTitle className='flex items-center gap-2 text-base'>
            <IconServer className='h-5 w-5' />
            Upstream account pools
          </DialogTitle>
          <DialogDescription>
            Manage account pools for {currentRow.name}. Channel credentials remain the fallback when no pool is configured.
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='min-h-0 flex-1'>
          <Tabs defaultValue='accounts' className='px-6 py-4'>
            <TabsList>
              <TabsTrigger value='accounts'>Accounts</TabsTrigger>
              <TabsTrigger value='pools'>Pools</TabsTrigger>
            </TabsList>

            <TabsContent value='accounts' className='mt-4 space-y-4'>
              <div className='grid gap-3 md:grid-cols-5'>
                <HealthMetric label='Total' value={accountHealth.total} />
                <HealthMetric label='Eligible' value={accountHealth.eligible} tone='good' />
                <HealthMetric label='Cooling' value={accountHealth.cooling} tone='warning' />
                <HealthMetric label='Error' value={accountHealth.error} tone='danger' />
                <HealthMetric label='Disabled' value={accountHealth.disabled} />
              </div>

              <Card>
                <CardHeader className='gap-1 px-4 py-3'>
                  <CardTitle className='text-sm'>Account inventory</CardTitle>
                  <CardDescription>Status, schedulability, quota, cooldown, and failure reason.</CardDescription>
                </CardHeader>
                <CardContent className='px-4 pb-4'>
                  {accountsQuery.isLoading ? (
                    <div className='text-muted-foreground py-8 text-center text-sm'>Loading upstream accounts...</div>
                  ) : accountsQuery.isError ? (
                    <div className='text-destructive py-8 text-center text-sm'>Failed to load upstream accounts.</div>
                  ) : accounts.length === 0 ? (
                    <div className='text-muted-foreground flex flex-col items-center justify-center gap-2 py-8 text-sm'>
                      <IconShieldLock className='h-9 w-9' />
                      No upstream accounts. Existing channel credentials will be used.
                    </div>
                  ) : (
                    <div className='overflow-x-auto'>
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>Name</TableHead>
                            <TableHead>Pool</TableHead>
                            <TableHead>Status</TableHead>
                            <TableHead>Sched.</TableHead>
                            <TableHead>Priority</TableHead>
                            <TableHead>Weight</TableHead>
                            <TableHead>Concurrency</TableHead>
                            <TableHead>Quota</TableHead>
                            <TableHead>Cooldown</TableHead>
                            <TableHead>Last used</TableHead>
                            <TableHead>Reason</TableHead>
                            <TableHead className='text-right'>Actions</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {accounts.map((account) => (
                            <TableRow key={account.id}>
                              <TableCell className='font-medium'>
                                <Button asChild variant='link' className='h-auto p-0 font-medium'>
                                  <Link to='/channels/accounts/$accountId' params={{ accountId: account.id }}>
                                    {account.name}
                                  </Link>
                                </Button>
                              </TableCell>
                              <TableCell>{account.poolID ? accountPoolName.get(account.poolID) || account.poolID : 'Fallback'}</TableCell>
                              <TableCell>
                                <Badge variant='outline' className={statusBadgeClass(account.status)}>
                                  {account.status}
                                </Badge>
                              </TableCell>
                              <TableCell>
                                {account.eligibleNow ? (
                                  <IconCheck className='h-4 w-4 text-emerald-600' />
                                ) : account.schedulable ? (
                                  'No'
                                ) : (
                                  'Paused'
                                )}
                              </TableCell>
                              <TableCell>{account.priority}</TableCell>
                              <TableCell>{account.weight}</TableCell>
                              <TableCell>{account.concurrencyLimit || 'Unlimited'}</TableCell>
                              <TableCell>
                                {account.quotaLimitMicros > 0
                                  ? `${account.quotaUsedMicros}/${account.quotaLimitMicros}`
                                  : `${account.quotaUsedMicros}/unlimited`}
                              </TableCell>
                              <TableCell>{account.cooldownUntil || account.rateLimitResetAt || account.overloadUntil || '-'}</TableCell>
                              <TableCell>{account.lastUsedAt || '-'}</TableCell>
                              <TableCell className='max-w-[180px] truncate'>
                                {account.ineligibleReason || account.errorMessage || '-'}
                              </TableCell>
                              <TableCell>
                                <div className='flex justify-end gap-1'>
                                  <Button variant='outline' size='sm' onClick={() => setAccountForm(accountFormFromAccount(account))}>
                                    Edit
                                  </Button>
                                  <Button variant='outline' size='sm' onClick={() => testAccount.mutateAsync(account.id)} disabled={busy}>
                                    <IconPlayerPlay className='h-3.5 w-3.5' />
                                  </Button>
                                  <Button
                                    variant='outline'
                                    size='sm'
                                    onClick={() => handleRecoverAccount(account)}
                                    disabled={busy || (account.status === 'active' && account.schedulable && account.eligibleNow)}
                                  >
                                    <IconRefresh className='h-3.5 w-3.5' />
                                  </Button>
                                  <Button
                                    variant='outline'
                                    size='sm'
                                    onClick={() =>
                                      updateAccount.mutateAsync({
                                        id: account.id,
                                        input: {
                                          status: account.status === 'active' ? 'disabled' : 'active',
                                          schedulable: account.status !== 'active',
                                        },
                                      })
                                    }
                                    disabled={busy}
                                  >
                                    {account.status === 'active' ? 'Disable' : 'Enable'}
                                  </Button>
                                  <Button
                                    variant='outline'
                                    size='sm'
                                    onClick={() => archiveAccount.mutateAsync({ id: account.id, channelID })}
                                    disabled={busy}
                                  >
                                    <IconArchive className='h-3.5 w-3.5' />
                                  </Button>
                                </div>
                              </TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </div>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader className='gap-1 px-4 py-3'>
                  <CardTitle className='text-sm'>{accountForm.id ? 'Edit account' : 'Add account'}</CardTitle>
                  <CardDescription>
                    Credential fields are write-only. Leave credential blank while editing to keep the existing secret.
                  </CardDescription>
                </CardHeader>
                <CardContent className='grid gap-3 px-4 pb-4 md:grid-cols-4'>
                  <Field label='Name'>
                    <Input value={accountForm.name} onChange={(event) => setAccountForm({ ...accountForm, name: event.target.value })} />
                  </Field>
                  <Field label='Pool'>
                    <Select value={accountForm.poolID} onValueChange={(value) => setAccountForm({ ...accountForm, poolID: value })}>
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='none'>No pool</SelectItem>
                        {pools.map((pool) => (
                          <SelectItem key={pool.id} value={pool.id}>
                            {pool.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </Field>
                  <Field label='Credential type'>
                    <Select
                      value={accountForm.credentialType}
                      onValueChange={(value: AccountForm['credentialType']) => setAccountForm({ ...accountForm, credentialType: value })}
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='api_key'>API key</SelectItem>
                        <SelectItem value='oauth'>OAuth</SelectItem>
                        <SelectItem value='custom'>Custom JSON</SelectItem>
                      </SelectContent>
                    </Select>
                  </Field>
                  <Field label='Credential'>
                    <Input
                      type='password'
                      value={accountForm.credential}
                      placeholder={accountForm.id ? 'Leave blank to keep existing' : 'Required'}
                      onChange={(event) => setAccountForm({ ...accountForm, credential: event.target.value })}
                    />
                  </Field>
                  <Field label='Status'>
                    <Select
                      value={accountForm.status}
                      onValueChange={(value: AccountForm['status']) => setAccountForm({ ...accountForm, status: value })}
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='active'>Active</SelectItem>
                        <SelectItem value='disabled'>Disabled</SelectItem>
                        <SelectItem value='error'>Error</SelectItem>
                        <SelectItem value='archived'>Archived</SelectItem>
                      </SelectContent>
                    </Select>
                  </Field>
                  <Field label='Priority'>
                    <Input
                      value={accountForm.priority}
                      onChange={(event) => setAccountForm({ ...accountForm, priority: event.target.value })}
                    />
                  </Field>
                  <Field label='Weight'>
                    <Input
                      value={accountForm.weight}
                      onChange={(event) => setAccountForm({ ...accountForm, weight: event.target.value })}
                    />
                  </Field>
                  <Field label='Concurrency'>
                    <Input
                      value={accountForm.concurrencyLimit}
                      onChange={(event) => setAccountForm({ ...accountForm, concurrencyLimit: event.target.value })}
                    />
                  </Field>
                  <Field label='Rate multiplier'>
                    <Input
                      value={accountForm.rateMultiplier}
                      onChange={(event) => setAccountForm({ ...accountForm, rateMultiplier: event.target.value })}
                    />
                  </Field>
                  <Field label='Quota limit micros'>
                    <Input
                      value={accountForm.quotaLimitMicros}
                      onChange={(event) => setAccountForm({ ...accountForm, quotaLimitMicros: event.target.value })}
                    />
                  </Field>
                  <Field label='Quota used micros'>
                    <Input
                      value={accountForm.quotaUsedMicros}
                      onChange={(event) => setAccountForm({ ...accountForm, quotaUsedMicros: event.target.value })}
                    />
                  </Field>
                  <div className='flex items-end gap-2 pb-2'>
                    <Switch
                      checked={accountForm.schedulable}
                      onCheckedChange={(checked) => setAccountForm({ ...accountForm, schedulable: checked })}
                    />
                    <Label>Schedulable</Label>
                  </div>
                  <Field label='Expires at'>
                    <Input
                      value={accountForm.expiresAt}
                      onChange={(event) => setAccountForm({ ...accountForm, expiresAt: event.target.value })}
                    />
                  </Field>
                  <Field label='Rate limit reset'>
                    <Input
                      value={accountForm.rateLimitResetAt}
                      onChange={(event) => setAccountForm({ ...accountForm, rateLimitResetAt: event.target.value })}
                    />
                  </Field>
                  <Field label='Overload until'>
                    <Input
                      value={accountForm.overloadUntil}
                      onChange={(event) => setAccountForm({ ...accountForm, overloadUntil: event.target.value })}
                    />
                  </Field>
                  <Field label='Cooldown until'>
                    <Input
                      value={accountForm.cooldownUntil}
                      onChange={(event) => setAccountForm({ ...accountForm, cooldownUntil: event.target.value })}
                    />
                  </Field>
                  <div className='md:col-span-4'>
                    <Field label='Cooldown reason'>
                      <Input
                        value={accountForm.cooldownReason}
                        onChange={(event) => setAccountForm({ ...accountForm, cooldownReason: event.target.value })}
                      />
                    </Field>
                  </div>
                  <div className='flex gap-2 md:col-span-4'>
                    <Button
                      onClick={handleSaveAccount}
                      disabled={busy || !accountForm.name.trim() || (!accountForm.id && !accountForm.credential.trim())}
                    >
                      <IconPlus className='mr-1 h-4 w-4' />
                      {accountForm.id ? 'Save account' : 'Create account'}
                    </Button>
                    {accountForm.id && (
                      <Button variant='outline' onClick={() => setAccountForm(emptyAccountForm)} disabled={busy}>
                        New account
                      </Button>
                    )}
                  </div>
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value='pools' className='mt-4 space-y-4'>
              <Card>
                <CardHeader className='gap-1 px-4 py-3'>
                  <CardTitle className='text-sm'>Pool targeting</CardTitle>
                  <CardDescription>
                    Use pools to target model patterns or project IDs when a channel needs more than one account group.
                  </CardDescription>
                </CardHeader>
                <CardContent className='px-4 pb-4'>
                  {poolsQuery.isLoading ? (
                    <div className='text-muted-foreground py-8 text-center text-sm'>Loading pools...</div>
                  ) : pools.length === 0 ? (
                    <div className='text-muted-foreground py-8 text-center text-sm'>No pools configured.</div>
                  ) : (
                    <div className='grid gap-2'>
                      {pools.map((pool) => (
                        <div
                          key={pool.id}
                          className='flex flex-col gap-2 rounded-md border p-3 md:flex-row md:items-center md:justify-between'
                        >
                          <div className='min-w-0'>
                            <div className='flex items-center gap-2'>
                              <span className='font-medium'>{pool.name}</span>
                              <Badge variant='outline' className={statusBadgeClass(pool.status)}>
                                {pool.status}
                              </Badge>
                              <span className='text-muted-foreground text-xs'>priority {pool.priority}</span>
                            </div>
                            <div className='text-muted-foreground mt-1 text-xs'>
                              models {(pool.modelPatterns || []).join(', ') || '*'} · projects {(pool.projectIds || []).join(', ') || '*'}
                            </div>
                          </div>
                          <div className='flex gap-2'>
                            <Button variant='outline' size='sm' onClick={() => setPoolForm(poolFormFromPool(pool))}>
                              Edit
                            </Button>
                            <Button
                              variant='outline'
                              size='sm'
                              onClick={() => archivePool.mutateAsync({ id: pool.id, channelID })}
                              disabled={busy}
                            >
                              <IconArchive className='h-3.5 w-3.5' />
                            </Button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader className='gap-1 px-4 py-3'>
                  <CardTitle className='text-sm'>{poolForm.id ? 'Edit pool' : 'Add pool'}</CardTitle>
                  <CardDescription>Model patterns and project IDs are comma-separated.</CardDescription>
                </CardHeader>
                <CardContent className='grid gap-3 px-4 pb-4 md:grid-cols-3'>
                  <Field label='Name'>
                    <Input value={poolForm.name} onChange={(event) => setPoolForm({ ...poolForm, name: event.target.value })} />
                  </Field>
                  <Field label='Status'>
                    <Select
                      value={poolForm.status}
                      onValueChange={(value: PoolForm['status']) => setPoolForm({ ...poolForm, status: value })}
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='enabled'>Enabled</SelectItem>
                        <SelectItem value='disabled'>Disabled</SelectItem>
                        <SelectItem value='archived'>Archived</SelectItem>
                      </SelectContent>
                    </Select>
                  </Field>
                  <Field label='Priority'>
                    <Input value={poolForm.priority} onChange={(event) => setPoolForm({ ...poolForm, priority: event.target.value })} />
                  </Field>
                  <Field label='Model patterns'>
                    <Input
                      value={poolForm.modelPatterns}
                      onChange={(event) => setPoolForm({ ...poolForm, modelPatterns: event.target.value })}
                    />
                  </Field>
                  <Field label='Project IDs'>
                    <Input value={poolForm.projectIDs} onChange={(event) => setPoolForm({ ...poolForm, projectIDs: event.target.value })} />
                  </Field>
                  <div className='md:col-span-3'>
                    <Field label='Remark'>
                      <Textarea value={poolForm.remark} onChange={(event) => setPoolForm({ ...poolForm, remark: event.target.value })} />
                    </Field>
                  </div>
                  <div className='flex gap-2 md:col-span-3'>
                    <Button onClick={handleSavePool} disabled={busy || !poolForm.name.trim()}>
                      <IconPlus className='mr-1 h-4 w-4' />
                      {poolForm.id ? 'Save pool' : 'Create pool'}
                    </Button>
                    {poolForm.id && (
                      <Button variant='outline' onClick={() => setPoolForm(emptyPoolForm)} disabled={busy}>
                        New pool
                      </Button>
                    )}
                    <Button
                      variant='ghost'
                      onClick={() => {
                        poolsQuery.refetch();
                        accountsQuery.refetch();
                      }}
                    >
                      <IconRefresh className='mr-1 h-4 w-4' />
                      Refresh
                    </Button>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </ScrollArea>

        <DialogFooter className='border-t px-6 py-4'>
          <Button variant='outline' onClick={handleClose}>
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
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

function HealthMetric({ label, value, tone }: { label: string; value: number; tone?: 'good' | 'warning' | 'danger' }) {
  const valueClass =
    tone === 'good' ? 'text-emerald-700' : tone === 'warning' ? 'text-amber-700' : tone === 'danger' ? 'text-red-700' : 'text-foreground';

  return (
    <div className='bg-background rounded-md border px-3 py-2'>
      <div className='text-muted-foreground text-xs'>{label}</div>
      <div className={`mt-1 text-xl font-semibold ${valueClass}`}>{value}</div>
    </div>
  );
}
