package handlers

import (
	"net/http"
	"strconv"

	"github.com/edge-platform/server/internal/api/middleware"
	"github.com/edge-platform/server/internal/domain/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DeviceGroupHandler struct {
	db  *gorm.DB
	svc *service.DeviceGroupService
}

func NewDeviceGroupHandler(db *gorm.DB) *DeviceGroupHandler {
	return &DeviceGroupHandler{
		db:  db,
		svc: service.NewDeviceGroupService(db),
	}
}

type CreateGroupRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=128"`
	Description string `json:"description" binding:"max=512"`
}

type UpdateGroupRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=1,max=128"`
	Description *string `json:"description" binding:"omitempty,max=512"`
}

type AssignDevicesRequest struct {
	DeviceIDs []string `json:"device_ids" binding:"required"`
}

func (h *DeviceGroupHandler) ListGroups(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	page := 1
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	size := 20
	if s := c.Query("size"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			size = v
		}
	}

	resp, err := h.svc.ListGroups(c.Request.Context(), tenantID, page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list groups"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *DeviceGroupHandler) CreateGroup(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	userID, _ := middleware.GetUserID(c)

	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	input := service.CreateGroupInput{
		Name:        req.Name,
		Description: req.Description,
	}

	group, err := h.svc.CreateGroup(c.Request.Context(), tenantID, input)
	if err != nil {
		if err.Error() == "group name already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create group"})
		return
	}

	h.svc.AuditGroup(c.Request.Context(), tenantID, userID, group.ID, "group_created", map[string]any{
		"group_name": group.Name,
	})

	c.JSON(http.StatusCreated, group)
}

func (h *DeviceGroupHandler) GetGroup(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	groupID := c.Param("id")

	detail, err := h.svc.GetGroup(c.Request.Context(), tenantID, groupID)
	if err != nil {
		if err.Error() == "group not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get group"})
		return
	}

	c.JSON(http.StatusOK, detail)
}

func (h *DeviceGroupHandler) UpdateGroup(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	userID, _ := middleware.GetUserID(c)
	groupID := c.Param("id")

	var req UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Name == nil && req.Description == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one field must be provided for update"})
		return
	}

	input := service.UpdateGroupInput{
		Name:        req.Name,
		Description: req.Description,
	}

	group, err := h.svc.UpdateGroup(c.Request.Context(), tenantID, groupID, input)
	if err != nil {
		if err.Error() == "group not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
			return
		}
		if err.Error() == "group name already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update group"})
		return
	}

	h.svc.AuditGroup(c.Request.Context(), tenantID, userID, groupID, "group_updated", map[string]any{
		"group_id":   groupID,
		"group_name": group.Name,
	})

	c.JSON(http.StatusOK, group)
}

func (h *DeviceGroupHandler) DeleteGroup(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	userID, _ := middleware.GetUserID(c)
	groupID := c.Param("id")

	var groupDetail *service.DeviceGroupDetail
	groupDetail, _ = h.svc.GetGroup(c.Request.Context(), tenantID, groupID)

	if err := h.svc.DeleteGroup(c.Request.Context(), tenantID, groupID); err != nil {
		if err.Error() == "group not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
			return
		}
		if err.Error() == "cannot delete group with devices, remove or reassign devices first" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete group"})
		return
	}

	if groupDetail != nil {
		h.svc.AuditGroup(c.Request.Context(), tenantID, userID, groupID, "group_deleted", map[string]any{
			"group_id":   groupID,
			"group_name": groupDetail.Name,
		})
	}

	c.Status(http.StatusNoContent)
}

func (h *DeviceGroupHandler) AssignDevices(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	groupID := c.Param("id")

	var req AssignDevicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if len(req.DeviceIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_ids cannot be empty"})
		return
	}

	input := service.AssignDevicesInput{
		DeviceIDs: req.DeviceIDs,
	}

	result, err := h.svc.AssignDevices(c.Request.Context(), tenantID, groupID, input)
	if err != nil {
		if err.Error() == "group not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign devices"})
		return
	}

	c.JSON(http.StatusOK, result)
}
