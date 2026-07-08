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

type BillingAccountBinding struct {
	ent.Schema
}

func (BillingAccountBinding) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (BillingAccountBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_type", "owner_id").
			StorageKey("billing_account_bindings_by_owner").
			Unique(),
	}
}

func (BillingAccountBinding) Fields() []ent.Field {
	return []ent.Field{
		field.Int("billing_account_id").
			Immutable().
			Comment("Bound billing account ID."),
		field.Enum("owner_type").
			Values("project").
			Default("project").
			Immutable().
			Comment("Bound owner type. v1 supports project."),
		field.Int("owner_id").
			Immutable().
			Comment("Bound owner identifier."),
		field.String("relation").
			Default("primary").
			Comment("Binding relation, reserved for shared accounts and reseller models."),
	}
}

func (BillingAccountBinding) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("billing_account", BillingAccount.Type).
			Ref("bindings").
			Field("billing_account_id").
			Required().
			Immutable().
			Unique(),
	}
}

func (BillingAccountBinding) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (BillingAccountBinding) Policy() ent.Policy {
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
