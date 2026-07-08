import { FormEvent, useMemo, useState } from 'react';
import { AlertCircle, Loader2, RefreshCw, Save, WalletCards } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { extractNumberIDAsNumber } from '@/lib/utils';
import { useUsers } from '@/features/users/data/users';
import {
  type BillingPriceRule,
  type LedgerTransactionDirection,
  type ModelPrice,
  useAdminBillingOverview,
  useAdminUserBillingDetail,
  useAdjustUserBalance,
  useSaveBillingPriceRule,
  useUpsertEPayPaymentProvider,
} from './data/admin-billing';

type PriceForm = {
  id?: string;
  scopeType: 'global' | 'project';
  scopeId: string;
  modelPattern: string;
  promptPrice: string;
  completionPrice: string;
  currency: string;
  priority: string;
  enabled: boolean;
};

function microsToAmount(value: number) {
  return value / 1_000_000;
}

function formatDate(value?: string | null) {
  if (!value) return '-';
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

function normalizeAmount(value: string, scale = 6) {
  const trimmed = value.trim();
  const pattern = new RegExp(`^\\d+(\\.\\d{1,${scale}})?$`);
  if (!pattern.test(trimmed)) return '';
  const amount = Number(trimmed);
  if (!Number.isFinite(amount) || amount <= 0) return '';
  return trimmed;
}

function modelPriceFromForm(promptPrice: string, completionPrice: string): ModelPrice {
  return {
    items: [
      {
        itemCode: 'prompt_tokens',
        pricing: { mode: 'usage_per_unit', usagePerUnit: promptPrice },
      },
      {
        itemCode: 'completion_tokens',
        pricing: { mode: 'usage_per_unit', usagePerUnit: completionPrice },
      },
    ],
  };
}

function priceSummary(rule: BillingPriceRule) {
  const unitPrices = rule.price.items
    .filter((item) => item.pricing.mode === 'usage_per_unit' && item.pricing.usagePerUnit)
    .map((item) => `${item.itemCode}: ${item.pricing.usagePerUnit}`);
  return unitPrices.length > 0 ? unitPrices.join(' / ') : rule.price.items.map((item) => item.itemCode).join(' / ');
}

function priceFormFromRule(rule: BillingPriceRule): PriceForm {
  const prompt = rule.price.items.find((item) => item.itemCode === 'prompt_tokens')?.pricing.usagePerUnit || '';
  const completion = rule.price.items.find((item) => item.itemCode === 'completion_tokens')?.pricing.usagePerUnit || '';
  return {
    id: rule.id,
    scopeType: rule.scopeType === 'project' ? 'project' : 'global',
    scopeId: String(rule.scopeID),
    modelPattern: rule.modelPattern,
    promptPrice: prompt,
    completionPrice: completion,
    currency: rule.currency,
    priority: String(rule.priority),
    enabled: rule.enabled,
  };
}

function defaultPriceForm(): PriceForm {
  return {
    scopeType: 'global',
    scopeId: '0',
    modelPattern: '*',
    promptPrice: '0.01',
    completionPrice: '0.02',
    currency: 'CNY',
    priority: '100',
    enabled: true,
  };
}

export default function AdminBillingPage() {
  const { t, i18n } = useTranslation();
  const { data, isLoading, isFetching, error, refetch } = useAdminBillingOverview(20);
  const { data: usersData } = useUsers({ first: 50, orderBy: { field: 'CREATED_AT', direction: 'DESC' } });
  const [selectedUserID, setSelectedUserID] = useState('');
  const [adjustDirection, setAdjustDirection] = useState<LedgerTransactionDirection>('credit');
  const [adjustAmount, setAdjustAmount] = useState('10.00');
  const [adjustMemo, setAdjustMemo] = useState('manual wallet adjustment');
  const [priceForm, setPriceForm] = useState<PriceForm>(() => defaultPriceForm());
  const [providerForm, setProviderForm] = useState({
    name: 'Default ePay',
    status: 'enabled' as 'enabled' | 'disabled',
    currency: 'CNY',
    gatewayUrl: '',
    pid: '',
    key: '',
    notifyUrl: '',
    returnUrl: '',
    type: 'alipay',
    siteName: 'AxonHub',
  });

  const users = useMemo(() => usersData?.edges?.map((edge) => edge.node) ?? [], [usersData?.edges]);
  const selectedUser = users.find((user) => user.id === selectedUserID);
  const selectedUserBilling = useAdminUserBillingDetail(selectedUserID || undefined, 10);
  const adjustBalance = useAdjustUserBalance();
  const savePriceRule = useSaveBillingPriceRule();
  const upsertEPay = useUpsertEPayPaymentProvider();

  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';
  const currency = selectedUserBilling.data?.account.currency || data?.accounts[0]?.currency || 'CNY';
  const formatCurrency = useMemo(
    () =>
      new Intl.NumberFormat(locale, {
        style: 'currency',
        currency,
        currencyDisplay: 'narrowSymbol',
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      }),
    [currency, locale]
  );

  const userEmailByID = useMemo(() => {
    const map = new Map<number, string>();
    users.forEach((user) => map.set(extractNumberIDAsNumber(user.id), user.email));
    return map;
  }, [users]);

  async function handleAdjustBalance(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedUserID) {
      toast.error(t('adminBilling.adjust.selectUserRequired'));
      return;
    }
    const amount = normalizeAmount(adjustAmount, 2);
    if (!amount) {
      toast.error(t('adminBilling.adjust.invalidAmount'));
      return;
    }

    try {
      await adjustBalance.mutateAsync({
        userId: selectedUserID,
        direction: adjustDirection,
        amount,
        currency,
        memo: adjustMemo.trim() || undefined,
      });
      toast.success(t('adminBilling.adjust.success'));
      setAdjustAmount('10.00');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

  async function handleSavePriceRule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const promptPrice = normalizeAmount(priceForm.promptPrice, 6);
    const completionPrice = normalizeAmount(priceForm.completionPrice, 6);
    if (!promptPrice || !completionPrice) {
      toast.error(t('adminBilling.pricing.invalidPrice'));
      return;
    }
    const scopeId = Number(priceForm.scopeId);
    const priority = Number(priceForm.priority);
    if (!Number.isInteger(scopeId) || scopeId < 0 || !Number.isInteger(priority)) {
      toast.error(t('adminBilling.pricing.invalidScope'));
      return;
    }

    try {
      await savePriceRule.mutateAsync({
        id: priceForm.id,
        scopeType: priceForm.scopeType,
        scopeId,
        modelPattern: priceForm.modelPattern.trim() || '*',
        price: modelPriceFromForm(promptPrice, completionPrice),
        currency: priceForm.currency.trim() || 'CNY',
        priority,
        enabled: priceForm.enabled,
      });
      toast.success(t('adminBilling.pricing.success'));
      setPriceForm(defaultPriceForm());
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

  async function handleSaveEPayProvider(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    try {
      await upsertEPay.mutateAsync({
        ...providerForm,
        key: providerForm.key.trim() || undefined,
      });
      toast.success(t('adminBilling.epay.success'));
      setProviderForm((prev) => ({ ...prev, key: '' }));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between gap-4'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('adminBilling.title')}</h2>
            <p className='text-muted-foreground text-sm'>{t('adminBilling.description')}</p>
          </div>
          <Button variant='outline' size='sm' onClick={() => void refetch()} disabled={isFetching}>
            {isFetching ? <Loader2 className='size-4 animate-spin' /> : <RefreshCw className='size-4' />}
            {t('common.refresh')}
          </Button>
        </div>
      </Header>

      <Main fixed className='flex flex-col gap-4 overflow-auto'>
        {error && (
          <Alert variant='destructive'>
            <AlertCircle className='size-4' />
            <AlertTitle>{t('common.loadError')}</AlertTitle>
            <AlertDescription>{error instanceof Error ? error.message : t('common.errors.unknownError')}</AlertDescription>
          </Alert>
        )}

        <div className='grid gap-4 md:grid-cols-3'>
          <MetricCard title={t('adminBilling.metrics.accounts')} value={String(data?.accounts.length ?? 0)} loading={isLoading} />
          <MetricCard title={t('adminBilling.metrics.providers')} value={String(data?.providers.length ?? 0)} loading={isLoading} />
          <MetricCard title={t('adminBilling.metrics.priceRules')} value={String(data?.priceRules.length ?? 0)} loading={isLoading} />
        </div>

        <div className='grid gap-4 xl:grid-cols-[minmax(0,1fr)_420px]'>
          <Card className='rounded-lg'>
            <CardHeader>
              <CardTitle className='text-base'>{t('adminBilling.accounts.title')}</CardTitle>
              <CardDescription>{t('adminBilling.accounts.description')}</CardDescription>
            </CardHeader>
            <CardContent className='overflow-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('adminBilling.columns.owner')}</TableHead>
                    <TableHead>{t('adminBilling.columns.status')}</TableHead>
                    <TableHead>{t('adminBilling.columns.credit')}</TableHead>
                    <TableHead className='text-right'>{t('adminBilling.columns.balance')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(data?.accounts ?? []).length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={4} className='text-muted-foreground h-24 text-center'>
                        {isLoading ? t('common.loading') : t('common.noData')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    data?.accounts.map((account) => (
                      <TableRow key={account.id}>
                        <TableCell>
                          <div className='font-medium'>{userEmailByID.get(account.ownerID) || `${account.ownerType}:${account.ownerID}`}</div>
                          <div className='text-muted-foreground text-xs'>{account.ownerType}</div>
                        </TableCell>
                        <TableCell>
                          <Badge variant={account.status === 'active' ? 'default' : 'destructive'}>{account.status}</Badge>
                        </TableCell>
                        <TableCell className='font-mono'>{formatCurrency.format(microsToAmount(account.creditLimitMicros))}</TableCell>
                        <TableCell className='text-right font-mono'>{formatCurrency.format(microsToAmount(account.balanceMicros))}</TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          <Card className='rounded-lg'>
            <CardHeader>
              <CardTitle className='flex items-center gap-2 text-base'>
                <WalletCards className='size-4' />
                {t('adminBilling.adjust.title')}
              </CardTitle>
              <CardDescription>{t('adminBilling.adjust.description')}</CardDescription>
            </CardHeader>
            <CardContent>
              <form className='space-y-4' onSubmit={handleAdjustBalance}>
                <div className='space-y-2'>
                  <Label>{t('adminBilling.adjust.user')}</Label>
                  <Select value={selectedUserID} onValueChange={setSelectedUserID}>
                    <SelectTrigger>
                      <SelectValue placeholder={t('adminBilling.adjust.userPlaceholder')} />
                    </SelectTrigger>
                    <SelectContent>
                      {users.map((user) => (
                        <SelectItem key={user.id} value={user.id}>
                          {user.email}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className='grid grid-cols-2 gap-3'>
                  <div className='space-y-2'>
                    <Label>{t('adminBilling.adjust.direction')}</Label>
                    <Select value={adjustDirection} onValueChange={(value) => setAdjustDirection(value as LedgerTransactionDirection)}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='credit'>{t('adminBilling.adjust.credit')}</SelectItem>
                        <SelectItem value='debit'>{t('adminBilling.adjust.debit')}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className='space-y-2'>
                    <Label htmlFor='admin-billing-adjust-amount'>{t('adminBilling.adjust.amount')}</Label>
                    <Input id='admin-billing-adjust-amount' inputMode='decimal' value={adjustAmount} onChange={(event) => setAdjustAmount(event.target.value)} />
                  </div>
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='admin-billing-adjust-memo'>{t('adminBilling.adjust.memo')}</Label>
                  <Input id='admin-billing-adjust-memo' value={adjustMemo} onChange={(event) => setAdjustMemo(event.target.value)} />
                </div>
                {selectedUserBilling.data?.account && (
                  <div className='bg-muted/50 rounded-md border p-3 text-sm'>
                    <div className='text-muted-foreground'>{selectedUser?.email}</div>
                    <div className='font-mono text-lg font-semibold'>
                      {formatCurrency.format(microsToAmount(selectedUserBilling.data.account.balanceMicros))}
                    </div>
                  </div>
                )}
                <Button type='submit' className='w-full' disabled={adjustBalance.isPending}>
                  {adjustBalance.isPending ? <Loader2 className='size-4 animate-spin' /> : <Save className='size-4' />}
                  {t('adminBilling.adjust.submit')}
                </Button>
              </form>
            </CardContent>
          </Card>
        </div>

        <div className='grid gap-4 xl:grid-cols-2'>
          <Card className='rounded-lg'>
            <CardHeader>
              <CardTitle className='text-base'>{t('adminBilling.pricing.title')}</CardTitle>
              <CardDescription>{t('adminBilling.pricing.description')}</CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              <form className='grid gap-3 md:grid-cols-2' onSubmit={handleSavePriceRule}>
                <div className='space-y-2'>
                  <Label>{t('adminBilling.pricing.scopeType')}</Label>
                  <Select
                    value={priceForm.scopeType}
                    onValueChange={(value) =>
                      setPriceForm((prev) => ({ ...prev, scopeType: value as PriceForm['scopeType'], scopeId: value === 'global' ? '0' : prev.scopeId }))
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='global'>{t('adminBilling.pricing.global')}</SelectItem>
                      <SelectItem value='project'>{t('adminBilling.pricing.project')}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='admin-billing-scope-id'>{t('adminBilling.pricing.scopeId')}</Label>
                  <Input
                    id='admin-billing-scope-id'
                    value={priceForm.scopeId}
                    disabled={priceForm.scopeType === 'global'}
                    onChange={(event) => setPriceForm((prev) => ({ ...prev, scopeId: event.target.value }))}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='admin-billing-model-pattern'>{t('adminBilling.pricing.modelPattern')}</Label>
                  <Input
                    id='admin-billing-model-pattern'
                    value={priceForm.modelPattern}
                    onChange={(event) => setPriceForm((prev) => ({ ...prev, modelPattern: event.target.value }))}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='admin-billing-priority'>{t('adminBilling.pricing.priority')}</Label>
                  <Input
                    id='admin-billing-priority'
                    inputMode='numeric'
                    value={priceForm.priority}
                    onChange={(event) => setPriceForm((prev) => ({ ...prev, priority: event.target.value }))}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='admin-billing-prompt-price'>{t('adminBilling.pricing.promptPrice')}</Label>
                  <Input
                    id='admin-billing-prompt-price'
                    inputMode='decimal'
                    value={priceForm.promptPrice}
                    onChange={(event) => setPriceForm((prev) => ({ ...prev, promptPrice: event.target.value }))}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='admin-billing-completion-price'>{t('adminBilling.pricing.completionPrice')}</Label>
                  <Input
                    id='admin-billing-completion-price'
                    inputMode='decimal'
                    value={priceForm.completionPrice}
                    onChange={(event) => setPriceForm((prev) => ({ ...prev, completionPrice: event.target.value }))}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='admin-billing-price-currency'>{t('adminBilling.columns.currency')}</Label>
                  <Input
                    id='admin-billing-price-currency'
                    value={priceForm.currency}
                    onChange={(event) => setPriceForm((prev) => ({ ...prev, currency: event.target.value.toUpperCase() }))}
                  />
                </div>
                <div className='flex items-center justify-between rounded-md border px-3 py-2'>
                  <Label htmlFor='admin-billing-price-enabled'>{t('adminBilling.pricing.enabled')}</Label>
                  <Switch
                    id='admin-billing-price-enabled'
                    checked={priceForm.enabled}
                    onCheckedChange={(checked) => setPriceForm((prev) => ({ ...prev, enabled: checked }))}
                  />
                </div>
                <div className='flex gap-2 md:col-span-2'>
                  <Button type='submit' disabled={savePriceRule.isPending}>
                    {savePriceRule.isPending ? <Loader2 className='size-4 animate-spin' /> : <Save className='size-4' />}
                    {priceForm.id ? t('adminBilling.pricing.update') : t('adminBilling.pricing.create')}
                  </Button>
                  <Button type='button' variant='outline' onClick={() => setPriceForm(defaultPriceForm())}>
                    {t('adminBilling.pricing.newRule')}
                  </Button>
                </div>
              </form>

              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('adminBilling.columns.scope')}</TableHead>
                    <TableHead>{t('adminBilling.columns.model')}</TableHead>
                    <TableHead>{t('adminBilling.columns.price')}</TableHead>
                    <TableHead>{t('adminBilling.columns.status')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(data?.priceRules ?? []).length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={4} className='text-muted-foreground h-24 text-center'>
                        {isLoading ? t('common.loading') : t('common.noData')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    data?.priceRules.map((rule) => (
                      <TableRow key={rule.id} className='cursor-pointer' onClick={() => setPriceForm(priceFormFromRule(rule))}>
                        <TableCell>{`${rule.scopeType}:${rule.scopeID}`}</TableCell>
                        <TableCell className='font-mono text-xs'>{rule.modelPattern}</TableCell>
                        <TableCell className='max-w-[260px] truncate font-mono text-xs'>{priceSummary(rule)}</TableCell>
                        <TableCell>
                          <Badge variant={rule.enabled ? 'default' : 'secondary'}>{rule.enabled ? 'enabled' : 'disabled'}</Badge>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          <Card className='rounded-lg'>
            <CardHeader>
              <CardTitle className='text-base'>{t('adminBilling.epay.title')}</CardTitle>
              <CardDescription>{t('adminBilling.epay.description')}</CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              <form className='grid gap-3 md:grid-cols-2' onSubmit={handleSaveEPayProvider}>
                <div className='space-y-2'>
                  <Label htmlFor='admin-billing-epay-name'>{t('adminBilling.epay.name')}</Label>
                  <Input id='admin-billing-epay-name' value={providerForm.name} onChange={(event) => setProviderForm((prev) => ({ ...prev, name: event.target.value }))} />
                </div>
                <div className='space-y-2'>
                  <Label>{t('adminBilling.columns.status')}</Label>
                  <Select value={providerForm.status} onValueChange={(value) => setProviderForm((prev) => ({ ...prev, status: value as 'enabled' | 'disabled' }))}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='enabled'>enabled</SelectItem>
                      <SelectItem value='disabled'>disabled</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-2 md:col-span-2'>
                  <Label htmlFor='admin-billing-epay-gateway'>{t('adminBilling.epay.gatewayUrl')}</Label>
                  <Input id='admin-billing-epay-gateway' value={providerForm.gatewayUrl} onChange={(event) => setProviderForm((prev) => ({ ...prev, gatewayUrl: event.target.value }))} />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='admin-billing-epay-pid'>{t('adminBilling.epay.pid')}</Label>
                  <Input id='admin-billing-epay-pid' value={providerForm.pid} onChange={(event) => setProviderForm((prev) => ({ ...prev, pid: event.target.value }))} />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='admin-billing-epay-key'>{t('adminBilling.epay.key')}</Label>
                  <Input id='admin-billing-epay-key' value={providerForm.key} onChange={(event) => setProviderForm((prev) => ({ ...prev, key: event.target.value }))} />
                </div>
                <div className='space-y-2 md:col-span-2'>
                  <Label htmlFor='admin-billing-epay-notify'>{t('adminBilling.epay.notifyUrl')}</Label>
                  <Input id='admin-billing-epay-notify' value={providerForm.notifyUrl} onChange={(event) => setProviderForm((prev) => ({ ...prev, notifyUrl: event.target.value }))} />
                </div>
                <div className='space-y-2 md:col-span-2'>
                  <Label htmlFor='admin-billing-epay-return'>{t('adminBilling.epay.returnUrl')}</Label>
                  <Input id='admin-billing-epay-return' value={providerForm.returnUrl} onChange={(event) => setProviderForm((prev) => ({ ...prev, returnUrl: event.target.value }))} />
                </div>
                <Button type='submit' disabled={upsertEPay.isPending}>
                  {upsertEPay.isPending ? <Loader2 className='size-4 animate-spin' /> : <Save className='size-4' />}
                  {t('adminBilling.epay.submit')}
                </Button>
              </form>

              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('adminBilling.epay.name')}</TableHead>
                    <TableHead>{t('adminBilling.columns.status')}</TableHead>
                    <TableHead>{t('adminBilling.columns.currency')}</TableHead>
                    <TableHead>{t('adminBilling.columns.updatedAt')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(data?.providers ?? []).length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={4} className='text-muted-foreground h-24 text-center'>
                        {isLoading ? t('common.loading') : t('common.noData')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    data?.providers.map((provider) => (
                      <TableRow key={provider.id}>
                        <TableCell>{provider.name}</TableCell>
                        <TableCell>
                          <Badge variant={provider.status === 'enabled' ? 'default' : 'secondary'}>{provider.status}</Badge>
                        </TableCell>
                        <TableCell>{provider.currency}</TableCell>
                        <TableCell>{formatDate(provider.updatedAt)}</TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      </Main>
    </div>
  );
}

function MetricCard({ title, value, loading }: { title: string; value: string; loading: boolean }) {
  return (
    <Card className='rounded-lg'>
      <CardHeader className='pb-2'>
        <CardTitle className='text-muted-foreground text-sm font-medium'>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className='font-mono text-2xl font-semibold'>{loading ? '-' : value}</div>
      </CardContent>
    </Card>
  );
}
