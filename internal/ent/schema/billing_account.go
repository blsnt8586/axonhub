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

type BillingAccount struct {
	ent.Schema
}

func (BillingAccount) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (BillingAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_type", "owner_id").
			StorageKey("billing_accounts_by_owner").
			Unique(),
	}
}

func (BillingAccount) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("owner_type").
			Values("user", "project").
			Default("user").
			Immutable().
			Comment("Commercial owner type. User is the default wallet owner; project is reserved for shared enterprise wallets."),
		field.Int("owner_id").
			Immutable().
			Comment("Owner identifier matching owner_type."),
		field.String("currency").
			Default("CNY").
			Comment("ISO-like currency code used by this billing account."),
		field.Int64("balance_micros").
			Default(0).
			Comment("Materialized balance in micro currency units. Ledger transactions are the source of truth."),
		field.Int64("held_balance_micros").
			Default(0).
			Comment("Materialized amount currently reserved by active billing holds in micro currency units."),
		field.Int64("credit_limit_micros").
			Default(0).
			Comment("Allowed overdraft in micro currency units."),
		field.Enum("status").
			Values("active", "frozen", "closed").
			Default("active").
			Comment("Billing account lifecycle status."),
	}
}

func (BillingAccount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("bindings", BillingAccountBinding.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("ledger_transactions", LedgerTransaction.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("billing_holds", BillingHold.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("usage_billing_records", UsageBillingRecord.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("payment_orders", PaymentOrder.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("promo_usages", PromoUsage.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (BillingAccount) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (BillingAccount) Policy() ent.Policy {
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
