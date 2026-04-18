package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/edge-platform/server/internal/domain/models"
	"github.com/edge-platform/server/internal/pkg/fileutil"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/edge-platform/server/internal/websocket"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	fileOpTimeout       = 10 * time.Second
	maxFileSize         = 100 * 1024 * 1024 // 100MB
	downloadTokenTTL    = 10 * time.Minute
	downloadTokenPrefix = "download:"
	fileVersionPrefix   = "file:version:"
	fileVersionTTL      = 24 * time.Hour
)

var allowedFileExtensions = map[string]bool{
	".py":      true,
	".js":      true,
	".yaml":    true,
	".yml":     true,
	".json":    true,
	".txt":     true,
	".sh":      true,
	".go":      true,
	".rs":      true,
	".c":       true,
	".cpp":     true,
	".h":       true,
	".hpp":     true,
	".md":      true,
	".csv":     true,
	".xml":     true,
	".html":    true,
	".css":     true,
	".ts":      true,
	".tsx":     true,
	".vue":     true,
	".java":    true,
	".rb":      true,
	".php":     true,
	".sql":     true,
	".toml":    true,
	".ini":     true,
	".conf":    true,
	".env":     true,
	".log":     true,
	".cfg":     true,
	".mod":     true,
	".sum":     true,
	".proto":   true,
	".graphql": true,
	".svg":     true,
	".png":     true,
	".jpg":     true,
	".jpeg":    true,
	".gif":     true,
	".ico":     true,
	".woff":    true,
	".woff2":   true,
	".ttf":     true,
	".zip":     true,
	".tar":     true,
	".gz":      true,
	".tar.gz":  true,
	".bin":     true,
	".exe":     true,
	".so":      true,
	".dll":     true,
	".deb":     true,
	".rpm":     true,
}

var protectedPaths = []string{
	"/etc/shadow",
	"/etc/passwd",
	"/etc/sudoers",
	"/etc/ssh",
	"/.ssh",
	"/root/.ssh",
	"/etc/ssl",
	"/etc/pki",
	"/proc",
	"/sys",
	"/dev",
	"/boot",
	"/etc/fstab",
	"/etc/crontab",
	"/var/log/auth.log",
	"/var/log/secure",
}

type FileInfo struct {
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	Size          int64     `json:"size"`
	Mode          string    `json:"mode"`
	ModifiedAt    time.Time `json:"modified_at"`
	IsSymlink     bool      `json:"is_symlink,omitempty"`
	SymlinkTarget string    `json:"symlink_target,omitempty"`
}

type DetailedFileInfo struct {
	Name          string    `json:"name"`
	Path          string    `json:"path"`
	Size          int64     `json:"size"`
	Mode          string    `json:"mode"`
	Owner         string    `json:"owner"`
	Group         string    `json:"group"`
	ModifiedAt    time.Time `json:"modified_at"`
	IsDir         bool      `json:"is_dir"`
	IsSymlink     bool      `json:"is_symlink"`
	SymlinkTarget string    `json:"symlink_target,omitempty"`
}

type ListDirResponse struct {
	Files []FileInfo `json:"files"`
	Path  string     `json:"path"`
	Total int        `json:"total"`
	Page  int        `json:"page"`
	Size  int        `json:"page_size"`
}

type FileContentResponse struct {
	Content  string `json:"content"`
	Mimetype string `json:"mimetype"`
	Size     int64  `json:"size"`
}

type WriteFileResponse struct {
	Success    bool `json:"success"`
	NewVersion int  `json:"new_version"`
}

type UploadResponse struct {
	UploadID     string `json:"upload_id"`
	BytesWritten int64  `json:"bytes_written"`
	Success      bool   `json:"success"`
}

type DownloadTokenData struct {
	DeviceID string `json:"device_id"`
	Path     string `json:"path"`
	UserID   string `json:"user_id"`
}

type FileService struct {
	db    *gorm.DB
	redis *pkgRedis.RedisClient
	gw    *websocket.Gateway
}

func NewFileService(db *gorm.DB, redis *pkgRedis.RedisClient, gw *websocket.Gateway) *FileService {
	return &FileService{
		db:    db,
		redis: redis,
		gw:    gw,
	}
}

func (s *FileService) sanitizePath(userPath string) (string, error) {
	if userPath == "" {
		userPath = "/"
	}

	cleaned := path.Clean(userPath)

	if strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("path traversal is not allowed")
	}

	if !path.IsAbs(cleaned) {
		cleaned = "/" + cleaned
	}

	for _, protected := range protectedPaths {
		if cleaned == protected || strings.HasPrefix(cleaned, protected+"/") {
			return "", fmt.Errorf("access to protected path '%s' is not allowed", protected)
		}
	}

	if fileutil.IsSensitivePath(cleaned) {
		return "", fmt.Errorf("access to sensitive path '%s' is not allowed", cleaned)
	}

	return cleaned, nil
}

func (s *FileService) validateDeviceOnline(ctx context.Context, deviceID string) error {
	if !s.gw.IsDeviceOnline(deviceID) {
		return fmt.Errorf("device %s is not online", deviceID)
	}
	return nil
}

func (s *FileService) ListDir(ctx context.Context, userID, deviceID, targetPath string, page, pageSize int) (*ListDirResponse, error) {
	if err := s.validateDeviceOnline(ctx, deviceID); err != nil {
		return nil, err
	}

	cleanedPath, err := s.sanitizePath(targetPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	reqPayload := map[string]interface{}{
		"path": cleanedPath,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, fileOpTimeout)
	defer cancel()

	sessionID := uuid.New().String()

	resp, err := s.gw.SendAndWait(timeoutCtx, deviceID, websocket.MessageTypeListDir, reqPayload, sessionID, fileOpTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	var rawResp struct {
		Files []struct {
			Name          string `json:"name"`
			Type          string `json:"type"`
			Size          int64  `json:"size"`
			Mode          string `json:"mode"`
			ModifiedAt    string `json:"modified_at"`
			IsSymlink     bool   `json:"is_symlink,omitempty"`
			SymlinkTarget string `json:"symlink_target,omitempty"`
		} `json:"files"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(resp.Payload, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to parse directory listing response: %w", err)
	}

	totalFiles := len(rawResp.Files)
	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize

	var pagedFiles []FileInfo
	if startIdx < totalFiles {
		if endIdx > totalFiles {
			endIdx = totalFiles
		}
		for _, f := range rawResp.Files[startIdx:endIdx] {
			modifiedAt := time.Time{}
			if f.ModifiedAt != "" {
				if t, err := time.Parse(time.RFC3339, f.ModifiedAt); err == nil {
					modifiedAt = t
				}
			}

			entry := FileInfo{
				Name:          f.Name,
				Type:          f.Type,
				Size:          f.Size,
				Mode:          f.Mode,
				ModifiedAt:    modifiedAt,
				IsSymlink:     f.IsSymlink,
				SymlinkTarget: f.SymlinkTarget,
			}

			if f.IsSymlink && f.SymlinkTarget != "" {
				if fileutil.IsSensitivePath(f.SymlinkTarget) {
					slog.Warn("blocked access to symlink targeting sensitive path",
						"name", f.Name,
						"target", f.SymlinkTarget)
					entry.SymlinkTarget = "[REDACTED - sensitive target]"
				}
			}

			pagedFiles = append(pagedFiles, entry)
		}
	}

	s.writeAuditLog(ctx, userID, deviceID, models.ActionFileList, map[string]interface{}{
		"path":      cleanedPath,
		"page":      page,
		"page_size": pageSize,
		"total":     totalFiles,
	})

	return &ListDirResponse{
		Files: pagedFiles,
		Path:  rawResp.Path,
		Total: totalFiles,
		Page:  page,
		Size:  pageSize,
	}, nil
}

func (s *FileService) GetFileContent(ctx context.Context, userID, deviceID, filePath string) (*FileContentResponse, string, error) {
	if err := s.validateDeviceOnline(ctx, deviceID); err != nil {
		return nil, "", err
	}

	cleanedPath, err := s.sanitizePath(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("invalid path: %w", err)
	}

	reqPayload := map[string]interface{}{
		"path": cleanedPath,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, fileOpTimeout)
	defer cancel()

	sessionID := uuid.New().String()

	resp, err := s.gw.SendAndWait(timeoutCtx, deviceID, websocket.MessageTypeReadFile, reqPayload, sessionID, fileOpTimeout)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file: %w", err)
	}

	var rawResp struct {
		Content  string `json:"content"`
		Mimetype string `json:"mimetype"`
		Size     int64  `json:"size"`
		IsBinary bool   `json:"is_binary"`
	}
	if err := json.Unmarshal(resp.Payload, &rawResp); err != nil {
		return nil, "", fmt.Errorf("failed to parse file content response: %w", err)
	}

	if rawResp.Mimetype == "" {
		rawResp.Mimetype = fileutil.DetectMimeType(cleanedPath)
	}

	s.writeAuditLog(ctx, userID, deviceID, models.ActionFileRead, map[string]interface{}{
		"path":     cleanedPath,
		"size":     rawResp.Size,
		"mimetype": rawResp.Mimetype,
	})

	if rawResp.IsBinary {
		return nil, "", nil
	}

	return &FileContentResponse{
		Content:  rawResp.Content,
		Mimetype: rawResp.Mimetype,
		Size:     rawResp.Size,
	}, "", nil
}

func (s *FileService) WriteFile(ctx context.Context, userID, deviceID, filePath, content string) (*WriteFileResponse, error) {
	if err := s.validateDeviceOnline(ctx, deviceID); err != nil {
		return nil, err
	}

	cleanedPath, err := s.sanitizePath(filePath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	decodedContent, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 content: %w", err)
	}

	fileMode := "0644"
	if err := fileutil.ValidateFileMode(fileMode); err != nil {
		return nil, fmt.Errorf("invalid file mode: %w", err)
	}

	reqPayload := map[string]interface{}{
		"path":    cleanedPath,
		"content": content,
		"mode":    fileMode,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, fileOpTimeout)
	defer cancel()

	sessionID := uuid.New().String()

	resp, err := s.gw.SendAndWait(timeoutCtx, deviceID, websocket.MessageTypeWriteFile, reqPayload, sessionID, fileOpTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	var rawResp struct {
		Success    bool `json:"success"`
		NewVersion int  `json:"new_version"`
	}
	if err := json.Unmarshal(resp.Payload, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to parse write file response: %w", err)
	}

	s.writeAuditLog(ctx, userID, deviceID, models.ActionFileWrite, map[string]interface{}{
		"path":          cleanedPath,
		"bytes_written": len(decodedContent),
	})

	return &WriteFileResponse{
		Success:    true,
		NewVersion: rawResp.NewVersion,
	}, nil
}

func (s *FileService) UploadFile(ctx context.Context, userID, deviceID, directory string, fileContent []byte, fileName string) (*UploadResponse, error) {
	if err := s.validateDeviceOnline(ctx, deviceID); err != nil {
		return nil, err
	}

	if len(fileContent) > maxFileSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size of %d bytes", maxFileSize)
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	if !allowedFileExtensions[ext] {
		return nil, fmt.Errorf("file type '%s' is not allowed", ext)
	}

	cleanedDir, err := s.sanitizePath(directory)
	if err != nil {
		return nil, fmt.Errorf("invalid directory: %w", err)
	}

	targetPath := path.Join(cleanedDir, fileName)

	c, cancel := context.WithTimeout(ctx, fileOpTimeout*3)
	defer cancel()

	contentBase64 := base64.StdEncoding.EncodeToString(fileContent)

	fileMode := "0644"
	if err := fileutil.ValidateFileMode(fileMode); err != nil {
		return nil, fmt.Errorf("invalid file mode: %w", err)
	}

	reqPayload := map[string]interface{}{
		"path":    targetPath,
		"content": contentBase64,
		"mode":    fileMode,
	}

	sessionID := uuid.New().String()

	resp, err := s.gw.SendAndWait(c, deviceID, websocket.MessageTypeUploadFile, reqPayload, sessionID, fileOpTimeout*3)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	var rawResp struct {
		UploadID     string `json:"upload_id"`
		BytesWritten int64  `json:"bytes_written"`
		Success      bool   `json:"success"`
	}
	if err := json.Unmarshal(resp.Payload, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to parse upload response: %w", err)
	}

	s.writeAuditLog(ctx, userID, deviceID, models.ActionFileUpload, map[string]interface{}{
		"path":          targetPath,
		"file_name":     fileName,
		"bytes_written": rawResp.BytesWritten,
	})

	return &UploadResponse{
		UploadID:     rawResp.UploadID,
		BytesWritten: rawResp.BytesWritten,
		Success:      rawResp.Success,
	}, nil
}

func (s *FileService) GenerateDownloadToken(ctx context.Context, userID, deviceID, filePath string) (string, error) {
	if err := s.validateDeviceOnline(ctx, deviceID); err != nil {
		return "", err
	}

	cleanedPath, err := s.sanitizePath(filePath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	token := uuid.New().String()

	tokenData := DownloadTokenData{
		DeviceID: deviceID,
		Path:     cleanedPath,
		UserID:   userID,
	}
	tokenJSON, err := json.Marshal(tokenData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal download token data: %w", err)
	}

	if err := s.redis.Set(ctx, downloadTokenPrefix+token, tokenJSON, downloadTokenTTL); err != nil {
		return "", fmt.Errorf("failed to store download token: %w", err)
	}

	s.writeAuditLog(ctx, userID, deviceID, models.ActionDownload, map[string]interface{}{
		"path":  cleanedPath,
		"token": token,
	})

	return token, nil
}

func (s *FileService) ValidateDownloadToken(ctx context.Context, token string) (*DownloadTokenData, error) {
	tokenJSON, err := s.redis.Get(ctx, downloadTokenPrefix+token)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired download token")
	}

	var tokenData DownloadTokenData
	if err := json.Unmarshal([]byte(tokenJSON), &tokenData); err != nil {
		return nil, fmt.Errorf("invalid download token data")
	}

	return &tokenData, nil
}

func (s *FileService) RevokeDownloadToken(ctx context.Context, token string) error {
	return s.redis.Del(ctx, downloadTokenPrefix+token)
}

func (s *FileService) ChangePermission(ctx context.Context, userID, deviceID, filePath, mode string) error {
	if err := s.validateDeviceOnline(ctx, deviceID); err != nil {
		return err
	}

	cleanedPath, err := s.sanitizePath(filePath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	reqPayload := map[string]interface{}{
		"path": cleanedPath,
		"mode": mode,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, fileOpTimeout)
	defer cancel()

	sessionID := uuid.New().String()

	resp, err := s.gw.SendAndWait(timeoutCtx, deviceID, websocket.MessageTypeFileChmod, reqPayload, sessionID, fileOpTimeout)
	if err != nil {
		return fmt.Errorf("failed to change file permission: %w", err)
	}

	var rawResp struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(resp.Payload, &rawResp); err != nil {
		return fmt.Errorf("failed to parse chmod response: %w", err)
	}

	if !rawResp.Success {
		return fmt.Errorf("device rejected chmod: %s", rawResp.Error)
	}

	s.writeAuditLog(ctx, userID, deviceID, models.ActionFileChmod, map[string]interface{}{
		"path": cleanedPath,
		"mode": mode,
	})

	return nil
}

func (s *FileService) ChangeOwnership(ctx context.Context, userID, deviceID, filePath, owner, group string) error {
	if err := s.validateDeviceOnline(ctx, deviceID); err != nil {
		return err
	}

	cleanedPath, err := s.sanitizePath(filePath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	reqPayload := map[string]interface{}{
		"path":  cleanedPath,
		"owner": owner,
		"group": group,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, fileOpTimeout)
	defer cancel()

	sessionID := uuid.New().String()

	resp, err := s.gw.SendAndWait(timeoutCtx, deviceID, websocket.MessageTypeFileChown, reqPayload, sessionID, fileOpTimeout)
	if err != nil {
		return fmt.Errorf("failed to change file ownership: %w", err)
	}

	var rawResp struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(resp.Payload, &rawResp); err != nil {
		return fmt.Errorf("failed to parse chown response: %w", err)
	}

	if !rawResp.Success {
		return fmt.Errorf("device rejected chown: %s", rawResp.Error)
	}

	s.writeAuditLog(ctx, userID, deviceID, models.ActionFileChown, map[string]interface{}{
		"path":  cleanedPath,
		"owner": owner,
		"group": group,
	})

	return nil
}

func (s *FileService) GetFileInfo(ctx context.Context, userID, deviceID, filePath string) (*DetailedFileInfo, error) {
	if err := s.validateDeviceOnline(ctx, deviceID); err != nil {
		return nil, err
	}

	cleanedPath, err := s.sanitizePath(filePath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	reqPayload := map[string]interface{}{
		"path": cleanedPath,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, fileOpTimeout)
	defer cancel()

	sessionID := uuid.New().String()

	resp, err := s.gw.SendAndWait(timeoutCtx, deviceID, websocket.MessageTypeFileStat, reqPayload, sessionID, fileOpTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	var rawResp struct {
		Name          string `json:"name"`
		Size          int64  `json:"size"`
		Mode          string `json:"mode"`
		Owner         string `json:"owner"`
		Group         string `json:"group"`
		ModifiedAt    string `json:"modified_at"`
		IsDir         bool   `json:"is_dir"`
		IsSymlink     bool   `json:"is_symlink"`
		SymlinkTarget string `json:"symlink_target,omitempty"`
	}
	if err := json.Unmarshal(resp.Payload, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to parse stat response: %w", err)
	}

	modifiedAt := time.Time{}
	if rawResp.ModifiedAt != "" {
		if t, err := time.Parse(time.RFC3339, rawResp.ModifiedAt); err == nil {
			modifiedAt = t
		}
	}

	s.writeAuditLog(ctx, userID, deviceID, models.ActionFileInfo, map[string]interface{}{
		"path": cleanedPath,
	})

	return &DetailedFileInfo{
		Name:          rawResp.Name,
		Path:          cleanedPath,
		Size:          rawResp.Size,
		Mode:          rawResp.Mode,
		Owner:         rawResp.Owner,
		Group:         rawResp.Group,
		ModifiedAt:    modifiedAt,
		IsDir:         rawResp.IsDir,
		IsSymlink:     rawResp.IsSymlink,
		SymlinkTarget: rawResp.SymlinkTarget,
	}, nil
}

func (s *FileService) DeleteFile(ctx context.Context, userID, deviceID, filePath string) error {
	if err := s.validateDeviceOnline(ctx, deviceID); err != nil {
		return err
	}

	cleanedPath, err := s.sanitizePath(filePath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	reqPayload := map[string]interface{}{
		"path": cleanedPath,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, fileOpTimeout)
	defer cancel()

	sessionID := uuid.New().String()

	resp, err := s.gw.SendAndWait(timeoutCtx, deviceID, websocket.MessageTypeDeleteFile, reqPayload, sessionID, fileOpTimeout)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	var rawResp struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(resp.Payload, &rawResp); err != nil {
		return fmt.Errorf("failed to parse delete file response: %w", err)
	}

	if !rawResp.Success {
		return fmt.Errorf("device rejected file deletion: %s", rawResp.Error)
	}

	s.writeAuditLog(ctx, userID, deviceID, models.ActionFileDelete, map[string]interface{}{
		"path": cleanedPath,
	})

	return nil
}

func (s *FileService) writeAuditLog(ctx context.Context, userID, deviceID, action string, detail map[string]interface{}) {
	if s.db == nil {
		return
	}

	detailBytes, err := json.Marshal(detail)
	if err != nil {
		slog.WarnContext(ctx, "failed to marshal audit log detail",
			"error", err, "action", action)
		return
	}

	var tenantID string
	var device models.Device
	if err := s.db.WithContext(ctx).Where("id = ?", deviceID).First(&device).Error; err == nil {
		tenantID = device.TenantID
	}

	auditLog := models.AuditLog{
		TenantID: tenantID,
		UserID:   userID,
		DeviceID: deviceID,
		Action:   action,
		Detail:   datatypes.JSON(detailBytes),
	}

	if err := s.db.WithContext(ctx).Create(&auditLog).Error; err != nil {
		slog.ErrorContext(ctx, "failed to create audit log",
			"action", action, "device_id", deviceID, "error", err)
	}
}

func GetMaxFileSize() int {
	return maxFileSize
}

func ValidateFileType(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	return allowedFileExtensions[ext]
}
