import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

export type BillingAccountStatus = 'active' | 'frozen' | 'closed';
export type LedgerTransactionDirection = 'credit' | 'debit';

export interface BillingAccount {
  id: string;
  ownerType: string;
  ownerID: number;
  currency: string;
  balanceMicros: number;
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
  direction: LedgerTransactionDirection;
  amountMicros: number;
  currency: string;
  type: string;
  status: string;
  memo: string;
}

export interface PaymentOrder {
  id: string;
  createdAt: string;
  orderNo: string;
  providerType: string;
  amountMicros: number;
  currency: string;
  status: string;
  externalTradeNo?: string | null;
  paidAt?: string | null;
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
      status
    }
    userLedgerTransactions(userId: $userId, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
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
    userPaymentOrders(userId: $userId, first: $first, orderBy: { field: CREATED_AT, direction: DESC }) {
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
        }
      }
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
      }>(USER_BILLING_DETAIL_QUERY, { userId, first });

      return {
        account: data.userBillingAccount,
        ledgerTransactions: nodes(data.userLedgerTransactions),
        paymentOrders: nodes(data.userPaymentOrders),
      };
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
