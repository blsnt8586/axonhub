package gql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"entgo.io/contrib/entgql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billinghold"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/paymentevent"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	"github.com/looplj/axonhub/internal/ent/redeemcode"
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

func (r *queryResolver) userRedeemCodes(ctx context.Context, userID int, after *entgql.Cursor[int], first *int, before *entgql.Cursor[int], last *int, orderBy *ent.RedeemCodeOrder) (*ent.RedeemCodeConnection, error) {
	if err := validatePaginationArgs(first, last); err != nil {
		return nil, err
	}

	return authz.RunWithSystemBypass(ctx, "billing-user-redeem-codes", func(ctx context.Context) (*ent.RedeemCodeConnection, error) {
		return r.client.RedeemCode.Query().
			Where(redeemcode.UsedByIDEQ(userID)).
			Paginate(ctx, after, first, before, last, ent.WithRedeemCodeOrder(orderBy))
	})
}

func (r *queryResolver) adminLedgerTransactions(ctx context.Context, filter *AdminLedgerTransactionsFilter, after *entgql.Cursor[int], first *int, before *entgql.Cursor[int], last *int, orderBy *ent.LedgerTransactionOrder) (*ent.LedgerTransactionConnection, error) {
	if err := validatePaginationArgs(first, last); err != nil {
		return nil, err
	}

	return authz.RunWithSystemBypass(ctx, "billing-admin-ledger-transactions", func(ctx context.Context) (*ent.LedgerTransactionConnection, error) {
		query := r.client.LedgerTransaction.Query()
		if filter != nil {
			if filter.UserID != nil {
				account, err := r.billingAccountService.GetBySubject(ctx, biz.UserBillingSubject(*filter.UserID))
				if err != nil {
					if !errors.Is(err, biz.ErrBillingAccountNotFound) {
						return nil, err
					}
					query.Where(ledgertransaction.BillingAccountIDEQ(-1))
				} else {
					query.Where(ledgertransaction.BillingAccountIDEQ(account.ID))
				}
			}
			if filter.BillingAccountID != nil {
				query.Where(ledgertransaction.BillingAccountIDEQ(*filter.BillingAccountID))
			}
			if filter.Direction != nil {
				query.Where(ledgertransaction.DirectionEQ(*filter.Direction))
			}
			if filter.Status != nil {
				query.Where(ledgertransaction.StatusEQ(*filter.Status))
			}
			if filter.Type != nil {
				query.Where(ledgertransaction.TypeEQ(*filter.Type))
			}
			if value := strings.TrimSpace(stringValue(filter.ReferenceType)); value != "" {
				query.Where(ledgertransaction.ReferenceTypeEQ(value))
			}
			if filter.From != nil {
				query.Where(ledgertransaction.CreatedAtGTE(*filter.From))
			}
			if filter.To != nil {
				query.Where(ledgertransaction.CreatedAtLTE(*filter.To))
			}
		}

		return query.Paginate(ctx, after, first, before, last, ent.WithLedgerTransactionOrder(orderBy))
	})
}

func (r *queryResolver) adminRedeemCodes(ctx context.Context, filter *AdminRedeemCodesFilter, after *entgql.Cursor[int], first *int, before *entgql.Cursor[int], last *int, orderBy *ent.RedeemCodeOrder) (*ent.RedeemCodeConnection, error) {
	if err := validatePaginationArgs(first, last); err != nil {
		return nil, err
	}

	return authz.RunWithSystemBypass(ctx, "billing-admin-redeem-codes", func(ctx context.Context) (*ent.RedeemCodeConnection, error) {
		query := r.client.RedeemCode.Query()
		if filter != nil {
			if filter.UserID != nil {
				query.Where(redeemcode.UsedByIDEQ(*filter.UserID))
			}
			if filter.CreatedByID != nil {
				query.Where(redeemcode.CreatedByIDEQ(*filter.CreatedByID))
			}
			if filter.Status != nil {
				query.Where(redeemcode.StatusEQ(*filter.Status))
			}
			if filter.Type != nil {
				query.Where(redeemcode.TypeEQ(*filter.Type))
			}
			if value := strings.TrimSpace(stringValue(filter.Code)); value != "" {
				query.Where(redeemcode.CodeContainsFold(value))
			}
			if value := strings.TrimSpace(stringValue(filter.BatchID)); value != "" {
				query.Where(redeemcode.BatchIDEQ(value))
			}
			if filter.From != nil {
				query.Where(redeemcode.CreatedAtGTE(*filter.From))
			}
			if filter.To != nil {
				query.Where(redeemcode.CreatedAtLTE(*filter.To))
			}
			if filter.ExpiresBefore != nil {
				query.Where(redeemcode.ExpiresAtLTE(*filter.ExpiresBefore))
			}
		}

		return query.Paginate(ctx, after, first, before, last, ent.WithRedeemCodeOrder(orderBy))
	})
}

func (r *queryResolver) adminUsageBillingRecords(ctx context.Context, filter *AdminUsageBillingRecordsFilter, after *entgql.Cursor[int], first *int, before *entgql.Cursor[int], last *int, orderBy *ent.UsageBillingRecordOrder) (*ent.UsageBillingRecordConnection, error) {
	if err := validatePaginationArgs(first, last); err != nil {
		return nil, err
	}

	return authz.RunWithSystemBypass(ctx, "billing-admin-usage-records", func(ctx context.Context) (*ent.UsageBillingRecordConnection, error) {
		query := r.client.UsageBillingRecord.Query()
		if filter != nil {
			if filter.UserID != nil {
				query.Where(usagebillingrecord.UserIDEQ(*filter.UserID))
			}
			if filter.ProjectID != nil {
				query.Where(usagebillingrecord.ProjectIDEQ(*filter.ProjectID))
			}
			if filter.APIKeyID != nil {
				query.Where(usagebillingrecord.APIKeyIDEQ(*filter.APIKeyID))
			}
			if filter.BillingAccountID != nil {
				query.Where(usagebillingrecord.BillingAccountIDEQ(*filter.BillingAccountID))
			}
			if value := strings.TrimSpace(stringValue(filter.ModelID)); value != "" {
				query.Where(usagebillingrecord.ModelIDContainsFold(value))
			}
			if filter.Status != nil {
				query.Where(usagebillingrecord.StatusEQ(*filter.Status))
			}
			if filter.From != nil {
				query.Where(usagebillingrecord.CreatedAtGTE(*filter.From))
			}
			if filter.To != nil {
				query.Where(usagebillingrecord.CreatedAtLTE(*filter.To))
			}
		}

		return query.Paginate(ctx, after, first, before, last, ent.WithUsageBillingRecordOrder(orderBy))
	})
}

func (r *queryResolver) adminBillingHolds(ctx context.Context, filter *AdminBillingHoldsFilter, after *entgql.Cursor[int], first *int, before *entgql.Cursor[int], last *int, orderBy *ent.BillingHoldOrder) (*ent.BillingHoldConnection, error) {
	if err := validatePaginationArgs(first, last); err != nil {
		return nil, err
	}

	return authz.RunWithSystemBypass(ctx, "billing-admin-holds", func(ctx context.Context) (*ent.BillingHoldConnection, error) {
		query := r.client.BillingHold.Query()
		if filter != nil {
			if filter.UserID != nil {
				query.Where(billinghold.UserIDEQ(*filter.UserID))
			}
			if filter.ProjectID != nil {
				query.Where(billinghold.ProjectIDEQ(*filter.ProjectID))
			}
			if filter.APIKeyID != nil {
				query.Where(billinghold.APIKeyIDEQ(*filter.APIKeyID))
			}
			if filter.BillingAccountID != nil {
				query.Where(billinghold.BillingAccountIDEQ(*filter.BillingAccountID))
			}
			if value := strings.TrimSpace(stringValue(filter.ModelID)); value != "" {
				query.Where(billinghold.ModelIDContainsFold(value))
			}
			if filter.Status != nil {
				query.Where(billinghold.StatusEQ(*filter.Status))
			}
			if filter.From != nil {
				query.Where(billinghold.CreatedAtGTE(*filter.From))
			}
			if filter.To != nil {
				query.Where(billinghold.CreatedAtLTE(*filter.To))
			}
			if filter.ExpiresBefore != nil {
				query.Where(billinghold.ExpiresAtLTE(*filter.ExpiresBefore))
			}
		}

		return query.Paginate(ctx, after, first, before, last, ent.WithBillingHoldOrder(orderBy))
	})
}

func (r *queryResolver) adminPaymentOrders(ctx context.Context, filter *AdminPaymentOrdersFilter, after *entgql.Cursor[int], first *int, before *entgql.Cursor[int], last *int, orderBy *ent.PaymentOrderOrder) (*ent.PaymentOrderConnection, error) {
	if err := validatePaginationArgs(first, last); err != nil {
		return nil, err
	}

	return authz.RunWithSystemBypass(ctx, "billing-admin-payment-orders", func(ctx context.Context) (*ent.PaymentOrderConnection, error) {
		query := r.client.PaymentOrder.Query()
		if filter != nil {
			if filter.UserID != nil {
				account, err := r.billingAccountService.GetBySubject(ctx, biz.UserBillingSubject(*filter.UserID))
				if err != nil {
					if !errors.Is(err, biz.ErrBillingAccountNotFound) {
						return nil, err
					}
					query.Where(paymentorder.BillingAccountIDEQ(-1))
				} else {
					query.Where(paymentorder.BillingAccountIDEQ(account.ID))
				}
			}
			if filter.ProjectID != nil {
				query.Where(paymentorder.ProjectIDEQ(*filter.ProjectID))
			}
			if filter.BillingAccountID != nil {
				query.Where(paymentorder.BillingAccountIDEQ(*filter.BillingAccountID))
			}
			if filter.ProviderType != nil {
				query.Where(paymentorder.ProviderTypeEQ(*filter.ProviderType))
			}
			if filter.Status != nil {
				query.Where(paymentorder.StatusEQ(*filter.Status))
			}
			if value := strings.TrimSpace(stringValue(filter.OrderNo)); value != "" {
				query.Where(paymentorder.OrderNoContainsFold(value))
			}
			if value := strings.TrimSpace(stringValue(filter.ExternalTradeNo)); value != "" {
				query.Where(paymentorder.ExternalTradeNoContainsFold(value))
			}
			if filter.From != nil {
				query.Where(paymentorder.CreatedAtGTE(*filter.From))
			}
			if filter.To != nil {
				query.Where(paymentorder.CreatedAtLTE(*filter.To))
			}
		}

		return query.Paginate(ctx, after, first, before, last, ent.WithPaymentOrderOrder(orderBy))
	})
}

func (r *queryResolver) adminPaymentEvents(ctx context.Context, filter *AdminPaymentEventsFilter, after *entgql.Cursor[int], first *int, before *entgql.Cursor[int], last *int, orderBy *ent.PaymentEventOrder) (*ent.PaymentEventConnection, error) {
	if err := validatePaginationArgs(first, last); err != nil {
		return nil, err
	}

	return authz.RunWithSystemBypass(ctx, "billing-admin-payment-events", func(ctx context.Context) (*ent.PaymentEventConnection, error) {
		query := r.client.PaymentEvent.Query()
		if filter != nil {
			if filter.PaymentOrderID != nil {
				query.Where(paymentevent.PaymentOrderIDEQ(*filter.PaymentOrderID))
			}
			if filter.ProviderInstanceID != nil {
				query.Where(paymentevent.ProviderInstanceIDEQ(*filter.ProviderInstanceID))
			}
			if filter.ProviderType != nil {
				query.Where(paymentevent.ProviderTypeEQ(*filter.ProviderType))
			}
			if filter.Status != nil {
				query.Where(paymentevent.StatusEQ(*filter.Status))
			}
			if value := strings.TrimSpace(stringValue(filter.EventType)); value != "" {
				query.Where(paymentevent.EventTypeContainsFold(value))
			}
			if value := strings.TrimSpace(stringValue(filter.EventKey)); value != "" {
				query.Where(paymentevent.EventKeyContainsFold(value))
			}
			if filter.From != nil {
				query.Where(paymentevent.CreatedAtGTE(*filter.From))
			}
			if filter.To != nil {
				query.Where(paymentevent.CreatedAtLTE(*filter.To))
			}
		}

		return query.Paginate(ctx, after, first, before, last, ent.WithPaymentEventOrder(orderBy))
	})
}
