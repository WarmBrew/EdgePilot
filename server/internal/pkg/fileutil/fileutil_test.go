package fileutil

import (
	"os"
	"testing"
)

func TestDetectMimeType(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{"python file", "test.py", "text/x-python"},
		{"go file", "main.go", "text/x-go"},
		{"rust file", "lib.rs", "text/x-rust"},
		{"json file", "config.json", "application/json"},
		{"yaml file", "docker-compose.yaml", "text/x-yaml"},
		{"yml file", "action.yml", "text/x-yaml"},
		{"markdown file", "README.md", "text/markdown"},
		{"shell script", "setup.sh", "text/x-shellscript"},
		{"png image", "logo.png", "image/png"},
		{"jpeg image", "photo.jpg", "image/jpeg"},
		{"svg image", "icon.svg", "image/svg+xml"},
		{"csv file", "data.csv", "text/csv"},
		{"xml file", "config.xml", "text/xml"},
		{"binary file", "program.bin", "application/octet-stream"},
		{"executable", "app.exe", "application/x-msdos-program"},
		{"shared lib", "lib.so", "text/plain"},
		{"tarball", "archive.tar.gz", "application/gzip"},
		{"zip file", "archive.zip", "application/zip"},
		{"unknown extension", "file.xyz", "text/plain"},
		{"empty path", "", "application/octet-stream"},
		{"no extension", "Makefile", "text/x-makefile"},
		{"dockerfile", "Dockerfile", "text/x-dockerfile"},
		{"env file", ".env", "text/plain"},
		{"toml file", "Cargo.toml", "text/x-toml"},
		{"proto file", "service.proto", "text/x-protobuf"},
		{"graphql file", "schema.graphql", "application/graphql"},
		{"log file", "app.log", "text/plain"},
		{"ini file", "settings.ini", "text/x-ini"},
		{"conf file", "nginx.conf", "text/plain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectMimeType(tt.filePath)
			if result != tt.expected {
				t.Errorf("DetectMimeType(%q) = %q, want %q", tt.filePath, result, tt.expected)
			}
		})
	}
}

func TestValidateFileMode(t *testing.T) {
	tests := []struct {
		name          string
		mode          string
		expectError   bool
		errorContains string
	}{
		{"empty mode", "", false, ""},
		{"normal file mode", "0644", false, ""},
		{"executable mode", "0755", false, ""},
		{"read only", "0444", false, ""},
		{"owner read write", "0600", false, ""},
		{"owner read write exec", "0700", false, ""},
		{"world writable", "0666", true, "too permissive"},
		{"full open", "0777", true, "too permissive"},
		{"world writable exec", "0766", true, "too permissive"},
		{"group world write", "0676", true, "world-writable"},
		{"octal invalid", "0999", true, "not a valid octal value"},
		{"letters in mode", "0abc", true, "not a valid octal value"},
		{"setuid bit", "4755", true, "setuid/setgid"},
		{"setgid bit", "2755", true, "setuid/setgid"},
		{"world readable only", "0444", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFileMode(tt.mode)
			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateFileMode(%q) expected error, got nil", tt.mode)
					return
				}
				if tt.errorContains != "" && !containsStr(err.Error(), tt.errorContains) {
					t.Errorf("ValidateFileMode(%q) error = %q, want containing %q", tt.mode, err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateFileMode(%q) unexpected error: %v", tt.mode, err)
				}
			}
		})
	}
}

func TestNormalizeFileMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		expected os.FileMode
	}{
		{"empty defaults to 0644", "", 0o644},
		{"valid 0644", "0644", 0o644},
		{"valid 0755", "0755", 0o755},
		{"invalid falls back", "0666", 0o644},
		{"invalid octal falls back", "0999", 0o644},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeFileMode(tt.mode)
			if result != tt.expected {
				t.Errorf("NormalizeFileMode(%q) = %v, want %v", tt.mode, result, tt.expected)
			}
		})
	}
}

func TestIsSensitivePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"shadow file", "/etc/shadow", true},
		{"passwd file", "/etc/passwd", true},
		{"sudoers", "/etc/sudoers", true},
		{"ssh dir", "/.ssh", true},
		{"ssh dir child", "/.ssh/authorized_keys", true},
		{"root ssh", "/root/.ssh", true},
		{"root home", "/root/somefile", true},
		{"root bashrc", "/root/.bashrc", true},
		{"proc", "/proc", true},
		{"proc child", "/proc/1/fd", true},
		{"sys", "/sys", true},
		{"sys child", "/sys/class", true},
		{"dev", "/dev", true},
		{"boot", "/boot", true},
		{"fstab", "/etc/fstab", true},
		{"crontab", "/etc/crontab", true},
		{"auth log", "/var/log/auth.log", true},
		{"normal home", "/home/user/file.txt", false},
		{"normal var", "/var/www/html", false},
		{"normal opt", "/opt/app/config.json", false},
		{"empty path", "", false},
		{"tmp dir", "/tmp/test", false},
		{"usr dir", "/usr/local/bin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSensitivePath(tt.path)
			if result != tt.expected {
				t.Errorf("IsSensitivePath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	realFile := tmpDir + "/real.txt"
	if err := os.WriteFile(realFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkPath := tmpDir + "/link.txt"
	if err := os.Symlink(realFile, linkPath); err != nil {
		t.Fatal(err)
	}

	t.Run("real file is not symlink", func(t *testing.T) {
		isLink, err := IsSymlink(realFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isLink {
			t.Error("expected real file to not be a symlink")
		}
	})

	t.Run("symlink is detected", func(t *testing.T) {
		isLink, err := IsSymlink(linkPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isLink {
			t.Error("expected symlink to be detected")
		}
	})

	t.Run("nonexistent path errors", func(t *testing.T) {
		_, err := IsSymlink(tmpDir + "/nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent path")
		}
	})

	t.Run("empty path errors", func(t *testing.T) {
		_, err := IsSymlink("")
		if err == nil {
			t.Error("expected error for empty path")
		}
	})
}

func TestResolveSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	realFile := tmpDir + "/real.txt"
	if err := os.WriteFile(realFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkPath := tmpDir + "/link.txt"
	if err := os.Symlink(realFile, linkPath); err != nil {
		t.Fatal(err)
	}

	t.Run("resolves valid symlink", func(t *testing.T) {
		resolved, err := ResolveSymlink(linkPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolved != realFile {
			t.Errorf("expected %q, got %q", realFile, resolved)
		}
	})

	t.Run("empty path errors", func(t *testing.T) {
		_, err := ResolveSymlink("")
		if err == nil {
			t.Error("expected error for empty path")
		}
	})

	t.Run("nonexistent link errors", func(t *testing.T) {
		_, err := ResolveSymlink(tmpDir + "/broken_link")
		if err == nil {
			t.Error("expected error for broken symlink")
		}
	})

	t.Run("sensitive target blocked", func(t *testing.T) {
		sensitiveLink := tmpDir + "/shadow_link"
		if err := os.Symlink("/etc/shadow", sensitiveLink); err != nil {
			t.Skip("could not create test symlink, skipping")
		}
		defer os.Remove(sensitiveLink)

		_, err := ResolveSymlink(sensitiveLink)
		if err == nil {
			t.Error("expected error for symlink to sensitive path")
		}
		if err != nil && !containsStr(err.Error(), "sensitive") {
			t.Errorf("expected sensitive path error, got: %v", err)
		}
	})
}

func TestIsSymlinkTargetSafe(t *testing.T) {
	tmpDir := t.TempDir()

	realFile := tmpDir + "/safe.txt"
	if err := os.WriteFile(realFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	safeLink := tmpDir + "/safe_link"
	if err := os.Symlink(realFile, safeLink); err != nil {
		t.Fatal(err)
	}

	t.Run("safe symlink", func(t *testing.T) {
		safe, resolved, err := IsSymlinkTargetSafe(safeLink)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !safe {
			t.Error("expected symlink to be safe")
		}
		if resolved == "" {
			t.Error("expected resolved path")
		}
	})

	t.Run("empty path errors", func(t *testing.T) {
		_, _, err := IsSymlinkTargetSafe("")
		if err == nil {
			t.Error("expected error for empty path")
		}
	})

	t.Run("sensitive target unsafe", func(t *testing.T) {
		sensitiveLink := tmpDir + "/shadow_link"
		if err := os.Symlink("/etc/shadow", sensitiveLink); err != nil {
			t.Skip("could not create test symlink, skipping")
		}
		defer os.Remove(sensitiveLink)

		safe, _, err := IsSymlinkTargetSafe(sensitiveLink)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if safe {
			t.Error("expected sensitive symlink to be unsafe")
		}
	})
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

func TestIsAllowedUploadType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"python file", "test.py", true},
		{"go file", "main.go", true},
		{"typescript file", "app.ts", true},
		{"js file", "index.js", true},
		{"json file", "config.json", true},
		{"yaml file", "config.yaml", true},
		{"yml file", "action.yml", true},
		{"markdown file", "README.md", true},
		{"rust file", "lib.rs", true},
		{"c file", "main.c", true},
		{"h file", "header.h", true},
		{"html file", "index.html", true},
		{"css file", "style.css", true},
		{"sql file", "query.sql", true},
		{"shell file", "run.sh", true},
		{"xml file", "data.xml", true},
		{"log file", "app.log", true},
		{"csv file", "data.csv", true},
		{"env file", ".env", true},
		{"toml file", "Cargo.toml", true},
		{"ini file", "settings.ini", true},
		{"conf file", "nginx.conf", true},
		{"cfg file", "app.cfg", true},
		{"file without extension", "Makefile", true},
		{"binary file", "program.exe", false},
		{"dll file", "lib.dll", false},
		{"so file", "lib.so", false},
		{"tar file", "archive.tar", false},
		{"gz file", "archive.gz", false},
		{"zip file", "files.zip", false},
		{"pdf file", "doc.pdf", false},
		{"png file", "image.png", false},
		{"jpg file", "photo.jpg", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAllowedUploadType(tt.filename)
			if result != tt.expected {
				t.Errorf("IsAllowedUploadType(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestDetectMimeTypeTextFile(t *testing.T) {
	tests := []struct {
		mimeType string
		expected bool
	}{
		{"text/plain", true},
		{"text/x-python", true},
		{"text/html", true},
		{"text/markdown", true},
		{"application/json", true},
		{"application/xml", true},
		{"application/octet-stream", false},
		{"image/png", false},
		{"video/mp4", false},
		{"application/zip", false},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			result := IsTextFile(tt.mimeType)
			if result != tt.expected {
				t.Errorf("IsTextFile(%q) = %v, want %v", tt.mimeType, result, tt.expected)
			}
		})
	}
}
