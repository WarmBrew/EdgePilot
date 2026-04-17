package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/edge-platform/server/internal/api/middleware"
	"github.com/edge-platform/server/internal/service"
	"github.com/edge-platform/server/internal/websocket"

	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type FileHandler struct {
	db    *gorm.DB
	redis *pkgRedis.RedisClient
	gw    *websocket.Gateway
	svc   *service.FileService
}

func NewFileHandler(db *gorm.DB, redis *pkgRedis.RedisClient, gw *websocket.Gateway) *FileHandler {
	return &FileHandler{
		db:    db,
		redis: redis,
		gw:    gw,
		svc:   service.NewFileService(db, redis, gw),
	}
}

type UpdateFileRequest struct {
	Content string `json:"content" binding:"required"`
	Version *int   `json:"version"`
}

type UploadFormRequest struct {
	Directory string `form:"directory" binding:"required"`
}

type ListDirResponse struct {
	Files []service.FileInfo `json:"files"`
	Path  string             `json:"path"`
	Total int                `json:"total"`
	Page  int                `json:"page"`
	Size  int                `json:"page_size"`
}

// ListFiles handles GET /api/v1/devices/:id/files?path=/home/user
func (h *FileHandler) ListFiles(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	deviceID := c.Param("id")
	targetPath := c.Query("path")

	page := 1
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	pageSize := 50
	if s := c.Query("page_size"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			pageSize = v
		}
	}

	result, err := h.svc.ListDir(c.Request.Context(), userID, deviceID, targetPath, page, pageSize)
	if err != nil {
		if err.Error() == fmt.Sprintf("device %s is not online", deviceID) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device is offline"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetFileContent handles GET /api/v1/devices/:id/files/:filepath
func (h *FileHandler) GetFileContent(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	deviceID := c.Param("id")
	filePath := c.Param("filepath")

	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file path is required"})
		return
	}

	result, _, err := h.svc.GetFileContent(c.Request.Context(), userID, deviceID, filePath)
	if err != nil {
		if err.Error() == fmt.Sprintf("device %s is not online", deviceID) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device is offline"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if result == nil {
		token, tokenErr := h.svc.GenerateDownloadToken(c.Request.Context(), userID, deviceID, filePath)
		if tokenErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": tokenErr.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"download_url": "/api/v1/download/" + token,
			"message":      "binary file, use download URL",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content":  result.Content,
		"mimetype": result.Mimetype,
		"size":     result.Size,
	})
}

// UpdateFile handles PUT /api/v1/devices/:id/files/:filepath
func (h *FileHandler) UpdateFile(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	deviceID := c.Param("id")
	filePath := c.Param("filepath")

	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file path is required"})
		return
	}

	var req UpdateFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if _, err := base64.StdEncoding.DecodeString(req.Content); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content must be valid base64 encoded string"})
		return
	}

	result, err := h.svc.WriteFile(c.Request.Context(), userID, deviceID, filePath, req.Content)
	if err != nil {
		if err.Error() == fmt.Sprintf("device %s is not online", deviceID) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device is offline"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// UploadFile handles POST /api/v1/devices/:id/files/upload
func (h *FileHandler) UploadFile(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	deviceID := c.Param("id")

	_ = c.Request.ParseMultipartForm(32 << 20)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	directory := c.Request.FormValue("directory")
	if directory == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "directory is required"})
		return
	}

	if !service.ValidateFileType(header.Filename) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("file type '%s' is not allowed", header.Filename),
		})
		return
	}

	fileContent, err := readFileContent(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read uploaded file"})
		return
	}

	if len(fileContent) > service.GetMaxFileSize() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("file size exceeds maximum allowed size of %d bytes", service.GetMaxFileSize()),
		})
		return
	}

	result, err := h.svc.UploadFile(c.Request.Context(), userID, deviceID, directory, fileContent, header.Filename)
	if err != nil {
		if err.Error() == fmt.Sprintf("device %s is not online", deviceID) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device is offline"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// DownloadFile handles GET /api/v1/devices/:id/files/:filepath/download
func (h *FileHandler) DownloadFile(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	deviceID := c.Param("id")
	filePath := c.Param("filepath")

	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file path is required"})
		return
	}

	token, err := h.svc.GenerateDownloadToken(c.Request.Context(), userID, deviceID, filePath)
	if err != nil {
		if err.Error() == fmt.Sprintf("device %s is not online", deviceID) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device is offline"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	escapedPath := url.QueryEscape(filePath)
	c.JSON(http.StatusOK, gin.H{
		"download_url": "/api/v1/download/" + token,
		"filename":     escapedPath,
		"expires_in":   "10 minutes",
	})
}

// DeleteFile handles DELETE /api/v1/devices/:id/files/:filepath
func (h *FileHandler) DeleteFile(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	deviceID := c.Param("id")
	filePath := c.Param("filepath")

	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file path is required"})
		return
	}

	err := h.svc.DeleteFile(c.Request.Context(), userID, deviceID, filePath)
	if err != nil {
		if err.Error() == fmt.Sprintf("device %s is not online", deviceID) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device is offline"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// HandleDownloadToken handles GET /api/v1/download/:token
func (h *FileHandler) HandleDownloadToken(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "download token is required"})
		return
	}

	tokenData, err := h.svc.ValidateDownloadToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invalid or expired download token"})
		return
	}

	if !h.gw.IsDeviceOnline(tokenData.DeviceID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device is offline"})
		return
	}

	userID, _ := middleware.GetUserID(c)
	if userID == "" {
		userID = tokenData.UserID
	}

	resp, _, err := h.svc.GetFileContent(c.Request.Context(), userID, tokenData.DeviceID, tokenData.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_ = h.svc.RevokeDownloadToken(c.Request.Context(), token)

	if resp != nil {
		c.Header("Content-Type", resp.Mimetype)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", extractFilename(tokenData.Path)))
		c.String(http.StatusOK, resp.Content)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content":  resp.Content,
		"mimetype": resp.Mimetype,
		"size":     resp.Size,
	})
}

func readFileContent(file interface{ Read([]byte) (int, error) }) ([]byte, error) {
	buf := make([]byte, 0, 1024)
	for {
		tmp := make([]byte, 1024)
		n, err := file.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}

func extractFilename(filePath string) string {
	if filePath == "" {
		return "download"
	}
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '/' {
			return filePath[i+1:]
		}
	}
	return filePath
}
