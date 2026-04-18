package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
)

func ResolveSymlink(linkPath string) (string, error) {
	if linkPath == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	resolved, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlink '%s': %w", linkPath, err)
	}

	if IsSensitivePath(resolved) {
		return "", fmt.Errorf("symlink '%s' resolves to sensitive path '%s', access denied", linkPath, resolved)
	}

	return resolved, nil
}

func IsSymlink(path string) (bool, error) {
	if path == "" {
		return false, fmt.Errorf("path cannot be empty")
	}

	info, err := os.Lstat(path)
	if err != nil {
		return false, fmt.Errorf("failed to stat path '%s': %w", path, err)
	}

	return info.Mode()&os.ModeSymlink != 0, nil
}

func IsSymlinkTargetSafe(linkPath string) (bool, string, error) {
	if linkPath == "" {
		return false, "", fmt.Errorf("path cannot be empty")
	}

	target, err := os.Readlink(linkPath)
	if err != nil {
		return false, "", fmt.Errorf("failed to read symlink '%s': %w", linkPath, err)
	}

	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(linkPath), target)
	}

	resolved, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		return false, target, fmt.Errorf("failed to resolve symlink target: %w", err)
	}

	if IsSensitivePath(resolved) {
		return false, resolved, nil
	}

	return true, resolved, nil
}
