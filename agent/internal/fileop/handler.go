package fileop

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/robot-remote-maint/agent/pkg/logger"
)

var protectedPaths = []string{
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
	"/root/.bash_logout",
	"/root/.config",
	"/root/.local",
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
	"/etc/hosts",
	"/etc/resolv.conf",
	"/etc/hostname",
	"/etc/systemd",
	"/etc/security",
	"/etc/security",
	"/var/spool/cron",
	"/etc/group",
	"/etc/gshadow",
	"/var/log/auth.log",
	"/var/log/secure",
	"/etc/sudoers.d",
}

type FileHandler struct {
	log *logger.Logger
}

func NewFileHandler(log *logger.Logger) *FileHandler {
	return &FileHandler{log: log}
}

func (h *FileHandler) isProtectedPath(path string) bool {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		cleaned = "/" + cleaned
	}

	for _, protected := range protectedPaths {
		if cleaned == protected || strings.HasPrefix(cleaned, protected+"/") {
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

func (h *FileHandler) validatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("path traversal is not allowed")
	}

	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to resolve path symlinks: %w", err)
	}
	checkPath := cleaned
	if err == nil {
		checkPath = resolved
	}

	if h.isProtectedPath(checkPath) {
		return fmt.Errorf("access to protected path '%s' is not allowed", checkPath)
	}

	return nil
}

func (h *FileHandler) ListDirectory(reqPath string) ([]*FileInfo, error) {
	if err := h.validatePath(reqPath); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(reqPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []*FileInfo

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			h.log.Warn("Failed to get file info", "name", entry.Name(), "error", err)
			continue
		}

		fullPath := filepath.Join(reqPath, entry.Name())
		fileInfo := h.buildFileInfo(entry.Name(), fullPath, info)

		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(fullPath)
			if err == nil {
				fileInfo.IsSymlink = true
				fileInfo.SymlinkTarget = target
			}
		}

		files = append(files, fileInfo)
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].Type == "dir" && files[j].Type != "dir" {
			return true
		}
		if files[i].Type != "dir" && files[j].Type == "dir" {
			return false
		}
		return files[i].Name < files[j].Name
	})

	h.log.Info("Directory listed", "path", reqPath, "count", len(files))
	return files, nil
}

func (h *FileHandler) ReadFile(reqPath string) (*ReadFileResponse, error) {
	if err := h.validatePath(reqPath); err != nil {
		return nil, err
	}

	content, err := os.ReadFile(reqPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if h.isBinary(content) {
		return &ReadFileResponse{
			IsBinary: true,
			Size:     int64(len(content)),
		}, nil
	}

	encoded := base64.StdEncoding.EncodeToString(content)

	return &ReadFileResponse{
		Content:  encoded,
		Size:     int64(len(content)),
		IsBinary: false,
	}, nil
}

func (h *FileHandler) WriteFile(reqPath, content, mode string) error {
	if err := h.validatePath(reqPath); err != nil {
		return err
	}

	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return fmt.Errorf("invalid base64 content: %w", err)
	}

	fileMode := h.parseMode(mode)

	if _, err := os.Stat(reqPath); err == nil {
		backupPath := reqPath + ".bak"
		if err := os.Rename(reqPath, backupPath); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
		h.log.Info("Backup created", "path", backupPath)
	}

	parentDir := filepath.Dir(reqPath)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}

	if err := os.WriteFile(reqPath, decoded, fileMode); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	h.log.Info("File written", "path", reqPath, "bytes", len(decoded), "mode", mode)
	return nil
}

func (h *FileHandler) DeleteFile(reqPath string) error {
	if err := h.validatePath(reqPath); err != nil {
		return err
	}

	info, err := os.Stat(reqPath)
	if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	}

	if info.IsDir() {
		if err := os.RemoveAll(reqPath); err != nil {
			return fmt.Errorf("failed to remove directory: %w", err)
		}
	} else {
		if err := os.Remove(reqPath); err != nil {
			return fmt.Errorf("failed to remove file: %w", err)
		}
	}

	h.log.Info("Path deleted", "path", reqPath)
	return nil
}

func (h *FileHandler) GetFileInfo(reqPath string) (*FileInfo, error) {
	if err := h.validatePath(reqPath); err != nil {
		return nil, err
	}

	linfo, err := os.Lstat(reqPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	finfo := h.buildFileInfo(filepath.Base(reqPath), reqPath, linfo)

	if linfo.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(reqPath)
		if err == nil {
			finfo.IsSymlink = true
			finfo.SymlinkTarget = target
		}
	}

	uid := linfo.Sys().(*syscall.Stat_t).Uid
	gid := linfo.Sys().(*syscall.Stat_t).Gid

	u, err := user.LookupId(fmt.Sprintf("%d", uid))
	if err == nil {
		finfo.Owner = u.Username
	}

	g, err := user.LookupGroupId(fmt.Sprintf("%d", gid))
	if err == nil {
		finfo.Group = g.Name
	}

	return finfo, nil
}

func (h *FileHandler) ChangePermission(reqPath, mode string) error {
	if err := h.validatePath(reqPath); err != nil {
		return err
	}

	if mode == "" {
		return fmt.Errorf("mode cannot be empty")
	}

	mode = strings.TrimPrefix(mode, "0")
	if mode == "" {
		return fmt.Errorf("invalid mode")
	}

	parsed, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return fmt.Errorf("invalid octal mode: %w", err)
	}

	if err := os.Chmod(reqPath, os.FileMode(parsed)); err != nil {
		return fmt.Errorf("failed to change permission: %w", err)
	}

	h.log.Info("Permission changed", "path", reqPath, "mode", mode)
	return nil
}

func (h *FileHandler) ChangeOwnership(reqPath, owner, group string) error {
	if err := h.validatePath(reqPath); err != nil {
		return err
	}

	if owner == "" && group == "" {
		return fmt.Errorf("owner or group must be specified")
	}

	uid := -1
	gid := -1

	if owner != "" {
		u, err := user.Lookup(owner)
		if err != nil {
			return fmt.Errorf("failed to lookup user '%s': %w", owner, err)
		}
		parsed, err := strconv.Atoi(u.Uid)
		if err != nil {
			return fmt.Errorf("invalid UID for user '%s': %w", owner, err)
		}
		uid = parsed
	}

	if group != "" {
		g, err := user.LookupGroup(group)
		if err != nil {
			return fmt.Errorf("failed to lookup group '%s': %w", group, err)
		}
		parsed, err := strconv.Atoi(g.Gid)
		if err != nil {
			return fmt.Errorf("invalid GID for group '%s': %w", group, err)
		}
		gid = parsed
	}

	if err := os.Chown(reqPath, uid, gid); err != nil {
		return fmt.Errorf("failed to change ownership: %w", err)
	}

	h.log.Info("Ownership changed", "path", reqPath, "owner", owner, "group", group)
	return nil
}

func (h *FileHandler) buildFileInfo(name, fullPath string, info os.FileInfo) *FileInfo {
	fileType := "file"
	if info.IsDir() {
		fileType = "dir"
	} else if info.Mode()&os.ModeSymlink != 0 {
		fileType = "symlink"
	}

	return &FileInfo{
		Name:       name,
		Type:       fileType,
		Size:       info.Size(),
		Mode:       fmt.Sprintf("%04o", info.Mode().Perm()),
		ModifiedAt: info.ModTime(),
	}
}

func (h *FileHandler) parseMode(mode string) os.FileMode {
	if mode == "" {
		return 0o644
	}

	mode = strings.TrimPrefix(mode, "0")
	if mode == "" {
		return 0o644
	}

	parsed, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return 0o644
	}

	return os.FileMode(parsed)
}

func (h *FileHandler) isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	if len(data) > 512 {
		data = data[:512]
	}

	for _, b := range data {
		if b == 0 {
			return true
		}
	}

	return false
}
