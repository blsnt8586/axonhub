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

type PaymentEvent struct {
	ent.Schema
}

func (PaymentEvent) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (PaymentEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("event_key").
			StorageKey("payment_events_by_event_key").
			Unique(),
		index.Fields("payment_order_id", "created_at").
			StorageKey("payment_events_by_order_created_at"),
	}
}

func (PaymentEvent) Fields() []ent.Field {
	return []ent.Field{
		field.String("event_key").
			Immutable().
			Comment("Idempotency key for provider callback or internal payment event."),
		field.Int("payment_order_id").
			Optional().
			Nillable().
			Immutable(),
		field.Int("provider_instance_id").
			Optional().
			Nillable().
			Immutable(),
		field.Enum("provider_type").
			Values("manual", "epay", "stripe", "custom").
			Immutable(),
		field.String("event_type").
			Immutable().
			Comment("Provider event type, for example paid/refund/notify."),
		field.JSON("payload", objects.JSONRawMessage{}).
			Optional(),
		field.Enum("status").
			Values("received", "processed", "failed", "ignored").
			Default("received"),
		field.String("error").
			Default(""),
	}
}

func (PaymentEvent) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("payment_order", PaymentOrder.Type).
			Ref("payment_events").
			Field("payment_order_id").
			Unique().
			Immutable(),
		edge.From("provider_instance", PaymentProviderInstance.Type).
			Ref("payment_events").
			Field("provider_instance_id").
			Unique().
			Immutable(),
	}
}

func (PaymentEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (PaymentEvent) Policy() ent.Policy {
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
