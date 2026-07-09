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

type SubscriptionPlan struct {
	ent.Schema
}

func (SubscriptionPlan) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (SubscriptionPlan) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "sort_order").
			StorageKey("subscription_plans_by_status_sort"),
	}
}

func (SubscriptionPlan) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("Customer-facing plan name."),
		field.String("description").
			Default(""),
		field.Enum("period").
			Values("day", "month", "year", "custom").
			Default("month"),
		field.Int("period_days").
			Default(30).
			Comment("Billing and quota reset period in days. Month/year use normalized day counts in v1."),
		field.Int64("price_micros").
			Default(0).
			Comment("Purchase price in micro currency units."),
		field.String("currency").
			Default("CNY"),
		field.Int64("included_amount_micros").
			Default(0).
			Comment("Included billable amount in micro currency units for each period. Zero means unlimited."),
		field.JSON("supported_model_ids", []string{}).
			Default([]string{}).
			Comment("Empty means all models."),
		field.JSON("supported_project_ids", []int{}).
			Default([]int{}).
			Comment("Empty means all projects."),
		field.JSON("supported_group_ids", []int{}).
			Default([]int{}).
			Comment("Reserved for user/group segmentation. Empty means all groups."),
		field.Bool("allow_wallet_fallback").
			Default(true).
			Comment("If enabled, requests can fall back to wallet billing after quota exhaustion."),
		field.Enum("status").
			Values("enabled", "disabled", "archived").
			Default("enabled"),
		field.Int("sort_order").
			Default(0),
		field.JSON("metadata", objects.JSONRawMessage{}).
			Optional(),
	}
}

func (SubscriptionPlan) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user_subscriptions", UserSubscription.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (SubscriptionPlan) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (SubscriptionPlan) Policy() ent.Policy {
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
