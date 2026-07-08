package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

type BillingOutbox struct {
	ent.Schema
}

func (BillingOutbox) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (BillingOutbox) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("event_key").
			StorageKey("billing_outbox_by_event_key").
			Unique(),
		index.Fields("status", "next_attempt_at").
			StorageKey("billing_outbox_by_status_next_attempt_at"),
	}
}

func (BillingOutbox) Fields() []ent.Field {
	return []ent.Field{
		field.String("event_key").
			Immutable(),
		field.Enum("event_type").
			Values("usage_billing_requested", "payment_reconcile_requested").
			Immutable(),
		field.JSON("payload", objects.JSONRawMessage{}).
			Optional(),
		field.Enum("status").
			Values("pending", "processing", "done", "failed").
			Default("pending"),
		field.Int("attempts").
			Default(0),
		field.Time("next_attempt_at").
			Optional().
			Nillable(),
		field.String("last_error").
			Default(""),
	}
}

func (BillingOutbox) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (BillingOutbox) Policy() ent.Policy {
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
