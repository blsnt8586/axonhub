package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

type PromoCode struct {
	ent.Schema
}

func (PromoCode) Mixin() []ent.Mixin {
	return []ent.Mixin{TimeMixin{}}
}

func (PromoCode) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").StorageKey("promo_codes_by_code").Unique(),
		index.Fields("status", "scope").StorageKey("promo_codes_by_status_scope"),
		index.Fields("expires_at").StorageKey("promo_codes_by_expires_at"),
		index.Fields("created_by_id", "created_at").StorageKey("promo_codes_by_creator_created_at"),
	}
}

func (PromoCode) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").Immutable().Comment("Human promo code. Stored normalized as uppercase."),
		field.String("description").Default(""),
		field.Enum("discount_type").Values("amount", "percent").Default("amount"),
		field.Int64("discount_amount_micros").NonNegative().Default(0),
		field.Int("discount_percent_bps").NonNegative().Default(0).Comment("Percentage discount in basis points, 10000 means 100%."),
		field.Enum("scope").Values("all", "recharge", "subscription").Default("all"),
		field.Enum("status").Values("active", "disabled", "expired").Default("active"),
		field.String("currency").Default("CNY"),
		field.Int("max_uses").NonNegative().Default(0).Comment("Zero means unlimited."),
		field.Int("used_count").NonNegative().Default(0),
		field.Int("per_user_limit").NonNegative().Default(0).Comment("Zero means unlimited."),
		field.Time("starts_at").Optional().Nillable(),
		field.Time("expires_at").Optional().Nillable(),
		field.Int("created_by_id").Optional().Nillable().Immutable(),
		field.String("notes").Default(""),
		field.JSON("metadata", objects.JSONRawMessage{}).Optional(),
	}
}

func (PromoCode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("usages", PromoUsage.Type).Annotations(
			entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			entgql.RelayConnection(),
		),
		edge.To("payment_orders", PaymentOrder.Type).Annotations(
			entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			entgql.RelayConnection(),
		),
		edge.To("user_subscriptions", UserSubscription.Type).Annotations(
			entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			entgql.RelayConnection(),
		),
	}
}

func (PromoCode) Annotations() []schema.Annotation {
	return []schema.Annotation{entgql.QueryField(), entgql.RelayConnection()}
}

func (PromoCode) Policy() ent.Policy {
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
