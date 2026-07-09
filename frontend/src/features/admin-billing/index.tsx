import { FormEvent, ReactNode, useEffect, useMemo, useState } from 'react';
import { AlertCircle, Ban, BarChart3, Clock, Download, Loader2, PackageCheck, RefreshCw, RotateCcw, Save, Ticket, Trash2, Unlock, WalletCards } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { extractNumberIDAsNumber } from '@/lib/utils';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Progress } from '@/components/ui/progress';
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
  type AdminBillingReportFilter,
  type AdminPaymentEventsFilter,
  type AdminPaymentOrdersFilter,
  type AdminRedeemCodesFilter,
  type AdminUserSubscriptionsFilter,
  type AdminUsageBillingRecordsFilter,
  type BillingCSVExportDataset,
  type BillingCommercialReport,
  type BillingHoldStatus,
  type BillingAccountStatus,
  type BillingPriceRule,
  type LedgerTransactionDirection,
  type ModelPrice,
  type PaymentEventStatus,
  type PaymentOrderStatus,
  type PaymentProviderType,
  type RedeemCode,
  type RedeemCodeStatus,
  type RedeemCodeType,
  type SubscriptionPlan,
  type SubscriptionPlanPeriod,
  type SubscriptionPlanStatus,
  type UsageBillingRecordStatus,
  type UserSubscription,
  type UserSubscriptionStatus,
  useAdminAssignSubscription,
  useAdjustUserBalance,
  useAdminBillingOverview,
  useAdminBillingReport,
  useAdminBillingHolds,
  useAdminLedgerTransactions,
  useAdminPaymentEvents,
  useAdminPaymentOrders,
  useAdminRedeemCodes,
  useAdminSubscriptionPlans,
  useAdminUsageBillingRecords,
  useAdminUserSubscriptions,
  useAdminUserBillingDetail,
  useAdminCreateAndRedeemCode,
  useCancelPaymentOrder,
  useCreateRedeemCodes,
  useDeleteRedeemCode,
  useDeleteSubscriptionPlan,
  useExtendUserSubscription,
  useExportAdminBillingCSV,
  useReleaseBillingHold,
  useResetUserSubscriptionUsage,
  useRestoreUserSubscription,
  useRevokeUserSubscription,
  useMakeUpPaymentOrder,
  useSaveBillingPriceRule,
  useSaveSubscriptionPlan,
  useUpdateRedeemCodeStatus,
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

type RedeemFilterForm = {
  userId: string;
  createdById: string;
  status: 'all' | RedeemCodeStatus;
  type: 'all' | RedeemCodeType;
  code: string;
  batchId: string;
  from: string;
  to: string;
  expiresBefore: string;
};

type RedeemGenerateForm = {
  count: string;
  amount: string;
  currency: string;
  prefix: string;
  expiresAt: string;
  notes: string;
};

type RedeemGrantForm = {
  userId: string;
  amount: string;
  currency: string;
  expiresAt: string;
  notes: string;
};

type ReportFilterForm = {
  from: string;
  to: string;
  currency: string;
  limit: string;
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

type SubscriptionPlanForm = {
  id?: string;
  name: string;
  description: string;
  period: SubscriptionPlanPeriod;
  periodDays: string;
  price: string;
  currency: string;
  includedAmount: string;
  supportedModelIDs: string;
  supportedProjectIDs: string;
  supportedGroupIDs: string;
  allowWalletFallback: boolean;
  status: SubscriptionPlanStatus;
  sortOrder: string;
};

type SubscriptionAssignForm = {
  userId: string;
  planId: string;
  startsAt: string;
  expiresAt: string;
  notes: string;
};

type SubscriptionFilterForm = {
  userId: string;
  planId: string;
  status: 'all' | UserSubscriptionStatus;
  from: string;
  to: string;
  expiresBefore: string;
};

type SubscriptionActionForm = {
  days: string;
  expiresAt: string;
  reason: string;
  notes: string;
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

function summarizeJSONPayload(payload: unknown) {
  if (payload === null || payload === undefined) return '-';
  if (typeof payload === 'string') {
    return payload.length > 160 ? `${payload.slice(0, 157)}...` : payload;
  }
  try {
    const serialized = JSON.stringify(payload);
    return serialized.length > 160 ? `${serialized.slice(0, 157)}...` : serialized;
  } catch {
    return String(payload);
  }
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

function toDateTimeLocalValue(date: Date) {
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function defaultReportFilter(): ReportFilterForm {
  const to = new Date();
  const from = new Date(to);
  from.setDate(from.getDate() - 6);
  from.setHours(0, 0, 0, 0);
  return {
    from: toDateTimeLocalValue(from),
    to: toDateTimeLocalValue(to),
    currency: 'CNY',
    limit: '10',
  };
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

function defaultRedeemFilter(): RedeemFilterForm {
  return {
    userId: '',
    createdById: '',
    status: 'all',
    type: 'all',
    code: '',
    batchId: '',
    from: '',
    to: '',
    expiresBefore: '',
  };
}

function defaultRedeemGenerateForm(): RedeemGenerateForm {
  return {
    count: '10',
    amount: '10.00',
    currency: 'CNY',
    prefix: 'AX',
    expiresAt: '',
    notes: '',
  };
}

function defaultRedeemGrantForm(): RedeemGrantForm {
  return {
    userId: '',
    amount: '10.00',
    currency: 'CNY',
    expiresAt: '',
    notes: '',
  };
}

function defaultSubscriptionPlanForm(): SubscriptionPlanForm {
  return {
    name: '',
    description: '',
    period: 'month',
    periodDays: '30',
    price: '29.00',
    currency: 'CNY',
    includedAmount: '100.00',
    supportedModelIDs: '',
    supportedProjectIDs: '',
    supportedGroupIDs: '',
    allowWalletFallback: true,
    status: 'enabled',
    sortOrder: '100',
  };
}

function subscriptionPlanFormFromPlan(plan: SubscriptionPlan): SubscriptionPlanForm {
  return {
    id: plan.id,
    name: plan.name,
    description: plan.description,
    period: plan.period,
    periodDays: String(plan.periodDays),
    price: microsToAmount(plan.priceMicros).toFixed(2),
    currency: plan.currency,
    includedAmount: microsToAmount(plan.includedAmountMicros).toFixed(2),
    supportedModelIDs: plan.supportedModelIds.join(', '),
    supportedProjectIDs: plan.supportedProjectIds.join(', '),
    supportedGroupIDs: plan.supportedGroupIds.join(', '),
    allowWalletFallback: plan.allowWalletFallback,
    status: plan.status,
    sortOrder: String(plan.sortOrder),
  };
}

function defaultSubscriptionAssignForm(): SubscriptionAssignForm {
  return { userId: '', planId: '', startsAt: '', expiresAt: '', notes: '' };
}

function defaultSubscriptionFilter(): SubscriptionFilterForm {
  return { userId: '', planId: '', status: 'all', from: '', to: '', expiresBefore: '' };
}

function defaultSubscriptionActionForm(): SubscriptionActionForm {
  return { days: '30', expiresAt: '', reason: '', notes: '' };
}

function buildReportFilter(form: ReportFilterForm): AdminBillingReportFilter {
  return {
    from: optionalTime(form.from),
    to: optionalTime(form.to),
    currency: optionalText(form.currency.toUpperCase()),
    limit: optionalInt(form.limit),
  };
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

function buildRedeemFilter(form: RedeemFilterForm): AdminRedeemCodesFilter {
  return {
    userId: optionalInt(form.userId),
    createdById: optionalInt(form.createdById),
    status: form.status === 'all' ? undefined : form.status,
    type: form.type === 'all' ? undefined : form.type,
    code: optionalText(form.code),
    batchId: optionalText(form.batchId),
    from: optionalTime(form.from),
    to: optionalTime(form.to),
    expiresBefore: optionalTime(form.expiresBefore),
  };
}

function buildSubscriptionFilter(form: SubscriptionFilterForm): AdminUserSubscriptionsFilter {
  return {
    userId: optionalInt(form.userId),
    planId: optionalInt(form.planId),
    status: form.status === 'all' ? undefined : form.status,
    from: optionalTime(form.from),
    to: optionalTime(form.to),
    expiresBefore: optionalTime(form.expiresBefore),
  };
}

function splitCSVText(value: string) {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
}

function splitCSVInts(value: string) {
  return splitCSVText(value)
    .map((item) => Number(item))
    .filter((item) => Number.isInteger(item) && item > 0);
}

function usagePercent(subscription: UserSubscription) {
  if (subscription.includedAmountMicros <= 0) return 0;
  return Math.min(100, Math.round((subscription.usedAmountMicros / subscription.includedAmountMicros) * 100));
}

function csvCell(value: unknown) {
  const text = value === null || value === undefined ? '' : String(value);
  return `"${text.replace(/"/g, '""')}"`;
}

function downloadTextFile(fileName: string, content: string, contentType = 'text/csv;charset=utf-8') {
  const blob = new Blob([content], { type: contentType });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = fileName;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
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
  const [redeemFilter, setRedeemFilter] = useState<RedeemFilterForm>(() => defaultRedeemFilter());
  const [redeemGenerateForm, setRedeemGenerateForm] = useState<RedeemGenerateForm>(() => defaultRedeemGenerateForm());
  const [redeemGrantForm, setRedeemGrantForm] = useState<RedeemGrantForm>(() => defaultRedeemGrantForm());
  const [subscriptionPlanForm, setSubscriptionPlanForm] = useState<SubscriptionPlanForm>(() => defaultSubscriptionPlanForm());
  const [subscriptionAssignForm, setSubscriptionAssignForm] = useState<SubscriptionAssignForm>(() => defaultSubscriptionAssignForm());
  const [subscriptionFilter, setSubscriptionFilter] = useState<SubscriptionFilterForm>(() => defaultSubscriptionFilter());
  const [subscriptionActions, setSubscriptionActions] = useState<Record<string, SubscriptionActionForm>>({});
  const [reportFilter, setReportFilter] = useState<ReportFilterForm>(() => defaultReportFilter());
  const [appliedLedgerFilter, setAppliedLedgerFilter] = useState<AdminLedgerTransactionsFilter>({});
  const [appliedUsageFilter, setAppliedUsageFilter] = useState<AdminUsageBillingRecordsFilter>({});
  const [appliedHoldFilter, setAppliedHoldFilter] = useState<AdminBillingHoldsFilter>({});
  const [appliedOrderFilter, setAppliedOrderFilter] = useState<AdminPaymentOrdersFilter>({});
  const [appliedEventFilter, setAppliedEventFilter] = useState<AdminPaymentEventsFilter>({});
  const [appliedRedeemFilter, setAppliedRedeemFilter] = useState<AdminRedeemCodesFilter>({});
  const [appliedSubscriptionFilter, setAppliedSubscriptionFilter] = useState<AdminUserSubscriptionsFilter>({});
  const [appliedReportFilter, setAppliedReportFilter] = useState<AdminBillingReportFilter>(() => buildReportFilter(defaultReportFilter()));
  const [holdReleaseReasons, setHoldReleaseReasons] = useState<Record<string, string>>({});
  const [orderReasons, setOrderReasons] = useState<Record<string, string>>({});
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
  const adminRedeemCodes = useAdminRedeemCodes(appliedRedeemFilter, 50);
  const adminSubscriptionPlans = useAdminSubscriptionPlans(100);
  const adminUserSubscriptions = useAdminUserSubscriptions(appliedSubscriptionFilter, 50);
  const adminReport = useAdminBillingReport(appliedReportFilter);
  const adjustBalance = useAdjustUserBalance();
  const updateAccount = useUpdateUserBillingAccount();
  const releaseHold = useReleaseBillingHold();
  const cancelOrder = useCancelPaymentOrder();
  const makeUpOrder = useMakeUpPaymentOrder();
  const exportBillingCSV = useExportAdminBillingCSV();
  const createRedeemCodes = useCreateRedeemCodes();
  const adminCreateAndRedeemCode = useAdminCreateAndRedeemCode();
  const updateRedeemCodeStatus = useUpdateRedeemCodeStatus();
  const deleteRedeemCode = useDeleteRedeemCode();
  const saveSubscriptionPlan = useSaveSubscriptionPlan();
  const deleteSubscriptionPlan = useDeleteSubscriptionPlan();
  const adminAssignSubscription = useAdminAssignSubscription();
  const extendUserSubscription = useExtendUserSubscription();
  const revokeUserSubscription = useRevokeUserSubscription();
  const restoreUserSubscription = useRestoreUserSubscription();
  const resetUserSubscriptionUsage = useResetUserSubscriptionUsage();
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
      adminRedeemCodes.refetch(),
      adminSubscriptionPlans.refetch(),
      adminUserSubscriptions.refetch(),
      adminReport.refetch(),
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

  async function handleCancelOrder(orderNo: string) {
    const reason = (orderReasons[orderNo] || t('adminBilling.orders.defaultCancelReason')).trim();
    if (!reason) {
      toast.error(t('adminBilling.orders.reasonRequired'));
      return;
    }

    try {
      await cancelOrder.mutateAsync({ orderNo, reason });
      toast.success(t('adminBilling.orders.cancelSuccess'));
      setOrderReasons((prev) => ({ ...prev, [orderNo]: '' }));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

  async function handleMakeUpOrder(orderNo: string) {
    const reason = (orderReasons[orderNo] || t('adminBilling.orders.defaultMakeupReason')).trim();
    if (!reason) {
      toast.error(t('adminBilling.orders.reasonRequired'));
      return;
    }

    try {
      await makeUpOrder.mutateAsync({ orderNo, reason });
      toast.success(t('adminBilling.orders.makeupSuccess'));
      setOrderReasons((prev) => ({ ...prev, [orderNo]: '' }));
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

  function handleExportReport(dataset: BillingCSVExportDataset) {
    exportBillingCSV.mutate({ dataset, ...appliedReportFilter });
  }

  async function handleCreateRedeemCodes(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const amount = normalizeAmount(redeemGenerateForm.amount, 2);
    const count = Number(redeemGenerateForm.count);
    if (!Number.isInteger(count) || count <= 0 || count > 500) {
      toast.error(t('adminBilling.redeem.invalidCount'));
      return;
    }
    if (!amount) {
      toast.error(t('adminBilling.redeem.invalidAmount'));
      return;
    }

    try {
      const created = await createRedeemCodes.mutateAsync({
        count,
        type: 'balance',
        amount,
        currency: redeemGenerateForm.currency.trim().toUpperCase() || 'CNY',
        prefix: optionalText(redeemGenerateForm.prefix.toUpperCase()),
        expiresAt: optionalTime(redeemGenerateForm.expiresAt),
        notes: optionalText(redeemGenerateForm.notes),
      });
      toast.success(t('adminBilling.redeem.generateSuccess', { count: created.length }));
      setRedeemGenerateForm((prev) => ({ ...prev, count: '10', notes: '' }));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

  async function handleAdminCreateAndRedeemCode(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!redeemGrantForm.userId) {
      toast.error(t('adminBilling.redeem.selectUserRequired'));
      return;
    }
    const amount = normalizeAmount(redeemGrantForm.amount, 2);
    if (!amount) {
      toast.error(t('adminBilling.redeem.invalidAmount'));
      return;
    }

    try {
      await adminCreateAndRedeemCode.mutateAsync({
        userId: redeemGrantForm.userId,
        amount,
        currency: redeemGrantForm.currency.trim().toUpperCase() || 'CNY',
        expiresAt: optionalTime(redeemGrantForm.expiresAt),
        notes: optionalText(redeemGrantForm.notes),
      });
      toast.success(t('adminBilling.redeem.createAndRedeemSuccess'));
      setRedeemGrantForm((prev) => ({ ...prev, amount: '10.00', notes: '' }));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

  async function handleUpdateRedeemCodeStatus(code: RedeemCode, status: RedeemCodeStatus) {
    try {
      await updateRedeemCodeStatus.mutateAsync({
        codeId: code.id,
        status,
        notes: code.notes || undefined,
      });
      toast.success(t('adminBilling.redeem.statusSuccess'));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

  async function handleDeleteRedeemCode(code: RedeemCode) {
    if (code.status === 'used') {
      toast.error(t('adminBilling.redeem.deleteUsedDenied'));
      return;
    }
    if (!window.confirm(t('adminBilling.redeem.deleteConfirm'))) {
      return;
    }

    try {
      await deleteRedeemCode.mutateAsync(code.id);
      toast.success(t('adminBilling.redeem.deleteSuccess'));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

  function handleExportRedeemCodes(rows: RedeemCode[]) {
    const header = [
      'code',
      'type',
      'status',
      'amountMicros',
      'currency',
      'createdAt',
      'expiresAt',
      'usedAt',
      'createdByID',
      'usedByID',
      'ledgerTransactionID',
      'batchID',
      'notes',
    ];
    const body = rows.map((row) =>
      [
        row.code,
        row.type,
        row.status,
        row.amountMicros,
        row.currency,
        row.createdAt,
        row.expiresAt,
        row.usedAt,
        row.createdByID,
        row.usedByID,
        row.ledgerTransactionID,
        row.batchID,
        row.notes,
      ]
        .map(csvCell)
        .join(',')
    );
    downloadTextFile(`redeem-codes-${new Date().toISOString().slice(0, 10)}.csv`, [header.join(','), ...body].join('\n'));
    toast.success(t('adminBilling.redeem.exportSuccess'));
  }

  async function handleSaveSubscriptionPlan(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const price = normalizeNonNegativeAmount(subscriptionPlanForm.price, 2);
    const includedAmount = normalizeNonNegativeAmount(subscriptionPlanForm.includedAmount, 2);
    const periodDays = Number(subscriptionPlanForm.periodDays);
    const sortOrder = Number(subscriptionPlanForm.sortOrder);
    if (!subscriptionPlanForm.name.trim()) {
      toast.error(t('adminBilling.subscriptions.planNameRequired'));
      return;
    }
    if (!price || !includedAmount || !Number.isInteger(periodDays) || periodDays <= 0 || !Number.isInteger(sortOrder)) {
      toast.error(t('adminBilling.subscriptions.invalidPlan'));
      return;
    }

    try {
      await saveSubscriptionPlan.mutateAsync({
        id: subscriptionPlanForm.id,
        name: subscriptionPlanForm.name.trim(),
        description: optionalText(subscriptionPlanForm.description),
        period: subscriptionPlanForm.period,
        periodDays,
        price,
        currency: subscriptionPlanForm.currency.trim().toUpperCase() || 'CNY',
        includedAmount,
        supportedModelIDs: splitCSVText(subscriptionPlanForm.supportedModelIDs),
        supportedProjectIDs: splitCSVInts(subscriptionPlanForm.supportedProjectIDs),
        supportedGroupIDs: splitCSVInts(subscriptionPlanForm.supportedGroupIDs),
        allowWalletFallback: subscriptionPlanForm.allowWalletFallback,
        status: subscriptionPlanForm.status,
        sortOrder,
      });
      toast.success(t('adminBilling.subscriptions.planSaveSuccess'));
      setSubscriptionPlanForm(defaultSubscriptionPlanForm());
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

  async function handleDeleteSubscriptionPlan(plan: SubscriptionPlan) {
    if (!window.confirm(t('adminBilling.subscriptions.deletePlanConfirm'))) return;
    try {
      await deleteSubscriptionPlan.mutateAsync(plan.id);
      toast.success(t('adminBilling.subscriptions.deletePlanSuccess'));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

  async function handleAssignSubscription(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!subscriptionAssignForm.userId || !subscriptionAssignForm.planId) {
      toast.error(t('adminBilling.subscriptions.selectUserAndPlan'));
      return;
    }

    try {
      await adminAssignSubscription.mutateAsync({
        userId: subscriptionAssignForm.userId,
        planId: subscriptionAssignForm.planId,
        startsAt: optionalTime(subscriptionAssignForm.startsAt),
        expiresAt: optionalTime(subscriptionAssignForm.expiresAt),
        notes: optionalText(subscriptionAssignForm.notes),
      });
      toast.success(t('adminBilling.subscriptions.assignSuccess'));
      setSubscriptionAssignForm(defaultSubscriptionAssignForm());
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

  function subscriptionActionValue(subscriptionID: string) {
    return subscriptionActions[subscriptionID] ?? defaultSubscriptionActionForm();
  }

  async function handleExtendSubscription(subscription: UserSubscription) {
    const action = subscriptionActionValue(subscription.id);
    const days = Number(action.days);
    const expiresAt = optionalTime(action.expiresAt);
    if (!expiresAt && (!Number.isInteger(days) || days <= 0)) {
      toast.error(t('adminBilling.subscriptions.invalidExtend'));
      return;
    }
    try {
      await extendUserSubscription.mutateAsync({
        subscriptionId: subscription.id,
        days: expiresAt ? undefined : days,
        expiresAt,
        notes: optionalText(action.notes),
      });
      toast.success(t('adminBilling.subscriptions.extendSuccess'));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

  async function handleRevokeSubscription(subscription: UserSubscription) {
    const action = subscriptionActionValue(subscription.id);
    try {
      await revokeUserSubscription.mutateAsync({ id: subscription.id, reason: optionalText(action.reason) });
      toast.success(t('adminBilling.subscriptions.revokeSuccess'));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

  async function handleRestoreSubscription(subscription: UserSubscription) {
    try {
      await restoreUserSubscription.mutateAsync(subscription.id);
      toast.success(t('adminBilling.subscriptions.restoreSuccess'));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errors.unknownError'));
    }
  }

  async function handleResetSubscriptionUsage(subscription: UserSubscription) {
    try {
      await resetUserSubscriptionUsage.mutateAsync(subscription.id);
      toast.success(t('adminBilling.subscriptions.resetSuccess'));
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
              adminEvents.isFetching ||
              adminRedeemCodes.isFetching ||
              adminSubscriptionPlans.isFetching ||
              adminUserSubscriptions.isFetching ||
              adminReport.isFetching
            }
          >
            {isFetching ||
            adminLedger.isFetching ||
            adminUsage.isFetching ||
            adminHolds.isFetching ||
            adminOrders.isFetching ||
            adminEvents.isFetching ||
            adminRedeemCodes.isFetching ||
            adminSubscriptionPlans.isFetching ||
            adminUserSubscriptions.isFetching ||
            adminReport.isFetching ? (
              <Loader2 className='size-4 animate-spin' />
            ) : (
              <RefreshCw className='size-4' />
            )}
            {t('common.refresh')}
          </Button>
        </div>
      </Header>

      <Main fixed className='flex flex-col gap-4 overflow-auto'>
        <ErrorAlert
          error={
            error ||
            adminLedger.error ||
            adminUsage.error ||
            adminHolds.error ||
            adminOrders.error ||
            adminEvents.error ||
            adminRedeemCodes.error ||
            adminSubscriptionPlans.error ||
            adminUserSubscriptions.error ||
            adminReport.error
          }
        />

        <div className='grid gap-4 md:grid-cols-4 xl:grid-cols-8'>
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
          <MetricCard
            title={t('adminBilling.metrics.redeemCodes')}
            value={String(adminRedeemCodes.data?.length ?? 0)}
            loading={adminRedeemCodes.isLoading}
          />
          <MetricCard
            title={t('adminBilling.metrics.subscriptions')}
            value={String(adminUserSubscriptions.data?.length ?? 0)}
            loading={adminUserSubscriptions.isLoading}
          />
        </div>

        <Tabs defaultValue='reports' className='gap-4'>
          <TabsList className='shadow-soft border-border bg-background flex h-auto w-full justify-start overflow-x-auto rounded-lg border p-1'>
            <TabsTrigger value='reports'>{t('adminBilling.tabs.reports')}</TabsTrigger>
            <TabsTrigger value='wallets'>{t('adminBilling.tabs.wallets')}</TabsTrigger>
            <TabsTrigger value='ledger'>{t('adminBilling.tabs.ledger')}</TabsTrigger>
            <TabsTrigger value='usage'>{t('adminBilling.tabs.usage')}</TabsTrigger>
            <TabsTrigger value='holds'>{t('adminBilling.tabs.holds')}</TabsTrigger>
            <TabsTrigger value='orders'>{t('adminBilling.tabs.orders')}</TabsTrigger>
            <TabsTrigger value='events'>{t('adminBilling.tabs.events')}</TabsTrigger>
            <TabsTrigger value='redeemCodes'>{t('adminBilling.tabs.redeemCodes')}</TabsTrigger>
            <TabsTrigger value='subscriptions'>{t('adminBilling.tabs.subscriptions')}</TabsTrigger>
            <TabsTrigger value='pricing'>{t('adminBilling.tabs.pricing')}</TabsTrigger>
            <TabsTrigger value='providers'>{t('adminBilling.tabs.providers')}</TabsTrigger>
          </TabsList>

          <TabsContent value='reports' className='mt-0'>
            <ReportsTab
              report={adminReport.data}
              isLoading={adminReport.isLoading}
              reportFilter={reportFilter}
              setReportFilter={setReportFilter}
              onApplyReportFilter={(event) => {
                event.preventDefault();
                setAppliedReportFilter(buildReportFilter(reportFilter));
              }}
              onResetReportFilter={() => {
                const next = defaultReportFilter();
                setReportFilter(next);
                setAppliedReportFilter(buildReportFilter(next));
              }}
              onExport={handleExportReport}
              exportPending={exportBillingCSV.isPending}
              formatMicros={formatMicros}
            />
          </TabsContent>

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
                      <TableHead>{t('adminBilling.columns.expiresAt')}</TableHead>
                      <TableHead>{t('adminBilling.columns.tradeNo')}</TableHead>
                      <TableHead>{t('adminBilling.columns.reason')}</TableHead>
                      <TableHead className='text-right'>{t('adminBilling.columns.amount')}</TableHead>
                      <TableHead>{t('adminBilling.columns.action')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <DataStateRow colSpan={10} isLoading={adminOrders.isLoading} isEmpty={(adminOrders.data ?? []).length === 0} />
                    {adminOrders.data?.map((order) => (
                      <TableRow key={order.id}>
                        <TableCell>{formatDate(order.createdAt)}</TableCell>
                        <TableCell className='font-mono text-xs'>{order.orderNo}</TableCell>
                        <TableCell>{order.projectID}</TableCell>
                        <TableCell>{order.providerType}</TableCell>
                        <TableCell>
                          <StatusBadge value={order.status} positive={order.status === 'paid'} />
                        </TableCell>
                        <TableCell>{formatDate(order.expiresAt)}</TableCell>
                        <TableCell className='font-mono text-xs'>{order.externalTradeNo || '-'}</TableCell>
                        <TableCell className='max-w-[220px] truncate text-xs'>
                          {order.failureReason || order.cancelReason || order.makeupReason || order.refundReason || '-'}
                        </TableCell>
                        <TableCell className='text-right font-mono'>{formatMicros(order.amountMicros, order.currency)}</TableCell>
                        <TableCell className='min-w-[320px]'>
                          {order.status === 'paid' || order.status === 'refunded' ? (
                            <span className='text-muted-foreground text-xs'>{order.paidAt ? formatDate(order.paidAt) : '-'}</span>
                          ) : (
                            <div className='flex items-center gap-2'>
                              <Input
                                className='h-8 min-w-[150px]'
                                value={orderReasons[order.orderNo] ?? ''}
                                placeholder={t('adminBilling.orders.reasonPlaceholder')}
                                onChange={(event) => setOrderReasons((prev) => ({ ...prev, [order.orderNo]: event.target.value }))}
                              />
                              {order.status === 'pending' && (
                                <Button
                                  type='button'
                                  size='sm'
                                  variant='outline'
                                  disabled={cancelOrder.isPending || makeUpOrder.isPending}
                                  onClick={() => void handleCancelOrder(order.orderNo)}
                                >
                                  {cancelOrder.isPending ? <Loader2 className='size-4 animate-spin' /> : <Ban className='size-4' />}
                                  {t('adminBilling.orders.cancel')}
                                </Button>
                              )}
                              <Button
                                type='button'
                                size='sm'
                                disabled={cancelOrder.isPending || makeUpOrder.isPending}
                                onClick={() => void handleMakeUpOrder(order.orderNo)}
                              >
                                {makeUpOrder.isPending ? <Loader2 className='size-4 animate-spin' /> : <Save className='size-4' />}
                                {t('adminBilling.orders.makeup')}
                              </Button>
                            </div>
                          )}
                        </TableCell>
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
                      <TableHead>{t('adminBilling.columns.payload')}</TableHead>
                      <TableHead>{t('adminBilling.columns.reason')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <DataStateRow colSpan={8} isLoading={adminEvents.isLoading} isEmpty={(adminEvents.data ?? []).length === 0} />
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
                        <TableCell className='max-w-[320px] truncate font-mono text-xs' title={summarizeJSONPayload(event.payload)}>
                          {summarizeJSONPayload(event.payload)}
                        </TableCell>
                        <TableCell className='max-w-[280px] truncate text-xs'>{event.error || '-'}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value='redeemCodes' className='mt-0'>
            <RedeemCodesTab
              users={users}
              rows={adminRedeemCodes.data ?? []}
              isLoading={adminRedeemCodes.isLoading}
              filter={redeemFilter}
              setFilter={setRedeemFilter}
              onApplyFilter={(event) => {
                event.preventDefault();
                setAppliedRedeemFilter(buildRedeemFilter(redeemFilter));
              }}
              onResetFilter={() => {
                const next = defaultRedeemFilter();
                setRedeemFilter(next);
                setAppliedRedeemFilter({});
              }}
              generateForm={redeemGenerateForm}
              setGenerateForm={setRedeemGenerateForm}
              onCreateRedeemCodes={handleCreateRedeemCodes}
              createPending={createRedeemCodes.isPending}
              grantForm={redeemGrantForm}
              setGrantForm={setRedeemGrantForm}
              onAdminCreateAndRedeemCode={handleAdminCreateAndRedeemCode}
              grantPending={adminCreateAndRedeemCode.isPending}
              onUpdateStatus={handleUpdateRedeemCodeStatus}
              updatePending={updateRedeemCodeStatus.isPending}
              onDelete={handleDeleteRedeemCode}
              deletePending={deleteRedeemCode.isPending}
              onExport={handleExportRedeemCodes}
              formatMicros={formatMicros}
            />
          </TabsContent>

          <TabsContent value='subscriptions' className='mt-0'>
            <SubscriptionsTab
              users={users}
              plans={adminSubscriptionPlans.data ?? []}
              subscriptions={adminUserSubscriptions.data ?? []}
              plansLoading={adminSubscriptionPlans.isLoading}
              subscriptionsLoading={adminUserSubscriptions.isLoading}
              planForm={subscriptionPlanForm}
              setPlanForm={setSubscriptionPlanForm}
              onSavePlan={handleSaveSubscriptionPlan}
              savePlanPending={saveSubscriptionPlan.isPending}
              onDeletePlan={handleDeleteSubscriptionPlan}
              deletePlanPending={deleteSubscriptionPlan.isPending}
              assignForm={subscriptionAssignForm}
              setAssignForm={setSubscriptionAssignForm}
              onAssign={handleAssignSubscription}
              assignPending={adminAssignSubscription.isPending}
              filter={subscriptionFilter}
              setFilter={setSubscriptionFilter}
              onApplyFilter={(event) => {
                event.preventDefault();
                setAppliedSubscriptionFilter(buildSubscriptionFilter(subscriptionFilter));
              }}
              onResetFilter={() => {
                const next = defaultSubscriptionFilter();
                setSubscriptionFilter(next);
                setAppliedSubscriptionFilter({});
              }}
              actionValues={subscriptionActions}
              setActionValues={setSubscriptionActions}
              onExtend={handleExtendSubscription}
              onRevoke={handleRevokeSubscription}
              onRestore={handleRestoreSubscription}
              onResetUsage={handleResetSubscriptionUsage}
              actionPending={
                extendUserSubscription.isPending ||
                revokeUserSubscription.isPending ||
                restoreUserSubscription.isPending ||
                resetUserSubscriptionUsage.isPending
              }
              formatMicros={formatMicros}
            />
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

function ReportsTab({
  report,
  isLoading,
  reportFilter,
  setReportFilter,
  onApplyReportFilter,
  onResetReportFilter,
  onExport,
  exportPending,
  formatMicros,
}: {
  report?: BillingCommercialReport;
  isLoading: boolean;
  reportFilter: ReportFilterForm;
  setReportFilter: (value: ReportFilterForm | ((prev: ReportFilterForm) => ReportFilterForm)) => void;
  onApplyReportFilter: (event: FormEvent<HTMLFormElement>) => void;
  onResetReportFilter: () => void;
  onExport: (dataset: BillingCSVExportDataset) => void;
  exportPending: boolean;
  formatMicros: (value: number, valueCurrency?: string, minimumFractionDigits?: number) => string;
}) {
  const { t } = useTranslation();
  const summary = report?.summary;
  const reportCurrency = report?.currency || reportFilter.currency || 'CNY';
  const exportItems: Array<{ dataset: BillingCSVExportDataset; label: string }> = [
    { dataset: 'ledger_transactions', label: t('adminBilling.reports.exportLedger') },
    { dataset: 'usage_billing_records', label: t('adminBilling.reports.exportUsage') },
    { dataset: 'payment_orders', label: t('adminBilling.reports.exportOrders') },
    { dataset: 'payment_events', label: t('adminBilling.reports.exportEvents') },
  ];

  return (
    <div className='space-y-4'>
      <Card className='rounded-lg'>
        <CardHeader>
          <CardTitle className='flex items-center gap-2 text-base'>
            <BarChart3 className='size-4' />
            {t('adminBilling.reports.title')}
          </CardTitle>
          <CardDescription>{t('adminBilling.reports.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <form className='grid gap-3 md:grid-cols-4 xl:grid-cols-6' onSubmit={onApplyReportFilter}>
            <FilterInput
              label={t('adminBilling.filters.from')}
              type='datetime-local'
              value={reportFilter.from}
              onChange={(value) => setReportFilter((prev) => ({ ...prev, from: value }))}
            />
            <FilterInput
              label={t('adminBilling.filters.to')}
              type='datetime-local'
              value={reportFilter.to}
              onChange={(value) => setReportFilter((prev) => ({ ...prev, to: value }))}
            />
            <FilterInput
              label={t('adminBilling.reports.currency')}
              value={reportFilter.currency}
              onChange={(value) => setReportFilter((prev) => ({ ...prev, currency: value.toUpperCase() }))}
            />
            <FilterInput
              label={t('adminBilling.reports.limit')}
              value={reportFilter.limit}
              onChange={(value) => setReportFilter((prev) => ({ ...prev, limit: value }))}
            />
            <FilterActions onReset={onResetReportFilter} />
          </form>

          <div className='flex flex-wrap items-center gap-2 border-t pt-4'>
            {exportItems.map((item) => (
              <Button
                key={item.dataset}
                type='button'
                variant='outline'
                size='sm'
                disabled={exportPending}
                onClick={() => onExport(item.dataset)}
              >
                {exportPending ? <Loader2 className='size-4 animate-spin' /> : <Download className='size-4' />}
                {item.label}
              </Button>
            ))}
          </div>
        </CardContent>
      </Card>

      <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-6'>
        <MetricCard
          title={t('adminBilling.reports.summary.recharge')}
          value={formatMicros(summary?.rechargeAmountMicros ?? 0, reportCurrency)}
          loading={isLoading}
        />
        <MetricCard
          title={t('adminBilling.reports.summary.consumption')}
          value={formatMicros(summary?.consumptionAmountMicros ?? 0, reportCurrency)}
          loading={isLoading}
        />
        <MetricCard
          title={t('adminBilling.reports.summary.net')}
          value={formatMicros(summary?.netMovementMicros ?? 0, reportCurrency)}
          loading={isLoading}
        />
        <MetricCard
          title={t('adminBilling.reports.summary.refund')}
          value={formatMicros(summary?.refundAmountMicros ?? 0, reportCurrency)}
          loading={isLoading}
        />
        <MetricCard
          title={t('adminBilling.reports.summary.failedPayments')}
          value={`${summary?.failedPaymentCount ?? 0} / ${summary?.failedPaymentEventCount ?? 0}`}
          loading={isLoading}
        />
        <MetricCard
          title={t('adminBilling.reports.summary.pendingHolds')}
          value={`${formatMicros(summary?.pendingHoldAmountMicros ?? 0, reportCurrency)} (${summary?.pendingHoldCount ?? 0})`}
          loading={isLoading}
        />
      </div>

      <div className='grid gap-4 xl:grid-cols-2'>
        <Card className='rounded-lg'>
          <CardHeader>
            <CardTitle className='text-base'>{t('adminBilling.reports.topModels')}</CardTitle>
          </CardHeader>
          <CardContent className='overflow-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('adminBilling.columns.model')}</TableHead>
                  <TableHead className='text-right'>{t('adminBilling.columns.count')}</TableHead>
                  <TableHead className='text-right'>{t('adminBilling.columns.amount')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <DataStateRow colSpan={3} isLoading={isLoading} isEmpty={(report?.topModels ?? []).length === 0} />
                {report?.topModels.map((model) => (
                  <TableRow key={model.modelId}>
                    <TableCell className='max-w-[280px] truncate font-mono text-xs'>{model.modelId}</TableCell>
                    <TableCell className='text-right font-mono'>{model.requestCount}</TableCell>
                    <TableCell className='text-right font-mono'>{formatMicros(model.chargeAmountMicros, reportCurrency)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card className='rounded-lg'>
          <CardHeader>
            <CardTitle className='text-base'>{t('adminBilling.reports.topProjects')}</CardTitle>
          </CardHeader>
          <CardContent className='overflow-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('adminBilling.columns.project')}</TableHead>
                  <TableHead className='text-right'>{t('adminBilling.columns.count')}</TableHead>
                  <TableHead className='text-right'>{t('adminBilling.columns.amount')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <DataStateRow colSpan={3} isLoading={isLoading} isEmpty={(report?.topProjects ?? []).length === 0} />
                {report?.topProjects.map((project) => (
                  <TableRow key={project.projectId}>
                    <TableCell>
                      <div className='font-medium'>{project.projectName || `#${project.projectId}`}</div>
                      <div className='text-muted-foreground text-xs'>ID {project.projectId}</div>
                    </TableCell>
                    <TableCell className='text-right font-mono'>{project.requestCount}</TableCell>
                    <TableCell className='text-right font-mono'>{formatMicros(project.chargeAmountMicros, reportCurrency)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>

      <div className='grid gap-4 xl:grid-cols-2'>
        <Card className='rounded-lg'>
          <CardHeader>
            <CardTitle className='text-base'>{t('adminBilling.reports.topUsers')}</CardTitle>
          </CardHeader>
          <CardContent className='overflow-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('adminBilling.columns.user')}</TableHead>
                  <TableHead className='text-right'>{t('adminBilling.reports.summary.recharge')}</TableHead>
                  <TableHead className='text-right'>{t('adminBilling.reports.summary.consumption')}</TableHead>
                  <TableHead className='text-right'>{t('adminBilling.columns.net')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <DataStateRow colSpan={4} isLoading={isLoading} isEmpty={(report?.topUsers ?? []).length === 0} />
                {report?.topUsers.map((user) => (
                  <TableRow key={user.userId}>
                    <TableCell>
                      <div className='font-medium'>{user.email || `#${user.userId}`}</div>
                      <div className='text-muted-foreground text-xs'>ID {user.userId}</div>
                    </TableCell>
                    <TableCell className='text-right font-mono'>{formatMicros(user.rechargeAmountMicros, reportCurrency)}</TableCell>
                    <TableCell className='text-right font-mono'>{formatMicros(user.consumptionAmountMicros, reportCurrency)}</TableCell>
                    <TableCell className='text-right font-mono'>{formatMicros(user.netAmountMicros, reportCurrency)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card className='rounded-lg'>
          <CardHeader>
            <CardTitle className='text-base'>{t('adminBilling.reports.daily')}</CardTitle>
          </CardHeader>
          <CardContent className='overflow-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('adminBilling.columns.date')}</TableHead>
                  <TableHead className='text-right'>{t('adminBilling.reports.summary.recharge')}</TableHead>
                  <TableHead className='text-right'>{t('adminBilling.reports.summary.consumption')}</TableHead>
                  <TableHead className='text-right'>{t('adminBilling.columns.net')}</TableHead>
                  <TableHead className='text-right'>{t('adminBilling.reports.summary.failedPayments')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <DataStateRow colSpan={5} isLoading={isLoading} isEmpty={(report?.daily ?? []).length === 0} />
                {report?.daily.map((row) => (
                  <TableRow key={row.date}>
                    <TableCell>{row.date}</TableCell>
                    <TableCell className='text-right font-mono'>{formatMicros(row.rechargeAmountMicros, reportCurrency)}</TableCell>
                    <TableCell className='text-right font-mono'>{formatMicros(row.consumptionAmountMicros, reportCurrency)}</TableCell>
                    <TableCell className='text-right font-mono'>{formatMicros(row.netMovementMicros, reportCurrency)}</TableCell>
                    <TableCell className='text-right font-mono'>
                      {row.failedPaymentCount} / {row.failedPaymentEventCount}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
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

function RedeemCodesTab({
  users,
  rows,
  isLoading,
  filter,
  setFilter,
  onApplyFilter,
  onResetFilter,
  generateForm,
  setGenerateForm,
  onCreateRedeemCodes,
  createPending,
  grantForm,
  setGrantForm,
  onAdminCreateAndRedeemCode,
  grantPending,
  onUpdateStatus,
  updatePending,
  onDelete,
  deletePending,
  onExport,
  formatMicros,
}: {
  users: Array<{ id: string; email: string }>;
  rows: RedeemCode[];
  isLoading: boolean;
  filter: RedeemFilterForm;
  setFilter: (value: RedeemFilterForm | ((prev: RedeemFilterForm) => RedeemFilterForm)) => void;
  onApplyFilter: (event: FormEvent<HTMLFormElement>) => void;
  onResetFilter: () => void;
  generateForm: RedeemGenerateForm;
  setGenerateForm: (value: RedeemGenerateForm | ((prev: RedeemGenerateForm) => RedeemGenerateForm)) => void;
  onCreateRedeemCodes: (event: FormEvent<HTMLFormElement>) => void;
  createPending: boolean;
  grantForm: RedeemGrantForm;
  setGrantForm: (value: RedeemGrantForm | ((prev: RedeemGrantForm) => RedeemGrantForm)) => void;
  onAdminCreateAndRedeemCode: (event: FormEvent<HTMLFormElement>) => void;
  grantPending: boolean;
  onUpdateStatus: (code: RedeemCode, status: RedeemCodeStatus) => void;
  updatePending: boolean;
  onDelete: (code: RedeemCode) => void;
  deletePending: boolean;
  onExport: (rows: RedeemCode[]) => void;
  formatMicros: (value: number, valueCurrency?: string, minimumFractionDigits?: number) => string;
}) {
  const { t } = useTranslation();

  return (
    <div className='space-y-4'>
      <div className='grid gap-4 xl:grid-cols-2'>
        <Card className='rounded-lg'>
          <CardHeader>
            <CardTitle className='flex items-center gap-2 text-base'>
              <Ticket className='size-4' />
              {t('adminBilling.redeem.generateTitle')}
            </CardTitle>
            <CardDescription>{t('adminBilling.redeem.generateDescription')}</CardDescription>
          </CardHeader>
          <CardContent>
            <form className='grid gap-3 md:grid-cols-2' onSubmit={onCreateRedeemCodes}>
              <FilterInput
                label={t('adminBilling.redeem.count')}
                value={generateForm.count}
                onChange={(value) => setGenerateForm((prev) => ({ ...prev, count: value }))}
              />
              <FilterInput
                label={t('adminBilling.columns.amount')}
                value={generateForm.amount}
                onChange={(value) => setGenerateForm((prev) => ({ ...prev, amount: value }))}
              />
              <FilterInput
                label={t('adminBilling.columns.currency')}
                value={generateForm.currency}
                onChange={(value) => setGenerateForm((prev) => ({ ...prev, currency: value.toUpperCase() }))}
              />
              <FilterInput
                label={t('adminBilling.redeem.prefix')}
                value={generateForm.prefix}
                onChange={(value) => setGenerateForm((prev) => ({ ...prev, prefix: value.toUpperCase() }))}
              />
              <FilterInput
                label={t('adminBilling.columns.expiresAt')}
                type='datetime-local'
                value={generateForm.expiresAt}
                onChange={(value) => setGenerateForm((prev) => ({ ...prev, expiresAt: value }))}
              />
              <FilterInput
                label={t('adminBilling.redeem.notes')}
                value={generateForm.notes}
                onChange={(value) => setGenerateForm((prev) => ({ ...prev, notes: value }))}
              />
              <div className='flex items-end md:col-span-2'>
                <Button type='submit' disabled={createPending}>
                  {createPending ? <Loader2 className='size-4 animate-spin' /> : <Ticket className='size-4' />}
                  {t('adminBilling.redeem.generate')}
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>

        <Card className='rounded-lg'>
          <CardHeader>
            <CardTitle className='text-base'>{t('adminBilling.redeem.createAndRedeemTitle')}</CardTitle>
            <CardDescription>{t('adminBilling.redeem.createAndRedeemDescription')}</CardDescription>
          </CardHeader>
          <CardContent>
            <form className='grid gap-3 md:grid-cols-2' onSubmit={onAdminCreateAndRedeemCode}>
              <div className='md:col-span-2'>
                <UserSelect
                  users={users}
                  value={grantForm.userId}
                  onChange={(value) => setGrantForm((prev) => ({ ...prev, userId: value }))}
                  label={t('adminBilling.adjust.user')}
                  placeholder={t('adminBilling.adjust.userPlaceholder')}
                />
              </div>
              <FilterInput
                label={t('adminBilling.columns.amount')}
                value={grantForm.amount}
                onChange={(value) => setGrantForm((prev) => ({ ...prev, amount: value }))}
              />
              <FilterInput
                label={t('adminBilling.columns.currency')}
                value={grantForm.currency}
                onChange={(value) => setGrantForm((prev) => ({ ...prev, currency: value.toUpperCase() }))}
              />
              <FilterInput
                label={t('adminBilling.columns.expiresAt')}
                type='datetime-local'
                value={grantForm.expiresAt}
                onChange={(value) => setGrantForm((prev) => ({ ...prev, expiresAt: value }))}
              />
              <FilterInput
                label={t('adminBilling.redeem.notes')}
                value={grantForm.notes}
                onChange={(value) => setGrantForm((prev) => ({ ...prev, notes: value }))}
              />
              <div className='flex items-end md:col-span-2'>
                <Button type='submit' disabled={grantPending}>
                  {grantPending ? <Loader2 className='size-4 animate-spin' /> : <Save className='size-4' />}
                  {t('adminBilling.redeem.createAndRedeem')}
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      </div>

      <Card className='rounded-lg'>
        <CardHeader>
          <div className='flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between'>
            <div>
              <CardTitle className='text-base'>{t('adminBilling.redeem.title')}</CardTitle>
              <CardDescription>{t('adminBilling.redeem.description')}</CardDescription>
            </div>
            <Button type='button' variant='outline' size='sm' onClick={() => onExport(rows)} disabled={rows.length === 0}>
              <Download className='size-4' />
              {t('adminBilling.redeem.export')}
            </Button>
          </div>
        </CardHeader>
        <CardContent className='space-y-4 overflow-auto'>
          <form className='grid gap-3 md:grid-cols-4 xl:grid-cols-9' onSubmit={onApplyFilter}>
            <FilterInput
              label={t('adminBilling.filters.userId')}
              value={filter.userId}
              onChange={(value) => setFilter((prev) => ({ ...prev, userId: value }))}
            />
            <FilterInput
              label={t('adminBilling.redeem.createdById')}
              value={filter.createdById}
              onChange={(value) => setFilter((prev) => ({ ...prev, createdById: value }))}
            />
            <FilterSelect
              label={t('adminBilling.columns.status')}
              value={filter.status}
              onChange={(value) => setFilter((prev) => ({ ...prev, status: value as RedeemFilterForm['status'] }))}
              options={['all', 'active', 'used', 'disabled', 'expired']}
            />
            <FilterSelect
              label={t('adminBilling.columns.type')}
              value={filter.type}
              onChange={(value) => setFilter((prev) => ({ ...prev, type: value as RedeemFilterForm['type'] }))}
              options={['all', 'balance', 'credit', 'subscription']}
            />
            <FilterInput
              label={t('adminBilling.redeem.code')}
              value={filter.code}
              onChange={(value) => setFilter((prev) => ({ ...prev, code: value }))}
            />
            <FilterInput
              label={t('adminBilling.redeem.batchId')}
              value={filter.batchId}
              onChange={(value) => setFilter((prev) => ({ ...prev, batchId: value }))}
            />
            <FilterInput
              label={t('adminBilling.filters.from')}
              type='datetime-local'
              value={filter.from}
              onChange={(value) => setFilter((prev) => ({ ...prev, from: value }))}
            />
            <FilterInput
              label={t('adminBilling.filters.to')}
              type='datetime-local'
              value={filter.to}
              onChange={(value) => setFilter((prev) => ({ ...prev, to: value }))}
            />
            <FilterInput
              label={t('adminBilling.filters.expiresBefore')}
              type='datetime-local'
              value={filter.expiresBefore}
              onChange={(value) => setFilter((prev) => ({ ...prev, expiresBefore: value }))}
            />
            <FilterActions onReset={onResetFilter} />
          </form>

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('adminBilling.redeem.code')}</TableHead>
                <TableHead>{t('adminBilling.columns.type')}</TableHead>
                <TableHead>{t('adminBilling.columns.status')}</TableHead>
                <TableHead>{t('adminBilling.columns.createdAt')}</TableHead>
                <TableHead>{t('adminBilling.columns.expiresAt')}</TableHead>
                <TableHead>{t('adminBilling.redeem.usedById')}</TableHead>
                <TableHead>{t('adminBilling.redeem.batchId')}</TableHead>
                <TableHead className='text-right'>{t('adminBilling.columns.amount')}</TableHead>
                <TableHead>{t('adminBilling.columns.action')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <DataStateRow colSpan={9} isLoading={isLoading} isEmpty={rows.length === 0} />
              {rows.map((code) => (
                <TableRow key={code.id}>
                  <TableCell>
                    <div className='font-mono text-xs'>{code.code}</div>
                    {code.notes && <div className='text-muted-foreground max-w-[180px] truncate text-xs'>{code.notes}</div>}
                  </TableCell>
                  <TableCell>{code.type}</TableCell>
                  <TableCell>
                    <StatusBadge value={code.status} positive={code.status === 'used'} />
                  </TableCell>
                  <TableCell>{formatDate(code.createdAt)}</TableCell>
                  <TableCell>{formatDate(code.expiresAt)}</TableCell>
                  <TableCell className='font-mono text-xs'>{code.usedByID || '-'}</TableCell>
                  <TableCell className='font-mono text-xs'>{code.batchID || '-'}</TableCell>
                  <TableCell className='text-right font-mono'>{formatMicros(code.amountMicros, code.currency)}</TableCell>
                  <TableCell className='min-w-[250px]'>
                    <div className='flex flex-wrap items-center gap-2'>
                      {code.status === 'active' && (
                        <>
                          <Button
                            type='button'
                            size='sm'
                            variant='outline'
                            disabled={updatePending}
                            onClick={() => onUpdateStatus(code, 'disabled')}
                          >
                            {updatePending ? <Loader2 className='size-4 animate-spin' /> : <Ban className='size-4' />}
                            {t('adminBilling.redeem.disable')}
                          </Button>
                          <Button
                            type='button'
                            size='sm'
                            variant='outline'
                            disabled={updatePending}
                            onClick={() => onUpdateStatus(code, 'expired')}
                          >
                            {updatePending ? <Loader2 className='size-4 animate-spin' /> : <Clock className='size-4' />}
                            {t('adminBilling.redeem.expire')}
                          </Button>
                        </>
                      )}
                      {code.status !== 'used' && (
                        <Button
                          type='button'
                          size='sm'
                          variant='destructive'
                          disabled={deletePending}
                          onClick={() => onDelete(code)}
                        >
                          {deletePending ? <Loader2 className='size-4 animate-spin' /> : <Trash2 className='size-4' />}
                          {t('adminBilling.redeem.delete')}
                        </Button>
                      )}
                      {code.status === 'used' && <span className='text-muted-foreground text-xs'>{formatDate(code.usedAt)}</span>}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}

function SubscriptionsTab({
  users,
  plans,
  subscriptions,
  plansLoading,
  subscriptionsLoading,
  planForm,
  setPlanForm,
  onSavePlan,
  savePlanPending,
  onDeletePlan,
  deletePlanPending,
  assignForm,
  setAssignForm,
  onAssign,
  assignPending,
  filter,
  setFilter,
  onApplyFilter,
  onResetFilter,
  actionValues,
  setActionValues,
  onExtend,
  onRevoke,
  onRestore,
  onResetUsage,
  actionPending,
  formatMicros,
}: {
  users: Array<{ id: string; email: string }>;
  plans: SubscriptionPlan[];
  subscriptions: UserSubscription[];
  plansLoading: boolean;
  subscriptionsLoading: boolean;
  planForm: SubscriptionPlanForm;
  setPlanForm: (value: SubscriptionPlanForm | ((prev: SubscriptionPlanForm) => SubscriptionPlanForm)) => void;
  onSavePlan: (event: FormEvent<HTMLFormElement>) => void;
  savePlanPending: boolean;
  onDeletePlan: (plan: SubscriptionPlan) => void;
  deletePlanPending: boolean;
  assignForm: SubscriptionAssignForm;
  setAssignForm: (value: SubscriptionAssignForm | ((prev: SubscriptionAssignForm) => SubscriptionAssignForm)) => void;
  onAssign: (event: FormEvent<HTMLFormElement>) => void;
  assignPending: boolean;
  filter: SubscriptionFilterForm;
  setFilter: (value: SubscriptionFilterForm | ((prev: SubscriptionFilterForm) => SubscriptionFilterForm)) => void;
  onApplyFilter: (event: FormEvent<HTMLFormElement>) => void;
  onResetFilter: () => void;
  actionValues: Record<string, SubscriptionActionForm>;
  setActionValues: (value: Record<string, SubscriptionActionForm> | ((prev: Record<string, SubscriptionActionForm>) => Record<string, SubscriptionActionForm>)) => void;
  onExtend: (subscription: UserSubscription) => void;
  onRevoke: (subscription: UserSubscription) => void;
  onRestore: (subscription: UserSubscription) => void;
  onResetUsage: (subscription: UserSubscription) => void;
  actionPending: boolean;
  formatMicros: (value: number, valueCurrency?: string, minimumFractionDigits?: number) => string;
}) {
  const { t } = useTranslation();
  const updateAction = (subscriptionID: string, patch: Partial<SubscriptionActionForm>) => {
    setActionValues((prev) => ({
      ...prev,
      [subscriptionID]: { ...defaultSubscriptionActionForm(), ...(prev[subscriptionID] ?? {}), ...patch },
    }));
  };

  return (
    <div className='space-y-4'>
      <div className='grid gap-4 xl:grid-cols-[minmax(0,1fr)_420px]'>
        <Card className='rounded-lg'>
          <CardHeader>
            <CardTitle className='flex items-center gap-2 text-base'>
              <PackageCheck className='size-4' />
              {t('adminBilling.subscriptions.plansTitle')}
            </CardTitle>
            <CardDescription>{t('adminBilling.subscriptions.plansDescription')}</CardDescription>
          </CardHeader>
          <CardContent className='space-y-4'>
            <form className='grid gap-3 md:grid-cols-3 xl:grid-cols-6' onSubmit={onSavePlan}>
              <FilterInput label={t('adminBilling.subscriptions.planName')} value={planForm.name} onChange={(value) => setPlanForm((prev) => ({ ...prev, name: value }))} />
              <FilterInput label={t('adminBilling.subscriptions.description')} value={planForm.description} onChange={(value) => setPlanForm((prev) => ({ ...prev, description: value }))} />
              <FilterSelect label={t('adminBilling.subscriptions.period')} value={planForm.period} onChange={(value) => setPlanForm((prev) => ({ ...prev, period: value as SubscriptionPlanPeriod }))} options={['day', 'month', 'year', 'custom']} />
              <FilterInput label={t('adminBilling.subscriptions.periodDays')} value={planForm.periodDays} onChange={(value) => setPlanForm((prev) => ({ ...prev, periodDays: value }))} />
              <FilterInput label={t('adminBilling.subscriptions.price')} value={planForm.price} onChange={(value) => setPlanForm((prev) => ({ ...prev, price: value }))} />
              <FilterInput label={t('adminBilling.columns.currency')} value={planForm.currency} onChange={(value) => setPlanForm((prev) => ({ ...prev, currency: value }))} />
              <FilterInput label={t('adminBilling.subscriptions.includedAmount')} value={planForm.includedAmount} onChange={(value) => setPlanForm((prev) => ({ ...prev, includedAmount: value }))} />
              <FilterInput label={t('adminBilling.subscriptions.modelsCSV')} value={planForm.supportedModelIDs} onChange={(value) => setPlanForm((prev) => ({ ...prev, supportedModelIDs: value }))} />
              <FilterInput label={t('adminBilling.subscriptions.projectsCSV')} value={planForm.supportedProjectIDs} onChange={(value) => setPlanForm((prev) => ({ ...prev, supportedProjectIDs: value }))} />
              <FilterInput label={t('adminBilling.subscriptions.groupsCSV')} value={planForm.supportedGroupIDs} onChange={(value) => setPlanForm((prev) => ({ ...prev, supportedGroupIDs: value }))} />
              <FilterSelect label={t('adminBilling.columns.status')} value={planForm.status} onChange={(value) => setPlanForm((prev) => ({ ...prev, status: value as SubscriptionPlanStatus }))} options={['enabled', 'disabled', 'archived']} />
              <FilterInput label={t('adminBilling.subscriptions.sortOrder')} value={planForm.sortOrder} onChange={(value) => setPlanForm((prev) => ({ ...prev, sortOrder: value }))} />
              <div className='flex items-end gap-2 md:col-span-3 xl:col-span-6'>
                <div className='flex h-10 items-center gap-2 rounded-md border px-3'>
                  <Switch checked={planForm.allowWalletFallback} onCheckedChange={(checked) => setPlanForm((prev) => ({ ...prev, allowWalletFallback: checked }))} />
                  <span className='text-sm'>{t('adminBilling.subscriptions.allowWalletFallback')}</span>
                </div>
                <Button type='submit' disabled={savePlanPending}>
                  {savePlanPending ? <Loader2 className='size-4 animate-spin' /> : <Save className='size-4' />}
                  {planForm.id ? t('adminBilling.subscriptions.updatePlan') : t('adminBilling.subscriptions.createPlan')}
                </Button>
                <Button type='button' variant='outline' onClick={() => setPlanForm(defaultSubscriptionPlanForm())}>
                  {t('adminBilling.subscriptions.newPlan')}
                </Button>
              </div>
            </form>

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('adminBilling.subscriptions.planName')}</TableHead>
                  <TableHead>{t('adminBilling.subscriptions.period')}</TableHead>
                  <TableHead>{t('adminBilling.columns.status')}</TableHead>
                  <TableHead>{t('adminBilling.subscriptions.scope')}</TableHead>
                  <TableHead className='text-right'>{t('adminBilling.subscriptions.price')}</TableHead>
                  <TableHead className='text-right'>{t('adminBilling.subscriptions.includedAmount')}</TableHead>
                  <TableHead>{t('adminBilling.columns.action')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <DataStateRow colSpan={7} isLoading={plansLoading} isEmpty={plans.length === 0} />
                {plans.map((plan) => (
                  <TableRow key={plan.id}>
                    <TableCell>
                      <div className='font-medium'>{plan.name}</div>
                      <div className='text-muted-foreground max-w-[240px] truncate text-xs'>{plan.description || '-'}</div>
                    </TableCell>
                    <TableCell>{plan.period} / {plan.periodDays}d</TableCell>
                    <TableCell><StatusBadge value={plan.status} positive={plan.status === 'enabled'} /></TableCell>
                    <TableCell className='max-w-[240px] truncate text-xs'>
                      {plan.supportedModelIds.length > 0 ? plan.supportedModelIds.join(', ') : t('adminBilling.subscriptions.allModels')}
                    </TableCell>
                    <TableCell className='text-right font-mono'>{formatMicros(plan.priceMicros, plan.currency)}</TableCell>
                    <TableCell className='text-right font-mono'>
                      {plan.includedAmountMicros > 0 ? formatMicros(plan.includedAmountMicros, plan.currency) : t('adminBilling.subscriptions.unlimited')}
                    </TableCell>
                    <TableCell>
                      <div className='flex gap-2'>
                        <Button type='button' size='sm' variant='outline' onClick={() => setPlanForm(subscriptionPlanFormFromPlan(plan))}>
                          {t('adminBilling.subscriptions.edit')}
                        </Button>
                        <Button type='button' size='sm' variant='destructive' disabled={deletePlanPending} onClick={() => onDeletePlan(plan)}>
                          {deletePlanPending ? <Loader2 className='size-4 animate-spin' /> : <Trash2 className='size-4' />}
                          {t('adminBilling.subscriptions.delete')}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card className='rounded-lg'>
          <CardHeader>
            <CardTitle className='text-base'>{t('adminBilling.subscriptions.assignTitle')}</CardTitle>
            <CardDescription>{t('adminBilling.subscriptions.assignDescription')}</CardDescription>
          </CardHeader>
          <CardContent>
            <form className='space-y-4' onSubmit={onAssign}>
              <UserSelect users={users} value={assignForm.userId} onChange={(value) => setAssignForm((prev) => ({ ...prev, userId: value }))} label={t('adminBilling.adjust.user')} placeholder={t('adminBilling.adjust.userPlaceholder')} />
              <div className='space-y-2'>
                <Label>{t('adminBilling.subscriptions.plan')}</Label>
                <Select value={assignForm.planId} onValueChange={(value) => setAssignForm((prev) => ({ ...prev, planId: value }))}>
                  <SelectTrigger><SelectValue placeholder={t('adminBilling.subscriptions.planPlaceholder')} /></SelectTrigger>
                  <SelectContent>
                    {plans.map((plan) => <SelectItem key={plan.id} value={plan.id}>{plan.name}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
              <div className='grid grid-cols-2 gap-3'>
                <FilterInput label={t('adminBilling.subscriptions.startsAt')} type='datetime-local' value={assignForm.startsAt} onChange={(value) => setAssignForm((prev) => ({ ...prev, startsAt: value }))} />
                <FilterInput label={t('adminBilling.columns.expiresAt')} type='datetime-local' value={assignForm.expiresAt} onChange={(value) => setAssignForm((prev) => ({ ...prev, expiresAt: value }))} />
              </div>
              <FilterInput label={t('adminBilling.redeem.notes')} value={assignForm.notes} onChange={(value) => setAssignForm((prev) => ({ ...prev, notes: value }))} />
              <Button className='w-full' type='submit' disabled={assignPending}>
                {assignPending ? <Loader2 className='size-4 animate-spin' /> : <PackageCheck className='size-4' />}
                {t('adminBilling.subscriptions.assign')}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>

      <Card className='rounded-lg'>
        <CardHeader>
          <CardTitle className='text-base'>{t('adminBilling.subscriptions.usersTitle')}</CardTitle>
          <CardDescription>{t('adminBilling.subscriptions.usersDescription')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-4 overflow-auto'>
          <form className='grid gap-3 md:grid-cols-4 xl:grid-cols-7' onSubmit={onApplyFilter}>
            <FilterInput label={t('adminBilling.filters.userId')} value={filter.userId} onChange={(value) => setFilter((prev) => ({ ...prev, userId: value }))} />
            <FilterInput label={t('adminBilling.subscriptions.planId')} value={filter.planId} onChange={(value) => setFilter((prev) => ({ ...prev, planId: value }))} />
            <FilterSelect label={t('adminBilling.columns.status')} value={filter.status} onChange={(value) => setFilter((prev) => ({ ...prev, status: value as SubscriptionFilterForm['status'] }))} options={['all', 'active', 'expired', 'revoked', 'canceled']} />
            <FilterInput label={t('adminBilling.filters.from')} type='datetime-local' value={filter.from} onChange={(value) => setFilter((prev) => ({ ...prev, from: value }))} />
            <FilterInput label={t('adminBilling.filters.to')} type='datetime-local' value={filter.to} onChange={(value) => setFilter((prev) => ({ ...prev, to: value }))} />
            <FilterInput label={t('adminBilling.filters.expiresBefore')} type='datetime-local' value={filter.expiresBefore} onChange={(value) => setFilter((prev) => ({ ...prev, expiresBefore: value }))} />
            <FilterActions onReset={onResetFilter} />
          </form>

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('adminBilling.columns.user')}</TableHead>
                <TableHead>{t('adminBilling.subscriptions.plan')}</TableHead>
                <TableHead>{t('adminBilling.columns.status')}</TableHead>
                <TableHead>{t('adminBilling.subscriptions.usage')}</TableHead>
                <TableHead>{t('adminBilling.columns.expiresAt')}</TableHead>
                <TableHead>{t('adminBilling.columns.action')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <DataStateRow colSpan={6} isLoading={subscriptionsLoading} isEmpty={subscriptions.length === 0} />
              {subscriptions.map((subscription) => {
                const action = actionValues[subscription.id] ?? defaultSubscriptionActionForm();
                return (
                  <TableRow key={subscription.id}>
                    <TableCell className='font-mono text-xs'>{subscription.userID}</TableCell>
                    <TableCell>
                      <div className='font-medium'>{subscription.plan?.name || '-'}</div>
                      <div className='text-muted-foreground text-xs'>{subscription.planID || '-'}</div>
                    </TableCell>
                    <TableCell><StatusBadge value={subscription.status} positive={subscription.status === 'active'} /></TableCell>
                    <TableCell className='min-w-[220px]'>
                      <div className='mb-1 flex justify-between gap-3 text-xs'>
                        <span className='font-mono'>{formatMicros(subscription.usedAmountMicros, subscription.currency)}</span>
                        <span className='text-muted-foreground'>
                          {subscription.includedAmountMicros > 0 ? formatMicros(subscription.includedAmountMicros, subscription.currency) : t('adminBilling.subscriptions.unlimited')}
                        </span>
                      </div>
                      <Progress value={usagePercent(subscription)} />
                    </TableCell>
                    <TableCell>
                      <div>{formatDate(subscription.expiresAt)}</div>
                      <div className='text-muted-foreground text-xs'>{t('adminBilling.subscriptions.resetAt')}: {formatDate(subscription.resetAt)}</div>
                    </TableCell>
                    <TableCell className='min-w-[520px]'>
                      <div className='grid gap-2 md:grid-cols-[80px_170px_150px_1fr]'>
                        <Input className='h-8' value={action.days} onChange={(event) => updateAction(subscription.id, { days: event.target.value })} placeholder={t('adminBilling.subscriptions.days')} />
                        <Input className='h-8' type='datetime-local' value={action.expiresAt} onChange={(event) => updateAction(subscription.id, { expiresAt: event.target.value })} />
                        <Input className='h-8' value={action.reason} onChange={(event) => updateAction(subscription.id, { reason: event.target.value })} placeholder={t('adminBilling.columns.reason')} />
                        <div className='flex flex-wrap gap-2'>
                          <Button type='button' size='sm' variant='outline' disabled={actionPending} onClick={() => onExtend(subscription)}>
                            {actionPending ? <Loader2 className='size-4 animate-spin' /> : <Clock className='size-4' />}
                            {t('adminBilling.subscriptions.extend')}
                          </Button>
                          <Button type='button' size='sm' variant='outline' disabled={actionPending} onClick={() => onResetUsage(subscription)}>
                            <RotateCcw className='size-4' />
                            {t('adminBilling.subscriptions.resetUsage')}
                          </Button>
                          {subscription.status === 'revoked' ? (
                            <Button type='button' size='sm' disabled={actionPending} onClick={() => onRestore(subscription)}>
                              <Unlock className='size-4' />
                              {t('adminBilling.subscriptions.restore')}
                            </Button>
                          ) : (
                            <Button type='button' size='sm' variant='destructive' disabled={actionPending} onClick={() => onRevoke(subscription)}>
                              <Ban className='size-4' />
                              {t('adminBilling.subscriptions.revoke')}
                            </Button>
                          )}
                        </div>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
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
