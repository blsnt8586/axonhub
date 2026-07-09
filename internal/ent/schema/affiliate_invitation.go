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

type AffiliateInvitation struct {
	ent.Schema
}

func (AffiliateInvitation) Mixin() []ent.Mixin {
	return []ent.Mixin{TimeMixin{}}
}

func (AffiliateInvitation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("invitee_user_id").
			StorageKey("affiliate_invitations_by_invitee").
			Unique(),
		index.Fields("inviter_user_id", "created_at").
			StorageKey("affiliate_invitations_by_inviter_created_at"),
	}
}

func (AffiliateInvitation) Fields() []ent.Field {
	return []ent.Field{
		field.Int("inviter_user_id").
			Immutable(),
		field.Int("invitee_user_id").
			Immutable(),
		field.String("invite_code").
			Immutable(),
		field.Enum("status").
			Values("active", "canceled").
			Default("active"),
		field.String("notes").
			Default(""),
	}
}

func (AffiliateInvitation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("inviter", User.Type).
			Ref("affiliate_inviters").
			Field("inviter_user_id").
			Required().
			Immutable().
			Unique(),
		edge.From("invitee", User.Type).
			Ref("affiliate_invitees").
			Field("invitee_user_id").
			Required().
			Immutable().
			Unique(),
		edge.To("rebates", AffiliateRebate.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (AffiliateInvitation) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (AffiliateInvitation) Policy() ent.Policy {
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
