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

type LedgerEntry struct {
	ent.Schema
}

func (LedgerEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (LedgerEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ledger_transaction_id").
			StorageKey("ledger_entries_by_transaction_id"),
	}
}

func (LedgerEntry) Fields() []ent.Field {
	return []ent.Field{
		field.Int("ledger_transaction_id").
			Immutable().
			Comment("Parent ledger transaction."),
		field.Enum("account_side").
			Values("customer_balance", "platform_revenue", "payment_clearing", "adjustment").
			Immutable().
			Comment("Accounting side reserved for double-entry expansion."),
		field.Enum("direction").
			Values("credit", "debit").
			Immutable().
			Comment("Entry direction."),
		field.Int64("amount_micros").
			Positive().
			Immutable().
			Comment("Positive amount in micro currency units."),
		field.String("currency").
			Default("CNY").
			Immutable(),
	}
}

func (LedgerEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("ledger_transaction", LedgerTransaction.Type).
			Ref("entries").
			Field("ledger_transaction_id").
			Required().
			Immutable().
			Unique(),
	}
}

func (LedgerEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (LedgerEntry) Policy() ent.Policy {
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
