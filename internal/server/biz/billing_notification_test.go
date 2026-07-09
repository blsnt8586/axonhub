package biz

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/billingnotification"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/usersubscription"
)

func TestBillingNotificationDeduplicatesAndPreferenceSuppresses(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:billing_notification_dedupe?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	svc := NewBillingNotificationService(BillingNotificationServiceParams{Ent: client})
	user := createPromoTestUser(t, client, ctx, "notify-dedupe@example.com")

	_, err := svc.CreateNotification(ctx, CreateBillingNotificationInput{
		UserID:   &user.ID,
		Category: billingnotification.CategoryPayment,
		EventKey: "payment:test:1",
		Title:    "Payment completed",
	})
	require.NoError(t, err)
	_, err = svc.CreateNotification(ctx, CreateBillingNotificationInput{
		UserID:   &user.ID,
		Category: billingnotification.CategoryPayment,
		EventKey: "payment:test:1",
		Title:    "Payment completed",
	})
	require.NoError(t, err)

	count, err := client.BillingNotification.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	_, err = svc.SavePreference(ctx, SaveBillingNotificationPreferenceInput{
		UserID:                  user.ID,
		Enabled:                 true,
		LowBalanceEnabled:       true,
		PaymentEnabled:          false,
		SubscriptionEnabled:     true,
		LargeConsumptionEnabled: true,
	})
	require.NoError(t, err)
	notification, err := svc.CreateNotification(ctx, CreateBillingNotificationInput{
		UserID:   &user.ID,
		Category: billingnotification.CategoryPayment,
		EventKey: "payment:test:2",
		Title:    "Payment completed",
	})
	require.NoError(t, err)
	require.Nil(t, notification)

	count, err = client.BillingNotification.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestBillingNotificationLowBalanceThreshold(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:billing_notification_low_balance?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	svc := NewBillingNotificationService(BillingNotificationServiceParams{Ent: client})
	accountSvc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	user := createPromoTestUser(t, client, ctx, "notify-low-balance@example.com")
	account, err := accountSvc.GetOrCreateForSubject(ctx, UserBillingSubject(user.ID))
	require.NoError(t, err)
	_, err = svc.SaveSetting(ctx, SaveBillingNotificationSettingInput{
		Enabled:                       true,
		UserNotificationsEnabled:      true,
		OperatorAlertsEnabled:         true,
		LowBalanceThreshold:           decimal.RequireFromString("20"),
		LargeConsumptionThreshold:     decimal.RequireFromString("100"),
		SubscriptionExpiryWarningDays: 3,
		Currency:                      "CNY",
	})
	require.NoError(t, err)

	err = svc.NotifyLowBalance(ctx, account)
	require.NoError(t, err)
	err = svc.NotifyLowBalance(ctx, account)
	require.NoError(t, err)

	notifications, err := client.BillingNotification.Query().
		Where(billingnotification.CategoryEQ(billingnotification.CategoryLowBalance)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, notifications, 1)
	require.Equal(t, account.ID, *notifications[0].BillingAccountID)
}

func TestBillingNotificationSubscriptionExpiryWarningsAreIdempotent(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:billing_notification_subscription_warning?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	svc := NewBillingNotificationService(BillingNotificationServiceParams{Ent: client})
	user := createPromoTestUser(t, client, ctx, "notify-subscription@example.com")
	now := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	sub, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetStatus(usersubscription.StatusActive).
		SetStartsAt(now.AddDate(0, 0, -27)).
		SetExpiresAt(now.AddDate(0, 0, 2)).
		SetCurrentPeriodStart(now.AddDate(0, 0, -27)).
		SetCurrentPeriodEnd(now.AddDate(0, 0, 2)).
		SetResetAt(now.AddDate(0, 0, 2)).
		SetPeriodDays(30).
		SetIncludedAmountMicros(100_000_000).
		SetUsedAmountMicros(0).
		SetCurrency("CNY").
		SetPayableAmountMicros(30_000_000).
		Save(ctx)
	require.NoError(t, err)

	first, err := svc.CreateSubscriptionExpiryWarnings(ctx, now, 100)
	require.NoError(t, err)
	require.Equal(t, 1, first)
	second, err := svc.CreateSubscriptionExpiryWarnings(ctx, now, 100)
	require.NoError(t, err)
	require.Zero(t, second)

	notifications, err := client.BillingNotification.Query().
		Where(billingnotification.UserSubscriptionIDEQ(sub.ID)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, notifications, 1)
	require.Equal(t, billingnotification.CategorySubscription, notifications[0].Category)
}
