package audit

import (
	"strconv"
	"strings"
)

const (
	maskChar        = '*'
	maskVisibleTail = 4
	maskMinLength   = 8
)

// MaskPassword replaces the middle of a password with asterisks, keeping the first 4 chars visible.
func MaskPassword(password string) string {
	if len(password) == 0 {
		return ""
	}
	if len(password) <= maskMinLength {
		return strings.Repeat(string(maskChar), len(password))
	}
	visible := password[:4]
	return visible + strings.Repeat(string(maskChar), len(password)-4)
}

// MaskToken masks a token showing only the first 3 and last 4 characters.
func MaskToken(token string) string {
	if len(token) == 0 {
		return ""
	}
	if len(token) <= maskMinLength {
		return "***"
	}
	prefix := token[:3]
	suffix := token[len(token)-4:]
	return prefix + "***..." + suffix
}

// MaskFileContent returns a placeholder for binary file content with the byte count.
func MaskFileContent(sizeInBytes int) string {
	return "[binary content - " + strconv.Itoa(sizeInBytes) + " bytes]"
}

// MaskIPAddr partially masks an IP address for privacy, keeping the first two octets visible.
func MaskIPAddr(ip string) string {
	if ip == "" {
		return ""
	}
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + ".*.*"
	}
	// IPv6 or unknown format, mask everything after first segment
	if idx := strings.Index(ip, ":"); idx > 0 && idx < len(ip)-1 {
		return ip[:idx+1] + "***"
	}
	return "***"
}

// MaskEmail partially masks an email address, showing the first character and domain.
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}
	atIdx := strings.Index(email, "@")
	if atIdx <= 0 {
		return strings.Repeat(string(maskChar), len(email))
	}
	local := email[:atIdx]
	domain := email[atIdx:]
	if len(local) <= 1 {
		return local[0:1] + "***" + domain
	}
	return local[:1] + strings.Repeat(string(maskChar), len(local)-1) + domain
}

// MaskString fully masks a string, replacing all characters with asterisks.
func MaskString(s string) string {
	if s == "" {
		return ""
	}
	return strings.Repeat(string(maskChar), len(s))
}
