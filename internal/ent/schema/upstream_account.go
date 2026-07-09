package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

type UpstreamAccount struct {
	ent.Schema
}

func (UpstreamAccount) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (UpstreamAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_id", "status").
			StorageKey("upstream_accounts_by_channel_status"),
		index.Fields("channel_id", "pool_id", "status").
			StorageKey("upstream_accounts_by_channel_pool_status"),
		index.Fields("channel_id", "priority", "weight").
			StorageKey("upstream_accounts_by_channel_priority_weight"),
	}
}

func (UpstreamAccount) Fields() []ent.Field {
	return []ent.Field{
		field.Int("channel_id").Immutable(),
		field.Int("pool_id").Optional().Nillable(),
		field.String("name").NotEmpty(),
		field.Enum("credential_type").Values("api_key", "oauth", "custom").Default("api_key"),
		field.JSON("credentials", objects.UpstreamAccountCredentials{}).
			Sensitive().
			Annotations(entgql.Skip(entgql.SkipAll)),
		field.Enum("status").Values("active", "disabled", "archived", "error").Default("active"),
		field.Bool("schedulable").Default(true),
		field.Int("priority").Default(0),
		field.Int("weight").Default(100),
		field.Int("concurrency_limit").Default(0).Comment("Zero means unlimited for Stage 15 configuration."),
		field.JSON("proxy_config", &objects.ProxyConfig{}).
			Optional().
			Annotations(entgql.Skip(entgql.SkipAll)),
		field.Float("rate_multiplier").Default(1),
		field.Time("expires_at").Optional().Nillable(),
		field.Time("last_used_at").Optional().Nillable(),
		field.String("error_message").Optional().Nillable(),
		field.Time("rate_limit_reset_at").Optional().Nillable(),
		field.Time("overload_until").Optional().Nillable(),
		field.Time("cooldown_until").Optional().Nillable(),
		field.String("cooldown_reason").Optional().Nillable(),
		field.Int64("quota_limit_micros").Default(0),
		field.Int64("quota_used_micros").Default(0),
	}
}

func (UpstreamAccount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channel", Channel.Type).
			Ref("upstream_accounts").
			Field("channel_id").
			Required().
			Immutable().
			Unique(),
		edge.From("pool", UpstreamAccountPool.Type).
			Ref("accounts").
			Field("pool_id").
			Unique(),
	}
}

func (UpstreamAccount) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.RelayConnection(),
	}
}

func (UpstreamAccount) Policy() ent.Policy {
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
