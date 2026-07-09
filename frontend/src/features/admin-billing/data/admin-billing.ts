import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { toast } from 'sonner';
import i18n from '@/lib/i18n';

export type BillingAccountStatus = 'active' | 'frozen' | 'closed';
export type LedgerTransactionDirection = 'credit' | 'debit';
export type LedgerTransactionStatus = 'posted' | 'voided';
export type LedgerTransactionType =
  | 'payment_recharge'
  | 'usage_charge'
  | 'admin_adjustment'
  | 'refund'
  | 'chargeback'
  | 'subscription_grant'
  | 'subscription_deduct'
  | 'redeem_code'
  | 'affiliate_rebate';
export type UsageBillingRecordStatus = 'pending' | 'charged' | 'skipped' | 'failed' | 'refunded';
export type PaymentOrderStatus = 'pending' | 'paid' | 'failed' | 'canceled' | 'expired' | 'refunded';
export type PaymentProviderType = 'manual' | 'epay' | 'stripe' | 'custom';
export type PaymentEventStatus = 'received' | 'processed' | 'failed' | 'ignored';
export type BillingHoldStatus = 'held' | 'captured' | 'released' | 'expired';
export type RedeemCodeStatus = 'active' | 'used' | 'disabled' | 'expired';
export type RedeemCodeType = 'balance' | 'credit' | 'subscription';
export type SubscriptionPlanPeriod = 'day' | 'month' | 'year' | 'custom';
export type SubscriptionPlanStatus = 'enabled' | 'disabled' | 'archived';
export type UserSubscriptionStatus = 'active' | 'expired' | 'revoked' | 'canceled';
export type PromoCodeStatus = 'active' | 'disabled' | 'expired';
export type PromoCodeScope = 'all' | 'recharge' | 'subscription';
export type PromoCodeDiscountType = 'amount' | 'percent';
export type PromoUsageStatus = 'reserved' | 'applied' | 'voided';
export type PromoUsageScope = 'recharge' | 'subscription';
export type AffiliateProfileStatus = 'active' | 'disabled';
export type AffiliateInvitationStatus = 'active' | 'canceled';
export type AffiliateRebateStatus = 'frozen' | 'available' | 'transferred' | 'voided';
export type AffiliateRebateSourceType = 'payment_order' | 'user_subscription';
export type BillingNotificationAudience = 'user' | 'operator';
export type BillingNotificationCategory = 'low_balance' | 'payment' | 'subscription' | 'large_consumption' | 'operator_alert';
export type BillingNotificationSeverity = 'info' | 'warning' | 'error';
export type BillingNotificationStatus = 'unread' | 'read' | 'dismissed';
export type CommercialSettingMode = 'disabled' | 'warn' | 'enforce';
export type BillingAuditActorType = 'admin' | 'system';

export interface BillingAccount {
  id: string;
  ownerType: string;
  ownerID: number;
  currency: string;
  balanceMicros: number;
  heldBalanceMicros: number;
  creditLimitMicros: number;
  status: BillingAccountStatus;
}

export interface PaymentProviderInstance {
  id: string;
  createdAt: string;
  updatedAt: string;
  name: string;
  providerType: string;
  status: string;
  currency: string;
}

export interface BillingPriceRule {
  id: string;
  createdAt: string;
  updatedAt: string;
  scopeType: 'global' | 'project' | 'group';
  scopeID: number;
  modelPattern: string;
  price: ModelPrice;
  currency: string;
  priority: number;
  enabled: boolean;
  referenceID: string;
}

export interface ModelPrice {
  items: Array<{
    itemCode: string;
    pricing: {
      mode: string;
      flatFee?: string | null;
      usagePerUnit?: string | null;
    };
  }>;
}

export interface LedgerTransaction {
  id: string;
  createdAt: string;
  billingAccountID: string;
  direction: LedgerTransactionDirection;
  amountMicros: number;
  currency: string;
  type: LedgerTransactionType;
  status: LedgerTransactionStatus;
  referenceType: string;
  referenceID: string;
  memo: string;
  createdByType: string;
  createdByID: string;
}

export interface PaymentOrder {
  id: string;
  createdAt: string;
  orderNo: string;
  projectID: number;
  billingAccountID: string;
  providerType: PaymentProviderType;
  amountMicros: number;
  currency: string;
  status: PaymentOrderStatus;
  externalTradeNo?: string | null;
  paidAt?: string | null;
  expiresAt?: string | null;
  canceledAt?: string | null;
  cancelReason: string;
  makeupReason: string;
  failureReason: string;
  refundedAt?: string | null;
  refundReason: string;
  refundAmountMicros: number;
}

export interface UsageBillingRecord {
  id: string;
  createdAt: string;
  billingAccountID: string;
  projectID: number;
  userID?: number | null;
  apiKeyID?: number | null;
  modelID: string;
  costAmountMicros: number;
  chargeAmountMicros: number;
  currency: string;
  status: UsageBillingRecordStatus;
  error: string;
}

export interface BillingHold {
  id: string;
  createdAt: string;
  billingAccountID: string;
  requestID?: string | null;
  usageLogID?: string | null;
  projectID?: number | null;
  userID?: number | null;
  apiKeyID?: number | null;
  modelID: string;
  amountMicros: number;
  capturedAmountMicros: number;
  currency: string;
  status: BillingHoldStatus;
  idempotencyKey: string;
  referenceType: string;
  referenceID: string;
  releaseReason: string;
  releasedByType: string;
  releasedByID: string;
  expiresAt: string;
  capturedAt?: string | null;
  releasedAt?: string | null;
}

export interface PaymentEvent {
  id: string;
  createdAt: string;
  eventKey: string;
  paymentOrderID?: string | null;
  providerInstanceID?: string | null;
  providerType: PaymentProviderType;
  eventType: string;
  payload?: unknown;
  status: PaymentEventStatus;
  error: string;
}

export interface RedeemCode {
  id: string;
  createdAt: string;
  updatedAt: string;
  code: string;
  type: RedeemCodeType;
  status: RedeemCodeStatus;
  amountMicros: number;
  currency: string;
  createdByID?: string | null;
  usedByID?: string | null;
  usedAt?: string | null;
  expiresAt?: string | null;
  notes: string;
  ledgerTransactionID?: string | null;
  batchID: string;
}

export interface SubscriptionPlan {
  id: string;
  createdAt: string;
  updatedAt: string;
  name: string;
  description: string;
  period: SubscriptionPlanPeriod;
  periodDays: number;
  priceMicros: number;
  currency: string;
  includedAmountMicros: number;
  supportedModelIds: string[];
  supportedProjectIds: number[];
  supportedGroupIds: number[];
  allowWalletFallback: boolean;
  status: SubscriptionPlanStatus;
  sortOrder: number;
}

export interface UserSubscription {
  id: string;
  createdAt: string;
  updatedAt: string;
  userID: string;
  planID?: string | null;
  status: UserSubscriptionStatus;
  startsAt: string;
  expiresAt: string;
  currentPeriodStart: string;
  currentPeriodEnd: string;
  resetAt: string;
  periodDays: number;
  includedAmountMicros: number;
  usedAmountMicros: number;
  currency: string;
  supportedModelIds: string[];
  supportedProjectIds: number[];
  supportedGroupIds: number[];
  allowWalletFallback: boolean;
  assignedByID?: string | null;
  purchaseLedgerTransactionID?: string | null;
  notes: string;
  revokeReason: string;
  plan?: Pick<SubscriptionPlan, 'id' | 'name' | 'period' | 'priceMicros' | 'currency'> | null;
}

export interface PromoCode {
  id: string;
  createdAt: string;
  updatedAt: string;
  code: string;
  description: string;
  discountType: PromoCodeDiscountType;
  discountAmountMicros: number;
  discountPercentBps: number;
  scope: PromoCodeScope;
  status: PromoCodeStatus;
  currency: string;
  maxUses: number;
  usedCount: number;
  perUserLimit: number;
  startsAt?: string | null;
  expiresAt?: string | null;
  createdByID?: string | null;
  notes: string;
}

export interface PromoUsage {
  id: string;
  createdAt: string;
  promoCodeID: string;
  code: string;
  userID?: string | null;
  billingAccountID?: string | null;
  paymentOrderID?: string | null;
  userSubscriptionID?: string | null;
  ledgerTransactionID?: string | null;
  scope: PromoUsageScope;
  status: PromoUsageStatus;
  originalAmountMicros: number;
  discountAmountMicros: number;
  payableAmountMicros: number;
  currency: string;
  idempotencyKey: string;
}

export interface AffiliateSetting {
  id: string;
  createdAt: string;
  updatedAt: string;
  enabled: boolean;
  defaultRebateRateBps: number;
  freezeDays: number;
  minTransferMicros: number;
  currency: string;
}

export interface AffiliateProfile {
  id: string;
  createdAt: string;
  updatedAt: string;
  userID: string;
  inviteCode: string;
  status: AffiliateProfileStatus;
  rebateRateOverrideBps?: number | null;
  notes: string;
}

export interface AffiliateInvitation {
  id: string;
  createdAt: string;
  updatedAt: string;
  inviterUserID: string;
  inviteeUserID: string;
  inviteCode: string;
  status: AffiliateInvitationStatus;
  notes: string;
}

export interface AffiliateRebate {
  id: string;
  createdAt: string;
  updatedAt: string;
  invitationID: string;
  inviterUserID: string;
  inviteeUserID: string;
  sourceType: AffiliateRebateSourceType;
  sourceID: number;
  paymentOrderID?: string | null;
  userSubscriptionID?: string | null;
  ledgerTransactionID?: string | null;
  baseAmountMicros: number;
  amountMicros: number;
  rateBps: number;
  currency: string;
  status: AffiliateRebateStatus;
  freezeUntil: string;
  transferredAt?: string | null;
}

export interface BillingNotificationSetting {
  id: string;
  createdAt: string;
  updatedAt: string;
  enabled: boolean;
  userNotificationsEnabled: boolean;
  operatorAlertsEnabled: boolean;
  lowBalanceThresholdMicros: number;
  largeConsumptionThresholdMicros: number;
  subscriptionExpiryWarningDays: number;
  currency: string;
}

export interface BillingNotification {
  id: string;
  createdAt: string;
  updatedAt: string;
  userID?: string | null;
  audience: BillingNotificationAudience;
  category: BillingNotificationCategory;
  severity: BillingNotificationSeverity;
  status: BillingNotificationStatus;
  eventKey: string;
  title: string;
  message: string;
  currency: string;
  amountMicros?: number | null;
  billingAccountID?: string | null;
  paymentOrderID?: string | null;
  userSubscriptionID?: string | null;
  usageBillingRecordID?: string | null;
  readAt?: string | null;
}

export interface CommercialSetting {
  id: string;
  createdAt: string;
  updatedAt: string;
  key: string;
  mode: CommercialSettingMode;
  requireAdminActionReason: boolean;
  paymentProviderSecretsEncrypted: boolean;
  workersEnabled: boolean;
  orderExpiryWorkerEnabled: boolean;
  holdExpiryWorkerEnabled: boolean;
  subscriptionExpiryWorkerEnabled: boolean;
  subscriptionResetWorkerEnabled: boolean;
  affiliateRebateThawWorkerEnabled: boolean;
  failedBillingRetryWorkerEnabled: boolean;
  workerBatchSize: number;
  currency: string;
}

export interface BillingAuditLog {
  id: string;
  createdAt: string;
  updatedAt: string;
  action: string;
  actorType: BillingAuditActorType;
  actorUserID?: number | null;
  targetType: string;
  targetID: string;
  targetUserID?: number | null;
  reason: string;
  metadata?: unknown;
}

export interface AdminLedgerTransactionsFilter {
  userId?: number;
  billingAccountId?: number;
  direction?: LedgerTransactionDirection;
  status?: LedgerTransactionStatus;
  type?: LedgerTransactionType;
  referenceType?: string;
  from?: string;
  to?: string;
}

export interface AdminUsageBillingRecordsFilter {
  userId?: number;
  projectId?: number;
  apiKeyId?: number;
  billingAccountId?: number;
  modelId?: string;
  status?: UsageBillingRecordStatus;
  from?: string;
  to?: string;
}

export interface AdminBillingHoldsFilter {
  userId?: number;
  projectId?: number;
  apiKeyId?: number;
  billingAccountId?: number;
  modelId?: string;
  status?: BillingHoldStatus;
  from?: string;
  to?: string;
  expiresBefore?: string;
}

export interface AdminPaymentOrdersFilter {
  userId?: number;
  projectId?: number;
  billingAccountId?: number;
  providerType?: PaymentProviderType;
  status?: PaymentOrderStatus;
  orderNo?: string;
  externalTradeNo?: string;
  from?: string;
  to?: string;
}

export interface AdminPaymentEventsFilter {
  paymentOrderId?: number;
  providerInstanceId?: number;
  providerType?: PaymentProviderType;
  status?: PaymentEventStatus;
  eventType?: string;
  eventKey?: string;
  from?: string;
  to?: string;
}

export interface AdminRedeemCodesFilter {
  userId?: number;
  createdById?: number;
  status?: RedeemCodeStatus;
  type?: RedeemCodeType;
  code?: string;
  batchId?: string;
  from?: string;
  to?: string;
  expiresBefore?: string;
}

export interface AdminUserSubscriptionsFilter {
  userId?: number;
  planId?: number;
  status?: UserSubscriptionStatus;
  from?: string;
  to?: string;
  expiresBefore?: string;
}

export interface AdminPromoCodesFilter {
  status?: PromoCodeStatus;
  scope?: PromoCodeScope;
  code?: string;
  createdById?: number;
  from?: string;
  to?: string;
  expiresBefore?: string;
}

export interface AdminPromoUsagesFilter {
  promoCodeId?: number;
  userId?: number;
  billingAccountId?: number;
  paymentOrderId?: number;
  userSubscriptionId?: number;
  scope?: PromoUsageScope;
  status?: PromoUsageStatus;
  code?: string;
  from?: string;
  to?: string;
}

export interface AdminAffiliateInvitationsFilter {
  inviterUserId?: number;
  inviteeUserId?: number;
  status?: AffiliateInvitationStatus;
  inviteCode?: string;
  from?: string;
  to?: string;
}

export interface AdminAffiliateRebatesFilter {
  inviterUserId?: number;
  inviteeUserId?: number;
  sourceType?: AffiliateRebateSourceType;
  status?: AffiliateRebateStatus;
  from?: string;
  to?: string;
  transferableBefore?: string;
}

export interface AdminBillingNotificationsFilter {
  userId?: number;
  audience?: BillingNotificationAudience;
  category?: BillingNotificationCategory;
  severity?: BillingNotificationSeverity;
  status?: BillingNotificationStatus;
  eventKey?: string;
  billingAccountId?: number;
  paymentOrderId?: number;
  userSubscriptionId?: number;
  usageBillingRecordId?: number;
  from?: string;
  to?: string;
}

export interface AdminBillingAuditLogsFilter {
  action?: string;
  actorUserId?: number;
  targetType?: string;
  targetId?: string;
  targetUserId?: number;
  from?: string;
  to?: string;
}

export interface AdminBillingReportFilter {
  from?: string;
  to?: string;
  currency?: string;
  limit?: number;
}

export interface BillingReportSummary {
  rechargeAmountMicros: number;
  consumptionAmountMicros: number;
  netMovementMicros: number;
  refundAmountMicros: number;
  failedPaymentCount: number;
  failedPaymentEventCount: number;
  pendingHoldAmountMicros: number;
  pendingHoldCount: number;
}

export interface BillingDailyReportRow {
  date: string;
  rechargeAmountMicros: number;
  consumptionAmountMicros: number;
  netMovementMicros: number;
  failedPaymentCount: number;
  failedPaymentEventCount: number;
}

export interface BillingTopModelRow {
  modelId: string;
  chargeAmountMicros: number;
  requestCount: number;
}

export interface BillingTopProjectRow {
  projectId: number;
  projectName: string;
  chargeAmountMicros: number;
  requestCount: number;
}

export interface BillingTopAPIKeyRow {
  apiKeyId: number;
  apiKeyName: string;
  chargeAmountMicros: number;
  requestCount: number;
}

export interface BillingTopChannelRow {
  channelId: number;
  channelName: string;
  chargeAmountMicros: number;
  requestCount: number;
}

export interface BillingTopUserReportRow {
  userId: number;
  email: string;
  rechargeAmountMicros: number;
  consumptionAmountMicros: number;
  netAmountMicros: number;
}

export interface BillingCommercialReport {
  from?: string | null;
  to?: string | null;
  currency: string;
  summary: BillingReportSummary;
  daily: BillingDailyReportRow[];
  topModels: BillingTopModelRow[];
  topProjects: BillingTopProjectRow[];
  topApiKeys: BillingTopAPIKeyRow[];
  topChannels: BillingTopChannelRow[];
  topUsers: BillingTopUserReportRow[];
}

export type BillingCSVExportDataset = 'ledger_transactions' | 'usage_billing_records' | 'payment_orders' | 'payment_events';

export interface ExportAdminBillingCSVInput extends AdminBillingReportFilter {
  dataset: BillingCSVExportDataset;
}

export interface BillingCSVExportPayload {
  fileName: string;
  content: string;
  contentType: string;
}

export interface CommercialMaintenanceRunResult {
  orderExpiryProcessed: number;
  holdExpiryProcessed: number;
  subscriptionExpiryProcessed: number;
  subscriptionResetProcessed: number;
  affiliateRebateThawProcessed: number;
  failedBillingRetryProcessed: number;
  usageAggregateRecordsProcessed: number;
  usageAggregateHourlyRows: number;
  usageAggregateDailyRows: number;
}

type Connection<T> = {
  edges?: Array<{ node?: T | null } | null> | null;
};

function nodes<T>(connection?: Connection<T> | null): T[] {
  return connection?.edges?.flatMap((edge) => (edge?.node ? [edge.node] : [])) ?? [];
}

const ADMIN_BILLING_OVERVIEW_QUERY = `
  query AdminBillingOverview($first: Int!) {
    billingAccounts(first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          ownerType
          ownerID
          currency
          balanceMicros
          heldBalanceMicros
          creditLimitMicros
          status
        }
      }
    }
    paymentProviderInstances(first: 20, orderBy: { field: CREATED_AT, direction: DESC }, where: { providerType: epay }) {
      edges {
        node {
          id
          createdAt
          updatedAt
          name
          providerType
          status
          currency
        }
      }
    }
    billingPriceRules(first: 20, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          updatedAt
          scopeType
          scopeID
          modelPattern
          price {
            items {
              itemCode
              pricing {
                mode
                flatFee
                usagePerUnit
              }
            }
          }
          currency
          priority
          enabled
          referenceID
        }
      }
    }
  }
`;

const USER_BILLING_DETAIL_QUERY = `
  query AdminUserBillingDetail($userId: ID!, $first: Int!) {
    userBillingAccount(userId: $userId) {
      id
      ownerType
      ownerID
      currency
      balanceMicros
      creditLimitMicros
      heldBalanceMicros
      status
    }
    userLedgerTransactions(userId: $userId, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          billingAccountID
          direction
          amountMicros
          currency
          type
          status
          referenceType
          referenceID
          memo
          createdByType
          createdByID
        }
      }
    }
    userPaymentOrders(userId: $userId, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          orderNo
          projectID
          billingAccountID
          providerType
          amountMicros
          currency
          status
          externalTradeNo
          paidAt
          expiresAt
          canceledAt
          cancelReason
          makeupReason
          failureReason
          refundedAt
          refundReason
          refundAmountMicros
        }
      }
    }
    userUsageBillingRecords(userId: $userId, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          billingAccountID
          projectID
          userID
          apiKeyID
          modelID
          costAmountMicros
          chargeAmountMicros
          currency
          status
          error
        }
      }
    }
  }
`;

const ADMIN_LEDGER_TRANSACTIONS_QUERY = `
  query AdminLedgerTransactions($filter: AdminLedgerTransactionsFilter, $first: Int!) {
    adminLedgerTransactions(filter: $filter, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          billingAccountID
          direction
          amountMicros
          currency
          type
          status
          referenceType
          referenceID
          memo
          createdByType
          createdByID
        }
      }
    }
  }
`;

const ADMIN_USAGE_BILLING_RECORDS_QUERY = `
  query AdminUsageBillingRecords($filter: AdminUsageBillingRecordsFilter, $first: Int!) {
    adminUsageBillingRecords(filter: $filter, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          billingAccountID
          projectID
          userID
          apiKeyID
          modelID
          costAmountMicros
          chargeAmountMicros
          currency
          status
          error
        }
      }
    }
  }
`;

const ADMIN_BILLING_HOLDS_QUERY = `
  query AdminBillingHolds($filter: AdminBillingHoldsFilter, $first: Int!) {
    adminBillingHolds(filter: $filter, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          billingAccountID
          requestID
          usageLogID
          projectID
          userID
          apiKeyID
          modelID
          amountMicros
          capturedAmountMicros
          currency
          status
          idempotencyKey
          referenceType
          referenceID
          releaseReason
          releasedByType
          releasedByID
          expiresAt
          capturedAt
          releasedAt
        }
      }
    }
  }
`;

const ADMIN_PAYMENT_ORDERS_QUERY = `
  query AdminPaymentOrders($filter: AdminPaymentOrdersFilter, $first: Int!) {
    adminPaymentOrders(filter: $filter, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          orderNo
          projectID
          billingAccountID
          providerType
          amountMicros
          currency
          status
          externalTradeNo
          paidAt
          expiresAt
          canceledAt
          cancelReason
          makeupReason
          failureReason
          refundedAt
          refundReason
          refundAmountMicros
        }
      }
    }
  }
`;

const ADMIN_PAYMENT_EVENTS_QUERY = `
  query AdminPaymentEvents($filter: AdminPaymentEventsFilter, $first: Int!) {
    adminPaymentEvents(filter: $filter, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          eventKey
          paymentOrderID
          providerInstanceID
          providerType
          eventType
          payload
          status
          error
        }
      }
    }
  }
`;

const ADMIN_REDEEM_CODES_QUERY = `
  query AdminRedeemCodes($filter: AdminRedeemCodesFilter, $first: Int!) {
    adminRedeemCodes(filter: $filter, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          updatedAt
          code
          type
          status
          amountMicros
          currency
          createdByID
          usedByID
          usedAt
          expiresAt
          notes
          ledgerTransactionID
          batchID
        }
      }
    }
  }
`;

const ADMIN_PROMO_CODES_QUERY = `
  query AdminPromoCodes($filter: AdminPromoCodesFilter, $first: Int!) {
    adminPromoCodes(filter: $filter, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          updatedAt
          code
          description
          discountType
          discountAmountMicros
          discountPercentBps
          scope
          status
          currency
          maxUses
          usedCount
          perUserLimit
          startsAt
          expiresAt
          createdByID
          notes
        }
      }
    }
  }
`;

const ADMIN_PROMO_USAGES_QUERY = `
  query AdminPromoUsages($filter: AdminPromoUsagesFilter, $first: Int!) {
    adminPromoUsages(filter: $filter, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          promoCodeID
          code
          userID
          billingAccountID
          paymentOrderID
          userSubscriptionID
          ledgerTransactionID
          scope
          status
          originalAmountMicros
          discountAmountMicros
          payableAmountMicros
          currency
          idempotencyKey
        }
      }
    }
  }
`;

const ADMIN_AFFILIATE_SETTING_QUERY = `
  query AdminAffiliateSetting {
    adminAffiliateSetting {
      id
      createdAt
      updatedAt
      enabled
      defaultRebateRateBps
      freezeDays
      minTransferMicros
      currency
    }
  }
`;

const ADMIN_AFFILIATE_PROFILES_QUERY = `
  query AdminAffiliateProfiles($first: Int!) {
    adminAffiliateProfiles(first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          updatedAt
          userID
          inviteCode
          status
          rebateRateOverrideBps
          notes
        }
      }
    }
  }
`;

const ADMIN_AFFILIATE_INVITATIONS_QUERY = `
  query AdminAffiliateInvitations($filter: AdminAffiliateInvitationsFilter, $first: Int!) {
    adminAffiliateInvitations(filter: $filter, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          updatedAt
          inviterUserID
          inviteeUserID
          inviteCode
          status
          notes
        }
      }
    }
  }
`;

const ADMIN_AFFILIATE_REBATES_QUERY = `
  query AdminAffiliateRebates($filter: AdminAffiliateRebatesFilter, $first: Int!) {
    adminAffiliateRebates(filter: $filter, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          updatedAt
          invitationID
          inviterUserID
          inviteeUserID
          sourceType
          sourceID
          paymentOrderID
          userSubscriptionID
          ledgerTransactionID
          baseAmountMicros
          amountMicros
          rateBps
          currency
          status
          freezeUntil
          transferredAt
        }
      }
    }
  }
`;

const ADMIN_BILLING_NOTIFICATION_SETTING_QUERY = `
  query AdminBillingNotificationSetting {
    adminBillingNotificationSetting {
      id
      createdAt
      updatedAt
      enabled
      userNotificationsEnabled
      operatorAlertsEnabled
      lowBalanceThresholdMicros
      largeConsumptionThresholdMicros
      subscriptionExpiryWarningDays
      currency
    }
  }
`;

const ADMIN_BILLING_NOTIFICATIONS_QUERY = `
  query AdminBillingNotifications($filter: AdminBillingNotificationsFilter, $first: Int!) {
    adminBillingNotifications(filter: $filter, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          updatedAt
          userID
          audience
          category
          severity
          status
          eventKey
          title
          message
          currency
          amountMicros
          billingAccountID
          paymentOrderID
          userSubscriptionID
          usageBillingRecordID
          readAt
        }
      }
    }
  }
`;

const ADMIN_COMMERCIAL_SETTING_QUERY = `
  query AdminCommercialSetting {
    adminCommercialSetting {
      id
      createdAt
      updatedAt
      key
      mode
      requireAdminActionReason
      paymentProviderSecretsEncrypted
      workersEnabled
      orderExpiryWorkerEnabled
      holdExpiryWorkerEnabled
      subscriptionExpiryWorkerEnabled
      subscriptionResetWorkerEnabled
      affiliateRebateThawWorkerEnabled
      failedBillingRetryWorkerEnabled
      workerBatchSize
      currency
    }
  }
`;

const ADMIN_BILLING_AUDIT_LOGS_QUERY = `
  query AdminBillingAuditLogs($filter: AdminBillingAuditLogsFilter, $first: Int!) {
    adminBillingAuditLogs(filter: $filter, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          updatedAt
          action
          actorType
          actorUserID
          targetType
          targetID
          targetUserID
          reason
          metadata
        }
      }
    }
  }
`;

const ADMIN_SUBSCRIPTION_PLANS_QUERY = `
  query AdminSubscriptionPlans($first: Int!) {
    subscriptionPlans(first: $first, orderBy: { field: CREATED_AT, direction: ASC }) {
      edges {
        node {
          id
          createdAt
          updatedAt
          name
          description
          period
          periodDays
          priceMicros
          currency
          includedAmountMicros
          supportedModelIds
          supportedProjectIds
          supportedGroupIds
          allowWalletFallback
          status
          sortOrder
        }
      }
    }
  }
`;

const ADMIN_USER_SUBSCRIPTIONS_QUERY = `
  query AdminUserSubscriptions($filter: AdminUserSubscriptionsFilter, $first: Int!) {
    adminUserSubscriptions(filter: $filter, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          updatedAt
          userID
          planID
          status
          startsAt
          expiresAt
          currentPeriodStart
          currentPeriodEnd
          resetAt
          periodDays
          includedAmountMicros
          usedAmountMicros
          currency
          supportedModelIds
          supportedProjectIds
          supportedGroupIds
          allowWalletFallback
          assignedByID
          purchaseLedgerTransactionID
          notes
          revokeReason
          plan {
            id
            name
            period
            priceMicros
            currency
          }
        }
      }
    }
  }
`;

const ADMIN_BILLING_REPORT_QUERY = `
  query AdminBillingReport($filter: AdminBillingReportFilter) {
    adminBillingReport(filter: $filter) {
      from
      to
      currency
      summary {
        rechargeAmountMicros
        consumptionAmountMicros
        netMovementMicros
        refundAmountMicros
        failedPaymentCount
        failedPaymentEventCount
        pendingHoldAmountMicros
        pendingHoldCount
      }
      daily {
        date
        rechargeAmountMicros
        consumptionAmountMicros
        netMovementMicros
        failedPaymentCount
        failedPaymentEventCount
      }
      topModels {
        modelId
        chargeAmountMicros
        requestCount
      }
      topProjects {
        projectId
        projectName
        chargeAmountMicros
        requestCount
      }
      topApiKeys {
        apiKeyId
        apiKeyName
        chargeAmountMicros
        requestCount
      }
      topChannels {
        channelId
        channelName
        chargeAmountMicros
        requestCount
      }
      topUsers {
        userId
        email
        rechargeAmountMicros
        consumptionAmountMicros
        netAmountMicros
      }
    }
  }
`;

const EXPORT_ADMIN_BILLING_CSV_QUERY = `
  query ExportAdminBillingCSV($input: ExportAdminBillingCSVInput!) {
    exportAdminBillingCSV(input: $input) {
      fileName
      content
      contentType
    }
  }
`;

const ADJUST_USER_BALANCE_MUTATION = `
  mutation AdjustUserBalance($input: AdjustUserBalanceInput!) {
    adjustUserBalance(input: $input) {
      id
      createdAt
      direction
      amountMicros
      currency
      type
      status
      memo
    }
  }
`;

const UPDATE_USER_BILLING_ACCOUNT_MUTATION = `
  mutation UpdateUserBillingAccount($input: UpdateUserBillingAccountInput!) {
    updateUserBillingAccount(input: $input) {
      id
      ownerType
      ownerID
      currency
      balanceMicros
      heldBalanceMicros
      creditLimitMicros
      status
    }
  }
`;

const CANCEL_PAYMENT_ORDER_MUTATION = `
  mutation CancelPaymentOrder($input: CancelPaymentOrderInput!) {
    cancelPaymentOrder(input: $input) {
      id
      orderNo
      status
      canceledAt
      cancelReason
      failureReason
    }
  }
`;

const MAKE_UP_PAYMENT_ORDER_MUTATION = `
  mutation MakeUpPaymentOrder($input: MakeUpPaymentOrderInput!) {
    makeUpPaymentOrder(input: $input) {
      id
      orderNo
      status
      paidAt
      ledgerTransactionID
      makeupReason
      failureReason
    }
  }
`;

const RELEASE_BILLING_HOLD_MUTATION = `
  mutation ReleaseBillingHold($id: ID!, $reason: String!) {
    releaseBillingHold(id: $id, reason: $reason) {
      id
      status
      releaseReason
      releasedAt
      releasedByType
      releasedByID
    }
  }
`;

const SAVE_BILLING_PRICE_RULE_MUTATION = `
  mutation SaveBillingPriceRule($input: SaveBillingPriceRuleForm!) {
    saveBillingPriceRule(input: $input) {
      id
      scopeType
      scopeID
      modelPattern
      currency
      priority
      enabled
      referenceID
      price {
        items {
          itemCode
          pricing {
            mode
            usagePerUnit
          }
        }
      }
    }
  }
`;

const UPSERT_EPAY_PROVIDER_MUTATION = `
  mutation UpsertEPayPaymentProvider($input: UpsertEPayPaymentProviderInput!) {
    upsertEPayPaymentProvider(input: $input) {
      id
      createdAt
      updatedAt
      name
      providerType
      status
      currency
    }
  }
`;

const CREATE_REDEEM_CODES_MUTATION = `
  mutation CreateRedeemCodes($input: CreateRedeemCodesInput!) {
    createRedeemCodes(input: $input) {
      id
      createdAt
      updatedAt
      code
      type
      status
      amountMicros
      currency
      createdByID
      usedByID
      usedAt
      expiresAt
      notes
      ledgerTransactionID
      batchID
    }
  }
`;

const ADMIN_CREATE_AND_REDEEM_CODE_MUTATION = `
  mutation AdminCreateAndRedeemCode($input: AdminCreateAndRedeemCodeInput!) {
    adminCreateAndRedeemCode(input: $input) {
      id
      createdAt
      updatedAt
      code
      type
      status
      amountMicros
      currency
      createdByID
      usedByID
      usedAt
      expiresAt
      notes
      ledgerTransactionID
      batchID
    }
  }
`;

const UPDATE_REDEEM_CODE_STATUS_MUTATION = `
  mutation UpdateRedeemCodeStatus($input: UpdateRedeemCodeStatusInput!) {
    updateRedeemCodeStatus(input: $input) {
      id
      createdAt
      updatedAt
      code
      type
      status
      amountMicros
      currency
      createdByID
      usedByID
      usedAt
      expiresAt
      notes
      ledgerTransactionID
      batchID
    }
  }
`;

const DELETE_REDEEM_CODE_MUTATION = `
  mutation DeleteRedeemCode($id: ID!) {
    deleteRedeemCode(id: $id)
  }
`;

const SAVE_PROMO_CODE_MUTATION = `
  mutation SavePromoCode($input: SavePromoCodeInput!) {
    savePromoCode(input: $input) {
      id
      createdAt
      updatedAt
      code
      description
      discountType
      discountAmountMicros
      discountPercentBps
      scope
      status
      currency
      maxUses
      usedCount
      perUserLimit
      startsAt
      expiresAt
      createdByID
      notes
    }
  }
`;

const UPDATE_PROMO_CODE_STATUS_MUTATION = `
  mutation UpdatePromoCodeStatus($input: UpdatePromoCodeStatusInput!) {
    updatePromoCodeStatus(input: $input) {
      id
      status
      notes
    }
  }
`;

const DELETE_PROMO_CODE_MUTATION = `
  mutation DeletePromoCode($id: ID!) {
    deletePromoCode(id: $id)
  }
`;

const SAVE_AFFILIATE_SETTING_MUTATION = `
  mutation SaveAffiliateSetting($input: SaveAffiliateSettingInput!) {
    saveAffiliateSetting(input: $input) {
      id
      createdAt
      updatedAt
      enabled
      defaultRebateRateBps
      freezeDays
      minTransferMicros
      currency
    }
  }
`;

const SAVE_AFFILIATE_PROFILE_MUTATION = `
  mutation SaveAffiliateProfile($input: SaveAffiliateProfileInput!) {
    saveAffiliateProfile(input: $input) {
      id
      createdAt
      updatedAt
      userID
      inviteCode
      status
      rebateRateOverrideBps
      notes
    }
  }
`;

const SAVE_BILLING_NOTIFICATION_SETTING_MUTATION = `
  mutation SaveBillingNotificationSetting($input: SaveBillingNotificationSettingInput!) {
    saveBillingNotificationSetting(input: $input) {
      id
      createdAt
      updatedAt
      enabled
      userNotificationsEnabled
      operatorAlertsEnabled
      lowBalanceThresholdMicros
      largeConsumptionThresholdMicros
      subscriptionExpiryWarningDays
      currency
    }
  }
`;

const SAVE_COMMERCIAL_SETTING_MUTATION = `
  mutation SaveCommercialSetting($input: SaveCommercialSettingInput!) {
    saveCommercialSetting(input: $input) {
      id
      createdAt
      updatedAt
      key
      mode
      requireAdminActionReason
      paymentProviderSecretsEncrypted
      workersEnabled
      orderExpiryWorkerEnabled
      holdExpiryWorkerEnabled
      subscriptionExpiryWorkerEnabled
      subscriptionResetWorkerEnabled
      affiliateRebateThawWorkerEnabled
      failedBillingRetryWorkerEnabled
      workerBatchSize
      currency
    }
  }
`;

const RUN_COMMERCIAL_MAINTENANCE_MUTATION = `
  mutation RunCommercialMaintenance($input: RunCommercialMaintenanceInput!) {
    runCommercialMaintenance(input: $input) {
      orderExpiryProcessed
      holdExpiryProcessed
      subscriptionExpiryProcessed
      subscriptionResetProcessed
      affiliateRebateThawProcessed
      failedBillingRetryProcessed
      usageAggregateRecordsProcessed
      usageAggregateHourlyRows
      usageAggregateDailyRows
    }
  }
`;

const SAVE_SUBSCRIPTION_PLAN_MUTATION = `
  mutation SaveSubscriptionPlan($input: SaveSubscriptionPlanInput!) {
    saveSubscriptionPlan(input: $input) {
      id
      createdAt
      updatedAt
      name
      description
      period
      periodDays
      priceMicros
      currency
      includedAmountMicros
      supportedModelIds
      supportedProjectIds
      supportedGroupIds
      allowWalletFallback
      status
      sortOrder
    }
  }
`;

const DELETE_SUBSCRIPTION_PLAN_MUTATION = `
  mutation DeleteSubscriptionPlan($id: ID!) {
    deleteSubscriptionPlan(id: $id)
  }
`;

const ADMIN_ASSIGN_SUBSCRIPTION_MUTATION = `
  mutation AdminAssignSubscription($input: AdminAssignSubscriptionInput!) {
    adminAssignSubscription(input: $input) {
      id
      status
      userID
      planID
      startsAt
      expiresAt
      notes
    }
  }
`;

const EXTEND_USER_SUBSCRIPTION_MUTATION = `
  mutation ExtendUserSubscription($input: ExtendUserSubscriptionInput!) {
    extendUserSubscription(input: $input) {
      id
      status
      expiresAt
      notes
    }
  }
`;

const REVOKE_USER_SUBSCRIPTION_MUTATION = `
  mutation RevokeUserSubscription($id: ID!, $reason: String) {
    revokeUserSubscription(id: $id, reason: $reason) {
      id
      status
      revokeReason
    }
  }
`;

const RESTORE_USER_SUBSCRIPTION_MUTATION = `
  mutation RestoreUserSubscription($id: ID!) {
    restoreUserSubscription(id: $id) {
      id
      status
      revokeReason
    }
  }
`;

const RESET_USER_SUBSCRIPTION_USAGE_MUTATION = `
  mutation ResetUserSubscriptionUsage($id: ID!) {
    resetUserSubscriptionUsage(id: $id) {
      id
      usedAmountMicros
      currentPeriodStart
      currentPeriodEnd
      resetAt
    }
  }
`;

export function useAdminBillingOverview(first = 20) {
  return useQuery({
    queryKey: ['admin-billing', 'overview', first],
    queryFn: async () => {
      const data = await graphqlRequest<{
        billingAccounts: Connection<BillingAccount>;
        paymentProviderInstances: Connection<PaymentProviderInstance>;
        billingPriceRules: Connection<BillingPriceRule>;
      }>(ADMIN_BILLING_OVERVIEW_QUERY, { first });

      return {
        accounts: nodes(data.billingAccounts),
        providers: nodes(data.paymentProviderInstances),
        priceRules: nodes(data.billingPriceRules),
      };
    },
  });
}

export function useAdminUserBillingDetail(userId?: string, first = 10) {
  return useQuery({
    queryKey: ['admin-billing', 'user-detail', userId, first],
    enabled: !!userId,
    queryFn: async () => {
      const data = await graphqlRequest<{
        userBillingAccount: BillingAccount;
        userLedgerTransactions: Connection<LedgerTransaction>;
        userPaymentOrders: Connection<PaymentOrder>;
        userUsageBillingRecords: Connection<UsageBillingRecord>;
      }>(USER_BILLING_DETAIL_QUERY, { userId, first });

      return {
        account: data.userBillingAccount,
        ledgerTransactions: nodes(data.userLedgerTransactions),
        paymentOrders: nodes(data.userPaymentOrders),
        usageBillingRecords: nodes(data.userUsageBillingRecords),
      };
    },
  });
}

export function useAdminLedgerTransactions(filter: AdminLedgerTransactionsFilter = {}, first = 20) {
  return useQuery({
    queryKey: ['admin-billing', 'ledger-transactions', filter, first],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminLedgerTransactions: Connection<LedgerTransaction> }>(ADMIN_LEDGER_TRANSACTIONS_QUERY, {
        filter,
        first,
      });
      return nodes(data.adminLedgerTransactions);
    },
  });
}

export function useAdminUsageBillingRecords(filter: AdminUsageBillingRecordsFilter = {}, first = 20) {
  return useQuery({
    queryKey: ['admin-billing', 'usage-billing-records', filter, first],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminUsageBillingRecords: Connection<UsageBillingRecord> }>(ADMIN_USAGE_BILLING_RECORDS_QUERY, {
        filter,
        first,
      });
      return nodes(data.adminUsageBillingRecords);
    },
  });
}

export function useAdminBillingHolds(filter: AdminBillingHoldsFilter = {}, first = 20) {
  return useQuery({
    queryKey: ['admin-billing', 'billing-holds', filter, first],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminBillingHolds: Connection<BillingHold> }>(ADMIN_BILLING_HOLDS_QUERY, { filter, first });
      return nodes(data.adminBillingHolds);
    },
  });
}

export function useAdminPaymentOrders(filter: AdminPaymentOrdersFilter = {}, first = 20) {
  return useQuery({
    queryKey: ['admin-billing', 'payment-orders', filter, first],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminPaymentOrders: Connection<PaymentOrder> }>(ADMIN_PAYMENT_ORDERS_QUERY, { filter, first });
      return nodes(data.adminPaymentOrders);
    },
  });
}

export function useAdminPaymentEvents(filter: AdminPaymentEventsFilter = {}, first = 20) {
  return useQuery({
    queryKey: ['admin-billing', 'payment-events', filter, first],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminPaymentEvents: Connection<PaymentEvent> }>(ADMIN_PAYMENT_EVENTS_QUERY, { filter, first });
      return nodes(data.adminPaymentEvents);
    },
  });
}

export function useAdminRedeemCodes(filter: AdminRedeemCodesFilter = {}, first = 50) {
  return useQuery({
    queryKey: ['admin-billing', 'redeem-codes', filter, first],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminRedeemCodes: Connection<RedeemCode> }>(ADMIN_REDEEM_CODES_QUERY, { filter, first });
      return nodes(data.adminRedeemCodes);
    },
  });
}

export function useAdminPromoCodes(filter: AdminPromoCodesFilter = {}, first = 50) {
  return useQuery({
    queryKey: ['admin-billing', 'promo-codes', filter, first],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminPromoCodes: Connection<PromoCode> }>(ADMIN_PROMO_CODES_QUERY, { filter, first });
      return nodes(data.adminPromoCodes);
    },
  });
}

export function useAdminPromoUsages(filter: AdminPromoUsagesFilter = {}, first = 50) {
  return useQuery({
    queryKey: ['admin-billing', 'promo-usages', filter, first],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminPromoUsages: Connection<PromoUsage> }>(ADMIN_PROMO_USAGES_QUERY, { filter, first });
      return nodes(data.adminPromoUsages);
    },
  });
}

export function useAdminAffiliateSetting() {
  return useQuery({
    queryKey: ['admin-billing', 'affiliate-setting'],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminAffiliateSetting: AffiliateSetting }>(ADMIN_AFFILIATE_SETTING_QUERY);
      return data.adminAffiliateSetting;
    },
  });
}

export function useAdminAffiliateProfiles(first = 50) {
  return useQuery({
    queryKey: ['admin-billing', 'affiliate-profiles', first],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminAffiliateProfiles: Connection<AffiliateProfile> }>(ADMIN_AFFILIATE_PROFILES_QUERY, { first });
      return nodes(data.adminAffiliateProfiles);
    },
  });
}

export function useAdminAffiliateInvitations(filter: AdminAffiliateInvitationsFilter = {}, first = 50) {
  return useQuery({
    queryKey: ['admin-billing', 'affiliate-invitations', filter, first],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminAffiliateInvitations: Connection<AffiliateInvitation> }>(ADMIN_AFFILIATE_INVITATIONS_QUERY, { filter, first });
      return nodes(data.adminAffiliateInvitations);
    },
  });
}

export function useAdminAffiliateRebates(filter: AdminAffiliateRebatesFilter = {}, first = 50) {
  return useQuery({
    queryKey: ['admin-billing', 'affiliate-rebates', filter, first],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminAffiliateRebates: Connection<AffiliateRebate> }>(ADMIN_AFFILIATE_REBATES_QUERY, { filter, first });
      return nodes(data.adminAffiliateRebates);
    },
  });
}

export function useAdminBillingNotificationSetting() {
  return useQuery({
    queryKey: ['admin-billing', 'notification-setting'],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminBillingNotificationSetting: BillingNotificationSetting }>(ADMIN_BILLING_NOTIFICATION_SETTING_QUERY);
      return data.adminBillingNotificationSetting;
    },
  });
}

export function useAdminBillingNotifications(filter: AdminBillingNotificationsFilter = {}, first = 50) {
  return useQuery({
    queryKey: ['admin-billing', 'notifications', filter, first],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminBillingNotifications: Connection<BillingNotification> }>(ADMIN_BILLING_NOTIFICATIONS_QUERY, { filter, first });
      return nodes(data.adminBillingNotifications);
    },
  });
}

export function useAdminCommercialSetting() {
  return useQuery({
    queryKey: ['admin-billing', 'commercial-setting'],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminCommercialSetting: CommercialSetting }>(ADMIN_COMMERCIAL_SETTING_QUERY);
      return data.adminCommercialSetting;
    },
  });
}

export function useAdminBillingAuditLogs(filter: AdminBillingAuditLogsFilter = {}, first = 50) {
  return useQuery({
    queryKey: ['admin-billing', 'audit-logs', filter, first],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminBillingAuditLogs: Connection<BillingAuditLog> }>(ADMIN_BILLING_AUDIT_LOGS_QUERY, {
        filter,
        first,
      });
      return nodes(data.adminBillingAuditLogs);
    },
  });
}

export function useAdminSubscriptionPlans(first = 100) {
  return useQuery({
    queryKey: ['admin-billing', 'subscription-plans', first],
    queryFn: async () => {
      const data = await graphqlRequest<{ subscriptionPlans: Connection<SubscriptionPlan> }>(ADMIN_SUBSCRIPTION_PLANS_QUERY, { first });
      return nodes(data.subscriptionPlans);
    },
  });
}

export function useAdminUserSubscriptions(filter: AdminUserSubscriptionsFilter = {}, first = 50) {
  return useQuery({
    queryKey: ['admin-billing', 'user-subscriptions', filter, first],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminUserSubscriptions: Connection<UserSubscription> }>(ADMIN_USER_SUBSCRIPTIONS_QUERY, {
        filter,
        first,
      });
      return nodes(data.adminUserSubscriptions);
    },
  });
}

export function useAdminBillingReport(filter: AdminBillingReportFilter = {}) {
  return useQuery({
    queryKey: ['admin-billing', 'commercial-report', filter],
    queryFn: async () => {
      const data = await graphqlRequest<{ adminBillingReport: BillingCommercialReport }>(ADMIN_BILLING_REPORT_QUERY, { filter });
      return data.adminBillingReport;
    },
  });
}

export function useExportAdminBillingCSV() {
  return useMutation({
    mutationFn: async (input: ExportAdminBillingCSVInput) => {
      const data = await graphqlRequest<{ exportAdminBillingCSV: BillingCSVExportPayload }>(EXPORT_ADMIN_BILLING_CSV_QUERY, { input });
      return data.exportAdminBillingCSV;
    },
    onSuccess: (data) => {
      const blob = new Blob([data.content], { type: data.contentType || 'text/csv;charset=utf-8' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = data.fileName;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      toast.success(i18n.t('adminBilling.reports.exportSuccess'));
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : i18n.t('common.errors.unknownError'));
    },
  });
}

export function useAdjustUserBalance() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: {
      userId: string;
      direction: LedgerTransactionDirection;
      amount: string;
      currency?: string;
      idempotencyKey?: string;
      memo?: string;
    }) => {
      const data = await graphqlRequest<{ adjustUserBalance: LedgerTransaction }>(ADJUST_USER_BALANCE_MUTATION, { input });
      return data.adjustUserBalance;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing'] });
    },
  });
}

export function useUpdateUserBillingAccount() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { userId: string; status?: BillingAccountStatus; creditLimit?: string }) => {
      const data = await graphqlRequest<{ updateUserBillingAccount: BillingAccount }>(UPDATE_USER_BILLING_ACCOUNT_MUTATION, { input });
      return data.updateUserBillingAccount;
    },
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing'] });
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'user-detail', variables.userId] });
    },
  });
}

export function useCancelPaymentOrder() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { orderNo: string; reason: string }) => {
      const data = await graphqlRequest<{ cancelPaymentOrder: PaymentOrder }>(CANCEL_PAYMENT_ORDER_MUTATION, { input });
      return data.cancelPaymentOrder;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useMakeUpPaymentOrder() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { orderNo: string; reason: string; paidAt?: string }) => {
      const data = await graphqlRequest<{ makeUpPaymentOrder: PaymentOrder }>(MAKE_UP_PAYMENT_ORDER_MUTATION, { input });
      return data.makeUpPaymentOrder;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useReleaseBillingHold() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { id: string; reason: string }) => {
      const data = await graphqlRequest<{ releaseBillingHold: BillingHold }>(RELEASE_BILLING_HOLD_MUTATION, input);
      return data.releaseBillingHold;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useSaveBillingPriceRule() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: {
      id?: string;
      scopeType: 'global' | 'project';
      scopeId: number;
      modelPattern: string;
      price: ModelPrice;
      currency?: string;
      priority?: number;
      enabled?: boolean;
      referenceId?: string;
    }) => {
      const data = await graphqlRequest<{ saveBillingPriceRule: BillingPriceRule }>(SAVE_BILLING_PRICE_RULE_MUTATION, { input });
      return data.saveBillingPriceRule;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'overview'] });
    },
  });
}

export function useUpsertEPayPaymentProvider() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: {
      name: string;
      status?: 'enabled' | 'disabled';
      currency?: string;
      gatewayUrl: string;
      pid: string;
      key?: string;
      notifyUrl: string;
      returnUrl: string;
      type?: string;
      siteName?: string;
    }) => {
      const data = await graphqlRequest<{ upsertEPayPaymentProvider: PaymentProviderInstance }>(UPSERT_EPAY_PROVIDER_MUTATION, { input });
      return data.upsertEPayPaymentProvider;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'overview'] });
    },
  });
}

export function useCreateRedeemCodes() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: {
      count?: number;
      type?: RedeemCodeType;
      amount: string;
      currency?: string;
      expiresAt?: string;
      notes?: string;
      prefix?: string;
    }) => {
      const data = await graphqlRequest<{ createRedeemCodes: RedeemCode[] }>(CREATE_REDEEM_CODES_MUTATION, { input });
      return data.createRedeemCodes;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'redeem-codes'] });
    },
  });
}

export function useAdminCreateAndRedeemCode() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { userId: string; amount: string; currency?: string; expiresAt?: string; notes?: string }) => {
      const data = await graphqlRequest<{ adminCreateAndRedeemCode: RedeemCode }>(ADMIN_CREATE_AND_REDEEM_CODE_MUTATION, { input });
      return data.adminCreateAndRedeemCode;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useUpdateRedeemCodeStatus() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { codeId: string; status: RedeemCodeStatus; notes?: string }) => {
      const data = await graphqlRequest<{ updateRedeemCodeStatus: RedeemCode }>(UPDATE_REDEEM_CODE_STATUS_MUTATION, { input });
      return data.updateRedeemCodeStatus;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'redeem-codes'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useDeleteRedeemCode() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ deleteRedeemCode: boolean }>(DELETE_REDEEM_CODE_MUTATION, { id });
      return data.deleteRedeemCode;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'redeem-codes'] });
    },
  });
}

export function useSavePromoCode() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: {
      id?: string;
      code: string;
      description?: string;
      discountType?: PromoCodeDiscountType;
      discountAmount: string;
      discountPercentBps?: number;
      scope?: PromoCodeScope;
      status?: PromoCodeStatus;
      currency?: string;
      maxUses?: number;
      perUserLimit?: number;
      startsAt?: string;
      expiresAt?: string;
      notes?: string;
    }) => {
      const data = await graphqlRequest<{ savePromoCode: PromoCode }>(SAVE_PROMO_CODE_MUTATION, { input });
      return data.savePromoCode;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'promo-codes'] });
    },
  });
}

export function useUpdatePromoCodeStatus() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { codeId: string; status: PromoCodeStatus; notes?: string }) => {
      const data = await graphqlRequest<{ updatePromoCodeStatus: PromoCode }>(UPDATE_PROMO_CODE_STATUS_MUTATION, { input });
      return data.updatePromoCodeStatus;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'promo-codes'] });
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'promo-usages'] });
    },
  });
}

export function useDeletePromoCode() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ deletePromoCode: boolean }>(DELETE_PROMO_CODE_MUTATION, { id });
      return data.deletePromoCode;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'promo-codes'] });
    },
  });
}

export function useSaveAffiliateSetting() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: {
      enabled: boolean;
      defaultRebateRateBps: number;
      freezeDays: number;
      minTransferAmount?: string;
      currency?: string;
    }) => {
      const data = await graphqlRequest<{ saveAffiliateSetting: AffiliateSetting }>(SAVE_AFFILIATE_SETTING_MUTATION, { input });
      return data.saveAffiliateSetting;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'affiliate-setting'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useSaveAffiliateProfile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: {
      userId: string;
      status?: AffiliateProfileStatus;
      rebateRateOverrideBps?: number | null;
      notes?: string;
    }) => {
      const data = await graphqlRequest<{ saveAffiliateProfile: AffiliateProfile }>(SAVE_AFFILIATE_PROFILE_MUTATION, { input });
      return data.saveAffiliateProfile;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'affiliate-profiles'] });
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'affiliate-rebates'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useSaveBillingNotificationSetting() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: {
      enabled: boolean;
      userNotificationsEnabled: boolean;
      operatorAlertsEnabled: boolean;
      lowBalanceThreshold: string;
      largeConsumptionThreshold: string;
      subscriptionExpiryWarningDays: number;
      currency?: string;
    }) => {
      const data = await graphqlRequest<{ saveBillingNotificationSetting: BillingNotificationSetting }>(
        SAVE_BILLING_NOTIFICATION_SETTING_MUTATION,
        { input }
      );
      return data.saveBillingNotificationSetting;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'notification-setting'] });
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'notifications'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useSaveCommercialSetting() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: {
      mode: CommercialSettingMode;
      requireAdminActionReason: boolean;
      paymentProviderSecretsEncrypted: boolean;
      workersEnabled: boolean;
      orderExpiryWorkerEnabled: boolean;
      holdExpiryWorkerEnabled: boolean;
      subscriptionExpiryWorkerEnabled: boolean;
      subscriptionResetWorkerEnabled: boolean;
      affiliateRebateThawWorkerEnabled: boolean;
      failedBillingRetryWorkerEnabled: boolean;
      workerBatchSize: number;
      currency?: string;
      reason?: string;
    }) => {
      const data = await graphqlRequest<{ saveCommercialSetting: CommercialSetting }>(SAVE_COMMERCIAL_SETTING_MUTATION, { input });
      return data.saveCommercialSetting;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing'] });
    },
  });
}

export function useRunCommercialMaintenance() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { now?: string; limit?: number; reason: string; rebuildUsageAggregates?: boolean }) => {
      const data = await graphqlRequest<{ runCommercialMaintenance: CommercialMaintenanceRunResult }>(RUN_COMMERCIAL_MAINTENANCE_MUTATION, {
        input,
      });
      return data.runCommercialMaintenance;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useSaveSubscriptionPlan() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: {
      id?: string;
      name: string;
      description?: string;
      period?: SubscriptionPlanPeriod;
      periodDays?: number;
      price: string;
      currency?: string;
      includedAmount: string;
      supportedModelIDs?: string[];
      supportedProjectIDs?: number[];
      supportedGroupIDs?: number[];
      allowWalletFallback?: boolean;
      status?: SubscriptionPlanStatus;
      sortOrder?: number;
    }) => {
      const data = await graphqlRequest<{ saveSubscriptionPlan: SubscriptionPlan }>(SAVE_SUBSCRIPTION_PLAN_MUTATION, { input });
      return data.saveSubscriptionPlan;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'subscription-plans'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useDeleteSubscriptionPlan() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ deleteSubscriptionPlan: boolean }>(DELETE_SUBSCRIPTION_PLAN_MUTATION, { id });
      return data.deleteSubscriptionPlan;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'subscription-plans'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useAdminAssignSubscription() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { userId: string; planId: string; startsAt?: string; expiresAt?: string; notes?: string }) => {
      const data = await graphqlRequest<{ adminAssignSubscription: UserSubscription }>(ADMIN_ASSIGN_SUBSCRIPTION_MUTATION, { input });
      return data.adminAssignSubscription;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'user-subscriptions'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useExtendUserSubscription() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { subscriptionId: string; days?: number; expiresAt?: string; notes?: string }) => {
      const data = await graphqlRequest<{ extendUserSubscription: UserSubscription }>(EXTEND_USER_SUBSCRIPTION_MUTATION, { input });
      return data.extendUserSubscription;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'user-subscriptions'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useRevokeUserSubscription() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { id: string; reason?: string }) => {
      const data = await graphqlRequest<{ revokeUserSubscription: UserSubscription }>(REVOKE_USER_SUBSCRIPTION_MUTATION, input);
      return data.revokeUserSubscription;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'user-subscriptions'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useRestoreUserSubscription() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ restoreUserSubscription: UserSubscription }>(RESTORE_USER_SUBSCRIPTION_MUTATION, { id });
      return data.restoreUserSubscription;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'user-subscriptions'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useResetUserSubscriptionUsage() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ resetUserSubscriptionUsage: UserSubscription }>(RESET_USER_SUBSCRIPTION_USAGE_MUTATION, { id });
      return data.resetUserSubscriptionUsage;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-billing', 'user-subscriptions'] });
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}
