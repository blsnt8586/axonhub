package conf

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/biz"
)

func TestLoadDefaultsBillingSubjectToUserWallet(t *testing.T) {
	t.Setenv("AXONHUB_BILLING_SUBJECT", "")
	t.Chdir(t.TempDir())

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, biz.BillingSubjectTypeUser, cfg.Billing.Subject)
}
