package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/edge-platform/server/internal/domain/models"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/redis/go-redis/v9"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	defaultQueueSize = 10000
	defaultBatchSize = 100
	maxBatchSize     = 500
	writeTimeout     = 5 * time.Second
	maxRetries       = 3
	retryBaseDelay   = 100 * time.Millisecond
)

// AuditConfig holds audit service configuration.
type AuditConfig struct {
	QueueSize int
	BatchSize int
}

// DefaultAuditConfig returns sensible defaults.
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		QueueSize: defaultQueueSize,
		BatchSize: defaultBatchSize,
	}
}

// AuditService handles async audit log writing with batching and retries.
type AuditService struct {
	db        *gorm.DB
	redis     *redis.Client
	queue     chan *models.AuditLog
	batchSize int
	done      chan struct{}
	stopped   chan struct{}
	// Prometheus-like counters (exposed for /metrics endpoint)
	totalWritten int64
	totalDropped int64
	totalErrors  int64
}

// NewAuditService creates a new audit service with a buffered async queue.
func NewAuditService(db *gorm.DB, redis *pkgRedis.RedisClient, cfg AuditConfig) *AuditService {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = defaultQueueSize
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.BatchSize > maxBatchSize {
		cfg.BatchSize = maxBatchSize
	}

	return &AuditService{
		db:        db,
		redis:     redis.Raw(),
		queue:     make(chan *models.AuditLog, cfg.QueueSize),
		batchSize: cfg.BatchSize,
		done:      make(chan struct{}),
		stopped:   make(chan struct{}),
	}
}

// Start launches the background worker goroutine for processing the audit queue.
func (s *AuditService) Start(ctx context.Context) {
	go s.worker(ctx)
	slog.Info("AuditService started", "queue_size", cap(s.queue), "batch_size", s.batchSize)
}

// Stop signals the worker to drain remaining items and shut down gracefully.
func (s *AuditService) Stop() {
	close(s.done)
	<-s.stopped
	slog.Info("AuditService stopped",
		"total_written", s.totalWritten,
		"total_dropped", s.totalDropped,
		"total_errors", s.totalErrors,
	)
}

// Log enqueues a single audit log entry for async write.
// Returns ErrQueueFull if the queue is at capacity (non-blocking).
func (s *AuditService) Log(ctx context.Context, log *models.AuditLog) error {
	select {
	case s.queue <- log:
		return nil
	default:
		s.totalDropped++
		slog.WarnContext(ctx, "audit log dropped: queue full",
			"action", log.Action,
			"user_id", log.UserID,
		)
		return ErrQueueFull
	}
}

// ErrQueueFull is returned when the audit queue is at capacity.
var ErrQueueFull = &queueFullError{}

type queueFullError struct{}

func (e *queueFullError) Error() string {
	return "audit queue is full"
}

// LogDeviceAction logs a device-related action.
func (s *AuditService) LogDeviceAction(ctx context.Context, userID, deviceID, action, detail string) {
	log := &models.AuditLog{
		UserID:   userID,
		DeviceID: deviceID,
		Action:   action,
		Detail:   datatypes.JSON(`{"detail":` + jsonString(detail) + `}`),
	}
	_ = s.Log(ctx, log)
}

// LogTerminalSession logs a terminal session event.
func (s *AuditService) LogTerminalSession(ctx context.Context, userID, deviceID, sessionID, action string) {
	detail := `{"session_id":` + jsonString(sessionID) + `,"action":` + jsonString(action) + `}`
	log := &models.AuditLog{
		UserID:   userID,
		DeviceID: deviceID,
		Action:   action,
		Detail:   datatypes.JSON(detail),
	}
	_ = s.Log(ctx, log)
}

// LogFileOperation logs a file operation (upload, download, edit, delete, etc.).
func (s *AuditService) LogFileOperation(ctx context.Context, userID, deviceID, filePath, action string) {
	detail := `{"file_path":` + jsonString(filePath) + `}`
	log := &models.AuditLog{
		UserID:   userID,
		DeviceID: deviceID,
		Action:   action,
		Detail:   datatypes.JSON(detail),
	}
	_ = s.Log(ctx, log)
}

// LogAuthEvent logs an authentication event (login, logout, token refresh, etc.).
func (s *AuditService) LogAuthEvent(ctx context.Context, userID, action, ipAddress string) {
	log := &models.AuditLog{
		UserID:    userID,
		DeviceID:  "N/A",
		Action:    action,
		IPAddress: ipAddress,
		Detail:    datatypes.JSON(`{}`),
	}
	_ = s.Log(ctx, log)
}

// LogPermissionChange logs a permission or role change.
func (s *AuditService) LogPermissionChange(ctx context.Context, userID, targetID, action, detail string) {
	detailJSON := `{"target_id":` + jsonString(targetID) + `,"action":` + jsonString(detail) + `}`
	log := &models.AuditLog{
		UserID:   userID,
		DeviceID: "N/A",
		Action:   action,
		Detail:   datatypes.JSON(detailJSON),
	}
	_ = s.Log(ctx, log)
}

// GetStats returns current audit service statistics.
func (s *AuditService) GetStats() map[string]int64 {
	return map[string]int64{
		"queue_size":    int64(len(s.queue)),
		"queue_cap":     int64(cap(s.queue)),
		"total_written": s.totalWritten,
		"total_dropped": s.totalDropped,
		"total_errors":  s.totalErrors,
	}
}

// ---- internal ----

func (s *AuditService) worker(ctx context.Context) {
	defer close(s.stopped)

	batch := make([]*models.AuditLog, 0, s.batchSize)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	flushAndReset := func() {
		if len(batch) > 0 {
			s.flushBatch(ctx, batch)
			batch = batch[:0]
		}
	}

	for {
		select {
		case <-ctx.Done():
			flushAndReset()
			return
		case <-s.done:
			flushAndReset()
			// Drain remaining items
			for {
				select {
				case log := <-s.queue:
					batch = append(batch, log)
					if len(batch) >= s.batchSize {
						s.flushBatch(ctx, batch)
						batch = batch[:0]
					}
				default:
					flushAndReset()
					return
				}
			}
		case log := <-s.queue:
			batch = append(batch, log)
			if len(batch) >= s.batchSize {
				s.flushBatch(ctx, batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			flushAndReset()
		}
	}
}

func (s *AuditService) flushBatch(ctx context.Context, batch []*models.AuditLog) {
	if len(batch) == 0 {
		return
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryBaseDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				s.totalErrors += int64(len(batch))
				slog.WarnContext(ctx, "audit flush aborted: context done",
					"batch_size", len(batch), "attempt", attempt+1,
				)
				return
			}
		}

		writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
		lastErr = s.db.WithContext(writeCtx).CreateInBatches(batch, s.batchSize).Error
		cancel()

		if lastErr == nil {
			s.totalWritten += int64(len(batch))
			slog.DebugContext(ctx, "audit batch flushed",
				"count", len(batch), "attempt", attempt+1,
			)
			return
		}

		slog.WarnContext(ctx, "audit batch write failed, retrying",
			"error", lastErr,
			"attempt", attempt+1,
			"max_retries", maxRetries,
		)
	}

	s.totalErrors += int64(len(batch))
	slog.ErrorContext(ctx, "audit batch write failed after all retries",
		"error", lastErr,
		"batch_size", len(batch),
	)
}

// jsonString safely marshals a string for inline JSON construction.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
