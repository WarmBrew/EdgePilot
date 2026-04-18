package fileop

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/robot-remote-maint/agent/pkg/logger"
)

func testLogger() *logger.Logger {
	return logger.New("error")
}

func TestNewFileHandler(t *testing.T) {
	log := testLogger()
	h := NewFileHandler(log)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.log != log {
		t.Error("expected log to be set")
	}
}

func TestIsProtectedPath(t *testing.T) {
	h := &FileHandler{log: testLogger()}

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
		{"tmp dir", "/tmp/test", false},
		{"usr dir", "/usr/local/bin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.isProtectedPath(tt.path)
			if result != tt.expected {
				t.Errorf("isProtectedPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	h := &FileHandler{log: testLogger()}

	t.Run("empty path errors", func(t *testing.T) {
		err := h.validatePath("")
		if err == nil {
			t.Error("expected error for empty path")
		}
	})

	t.Run("path traversal blocked", func(t *testing.T) {
		err := h.validatePath("/tmp/../../../etc/shadow")
		if err == nil {
			t.Error("expected error for path traversal")
		}
	})

	t.Run("protected path blocked", func(t *testing.T) {
		err := h.validatePath("/etc/shadow")
		if err == nil {
			t.Error("expected error for protected path")
		}
	})

	t.Run("normal path passes", func(t *testing.T) {
		err := h.validatePath("/tmp/test.txt")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestListDirectory(t *testing.T) {
	h := &FileHandler{log: testLogger()}

	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("world"), 0o644)

	t.Run("lists directory contents", func(t *testing.T) {
		files, err := h.ListDirectory(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(files) != 3 {
			t.Errorf("expected 3 entries, got %d", len(files))
		}
	})

	t.Run("directories come first", func(t *testing.T) {
		files, err := h.ListDirectory(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		dirFound := false
		for _, f := range files {
			if f.Type == "dir" {
				dirFound = true
				break
			}
			if f.Type == "file" {
				break
			}
		}
		if !dirFound {
			t.Error("expected directory to appear first")
		}
	})

	t.Run("symlink detected", func(t *testing.T) {
		target := filepath.Join(tmpDir, "file1.txt")
		linkPath := filepath.Join(tmpDir, "link.txt")
		os.Symlink(target, linkPath)

		files, err := h.ListDirectory(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var found bool
		for _, f := range files {
			if f.Name == "link.txt" && f.IsSymlink {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected symlink to be detected")
		}
	})

	t.Run("nonexistent directory errors", func(t *testing.T) {
		_, err := h.ListDirectory(filepath.Join(tmpDir, "nonexistent"))
		if err == nil {
			t.Error("expected error for nonexistent directory")
		}
	})

	t.Run("protected path blocked", func(t *testing.T) {
		_, err := h.ListDirectory("/etc/shadow")
		if err == nil {
			t.Error("expected error for protected path")
		}
	})
}

func TestReadFile(t *testing.T) {
	h := &FileHandler{log: testLogger()}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0o644)

	t.Run("reads text file", func(t *testing.T) {
		resp, err := h.ReadFile(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.IsBinary {
			t.Error("expected text file to not be binary")
		}

		decoded, err := base64.StdEncoding.DecodeString(resp.Content)
		if err != nil {
			t.Fatalf("failed to decode content: %v", err)
		}

		if string(decoded) != "hello world" {
			t.Errorf("expected 'hello world', got %q", string(decoded))
		}
	})

	t.Run("detects binary file", func(t *testing.T) {
		binFile := filepath.Join(tmpDir, "binary.bin")
		binData := []byte{0x00, 0x01, 0x02, 0x03}
		os.WriteFile(binFile, binData, 0o644)

		resp, err := h.ReadFile(binFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !resp.IsBinary {
			t.Error("expected binary file to be detected")
		}
	})

	t.Run("nonexistent file errors", func(t *testing.T) {
		_, err := h.ReadFile(filepath.Join(tmpDir, "nonexistent"))
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("protected path blocked", func(t *testing.T) {
		_, err := h.ReadFile("/etc/shadow")
		if err == nil {
			t.Error("expected error for protected path")
		}
	})
}

func TestWriteFile(t *testing.T) {
	h := &FileHandler{log: testLogger()}

	tmpDir := t.TempDir()

	t.Run("writes new file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "new.txt")
		content := base64.StdEncoding.EncodeToString([]byte("new content"))

		err := h.WriteFile(testFile, content, "0644")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read written file: %v", err)
		}

		if string(data) != "new content" {
			t.Errorf("expected 'new content', got %q", string(data))
		}
	})

	t.Run("creates backup before overwrite", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "overwrite.txt")
		os.WriteFile(testFile, []byte("original"), 0o644)

		content := base64.StdEncoding.EncodeToString([]byte("new"))
		err := h.WriteFile(testFile, content, "0644")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		backupData, err := os.ReadFile(testFile + ".bak")
		if err != nil {
			t.Fatalf("backup was not created: %v", err)
		}

		if string(backupData) != "original" {
			t.Errorf("backup should contain 'original', got %q", string(backupData))
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "deep", "nested", "file.txt")
		content := base64.StdEncoding.EncodeToString([]byte("nested"))

		err := h.WriteFile(testFile, content, "0644")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read nested file: %v", err)
		}

		if string(data) != "nested" {
			t.Errorf("expected 'nested', got %q", string(data))
		}
	})

	t.Run("invalid base64 errors", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "invalid.txt")
		err := h.WriteFile(testFile, "not-valid-base64!", "0644")
		if err == nil {
			t.Error("expected error for invalid base64")
		}
	})

	t.Run("protected path blocked", func(t *testing.T) {
		content := base64.StdEncoding.EncodeToString([]byte("malicious"))
		err := h.WriteFile("/etc/shadow", content, "0644")
		if err == nil {
			t.Error("expected error for protected path")
		}
	})
}

func TestDeleteFile(t *testing.T) {
	h := &FileHandler{log: testLogger()}

	tmpDir := t.TempDir()

	t.Run("deletes file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "todelete.txt")
		os.WriteFile(testFile, []byte("delete me"), 0o644)

		err := h.DeleteFile(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("file should have been deleted")
		}
	})

	t.Run("deletes directory", func(t *testing.T) {
		testDir := filepath.Join(tmpDir, "todeletedir")
		os.MkdirAll(testDir, 0o755)
		os.WriteFile(filepath.Join(testDir, "nested.txt"), []byte("nested"), 0o644)

		err := h.DeleteFile(testDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(testDir); !os.IsNotExist(err) {
			t.Error("directory should have been deleted")
		}
	})

	t.Run("nonexistent file errors", func(t *testing.T) {
		err := h.DeleteFile(filepath.Join(tmpDir, "nonexistent"))
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("protected path blocked", func(t *testing.T) {
		err := h.DeleteFile("/etc/shadow")
		if err == nil {
			t.Error("expected error for protected path")
		}
	})
}

func TestGetFileInfo(t *testing.T) {
	h := &FileHandler{log: testLogger()}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "info.txt")
	os.WriteFile(testFile, []byte("content"), 0o644)

	t.Run("gets file info", func(t *testing.T) {
		info, err := h.GetFileInfo(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if info.Name != "info.txt" {
			t.Errorf("expected name 'info.txt', got %q", info.Name)
		}

		if info.Type != "file" {
			t.Errorf("expected type 'file', got %q", info.Type)
		}

		if info.Size != 7 {
			t.Errorf("expected size 7, got %d", info.Size)
		}
	})

	t.Run("directory info", func(t *testing.T) {
		info, err := h.GetFileInfo(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if info.Type != "dir" {
			t.Errorf("expected type 'dir', got %q", info.Type)
		}
	})

	t.Run("symlink info", func(t *testing.T) {
		linkPath := filepath.Join(tmpDir, "link.txt")
		os.Symlink(testFile, linkPath)

		info, err := h.GetFileInfo(linkPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !info.IsSymlink {
			t.Error("expected symlink to be detected")
		}
	})

	t.Run("nonexistent file errors", func(t *testing.T) {
		_, err := h.GetFileInfo(filepath.Join(tmpDir, "nonexistent"))
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("protected path blocked", func(t *testing.T) {
		_, err := h.GetFileInfo("/etc/shadow")
		if err == nil {
			t.Error("expected error for protected path")
		}
	})
}

func TestChangePermission(t *testing.T) {
	h := &FileHandler{log: testLogger()}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "chmod.txt")
	os.WriteFile(testFile, []byte("test"), 0o644)

	t.Run("changes permission", func(t *testing.T) {
		err := h.ChangePermission(testFile, "0755")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		info, err := os.Stat(testFile)
		if err != nil {
			t.Fatalf("failed to stat: %v", err)
		}

		if info.Mode().Perm() != 0o755 {
			t.Errorf("expected mode 0755, got %v", info.Mode().Perm())
		}
	})

	t.Run("empty mode errors", func(t *testing.T) {
		err := h.ChangePermission(testFile, "")
		if err == nil {
			t.Error("expected error for empty mode")
		}
	})

	t.Run("invalid mode errors", func(t *testing.T) {
		err := h.ChangePermission(testFile, "invalid")
		if err == nil {
			t.Error("expected error for invalid mode")
		}
	})

	t.Run("protected path blocked", func(t *testing.T) {
		err := h.ChangePermission("/etc/shadow", "0755")
		if err == nil {
			t.Error("expected error for protected path")
		}
	})
}

func TestChangeOwnership(t *testing.T) {
	h := &FileHandler{log: testLogger()}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "chown.txt")
	os.WriteFile(testFile, []byte("test"), 0o644)

	t.Run("empty owner and group errors", func(t *testing.T) {
		err := h.ChangeOwnership(testFile, "", "")
		if err == nil {
			t.Error("expected error when both owner and group are empty")
		}
	})

	t.Run("nonexistent user errors", func(t *testing.T) {
		err := h.ChangeOwnership(testFile, "nonexistent_user_xyz", "")
		if err == nil {
			t.Error("expected error for nonexistent user")
		}
	})

	t.Run("nonexistent group errors", func(t *testing.T) {
		err := h.ChangeOwnership(testFile, "", "nonexistent_group_xyz")
		if err == nil {
			t.Error("expected error for nonexistent group")
		}
	})

	t.Run("protected path blocked", func(t *testing.T) {
		err := h.ChangeOwnership("/etc/shadow", "root", "root")
		if err == nil {
			t.Error("expected error for protected path")
		}
	})
}

func TestParseMode(t *testing.T) {
	h := &FileHandler{log: testLogger()}

	tests := []struct {
		name     string
		mode     string
		expected os.FileMode
	}{
		{"empty defaults to 0644", "", 0o644},
		{"0644", "0644", 0o644},
		{"0755", "0755", 0o755},
		{"644 without prefix", "644", 0o644},
		{"invalid falls back", "invalid", 0o644},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.parseMode(tt.mode)
			if result != tt.expected {
				t.Errorf("parseMode(%q) = %v, want %v", tt.mode, result, tt.expected)
			}
		})
	}
}

func TestIsBinary(t *testing.T) {
	h := &FileHandler{log: testLogger()}

	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{"empty data", []byte{}, false},
		{"text data", []byte("hello world"), false},
		{"null byte", []byte{0x00, 0x01, 0x02}, true},
		{"utf8 text", []byte("你好世界"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.isBinary(tt.data)
			if result != tt.expected {
				t.Errorf("isBinary(%v) = %v, want %v", tt.data, result, tt.expected)
			}
		})
	}
}
