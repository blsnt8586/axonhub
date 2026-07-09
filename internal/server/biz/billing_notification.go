package biz

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/billingnotification"
	"github.com/looplj/axonhub/internal/ent/billingnotificationpreference"
	"github.com/looplj/axonhub/internal/ent/billingnotificationsetting"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/ent/usersubscription"
	"github.com/looplj/axonhub/internal/objects"
)

const defaultBillingNotificationSettingKey = "default"

type BillingNotificationServiceParams struct {
	fx.In

	Ent *ent.Client
}

type BillingNotificationService struct {
	*AbstractService
}

func NewBillingNotificationService(params BillingNotificationServiceParams) *BillingNotificationService {
	return &BillingNotificationService{
		AbstractService: &AbstractService{db: params.Ent},
	}
}

type SaveBillingNotificationSettingInput struct {
	Enabled                       bool
	UserNotificationsEnabled      bool
	OperatorAlertsEnabled         bool
	LowBalanceThreshold           decimal.Decimal
	LargeConsumptionThreshold     decimal.Decimal
	SubscriptionExpiryWarningDays int
	Currency                      string
}

type SaveBillingNotificationPreferenceInput struct {
	UserID                  int
	Enabled                 bool
	LowBalanceEnabled       bool
	PaymentEnabled          bool
	SubscriptionEnabled     bool
	LargeConsumptionEnabled bool
}

type CreateBillingNotificationInput struct {
	UserID               *int
	Audience             billingnotification.Audience
	Category             billingnotification.Category
	Severity             billingnotification.Severity
	EventKey             string
	Title                string
	Message              string
	Currency             string
	AmountMicros         *int64
	BillingAccountID     *int
	PaymentOrderID       *int
	UserSubscriptionID   *int
	UsageBillingRecordID *int
	Metadata             objects.JSONRawMessage
}

func (s *BillingNotificationService) SettingOrDefault(ctx context.Context) (*ent.BillingNotificationSetting, error) {
	setting, err := s.entFromContext(ctx).BillingNotificationSetting.Query().
		Where(billingnotificationsetting.KeyEQ(defaultBillingNotificationSettingKey)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return s.entFromContext(ctx).BillingNotificationSetting.Create().
			SetKey(defaultBillingNotificationSettingKey).
			Save(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load billing notification setting: %w", err)
	}
	return setting, nil
}

func (s *BillingNotificationService) SaveSetting(ctx context.Context, input SaveBillingNotificationSettingInput) (*ent.BillingNotificationSetting, error) {
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}
	if input.SubscriptionExpiryWarningDays < 0 {
		return nil, fmt.Errorf("subscription expiry warning days cannot be negative")
	}
	lowBalanceMicros, err := decimalToMicros(input.LowBalanceThreshold)
	if err != nil {
		return nil, err
	}
	largeConsumptionMicros, err := decimalToMicros(input.LargeConsumptionThreshold)
	if err != nil {
		return nil, err
	}
	if lowBalanceMicros < 0 || largeConsumptionMicros < 0 {
		return nil, fmt.Errorf("notification thresholds cannot be negative")
	}

	existing, err := s.SettingOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	return s.entFromContext(ctx).BillingNotificationSetting.UpdateOneID(existing.ID).
		SetEnabled(input.Enabled).
		SetUserNotificationsEnabled(input.UserNotificationsEnabled).
		SetOperatorAlertsEnabled(input.OperatorAlertsEnabled).
		SetLowBalanceThresholdMicros(lowBalanceMicros).
		SetLargeConsumptionThresholdMicros(largeConsumptionMicros).
		SetSubscriptionExpiryWarningDays(input.SubscriptionExpiryWarningDays).
		SetCurrency(input.Currency).
		Save(ctx)
}

func (s *BillingNotificationService) PreferenceOrDefault(ctx context.Context, userID int) (*ent.BillingNotificationPreference, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("user id is required")
	}
	preference, err := s.entFromContext(ctx).BillingNotificationPreference.Query().
		Where(billingnotificationpreference.UserIDEQ(userID)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return s.entFromContext(ctx).BillingNotificationPreference.Create().
			SetUserID(userID).
			Save(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load billing notification preference: %w", err)
	}
	return preference, nil
}

func (s *BillingNotificationService) SavePreference(ctx context.Context, input SaveBillingNotificationPreferenceInput) (*ent.BillingNotificationPreference, error) {
	preference, err := s.PreferenceOrDefault(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	return s.entFromContext(ctx).BillingNotificationPreference.UpdateOneID(preference.ID).
		SetEnabled(input.Enabled).
		SetLowBalanceEnabled(input.LowBalanceEnabled).
		SetPaymentEnabled(input.PaymentEnabled).
		SetSubscriptionEnabled(input.SubscriptionEnabled).
		SetLargeConsumptionEnabled(input.LargeConsumptionEnabled).
		Save(ctx)
}

func (s *BillingNotificationService) CreateNotification(ctx context.Context, input CreateBillingNotificationInput) (*ent.BillingNotification, error) {
	input.EventKey = strings.TrimSpace(input.EventKey)
	input.Title = strings.TrimSpace(input.Title)
	if input.EventKey == "" {
		return nil, fmt.Errorf("billing notification event key is required")
	}
	if input.Title == "" {
		return nil, fmt.Errorf("billing notification title is required")
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}
	if input.Audience == "" {
		input.Audience = billingnotification.AudienceUser
	}
	if input.Severity == "" {
		input.Severity = billingnotification.SeverityInfo
	}

	setting, err := s.SettingOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	if !setting.Enabled {
		return nil, nil
	}
	if input.Audience == billingnotification.AudienceUser {
		if !setting.UserNotificationsEnabled {
			return nil, nil
		}
		if input.UserID == nil || *input.UserID <= 0 {
			return nil, fmt.Errorf("user notification requires user id")
		}
		allowed, err := s.userAllowsCategory(ctx, *input.UserID, input.Category)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, nil
		}
	}
	if input.Audience == billingnotification.AudienceOperator && !setting.OperatorAlertsEnabled {
		return nil, nil
	}

	create := s.entFromContext(ctx).BillingNotification.Create().
		SetAudience(input.Audience).
		SetCategory(input.Category).
		SetSeverity(input.Severity).
		SetEventKey(input.EventKey).
		SetTitle(input.Title).
		SetMessage(strings.TrimSpace(input.Message)).
		SetCurrency(input.Currency).
		SetNillableUserID(input.UserID).
		SetNillableAmountMicros(input.AmountMicros).
		SetNillableBillingAccountID(input.BillingAccountID).
		SetNillablePaymentOrderID(input.PaymentOrderID).
		SetNillableUserSubscriptionID(input.UserSubscriptionID).
		SetNillableUsageBillingRecordID(input.UsageBillingRecordID)
	if len(input.Metadata) > 0 {
		create.SetMetadata(input.Metadata)
	}
	notification, err := create.Save(ctx)
	if ent.IsConstraintError(err) {
		return s.entFromContext(ctx).BillingNotification.Query().
			Where(billingnotification.EventKeyEQ(input.EventKey)).
			Only(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create billing notification: %w", err)
	}
	return notification, nil
}

func (s *BillingNotificationService) MarkRead(ctx context.Context, userID int, id int) (*ent.BillingNotification, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("user id is required")
	}
	if id <= 0 {
		return nil, fmt.Errorf("notification id is required")
	}
	now := time.Now().UTC()
	count, err := s.entFromContext(ctx).BillingNotification.Update().
		Where(
			billingnotification.IDEQ(id),
			billingnotification.UserIDEQ(userID),
		).
		SetStatus(billingnotification.StatusRead).
		SetReadAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, fmt.Errorf("billing notification not found")
	}
	return s.entFromContext(ctx).BillingNotification.Get(ctx, id)
}

func (s *BillingNotificationService) MarkAllRead(ctx context.Context, userID int) (int, error) {
	if userID <= 0 {
		return 0, fmt.Errorf("user id is required")
	}
	now := time.Now().UTC()
	return s.entFromContext(ctx).BillingNotification.Update().
		Where(
			billingnotification.UserIDEQ(userID),
			billingnotification.StatusEQ(billingnotification.StatusUnread),
		).
		SetStatus(billingnotification.StatusRead).
		SetReadAt(now).
		Save(ctx)
}

func (s *BillingNotificationService) NotifyPaymentSuccess(ctx context.Context, order *ent.PaymentOrder) error {
	userID := s.userIDFromBillingAccount(ctx, order.BillingAccountID)
	if userID == nil {
		return nil
	}
	amount := paymentOrderPayableAmountMicros(order)
	_, err := s.CreateNotification(ctx, CreateBillingNotificationInput{
		UserID:           userID,
		Audience:         billingnotification.AudienceUser,
		Category:         billingnotification.CategoryPayment,
		Severity:         billingnotification.SeverityInfo,
		EventKey:         fmt.Sprintf("payment_success:%d", order.ID),
		Title:            "Payment completed",
		Message:          fmt.Sprintf("Recharge order %s was paid successfully.", order.OrderNo),
		Currency:         order.Currency,
		AmountMicros:     &amount,
		BillingAccountID: &order.BillingAccountID,
		PaymentOrderID:   &order.ID,
	})
	return err
}

func (s *BillingNotificationService) NotifyPaymentFailure(ctx context.Context, order *ent.PaymentOrder, reason string) error {
	if order == nil {
		return nil
	}
	userID := s.userIDFromBillingAccount(ctx, order.BillingAccountID)
	if userID == nil {
		return nil
	}
	amount := paymentOrderPayableAmountMicros(order)
	_, err := s.CreateNotification(ctx, CreateBillingNotificationInput{
		UserID:           userID,
		Audience:         billingnotification.AudienceUser,
		Category:         billingnotification.CategoryPayment,
		Severity:         billingnotification.SeverityError,
		EventKey:         fmt.Sprintf("payment_failure:%d:%s", order.ID, normalizeEventKeyPart(reason)),
		Title:            "Payment failed",
		Message:          fmt.Sprintf("Recharge order %s failed: %s", order.OrderNo, reason),
		Currency:         order.Currency,
		AmountMicros:     &amount,
		BillingAccountID: &order.BillingAccountID,
		PaymentOrderID:   &order.ID,
	})
	return err
}

func (s *BillingNotificationService) NotifyLowBalance(ctx context.Context, account *ent.BillingAccount) error {
	if account == nil || account.OwnerType != billingaccount.OwnerTypeUser {
		return nil
	}
	setting, err := s.SettingOrDefault(ctx)
	if err != nil {
		return err
	}
	if setting.LowBalanceThresholdMicros <= 0 || account.BalanceMicros > setting.LowBalanceThresholdMicros {
		return nil
	}
	userID := account.OwnerID
	amount := account.BalanceMicros
	_, err = s.CreateNotification(ctx, CreateBillingNotificationInput{
		UserID:           &userID,
		Audience:         billingnotification.AudienceUser,
		Category:         billingnotification.CategoryLowBalance,
		Severity:         billingnotification.SeverityWarning,
		EventKey:         fmt.Sprintf("low_balance:%d:%d", account.ID, setting.LowBalanceThresholdMicros),
		Title:            "Low wallet balance",
		Message:          "Wallet balance is below the configured warning threshold.",
		Currency:         account.Currency,
		AmountMicros:     &amount,
		BillingAccountID: &account.ID,
	})
	return err
}

func (s *BillingNotificationService) NotifyUsageCharged(ctx context.Context, record *ent.UsageBillingRecord) error {
	if record == nil || record.Status != usagebillingrecord.StatusCharged || record.UserID <= 0 {
		return nil
	}
	userID := record.UserID
	var firstErr error
	setting, err := s.SettingOrDefault(ctx)
	if err != nil {
		return err
	}
	if setting.LargeConsumptionThresholdMicros > 0 && record.ChargeAmountMicros >= setting.LargeConsumptionThresholdMicros {
		amount := record.ChargeAmountMicros
		if _, err := s.CreateNotification(ctx, CreateBillingNotificationInput{
			UserID:               &userID,
			Audience:             billingnotification.AudienceUser,
			Category:             billingnotification.CategoryLargeConsumption,
			Severity:             billingnotification.SeverityWarning,
			EventKey:             fmt.Sprintf("large_consumption:%d", record.ID),
			Title:                "Large usage charge",
			Message:              fmt.Sprintf("Usage record %d charged above the configured threshold.", record.ID),
			Currency:             record.Currency,
			AmountMicros:         &amount,
			BillingAccountID:     &record.BillingAccountID,
			UsageBillingRecordID: &record.ID,
		}); err != nil {
			firstErr = err
		}
	}
	account, err := s.entFromContext(ctx).BillingAccount.Get(ctx, record.BillingAccountID)
	if err == nil {
		if err := s.NotifyLowBalance(ctx, account); err != nil && firstErr == nil {
			firstErr = err
		}
	} else if firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (s *BillingNotificationService) NotifySubscriptionExpiring(ctx context.Context, sub *ent.UserSubscription, now time.Time) error {
	if sub == nil || sub.Status != usersubscription.StatusActive {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	setting, err := s.SettingOrDefault(ctx)
	if err != nil {
		return err
	}
	if setting.SubscriptionExpiryWarningDays <= 0 {
		return nil
	}
	warnAfter := now.AddDate(0, 0, setting.SubscriptionExpiryWarningDays)
	if sub.ExpiresAt.After(warnAfter) || !sub.ExpiresAt.After(now) {
		return nil
	}
	amount := sub.PayableAmountMicros
	_, err = s.CreateNotification(ctx, CreateBillingNotificationInput{
		UserID:             &sub.UserID,
		Audience:           billingnotification.AudienceUser,
		Category:           billingnotification.CategorySubscription,
		Severity:           billingnotification.SeverityWarning,
		EventKey:           fmt.Sprintf("subscription_expiring:%d:%s", sub.ID, sub.ExpiresAt.Format("2006-01-02")),
		Title:              "Subscription expiring soon",
		Message:            "An active subscription is close to expiration.",
		Currency:           sub.Currency,
		AmountMicros:       &amount,
		UserSubscriptionID: &sub.ID,
	})
	return err
}

func (s *BillingNotificationService) NotifySubscriptionExpired(ctx context.Context, sub *ent.UserSubscription) error {
	if sub == nil {
		return nil
	}
	amount := sub.PayableAmountMicros
	_, err := s.CreateNotification(ctx, CreateBillingNotificationInput{
		UserID:             &sub.UserID,
		Audience:           billingnotification.AudienceUser,
		Category:           billingnotification.CategorySubscription,
		Severity:           billingnotification.SeverityInfo,
		EventKey:           fmt.Sprintf("subscription_expired:%d", sub.ID),
		Title:              "Subscription expired",
		Message:            "A subscription has expired.",
		Currency:           sub.Currency,
		AmountMicros:       &amount,
		UserSubscriptionID: &sub.ID,
	})
	return err
}

func (s *BillingNotificationService) NotifyOperatorAlert(ctx context.Context, eventKey string, title string, message string, severity billingnotification.Severity) error {
	if severity == "" {
		severity = billingnotification.SeverityError
	}
	_, err := s.CreateNotification(ctx, CreateBillingNotificationInput{
		Audience: billingnotification.AudienceOperator,
		Category: billingnotification.CategoryOperatorAlert,
		Severity: severity,
		EventKey: eventKey,
		Title:    title,
		Message:  message,
	})
	return err
}

func (s *BillingNotificationService) CreateSubscriptionExpiryWarnings(ctx context.Context, now time.Time, limit int) (int, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 100
	}
	setting, err := s.SettingOrDefault(ctx)
	if err != nil {
		return 0, err
	}
	if !setting.Enabled || !setting.UserNotificationsEnabled || setting.SubscriptionExpiryWarningDays <= 0 {
		return 0, nil
	}
	subs, err := s.entFromContext(ctx).UserSubscription.Query().
		Where(
			usersubscription.StatusEQ(usersubscription.StatusActive),
			usersubscription.ExpiresAtGT(now),
			usersubscription.ExpiresAtLTE(now.AddDate(0, 0, setting.SubscriptionExpiryWarningDays)),
		).
		Order(usersubscription.ByExpiresAt()).
		Limit(limit).
		All(ctx)
	if err != nil {
		return 0, err
	}
	created := 0
	for _, sub := range subs {
		eventKey := fmt.Sprintf("subscription_expiring:%d:%s", sub.ID, sub.ExpiresAt.Format("2006-01-02"))
		exists, err := s.entFromContext(ctx).BillingNotification.Query().
			Where(billingnotification.EventKeyEQ(eventKey)).
			Exist(ctx)
		if err != nil {
			return created, err
		}
		notification, err := s.CreateNotification(ctx, CreateBillingNotificationInput{
			UserID:             &sub.UserID,
			Audience:           billingnotification.AudienceUser,
			Category:           billingnotification.CategorySubscription,
			Severity:           billingnotification.SeverityWarning,
			EventKey:           eventKey,
			Title:              "Subscription expiring soon",
			Message:            "An active subscription is close to expiration.",
			Currency:           sub.Currency,
			AmountMicros:       &sub.PayableAmountMicros,
			UserSubscriptionID: &sub.ID,
		})
		if err != nil {
			return created, err
		}
		if notification != nil && !exists {
			created++
		}
	}
	return created, nil
}

func (s *BillingNotificationService) userAllowsCategory(ctx context.Context, userID int, category billingnotification.Category) (bool, error) {
	preference, err := s.PreferenceOrDefault(ctx, userID)
	if err != nil {
		return false, err
	}
	if !preference.Enabled {
		return false, nil
	}
	switch category {
	case billingnotification.CategoryLowBalance:
		return preference.LowBalanceEnabled, nil
	case billingnotification.CategoryPayment:
		return preference.PaymentEnabled, nil
	case billingnotification.CategorySubscription:
		return preference.SubscriptionEnabled, nil
	case billingnotification.CategoryLargeConsumption:
		return preference.LargeConsumptionEnabled, nil
	default:
		return true, nil
	}
}

func (s *BillingNotificationService) userIDFromBillingAccount(ctx context.Context, billingAccountID int) *int {
	account, err := s.entFromContext(ctx).BillingAccount.Get(ctx, billingAccountID)
	if err != nil || account.OwnerType != billingaccount.OwnerTypeUser {
		return nil
	}
	return &account.OwnerID
}

func normalizeEventKeyPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer(" ", "_", ":", "_", "/", "_", "\\", "_", "\n", "_", "\t", "_")
	return replacer.Replace(value)
}
