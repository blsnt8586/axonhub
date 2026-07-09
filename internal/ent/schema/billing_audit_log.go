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

type BillingAuditLog struct {
	ent.Schema
}

func (BillingAuditLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (BillingAuditLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("action").
			StorageKey("billing_audit_logs_by_action"),
		index.Fields("actor_user_id").
			StorageKey("billing_audit_logs_by_actor_user_id"),
		index.Fields("target_type", "target_id").
			StorageKey("billing_audit_logs_by_target"),
	}
}

func (BillingAuditLog) Fields() []ent.Field {
	return []ent.Field{
		field.String("action").
			NotEmpty().
			Comment("Stable commercial action key."),
		field.Enum("actor_type").
			Values("admin", "system").
			Default("admin"),
		field.Int("actor_user_id").
			Optional().
			Nillable(),
		field.String("target_type").
			Default(""),
		field.String("target_id").
			Default(""),
		field.Int("target_user_id").
			Optional().
			Nillable(),
		field.String("reason").
			Default(""),
		field.JSON("metadata", objects.JSONRawMessage{}).
			Optional(),
	}
}

func (BillingAuditLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (BillingAuditLog) Policy() ent.Policy {
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
