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

type PaymentOrder struct {
	ent.Schema
}

func (PaymentOrder) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (PaymentOrder) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("order_no").
			StorageKey("payment_orders_by_order_no").
			Unique(),
		index.Fields("project_id", "created_at").
			StorageKey("payment_orders_by_project_created_at"),
		index.Fields("billing_account_id", "created_at").
			StorageKey("payment_orders_by_account_created_at"),
		index.Fields("provider_type", "external_trade_no").
			StorageKey("payment_orders_by_provider_trade_no").
			Unique(),
	}
}

func (PaymentOrder) Fields() []ent.Field {
	return []ent.Field{
		field.String("order_no").
			Immutable().
			Comment("Internal payment order number."),
		field.Int("project_id").
			Immutable().
			Comment("Project purchasing the balance."),
		field.Int("billing_account_id").
			Immutable().
			Comment("Billing account credited after successful payment."),
		field.Int("provider_instance_id").
			Optional().
			Nillable().
			Immutable().
			Comment("Payment provider instance used for the order."),
		field.Enum("provider_type").
			Values("manual", "epay", "stripe", "custom").
			Immutable().
			Comment("Provider adapter type snapshot."),
		field.Enum("purpose").
			Values("recharge", "subscription").
			Default("recharge").
			Immutable().
			Comment("Commercial purpose of the payment order."),
		field.Int64("amount_micros").
			Positive().
			Immutable().
			Comment("Order amount in micro currency units."),
		field.String("currency").
			Default("CNY").
			Immutable(),
		field.Enum("status").
			Values("pending", "paid", "failed", "canceled", "expired", "refunded").
			Default("pending").
			Comment("Payment order lifecycle status."),
		field.String("external_trade_no").
			Optional().
			Nillable().
			Comment("Provider-side trade number."),
		field.Time("paid_at").
			Optional().
			Nillable(),
		field.Int("ledger_transaction_id").
			Optional().
			Nillable().
			Comment("Ledger transaction posted for successful recharge."),
		field.JSON("metadata", objects.JSONRawMessage{}).
			Optional().
			Comment("Order metadata snapshot."),
	}
}

func (PaymentOrder) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("billing_account", BillingAccount.Type).
			Ref("payment_orders").
			Field("billing_account_id").
			Required().
			Immutable().
			Unique(),
		edge.From("provider_instance", PaymentProviderInstance.Type).
			Ref("payment_orders").
			Field("provider_instance_id").
			Unique().
			Immutable(),
		edge.From("ledger_transaction", LedgerTransaction.Type).
			Ref("payment_orders").
			Field("ledger_transaction_id").
			Unique(),
		edge.To("payment_events", PaymentEvent.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (PaymentOrder) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (PaymentOrder) Policy() ent.Policy {
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
