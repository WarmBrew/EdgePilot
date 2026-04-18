package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/edge-platform/server/internal/domain/models"
	"github.com/edge-platform/server/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

// AuditMiddlewareOptions configures the audit logging middleware.
type AuditMiddlewareOptions struct {
	AuditService *service.AuditService
	// AdminPrefixes are URL prefixes that trigger admin audit logging.
	AdminPrefixes []string
	// SkipPaths are paths that should not be audited.
	SkipPaths []string
}

// AuditMiddleware returns a gin middleware that automatically logs:
// - All auth events (login/logout/refresh token requests)
// - All 4xx/5xx responses
// - All admin actions (user management, system config)
func AuditMiddleware(opts *AuditMiddlewareOptions) gin.HandlerFunc {
	adminPrefixes := opts.AdminPrefixes
	if len(adminPrefixes) == 0 {
		adminPrefixes = []string{"/api/v1/admin", "/api/v1/users", "/api/v1/tenants", "/api/v1/config"}
	}

	skipPaths := make(map[string]bool)
	for _, p := range opts.SkipPaths {
		skipPaths[p] = true
	}

	return func(c *gin.Context) {
		start := time.Now()

		// Capture response body for error logging
		rw := &responseWriter{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Writer = rw

		c.Next()

		// Skip health checks and static assets
		if skipPaths[c.Request.URL.Path] {
			return
		}

		status := rw.status
		userID, _ := GetUserID(c)
		tenantID, _ := GetTenantID(c)
		path := c.Request.URL.Path
		method := c.Request.Method
		ip := c.ClientIP()
		latency := time.Since(start)

		ctx := c.Request.Context()

		// Log auth events
		if isAuthPath(path) {
			action := authActionFromPath(path, method, status)
			if action != "" {
				opts.AuditService.LogAuthEvent(ctx, userIDOrAnonymous(userID), action, ip)
			}
			return
		}

		// Log admin actions
		if isAdminPath(path, adminPrefixes) {
			detail := buildAdminDetail(method, path, status)
			opts.AuditService.LogPermissionChange(ctx, userID, path, "admin_action", detail)
			return
		}

		// Log client/server errors (4xx/5xx)
		if status >= 400 {
			detail := buildErrorDetail(method, path, status, latency)
			if userID != "" {
				opts.AuditService.Log(ctx, errorLog(userID, tenantID, ip, method, path, status, detail))
			} else {
				opts.AuditService.Log(ctx, anonErrorLog(tenantID, ip, method, path, status, detail))
			}
		}
	}
}

func isAuthPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/auth/")
}

func authActionFromPath(path, method string, status int) string {
	switch {
	case strings.HasSuffix(path, "/login") && method == "POST":
		if status >= 200 && status < 300 {
			return "login_success"
		}
		return "login_failed"
	case strings.HasSuffix(path, "/register") && method == "POST":
		if status >= 200 && status < 300 {
			return "register_success"
		}
		return "register_failed"
	case strings.HasSuffix(path, "/logout") && method == "POST":
		return "logout"
	case strings.HasSuffix(path, "/refresh") && method == "POST":
		if status >= 200 && status < 300 {
			return "token_refreshed"
		}
		return "token_refresh_failed"
	}
	return ""
}

func isAdminPath(path string, adminPrefixes []string) bool {
	for _, prefix := range adminPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func buildAdminDetail(method, path string, status int) string {
	return `{` +
		`"method":` + jsonStringLit(method) +
		`,"path":` + jsonStringLit(path) +
		`,"status":` + intLit(status) +
		`}`
}

func buildErrorDetail(method, path string, status int, latency time.Duration) string {
	return `{` +
		`"method":` + jsonStringLit(method) +
		`,"path":` + jsonStringLit(path) +
		`,"status":` + intLit(status) +
		`,"latency_ms":` + intLit(int(latency.Milliseconds())) +
		`}`
}

func userIDOrAnonymous(uid string) string {
	if uid == "" {
		return "anonymous"
	}
	return uid
}

func errorLog(userID, tenantID, ip, method, path string, status int, detail string) *models.AuditLog {
	return &models.AuditLog{
		TenantID:  tenantID,
		UserID:    userID,
		DeviceID:  "N/A",
		Action:    "http_error",
		IPAddress: ip,
		Detail:    datatypes.JSON(`{"method":` + jsonStringLit(method) + `,"path":` + jsonStringLit(path) + `,"status":` + strconv.Itoa(status) + `,"detail":` + detail + `}`),
	}
}

func anonErrorLog(tenantID, ip, method, path string, status int, detail string) *models.AuditLog {
	return &models.AuditLog{
		TenantID:  tenantID,
		UserID:    "anonymous",
		DeviceID:  "N/A",
		Action:    "http_error",
		IPAddress: ip,
		Detail:    datatypes.JSON(`{"method":` + jsonStringLit(method) + `,"path":` + jsonStringLit(path) + `,"status":` + strconv.Itoa(status) + `,"detail":` + detail + `}`),
	}
}

// helper: build a datatypes-compatible JSON string literal
func jsonStringLit(s string) string {
	b := make([]byte, 0, len(s)+2)
	b = append(b, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' || c == '\\' {
			b = append(b, '\\')
		}
		b = append(b, c)
	}
	b = append(b, '"')
	return string(b)
}

func intLit(n int) string {
	return strconv.Itoa(n)
}

// ---- response writer wrapper ----

type responseWriter struct {
	gin.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
