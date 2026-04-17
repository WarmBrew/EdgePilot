package middleware

// JWTClaims is an alias for the auth.Claims type for backwards compatibility.
// New code should use auth.Claims from the internal/pkg/auth package directly.
type JWTClaims = JWTClaimsStruct

// JWTClaimsStruct represents the claims embedded in a JWT token.
type JWTClaimsStruct struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
	Email    string `json:"email"`
}
