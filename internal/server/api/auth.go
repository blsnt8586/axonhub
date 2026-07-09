package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

type AuthHandlersParams struct {
	fx.In

	AuthService         *biz.AuthService
	RegistrationService *biz.RegistrationService
}

func NewAuthHandlers(params AuthHandlersParams) *AuthHandlers {
	return &AuthHandlers{
		AuthService:         params.AuthService,
		RegistrationService: params.RegistrationService,
	}
}

type AuthHandlers struct {
	AuthService         *biz.AuthService
	RegistrationService *biz.RegistrationService
}

// SignInRequest 登录请求.
type SignInRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// SignInResponse 登录响应.
type SignInResponse struct {
	User  *objects.UserInfo `json:"user"`
	Token string            `json:"token"`
}

type RegistrationStatusResponse struct {
	Enabled         bool `json:"enabled"`
	RequireApproval bool `json:"requireApproval"`
}

type RegisterRequest struct {
	Email          string `json:"email"                    binding:"required,email"`
	Password       string `json:"password"                 binding:"required"`
	FirstName      string `json:"firstName,omitempty"`
	LastName       string `json:"lastName,omitempty"`
	PreferLanguage string `json:"preferLanguage,omitempty"`
}

type RegisterResponse struct {
	Success         bool              `json:"success"`
	Message         string            `json:"message"`
	RequireApproval bool              `json:"requireApproval"`
	User            *objects.UserInfo `json:"user,omitempty"`
}

// SignIn handles user authentication.
func (h *AuthHandlers) SignIn(c *gin.Context) {
	var (
		ctx = c.Request.Context()
		req SignInRequest
	)

	err := c.ShouldBindJSON(&req)
	if err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("Invalid request format"))
		return
	}

	// Authenticate user
	user, err := h.AuthService.AuthenticateUser(ctx, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, biz.ErrInvalidPassword) {
			JSONError(c, http.StatusUnauthorized, errors.New("Invalid email or password"))
			return
		}

		JSONError(c, http.StatusInternalServerError, errors.New("Internal server error"))

		return
	}

	// Generate JWT token
	token, err := h.AuthService.GenerateJWTToken(ctx, user)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, errors.New("Internal server error"))
		return
	}

	response := SignInResponse{
		User:  biz.ConvertUserToUserInfo(ctx, user),
		Token: token,
	}

	c.JSON(http.StatusOK, response)
}

func (h *AuthHandlers) GetRegistrationStatus(c *gin.Context) {
	status, err := h.RegistrationService.PublicRegistrationStatus(c.Request.Context())
	if err != nil {
		JSONError(c, http.StatusInternalServerError, errors.New("Failed to get registration status"))
		return
	}

	c.JSON(http.StatusOK, RegistrationStatusResponse{
		Enabled:         status.Enabled,
		RequireApproval: status.RequireApproval,
	})
}

func (h *AuthHandlers) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("Invalid request format"))
		return
	}

	result, err := h.RegistrationService.Register(c.Request.Context(), biz.RegisterUserInput{
		Email:          req.Email,
		Password:       req.Password,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		PreferLanguage: req.PreferLanguage,
		ClientIP:       c.ClientIP(),
	})
	if err != nil {
		switch {
		case errors.Is(err, biz.ErrRegistrationDisabled):
			JSONError(c, http.StatusForbidden, err)
		case errors.Is(err, biz.ErrRegistrationEmailExists):
			JSONError(c, http.StatusConflict, err)
		case errors.Is(err, biz.ErrRegistrationInvalidEmail), errors.Is(err, biz.ErrRegistrationPasswordWeak):
			JSONError(c, http.StatusBadRequest, err)
		case errors.Is(err, biz.ErrRegistrationRateLimited):
			JSONError(c, http.StatusTooManyRequests, err)
		default:
			JSONError(c, http.StatusInternalServerError, errors.New("Internal server error"))
		}

		return
	}

	requireApproval := result.User.Status == "deactivated"
	message := "Registration successful"
	if requireApproval {
		message = "Registration submitted and awaiting approval"
	}

	c.JSON(http.StatusOK, RegisterResponse{
		Success:         true,
		Message:         message,
		RequireApproval: requireApproval,
		User:            biz.ConvertUserToUserInfo(c.Request.Context(), result.User),
	})
}
