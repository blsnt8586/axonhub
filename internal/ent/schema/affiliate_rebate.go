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

type AffiliateRebate struct {
	ent.Schema
}

func (AffiliateRebate) Mixin() []ent.Mixin {
	return []ent.Mixin{TimeMixin{}}
}

func (AffiliateRebate) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("idempotency_key").
			StorageKey("affiliate_rebates_by_idempotency_key").
			Unique(),
		index.Fields("source_type", "source_id").
			StorageKey("affiliate_rebates_by_source").
			Unique(),
		index.Fields("inviter_user_id", "status", "freeze_until").
			StorageKey("affiliate_rebates_by_inviter_status_freeze"),
	}
}

func (AffiliateRebate) Fields() []ent.Field {
	return []ent.Field{
		field.Int("invitation_id").
			Immutable(),
		field.Int("inviter_user_id").
			Immutable(),
		field.Int("invitee_user_id").
			Immutable(),
		field.Enum("source_type").
			Values("payment_order", "user_subscription").
			Immutable(),
		field.Int("source_id").
			Immutable(),
		field.Int("payment_order_id").
			Optional().
			Nillable().
			Immutable(),
		field.Int("user_subscription_id").
			Optional().
			Nillable().
			Immutable(),
		field.Int64("base_amount_micros").
			Positive().
			Immutable(),
		field.Int64("amount_micros").
			Positive().
			Immutable(),
		field.Int("rate_bps").
			NonNegative().
			Immutable(),
		field.String("currency").
			Default("CNY").
			Immutable(),
		field.Enum("status").
			Values("frozen", "available", "transferred", "voided").
			Default("frozen"),
		field.Time("freeze_until").
			Immutable(),
		field.Time("transferred_at").
			Optional().
			Nillable(),
		field.Int("ledger_transaction_id").
			Optional().
			Nillable(),
		field.String("idempotency_key").
			Immutable(),
		field.String("notes").
			Default(""),
	}
}

func (AffiliateRebate) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("invitation", AffiliateInvitation.Type).
			Ref("rebates").
			Field("invitation_id").
			Required().
			Immutable().
			Unique(),
		edge.From("inviter", User.Type).
			Ref("affiliate_rebates_earned").
			Field("inviter_user_id").
			Required().
			Immutable().
			Unique(),
		edge.From("invitee", User.Type).
			Ref("affiliate_rebates_generated").
			Field("invitee_user_id").
			Required().
			Immutable().
			Unique(),
		edge.From("payment_order", PaymentOrder.Type).
			Ref("affiliate_rebates").
			Field("payment_order_id").
			Unique().
			Immutable(),
		edge.From("user_subscription", UserSubscription.Type).
			Ref("affiliate_rebates").
			Field("user_subscription_id").
			Unique().
			Immutable(),
		edge.From("ledger_transaction", LedgerTransaction.Type).
			Ref("affiliate_rebates").
			Field("ledger_transaction_id").
			Unique(),
	}
}

func (AffiliateRebate) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (AffiliateRebate) Policy() ent.Policy {
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
