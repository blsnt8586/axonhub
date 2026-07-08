package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/scopes"
)

type BillingHold struct {
	ent.Schema
}

func (BillingHold) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (BillingHold) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("billing_account_id", "status", "expires_at").
			StorageKey("billing_holds_by_account_status_expires_at"),
		index.Fields("idempotency_key").
			StorageKey("billing_holds_by_idempotency_key").
			Unique(),
		index.Fields("request_id").
			StorageKey("billing_holds_by_request_id"),
		index.Fields("usage_log_id").
			StorageKey("billing_holds_by_usage_log_id"),
	}
}

func (BillingHold) Fields() []ent.Field {
	return []ent.Field{
		field.Int("billing_account_id").
			Immutable(),
		field.Int("request_id").
			Optional(),
		field.Int("usage_log_id").
			Optional(),
		field.Int("project_id").
			Optional().
			Immutable(),
		field.Int("user_id").
			Optional().
			Immutable(),
		field.Int("api_key_id").
			Optional().
			Immutable(),
		field.String("model_id").
			Default("").
			Immutable(),
		field.Int64("amount_micros").
			Positive().
			Immutable(),
		field.Int64("captured_amount_micros").
			Default(0),
		field.String("currency").
			Default("CNY").
			Immutable(),
		field.Enum("status").
			Values("held", "captured", "released", "expired").
			Default("held"),
		field.String("idempotency_key").
			Immutable(),
		field.String("reference_type").
			Default("").
			Immutable(),
		field.String("reference_id").
			Default("").
			Immutable(),
		field.Int("captured_ledger_transaction_id").
			Optional(),
		field.String("release_reason").
			Default(""),
		field.Enum("released_by_type").
			Values("system", "admin").
			Default("system"),
		field.String("released_by_id").
			Default(""),
		field.Time("expires_at"),
		field.Time("captured_at").
			Optional().
			Nillable(),
		field.Time("released_at").
			Optional().
			Nillable(),
	}
}

func (BillingHold) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("billing_account", BillingAccount.Type).
			Ref("billing_holds").
			Field("billing_account_id").
			Required().
			Immutable().
			Unique(),
		edge.From("request", Request.Type).
			Ref("billing_holds").
			Field("request_id").
			Unique(),
		edge.From("usage_log", UsageLog.Type).
			Ref("billing_holds").
			Field("usage_log_id").
			Unique(),
		edge.From("captured_ledger_transaction", LedgerTransaction.Type).
			Ref("billing_holds").
			Field("captured_ledger_transaction_id").
			Unique(),
	}
}

func (BillingHold) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (BillingHold) Policy() ent.Policy {
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
