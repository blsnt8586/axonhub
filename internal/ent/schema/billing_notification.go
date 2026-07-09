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

type BillingNotification struct {
	ent.Schema
}

func (BillingNotification) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (BillingNotification) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("event_key").
			StorageKey("billing_notifications_by_event_key").
			Unique(),
		index.Fields("user_id", "status", "created_at").
			StorageKey("billing_notifications_by_user_status_created_at"),
		index.Fields("audience", "category", "created_at").
			StorageKey("billing_notifications_by_audience_category_created_at"),
		index.Fields("payment_order_id").
			StorageKey("billing_notifications_by_payment_order"),
		index.Fields("user_subscription_id").
			StorageKey("billing_notifications_by_user_subscription"),
		index.Fields("usage_billing_record_id").
			StorageKey("billing_notifications_by_usage_record"),
	}
}

func (BillingNotification) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").
			Optional().
			Nillable(),
		field.Enum("audience").
			Values("user", "operator").
			Default("user"),
		field.Enum("category").
			Values("low_balance", "payment", "subscription", "large_consumption", "operator_alert").
			Immutable(),
		field.Enum("severity").
			Values("info", "warning", "error").
			Default("info"),
		field.Enum("status").
			Values("unread", "read", "dismissed").
			Default("unread"),
		field.String("event_key").
			Immutable(),
		field.String("title"),
		field.String("message").
			Default(""),
		field.String("currency").
			Default("CNY"),
		field.Int64("amount_micros").
			Optional().
			Nillable(),
		field.Int("billing_account_id").
			Optional().
			Nillable(),
		field.Int("payment_order_id").
			Optional().
			Nillable(),
		field.Int("user_subscription_id").
			Optional().
			Nillable(),
		field.Int("usage_billing_record_id").
			Optional().
			Nillable(),
		field.Time("read_at").
			Optional().
			Nillable(),
		field.JSON("metadata", objects.JSONRawMessage{}).
			Optional(),
	}
}

func (BillingNotification) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("billing_notifications").
			Field("user_id").
			Unique(),
		edge.From("billing_account", BillingAccount.Type).
			Ref("billing_notifications").
			Field("billing_account_id").
			Unique(),
		edge.From("payment_order", PaymentOrder.Type).
			Ref("billing_notifications").
			Field("payment_order_id").
			Unique(),
		edge.From("user_subscription", UserSubscription.Type).
			Ref("billing_notifications").
			Field("user_subscription_id").
			Unique(),
		edge.From("usage_billing_record", UsageBillingRecord.Type).
			Ref("billing_notifications").
			Field("usage_billing_record_id").
			Unique(),
	}
}

func (BillingNotification) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (BillingNotification) Policy() ent.Policy {
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
