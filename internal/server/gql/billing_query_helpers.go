package gql

import (
	"context"
	"fmt"

	"entgo.io/contrib/entgql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func requireBillingUser(ctx context.Context) (*ent.User, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, ErrNotOwner
	}

	return user, nil
}

func requireUserGUID(value objects.GUID) (int, error) {
	if value.Type != ent.TypeUser {
		return 0, fmt.Errorf("userId must be a User ID")
	}
	if value.ID <= 0 {
		return 0, fmt.Errorf("userId must be positive")
	}

	return value.ID, nil
}

func (r *queryResolver) userBillingAccount(ctx context.Context, userID int) (*ent.BillingAccount, error) {
	return authz.RunWithSystemBypass(ctx, "billing-user-account", func(ctx context.Context) (*ent.BillingAccount, error) {
		return r.billingAccountService.GetOrCreateForSubject(ctx, biz.UserBillingSubject(userID))
	})
}

func (r *queryResolver) userPaymentOrders(ctx context.Context, userID int, after *entgql.Cursor[int], first *int, before *entgql.Cursor[int], last *int, orderBy *ent.PaymentOrderOrder) (*ent.PaymentOrderConnection, error) {
	if err := validatePaginationArgs(first, last); err != nil {
		return nil, err
	}

	return authz.RunWithSystemBypass(ctx, "billing-user-payment-orders", func(ctx context.Context) (*ent.PaymentOrderConnection, error) {
		account, err := r.billingAccountService.GetOrCreateForSubject(ctx, biz.UserBillingSubject(userID))
		if err != nil {
			return nil, err
		}

		return r.client.PaymentOrder.Query().
			Where(paymentorder.BillingAccountIDEQ(account.ID)).
			Paginate(ctx, after, first, before, last, ent.WithPaymentOrderOrder(orderBy))
	})
}

func (r *queryResolver) userUsageBillingRecords(ctx context.Context, userID int, after *entgql.Cursor[int], first *int, before *entgql.Cursor[int], last *int, orderBy *ent.UsageBillingRecordOrder) (*ent.UsageBillingRecordConnection, error) {
	if err := validatePaginationArgs(first, last); err != nil {
		return nil, err
	}

	return authz.RunWithSystemBypass(ctx, "billing-user-usage-records", func(ctx context.Context) (*ent.UsageBillingRecordConnection, error) {
		return r.client.UsageBillingRecord.Query().
			Where(usagebillingrecord.UserIDEQ(userID)).
			Paginate(ctx, after, first, before, last, ent.WithUsageBillingRecordOrder(orderBy))
	})
}

func (r *queryResolver) userLedgerTransactions(ctx context.Context, userID int, after *entgql.Cursor[int], first *int, before *entgql.Cursor[int], last *int, orderBy *ent.LedgerTransactionOrder) (*ent.LedgerTransactionConnection, error) {
	if err := validatePaginationArgs(first, last); err != nil {
		return nil, err
	}

	return authz.RunWithSystemBypass(ctx, "billing-user-ledger-transactions", func(ctx context.Context) (*ent.LedgerTransactionConnection, error) {
		account, err := r.billingAccountService.GetOrCreateForSubject(ctx, biz.UserBillingSubject(userID))
		if err != nil {
			return nil, err
		}

		return r.client.LedgerTransaction.Query().
			Where(ledgertransaction.BillingAccountIDEQ(account.ID)).
			Paginate(ctx, after, first, before, last, ent.WithLedgerTransactionOrder(orderBy))
	})
}
