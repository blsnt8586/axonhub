package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/scopes"
)

type AffiliateSetting struct {
	ent.Schema
}

func (AffiliateSetting) Mixin() []ent.Mixin {
	return []ent.Mixin{TimeMixin{}}
}

func (AffiliateSetting) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("key").
			StorageKey("affiliate_settings_by_key").
			Unique(),
	}
}

func (AffiliateSetting) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").
			Default("default").
			Immutable().
			Comment("Singleton setting key."),
		field.Bool("enabled").
			Default(true),
		field.Int("default_rebate_rate_bps").
			NonNegative().
			Default(500).
			Comment("Default rebate rate in basis points. 500 means 5%."),
		field.Int("freeze_days").
			NonNegative().
			Default(7).
			Comment("Days before a rebate can be transferred."),
		field.Int64("min_transfer_micros").
			NonNegative().
			Default(0),
		field.String("currency").
			Default("CNY"),
	}
}

func (AffiliateSetting) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (AffiliateSetting) Policy() ent.Policy {
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
