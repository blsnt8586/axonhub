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

type LedgerTransaction struct {
	ent.Schema
}

func (LedgerTransaction) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (LedgerTransaction) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("billing_account_id").
			StorageKey("ledger_transactions_by_billing_account_id"),
		index.Fields("idempotency_key").
			StorageKey("ledger_transactions_by_idempotency_key").
			Unique(),
		index.Fields("reference_type", "reference_id").
			StorageKey("ledger_transactions_by_reference"),
	}
}

func (LedgerTransaction) Fields() []ent.Field {
	return []ent.Field{
		field.Int("billing_account_id").
			Immutable().
			Comment("Billing account affected by this transaction."),
		field.Enum("direction").
			Values("credit", "debit").
			Immutable().
			Comment("Balance movement direction."),
		field.Int64("amount_micros").
			Positive().
			Immutable().
			Comment("Positive amount in micro currency units."),
		field.String("currency").
			Default("CNY").
			Immutable().
			Comment("Currency for this transaction."),
		field.Enum("type").
			Values("payment_recharge", "usage_charge", "admin_adjustment", "refund", "chargeback", "subscription_grant", "subscription_deduct", "redeem_code").
			Immutable().
			Comment("Business reason for this transaction."),
		field.Enum("status").
			Values("posted", "voided").
			Default("posted").
			Comment("Ledger transaction status. Posted transactions are immutable business facts."),
		field.String("idempotency_key").
			Immutable().
			Comment("Unique idempotency key preventing duplicate balance mutation."),
		field.String("reference_type").
			Default("").
			Immutable().
			Comment("External business object type."),
		field.String("reference_id").
			Default("").
			Immutable().
			Comment("External business object identifier."),
		field.String("memo").
			Default("").
			Immutable().
			Comment("Human-readable memo."),
		field.Enum("created_by_type").
			Values("system", "admin", "provider").
			Default("system").
			Immutable().
			Comment("Actor type creating this transaction."),
		field.String("created_by_id").
			Default("").
			Immutable().
			Comment("Actor identifier creating this transaction."),
	}
}

func (LedgerTransaction) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("billing_account", BillingAccount.Type).
			Ref("ledger_transactions").
			Field("billing_account_id").
			Required().
			Immutable().
			Unique(),
		edge.To("entries", LedgerEntry.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("usage_billing_records", UsageBillingRecord.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("billing_holds", BillingHold.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("payment_orders", PaymentOrder.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("redeem_codes", RedeemCode.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (LedgerTransaction) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (LedgerTransaction) Policy() ent.Policy {
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
