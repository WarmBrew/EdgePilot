package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/edge-platform/server/internal/domain/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type DeviceGroupService struct {
	db *gorm.DB
}

func NewDeviceGroupService(db *gorm.DB) *DeviceGroupService {
	return &DeviceGroupService{db: db}
}

type DeviceGroupListResponse struct {
	Groups []DeviceGroupWithCount `json:"groups"`
	Total  int64                  `json:"total"`
}

type DeviceGroupWithCount struct {
	models.DeviceGroup
	DeviceCount int64 `json:"device_count"`
}

type DeviceGroupDetail struct {
	models.DeviceGroup
	DeviceCount int64 `json:"device_count"`
	OnlineCount int64 `json:"online_count"`
}

type CreateGroupInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateGroupInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

type AssignDevicesInput struct {
	DeviceIDs []string `json:"device_ids"`
}

type AssignDevicesResult struct {
	Success int `json:"success"`
	Failed  int `json:"failed"`
}

func (s *DeviceGroupService) ListGroups(ctx context.Context, tenantID string, page, size int) (*DeviceGroupListResponse, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	type groupCount struct {
		models.DeviceGroup
		DeviceCount int64
	}

	var total int64
	if err := s.db.Model(&models.DeviceGroup{}).Where("tenant_id = ?", tenantID).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count groups: %w", err)
	}

	var results []groupCount
	offset := (page - 1) * size
	if err := s.db.Model(&models.DeviceGroup{}).
		Select("device_groups.*, COUNT(devices.id) AS device_count").
		Joins("LEFT JOIN devices ON devices.group_id = device_groups.id AND devices.deleted_at IS NULL").
		Where("device_groups.tenant_id = ?", tenantID).
		Group("device_groups.id").
		Order("device_groups.created_at DESC").
		Offset(offset).
		Limit(size).
		Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	groups := make([]DeviceGroupWithCount, 0, len(results))
	for _, r := range results {
		groups = append(groups, DeviceGroupWithCount{
			DeviceGroup: r.DeviceGroup,
			DeviceCount: r.DeviceCount,
		})
	}

	return &DeviceGroupListResponse{
		Groups: groups,
		Total:  total,
	}, nil
}

func (s *DeviceGroupService) CreateGroup(ctx context.Context, tenantID string, input CreateGroupInput) (*models.DeviceGroup, error) {
	var existing int64
	if err := s.db.Model(&models.DeviceGroup{}).
		Where("tenant_id = ? AND name = ? AND deleted_at IS NULL", tenantID, input.Name).
		Count(&existing).Error; err != nil {
		return nil, fmt.Errorf("failed to check group name uniqueness: %w", err)
	}
	if existing > 0 {
		return nil, fmt.Errorf("group name already exists")
	}

	group := models.DeviceGroup{
		TenantID:    tenantID,
		Name:        input.Name,
		Description: input.Description,
	}
	if err := s.db.Create(&group).Error; err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	return &group, nil
}

func (s *DeviceGroupService) GetGroup(ctx context.Context, tenantID, groupID string) (*DeviceGroupDetail, error) {
	var group models.DeviceGroup
	if err := s.db.Where("id = ? AND tenant_id = ?", groupID, tenantID).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("group not found")
		}
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	type countResult struct {
		Total  int64
		Online int64
	}
	var counts countResult
	if err := s.db.Model(&models.Device{}).
		Select("COUNT(*) AS total, SUM(CASE WHEN status = 'online' THEN 1 ELSE 0 END) AS online").
		Where("group_id = ? AND tenant_id = ? AND deleted_at IS NULL", groupID, tenantID).
		Scan(&counts).Error; err != nil {
		return nil, fmt.Errorf("failed to count devices: %w", err)
	}

	return &DeviceGroupDetail{
		DeviceGroup: group,
		DeviceCount: counts.Total,
		OnlineCount: counts.Online,
	}, nil
}

func (s *DeviceGroupService) UpdateGroup(ctx context.Context, tenantID, groupID string, input UpdateGroupInput) (*models.DeviceGroup, error) {
	var group models.DeviceGroup
	if err := s.db.Where("id = ? AND tenant_id = ?", groupID, tenantID).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("group not found")
		}
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	updates := make(map[string]any)

	if input.Name != nil {
		if *input.Name != "" {
			var existing int64
			if err := s.db.Model(&models.DeviceGroup{}).
				Where("tenant_id = ? AND name = ? AND id != ? AND deleted_at IS NULL", tenantID, *input.Name, groupID).
				Count(&existing).Error; err != nil {
				return nil, fmt.Errorf("failed to check group name uniqueness: %w", err)
			}
			if existing > 0 {
				return nil, fmt.Errorf("group name already exists")
			}
			updates["name"] = *input.Name
		}
	}

	if input.Description != nil {
		updates["description"] = *input.Description
	}

	if len(updates) == 0 {
		return &group, nil
	}

	if err := s.db.Model(&group).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update group: %w", err)
	}

	if err := s.db.Where("id = ?", groupID).First(&group).Error; err != nil {
		return nil, fmt.Errorf("failed to reload group after update: %w", err)
	}

	return &group, nil
}

func (s *DeviceGroupService) DeleteGroup(ctx context.Context, tenantID, groupID string) error {
	var group models.DeviceGroup
	if err := s.db.Where("id = ? AND tenant_id = ?", groupID, tenantID).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("group not found")
		}
		return fmt.Errorf("failed to get group: %w", err)
	}

	var deviceCount int64
	if err := s.db.Model(&models.Device{}).
		Where("group_id = ? AND deleted_at IS NULL", groupID).
		Count(&deviceCount).Error; err != nil {
		return fmt.Errorf("failed to check device count: %w", err)
	}
	if deviceCount > 0 {
		return fmt.Errorf("cannot delete group with %d devices, remove or reassign devices first", deviceCount)
	}

	if err := s.db.Model(&group).Update("deleted_at", time.Now()).Error; err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	return nil
}

func (s *DeviceGroupService) AssignDevices(ctx context.Context, tenantID, groupID string, input AssignDevicesInput) (*AssignDevicesResult, error) {
	var group models.DeviceGroup
	if err := s.db.Where("id = ? AND tenant_id = ?", groupID, tenantID).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("group not found")
		}
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	result := &AssignDevicesResult{}

	for _, deviceID := range input.DeviceIDs {
		var device models.Device
		if err := s.db.Where("id = ? AND tenant_id = ?", deviceID, tenantID).First(&device).Error; err != nil {
			result.Failed++
			continue
		}

		if err := s.db.Model(&device).Update("group_id", groupID).Error; err != nil {
			result.Failed++
			continue
		}

		detailBytes, _ := json.Marshal(map[string]any{
			"device_id":  deviceID,
			"group_id":   groupID,
			"group_name": group.Name,
		})
		_ = s.db.Create(&models.AuditLog{
			TenantID: tenantID,
			UserID:   "",
			DeviceID: deviceID,
			Action:   "device_assigned_to_group",
			Detail:   datatypes.JSON(detailBytes),
		})

		result.Success++
	}

	return result, nil
}

func (s *DeviceGroupService) WriteAuditLog(ctx context.Context, tenantID, userID, deviceID, action string, detail any) {
	detailBytes, _ := json.Marshal(detail)
	_ = s.db.Create(&models.AuditLog{
		TenantID: tenantID,
		UserID:   userID,
		DeviceID: deviceID,
		Action:   action,
		Detail:   datatypes.JSON(detailBytes),
	})
}

func (s *DeviceGroupService) AuditGroup(ctx context.Context, tenantID, userID, groupID, action string, detail any) {
	detailBytes, _ := json.Marshal(detail)
	_ = s.db.Create(&models.AuditLog{
		TenantID: tenantID,
		UserID:   userID,
		DeviceID: groupID,
		Action:   action,
		Detail:   datatypes.JSON(detailBytes),
	})
}
