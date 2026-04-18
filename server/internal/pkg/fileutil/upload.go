package fileutil

import (
	"path/filepath"
	"strings"
)

var AllowedUploadExtensions = []string{
	".py", ".js", ".ts", ".go", ".sh", ".yaml", ".yml", ".json", ".txt", ".md",
	".c", ".h", ".cpp", ".rs", ".java", ".html", ".css", ".sql", ".lua", ".rb",
	".php", ".conf", ".cfg", ".env", ".ini", ".toml", ".xml", ".log", ".csv",
}

func IsAllowedUploadType(filename string) bool {
	ext := filepath.Ext(filename)
	extLower := strings.ToLower(ext)

	if extLower == "" {
		return true
	}

	for _, allowed := range AllowedUploadExtensions {
		if strings.ToLower(allowed) == extLower {
			return true
		}
	}

	return false
}
