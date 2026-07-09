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

type BillingNotificationPreference struct {
	ent.Schema
}

func (BillingNotificationPreference) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (BillingNotificationPreference) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").
			StorageKey("billing_notification_preferences_by_user").
			Unique(),
	}
}

func (BillingNotificationPreference) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").
			Immutable(),
		field.Bool("enabled").
			Default(true),
		field.Bool("low_balance_enabled").
			Default(true),
		field.Bool("payment_enabled").
			Default(true),
		field.Bool("subscription_enabled").
			Default(true),
		field.Bool("large_consumption_enabled").
			Default(true),
	}
}

func (BillingNotificationPreference) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("billing_notification_preferences").
			Field("user_id").
			Required().
			Immutable().
			Unique(),
	}
}

func (BillingNotificationPreference) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (BillingNotificationPreference) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadBilling),
			scopes.UserOwnedQueryRule(),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteBilling),
		},
	}
}
