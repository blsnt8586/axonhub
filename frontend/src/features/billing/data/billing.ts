import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

export type BillingAccountStatus = 'active' | 'frozen' | 'closed';
export type PaymentOrderStatus = 'pending' | 'paid' | 'failed' | 'canceled' | 'expired' | 'refunded';
export type LedgerTransactionDirection = 'credit' | 'debit';
export type UsageBillingRecordStatus = 'pending' | 'charged' | 'skipped' | 'failed' | 'refunded';
export type RedeemCodeStatus = 'active' | 'used' | 'disabled' | 'expired';
export type RedeemCodeType = 'balance' | 'credit' | 'subscription';
export type SubscriptionPlanPeriod = 'day' | 'month' | 'year' | 'custom';
export type SubscriptionPlanStatus = 'enabled' | 'disabled' | 'archived';
export type UserSubscriptionStatus = 'active' | 'expired' | 'revoked' | 'canceled';
export type AffiliateProfileStatus = 'active' | 'disabled';
export type AffiliateInvitationStatus = 'active' | 'canceled';
export type AffiliateRebateStatus = 'frozen' | 'available' | 'transferred' | 'voided';
export type AffiliateRebateSourceType = 'payment_order' | 'user_subscription';
export type BillingNotificationStatus = 'unread' | 'read' | 'dismissed';
export type BillingNotificationCategory = 'low_balance' | 'payment' | 'subscription' | 'large_consumption' | 'operator_alert';
export type BillingNotificationSeverity = 'info' | 'warning' | 'error';

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

export interface PaymentOrder {
  id: string;
  createdAt: string;
  orderNo: string;
  providerType: string;
  amountMicros: number;
  payableAmountMicros: number;
  discountAmountMicros: number;
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

export interface LedgerTransaction {
  id: string;
  createdAt: string;
  direction: LedgerTransactionDirection;
  amountMicros: number;
  currency: string;
  type: string;
  status: string;
  memo: string;
}

export interface UsageBillingRecord {
  id: string;
  createdAt: string;
  projectID: number;
  modelID: string;
  requestType: string;
  chargeAmountMicros: number;
  currency: string;
  status: UsageBillingRecordStatus;
  error: string;
  userSubscriptionID?: string | null;
}

export interface SubscriptionPlan {
  id: string;
  name: string;
  description: string;
  period: SubscriptionPlanPeriod;
  periodDays: number;
  priceMicros: number;
  currency: string;
  includedAmountMicros: number;
  supportedModelIds: string[];
  supportedProjectIds: number[];
  allowWalletFallback: boolean;
  status: SubscriptionPlanStatus;
}

export interface UserSubscription {
  id: string;
  createdAt: string;
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
  allowWalletFallback: boolean;
  originalPriceMicros: number;
  discountAmountMicros: number;
  payableAmountMicros: number;
  notes: string;
  revokeReason: string;
  plan?: Pick<SubscriptionPlan, 'id' | 'name' | 'period' | 'priceMicros' | 'currency'> | null;
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
  usedAt?: string | null;
  expiresAt?: string | null;
  notes: string;
  ledgerTransactionID?: string | null;
  batchID: string;
}

export interface PaymentCheckout {
  providerType: string;
  orderNo: string;
  method: string;
  url?: string | null;
  amount: string;
  currency: string;
}

export interface PromoQuote {
  code?: string | null;
  originalAmountMicros: number;
  discountAmountMicros: number;
  payableAmountMicros: number;
  currency: string;
}

export interface AffiliateSetting {
  id: string;
  enabled: boolean;
  defaultRebateRateBps: number;
  freezeDays: number;
  minTransferMicros: number;
  currency: string;
}

export interface AffiliateProfile {
  id: string;
  userID: string;
  inviteCode: string;
  status: AffiliateProfileStatus;
  rebateRateOverrideBps?: number | null;
  notes: string;
}

export interface AffiliateInvitation {
  id: string;
  createdAt: string;
  inviterUserID: string;
  inviteeUserID: string;
  inviteCode: string;
  status: AffiliateInvitationStatus;
  notes: string;
}

export interface AffiliateRebate {
  id: string;
  createdAt: string;
  inviterUserID: string;
  inviteeUserID: string;
  sourceType: AffiliateRebateSourceType;
  sourceID: number;
  baseAmountMicros: number;
  amountMicros: number;
  rateBps: number;
  currency: string;
  status: AffiliateRebateStatus;
  freezeUntil: string;
  transferredAt?: string | null;
  ledgerTransactionID?: string | null;
}

export interface AffiliateSummary {
  profile: AffiliateProfile;
  invitation?: AffiliateInvitation | null;
  inviteeCount: number;
  frozenMicros: number;
  availableMicros: number;
  transferredMicros: number;
  currency: string;
  setting: AffiliateSetting;
}

export interface AffiliateTransferResult {
  transferredCount: number;
  transferredMicros: number;
  currency: string;
  ledgerTransactionIDs: string[];
}

export interface BillingNotificationPreference {
  id: string;
  enabled: boolean;
  lowBalanceEnabled: boolean;
  paymentEnabled: boolean;
  subscriptionEnabled: boolean;
  largeConsumptionEnabled: boolean;
}

export interface BillingNotification {
  id: string;
  createdAt: string;
  category: BillingNotificationCategory;
  severity: BillingNotificationSeverity;
  status: BillingNotificationStatus;
  title: string;
  message: string;
  currency: string;
  amountMicros?: number | null;
  paymentOrderID?: string | null;
  userSubscriptionID?: string | null;
  usageBillingRecordID?: string | null;
  readAt?: string | null;
}

export interface BillingOverview {
  account: BillingAccount;
  paymentOrders: PaymentOrder[];
  ledgerTransactions: LedgerTransaction[];
  usageBillingRecords: UsageBillingRecord[];
  redeemCodes: RedeemCode[];
  availableSubscriptionPlans: SubscriptionPlan[];
  userSubscriptions: UserSubscription[];
  affiliateSummary: AffiliateSummary;
  affiliateInvitations: AffiliateInvitation[];
  affiliateRebates: AffiliateRebate[];
  notificationPreference: BillingNotificationPreference;
  notifications: BillingNotification[];
}

const BILLING_OVERVIEW_QUERY = `
  query MyBillingOverview($first: Int!) {
    myBillingAccount {
      id
      ownerType
      ownerID
      currency
      balanceMicros
      heldBalanceMicros
      creditLimitMicros
      status
    }
    myPaymentOrders(first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          orderNo
          providerType
          amountMicros
          payableAmountMicros
          discountAmountMicros
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
    myLedgerTransactions(first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
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
    }
    myUsageBillingRecords(first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          projectID
          modelID
          requestType
          chargeAmountMicros
          currency
          status
          error
          userSubscriptionID
        }
      }
    }
    myRedeemCodes(first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
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
          usedAt
          expiresAt
          notes
          ledgerTransactionID
          batchID
        }
      }
    }
    availableSubscriptionPlans(first: 20, orderBy: { field: CREATED_AT, direction: ASC }) {
      edges {
        node {
          id
          name
          description
          period
          periodDays
          priceMicros
          currency
          includedAmountMicros
          supportedModelIds
          supportedProjectIds
          allowWalletFallback
          status
        }
      }
    }
    myUserSubscriptions(first: 20, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
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
          allowWalletFallback
          originalPriceMicros
          discountAmountMicros
          payableAmountMicros
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
    myAffiliateSummary {
      inviteeCount
      frozenMicros
      availableMicros
      transferredMicros
      currency
      profile {
        id
        userID
        inviteCode
        status
        rebateRateOverrideBps
        notes
      }
      invitation {
        id
        createdAt
        inviterUserID
        inviteeUserID
        inviteCode
        status
        notes
      }
      setting {
        id
        enabled
        defaultRebateRateBps
        freezeDays
        minTransferMicros
        currency
      }
    }
    myAffiliateInvitations(first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          inviterUserID
          inviteeUserID
          inviteCode
          status
          notes
        }
      }
    }
    myAffiliateRebates(first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          inviterUserID
          inviteeUserID
          sourceType
          sourceID
          baseAmountMicros
          amountMicros
          rateBps
          currency
          status
          freezeUntil
          transferredAt
          ledgerTransactionID
        }
      }
    }
    myBillingNotificationPreference {
      id
      enabled
      lowBalanceEnabled
      paymentEnabled
      subscriptionEnabled
      largeConsumptionEnabled
    }
    myBillingNotifications(first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
      edges {
        node {
          id
          createdAt
          category
          severity
          status
          title
          message
          currency
          amountMicros
          paymentOrderID
          userSubscriptionID
          usageBillingRecordID
          readAt
        }
      }
    }
  }
`;

const CREATE_MY_EPAY_RECHARGE_CHECKOUT = `
  mutation CreateMyEPayRechargeCheckout($input: CreateMyEPayRechargeCheckoutInput!) {
    createMyEPayRechargeCheckout(input: $input) {
      providerType
      orderNo
      method
      url
      amount
      currency
    }
  }
`;

const QUOTE_RECHARGE_PROMO_QUERY = `
  query QuoteRechargePromo($input: QuoteRechargePromoInput!) {
    quoteRechargePromo(input: $input) {
      code
      originalAmountMicros
      discountAmountMicros
      payableAmountMicros
      currency
    }
  }
`;

const QUOTE_SUBSCRIPTION_PROMO_QUERY = `
  query QuoteSubscriptionPromo($input: QuoteSubscriptionPromoInput!) {
    quoteSubscriptionPromo(input: $input) {
      code
      originalAmountMicros
      discountAmountMicros
      payableAmountMicros
      currency
    }
  }
`;

const REDEEM_CODE_MUTATION = `
  mutation RedeemCode($input: RedeemCodeInput!) {
    redeemCode(input: $input) {
      id
      createdAt
      updatedAt
      code
      type
      status
      amountMicros
      currency
      usedAt
      expiresAt
      notes
      ledgerTransactionID
      batchID
    }
  }
`;

const PURCHASE_SUBSCRIPTION_PLAN_MUTATION = `
  mutation PurchaseSubscriptionPlan($input: PurchaseSubscriptionPlanInput!) {
    purchaseSubscriptionPlan(input: $input) {
      id
      status
      startsAt
      expiresAt
      includedAmountMicros
      usedAmountMicros
      currency
      plan {
        id
        name
        period
        priceMicros
        currency
      }
    }
  }
`;

const BIND_AFFILIATE_INVITE_MUTATION = `
  mutation BindAffiliateInvite($input: BindAffiliateInviteInput!) {
    bindAffiliateInvite(input: $input) {
      id
      createdAt
      inviterUserID
      inviteeUserID
      inviteCode
      status
      notes
    }
  }
`;

const TRANSFER_AFFILIATE_REBATES_MUTATION = `
  mutation TransferAffiliateRebates {
    transferAffiliateRebates {
      transferredCount
      transferredMicros
      currency
      ledgerTransactionIDs
    }
  }
`;

const SAVE_MY_BILLING_NOTIFICATION_PREFERENCE_MUTATION = `
  mutation SaveMyBillingNotificationPreference($input: SaveBillingNotificationPreferenceInput!) {
    saveMyBillingNotificationPreference(input: $input) {
      id
      enabled
      lowBalanceEnabled
      paymentEnabled
      subscriptionEnabled
      largeConsumptionEnabled
    }
  }
`;

const MARK_BILLING_NOTIFICATION_READ_MUTATION = `
  mutation MarkBillingNotificationRead($id: ID!) {
    markBillingNotificationRead(id: $id) {
      id
      status
      readAt
    }
  }
`;

const MARK_ALL_BILLING_NOTIFICATIONS_READ_MUTATION = `
  mutation MarkAllBillingNotificationsRead {
    markAllBillingNotificationsRead
  }
`;

type Connection<T> = {
  edges?: Array<{ node?: T | null } | null> | null;
};

function nodes<T>(connection?: Connection<T> | null): T[] {
  return connection?.edges?.flatMap((edge) => (edge?.node ? [edge.node] : [])) ?? [];
}

export function useMyBillingOverview(first = 10) {
  return useQuery({
    queryKey: ['billing', 'my-overview', first],
    queryFn: async () => {
      const data = await graphqlRequest<{
        myBillingAccount: BillingAccount;
        myPaymentOrders: Connection<PaymentOrder>;
        myLedgerTransactions: Connection<LedgerTransaction>;
        myUsageBillingRecords: Connection<UsageBillingRecord>;
        myRedeemCodes: Connection<RedeemCode>;
        availableSubscriptionPlans: Connection<SubscriptionPlan>;
        myUserSubscriptions: Connection<UserSubscription>;
        myAffiliateSummary: AffiliateSummary;
        myAffiliateInvitations: Connection<AffiliateInvitation>;
        myAffiliateRebates: Connection<AffiliateRebate>;
        myBillingNotificationPreference: BillingNotificationPreference;
        myBillingNotifications: Connection<BillingNotification>;
      }>(BILLING_OVERVIEW_QUERY, { first });

      return {
        account: data.myBillingAccount,
        paymentOrders: nodes(data.myPaymentOrders),
        ledgerTransactions: nodes(data.myLedgerTransactions),
        usageBillingRecords: nodes(data.myUsageBillingRecords),
        redeemCodes: nodes(data.myRedeemCodes),
        availableSubscriptionPlans: nodes(data.availableSubscriptionPlans),
        userSubscriptions: nodes(data.myUserSubscriptions),
        affiliateSummary: data.myAffiliateSummary,
        affiliateInvitations: nodes(data.myAffiliateInvitations),
        affiliateRebates: nodes(data.myAffiliateRebates),
        notificationPreference: data.myBillingNotificationPreference,
        notifications: nodes(data.myBillingNotifications),
      } satisfies BillingOverview;
    },
  });
}

export function useCreateMyEPayRechargeCheckout() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { amount: string; currency?: string; subject?: string; promoCode?: string }) => {
      const data = await graphqlRequest<{ createMyEPayRechargeCheckout: PaymentCheckout }>(CREATE_MY_EPAY_RECHARGE_CHECKOUT, {
        input,
      });
      return data.createMyEPayRechargeCheckout;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useRedeemCode() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { code: string }) => {
      const data = await graphqlRequest<{ redeemCode: RedeemCode }>(REDEEM_CODE_MUTATION, { input });
      return data.redeemCode;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function usePurchaseSubscriptionPlan() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { planId: string; promoCode?: string }) => {
      const data = await graphqlRequest<{ purchaseSubscriptionPlan: UserSubscription }>(PURCHASE_SUBSCRIPTION_PLAN_MUTATION, { input });
      return data.purchaseSubscriptionPlan;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useQuoteRechargePromo() {
  return useMutation({
    mutationFn: async (input: { amount: string; currency?: string; promoCode?: string }) => {
      const data = await graphqlRequest<{ quoteRechargePromo: PromoQuote }>(QUOTE_RECHARGE_PROMO_QUERY, { input });
      return data.quoteRechargePromo;
    },
  });
}

export function useQuoteSubscriptionPromo() {
  return useMutation({
    mutationFn: async (input: { planId: string; promoCode?: string }) => {
      const data = await graphqlRequest<{ quoteSubscriptionPromo: PromoQuote }>(QUOTE_SUBSCRIPTION_PROMO_QUERY, { input });
      return data.quoteSubscriptionPromo;
    },
  });
}

export function useBindAffiliateInvite() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { inviteCode: string; notes?: string }) => {
      const data = await graphqlRequest<{ bindAffiliateInvite: AffiliateInvitation }>(BIND_AFFILIATE_INVITE_MUTATION, { input });
      return data.bindAffiliateInvite;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useTransferAffiliateRebates() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      const data = await graphqlRequest<{ transferAffiliateRebates: AffiliateTransferResult }>(TRANSFER_AFFILIATE_REBATES_MUTATION);
      return data.transferAffiliateRebates;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useSaveMyBillingNotificationPreference() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: Omit<BillingNotificationPreference, 'id'>) => {
      const data = await graphqlRequest<{ saveMyBillingNotificationPreference: BillingNotificationPreference }>(
        SAVE_MY_BILLING_NOTIFICATION_PREFERENCE_MUTATION,
        { input }
      );
      return data.saveMyBillingNotificationPreference;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useMarkBillingNotificationRead() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ markBillingNotificationRead: Pick<BillingNotification, 'id' | 'status' | 'readAt'> }>(
        MARK_BILLING_NOTIFICATION_READ_MUTATION,
        { id }
      );
      return data.markBillingNotificationRead;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}

export function useMarkAllBillingNotificationsRead() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      const data = await graphqlRequest<{ markAllBillingNotificationsRead: number }>(MARK_ALL_BILLING_NOTIFICATIONS_READ_MUTATION);
      return data.markAllBillingNotificationsRead;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['billing', 'my-overview'] });
    },
  });
}
