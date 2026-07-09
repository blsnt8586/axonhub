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

type UserSubscription struct {
	ent.Schema
}

func (UserSubscription) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (UserSubscription) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "status", "expires_at").
			StorageKey("user_subscriptions_by_user_status_expiry"),
		index.Fields("plan_id", "created_at").
			StorageKey("user_subscriptions_by_plan_created_at"),
		index.Fields("reset_at").
			StorageKey("user_subscriptions_by_reset_at"),
	}
}

func (UserSubscription) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").
			Immutable(),
		field.Int("plan_id").
			Optional().
			Immutable(),
		field.JSON("plan_snapshot", objects.JSONRawMessage{}).
			Optional().
			Immutable(),
		field.Enum("status").
			Values("active", "expired", "revoked", "canceled").
			Default("active"),
		field.Time("starts_at"),
		field.Time("expires_at"),
		field.Time("current_period_start"),
		field.Time("current_period_end"),
		field.Time("reset_at"),
		field.Int("period_days").
			Default(30),
		field.Int64("included_amount_micros").
			Default(0),
		field.Int64("used_amount_micros").
			Default(0),
		field.String("currency").
			Default("CNY"),
		field.JSON("supported_model_ids", []string{}).
			Default([]string{}),
		field.JSON("supported_project_ids", []int{}).
			Default([]int{}),
		field.JSON("supported_group_ids", []int{}).
			Default([]int{}),
		field.Bool("allow_wallet_fallback").
			Default(true),
		field.Int("assigned_by_id").
			Optional(),
		field.Int("purchase_ledger_transaction_id").
			Optional(),
		field.Int64("original_price_micros").
			NonNegative().
			Default(0).
			Immutable(),
		field.Int64("discount_amount_micros").
			NonNegative().
			Default(0).
			Immutable(),
		field.Int64("payable_amount_micros").
			NonNegative().
			Default(0).
			Immutable(),
		field.Int("promo_code_id").
			Optional().
			Nillable().
			Immutable(),
		field.String("notes").
			Default(""),
		field.String("revoke_reason").
			Default(""),
	}
}

func (UserSubscription) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("user_subscriptions").
			Field("user_id").
			Required().
			Immutable().
			Unique(),
		edge.From("plan", SubscriptionPlan.Type).
			Ref("user_subscriptions").
			Field("plan_id").
			Immutable().
			Unique(),
		edge.From("assigned_by", User.Type).
			Ref("assigned_user_subscriptions").
			Field("assigned_by_id").
			Unique(),
		edge.From("purchase_ledger_transaction", LedgerTransaction.Type).
			Ref("purchased_user_subscriptions").
			Field("purchase_ledger_transaction_id").
			Unique(),
		edge.From("promo_code", PromoCode.Type).
			Ref("user_subscriptions").
			Field("promo_code_id").
			Unique().
			Immutable(),
		edge.To("promo_usages", PromoUsage.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("usage_billing_records", UsageBillingRecord.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (UserSubscription) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (UserSubscription) Policy() ent.Policy {
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
