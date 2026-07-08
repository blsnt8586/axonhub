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

type BillingPriceRule struct {
	ent.Schema
}

func (BillingPriceRule) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (BillingPriceRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("scope_type", "scope_id", "model_pattern", "priority").
			StorageKey("billing_price_rules_lookup"),
		index.Fields("reference_id").
			StorageKey("billing_price_rules_by_reference_id").
			Unique(),
	}
}

func (BillingPriceRule) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("scope_type").
			Values("global", "project", "group").
			Default("global").
			Comment("Pricing scope. v1 supports global and project; group is reserved."),
		field.Int("scope_id").
			Default(0).
			Comment("Scope identifier. Global rules use 0."),
		field.String("model_pattern").
			Comment("Model matcher. v1 supports exact model ID or * wildcard."),
		field.JSON("price", objects.ModelPrice{}).
			Comment("Customer sell price, separated from channel cost price."),
		field.String("currency").
			Default("CNY"),
		field.Int("priority").
			Default(0).
			Comment("Higher priority wins within the same scope."),
		field.Bool("enabled").
			Default(true),
		field.String("reference_id").
			Comment("Stable price version reference copied to billing records."),
	}
}

func (BillingPriceRule) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (BillingPriceRule) Policy() ent.Policy {
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
