package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
)

const (
	metricsKeyPrefix = "device:metrics:"
	metricsTTL       = 5 * time.Minute
)

type SystemMetrics struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	Uptime      int64   `json:"uptime"`
	CollectedAt string  `json:"collected_at"`
}

type MetricsService struct {
	redisClient *pkgRedis.RedisClient
}

func NewMetricsService(redis *pkgRedis.RedisClient) *MetricsService {
	return &MetricsService{redisClient: redis}
}

func (s *MetricsService) StoreMetrics(ctx context.Context, deviceID string, metrics SystemMetrics) error {
	metrics.CollectedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	key := metricsKey(deviceID)
	if err := s.redisClient.Raw().Set(ctx, key, string(data), metricsTTL).Err(); err != nil {
		return fmt.Errorf("failed to store metrics in Redis: %w", err)
	}

	slog.DebugContext(ctx, "metrics stored", "device_id", deviceID, "key", key)
	return nil
}

func (s *MetricsService) GetDeviceMetrics(ctx context.Context, deviceID string) (*SystemMetrics, error) {
	key := metricsKey(deviceID)

	data, err := s.redisClient.Raw().Get(ctx, key).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get metrics from Redis: %w", err)
	}

	var metrics SystemMetrics
	if err := json.Unmarshal([]byte(data), &metrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	return &metrics, nil
}

func metricsKey(deviceID string) string {
	return metricsKeyPrefix + deviceID
}
