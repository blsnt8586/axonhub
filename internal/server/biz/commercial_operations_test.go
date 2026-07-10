package biz

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/affiliaterebate"
	"github.com/looplj/axonhub/internal/ent/commercialsetting"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/paymentproviderinstance"
	"github.com/looplj/axonhub/internal/objects"
)

func TestCommercialOperationsEnsureDefaultsEncryptsLegacyEPaySecretsIdempotently(t *testing.T) {
	t.Setenv("AXONHUB_PAYMENT_SECRET_KEY", "commercial-operations-legacy-migration-key")

	client, ctx, svc := newCommercialOperationsTestService(t, "commercial_legacy_epay_encryption")
	defer client.Close()

	legacySecret := "stage25-legacy-plaintext-epay-key"
	legacyConfig, err := json.Marshal(EPayConfig{
		GatewayURL: "https://legacy-pay.example.com/submit.php",
		PID:        "stage25-pid",
		Key:        legacySecret,
		NotifyURL:  "https://axon.example.com/payment/notify/epay",
		ReturnURL:  "https://axon.example.com/billing",
		Type:       "alipay",
		SiteName:   "AxonHub Legacy",
	})
	require.NoError(t, err)

	provider, err := client.PaymentProviderInstance.Create().
		SetName("Stage 25 Legacy ePay").
		SetProviderType(paymentproviderinstance.ProviderTypeEpay).
		SetStatus(paymentproviderinstance.StatusEnabled).
		SetCurrency("CNY").
		SetConfig(objects.JSONRawMessage(legacyConfig)).
		Save(ctx)
	require.NoError(t, err)
	require.Contains(t, string(provider.Config), legacySecret)

	require.NoError(t, svc.EnsureDefaults(ctx))

	migrated, err := client.PaymentProviderInstance.Get(ctx, provider.ID)
	require.NoError(t, err)
	require.NotContains(t, string(migrated.Config), legacySecret)

	var stored EPayConfig
	require.NoError(t, json.Unmarshal(migrated.Config, &stored))
	require.True(t, strings.HasPrefix(stored.Key, encryptedPaymentSecretPrefix))

	parsed, err := ParseEPayConfig(migrated.Config)
	require.NoError(t, err)
	require.Equal(t, legacySecret, parsed.Key)

	firstEncryptedConfig := string(migrated.Config)
	require.NoError(t, svc.EnsureDefaults(ctx))

	migratedAgain, err := client.PaymentProviderInstance.Get(ctx, provider.ID)
	require.NoError(t, err)
	require.Equal(t, firstEncryptedConfig, string(migratedAgain.Config))
}

func TestCommercialOperationsEnsureDefaultsRollsBackLegacyProviderMigrationOnInvalidConfig(t *testing.T) {
	t.Setenv("AXONHUB_PAYMENT_SECRET_KEY", "commercial-operations-legacy-rollback-key")

	client, ctx, svc := newCommercialOperationsTestService(t, "commercial_legacy_epay_rollback")
	defer client.Close()

	legacySecret := "stage25-rollback-plaintext-epay-key"
	validConfig, err := json.Marshal(EPayConfig{
		GatewayURL: "https://legacy-pay.example.com/submit.php",
		PID:        "stage25-valid-pid",
		Key:        legacySecret,
		NotifyURL:  "https://axon.example.com/payment/notify/epay",
		ReturnURL:  "https://axon.example.com/billing",
	})
	require.NoError(t, err)

	validProvider, err := client.PaymentProviderInstance.Create().
		SetName("Stage 25 Valid Legacy ePay").
		SetProviderType(paymentproviderinstance.ProviderTypeEpay).
		SetConfig(objects.JSONRawMessage(validConfig)).
		Save(ctx)
	require.NoError(t, err)

	invalidConfig, err := json.Marshal(EPayConfig{
		GatewayURL: "https://legacy-pay.example.com/submit.php",
		PID:        "stage25-invalid-pid",
		NotifyURL:  "https://axon.example.com/payment/notify/epay",
		ReturnURL:  "https://axon.example.com/billing",
	})
	require.NoError(t, err)
	_, err = client.PaymentProviderInstance.Create().
		SetName("Stage 25 Invalid Legacy ePay").
		SetProviderType(paymentproviderinstance.ProviderTypeEpay).
		SetConfig(objects.JSONRawMessage(invalidConfig)).
		Save(ctx)
	require.NoError(t, err)

	err = svc.EnsureDefaults(ctx)
	require.ErrorContains(t, err, "epay key is required")

	reloaded, err := client.PaymentProviderInstance.Get(ctx, validProvider.ID)
	require.NoError(t, err)
	require.Contains(t, string(reloaded.Config), legacySecret)
	require.NotContains(t, string(reloaded.Config), encryptedPaymentSecretPrefix)
}

func TestCommercialOperationsGetOrCreateSettingIsIdempotent(t *testing.T) {
	t.Parallel()

	client, ctx, svc := newCommercialOperationsTestService(t, "commercial_setting_idempotent")
	defer client.Close()

	first, err := svc.GetOrCreateSetting(ctx)
	require.NoError(t, err)
	require.Equal(t, commercialsetting.ModeEnforce, first.Mode)
	require.True(t, first.WorkersEnabled)

	second, err := svc.GetOrCreateSetting(ctx)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)

	count, err := client.CommercialSetting.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestCommercialOperationsRunMaintenanceNoopsWhenWorkersDisabled(t *testing.T) {
	t.Parallel()

	client, ctx, svc := newCommercialOperationsTestService(t, "commercial_maintenance_disabled")
	defer client.Close()

	_, err := svc.SaveSetting(ctx, SaveCommercialSettingInput{
		Mode:                            commercialsetting.ModeEnforce,
		RequireAdminActionReason:        true,
		PaymentProviderSecretsEncrypted: true,
		WorkersEnabled:                  false,
		WorkerBatchSize:                 25,
		Currency:                        "CNY",
		Reason:                          "disable workers for maintenance test",
	})
	require.NoError(t, err)

	result, err := svc.RunMaintenance(ctx, CommercialMaintenanceRunInput{
		Now:    time.Now().UTC(),
		Limit:  10,
		Reason: "manual disabled maintenance run",
	})
	require.NoError(t, err)
	require.Zero(t, result.OrderExpiryProcessed)
	require.Zero(t, result.HoldExpiryProcessed)
	require.Zero(t, result.SubscriptionExpiryProcessed)
	require.Zero(t, result.SubscriptionResetProcessed)
	require.Zero(t, result.AffiliateRebateThawProcessed)
	require.Zero(t, result.FailedBillingRetryProcessed)
}

func TestAffiliateThawDueRebatesIsIdempotent(t *testing.T) {
	t.Parallel()

	client, ctx, svc := newAffiliateServiceWithContext(t, "affiliate_thaw_due_idempotent")
	defer client.Close()

	inviter := createPromoTestUser(t, client, ctx, "affiliate-thaw-inviter@example.com")
	invitee := createPromoTestUser(t, client, ctx, "affiliate-thaw-invitee@example.com")
	profile, err := svc.GetOrCreateProfile(ctx, inviter.ID)
	require.NoError(t, err)
	invitation, err := svc.BindInvite(ctx, BindAffiliateInviteInput{InviteeUserID: invitee.ID, InviteCode: profile.InviteCode})
	require.NoError(t, err)

	now := time.Now().UTC()
	rebate, err := client.AffiliateRebate.Create().
		SetInvitationID(invitation.ID).
		SetInviterUserID(inviter.ID).
		SetInviteeUserID(invitee.ID).
		SetSourceType(affiliaterebate.SourceTypePaymentOrder).
		SetSourceID(1001).
		SetBaseAmountMicros(10_000_000).
		SetAmountMicros(1_000_000).
		SetRateBps(1000).
		SetCurrency("CNY").
		SetStatus(affiliaterebate.StatusFrozen).
		SetFreezeUntil(now.Add(-time.Minute)).
		SetIdempotencyKey("affiliate-thaw-due-once").
		Save(ctx)
	require.NoError(t, err)

	thawed, err := svc.ThawDueRebates(ctx, now, 10)
	require.NoError(t, err)
	require.Equal(t, 1, thawed)
	loaded, err := client.AffiliateRebate.Get(ctx, rebate.ID)
	require.NoError(t, err)
	require.Equal(t, affiliaterebate.StatusAvailable, loaded.Status)

	thawed, err = svc.ThawDueRebates(ctx, now.Add(time.Minute), 10)
	require.NoError(t, err)
	require.Zero(t, thawed)
}

func newCommercialOperationsTestService(t *testing.T, name string) (*ent.Client, context.Context, *CommercialOperationsService) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	auditSvc := NewBillingAuditService(BillingAuditServiceParams{Ent: client})
	return client, ctx, NewCommercialOperationsService(CommercialOperationsServiceParams{
		Ent:                 client,
		BillingAuditService: auditSvc,
	})
}
