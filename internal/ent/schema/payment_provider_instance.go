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

type PaymentProviderInstance struct {
	ent.Schema
}

func (PaymentProviderInstance) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (PaymentProviderInstance) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("provider_type", "name").
			StorageKey("payment_provider_instances_by_type_name").
			Unique(),
	}
}

func (PaymentProviderInstance) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Immutable().
			Comment("Operator-facing provider instance name."),
		field.Enum("provider_type").
			Values("manual", "epay", "stripe", "custom").
			Immutable().
			Comment("Payment provider adapter type."),
		field.Enum("status").
			Values("enabled", "disabled").
			Default("enabled").
			Comment("Provider instance availability."),
		field.String("currency").
			Default("CNY").
			Comment("Default settlement currency."),
		field.JSON("config", objects.JSONRawMessage{}).
			Optional().
			Sensitive().
			Annotations(entgql.Skip(entgql.SkipAll)).
			Comment("Provider-specific encrypted or externalized configuration payload."),
	}
}

func (PaymentProviderInstance) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("payment_orders", PaymentOrder.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("payment_events", PaymentEvent.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (PaymentProviderInstance) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (PaymentProviderInstance) Policy() ent.Policy {
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
