package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole checks if the authenticated user has one of the allowed roles.
// Must be used after JWTAuth middleware.
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get(ContextKeyUserRole)
		if !exists {
			slog.WarnContext(c.Request.Context(), "RequireRole: user role not found in context",
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			c.Abort()
			return
		}

		roleStr, ok := userRole.(string)
		if !ok {
			slog.ErrorContext(c.Request.Context(), "RequireRole: user role is not a string",
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			c.Abort()
			return
		}

		for _, r := range allowedRoles {
			if roleStr == r {
				c.Next()
				return
			}
		}

		slog.WarnContext(c.Request.Context(), "RequireRole: insufficient permissions",
			"user_role", roleStr,
			"required_roles", allowedRoles,
			"path", c.Request.URL.Path,
		)
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		c.Abort()
	}
}

// RequireAnyRole checks if the authenticated user has ANY of the allowed roles.
// This is an alias for RequireRole for clearer semantics.
func RequireAnyRole(roles ...string) gin.HandlerFunc {
	return RequireRole(roles...)
}

// RequireTenantIsolation ensures that the request context contains a tenant_id.
// This middleware is used as a gate to verify tenant isolation is enforced.
func RequireTenantIsolation() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, exists := c.Get(ContextKeyTenantID)
		if !exists {
			slog.WarnContext(c.Request.Context(), "RequireTenantIsolation: tenant_id not found in context",
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
			c.Abort()
			return
		}

		tid, ok := tenantID.(string)
		if !ok || tid == "" {
			slog.WarnContext(c.Request.Context(), "RequireTenantIsolation: invalid tenant_id",
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid tenant context"})
			c.Abort()
			return
		}

		c.Next()
	}
}
