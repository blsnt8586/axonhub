import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

export type BillingAccountStatus = 'active' | 'frozen' | 'closed';
export type PaymentOrderStatus = 'pending' | 'paid' | 'failed' | 'canceled' | 'expired' | 'refunded';
export type LedgerTransactionDirection = 'credit' | 'debit';
export type UsageBillingRecordStatus = 'pending' | 'charged' | 'failed';
export type RedeemCodeStatus = 'active' | 'used' | 'disabled' | 'expired';
export type RedeemCodeType = 'balance' | 'credit' | 'subscription';

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

export interface BillingOverview {
  account: BillingAccount;
  paymentOrders: PaymentOrder[];
  ledgerTransactions: LedgerTransaction[];
  usageBillingRecords: UsageBillingRecord[];
  redeemCodes: RedeemCode[];
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
      }>(BILLING_OVERVIEW_QUERY, { first });

      return {
        account: data.myBillingAccount,
        paymentOrders: nodes(data.myPaymentOrders),
        ledgerTransactions: nodes(data.myLedgerTransactions),
        usageBillingRecords: nodes(data.myUsageBillingRecords),
        redeemCodes: nodes(data.myRedeemCodes),
      } satisfies BillingOverview;
    },
  });
}

export function useCreateMyEPayRechargeCheckout() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { amount: string; currency?: string; subject?: string }) => {
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
