package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/edge-platform/server/internal/domain/models"
	"github.com/edge-platform/server/internal/pkg/auth"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db    *gorm.DB
	redis *pkgRedis.RedisClient
}

func NewAuthHandler(db *gorm.DB, redis *pkgRedis.RedisClient) *AuthHandler {
	return &AuthHandler{db: db, redis: redis}
}

func (h *AuthHandler) RegisterValidators() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("password_strength", passwordStrengthValidator)
	}
}

var uppercaseRegex = regexp.MustCompile(`[A-Z]`)
var lowercaseRegex = regexp.MustCompile(`[a-z]`)
var numberRegex = regexp.MustCompile(`[0-9]`)

const specialChars = "!@#$%^&*()_+-=[]{}|;':\",.<>?/~`"

func hasSpecialChar(s string) bool {
	for _, c := range s {
		for _, sc := range specialChars {
			if c == sc {
				return true
			}
		}
	}
	return false
}

func passwordStrengthValidator(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	if len(password) < 8 {
		return false
	}
	if !uppercaseRegex.MatchString(password) {
		return false
	}
	if !lowercaseRegex.MatchString(password) {
		return false
	}
	if !numberRegex.MatchString(password) {
		return false
	}
	if !hasSpecialChar(password) {
		return false
	}
	return true
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,password_strength"`
	Name     string `json:"name" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	TenantID  string    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AuthResponse struct {
	AccessToken string       `json:"access_token"`
	User        UserResponse `json:"user"`
}

func userToResponse(user models.User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Role:      user.Role,
		TenantID:  user.TenantID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + sanitizeValidationError(err)})
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var existingCount int64
	if err := h.db.Model(&models.User{}).Where("email = ?", req.Email).Count(&existingCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if existingCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "registration failed"})
		return
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Auto-assign to first tenant or create default
	var tenant models.Tenant
	if err := h.db.First(&tenant).Error; err != nil {
		// No tenant exists, create default
		tenant = models.Tenant{
			Name: "Default",
			Plan: "free",
		}
		if err := h.db.Create(&tenant).Error; err != nil {
			// Another concurrent request may have created the tenant, try to fetch it
			if err2 := h.db.Where("name = ?", "Default").First(&tenant).Error; err2 != nil {
				slog.Error("failed to get or create default tenant", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
				return
			}
		}
		slog.Info("default tenant ready", "tenant_id", tenant.ID)
	}

	user := models.User{
		Email:    req.Email,
		Password: hashedPassword,
		Role:     models.RoleAdmin,
		TenantID: tenant.ID,
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "registration failed"})
		return
	}

	accessToken, err := auth.GenerateAccessToken(user.ID, user.TenantID, user.Role, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	slog.Info("user registered successfully",
		"user_id", user.ID, "email", user.Email, "tenant_id", tenant.ID, "role", user.Role)

	c.JSON(http.StatusCreated, AuthResponse{
		AccessToken: accessToken,
		User:        userToResponse(user),
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var user models.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "email or password is incorrect"})
		return
	}

	if err := auth.CheckPassword(user.Password, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "email or password is incorrect"})
		return
	}

	now := time.Now()
	h.db.Model(&user).Update("last_login", now)

	accessToken, err := auth.GenerateAccessToken(user.ID, user.TenantID, user.Role, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(user.ID, user.TenantID, user.Role, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	refreshKey := fmt.Sprintf("refresh_token:%s:%s", user.ID, refreshToken)
	if err := h.redis.Raw().Set(c, refreshKey, "1", 7*24*time.Hour).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("refresh_token", refreshToken, int((7*24*time.Hour)/time.Second), "/", "", true, true)

	c.JSON(http.StatusOK, AuthResponse{
		AccessToken: accessToken,
		User:        userToResponse(user),
	})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token required"})
		return
	}

	claims, err := auth.ValidateToken(refreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	refreshKey := fmt.Sprintf("refresh_token:%s:%s", claims.UserID, refreshToken)

	// Use Redis Lua script for atomic check-delete-set to prevent race conditions
	luaScript := `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			redis.call("DEL", KEYS[1])
			redis.call("SET", KEYS[2], "1", "EX", ARGV[2])
			return 1
		else
			return 0
		end
	`

	refreshExpiry := 7 * 24 * 3600 // 7 days in seconds
	newRefreshTokenVal := auth.GenerateRefreshTokenRaw(claims.UserID, claims.TenantID, claims.Role, claims.Email, 7*24*time.Hour)
	newRefreshKey := fmt.Sprintf("refresh_token:%s:%s", claims.UserID, newRefreshTokenVal)

	result, err := h.redis.Raw().Eval(c, luaScript, []string{refreshKey, newRefreshKey}, "1", newRefreshKey, refreshExpiry).Int()
	if err != nil || result != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token expired or revoked"})
		return
	}

	newAccessToken, err := auth.GenerateAccessToken(claims.UserID, claims.TenantID, claims.Role, claims.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("refresh_token", newRefreshTokenVal, refreshExpiry, "/", "", true, true)

	c.JSON(http.StatusOK, gin.H{"access_token": newAccessToken})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err == nil && refreshToken != "" {
		claims, err := auth.ValidateToken(refreshToken)
		if err == nil {
			refreshKey := fmt.Sprintf("refresh_token:%s:%s", claims.UserID, refreshToken)
			_ = h.redis.Raw().Del(c, refreshKey).Err()
		}
	}

	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("refresh_token", "", -1, "/", "", true, true)

	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func sanitizeValidationError(err error) string {
	if ve, ok := err.(validator.ValidationErrors); ok {
		var fields []string
		for _, e := range ve {
			fields = append(fields, e.Field())
		}
		return fmt.Sprintf("validation failed for: %s", strings.Join(fields, ", "))
	}
	return err.Error()
}
