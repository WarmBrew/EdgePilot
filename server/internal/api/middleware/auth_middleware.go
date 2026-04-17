package middleware

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/edge-platform/server/internal/config"
	"github.com/edge-platform/server/internal/pkg/auth"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/gin-gonic/gin"
)

// Context keys for storing user information in gin.Context
const (
	ContextKeyUserID    = "user_id"
	ContextKeyTenantID  = "tenant_id"
	ContextKeyUserRole  = "user_role"
	ContextKeyUserEmail = "user_email"
)

// Redis key prefix for token blacklist
const tokenBlacklistPrefix = "auth:blacklist:"

// JWTAuth validates the JWT token from the Authorization header,
// checks the token blacklist in Redis, and sets user context.
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			slog.WarnContext(c.Request.Context(), "JWTAuth: missing authorization header",
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		tokenString := extractBearerToken(authHeader)
		if tokenString == "" {
			slog.WarnContext(c.Request.Context(), "JWTAuth: invalid authorization format",
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			c.Abort()
			return
		}

		claims, err := auth.ValidateToken(tokenString)
		if err != nil {
			slog.WarnContext(c.Request.Context(), "JWTAuth: token validation failed",
				"error", err,
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Check if token is blacklisted in Redis
		if isTokenBlacklisted(c, tokenString) {
			slog.WarnContext(c.Request.Context(), "JWTAuth: token is blacklisted",
				"user_id", claims.UserID,
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
			c.Abort()
			return
		}

		// Set user context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyTenantID, claims.TenantID)
		c.Set(ContextKeyUserRole, claims.Role)
		c.Set(ContextKeyUserEmail, claims.Email)

		c.Next()
	}
}

// BlacklistToken adds a token to the Redis blacklist with the remaining TTL.
func BlacklistToken(tokenString string) error {
	client := pkgRedis.GetClient()
	ctx, cancel := pkgRedis.CtxWithTimeout(nil)
	defer cancel()
	key := blacklistKey(tokenString)

	err := client.Raw().Set(ctx, key, "1", 0).Err()
	if err != nil {
		return fmt.Errorf("failed to blacklist token: %w", err)
	}

	slog.Info("BlacklistToken: token blacklisted successfully")
	return nil
}

// GetUserID extracts the user ID from gin.Context.
func GetUserID(c *gin.Context) (string, bool) {
	val, exists := c.Get(ContextKeyUserID)
	if !exists {
		return "", false
	}
	userID, ok := val.(string)
	return userID, ok
}

// GetTenantID extracts the tenant ID from gin.Context.
func GetTenantID(c *gin.Context) (string, bool) {
	val, exists := c.Get(ContextKeyTenantID)
	if !exists {
		return "", false
	}
	tenantID, ok := val.(string)
	return tenantID, ok
}

// GetRole extracts the user role from gin.Context.
func GetRole(c *gin.Context) (string, bool) {
	val, exists := c.Get(ContextKeyUserRole)
	if !exists {
		return "", false
	}
	role, ok := val.(string)
	return role, ok
}

// GetEmail extracts the user email from gin.Context.
func GetEmail(c *gin.Context) (string, bool) {
	val, exists := c.Get(ContextKeyUserEmail)
	if !exists {
		return "", false
	}
	email, ok := val.(string)
	return email, ok
}

// MustGetUserID extracts user ID or panics if not set.
// Use only in handlers that are guaranteed to run after JWTAuth.
func MustGetUserID(c *gin.Context) string {
	id, ok := GetUserID(c)
	if !ok {
		panic("user_id not found in context, ensure JWTAuth middleware is registered")
	}
	return id
}

// MustGetTenantID extracts tenant ID or panics if not set.
func MustGetTenantID(c *gin.Context) string {
	id, ok := GetTenantID(c)
	if !ok {
		panic("tenant_id not found in context, ensure JWTAuth middleware is registered")
	}
	return id
}

// MustGetRole extracts role or panics if not set.
func MustGetRole(c *gin.Context) string {
	role, ok := GetRole(c)
	if !ok {
		panic("user_role not found in context, ensure JWTAuth middleware is registered")
	}
	return role
}

// extractBearerToken parses "Bearer <token>" and returns the token part.
func extractBearerToken(authHeader string) string {
	const prefix = "Bearer "
	if len(authHeader) < len(prefix) || authHeader[:len(prefix)] != prefix {
		return ""
	}
	return authHeader[len(prefix):]
}

// isTokenBlacklisted checks if the token exists in the Redis blacklist.
func isTokenBlacklisted(c *gin.Context, tokenString string) bool {
	client := pkgRedis.GetClient()
	reqCtx := c.Request.Context()
	ctx, cancel := pkgRedis.CtxWithTimeout(&reqCtx)
	defer cancel()
	key := blacklistKey(tokenString)

	exists, err := client.Raw().Exists(ctx, key).Result()
	if err != nil {
		slog.Error("isTokenBlacklisted: redis check failed",
			"error", err,
		)
		// On Redis failure, we allow the request to proceed
		// (fail-open rather than fail-closed for availability)
		return false
	}
	return exists > 0
}

// blacklistKey generates the Redis key for a blacklisted token.
// Uses JWT ID or a hash of the token string as the key identifier.
func blacklistKey(tokenString string) string {
	claims, err := auth.ValidateToken(tokenString)
	if err == nil && claims.ID != "" {
		return tokenBlacklistPrefix + claims.ID
	}
	// Fallback: use token hash if JWT ID is not set
	cfg := config.Get()
	hash := fmt.Sprintf("%x", auth.HashTokenForBlacklist(tokenString, cfg.JWT.Secret))
	return tokenBlacklistPrefix + "hash:" + hash
}
