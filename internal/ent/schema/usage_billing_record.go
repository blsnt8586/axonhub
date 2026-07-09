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

type UsageBillingRecord struct {
	ent.Schema
}

func (UsageBillingRecord) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (UsageBillingRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("usage_log_id").
			StorageKey("usage_billing_records_by_usage_log_id").
			Unique(),
		index.Fields("billing_account_id", "created_at").
			StorageKey("usage_billing_records_by_account_created_at"),
		index.Fields("project_id", "created_at").
			StorageKey("usage_billing_records_by_project_created_at"),
		index.Fields("user_id", "created_at").
			StorageKey("usage_billing_records_by_user_created_at"),
		index.Fields("idempotency_key").
			StorageKey("usage_billing_records_by_idempotency_key").
			Unique(),
		index.Fields("user_subscription_id", "created_at").
			StorageKey("usage_billing_records_by_subscription_created_at"),
	}
}

func (UsageBillingRecord) Fields() []ent.Field {
	return []ent.Field{
		field.Int("usage_log_id").
			Immutable(),
		field.Int("billing_account_id").
			Immutable(),
		field.Int("project_id").
			Immutable(),
		field.Int("user_id").
			Optional().
			Immutable(),
		field.Int("api_key_id").
			Optional().
			Immutable(),
		field.String("model_id").
			Immutable(),
		field.Enum("request_type").
			Values("chat", "image", "video", "embedding", "audio", "other").
			Default("chat").
			Immutable(),
		field.JSON("usage_snapshot", objects.JSONRawMessage{}).
			Optional().
			Immutable(),
		field.JSON("price_snapshot", objects.ModelPrice{}).
			Immutable(),
		field.String("price_reference_id").
			Immutable(),
		field.JSON("charge_items", []objects.CostItem{}).
			Default([]objects.CostItem{}).
			Immutable(),
		field.Int64("cost_amount_micros").
			Default(0).
			Immutable(),
		field.Int64("charge_amount_micros").
			Immutable(),
		field.String("currency").
			Default("CNY").
			Immutable(),
		field.Enum("status").
			Values("pending", "charged", "skipped", "failed", "refunded").
			Default("pending"),
		field.Int("ledger_transaction_id").
			Optional(),
		field.Int("user_subscription_id").
			Optional(),
		field.String("idempotency_key").
			Immutable(),
		field.String("error").
			Default(""),
	}
}

func (UsageBillingRecord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("usage_log", UsageLog.Type).
			Ref("usage_billing_records").
			Field("usage_log_id").
			Required().
			Immutable().
			Unique(),
		edge.From("billing_account", BillingAccount.Type).
			Ref("usage_billing_records").
			Field("billing_account_id").
			Required().
			Immutable().
			Unique(),
		edge.From("ledger_transaction", LedgerTransaction.Type).
			Ref("usage_billing_records").
			Field("ledger_transaction_id").
			Unique(),
		edge.From("user_subscription", UserSubscription.Type).
			Ref("usage_billing_records").
			Field("user_subscription_id").
			Unique(),
	}
}

func (UsageBillingRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (UsageBillingRecord) Policy() ent.Policy {
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
