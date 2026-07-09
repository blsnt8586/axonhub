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

type AffiliateProfile struct {
	ent.Schema
}

func (AffiliateProfile) Mixin() []ent.Mixin {
	return []ent.Mixin{TimeMixin{}}
}

func (AffiliateProfile) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").
			StorageKey("affiliate_profiles_by_user").
			Unique(),
		index.Fields("invite_code").
			StorageKey("affiliate_profiles_by_invite_code").
			Unique(),
	}
}

func (AffiliateProfile) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").
			Immutable(),
		field.String("invite_code").
			Immutable(),
		field.Enum("status").
			Values("active", "disabled").
			Default("active"),
		field.Int("rebate_rate_override_bps").
			Optional().
			Nillable().
			NonNegative().
			Comment("Optional user-specific rebate rate override in basis points."),
		field.String("notes").
			Default(""),
	}
}

func (AffiliateProfile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("affiliate_profiles").
			Field("user_id").
			Required().
			Immutable().
			Unique(),
	}
}

func (AffiliateProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (AffiliateProfile) Policy() ent.Policy {
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
