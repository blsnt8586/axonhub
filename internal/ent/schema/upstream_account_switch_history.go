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

type UpstreamAccountSwitchHistory struct {
	ent.Schema
}

func (UpstreamAccountSwitchHistory) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (UpstreamAccountSwitchHistory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("request_id", "created_at").
			StorageKey("upstream_account_switch_histories_by_request_created_at"),
		index.Fields("channel_id", "created_at").
			StorageKey("upstream_account_switch_histories_by_channel_created_at"),
		index.Fields("to_account_id", "created_at").
			StorageKey("upstream_account_switch_histories_by_to_account_created_at"),
		index.Fields("from_account_id", "created_at").
			StorageKey("upstream_account_switch_histories_by_from_account_created_at"),
	}
}

func (UpstreamAccountSwitchHistory) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Default(0).
			Immutable(),
		field.Int("request_id").
			Optional().
			Nillable().
			Immutable().
			Comment("Request that triggered this account selection or switch."),
		field.Int("request_execution_id").
			Optional().
			Nillable().
			Immutable().
			Comment("Previous failed execution when this row records retry switching."),
		field.Int("channel_id").
			Immutable(),
		field.Int("from_account_id").
			Optional().
			Nillable().
			Immutable(),
		field.Int("to_account_id").
			Optional().
			Nillable().
			Immutable(),
		field.String("model_id").
			Default("").
			Immutable(),
		field.String("reason").
			Default("").
			Immutable(),
		field.Int("error_code").
			Optional().
			Nillable().
			Immutable(),
		field.String("error_message").
			Default("").
			Immutable(),
		field.Int64("latency_ms").
			Optional().
			Nillable().
			Immutable(),
	}
}

func (UpstreamAccountSwitchHistory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("request", Request.Type).
			Ref("upstream_account_switch_histories").
			Field("request_id").
			Immutable().
			Unique(),
		edge.From("request_execution", RequestExecution.Type).
			Ref("upstream_account_switch_histories").
			Field("request_execution_id").
			Immutable().
			Unique(),
		edge.From("channel", Channel.Type).
			Ref("upstream_account_switch_histories").
			Field("channel_id").
			Required().
			Immutable().
			Unique(),
		edge.From("from_account", UpstreamAccount.Type).
			Ref("switch_histories_from").
			Field("from_account_id").
			Immutable().
			Unique(),
		edge.From("to_account", UpstreamAccount.Type).
			Ref("switch_histories_to").
			Field("to_account_id").
			Immutable().
			Unique(),
	}
}

func (UpstreamAccountSwitchHistory) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (UpstreamAccountSwitchHistory) Policy() ent.Policy {
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
