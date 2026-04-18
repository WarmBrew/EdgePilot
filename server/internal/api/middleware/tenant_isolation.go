package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TenantScopedModel is the interface that models must implement
// to support tenant-based row-level isolation.
type TenantScopedModel interface {
	GetTenantID() string
}

// ScopeByTenant adds a WHERE clause to a GORM query to filter by tenant_id.
// This ensures row-level data isolation for multi-tenant applications.
//
// Usage:
//
//	db := middleware.ScopeByTenant(db).Find(&devices)
//	db := middleware.ScopeByTenant(db).First(&device, id)
func ScopeByTenant(query *gorm.DB) *gorm.DB {
	tenantID := getTenantIDFromQueryContext(query)
	if tenantID == "" {
		slog.Warn("ScopeByTenant: tenant_id not available in context, query not scoped")
		return query
	}

	return query.Where("tenant_id = ?", tenantID)
}

// ScopeByTenantOr adds an optional tenant scope. If tenantID is empty,
// the query is returned unchanged (useful for admin/superuser queries).
func ScopeByTenantOr(query *gorm.DB, tenantID string) *gorm.DB {
	if tenantID == "" {
		return query
	}
	return query.Where("tenant_id = ?", tenantID)
}

// CheckResourceOwnership verifies that the given resource belongs to
// the tenant of the current user in the gin.Context.
//
// The resource must either:
// - Implement the TenantScopedModel interface, or
// - Have a TenantID or tenant_id field (struct tag or field name)
func CheckResourceOwnership(c *gin.Context, resource interface{}) bool {
	tenantID, ok := GetTenantID(c)
	if !ok || tenantID == "" {
		slog.WarnContext(c.Request.Context(), "CheckResourceOwnership: tenant_id not in context")
		return false
	}

	resourceTenantID := extractTenantID(resource)
	if resourceTenantID == "" {
		slog.WarnContext(c.Request.Context(), "CheckResourceOwnership: could not extract tenant_id from resource",
			"resource_type", reflect.TypeOf(resource).String(),
		)
		return false
	}

	if resourceTenantID != tenantID {
		slog.WarnContext(c.Request.Context(), "CheckResourceOwnership: tenant mismatch",
			"user_tenant_id", tenantID,
			"resource_tenant_id", resourceTenantID,
		)
		return false
	}

	return true
}

// RequireOwnership is a gin.HandlerFunc that checks resource ownership
// and aborts with 403 if the resource does not belong to the current tenant.
// The resource must be pre-loaded and passed via context key "resource".
func RequireOwnership() gin.HandlerFunc {
	return func(c *gin.Context) {
		resource := c.Value("resource")
		if resource == nil {
			slog.ErrorContext(c.Request.Context(), "RequireOwnership: resource not set in context")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			c.Abort()
			return
		}

		if !CheckResourceOwnership(c, resource) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied: resource does not belong to your tenant"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// extractTenantID extracts the tenant ID from a resource using reflection.
// Supports: interface method, struct field "TenantID", struct field "tenant_id".
func extractTenantID(resource interface{}) string {
	if resource == nil {
		return ""
	}

	// Try TenantScopedModel interface first
	if tm, ok := resource.(TenantScopedModel); ok {
		return tm.GetTenantID()
	}

	// Try pointer receiver
	v := reflect.ValueOf(resource)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return ""
	}

	// Check for TenantID or tenant_id field
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		fieldName := field.Name

		// Match by field name or json tag
		if fieldName == "TenantID" || fieldName == "tenant_id" ||
			tag == "tenant_id" {
			val := v.Field(i)
			if val.Kind() == reflect.String {
				return val.String()
			}
		}
	}

	return ""
}

// getTenantIDFromQueryContext tries to extract tenant_id from GORM's
// statement context or falls back to the gin context if available.
func getTenantIDFromQueryContext(query *gorm.DB) string {
	// Try the query's statement hints
	if query.Statement != nil && query.Statement.Context != nil {
		if val := query.Statement.Context.Value(ContextKeyTenantID); val != nil {
			if tid, ok := val.(string); ok {
				return tid
			}
		}
	}

	return ""
}

// WithTenantContext wraps db.Statement.Context with tenant_id for GORM queries.
// Use this when building queries outside of direct gin handler flow.
func WithTenantContext(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ContextKeyTenantID, tenantID)
}
