package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/edge-platform/server/internal/domain/models"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	deviceOnlinePrefix = "device:online:"
	deviceConnPrefix   = "device:conn:"
)

type DeviceListFilters struct {
	Status  string
	GroupID string
	Search  string
	Page    int
	Size    int
	SortBy  string
	SortDir string
}

type DeviceListResponse struct {
	Devices []DeviceWithStatus `json:"devices"`
	Total   int64              `json:"total"`
	Page    int                `json:"page"`
	Size    int                `json:"size"`
}

type DeviceWithStatus struct {
	models.Device
	IsOnline bool `json:"is_online"`
}

type UpdateDeviceInput struct {
	Name        string  `json:"name"`
	GroupID     *string `json:"group_id"`
	Description *string `json:"description"`
}

type BatchOperationInput struct {
	DeviceIDs []string       `json:"device_ids"`
	Action    string         `json:"action"`
	Params    map[string]any `json:"params,omitempty"`
}

type BatchResult struct {
	Success int                   `json:"success"`
	Failed  int                   `json:"failed"`
	Errors  []BatchOperationError `json:"errors"`
}

type BatchOperationError struct {
	DeviceID string `json:"device_id"`
	Reason   string `json:"reason"`
}

type AuditLogEntry struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id"`
	Action   string `json:"action"`
	Detail   any    `json:"detail"`
}

type DeviceService struct {
	db          *gorm.DB
	redisClient *pkgRedis.RedisClient
}

func NewDeviceService(db *gorm.DB, redis *pkgRedis.RedisClient) *DeviceService {
	return &DeviceService{db: db, redisClient: redis}
}

func (s *DeviceService) normalizeFilters(f DeviceListFilters) DeviceListFilters {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Size < 1 {
		f.Size = 20
	}
	if f.Size > 100 {
		f.Size = 100
	}
	if f.SortBy == "" {
		f.SortBy = "created_at"
	}
	if f.SortDir == "" || (f.SortDir != "asc" && f.SortDir != "desc") {
		f.SortDir = "desc"
	}
	return f
}

func (s *DeviceService) applySort(query *gorm.DB, sortBy, sortDir string) *gorm.DB {
	allowedSort := map[string]bool{
		"created_at":     true,
		"name":           true,
		"status":         true,
		"last_heartbeat": true,
	}
	if !allowedSort[sortBy] {
		sortBy = "created_at"
	}
	return query.Order(fmt.Sprintf("%s %s", sortBy, sortDir))
}

func (s *DeviceService) isDeviceOnline(ctx context.Context, deviceID string) bool {
	val, err := s.redisClient.Raw().Get(ctx, deviceOnlinePrefix+deviceID).Result()
	return err == nil && val == "1"
}

func (s *DeviceService) ListDevices(ctx context.Context, tenantID string, filters DeviceListFilters) (*DeviceListResponse, error) {
	filters = s.normalizeFilters(filters)

	query := s.db.Model(&models.Device{}).Where("tenant_id = ?", tenantID)

	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.GroupID != "" {
		if filters.GroupID == "null" || filters.GroupID == "" {
			query = query.Where("group_id IS NULL")
		} else {
			query = query.Where("group_id = ?", filters.GroupID)
		}
	}
	if filters.Search != "" {
		query = query.Where("name ILIKE ?", "%"+filters.Search+"%")
	}

	query = s.applySort(query, filters.SortBy, filters.SortDir)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count devices: %w", err)
	}

	var devices []models.Device
	offset := (filters.Page - 1) * filters.Size
	if err := query.Offset(offset).Limit(filters.Size).Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	result := make([]DeviceWithStatus, 0, len(devices))
	for _, d := range devices {
		dws := DeviceWithStatus{Device: d}
		if d.Status == models.StatusOnline {
			dws.IsOnline = s.isDeviceOnline(ctx, d.ID)
		}
		result = append(result, dws)
	}

	return &DeviceListResponse{
		Devices: result,
		Total:   total,
		Page:    filters.Page,
		Size:    filters.Size,
	}, nil
}

func (s *DeviceService) GetDevice(ctx context.Context, tenantID, deviceID string) (*DeviceWithStatus, error) {
	var device models.Device
	if err := s.db.Where("id = ? AND tenant_id = ?", deviceID, tenantID).First(&device).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("device not found")
		}
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	dws := &DeviceWithStatus{Device: device}
	if device.Status == models.StatusOnline {
		dws.IsOnline = s.isDeviceOnline(ctx, device.ID)
	}

	return dws, nil
}

func (s *DeviceService) UpdateDevice(ctx context.Context, tenantID, deviceID string, input UpdateDeviceInput) (*models.Device, error) {
	var device models.Device
	if err := s.db.Where("id = ? AND tenant_id = ?", deviceID, tenantID).First(&device).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("device not found")
		}
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	updates := make(map[string]any)

	if input.Name != "" {
		updates["name"] = input.Name
	}

	if input.GroupID != nil {
		if *input.GroupID != "" {
			var group models.DeviceGroup
			if err := s.db.Where("id = ? AND tenant_id = ?", *input.GroupID, tenantID).First(&group).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, fmt.Errorf("device group not found or does not belong to tenant")
				}
				return nil, fmt.Errorf("failed to verify device group: %w", err)
			}
			updates["group_id"] = *input.GroupID
		} else {
			updates["group_id"] = nil
		}
	}

	if len(updates) == 0 {
		return &device, nil
	}

	if err := s.db.Model(&device).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update device: %w", err)
	}

	if err := s.db.Where("id = ?", deviceID).First(&device).Error; err != nil {
		return nil, fmt.Errorf("failed to reload device after update: %w", err)
	}

	return &device, nil
}

func (s *DeviceService) DeleteDevice(ctx context.Context, tenantID, deviceID, userID string) error {
	var device models.Device
	if err := s.db.Where("id = ? AND tenant_id = ?", deviceID, tenantID).First(&device).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("device not found")
		}
		return fmt.Errorf("failed to get device: %w", err)
	}

	if s.isDeviceOnline(ctx, device.ID) {
		_ = s.redisClient.Raw().Del(ctx, deviceConnPrefix+device.ID).Err()
	}

	_ = s.redisClient.Raw().Del(ctx, deviceOnlinePrefix+device.ID).Err()

	if err := s.db.Model(&device).Update("deleted_at", time.Now()).Error; err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}

	detailBytes, _ := json.Marshal(map[string]any{
		"device_id":   deviceID,
		"device_name": device.Name,
		"deleted_by":  userID,
	})

	auditLog := models.AuditLog{
		TenantID: tenantID,
		UserID:   userID,
		DeviceID: deviceID,
		Action:   "device_deleted",
		Detail:   datatypes.JSON(detailBytes),
	}
	_ = s.db.Create(&auditLog).Error

	return nil
}

func (s *DeviceService) BatchOperation(ctx context.Context, tenantID string, deviceIDs []string, action string, params map[string]any, userID string) (*BatchResult, error) {
	result := &BatchResult{}

	switch action {
	case "delete":
		if len(deviceIDs) > 100 {
			return nil, fmt.Errorf("batch operation limited to 100 devices at a time")
		}
		for _, id := range deviceIDs {
			if err := s.DeleteDevice(ctx, tenantID, id, userID); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, BatchOperationError{
					DeviceID: id,
					Reason:   err.Error(),
				})
			} else {
				result.Success++
			}
		}

	case "move_to_group":
		groupIDVal, ok := params["group_id"]
		if !ok {
			return nil, fmt.Errorf("group_id is required for move_to_group action")
		}
		groupID, ok := groupIDVal.(string)
		if !ok {
			return nil, fmt.Errorf("group_id must be a string")
		}

		if groupID != "" {
			var group models.DeviceGroup
			if err := s.db.Where("id = ? AND tenant_id = ?", groupID, tenantID).First(&group).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, fmt.Errorf("device group not found or does not belong to tenant")
				}
				return nil, fmt.Errorf("failed to verify device group: %w", err)
			}
		}

		for _, id := range deviceIDs {
			var device models.Device
			if err := s.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&device).Error; err != nil {
				result.Failed++
				result.Errors = append(result.Errors, BatchOperationError{
					DeviceID: id,
					Reason:   "device not found",
				})
				continue
			}

			targetGroupID := groupID
			if targetGroupID == "" {
				if err := s.db.Model(&device).Update("group_id", nil).Error; err != nil {
					result.Failed++
					result.Errors = append(result.Errors, BatchOperationError{
						DeviceID: id,
						Reason:   err.Error(),
					})
					continue
				}
			} else {
				if err := s.db.Model(&device).Update("group_id", targetGroupID).Error; err != nil {
					result.Failed++
					result.Errors = append(result.Errors, BatchOperationError{
						DeviceID: id,
						Reason:   err.Error(),
					})
					continue
				}
			}

			detailBytes, _ := json.Marshal(map[string]any{
				"device_id": id,
				"group_id":  targetGroupID,
			})
			_ = s.db.Create(&models.AuditLog{
				TenantID: tenantID,
				UserID:   userID,
				DeviceID: id,
				Action:   "device_moved_to_group",
				Detail:   datatypes.JSON(detailBytes),
			})

			result.Success++
		}

	default:
		return nil, fmt.Errorf("unsupported batch action: %s", action)
	}

	return result, nil
}
