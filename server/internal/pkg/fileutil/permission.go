package fileutil

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

var forbiddenModes = map[os.FileMode]bool{
	0o777: true,
	0o666: true,
	0o766: true,
	0o677: true,
	0o757: true,
	0o775: true,
}

var defaultFileMode = os.FileMode(0o644)

var sensitivePaths = []string{
	"/etc/shadow",
	"/etc/passwd",
	"/etc/sudoers",
	"/etc/gshadow",
	"/etc/ssh",
	"/.ssh",
	"/root/.ssh",
	"/root/.bash_history",
	"/root/.profile",
	"/root/.bashrc",
	"/etc/ssl",
	"/etc/pki",
	"/etc/crypttab",
	"/proc",
	"/sys",
	"/dev",
	"/boot",
	"/etc/fstab",
	"/etc/crontab",
	"/etc/cron.d",
	"/var/log/auth.log",
	"/var/log/secure",
	"/etc/sudoers.d",
}

func ValidateFileMode(mode string) error {
	if mode == "" {
		return nil
	}

	mode = strings.TrimPrefix(mode, "0")
	if mode == "" {
		return nil
	}

	parsed, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return fmt.Errorf("invalid file mode '%s': not a valid octal value", mode)
	}

	fileMode := os.FileMode(parsed)

	if forbiddenModes[fileMode&0o7777] {
		return fmt.Errorf("file mode '%s' is too permissive and is not allowed", mode)
	}

	worldWrite := fileMode & 0o002
	if worldWrite != 0 {
		return fmt.Errorf("file mode '%s' has world-writable bit set, which is not allowed", mode)
	}

	if fileMode&os.ModeSetuid != 0 {
		return fmt.Errorf("file mode '%s' has setuid bit set, which is not allowed", mode)
	}
	if fileMode&os.ModeSetgid != 0 {
		return fmt.Errorf("file mode '%s' has setgid bit set, which is not allowed", mode)
	}

	if fileMode&(1<<11) != 0 || fileMode&(1<<10) != 0 {
		return fmt.Errorf("file mode '%s' has setuid/setgid/sticky bit set, which is not allowed", mode)
	}

	return nil
}

func NormalizeFileMode(mode string) os.FileMode {
	if mode == "" {
		return defaultFileMode
	}

	if err := ValidateFileMode(mode); err != nil {
		return defaultFileMode
	}

	mode = strings.TrimPrefix(mode, "0")
	parsed, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return defaultFileMode
	}

	return os.FileMode(parsed)
}

func IsSensitivePath(targetPath string) bool {
	if targetPath == "" {
		return false
	}

	cleaned := targetPath
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}

	for _, sensitive := range sensitivePaths {
		if cleaned == sensitive || strings.HasPrefix(cleaned, sensitive+"/") {
			return true
		}
	}

	if strings.HasPrefix(cleaned, "/root/") {
		return true
	}

	if strings.HasPrefix(cleaned, "/.ssh/") {
		return true
	}

	return false
}

func LogPermissionChange(path string, oldMode, newMode os.FileMode, userID string) {
	slog := getLogger()
	if slog != nil {
		slog.Info("file permission changed",
			"path", path,
			"old_mode", oldMode.String(),
			"new_mode", newMode.String(),
			"user_id", userID,
		)
	}
}

func getLogger() interface{ Info(string, ...any) } {
	return nil
}
