package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/gin-gonic/gin"
)

const (
	// Header names for request signature verification
	HeaderSignature = "X-Request-Signature"
	HeaderNonce     = "X-Request-Nonce"
	HeaderTimestamp = "X-Request-Timestamp"
	HeaderBodyHash  = "X-Request-Body-Hash"

	// Redis key prefix for used nonces
	nonceKeyPrefix = "auth:nonce:"

	// Default signature validation window
	DefaultSignatureWindow = 5 * time.Minute

	// Nonce TTL in Redis (slightly longer than the signature window)
	nonceTTL = 10 * time.Minute
)

// VerifySignature validates HMAC-SHA256 request signatures to prevent
// replay attacks and ensure request integrity.
//
// Required headers:
//   - X-Request-Signature: HMAC-SHA256 hex digest
//   - X-Request-Nonce: unique random string per request
//   - X-Request-Timestamp: Unix timestamp (seconds)
//
// Optional headers:
//   - X-Request-Body-Hash: SHA-256 hex digest of request body (for POST/PUT/PATCH)
func VerifySignature(secret string) gin.HandlerFunc {
	return VerifySignatureWithWindow(secret, DefaultSignatureWindow)
}

// VerifySignatureWithWindow is like VerifySignature but allows
// configuring the timestamp validation window.
func VerifySignatureWithWindow(secret string, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		signature := c.GetHeader(HeaderSignature)
		nonce := c.GetHeader(HeaderNonce)
		timestampStr := c.GetHeader(HeaderTimestamp)

		// All three headers are required
		if signature == "" || nonce == "" || timestampStr == "" {
			slog.WarnContext(c.Request.Context(), "VerifySignature: missing signature headers",
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusForbidden, gin.H{"error": "missing signature headers"})
			c.Abort()
			return
		}

		// Validate timestamp is within the allowed window
		if !isTimestampValid(timestampStr, window) {
			slog.WarnContext(c.Request.Context(), "VerifySignature: timestamp out of window",
				"timestamp", timestampStr,
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusForbidden, gin.H{"error": "request expired"})
			c.Abort()
			return
		}

		// Check nonce has not been used before (replay protection)
		if isNonceUsed(c, nonce) {
			slog.WarnContext(c.Request.Context(), "VerifySignature: replay detected",
				"nonce", nonce,
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusForbidden, gin.H{"error": "replay request rejected"})
			c.Abort()
			return
		}

		// Read body for hash computation
		bodyHash := c.GetHeader(HeaderBodyHash)
		if !verifySignature(c, secret, nonce, timestampStr, bodyHash, signature) {
			slog.WarnContext(c.Request.Context(), "VerifySignature: signature verification failed",
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid signature"})
			c.Abort()
			return
		}

		// Mark nonce as used
		markNonceUsed(c, nonce)

		c.Next()
	}
}

// isTimestampValid checks if the timestamp is within the allowed window.
func isTimestampValid(timestampStr string, window time.Duration) bool {
	var timestamp int64
	if _, err := fmt.Sscanf(timestampStr, "%d", &timestamp); err != nil {
		return false
	}

	ts := time.Unix(timestamp, 0)
	now := time.Now()

	diff := now.Sub(ts)
	if diff < 0 {
		diff = -diff
	}

	return diff <= window
}

// isNonceUsed checks if the nonce has already been used.
func isNonceUsed(c *gin.Context, nonce string) bool {
	client := pkgRedis.GetClient()
	reqCtx := c.Request.Context()
	ctx, cancel := pkgRedis.CtxWithTimeout(&reqCtx)
	defer cancel()
	key := nonceKeyWithPrefix(nonce)

	result, err := client.Raw().Exists(ctx, key).Result()
	if err != nil {
		slog.Error("isNonceUsed: redis check failed",
			"error", err,
		)
		// Fail-open on Redis error for availability
		return false
	}
	return result > 0
}

// markNonceUsed stores the nonce in Redis with a TTL.
func markNonceUsed(c *gin.Context, nonce string) {
	client := pkgRedis.GetClient()
	reqCtx := c.Request.Context()
	ctx, cancel := pkgRedis.CtxWithTimeout(&reqCtx)
	defer cancel()
	key := nonceKeyWithPrefix(nonce)

	err := client.Raw().Set(ctx, key, "1", nonceTTL).Err()
	if err != nil {
		slog.Error("markNonceUsed: failed to store nonce",
			"error", err,
			"nonce", nonce,
		)
	}
}

// verifySignature computes the expected HMAC-SHA256 and compares with the provided signature.
func verifySignature(c *gin.Context, secret, nonce, timestampStr, bodyHash, signature string) bool {
	// Read body for signature string computation
	var body []byte
	var err error

	if c.Request.Body != nil {
		body, err = io.ReadAll(c.Request.Body)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "verifySignature: failed to read body",
				"error", err,
			)
			return false
		}
		// Restore the body for downstream handlers
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
	}

	// If body hash header is provided, validate it
	if bodyHash != "" {
		actualBodyHash := sha256HashHex(body)
		if actualBodyHash != bodyHash {
			slog.WarnContext(c.Request.Context(), "verifySignature: body hash mismatch",
				"expected", bodyHash,
				"actual", actualBodyHash,
			)
			return false
		}
	}

	// Build signing string: timestamp + nonce + method + path + body_hash
	method := c.Request.Method
	path := c.Request.URL.Path
	computedBodyHash := sha256HashHex(body)

	signingString := fmt.Sprintf("%s+%s+%s+%s+%s",
		timestampStr, nonce, method, path, computedBodyHash,
	)

	// Compute HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

// sha256HashHex returns the hex-encoded SHA-256 hash of the input.
func sha256HashHex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// nonceKeyWithPrefix returns the Redis key for a nonce.
func nonceKeyWithPrefix(nonce string) string {
	return nonceKeyPrefix + nonce
}
