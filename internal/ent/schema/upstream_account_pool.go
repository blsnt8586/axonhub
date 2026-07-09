package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/scopes"
)

type UpstreamAccountPool struct {
	ent.Schema
}

func (UpstreamAccountPool) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (UpstreamAccountPool) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_id", "name", "deleted_at").
			StorageKey("upstream_account_pools_by_channel_name").
			Unique(),
		index.Fields("channel_id", "status").
			StorageKey("upstream_account_pools_by_channel_status"),
	}
}

func (UpstreamAccountPool) Fields() []ent.Field {
	return []ent.Field{
		field.Int("channel_id").Immutable(),
		field.String("name").NotEmpty(),
		field.Enum("status").Values("enabled", "disabled", "archived").Default("enabled"),
		field.Int("priority").Default(0),
		field.Strings("model_patterns").Optional().Default([]string{}),
		field.Ints("project_ids").Optional().Default([]int{}),
		field.String("remark").Optional().Nillable(),
	}
}

func (UpstreamAccountPool) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channel", Channel.Type).
			Ref("upstream_account_pools").
			Field("channel_id").
			Required().
			Immutable().
			Unique(),
		edge.To("accounts", UpstreamAccount.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
	}
}

func (UpstreamAccountPool) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.RelayConnection(),
	}
}

func (UpstreamAccountPool) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadChannels),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteChannels),
		},
	}
}
