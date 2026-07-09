package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/scopes"
)

type UsageHourlyAggregate struct {
	ent.Schema
}

func (UsageHourlyAggregate) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (UsageHourlyAggregate) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("bucket_start").
			StorageKey("usage_hourly_aggregates_by_bucket_start"),
		index.Fields("user_id", "bucket_start").
			StorageKey("usage_hourly_aggregates_by_user_bucket"),
		index.Fields("project_id", "bucket_start").
			StorageKey("usage_hourly_aggregates_by_project_bucket"),
		index.Fields("api_key_id", "bucket_start").
			StorageKey("usage_hourly_aggregates_by_api_key_bucket"),
		index.Fields("channel_id", "bucket_start").
			StorageKey("usage_hourly_aggregates_by_channel_bucket"),
		index.Fields("model_id", "bucket_start").
			StorageKey("usage_hourly_aggregates_by_model_bucket"),
		index.Fields("bucket_start", "user_id", "api_key_id", "project_id", "channel_id", "model_id", "request_type", "status", "currency", "upstream_account_id").
			StorageKey("usage_hourly_aggregates_unique_dimension").
			Unique(),
	}
}

func (UsageHourlyAggregate) Fields() []ent.Field {
	return []ent.Field{
		field.Time("bucket_start").
			Immutable(),
		field.Int("user_id").
			Default(0),
		field.Int("api_key_id").
			Default(0),
		field.Int("project_id").
			Default(0),
		field.Int("channel_id").
			Default(0),
		field.Int("upstream_account_id").
			Default(0).
			Comment("Reserved for Stage 15+ upstream account pool aggregation. Zero means not assigned."),
		field.String("model_id").
			Default(""),
		field.Enum("request_type").
			Values("chat", "image", "video", "embedding", "audio", "other").
			Default("chat"),
		field.Enum("status").
			Values("pending", "charged", "skipped", "failed", "refunded").
			Default("charged"),
		field.String("currency").
			Default("CNY"),
		field.Int64("request_count").
			Default(0),
		field.Int64("success_count").
			Default(0),
		field.Int64("error_count").
			Default(0),
		field.Int64("prompt_tokens").
			Default(0),
		field.Int64("completion_tokens").
			Default(0),
		field.Int64("total_tokens").
			Default(0),
		field.Int64("user_charge_micros").
			Default(0),
		field.Int64("upstream_cost_micros").
			Default(0),
		field.Int64("gross_margin_micros").
			Default(0),
	}
}

func (UsageHourlyAggregate) Policy() ent.Policy {
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
