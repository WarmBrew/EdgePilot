package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/edge-platform/server/internal/domain/models"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/edge-platform/server/internal/websocket"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	createPTYTimeout     = 10 * time.Second
	sessionInactiveLimit = 30 * time.Minute
	sessionCheckInterval = 5 * time.Minute
)

type PTYRequest struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	DeviceID  string `json:"device_id"`
	Shell     string `json:"shell,omitempty"`
	Rows      int    `json:"rows"`
	Cols      int    `json:"cols"`
}

type PTYResponse struct {
	Success bool   `json:"success"`
	PtyPath string `json:"pty_path,omitempty"`
	Error   string `json:"error,omitempty"`
}

type TerminalSessionService struct {
	db    *gorm.DB
	redis *pkgRedis.RedisClient
	gw    *websocket.Gateway
}

func NewTerminalSessionService(db *gorm.DB, redis *pkgRedis.RedisClient, gw *websocket.Gateway) *TerminalSessionService {
	return &TerminalSessionService{
		db:    db,
		redis: redis,
		gw:    gw,
	}
}

type CreateSessionResult struct {
	SessionID string    `json:"session_id"`
	DeviceID  string    `json:"device_id"`
	UserID    string    `json:"user_id"`
	PtyPath   string    `json:"pty_path"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
}

func (s *TerminalSessionService) CreateSession(ctx context.Context, userID, deviceID string) (*CreateSessionResult, error) {
	if !s.gw.IsDeviceOnline(deviceID) {
		return nil, fmt.Errorf("device %s is not online", deviceID)
	}

	now := time.Now()
	session := models.TerminalSession{
		DeviceID:  deviceID,
		UserID:    userID,
		Status:    models.SessionPending,
		StartedAt: now,
	}

	if err := s.db.Create(&session).Error; err != nil {
		slog.ErrorContext(ctx, "failed to create terminal session record",
			"device_id", deviceID, "user_id", userID, "error", err)
		return nil, fmt.Errorf("failed to create session record: %w", err)
	}

	sessionID := session.ID
	slog.InfoContext(ctx, "terminal session created, sending create_pty to device",
		"session_id", sessionID, "device_id", deviceID)

	req := PTYRequest{
		SessionID: sessionID,
		UserID:    userID,
		DeviceID:  deviceID,
		Rows:      24,
		Cols:      80,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, createPTYTimeout)
	defer cancel()

	resp, err := s.gw.SendAndWait(timeoutCtx, deviceID, websocket.MessageTypeCreatePTY, req, sessionID, createPTYTimeout)
	if err != nil {
		slog.WarnContext(ctx, "failed to get PTY response from device, marking session as failed",
			"session_id", sessionID, "device_id", deviceID, "error", err)

		s.db.Model(&models.TerminalSession{}).
			Where("id = ?", sessionID).
			Updates(map[string]interface{}{
				"status": models.SessionFailed,
			})

		isTimeout := errors.Is(err, context.DeadlineExceeded) || err.Error() == "device is not online"
		if isTimeout {
			return nil, fmt.Errorf("device did not respond within %v", createPTYTimeout)
		}
		return nil, fmt.Errorf("failed to create PTY on device: %w", err)
	}

	var ptyResp PTYResponse
	if err := json.Unmarshal(resp.Payload, &ptyResp); err != nil {
		slog.WarnContext(ctx, "failed to parse PTY response payload",
			"session_id", sessionID, "error", err)

		s.db.Model(&models.TerminalSession{}).
			Where("id = ?", sessionID).
			Updates(map[string]interface{}{
				"status": models.SessionFailed,
			})

		return nil, fmt.Errorf("invalid PTY response from device")
	}

	if !ptyResp.Success {
		slog.WarnContext(ctx, "device rejected PTY creation",
			"session_id", sessionID, "error", ptyResp.Error)

		s.db.Model(&models.TerminalSession{}).
			Where("id = ?", sessionID).
			Updates(map[string]interface{}{
				"status": models.SessionFailed,
			})

		return nil, fmt.Errorf("device rejected PTY: %s", ptyResp.Error)
	}

	updates := map[string]interface{}{
		"status":   models.SessionActive,
		"pty_path": ptyResp.PtyPath,
	}
	if err := s.db.Model(&models.TerminalSession{}).
		Where("id = ?", sessionID).
		Updates(updates).Error; err != nil {
		slog.ErrorContext(ctx, "failed to update session after PTY success",
			"session_id", sessionID, "error", err)
	}

	if s.redis != nil {
		cacheEntry := pkgRedis.TerminalSessionCacheEntry{
			UserID:    userID,
			DeviceID:  deviceID,
			PtyPath:   ptyResp.PtyPath,
			CreatedAt: now,
		}
		if err := s.redis.CacheTerminalSession(ctx, sessionID, cacheEntry); err != nil {
			slog.WarnContext(ctx, "failed to cache terminal session in redis",
				"session_id", sessionID, "error", err)
		}
	}

	s.writeAuditLog(ctx, userID, deviceID, sessionID, models.ActionTerminalOpen, map[string]interface{}{
		"pty_path":  ptyResp.PtyPath,
		"device_id": deviceID,
	})

	return &CreateSessionResult{
		SessionID: sessionID,
		DeviceID:  deviceID,
		UserID:    userID,
		PtyPath:   ptyResp.PtyPath,
		Status:    models.SessionActive,
		StartedAt: now,
	}, nil
}

func (s *TerminalSessionService) CloseSession(ctx context.Context, userID, sessionID string) error {
	var session models.TerminalSession
	if err := s.db.Preload("Device").Where("id = ?", sessionID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("session %s not found", sessionID)
		}
		return fmt.Errorf("failed to query session: %w", err)
	}

	if session.UserID != userID {
		return fmt.Errorf("session %s does not belong to user %s", sessionID, userID)
	}

	if session.Status == models.SessionClosed {
		return fmt.Errorf("session %s is already closed", sessionID)
	}

	now := time.Now()

	s.sendPTYClose(session.DeviceID, sessionID)

	updates := map[string]interface{}{
		"status":    models.SessionClosed,
		"closed_at": now,
	}
	if err := s.db.Model(&models.TerminalSession{}).
		Where("id = ?", sessionID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update session status: %w", err)
	}

	if s.redis != nil {
		if err := s.redis.DeleteTerminalSessionCache(ctx, sessionID); err != nil {
			slog.WarnContext(ctx, "failed to remove terminal session cache",
				"session_id", sessionID, "error", err)
		}
	}

	s.writeAuditLog(ctx, userID, session.DeviceID, sessionID, models.ActionTerminalClose, map[string]interface{}{
		"closed_at": now.Format(time.RFC3339),
		"pty_path":  session.PtyPath,
	})

	slog.InfoContext(ctx, "terminal session closed",
		"session_id", sessionID, "user_id", userID, "device_id", session.DeviceID)

	return nil
}

func (s *TerminalSessionService) GetSession(sessionID string) (*models.TerminalSession, error) {
	var session models.TerminalSession
	if err := s.db.Preload("Device").Preload("User").
		Where("id = ?", sessionID).
		First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("session %s not found", sessionID)
		}
		return nil, fmt.Errorf("failed to query session: %w", err)
	}

	return &session, nil
}

type ListSessionsFilter struct {
	UserID   string
	DeviceID string
	Status   string
	Page     int
	PageSize int
}

type ListSessionsResult struct {
	Sessions []*models.TerminalSession `json:"sessions"`
	Total    int64                     `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
}

func (s *TerminalSessionService) ListSessions(ctx context.Context, filter ListSessionsFilter) (*ListSessionsResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	query := s.db.Model(&models.TerminalSession{}).
		Preload("Device").
		Preload("User")

	if filter.UserID != "" {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.DeviceID != "" {
		query = query.Where("device_id = ?", filter.DeviceID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count sessions: %w", err)
	}

	var sessions []*models.TerminalSession
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order("created_at DESC").
		Offset(offset).
		Limit(filter.PageSize).
		Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return &ListSessionsResult{
		Sessions: sessions,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}, nil
}

func (s *TerminalSessionService) CleanupExpiredSessions(ctx context.Context) (int, error) {
	expiredBefore := time.Now().Add(-sessionInactiveLimit)

	var expiredSessions []models.TerminalSession
	if err := s.db.
		Where("status = ? AND updated_at < ?", models.SessionActive, expiredBefore).
		Find(&expiredSessions).Error; err != nil {
		return 0, fmt.Errorf("failed to query expired sessions: %w", err)
	}

	if len(expiredSessions) == 0 {
		return 0, nil
	}

	slog.InfoContext(ctx, "found expired sessions to close",
		"count", len(expiredSessions), "expired_before", expiredBefore)

	closedCount := 0
	now := time.Now()

	for _, session := range expiredSessions {
		s.sendPTYClose(session.DeviceID, session.ID)

		if err := s.db.Model(&models.TerminalSession{}).
			Where("id = ?", session.ID).
			Updates(map[string]interface{}{
				"status":    models.SessionClosed,
				"closed_at": now,
			}).Error; err != nil {
			slog.ErrorContext(ctx, "failed to close expired session",
				"session_id", session.ID, "error", err)
			continue
		}

		if s.redis != nil {
			if err := s.redis.DeleteTerminalSessionCache(ctx, session.ID); err != nil {
				slog.WarnContext(ctx, "failed to delete redis cache for expired session",
					"session_id", session.ID, "error", err)
			}
		}

		s.writeAuditLog(ctx, session.UserID, session.DeviceID, session.ID, models.ActionTerminalExpire, map[string]interface{}{
			"reason":        "inactive_timeout",
			"timeout_value": sessionInactiveLimit.String(),
			"updated_at":    session.UpdatedAt.Format(time.RFC3339),
		})

		closedCount++
	}

	slog.InfoContext(ctx, "expired sessions cleanup completed",
		"total_found", len(expiredSessions), "closed", closedCount)

	return closedCount, nil
}

func (s *TerminalSessionService) sendPTYClose(deviceID, sessionID string) {
	if deviceID == "" || sessionID == "" {
		slog.Warn("cannot send pty_close: missing device_id or session_id",
			"device_id", deviceID, "session_id", sessionID)
		return
	}

	go func() {
		payload := map[string]string{
			"session_id": sessionID,
		}

		if err := s.gw.SendMessageToDevice(deviceID, websocket.MessageTypePTYClose, payload); err != nil {
			slog.Warn("failed to send pty_close to device",
				"session_id", sessionID, "device_id", deviceID, "error", err)
		}
	}()
}

func (s *TerminalSessionService) writeAuditLog(ctx context.Context, userID, deviceID, sessionID, action string, detail map[string]interface{}) {
	if s.db == nil {
		return
	}

	detailBytes, err := json.Marshal(detail)
	if err != nil {
		slog.WarnContext(ctx, "failed to marshal audit log detail",
			"error", err, "session_id", sessionID)
		return
	}

	var tenantID string
	var device models.Device
	if err := s.db.Where("id = ?", deviceID).First(&device).Error; err == nil {
		tenantID = device.TenantID
	}

	auditLog := models.AuditLog{
		TenantID: tenantID,
		UserID:   userID,
		DeviceID: deviceID,
		Action:   action,
		Detail:   datatypes.JSON(detailBytes),
	}

	if err := s.db.Create(&auditLog).Error; err != nil {
		slog.ErrorContext(ctx, "failed to create audit log",
			"action", action, "session_id", sessionID, "error", err)
	} else {
		slog.InfoContext(ctx, "audit log created",
			"action", action, "session_id", sessionID, "device_id", deviceID)
	}
}
