package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
)

const SystemKeyRegistrationSettings = "registration_settings"

const (
	defaultRegistrationProjectName = "My Project"
	defaultRegistrationAPIKeyName  = "Default API Key"
	defaultRegistrationRateWindow  = 3600
	defaultRegistrationRateMax     = 20
)

type RegistrationSettings struct {
	Enabled                bool   `json:"enabled"`
	RequireApproval        bool   `json:"require_approval"`
	CreateDefaultProject   bool   `json:"create_default_project"`
	CreateDefaultAPIKey    bool   `json:"create_default_api_key"`
	SignupGrantAmount      string `json:"signup_grant_amount"`
	DefaultProjectName     string `json:"default_project_name"`
	DefaultAPIKeyName      string `json:"default_api_key_name"`
	RateLimitWindowSeconds int    `json:"rate_limit_window_seconds"`
	RateLimitMaxAttempts   int    `json:"rate_limit_max_attempts"`
}

type PublicRegistrationStatus struct {
	Enabled         bool `json:"enabled"`
	RequireApproval bool `json:"require_approval"`
}

type RegisterUserInput struct {
	Email          string
	Password       string
	FirstName      string
	LastName       string
	PreferLanguage string
	ClientIP       string
}

type RegisterUserResult struct {
	User           *ent.User
	BillingAccount *ent.BillingAccount
	Project        *ent.Project
	APIKey         *ent.APIKey
}

type registrationAttemptBucket struct {
	WindowStart time.Time
	Attempts    int
}

type RegistrationServiceParams struct {
	fx.In

	Ent                   *ent.Client
	SystemService         *SystemService
	BillingAccountService *BillingAccountService
	LedgerService         *LedgerService
	ProjectService        *ProjectService
	APIKeyService         *APIKeyService
	BillingAuditService   *BillingAuditService
}

type RegistrationService struct {
	*AbstractService

	SystemService         *SystemService
	BillingAccountService *BillingAccountService
	LedgerService         *LedgerService
	ProjectService        *ProjectService
	APIKeyService         *APIKeyService
	BillingAuditService   *BillingAuditService
	rateLimitMu           sync.Mutex
	registrationAttempts  sync.Map
}

func NewRegistrationService(params RegistrationServiceParams) *RegistrationService {
	return &RegistrationService{
		AbstractService: &AbstractService{db: params.Ent},

		SystemService:         params.SystemService,
		BillingAccountService: params.BillingAccountService,
		LedgerService:         params.LedgerService,
		ProjectService:        params.ProjectService,
		APIKeyService:         params.APIKeyService,
		BillingAuditService:   params.BillingAuditService,
	}
}

func DefaultRegistrationSettings() RegistrationSettings {
	return RegistrationSettings{
		Enabled:                false,
		RequireApproval:        false,
		CreateDefaultProject:   false,
		CreateDefaultAPIKey:    false,
		SignupGrantAmount:      "0",
		DefaultProjectName:     defaultRegistrationProjectName,
		DefaultAPIKeyName:      defaultRegistrationAPIKeyName,
		RateLimitWindowSeconds: defaultRegistrationRateWindow,
		RateLimitMaxAttempts:   defaultRegistrationRateMax,
	}
}

func (s *RegistrationService) RegistrationSettings(ctx context.Context) (*RegistrationSettings, error) {
	value, err := s.SystemService.getSystemValue(ctx, SystemKeyRegistrationSettings)
	if err != nil {
		if ent.IsNotFound(err) {
			settings := DefaultRegistrationSettings()
			return &settings, nil
		}

		return nil, fmt.Errorf("failed to get registration settings: %w", err)
	}

	settings := DefaultRegistrationSettings()
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal registration settings: %w", err)
	}
	normalizeRegistrationSettings(&settings)

	return &settings, nil
}

func (s *RegistrationService) SetRegistrationSettings(ctx context.Context, settings RegistrationSettings) error {
	normalizeRegistrationSettings(&settings)

	if _, err := parseSignupGrantAmount(settings.SignupGrantAmount); err != nil {
		return err
	}

	jsonBytes, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal registration settings: %w", err)
	}

	if err := s.SystemService.setSystemValue(ctx, SystemKeyRegistrationSettings, string(jsonBytes)); err != nil {
		return fmt.Errorf("failed to set registration settings: %w", err)
	}

	return nil
}

func (s *RegistrationService) PublicRegistrationStatus(ctx context.Context) (*PublicRegistrationStatus, error) {
	settings, err := authz.RunWithSystemBypass(ctx, "registration-public-status", func(bypassCtx context.Context) (*RegistrationSettings, error) {
		return s.RegistrationSettings(bypassCtx)
	})
	if err != nil {
		return nil, err
	}

	return &PublicRegistrationStatus{
		Enabled:         settings.Enabled,
		RequireApproval: settings.RequireApproval,
	}, nil
}

func (s *RegistrationService) Register(ctx context.Context, input RegisterUserInput) (*RegisterUserResult, error) {
	email, err := normalizeRegistrationEmail(input.Email)
	if err != nil {
		return nil, err
	}
	if err := validateRegistrationPassword(input.Password); err != nil {
		return nil, err
	}

	bypassCtx := authz.WithSystemBypass(ctx, "public-registration")

	settings, err := s.RegistrationSettings(bypassCtx)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return nil, ErrRegistrationDisabled
	}

	grantAmount, err := parseSignupGrantAmount(settings.SignupGrantAmount)
	if err != nil {
		return nil, err
	}
	if err := s.checkRegistrationRateLimit(email, input.ClientIP, *settings); err != nil {
		return nil, err
	}

	var result *RegisterUserResult
	err = s.RunInTransaction(bypassCtx, func(txCtx context.Context) error {
		client := s.entFromContext(txCtx)

		exists, err := client.User.Query().Where(user.EmailEQ(email)).Exist(txCtx)
		if err != nil {
			return fmt.Errorf("failed to check registration email: %w", err)
		}
		if exists {
			return ErrRegistrationEmailExists
		}

		hashedPassword, err := HashPassword(input.Password)
		if err != nil {
			return err
		}

		status := user.StatusActivated
		if settings.RequireApproval {
			status = user.StatusDeactivated
		}

		createdUser, err := client.User.Create().
			SetEmail(email).
			SetPassword(hashedPassword).
			SetStatus(status).
			SetPreferLanguage(normalizeRegistrationLanguage(input.PreferLanguage)).
			SetFirstName(strings.TrimSpace(input.FirstName)).
			SetLastName(strings.TrimSpace(input.LastName)).
			SetIsOwner(false).
			SetScopes([]string{}).
			Save(txCtx)
		if ent.IsConstraintError(err) {
			return ErrRegistrationEmailExists
		}
		if err != nil {
			return fmt.Errorf("failed to create registered user: %w", err)
		}

		billingAccount, err := s.BillingAccountService.GetOrCreateForSubject(txCtx, UserBillingSubject(createdUser.ID))
		if err != nil {
			return fmt.Errorf("failed to create user billing account: %w", err)
		}

		if grantAmount.IsPositive() {
			_, err = s.LedgerService.Post(txCtx, LedgerPostInput{
				BillingAccountID: billingAccount.ID,
				Direction:        ledgertransaction.DirectionCredit,
				Amount:           grantAmount,
				Currency:         billingAccount.Currency,
				Type:             ledgertransaction.TypeAdminAdjustment,
				IdempotencyKey:   fmt.Sprintf("registration:signup-grant:user:%d", createdUser.ID),
				ReferenceType:    "registration",
				ReferenceID:      fmt.Sprintf("%d", createdUser.ID),
				Memo:             "signup grant",
				CreatedByType:    ledgertransaction.CreatedByTypeSystem,
			})
			if err != nil {
				return fmt.Errorf("failed to grant signup balance: %w", err)
			}

			updatedBillingAccount, err := client.BillingAccount.Get(txCtx, billingAccount.ID)
			if err != nil {
				return fmt.Errorf("failed to reload signup billing account: %w", err)
			}
			billingAccount = updatedBillingAccount
		}

		result = &RegisterUserResult{
			User:           createdUser,
			BillingAccount: billingAccount,
		}

		if settings.CreateDefaultProject {
			userCtx := contexts.WithUser(txCtx, createdUser)
			projectName, err := s.uniqueRegistrationProjectName(txCtx, settings.DefaultProjectName, createdUser.ID)
			if err != nil {
				return err
			}

			createdProject, err := s.ProjectService.CreateProject(userCtx, ent.CreateProjectInput{
				Name: projectName,
			})
			if err != nil {
				return fmt.Errorf("failed to create registration project: %w", err)
			}
			result.Project = createdProject

			if settings.CreateDefaultAPIKey {
				apiKeyName := strings.TrimSpace(settings.DefaultAPIKeyName)
				if apiKeyName == "" {
					apiKeyName = defaultRegistrationAPIKeyName
				}

				createdAPIKey, err := s.APIKeyService.CreateAPIKey(userCtx, ent.CreateAPIKeyInput{
					Name:      apiKeyName,
					ProjectID: createdProject.ID,
				})
				if err != nil {
					return fmt.Errorf("failed to create registration api key: %w", err)
				}
				result.APIKey = createdAPIKey
			}
		}

		if s.BillingAuditService != nil {
			metadata, err := json.Marshal(map[string]any{
				"email":                     createdUser.Email,
				"status":                    createdUser.Status,
				"created_default_project":   result.Project != nil,
				"created_default_api_key":   result.APIKey != nil,
				"signup_grant_amount":       settings.SignupGrantAmount,
				"rate_limit_window_seconds": settings.RateLimitWindowSeconds,
				"rate_limit_max_attempts":   settings.RateLimitMaxAttempts,
			})
			if err != nil {
				return fmt.Errorf("failed to marshal registration audit metadata: %w", err)
			}
			_, err = s.BillingAuditService.RecordSystem(txCtx, BillingAuditInput{
				Action:       "registration.user_created",
				TargetType:   "user",
				TargetID:     AuditTargetID(createdUser.ID),
				TargetUserID: &createdUser.ID,
				Reason:       "public registration",
				Metadata:     objects.JSONRawMessage(metadata),
			})
			if err != nil {
				return fmt.Errorf("failed to write registration audit log: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *RegistrationService) uniqueRegistrationProjectName(ctx context.Context, baseName string, userID int) (string, error) {
	name := strings.TrimSpace(baseName)
	if name == "" {
		name = defaultRegistrationProjectName
	}

	client := s.entFromContext(ctx)
	exists, err := client.Project.Query().Where(project.NameEQ(name)).Exist(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to check registration project name: %w", err)
	}
	if !exists {
		return name, nil
	}

	return fmt.Sprintf("%s #%d", name, userID), nil
}

func normalizeRegistrationSettings(settings *RegistrationSettings) {
	settings.SignupGrantAmount = strings.TrimSpace(settings.SignupGrantAmount)
	if settings.SignupGrantAmount == "" {
		settings.SignupGrantAmount = "0"
	}

	settings.DefaultProjectName = strings.TrimSpace(settings.DefaultProjectName)
	if settings.DefaultProjectName == "" {
		settings.DefaultProjectName = defaultRegistrationProjectName
	}

	settings.DefaultAPIKeyName = strings.TrimSpace(settings.DefaultAPIKeyName)
	if settings.DefaultAPIKeyName == "" {
		settings.DefaultAPIKeyName = defaultRegistrationAPIKeyName
	}

	if settings.CreateDefaultAPIKey {
		settings.CreateDefaultProject = true
	}

	if settings.RateLimitWindowSeconds < 0 {
		settings.RateLimitWindowSeconds = defaultRegistrationRateWindow
	}
	if settings.RateLimitMaxAttempts < 0 {
		settings.RateLimitMaxAttempts = defaultRegistrationRateMax
	}
}

func parseSignupGrantAmount(raw string) (decimal.Decimal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "0"
	}

	amount, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid signup grant amount: %w", err)
	}
	if amount.IsNegative() {
		return decimal.Zero, fmt.Errorf("signup grant amount must not be negative")
	}

	return amount, nil
}

func normalizeRegistrationEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" {
		return "", ErrRegistrationInvalidEmail
	}

	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return "", ErrRegistrationInvalidEmail
	}

	return email, nil
}

func normalizeRegistrationLanguage(raw string) string {
	language := strings.TrimSpace(raw)
	if language == "" {
		return "en"
	}

	return language
}

func validateRegistrationPassword(password string) error {
	if len(password) < 8 {
		return ErrRegistrationPasswordWeak
	}

	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit {
		return ErrRegistrationPasswordWeak
	}

	return nil
}

func (s *RegistrationService) checkRegistrationRateLimit(email, clientIP string, settings RegistrationSettings) error {
	if settings.RateLimitWindowSeconds <= 0 || settings.RateLimitMaxAttempts <= 0 {
		return nil
	}

	key := strings.TrimSpace(clientIP) + "|" + email
	now := time.Now()
	window := time.Duration(settings.RateLimitWindowSeconds) * time.Second

	s.rateLimitMu.Lock()
	defer s.rateLimitMu.Unlock()

	value, _ := s.registrationAttempts.LoadOrStore(key, registrationAttemptBucket{
		WindowStart: now,
		Attempts:    0,
	})
	bucket := value.(registrationAttemptBucket)
	if now.Sub(bucket.WindowStart) >= window {
		bucket = registrationAttemptBucket{WindowStart: now}
	}

	bucket.Attempts++
	s.registrationAttempts.Store(key, bucket)
	if bucket.Attempts > settings.RateLimitMaxAttempts {
		return ErrRegistrationRateLimited
	}

	return nil
}
