package fileutil

import (
	"mime"
	"path/filepath"
	"strings"
)

var mimeTypeOverrides = map[string]string{
	".py":         "text/x-python",
	".pyc":        "application/octet-stream",
	".go":         "text/x-go",
	".rs":         "text/x-rust",
	".sh":         "text/x-shellscript",
	".yaml":       "text/x-yaml",
	".yml":        "text/x-yaml",
	".toml":       "text/x-toml",
	".ini":        "text/x-ini",
	".conf":       "text/plain",
	".env":        "text/plain",
	".log":        "text/plain",
	".md":         "text/markdown",
	".csv":        "text/csv",
	".xml":        "text/xml",
	".svg":        "image/svg+xml",
	".mod":        "text/plain",
	".sum":        "text/plain",
	".proto":      "text/x-protobuf",
	".graphql":    "application/graphql",
	".gitignore":  "text/plain",
	".dockerfile": "text/x-dockerfile",
	".makefile":   "text/x-makefile",
	".png":        "image/png",
	".jpg":        "image/jpeg",
	".jpeg":       "image/jpeg",
	".gif":        "image/gif",
	".ico":        "image/x-icon",
	".bmp":        "image/bmp",
	".tiff":       "image/tiff",
	".webp":       "image/webp",
	".zip":        "application/zip",
	".gz":         "application/gzip",
	".gzip":       "application/gzip",
	".pdf":        "application/pdf",
	".mp3":        "audio/mpeg",
	".mp4":        "video/mp4",
	".wav":        "audio/wav",
}

var binaryExtensions = map[string]bool{
	".bin":  true,
	".exe":  true,
	".so":   true,
	".dll":  true,
	".deb":  true,
	".rpm":  true,
	".zip":  true,
	".tar":  true,
	".gz":   true,
	".gzip": true,
	".bz2":  true,
	".xz":   true,
	".7z":   true,
	".rar":  true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".ico":  true,
	".bmp":  true,
	".tiff": true,
	".webp": true,
	".wav":  true,
	".mp3":  true,
	".mp4":  true,
	".avi":  true,
	".mov":  true,
	".pdf":  true,
	".doc":  true,
	".docx": true,
	".xls":  true,
	".xlsx": true,
	".ppt":  true,
	".pptx": true,
	".o":    true,
	".a":    true,
	".wasm": true,
}

func DetectMimeType(filePath string) string {
	if filePath == "" {
		return "application/octet-stream"
	}

	base := filepath.Base(filePath)
	ext := strings.ToLower(filepath.Ext(filePath))

	if base == "Dockerfile" {
		return "text/x-dockerfile"
	}
	if base == "Makefile" || strings.HasPrefix(base, "Makefile") {
		return "text/x-makefile"
	}

	if mt, ok := mimeTypeOverrides[ext]; ok {
		return mt
	}

	if mt := mime.TypeByExtension(ext); mt != "" {
		if strings.HasPrefix(mt, "chemical/") || strings.HasPrefix(mt, "x-") {
			return "text/plain"
		}
		return mt
	}

	return "text/plain"
}

func IsTextFile(mimeType string) bool {
	return strings.HasPrefix(mimeType, "text/") ||
		mimeType == "application/json" ||
		mimeType == "application/xml"
}
