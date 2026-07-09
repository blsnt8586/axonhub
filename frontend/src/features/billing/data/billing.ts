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

export interface BillingOverview {
  account: BillingAccount;
  paymentOrders: PaymentOrder[];
  ledgerTransactions: LedgerTransaction[];
  usageBillingRecords: UsageBillingRecord[];
  redeemCodes: RedeemCode[];
  availableSubscriptionPlans: SubscriptionPlan[];
  userSubscriptions: UserSubscription[];
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
      }>(BILLING_OVERVIEW_QUERY, { first });

      return {
        account: data.myBillingAccount,
        paymentOrders: nodes(data.myPaymentOrders),
        ledgerTransactions: nodes(data.myLedgerTransactions),
        usageBillingRecords: nodes(data.myUsageBillingRecords),
        redeemCodes: nodes(data.myRedeemCodes),
        availableSubscriptionPlans: nodes(data.availableSubscriptionPlans),
        userSubscriptions: nodes(data.myUserSubscriptions),
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
