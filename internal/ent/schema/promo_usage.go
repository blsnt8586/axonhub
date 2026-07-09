package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

type PromoUsage struct {
	ent.Schema
}

func (PromoUsage) Mixin() []ent.Mixin {
	return []ent.Mixin{TimeMixin{}}
}

func (PromoUsage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("idempotency_key").StorageKey("promo_usages_by_idempotency_key").Unique(),
		index.Fields("promo_code_id", "created_at").StorageKey("promo_usages_by_code_created_at"),
		index.Fields("user_id", "promo_code_id").StorageKey("promo_usages_by_user_code"),
		index.Fields("payment_order_id").StorageKey("promo_usages_by_payment_order"),
		index.Fields("user_subscription_id").StorageKey("promo_usages_by_user_subscription"),
	}
}

func (PromoUsage) Fields() []ent.Field {
	return []ent.Field{
		field.Int("promo_code_id").Immutable(),
		field.String("code").Immutable(),
		field.JSON("code_snapshot", objects.JSONRawMessage{}).Optional().Immutable(),
		field.Int("user_id").Optional().Nillable().Immutable(),
		field.Int("billing_account_id").Optional().Nillable().Immutable(),
		field.Int("payment_order_id").Optional().Nillable(),
		field.Int("user_subscription_id").Optional().Nillable(),
		field.Int("ledger_transaction_id").Optional().Nillable(),
		field.Enum("scope").Values("recharge", "subscription").Immutable(),
		field.Enum("status").Values("reserved", "applied", "voided").Default("reserved"),
		field.Int64("original_amount_micros").NonNegative().Immutable(),
		field.Int64("discount_amount_micros").NonNegative().Immutable(),
		field.Int64("payable_amount_micros").NonNegative().Immutable(),
		field.String("currency").Default("CNY").Immutable(),
		field.String("idempotency_key").Immutable(),
		field.String("failure_reason").Default(""),
	}
}

func (PromoUsage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("promo_code", PromoCode.Type).
			Ref("usages").
			Field("promo_code_id").
			Required().
			Immutable().
			Unique(),
		edge.From("user", User.Type).
			Ref("promo_usages").
			Field("user_id").
			Unique().
			Immutable(),
		edge.From("billing_account", BillingAccount.Type).
			Ref("promo_usages").
			Field("billing_account_id").
			Unique().
			Immutable(),
		edge.From("payment_order", PaymentOrder.Type).
			Ref("promo_usages").
			Field("payment_order_id").
			Unique(),
		edge.From("user_subscription", UserSubscription.Type).
			Ref("promo_usages").
			Field("user_subscription_id").
			Unique(),
		edge.From("ledger_transaction", LedgerTransaction.Type).
			Ref("promo_usages").
			Field("ledger_transaction_id").
			Unique(),
	}
}

func (PromoUsage) Annotations() []schema.Annotation {
	return []schema.Annotation{entgql.QueryField(), entgql.RelayConnection()}
}

func (PromoUsage) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadBilling),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteBilling),
		},
	}
}
