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

type RedeemCode struct {
	ent.Schema
}

func (RedeemCode) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (RedeemCode) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").
			StorageKey("redeem_codes_by_code").
			Unique(),
		index.Fields("status", "expires_at").
			StorageKey("redeem_codes_by_status_expires_at"),
		index.Fields("used_by_id", "used_at").
			StorageKey("redeem_codes_by_used_by_used_at"),
		index.Fields("created_by_id", "created_at").
			StorageKey("redeem_codes_by_created_by_created_at"),
		index.Fields("batch_id").
			StorageKey("redeem_codes_by_batch_id"),
	}
}

func (RedeemCode) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").
			Immutable().
			Comment("Human redeem code. Stored normalized as uppercase."),
		field.Enum("type").
			Values("balance", "credit", "subscription").
			Default("balance").
			Immutable().
			Comment("Redeem target type. Balance is implemented first; credit and subscription are reserved."),
		field.Enum("status").
			Values("active", "used", "disabled", "expired").
			Default("active").
			Comment("Redeem code lifecycle status."),
		field.Int64("amount_micros").
			Positive().
			Immutable().
			Comment("Balance grant amount in micro currency units."),
		field.String("currency").
			Default("CNY").
			Immutable(),
		field.Int("created_by_id").
			Optional().
			Nillable().
			Immutable().
			Comment("Admin user that created the code."),
		field.Int("used_by_id").
			Optional().
			Nillable().
			Comment("User that redeemed the code."),
		field.Time("used_at").
			Optional().
			Nillable(),
		field.Time("expires_at").
			Optional().
			Nillable(),
		field.String("notes").
			Default(""),
		field.Int("ledger_transaction_id").
			Optional().
			Nillable().
			Comment("Ledger transaction posted for successful redeem."),
		field.String("batch_id").
			Default("").
			Immutable().
			Comment("Batch identifier for admin-generated campaigns."),
	}
}

func (RedeemCode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("created_by", User.Type).
			Ref("created_redeem_codes").
			Field("created_by_id").
			Unique().
			Immutable(),
		edge.From("used_by", User.Type).
			Ref("used_redeem_codes").
			Field("used_by_id").
			Unique(),
		edge.From("ledger_transaction", LedgerTransaction.Type).
			Ref("redeem_codes").
			Field("ledger_transaction_id").
			Unique(),
	}
}

func (RedeemCode) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (RedeemCode) Policy() ent.Policy {
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
