package scopes

import (
	"context"

	"entgo.io/ent/entql"

	"github.com/looplj/axonhub/internal/ent/privacy"
)

type enabledSubscriptionPlanFilter interface {
	WhereStatus(entql.StringP)
}

// EnabledSubscriptionPlanQueryRule lets authenticated users read sellable plans
// while still filtering non-owner plan queries down to enabled plans.
func EnabledSubscriptionPlanQueryRule() privacy.QueryRule {
	return privacy.FilterFunc(func(ctx context.Context, q privacy.Filter) error {
		user, err := getUserFromContext(ctx)
		if err != nil {
			return err
		}
		if user.IsOwner {
			return privacy.Skipf("owner handled by owner rule")
		}

		switch q := q.(type) {
		case enabledSubscriptionPlanFilter:
			q.WhereStatus(entql.StringEQ("enabled"))
			return privacy.Allowf("User %d can query enabled subscription plans", user.ID)
		default:
			return privacy.Skipf("subscription plan filter is not supported for user %d", user.ID)
		}
	})
}
