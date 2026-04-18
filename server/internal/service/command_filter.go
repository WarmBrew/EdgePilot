package service

import (
	"fmt"
	"regexp"
	"strings"
)

// DangerousCommands defines patterns of commands that are blacklisted for security.
var DangerousCommands = []string{
	`rm\s+(-[a-zA-Z0-9]+\s+)*--no-preserve-root\s+/`,
	`rm\s+-rf\s+/`,
	`rm\s+-rf\s+/\*`,
	`>\s*/dev/sd[a-z]`,
	`>\s*/dev/hd[a-z]`,
	`:\(\)\{:\|:&\};:`,
	`curl\s+.*\|\s*(ba)?sh`,
	`curl\s+.*\|\s*python`,
	`curl\s+.*\|\s*perl`,
	`wget\s+.*\|\s*(ba)?sh`,
	`wget\s+.*\|\s*python`,
	`eval\s+\$\(.*\|.*(base64|curl|wget)`,
	`\$\(\s*.*(curl|wget|nc|ncat|nmap)\s+`,
	";\\s*(rm|dd|mkfs)\\b",
	"mkfs\\.\\w+",
	"\\bmkfs\\b",
	"fdisk\\b",
	"\\bdd\\b.*if=.*of=.*",
	"nc\\s+-[el].*",
}

var SensitiveCommands = []string{
	"\\bsudo\\b",
	"\\breboot\\b",
	"\\bshutdown\\b",
	"systemctl\\s+(restart|stop|disable|mask|kill)",
	"service\\s+.*\\s+(restart|stop)",
	"kill\\s+-9",
	"\\bpkill\\b",
	"\\biptables\\b",
	"\\bufw\\b",
	"\\bfwadm\\b",
	"\\bnft\\b",
	"chmod\\s+[0-7]?77[7]",
	"\\bchown\\s+root",
}

// ReadOnlyCommands defines the whitelist of commands allowed for viewer role.
var ReadOnlyCommands = []string{"ls", "cat", "top", "ps", "df", "free", "uptime", "whoami"}

var (
	dangerousPatterns []*regexp.Regexp
	sensitivePatterns []*regexp.Regexp
	readOnlySet       map[string]struct{}
)

func init() {
	for _, pattern := range DangerousCommands {
		compiled, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			compiled, _ = regexp.Compile("(?i)" + regexp.QuoteMeta(pattern))
		}
		dangerousPatterns = append(dangerousPatterns, compiled)
	}

	for _, pattern := range SensitiveCommands {
		compiled, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			compiled, _ = regexp.Compile("(?i)" + regexp.QuoteMeta(pattern))
		}
		sensitivePatterns = append(sensitivePatterns, compiled)
	}

	readOnlySet = make(map[string]struct{}, len(ReadOnlyCommands))
	for _, cmd := range ReadOnlyCommands {
		readOnlySet[cmd] = struct{}{}
	}
}

// CommandCheckResult is the return type of CheckCommand.
type CommandCheckResult struct {
	Allowed      bool   `json:"allowed"`
	Blocked      bool   `json:"blocked"`
	ViewerOnly   bool   `json:"viewer_only"`
	NeedsConfirm bool   `json:"needs_confirm"`
	Reason       string `json:"reason"`
	Command      string `json:"command,omitempty"`
}

// CheckCommand inspects input against the command filter rules.
// Returns a CommandCheckResult indicating whether the input should be forwarded.
func CheckCommand(input, role string) CommandCheckResult {
	decoded := input
	trimmed := strings.TrimSpace(decoded)

	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(trimmed) {
			return CommandCheckResult{
				Allowed: false,
				Blocked: true,
				Reason:  "dangerous_command",
			}
		}
	}

	if role == "viewer" {
		cmd := extractCommand(trimmed)
		if _, ok := readOnlySet[cmd]; !ok {
			return CommandCheckResult{
				Allowed:    false,
				Blocked:    true,
				ViewerOnly: true,
				Reason:     "readonly_violation",
			}
		}
		return CommandCheckResult{Allowed: true}
	}

	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(trimmed) {
			return CommandCheckResult{
				Allowed:      true,
				NeedsConfirm: true,
				Reason:       "sensitive_command",
				Command:      trimmed,
			}
		}
	}

	return CommandCheckResult{Allowed: true}
}

func extractCommand(input string) string {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return ""
	}
	cmd := parts[0]
	if idx := strings.LastIndex(cmd, "/"); idx >= 0 {
		cmd = cmd[idx+1:]
	}
	return strings.ToLower(cmd)
}

// DangerousCommandError is returned when a command is blocked.
type DangerousCommandError struct {
	Reason string
	Detail string
}

func (e *DangerousCommandError) Error() string {
	return fmt.Sprintf("command blocked: %s", e.Reason)
}
