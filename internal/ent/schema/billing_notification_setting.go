package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/scopes"
)

type BillingNotificationSetting struct {
	ent.Schema
}

func (BillingNotificationSetting) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (BillingNotificationSetting) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("key").
			StorageKey("billing_notification_settings_by_key").
			Unique(),
	}
}

func (BillingNotificationSetting) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").
			Default("default").
			Immutable(),
		field.Bool("enabled").
			Default(true),
		field.Bool("user_notifications_enabled").
			Default(true),
		field.Bool("operator_alerts_enabled").
			Default(true),
		field.Int64("low_balance_threshold_micros").
			NonNegative().
			Default(10_000_000),
		field.Int64("large_consumption_threshold_micros").
			NonNegative().
			Default(100_000_000),
		field.Int("subscription_expiry_warning_days").
			NonNegative().
			Default(3),
		field.String("currency").
			Default("CNY"),
	}
}

func (BillingNotificationSetting) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (BillingNotificationSetting) Policy() ent.Policy {
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
