import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

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
  | 'subscription_deduct';
export type UsageBillingRecordStatus = 'pending' | 'charged' | 'skipped' | 'failed' | 'refunded';
export type PaymentOrderStatus = 'pending' | 'paid' | 'failed' | 'canceled' | 'expired' | 'refunded';
export type PaymentProviderType = 'manual' | 'epay' | 'stripe' | 'custom';
export type PaymentEventStatus = 'received' | 'processed' | 'failed' | 'ignored';
export type BillingHoldStatus = 'held' | 'captured' | 'released' | 'expired';

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
