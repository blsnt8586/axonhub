package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/scopes"
)

type CommercialSetting struct {
	ent.Schema
}

func (CommercialSetting) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (CommercialSetting) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("key").
			StorageKey("commercial_settings_by_key").
			Unique(),
	}
}

func (CommercialSetting) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").
			Default("default").
			Immutable().
			Comment("Singleton commercial setting key."),
		field.Enum("mode").
			Values("disabled", "warn", "enforce").
			Default("enforce").
			Comment("Commercial billing mode exposed to administrators."),
		field.Bool("require_admin_action_reason").
			Default(true).
			Comment("Require explicit reasons for dangerous commercial admin actions."),
		field.Bool("payment_provider_secrets_encrypted").
			Default(true).
			Comment("Whether newly saved payment provider secrets are encrypted at rest."),
		field.Bool("workers_enabled").
			Default(true).
			Comment("Master switch for commercial maintenance workers."),
		field.Bool("order_expiry_worker_enabled").
			Default(true),
		field.Bool("hold_expiry_worker_enabled").
			Default(true),
		field.Bool("subscription_expiry_worker_enabled").
			Default(true),
		field.Bool("subscription_reset_worker_enabled").
			Default(true),
		field.Bool("affiliate_rebate_thaw_worker_enabled").
			Default(true),
		field.Bool("failed_billing_retry_worker_enabled").
			Default(true),
		field.Int("worker_batch_size").
			Positive().
			Default(100),
		field.String("currency").
			Default("CNY"),
	}
}

func (CommercialSetting) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (CommercialSetting) Policy() ent.Policy {
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
