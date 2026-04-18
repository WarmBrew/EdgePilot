package middleware

import (
	"encoding/json"
	"html"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	maxStringLength = 10 * 1024
)

// SanitizerConfig holds configuration for the request sanitizer middleware.
type SanitizerConfig struct {
	EscapeHTML     bool
	TrimSpaces     bool
	StripNullBytes bool
	MaxLength      int
}

// DefaultSanitizerConfig returns a default sanitizer configuration.
func DefaultSanitizerConfig() *SanitizerConfig {
	return &SanitizerConfig{
		EscapeHTML:     false,
		TrimSpaces:     true,
		StripNullBytes: true,
		MaxLength:      maxStringLength,
	}
}

// sanitizeValue recursively sanitizes strings in interface{} values.
func sanitizeValue(v interface{}, cfg *SanitizerConfig) interface{} {
	switch val := v.(type) {
	case string:
		return sanitizeString(val, cfg)
	case map[string]interface{}:
		result := make(map[string]interface{}, len(val))
		for k, v := range val {
			result[k] = sanitizeValue(v, cfg)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, v := range val {
			result[i] = sanitizeValue(v, cfg)
		}
		return result
	default:
		return v
	}
}

// sanitizeString applies string sanitization rules.
func sanitizeString(s string, cfg *SanitizerConfig) string {
	if cfg.StripNullBytes {
		s = nullByteRegex.ReplaceAllString(s, "")
	}
	if cfg.TrimSpaces {
		s = strings.TrimSpace(s)
	}
	if cfg.EscapeHTML {
		s = html.EscapeString(s)
	}
	if len(s) > cfg.MaxLength {
		s = s[:cfg.MaxLength]
	}
	return s
}

// sanitizeRequestBody reads the request body, sanitizes it, and restores it to the context.
func sanitizeRequestBody(c *gin.Context, cfg *SanitizerConfig) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	defer c.Request.Body.Close()

	if len(body) == 0 {
		return nil
	}

	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		c.Request.Body = io.NopCloser(strings.NewReader(string(body)))
		return nil
	}

	sanitized := sanitizeValue(data, cfg)

	sanitizedBody, err := json.Marshal(sanitized)
	if err != nil {
		c.Request.Body = io.NopCloser(strings.NewReader(string(body)))
		return nil
	}

	c.Request.Body = io.NopCloser(strings.NewReader(string(sanitizedBody)))
	c.Request.ContentLength = int64(len(sanitizedBody))

	return nil
}

// SanitizeRequests creates a middleware that sanitizes all request bodies.
func SanitizeRequests(config ...*SanitizerConfig) gin.HandlerFunc {
	cfg := DefaultSanitizerConfig()
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	return func(c *gin.Context) {
		if c.Request.Body != nil && c.Request.ContentLength > 0 {
			_ = sanitizeRequestBody(c, cfg)
		}
		c.Next()
	}
}

// SanitizeQueryParams sanitizes query parameter values.
func SanitizeQueryParams(config ...*SanitizerConfig) gin.HandlerFunc {
	cfg := DefaultSanitizerConfig()
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	return func(c *gin.Context) {
		for key, values := range c.Request.URL.Query() {
			for i, v := range values {
				values[i] = sanitizeString(v, cfg)
			}
			c.Request.URL.Query()[key] = values
		}
		c.Next()
	}
}

// RequestSizeLimiter creates a middleware that limits request body size.
func RequestSizeLimiter(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":   "REQUEST_TOO_LARGE",
				"message": "请求体过大，请减小请求内容",
			})
			c.Abort()
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		c.Next()
	}
}

// SanitizeString is a utility function for sanitizing individual strings.
// Useful for handlers that need to sanitize specific fields.
func SanitizeString(s string) string {
	cfg := DefaultSanitizerConfig()
	return sanitizeString(s, cfg)
}
