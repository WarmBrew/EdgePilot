package service

import (
	"context"
	"fmt"
	"log/slog"

	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type SessionMonitor struct {
	db       *gorm.DB
	redis    *pkgRedis.RedisClient
	service  *TerminalSessionService
	cron     *cron.Cron
	workerID string
}

func NewSessionMonitor(db *gorm.DB, redis *pkgRedis.RedisClient, service *TerminalSessionService) *SessionMonitor {
	return &SessionMonitor{
		db:       db,
		redis:    redis,
		service:  service,
		cron:     cron.New(cron.WithSeconds()),
		workerID: "session-monitor",
	}
}

func (m *SessionMonitor) Start(ctx context.Context) error {
	_, err := m.cron.AddFunc("*/30 * * * * *", func() {
		m.checkExpiredSessions(ctx)
	})
	if err != nil {
		return fmt.Errorf("failed to schedule session cleanup: %w", err)
	}

	m.cron.Start()
	slog.Info("session monitor started", "worker_id", m.workerID, "check_interval", sessionCheckInterval)

	go func() {
		<-ctx.Done()
		slog.Info("session monitor shutting down", "worker_id", m.workerID)
		m.cron.Stop()
	}()

	return nil
}

func (m *SessionMonitor) Stop() {
	m.cron.Stop()
	slog.Info("session monitor stopped", "worker_id", m.workerID)
}

func (m *SessionMonitor) checkExpiredSessions(ctx context.Context) {
	slog.Info("running expired session check", "worker_id", m.workerID)

	closedCount, err := m.service.CleanupExpiredSessions(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to cleanup expired sessions", "error", err)
		return
	}

	if closedCount > 0 {
		slog.InfoContext(ctx, "expired sessions cleanup completed", "closed_count", closedCount)
	}
}

func (m *SessionMonitor) RunOnce(ctx context.Context) error {
	closedCount, err := m.service.CleanupExpiredSessions(ctx)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	slog.InfoContext(ctx, "session monitor one-time run completed", "closed_count", closedCount)
	return nil
}
