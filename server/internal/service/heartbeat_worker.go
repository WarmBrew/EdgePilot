package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/edge-platform/server/internal/domain/models"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

const (
	heartbeatMissThreshold    = 90 * time.Second
	heartbeatOfflineThreshold = 300 * time.Second
	heartbeatCheckInterval    = "*/60 * * * * *"
	pubsubChannel             = "device:status:events"
)

type DeviceStatusEvent struct {
	DeviceID  string `json:"device_id"`
	TenantID  string `json:"tenant_id"`
	OldStatus string `json:"old_status"`
	NewStatus string `json:"new_status"`
	Timestamp string `json:"timestamp"`
}

type HeartbeatWorker struct {
	db       *gorm.DB
	cron     *cron.Cron
	workerID string
}

func NewHeartbeatWorker(db *gorm.DB) *HeartbeatWorker {
	return &HeartbeatWorker{
		db:       db,
		cron:     cron.New(cron.WithSeconds()),
		workerID: "heartbeat-monitor",
	}
}

func (w *HeartbeatWorker) Start(ctx context.Context) error {
	_, err := w.cron.AddFunc(heartbeatCheckInterval, func() {
		w.checkHeartbeats(ctx)
	})
	if err != nil {
		return fmt.Errorf("failed to schedule heartbeat check: %w", err)
	}

	w.cron.Start()
	slog.Info("heartbeat worker started", "worker_id", w.workerID, "check_interval", heartbeatCheckInterval)

	go func() {
		<-ctx.Done()
		slog.Info("heartbeat worker shutting down", "worker_id", w.workerID)
		w.cron.Stop()
	}()

	return nil
}

func (w *HeartbeatWorker) Stop() {
	w.cron.Stop()
	slog.Info("heartbeat worker stopped", "worker_id", w.workerID)
}

func (w *HeartbeatWorker) checkHeartbeats(ctx context.Context) {
	slog.Info("running heartbeat check", "worker_id", w.workerID)

	now := time.Now()
	missThreshold := now.Add(-heartbeatMissThreshold)
	offlineThreshold := now.Add(-heartbeatOfflineThreshold)

	w.markAsHeartbeatMiss(ctx, missThreshold, offlineThreshold)
	w.markAsOffline(ctx, offlineThreshold)
}

func (w *HeartbeatWorker) markAsHeartbeatMiss(ctx context.Context, missThreshold, offlineThreshold time.Time) {
	var devices []models.Device
	if err := w.db.
		Where("status IN (?, ?) AND last_heartbeat < ? AND last_heartbeat >= ?",
			models.StatusOnline, models.StatusHeartbeatMiss, missThreshold, offlineThreshold).
		Find(&devices).Error; err != nil {
		slog.ErrorContext(ctx, "failed to query heartbeat miss devices", "error", err)
		return
	}

	for _, device := range devices {
		oldStatus := device.Status
		if err := w.db.Model(&device).Update("status", models.StatusHeartbeatMiss).Error; err != nil {
			slog.ErrorContext(ctx, "failed to update device status to heartbeat_miss",
				"device_id", device.ID, "error", err)
			continue
		}

		slog.WarnContext(ctx, "device heartbeat missed",
			"device_id", device.ID,
			"device_name", device.Name,
			"old_status", oldStatus,
			"last_heartbeat", device.LastHeartbeat,
		)

		w.publishStatusEvent(ctx, device, oldStatus, models.StatusHeartbeatMiss)
	}
}

func (w *HeartbeatWorker) markAsOffline(ctx context.Context, offlineThreshold time.Time) {
	var devices []models.Device
	if err := w.db.
		Where("status = ? AND last_heartbeat < ?",
			models.StatusHeartbeatMiss, offlineThreshold).
		Find(&devices).Error; err != nil {
		slog.ErrorContext(ctx, "failed to query offline devices", "error", err)
		return
	}

	for _, device := range devices {
		oldStatus := device.Status
		if err := w.db.Model(&device).Update("status", models.StatusOffline).Error; err != nil {
			slog.ErrorContext(ctx, "failed to update device status to offline",
				"device_id", device.ID, "error", err)
			continue
		}

		slog.WarnContext(ctx, "device marked as offline",
			"device_id", device.ID,
			"device_name", device.Name,
			"old_status", oldStatus,
			"last_heartbeat", device.LastHeartbeat,
		)

		w.publishStatusEvent(ctx, device, oldStatus, models.StatusOffline)
	}
}

func (w *HeartbeatWorker) publishStatusEvent(ctx context.Context, device models.Device, oldStatus, newStatus string) {
	event := DeviceStatusEvent{
		DeviceID:  device.ID,
		TenantID:  device.TenantID,
		OldStatus: oldStatus,
		NewStatus: newStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(event)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal status event", "error", err)
		return
	}

	if err := w.db.Where("id = ?", device.ID).First(&device).Error; err == nil {
		deviceName := device.Name
		slog.InfoContext(ctx, "publishing status event",
			"device_id", device.ID,
			"device_name", deviceName,
			"old_status", oldStatus,
			"new_status", newStatus,
		)
	}

	client := getRedisClient()
	if client == nil {
		slog.ErrorContext(ctx, "redis client not available for publishing event")
		return
	}

	if err := client.Raw().Publish(ctx, pubsubChannel, string(data)).Err(); err != nil {
		slog.ErrorContext(ctx, "failed to publish status event", "error", err, "channel", pubsubChannel)
	}
}

func getRedisClient() *pkgRedis.RedisClient {
	defer func() {
		recover()
	}()
	return pkgRedis.GetClient()
}
