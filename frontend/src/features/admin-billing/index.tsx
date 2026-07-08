import { FormEvent, ReactNode, useEffect, useMemo, useState } from 'react';
import { AlertCircle, Loader2, RefreshCw, Save, Unlock, WalletCards } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { extractNumberIDAsNumber } from '@/lib/utils';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { useUsers } from '@/features/users/data/users';
import {
  type AdminLedgerTransactionsFilter,
  type AdminBillingHoldsFilter,
  type AdminPaymentEventsFilter,
  type AdminPaymentOrdersFilter,
  type AdminUsageBillingRecordsFilter,
  type BillingHoldStatus,
  type BillingAccountStatus,
  type BillingPriceRule,
  type LedgerTransactionDirection,
  type ModelPrice,
  type PaymentEventStatus,
  type PaymentOrderStatus,
  type PaymentProviderType,
  type UsageBillingRecordStatus,
  useAdjustUserBalance,
  useAdminBillingOverview,
  useAdminBillingHolds,
  useAdminLedgerTransactions,
  useAdminPaymentEvents,
  useAdminPaymentOrders,
  useAdminUsageBillingRecords,
  useAdminUserBillingDetail,
  useReleaseBillingHold,
  useSaveBillingPriceRule,
  useUpdateUserBillingAccount,
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

type LedgerFilterForm = {
  userId: string;
  billingAccountId: string;
  direction: 'all' | LedgerTransactionDirection;
  status: 'all' | 'posted' | 'voided';
  type: string;
  referenceType: string;
  from: string;
  to: string;
};

type UsageFilterForm = {
  userId: string;
  projectId: string;
  apiKeyId: string;
  billingAccountId: string;
  modelId: string;
  status: 'all' | UsageBillingRecordStatus;
  from: string;
  to: string;
};

type HoldFilterForm = {
  userId: string;
  projectId: string;
  apiKeyId: string;
  billingAccountId: string;
  modelId: string;
  status: 'all' | BillingHoldStatus;
  from: string;
  to: string;
  expiresBefore: string;
};

type OrderFilterForm = {
  userId: string;
  projectId: string;
  billingAccountId: string;
  providerType: 'all' | PaymentProviderType;
  status: 'all' | PaymentOrderStatus;
  orderNo: string;
  externalTradeNo: string;
  from: string;
  to: string;
};

type EventFilterForm = {
  paymentOrderId: string;
  providerInstanceId: string;
  providerType: 'all' | PaymentProviderType;
  status: 'all' | PaymentEventStatus;
  eventType: string;
  eventKey: string;
  from: string;
  to: string;
};

type EPayProviderForm = {
  name: string;
  status: 'enabled' | 'disabled';
  currency: string;
  gatewayUrl: string;
  pid: string;
  key: string;
  notifyUrl: string;
  returnUrl: string;
  type: string;
  siteName: string;
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

function normalizeNonNegativeAmount(value: string, scale = 6) {
  const trimmed = value.trim();
  const pattern = new RegExp(`^\\d+(\\.\\d{1,${scale}})?$`);
  if (!pattern.test(trimmed)) return '';
  const amount = Number(trimmed);
  if (!Number.isFinite(amount) || amount < 0) return '';
  return trimmed;
}

function optionalInt(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const parsed = Number(trimmed);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined;
}

function optionalText(value: string) {
  const trimmed = value.trim();
  return trimmed || undefined;
}

function optionalTime(value: string) {
  return value ? new Date(value).toISOString() : undefined;
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

function defaultLedgerFilter(): LedgerFilterForm {
  return { userId: '', billingAccountId: '', direction: 'all', status: 'all', type: '', referenceType: '', from: '', to: '' };
}

function defaultUsageFilter(): UsageFilterForm {
  return { userId: '', projectId: '', apiKeyId: '', billingAccountId: '', modelId: '', status: 'all', from: '', to: '' };
}

function defaultHoldFilter(): HoldFilterForm {
  return {
    userId: '',
    projectId: '',
    apiKeyId: '',
    billingAccountId: '',
    modelId: '',
    status: 'all',
    from: '',
    to: '',
    expiresBefore: '',
  };
}

function defaultOrderFilter(): OrderFilterForm {
  return {
    userId: '',
    projectId: '',
    billingAccountId: '',
    providerType: 'all',
    status: 'all',
    orderNo: '',
    externalTradeNo: '',
    from: '',
    to: '',
  };
}

function defaultEventFilter(): EventFilterForm {
  return { paymentOrderId: '', providerInstanceId: '', providerType: 'all', status: 'all', eventType: '', eventKey: '', from: '', to: '' };
}

function buildLedgerFilter(form: LedgerFilterForm): AdminLedgerTransactionsFilter {
  return {
    userId: optionalInt(form.userId),
    billingAccountId: optionalInt(form.billingAccountId),
    direction: form.direction === 'all' ? undefined : form.direction,
    status: form.status === 'all' ? undefined : form.status,
    type: optionalText(form.type) as AdminLedgerTransactionsFilter['type'],
    referenceType: optionalText(form.referenceType),
    from: optionalTime(form.from),
    to: optionalTime(form.to),
  };
}

function buildUsageFilter(form: UsageFilterForm): AdminUsageBillingRecordsFilter {
  return {
    userId: optionalInt(form.userId),
    projectId: optionalInt(form.projectId),
    apiKeyId: optionalInt(form.apiKeyId),
    billingAccountId: optionalInt(form.billingAccountId),
    modelId: optionalText(form.modelId),
    status: form.status === 'all' ? undefined : form.status,
    from: optionalTime(form.from),
    to: optionalTime(form.to),
  };
}

function buildHoldFilter(form: HoldFilterForm): AdminBillingHoldsFilter {
  return {
    userId: optionalInt(form.userId),
    projectId: optionalInt(form.projectId),
    apiKeyId: optionalInt(form.apiKeyId),
    billingAccountId: optionalInt(form.billingAccountId),
    modelId: optionalText(form.modelId),
    status: form.status === 'all' ? undefined : form.status,
    from: optionalTime(form.from),
    to: optionalTime(form.to),
    expiresBefore: optionalTime(form.expiresBefore),
  };
}

function buildOrderFilter(form: OrderFilterForm): AdminPaymentOrdersFilter {
  return {
    userId: optionalInt(form.userId),
    projectId: optionalInt(form.projectId),
    billingAccountId: optionalInt(form.billingAccountId),
    providerType: form.providerType === 'all' ? undefined : form.providerType,
    status: form.status === 'all' ? undefined : form.status,
    orderNo: optionalText(form.orderNo),
    externalTradeNo: optionalText(form.externalTradeNo),
    from: optionalTime(form.from),
    to: optionalTime(form.to),
  };
}

function buildEventFilter(form: EventFilterForm): AdminPaymentEventsFilter {
  return {
    paymentOrderId: optionalInt(form.paymentOrderId),
    providerInstanceId: optionalInt(form.providerInstanceId),
    providerType: form.providerType === 'all' ? undefined : form.providerType,
    status: form.status === 'all' ? undefined : form.status,
    eventType: optionalText(form.eventType),
    eventKey: optionalText(form.eventKey),
    from: optionalTime(form.from),
    to: optionalTime(form.to),
  };
}

export default function AdminBillingPage() {
  const { t, i18n } = useTranslation();
  const { data, isLoading, isFetching, error, refetch } = useAdminBillingOverview(50);
  const { data: usersData } = useUsers({ first: 50, orderBy: { field: 'CREATED_AT', direction: 'DESC' } });
  const [selectedUserID, setSelectedUserID] = useState('');
  const [accountStatus, setAccountStatus] = useState<BillingAccountStatus>('active');
  const [creditLimit, setCreditLimit] = useState('0.00');
  const [adjustDirection, setAdjustDirection] = useState<LedgerTransactionDirection>('credit');
  const [adjustAmount, setAdjustAmount] = useState('10.00');
  const [adjustMemo, setAdjustMemo] = useState('manual wallet adjustment');
  const [priceForm, setPriceForm] = useState<PriceForm>(() => defaultPriceForm());
  const [ledgerFilter, setLedgerFilter] = useState<LedgerFilterForm>(() => defaultLedgerFilter());
  const [usageFilter, setUsageFilter] = useState<UsageFilterForm>(() => defaultUsageFilter());
  const [holdFilter, setHoldFilter] = useState<HoldFilterForm>(() => defaultHoldFilter());
  const [orderFilter, setOrderFilter] = useState<OrderFilterForm>(() => defaultOrderFilter());
  const [eventFilter, setEventFilter] = useState<EventFilterForm>(() => defaultEventFilter());
  const [appliedLedgerFilter, setAppliedLedgerFilter] = useState<AdminLedgerTransactionsFilter>({});
  const [appliedUsageFilter, setAppliedUsageFilter] = useState<AdminUsageBillingRecordsFilter>({});
  const [appliedHoldFilter, setAppliedHoldFilter] = useState<AdminBillingHoldsFilter>({});
  const [appliedOrderFilter, setAppliedOrderFilter] = useState<AdminPaymentOrdersFilter>({});
  const [appliedEventFilter, setAppliedEventFilter] = useState<AdminPaymentEventsFilter>({});
  const [holdReleaseReasons, setHoldReleaseReasons] = useState<Record<string, string>>({});
  const [providerForm, setProviderForm] = useState<EPayProviderForm>({
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
  const selectedUserNumericID = selectedUserID ? extractNumberIDAsNumber(selectedUserID) : 0;
  const selectedUserBilling = useAdminUserBillingDetail(selectedUserID || undefined, 10);
  const adminLedger = useAdminLedgerTransactions(appliedLedgerFilter, 50);
  const adminUsage = useAdminUsageBillingRecords(appliedUsageFilter, 50);
  const adminHolds = useAdminBillingHolds(appliedHoldFilter, 50);
  const adminOrders = useAdminPaymentOrders(appliedOrderFilter, 50);
  const adminEvents = useAdminPaymentEvents(appliedEventFilter, 50);
  const adjustBalance = useAdjustUserBalance();
  const updateAccount = useUpdateUserBillingAccount();
  const releaseHold = useReleaseBillingHold();
  const savePriceRule = useSaveBillingPriceRule();
  const upsertEPay = useUpsertEPayPaymentProvider();

  useEffect(() => {
    const account = selectedUserBilling.data?.account;
    if (!account) return;
    setAccountStatus(account.status);
    setCreditLimit(microsToAmount(account.creditLimitMicros).toFixed(2));
  }, [selectedUserBilling.data?.account]);

  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';
  const currency = selectedUserBilling.data?.account.currency || data?.accounts[0]?.currency || 'CNY';
  const formatMicros = (value: number, valueCurrency = currency, minimumFractionDigits = 2) =>
    t('currencies.format', {
      val: microsToAmount(value),
      currency: valueCurrency,
      locale,
      minimumFractionDigits,
      maximumFractionDigits: Math.max(minimumFractionDigits, 6),
    });

  const userEmailByID = useMemo(() => {
    const map = new Map<number, string>();
    users.forEach((user) => map.set(extractNumberIDAsNumber(user.id), user.email));
    return map;
  }, [users]);

  async function refreshAll() {
    await Promise.all([
      refetch(),
      adminLedger.refetch(),
      adminUsage.refetch(),
      adminHolds.refetch(),
      adminOrders.refetch(),
      adminEvents.refetch(),
      selectedUserBilling.refetch(),
    ]);
  }

  async function handleReleaseHold(holdId: string) {
    const reason = (holdReleaseReasons[holdId] || t('adminBilling.holds.defaultReleaseReason')).trim();
    if (!reason) {
      toast.error(t('adminBilling.holds.reasonRequired'));
      return;
    }

    try {
      await releaseHold.mutateAsync({ id: holdId, reason });
      toast.success(t('adminBilling.holds.releaseSuccess'));
      setHoldReleaseReasons((prev) => ({ ...prev, [holdId]: '' }));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

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

  async function handleUpdateAccount(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedUserID) {
      toast.error(t('adminBilling.adjust.selectUserRequired'));
      return;
    }
    const normalizedCreditLimit = normalizeNonNegativeAmount(creditLimit, 2);
    if (!normalizedCreditLimit) {
      toast.error(t('adminBilling.account.invalidCreditLimit'));
      return;
    }

    try {
      await updateAccount.mutateAsync({
        userId: selectedUserID,
        status: accountStatus,
        creditLimit: normalizedCreditLimit,
      });
      toast.success(t('adminBilling.account.success'));
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
          <Button
            variant='outline'
            size='sm'
            onClick={() => void refreshAll()}
            disabled={
              isFetching ||
              adminLedger.isFetching ||
              adminUsage.isFetching ||
              adminHolds.isFetching ||
              adminOrders.isFetching ||
              adminEvents.isFetching
            }
          >
            {isFetching || adminLedger.isFetching || adminUsage.isFetching || adminHolds.isFetching || adminOrders.isFetching || adminEvents.isFetching ? (
              <Loader2 className='size-4 animate-spin' />
            ) : (
              <RefreshCw className='size-4' />
            )}
            {t('common.refresh')}
          </Button>
        </div>
      </Header>

      <Main fixed className='flex flex-col gap-4 overflow-auto'>
        <ErrorAlert error={error || adminLedger.error || adminUsage.error || adminHolds.error || adminOrders.error || adminEvents.error} />

        <div className='grid gap-4 md:grid-cols-6'>
          <MetricCard title={t('adminBilling.metrics.accounts')} value={String(data?.accounts.length ?? 0)} loading={isLoading} />
          <MetricCard
            title={t('adminBilling.metrics.ledger')}
            value={String(adminLedger.data?.length ?? 0)}
            loading={adminLedger.isLoading}
          />
          <MetricCard title={t('adminBilling.metrics.usage')} value={String(adminUsage.data?.length ?? 0)} loading={adminUsage.isLoading} />
          <MetricCard title={t('adminBilling.metrics.holds')} value={String(adminHolds.data?.length ?? 0)} loading={adminHolds.isLoading} />
          <MetricCard
            title={t('adminBilling.metrics.orders')}
            value={String(adminOrders.data?.length ?? 0)}
            loading={adminOrders.isLoading}
          />
          <MetricCard
            title={t('adminBilling.metrics.events')}
            value={String(adminEvents.data?.length ?? 0)}
            loading={adminEvents.isLoading}
          />
        </div>

        <Tabs defaultValue='wallets' className='gap-4'>
          <TabsList className='shadow-soft border-border bg-background flex h-auto w-full justify-start overflow-x-auto rounded-lg border p-1'>
            <TabsTrigger value='wallets'>{t('adminBilling.tabs.wallets')}</TabsTrigger>
            <TabsTrigger value='ledger'>{t('adminBilling.tabs.ledger')}</TabsTrigger>
            <TabsTrigger value='usage'>{t('adminBilling.tabs.usage')}</TabsTrigger>
            <TabsTrigger value='holds'>{t('adminBilling.tabs.holds')}</TabsTrigger>
            <TabsTrigger value='orders'>{t('adminBilling.tabs.orders')}</TabsTrigger>
            <TabsTrigger value='events'>{t('adminBilling.tabs.events')}</TabsTrigger>
            <TabsTrigger value='pricing'>{t('adminBilling.tabs.pricing')}</TabsTrigger>
            <TabsTrigger value='providers'>{t('adminBilling.tabs.providers')}</TabsTrigger>
          </TabsList>

          <TabsContent value='wallets' className='mt-0'>
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
                        <TableHead>{t('adminBilling.columns.held')}</TableHead>
                        <TableHead>{t('adminBilling.columns.available')}</TableHead>
                        <TableHead className='text-right'>{t('adminBilling.columns.balance')}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      <DataStateRow colSpan={6} isLoading={isLoading} isEmpty={(data?.accounts ?? []).length === 0} />
                      {data?.accounts.map((account) => (
                        <TableRow key={account.id}>
                          <TableCell>
                            <div className='font-medium'>
                              {userEmailByID.get(account.ownerID) || `${account.ownerType}:${account.ownerID}`}
                            </div>
                            <div className='text-muted-foreground text-xs'>{account.ownerType}</div>
                          </TableCell>
                          <TableCell>
                            <StatusBadge value={account.status} positive={account.status === 'active'} />
                          </TableCell>
                          <TableCell className='font-mono'>{formatMicros(account.creditLimitMicros, account.currency)}</TableCell>
                          <TableCell className='font-mono'>{formatMicros(account.heldBalanceMicros, account.currency)}</TableCell>
                          <TableCell className='font-mono'>
                            {formatMicros(account.balanceMicros + account.creditLimitMicros - account.heldBalanceMicros, account.currency)}
                          </TableCell>
                          <TableCell className='text-right font-mono'>{formatMicros(account.balanceMicros, account.currency)}</TableCell>
                        </TableRow>
                      ))}
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
                <CardContent className='space-y-5'>
                  <UserSelect
                    users={users}
                    value={selectedUserID}
                    onChange={setSelectedUserID}
                    placeholder={t('adminBilling.adjust.userPlaceholder')}
                    label={t('adminBilling.adjust.user')}
                  />

                  {selectedUserBilling.data?.account && (
                    <div className='bg-muted/50 rounded-md border p-3 text-sm'>
                      <div className='text-muted-foreground'>{selectedUser?.email}</div>
                      <div className='font-mono text-lg font-semibold'>
                        {formatMicros(selectedUserBilling.data.account.balanceMicros, selectedUserBilling.data.account.currency)}
                      </div>
                      <div className='text-muted-foreground mt-1 text-xs'>
                        {t('adminBilling.account.creditLimit')}:{' '}
                        {formatMicros(selectedUserBilling.data.account.creditLimitMicros, selectedUserBilling.data.account.currency)}
                      </div>
                      <div className='text-muted-foreground mt-1 text-xs'>
                        {t('adminBilling.account.held')}:{' '}
                        {formatMicros(selectedUserBilling.data.account.heldBalanceMicros, selectedUserBilling.data.account.currency)}
                      </div>
                    </div>
                  )}

                  <form className='space-y-4' onSubmit={handleUpdateAccount}>
                    <div className='text-sm font-medium'>{t('adminBilling.account.title')}</div>
                    <div className='grid grid-cols-2 gap-3'>
                      <div className='space-y-2'>
                        <Label>{t('adminBilling.account.status')}</Label>
                        <Select
                          value={accountStatus}
                          onValueChange={(value) => setAccountStatus(value as BillingAccountStatus)}
                          disabled={!selectedUserID}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value='active'>{t('adminBilling.account.active')}</SelectItem>
                            <SelectItem value='frozen'>{t('adminBilling.account.frozen')}</SelectItem>
                            <SelectItem value='closed'>{t('adminBilling.account.closed')}</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div className='space-y-2'>
                        <Label htmlFor='admin-billing-credit-limit'>{t('adminBilling.account.creditLimit')}</Label>
                        <Input
                          id='admin-billing-credit-limit'
                          inputMode='decimal'
                          value={creditLimit}
                          disabled={!selectedUserID}
                          onChange={(event) => setCreditLimit(event.target.value)}
                        />
                      </div>
                    </div>
                    <Button type='submit' className='w-full' variant='outline' disabled={updateAccount.isPending || !selectedUserID}>
                      {updateAccount.isPending ? <Loader2 className='size-4 animate-spin' /> : <Save className='size-4' />}
                      {t('adminBilling.account.submit')}
                    </Button>
                  </form>

                  <form className='space-y-4 border-t pt-5' onSubmit={handleAdjustBalance}>
                    <div className='text-sm font-medium'>{t('adminBilling.adjust.sectionTitle')}</div>
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
                        <Input
                          id='admin-billing-adjust-amount'
                          inputMode='decimal'
                          value={adjustAmount}
                          onChange={(event) => setAdjustAmount(event.target.value)}
                        />
                      </div>
                    </div>
                    <div className='space-y-2'>
                      <Label htmlFor='admin-billing-adjust-memo'>{t('adminBilling.adjust.memo')}</Label>
                      <Input id='admin-billing-adjust-memo' value={adjustMemo} onChange={(event) => setAdjustMemo(event.target.value)} />
                    </div>
                    <Button type='submit' className='w-full' disabled={adjustBalance.isPending || !selectedUserID}>
                      {adjustBalance.isPending ? <Loader2 className='size-4 animate-spin' /> : <Save className='size-4' />}
                      {t('adminBilling.adjust.submit')}
                    </Button>
                  </form>
                </CardContent>
              </Card>
            </div>

            {selectedUserID && (
              <Card className='mt-4 rounded-lg'>
                <CardHeader>
                  <CardTitle className='text-base'>{t('adminBilling.userDetail.title')}</CardTitle>
                  <CardDescription>{selectedUser?.email || selectedUserNumericID}</CardDescription>
                </CardHeader>
                <CardContent className='grid gap-4 xl:grid-cols-3'>
                  <MiniList
                    title={t('adminBilling.tabs.ledger')}
                    isLoading={selectedUserBilling.isLoading}
                    empty={(selectedUserBilling.data?.ledgerTransactions ?? []).length === 0}
                  >
                    {selectedUserBilling.data?.ledgerTransactions.map((tx) => (
                      <MiniRow
                        key={tx.id}
                        left={tx.type}
                        right={`${tx.direction === 'debit' ? '-' : '+'}${formatMicros(tx.amountMicros, tx.currency)}`}
                        sub={formatDate(tx.createdAt)}
                      />
                    ))}
                  </MiniList>
                  <MiniList
                    title={t('adminBilling.tabs.orders')}
                    isLoading={selectedUserBilling.isLoading}
                    empty={(selectedUserBilling.data?.paymentOrders ?? []).length === 0}
                  >
                    {selectedUserBilling.data?.paymentOrders.map((order) => (
                      <MiniRow
                        key={order.id}
                        left={order.orderNo}
                        right={formatMicros(order.amountMicros, order.currency)}
                        sub={order.status}
                      />
                    ))}
                  </MiniList>
                  <MiniList
                    title={t('adminBilling.tabs.usage')}
                    isLoading={selectedUserBilling.isLoading}
                    empty={(selectedUserBilling.data?.usageBillingRecords ?? []).length === 0}
                  >
                    {selectedUserBilling.data?.usageBillingRecords.map((record) => (
                      <MiniRow
                        key={record.id}
                        left={record.modelID}
                        right={formatMicros(record.chargeAmountMicros, record.currency)}
                        sub={record.error || record.status}
                      />
                    ))}
                  </MiniList>
                </CardContent>
              </Card>
            )}
          </TabsContent>

          <TabsContent value='ledger' className='mt-0'>
            <Card className='rounded-lg'>
              <CardHeader>
                <CardTitle className='text-base'>{t('adminBilling.ledger.title')}</CardTitle>
                <CardDescription>{t('adminBilling.ledger.description')}</CardDescription>
              </CardHeader>
              <CardContent className='space-y-4'>
                <form
                  className='grid gap-3 md:grid-cols-4 xl:grid-cols-8'
                  onSubmit={(event) => {
                    event.preventDefault();
                    setAppliedLedgerFilter(buildLedgerFilter(ledgerFilter));
                  }}
                >
                  <FilterInput
                    label={t('adminBilling.filters.userId')}
                    value={ledgerFilter.userId}
                    onChange={(value) => setLedgerFilter((prev) => ({ ...prev, userId: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.accountId')}
                    value={ledgerFilter.billingAccountId}
                    onChange={(value) => setLedgerFilter((prev) => ({ ...prev, billingAccountId: value }))}
                  />
                  <FilterSelect
                    label={t('adminBilling.adjust.direction')}
                    value={ledgerFilter.direction}
                    onChange={(value) => setLedgerFilter((prev) => ({ ...prev, direction: value as LedgerFilterForm['direction'] }))}
                    options={['all', 'credit', 'debit']}
                  />
                  <FilterSelect
                    label={t('adminBilling.columns.status')}
                    value={ledgerFilter.status}
                    onChange={(value) => setLedgerFilter((prev) => ({ ...prev, status: value as LedgerFilterForm['status'] }))}
                    options={['all', 'posted', 'voided']}
                  />
                  <FilterInput
                    label={t('adminBilling.columns.type')}
                    value={ledgerFilter.type}
                    onChange={(value) => setLedgerFilter((prev) => ({ ...prev, type: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.columns.reference')}
                    value={ledgerFilter.referenceType}
                    onChange={(value) => setLedgerFilter((prev) => ({ ...prev, referenceType: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.from')}
                    type='datetime-local'
                    value={ledgerFilter.from}
                    onChange={(value) => setLedgerFilter((prev) => ({ ...prev, from: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.to')}
                    type='datetime-local'
                    value={ledgerFilter.to}
                    onChange={(value) => setLedgerFilter((prev) => ({ ...prev, to: value }))}
                  />
                  <FilterActions
                    onReset={() => {
                      const next = defaultLedgerFilter();
                      setLedgerFilter(next);
                      setAppliedLedgerFilter({});
                    }}
                  />
                </form>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('adminBilling.columns.createdAt')}</TableHead>
                      <TableHead>{t('adminBilling.filters.accountId')}</TableHead>
                      <TableHead>{t('adminBilling.columns.type')}</TableHead>
                      <TableHead>{t('adminBilling.adjust.direction')}</TableHead>
                      <TableHead>{t('adminBilling.columns.reference')}</TableHead>
                      <TableHead className='text-right'>{t('adminBilling.columns.amount')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <DataStateRow colSpan={6} isLoading={adminLedger.isLoading} isEmpty={(adminLedger.data ?? []).length === 0} />
                    {adminLedger.data?.map((tx) => (
                      <TableRow key={tx.id}>
                        <TableCell>{formatDate(tx.createdAt)}</TableCell>
                        <TableCell className='font-mono text-xs'>{tx.billingAccountID}</TableCell>
                        <TableCell>{tx.type}</TableCell>
                        <TableCell>
                          <StatusBadge value={tx.direction} positive={tx.direction === 'credit'} />
                        </TableCell>
                        <TableCell className='max-w-[220px] truncate text-xs'>
                          {tx.referenceType || '-'} {tx.referenceID || ''}
                        </TableCell>
                        <TableCell className='text-right font-mono'>
                          {tx.direction === 'debit' ? '-' : '+'}
                          {formatMicros(tx.amountMicros, tx.currency)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value='usage' className='mt-0'>
            <Card className='rounded-lg'>
              <CardHeader>
                <CardTitle className='text-base'>{t('adminBilling.usage.title')}</CardTitle>
                <CardDescription>{t('adminBilling.usage.description')}</CardDescription>
              </CardHeader>
              <CardContent className='space-y-4'>
                <form
                  className='grid gap-3 md:grid-cols-4 xl:grid-cols-8'
                  onSubmit={(event) => {
                    event.preventDefault();
                    setAppliedUsageFilter(buildUsageFilter(usageFilter));
                  }}
                >
                  <FilterInput
                    label={t('adminBilling.filters.userId')}
                    value={usageFilter.userId}
                    onChange={(value) => setUsageFilter((prev) => ({ ...prev, userId: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.projectId')}
                    value={usageFilter.projectId}
                    onChange={(value) => setUsageFilter((prev) => ({ ...prev, projectId: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.apiKeyId')}
                    value={usageFilter.apiKeyId}
                    onChange={(value) => setUsageFilter((prev) => ({ ...prev, apiKeyId: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.columns.model')}
                    value={usageFilter.modelId}
                    onChange={(value) => setUsageFilter((prev) => ({ ...prev, modelId: value }))}
                  />
                  <FilterSelect
                    label={t('adminBilling.columns.status')}
                    value={usageFilter.status}
                    onChange={(value) => setUsageFilter((prev) => ({ ...prev, status: value as UsageFilterForm['status'] }))}
                    options={['all', 'pending', 'charged', 'skipped', 'failed', 'refunded']}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.accountId')}
                    value={usageFilter.billingAccountId}
                    onChange={(value) => setUsageFilter((prev) => ({ ...prev, billingAccountId: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.from')}
                    type='datetime-local'
                    value={usageFilter.from}
                    onChange={(value) => setUsageFilter((prev) => ({ ...prev, from: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.to')}
                    type='datetime-local'
                    value={usageFilter.to}
                    onChange={(value) => setUsageFilter((prev) => ({ ...prev, to: value }))}
                  />
                  <FilterActions
                    onReset={() => {
                      const next = defaultUsageFilter();
                      setUsageFilter(next);
                      setAppliedUsageFilter({});
                    }}
                  />
                </form>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('adminBilling.columns.createdAt')}</TableHead>
                      <TableHead>{t('adminBilling.filters.userId')}</TableHead>
                      <TableHead>{t('adminBilling.filters.projectId')}</TableHead>
                      <TableHead>{t('adminBilling.columns.model')}</TableHead>
                      <TableHead>{t('adminBilling.columns.status')}</TableHead>
                      <TableHead>{t('adminBilling.columns.reason')}</TableHead>
                      <TableHead className='text-right'>{t('adminBilling.columns.amount')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <DataStateRow colSpan={7} isLoading={adminUsage.isLoading} isEmpty={(adminUsage.data ?? []).length === 0} />
                    {adminUsage.data?.map((record) => (
                      <TableRow key={record.id}>
                        <TableCell>{formatDate(record.createdAt)}</TableCell>
                        <TableCell>{record.userID ?? '-'}</TableCell>
                        <TableCell>{record.projectID}</TableCell>
                        <TableCell className='max-w-[220px] truncate font-mono text-xs'>{record.modelID}</TableCell>
                        <TableCell>
                          <StatusBadge value={record.status} positive={record.status === 'charged'} />
                        </TableCell>
                        <TableCell className='max-w-[280px] truncate text-xs'>{record.error || '-'}</TableCell>
                        <TableCell className='text-right font-mono'>{formatMicros(record.chargeAmountMicros, record.currency)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value='holds' className='mt-0'>
            <Card className='rounded-lg'>
              <CardHeader>
                <CardTitle className='text-base'>{t('adminBilling.holds.title')}</CardTitle>
                <CardDescription>{t('adminBilling.holds.description')}</CardDescription>
              </CardHeader>
              <CardContent className='space-y-4 overflow-auto'>
                <form
                  className='grid gap-3 md:grid-cols-4 xl:grid-cols-9'
                  onSubmit={(event) => {
                    event.preventDefault();
                    setAppliedHoldFilter(buildHoldFilter(holdFilter));
                  }}
                >
                  <FilterInput
                    label={t('adminBilling.filters.userId')}
                    value={holdFilter.userId}
                    onChange={(value) => setHoldFilter((prev) => ({ ...prev, userId: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.projectId')}
                    value={holdFilter.projectId}
                    onChange={(value) => setHoldFilter((prev) => ({ ...prev, projectId: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.apiKeyId')}
                    value={holdFilter.apiKeyId}
                    onChange={(value) => setHoldFilter((prev) => ({ ...prev, apiKeyId: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.accountId')}
                    value={holdFilter.billingAccountId}
                    onChange={(value) => setHoldFilter((prev) => ({ ...prev, billingAccountId: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.columns.model')}
                    value={holdFilter.modelId}
                    onChange={(value) => setHoldFilter((prev) => ({ ...prev, modelId: value }))}
                  />
                  <FilterSelect
                    label={t('adminBilling.columns.status')}
                    value={holdFilter.status}
                    onChange={(value) => setHoldFilter((prev) => ({ ...prev, status: value as HoldFilterForm['status'] }))}
                    options={['all', 'held', 'captured', 'released', 'expired']}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.from')}
                    type='datetime-local'
                    value={holdFilter.from}
                    onChange={(value) => setHoldFilter((prev) => ({ ...prev, from: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.to')}
                    type='datetime-local'
                    value={holdFilter.to}
                    onChange={(value) => setHoldFilter((prev) => ({ ...prev, to: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.expiresBefore')}
                    type='datetime-local'
                    value={holdFilter.expiresBefore}
                    onChange={(value) => setHoldFilter((prev) => ({ ...prev, expiresBefore: value }))}
                  />
                  <FilterActions
                    onReset={() => {
                      const next = defaultHoldFilter();
                      setHoldFilter(next);
                      setAppliedHoldFilter({});
                    }}
                  />
                </form>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('adminBilling.columns.createdAt')}</TableHead>
                      <TableHead>{t('adminBilling.filters.accountId')}</TableHead>
                      <TableHead>{t('adminBilling.filters.userId')}</TableHead>
                      <TableHead>{t('adminBilling.filters.projectId')}</TableHead>
                      <TableHead>{t('adminBilling.columns.model')}</TableHead>
                      <TableHead>{t('adminBilling.columns.status')}</TableHead>
                      <TableHead>{t('adminBilling.columns.expiresAt')}</TableHead>
                      <TableHead>{t('adminBilling.columns.reference')}</TableHead>
                      <TableHead className='text-right'>{t('adminBilling.columns.held')}</TableHead>
                      <TableHead className='text-right'>{t('adminBilling.columns.captured')}</TableHead>
                      <TableHead>{t('adminBilling.columns.action')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <DataStateRow colSpan={11} isLoading={adminHolds.isLoading} isEmpty={(adminHolds.data ?? []).length === 0} />
                    {adminHolds.data?.map((hold) => (
                      <TableRow key={hold.id}>
                        <TableCell>{formatDate(hold.createdAt)}</TableCell>
                        <TableCell className='font-mono text-xs'>{hold.billingAccountID}</TableCell>
                        <TableCell>{hold.userID ?? '-'}</TableCell>
                        <TableCell>{hold.projectID ?? '-'}</TableCell>
                        <TableCell className='max-w-[180px] truncate font-mono text-xs'>{hold.modelID || '-'}</TableCell>
                        <TableCell>
                          <StatusBadge value={hold.status} positive={hold.status === 'captured'} />
                        </TableCell>
                        <TableCell>{formatDate(hold.expiresAt)}</TableCell>
                        <TableCell className='max-w-[220px] truncate text-xs'>
                          {hold.releaseReason || `${hold.referenceType || '-'} ${hold.referenceID || ''}`}
                        </TableCell>
                        <TableCell className='text-right font-mono'>{formatMicros(hold.amountMicros, hold.currency)}</TableCell>
                        <TableCell className='text-right font-mono'>{formatMicros(hold.capturedAmountMicros, hold.currency)}</TableCell>
                        <TableCell className='min-w-[260px]'>
                          {hold.status === 'held' ? (
                            <div className='flex items-center gap-2'>
                              <Input
                                className='h-8 min-w-[160px]'
                                value={holdReleaseReasons[hold.id] ?? ''}
                                placeholder={t('adminBilling.holds.releaseReasonPlaceholder')}
                                onChange={(event) => setHoldReleaseReasons((prev) => ({ ...prev, [hold.id]: event.target.value }))}
                              />
                              <Button
                                type='button'
                                size='sm'
                                variant='outline'
                                disabled={releaseHold.isPending}
                                onClick={() => void handleReleaseHold(hold.id)}
                              >
                                {releaseHold.isPending ? <Loader2 className='size-4 animate-spin' /> : <Unlock className='size-4' />}
                                {t('adminBilling.holds.release')}
                              </Button>
                            </div>
                          ) : (
                            <span className='text-muted-foreground text-xs'>
                              {hold.releasedAt ? `${formatDate(hold.releasedAt)} ${hold.releasedByID || ''}` : '-'}
                            </span>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value='orders' className='mt-0'>
            <Card className='rounded-lg'>
              <CardHeader>
                <CardTitle className='text-base'>{t('adminBilling.orders.title')}</CardTitle>
                <CardDescription>{t('adminBilling.orders.description')}</CardDescription>
              </CardHeader>
              <CardContent className='space-y-4'>
                <form
                  className='grid gap-3 md:grid-cols-4 xl:grid-cols-8'
                  onSubmit={(event) => {
                    event.preventDefault();
                    setAppliedOrderFilter(buildOrderFilter(orderFilter));
                  }}
                >
                  <FilterInput
                    label={t('adminBilling.filters.userId')}
                    value={orderFilter.userId}
                    onChange={(value) => setOrderFilter((prev) => ({ ...prev, userId: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.projectId')}
                    value={orderFilter.projectId}
                    onChange={(value) => setOrderFilter((prev) => ({ ...prev, projectId: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.accountId')}
                    value={orderFilter.billingAccountId}
                    onChange={(value) => setOrderFilter((prev) => ({ ...prev, billingAccountId: value }))}
                  />
                  <FilterSelect
                    label={t('adminBilling.columns.provider')}
                    value={orderFilter.providerType}
                    onChange={(value) => setOrderFilter((prev) => ({ ...prev, providerType: value as OrderFilterForm['providerType'] }))}
                    options={['all', 'manual', 'epay', 'stripe', 'custom']}
                  />
                  <FilterSelect
                    label={t('adminBilling.columns.status')}
                    value={orderFilter.status}
                    onChange={(value) => setOrderFilter((prev) => ({ ...prev, status: value as OrderFilterForm['status'] }))}
                    options={['all', 'pending', 'paid', 'failed', 'canceled', 'expired', 'refunded']}
                  />
                  <FilterInput
                    label={t('adminBilling.columns.orderNo')}
                    value={orderFilter.orderNo}
                    onChange={(value) => setOrderFilter((prev) => ({ ...prev, orderNo: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.columns.tradeNo')}
                    value={orderFilter.externalTradeNo}
                    onChange={(value) => setOrderFilter((prev) => ({ ...prev, externalTradeNo: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.from')}
                    type='datetime-local'
                    value={orderFilter.from}
                    onChange={(value) => setOrderFilter((prev) => ({ ...prev, from: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.to')}
                    type='datetime-local'
                    value={orderFilter.to}
                    onChange={(value) => setOrderFilter((prev) => ({ ...prev, to: value }))}
                  />
                  <FilterActions
                    onReset={() => {
                      const next = defaultOrderFilter();
                      setOrderFilter(next);
                      setAppliedOrderFilter({});
                    }}
                  />
                </form>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('adminBilling.columns.createdAt')}</TableHead>
                      <TableHead>{t('adminBilling.columns.orderNo')}</TableHead>
                      <TableHead>{t('adminBilling.filters.projectId')}</TableHead>
                      <TableHead>{t('adminBilling.columns.provider')}</TableHead>
                      <TableHead>{t('adminBilling.columns.status')}</TableHead>
                      <TableHead>{t('adminBilling.columns.tradeNo')}</TableHead>
                      <TableHead className='text-right'>{t('adminBilling.columns.amount')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <DataStateRow colSpan={7} isLoading={adminOrders.isLoading} isEmpty={(adminOrders.data ?? []).length === 0} />
                    {adminOrders.data?.map((order) => (
                      <TableRow key={order.id}>
                        <TableCell>{formatDate(order.createdAt)}</TableCell>
                        <TableCell className='font-mono text-xs'>{order.orderNo}</TableCell>
                        <TableCell>{order.projectID}</TableCell>
                        <TableCell>{order.providerType}</TableCell>
                        <TableCell>
                          <StatusBadge value={order.status} positive={order.status === 'paid'} />
                        </TableCell>
                        <TableCell className='font-mono text-xs'>{order.externalTradeNo || '-'}</TableCell>
                        <TableCell className='text-right font-mono'>{formatMicros(order.amountMicros, order.currency)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value='events' className='mt-0'>
            <Card className='rounded-lg'>
              <CardHeader>
                <CardTitle className='text-base'>{t('adminBilling.events.title')}</CardTitle>
                <CardDescription>{t('adminBilling.events.description')}</CardDescription>
              </CardHeader>
              <CardContent className='space-y-4'>
                <form
                  className='grid gap-3 md:grid-cols-4 xl:grid-cols-8'
                  onSubmit={(event) => {
                    event.preventDefault();
                    setAppliedEventFilter(buildEventFilter(eventFilter));
                  }}
                >
                  <FilterInput
                    label={t('adminBilling.filters.paymentOrderId')}
                    value={eventFilter.paymentOrderId}
                    onChange={(value) => setEventFilter((prev) => ({ ...prev, paymentOrderId: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.providerInstanceId')}
                    value={eventFilter.providerInstanceId}
                    onChange={(value) => setEventFilter((prev) => ({ ...prev, providerInstanceId: value }))}
                  />
                  <FilterSelect
                    label={t('adminBilling.columns.provider')}
                    value={eventFilter.providerType}
                    onChange={(value) => setEventFilter((prev) => ({ ...prev, providerType: value as EventFilterForm['providerType'] }))}
                    options={['all', 'manual', 'epay', 'stripe', 'custom']}
                  />
                  <FilterSelect
                    label={t('adminBilling.columns.status')}
                    value={eventFilter.status}
                    onChange={(value) => setEventFilter((prev) => ({ ...prev, status: value as EventFilterForm['status'] }))}
                    options={['all', 'received', 'processed', 'failed', 'ignored']}
                  />
                  <FilterInput
                    label={t('adminBilling.columns.eventType')}
                    value={eventFilter.eventType}
                    onChange={(value) => setEventFilter((prev) => ({ ...prev, eventType: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.columns.eventKey')}
                    value={eventFilter.eventKey}
                    onChange={(value) => setEventFilter((prev) => ({ ...prev, eventKey: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.from')}
                    type='datetime-local'
                    value={eventFilter.from}
                    onChange={(value) => setEventFilter((prev) => ({ ...prev, from: value }))}
                  />
                  <FilterInput
                    label={t('adminBilling.filters.to')}
                    type='datetime-local'
                    value={eventFilter.to}
                    onChange={(value) => setEventFilter((prev) => ({ ...prev, to: value }))}
                  />
                  <FilterActions
                    onReset={() => {
                      const next = defaultEventFilter();
                      setEventFilter(next);
                      setAppliedEventFilter({});
                    }}
                  />
                </form>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('adminBilling.columns.createdAt')}</TableHead>
                      <TableHead>{t('adminBilling.columns.eventKey')}</TableHead>
                      <TableHead>{t('adminBilling.filters.paymentOrderId')}</TableHead>
                      <TableHead>{t('adminBilling.columns.provider')}</TableHead>
                      <TableHead>{t('adminBilling.columns.eventType')}</TableHead>
                      <TableHead>{t('adminBilling.columns.status')}</TableHead>
                      <TableHead>{t('adminBilling.columns.reason')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <DataStateRow colSpan={7} isLoading={adminEvents.isLoading} isEmpty={(adminEvents.data ?? []).length === 0} />
                    {adminEvents.data?.map((event) => (
                      <TableRow key={event.id}>
                        <TableCell>{formatDate(event.createdAt)}</TableCell>
                        <TableCell className='font-mono text-xs'>{event.eventKey}</TableCell>
                        <TableCell className='font-mono text-xs'>{event.paymentOrderID || '-'}</TableCell>
                        <TableCell>{event.providerType}</TableCell>
                        <TableCell>{event.eventType}</TableCell>
                        <TableCell>
                          <StatusBadge value={event.status} positive={event.status === 'processed'} />
                        </TableCell>
                        <TableCell className='max-w-[280px] truncate text-xs'>{event.error || '-'}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value='pricing' className='mt-0'>
            <PricingCard
              data={data?.priceRules ?? []}
              isLoading={isLoading}
              priceForm={priceForm}
              setPriceForm={setPriceForm}
              handleSavePriceRule={handleSavePriceRule}
              savePending={savePriceRule.isPending}
            />
          </TabsContent>

          <TabsContent value='providers' className='mt-0'>
            <ProvidersCard
              providers={data?.providers ?? []}
              isLoading={isLoading}
              providerForm={providerForm}
              setProviderForm={setProviderForm}
              handleSaveEPayProvider={handleSaveEPayProvider}
              savePending={upsertEPay.isPending}
            />
          </TabsContent>
        </Tabs>
      </Main>
    </div>
  );
}

function ErrorAlert({ error }: { error: unknown }) {
  const { t } = useTranslation();
  if (!error) return null;
  return (
    <Alert variant='destructive'>
      <AlertCircle className='size-4' />
      <AlertTitle>{t('common.loadError')}</AlertTitle>
      <AlertDescription>{error instanceof Error ? error.message : t('common.errors.unknownError')}</AlertDescription>
    </Alert>
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

function DataStateRow({ colSpan, isLoading, isEmpty }: { colSpan: number; isLoading: boolean; isEmpty: boolean }) {
  const { t } = useTranslation();
  if (!isLoading && !isEmpty) return null;
  return (
    <TableRow>
      <TableCell colSpan={colSpan} className='text-muted-foreground h-24 text-center'>
        {isLoading ? t('common.loading') : t('common.noData')}
      </TableCell>
    </TableRow>
  );
}

function StatusBadge({ value, positive }: { value: string; positive: boolean }) {
  return <Badge variant={positive ? 'default' : 'secondary'}>{value}</Badge>;
}

function FilterInput({
  label,
  value,
  onChange,
  type = 'text',
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: string;
}) {
  return (
    <div className='space-y-2'>
      <Label>{label}</Label>
      <Input type={type} value={value} onChange={(event) => onChange(event.target.value)} />
    </div>
  );
}

function FilterSelect({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: string[];
}) {
  return (
    <div className='space-y-2'>
      <Label>{label}</Label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option} value={option}>
              {option}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

function FilterActions({ onReset }: { onReset: () => void }) {
  const { t } = useTranslation();
  return (
    <div className='flex items-end gap-2 md:col-span-2'>
      <Button type='submit'>{t('adminBilling.filters.apply')}</Button>
      <Button type='button' variant='outline' onClick={onReset}>
        {t('adminBilling.filters.reset')}
      </Button>
    </div>
  );
}

function UserSelect({
  users,
  value,
  onChange,
  label,
  placeholder,
}: {
  users: Array<{ id: string; email: string }>;
  value: string;
  onChange: (value: string) => void;
  label: string;
  placeholder: string;
}) {
  return (
    <div className='space-y-2'>
      <Label>{label}</Label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger>
          <SelectValue placeholder={placeholder} />
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
  );
}

function MiniList({ title, isLoading, empty, children }: { title: string; isLoading: boolean; empty: boolean; children: ReactNode }) {
  const { t } = useTranslation();
  return (
    <div className='rounded-md border p-3'>
      <div className='mb-2 text-sm font-medium'>{title}</div>
      {isLoading ? (
        <div className='text-muted-foreground text-sm'>{t('common.loading')}</div>
      ) : empty ? (
        <div className='text-muted-foreground text-sm'>{t('common.noData')}</div>
      ) : (
        <div className='space-y-2'>{children}</div>
      )}
    </div>
  );
}

function MiniRow({ left, right, sub }: { left: string; right: string; sub: string }) {
  return (
    <div className='flex items-start justify-between gap-3 text-sm'>
      <div className='min-w-0'>
        <div className='truncate font-medium'>{left}</div>
        <div className='text-muted-foreground truncate text-xs'>{sub}</div>
      </div>
      <div className='font-mono text-xs'>{right}</div>
    </div>
  );
}

function PricingCard({
  data,
  isLoading,
  priceForm,
  setPriceForm,
  handleSavePriceRule,
  savePending,
}: {
  data: BillingPriceRule[];
  isLoading: boolean;
  priceForm: PriceForm;
  setPriceForm: (value: PriceForm | ((prev: PriceForm) => PriceForm)) => void;
  handleSavePriceRule: (event: FormEvent<HTMLFormElement>) => void;
  savePending: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Card className='rounded-lg'>
      <CardHeader>
        <CardTitle className='text-base'>{t('adminBilling.pricing.title')}</CardTitle>
        <CardDescription>{t('adminBilling.pricing.description')}</CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <form className='grid gap-3 md:grid-cols-2 xl:grid-cols-4' onSubmit={handleSavePriceRule}>
          <div className='space-y-2'>
            <Label>{t('adminBilling.pricing.scopeType')}</Label>
            <Select
              value={priceForm.scopeType}
              onValueChange={(value) =>
                setPriceForm((prev) => ({
                  ...prev,
                  scopeType: value as PriceForm['scopeType'],
                  scopeId: value === 'global' ? '0' : prev.scopeId,
                }))
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
          <FilterInput
            label={t('adminBilling.pricing.scopeId')}
            value={priceForm.scopeId}
            onChange={(value) => setPriceForm((prev) => ({ ...prev, scopeId: value }))}
          />
          <FilterInput
            label={t('adminBilling.pricing.modelPattern')}
            value={priceForm.modelPattern}
            onChange={(value) => setPriceForm((prev) => ({ ...prev, modelPattern: value }))}
          />
          <FilterInput
            label={t('adminBilling.pricing.priority')}
            value={priceForm.priority}
            onChange={(value) => setPriceForm((prev) => ({ ...prev, priority: value }))}
          />
          <FilterInput
            label={t('adminBilling.pricing.promptPrice')}
            value={priceForm.promptPrice}
            onChange={(value) => setPriceForm((prev) => ({ ...prev, promptPrice: value }))}
          />
          <FilterInput
            label={t('adminBilling.pricing.completionPrice')}
            value={priceForm.completionPrice}
            onChange={(value) => setPriceForm((prev) => ({ ...prev, completionPrice: value }))}
          />
          <FilterInput
            label={t('adminBilling.columns.currency')}
            value={priceForm.currency}
            onChange={(value) => setPriceForm((prev) => ({ ...prev, currency: value.toUpperCase() }))}
          />
          <div className='flex items-center justify-between rounded-md border px-3 py-2'>
            <Label htmlFor='admin-billing-price-enabled'>{t('adminBilling.pricing.enabled')}</Label>
            <Switch
              id='admin-billing-price-enabled'
              checked={priceForm.enabled}
              onCheckedChange={(checked) => setPriceForm((prev) => ({ ...prev, enabled: checked }))}
            />
          </div>
          <div className='flex items-end gap-2 md:col-span-2 xl:col-span-4'>
            <Button type='submit' disabled={savePending}>
              {savePending ? <Loader2 className='size-4 animate-spin' /> : <Save className='size-4' />}
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
            <DataStateRow colSpan={4} isLoading={isLoading} isEmpty={data.length === 0} />
            {data.map((rule) => (
              <TableRow key={rule.id} className='cursor-pointer' onClick={() => setPriceForm(priceFormFromRule(rule))}>
                <TableCell>{`${rule.scopeType}:${rule.scopeID}`}</TableCell>
                <TableCell className='font-mono text-xs'>{rule.modelPattern}</TableCell>
                <TableCell className='max-w-[360px] truncate font-mono text-xs'>{priceSummary(rule)}</TableCell>
                <TableCell>
                  <StatusBadge value={rule.enabled ? 'enabled' : 'disabled'} positive={rule.enabled} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}

function ProvidersCard({
  providers,
  isLoading,
  providerForm,
  setProviderForm,
  handleSaveEPayProvider,
  savePending,
}: {
  providers: Array<{ id: string; name: string; status: string; currency: string; updatedAt: string }>;
  isLoading: boolean;
  providerForm: EPayProviderForm;
  setProviderForm: (value: EPayProviderForm | ((prev: EPayProviderForm) => EPayProviderForm)) => void;
  handleSaveEPayProvider: (event: FormEvent<HTMLFormElement>) => void;
  savePending: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Card className='rounded-lg'>
      <CardHeader>
        <CardTitle className='text-base'>{t('adminBilling.epay.title')}</CardTitle>
        <CardDescription>{t('adminBilling.epay.description')}</CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <form className='grid gap-3 md:grid-cols-2 xl:grid-cols-4' onSubmit={handleSaveEPayProvider}>
          <FilterInput
            label={t('adminBilling.epay.name')}
            value={providerForm.name}
            onChange={(value) => setProviderForm((prev) => ({ ...prev, name: value }))}
          />
          <div className='space-y-2'>
            <Label>{t('adminBilling.columns.status')}</Label>
            <Select
              value={providerForm.status}
              onValueChange={(value) => setProviderForm((prev) => ({ ...prev, status: value as 'enabled' | 'disabled' }))}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='enabled'>enabled</SelectItem>
                <SelectItem value='disabled'>disabled</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <FilterInput
            label={t('adminBilling.epay.gatewayUrl')}
            value={providerForm.gatewayUrl}
            onChange={(value) => setProviderForm((prev) => ({ ...prev, gatewayUrl: value }))}
          />
          <FilterInput
            label={t('adminBilling.epay.pid')}
            value={providerForm.pid}
            onChange={(value) => setProviderForm((prev) => ({ ...prev, pid: value }))}
          />
          <FilterInput
            label={t('adminBilling.epay.key')}
            value={providerForm.key}
            onChange={(value) => setProviderForm((prev) => ({ ...prev, key: value }))}
          />
          <FilterInput
            label={t('adminBilling.epay.notifyUrl')}
            value={providerForm.notifyUrl}
            onChange={(value) => setProviderForm((prev) => ({ ...prev, notifyUrl: value }))}
          />
          <FilterInput
            label={t('adminBilling.epay.returnUrl')}
            value={providerForm.returnUrl}
            onChange={(value) => setProviderForm((prev) => ({ ...prev, returnUrl: value }))}
          />
          <FilterInput
            label={t('adminBilling.columns.currency')}
            value={providerForm.currency}
            onChange={(value) => setProviderForm((prev) => ({ ...prev, currency: value.toUpperCase() }))}
          />
          <div className='flex items-end'>
            <Button type='submit' disabled={savePending}>
              {savePending ? <Loader2 className='size-4 animate-spin' /> : <Save className='size-4' />}
              {t('adminBilling.epay.submit')}
            </Button>
          </div>
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
            <DataStateRow colSpan={4} isLoading={isLoading} isEmpty={providers.length === 0} />
            {providers.map((provider) => (
              <TableRow key={provider.id}>
                <TableCell>{provider.name}</TableCell>
                <TableCell>
                  <StatusBadge value={provider.status} positive={provider.status === 'enabled'} />
                </TableCell>
                <TableCell>{provider.currency}</TableCell>
                <TableCell>{formatDate(provider.updatedAt)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
