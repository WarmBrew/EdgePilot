package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/edge-platform/server/internal/domain/models"
	pkgAudit "github.com/edge-platform/server/internal/pkg/audit"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	defaultAuditPage = 1
	defaultAuditSize = 20
	maxAuditSize     = 100
	exportDir        = "/tmp/audit-exports"
	exportTTL        = 1 * time.Hour
)

// AuditLogFilter holds query parameters for filtering audit logs.
type AuditLogFilter struct {
	UserID    *string
	DeviceID  *string
	Action    *string
	StartTime *time.Time
	EndTime   *time.Time
	Page      int
	Size      int
	SortBy    string
	SortDir   string
}

// AuditLogListResponse wraps paginated audit log results.
type AuditLogListResponse struct {
	Logs  []MaskedAuditLog `json:"logs"`
	Total int64            `json:"total"`
	Page  int              `json:"page"`
	Size  int              `json:"size"`
}

// MaskedAuditLog is an audit log with sensitive fields masked.
type MaskedAuditLog struct {
	ID        uint            `json:"id"`
	TenantID  string          `json:"tenant_id"`
	UserID    string          `json:"user_id"`
	DeviceID  string          `json:"device_id"`
	Action    string          `json:"action"`
	Detail    json.RawMessage `json:"detail"`
	IPAddress string          `json:"ip_address,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// ExportResult holds the outcome of an export operation.
type ExportResult struct {
	ExportID    string    `json:"export_id"`
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// ExportRequest represents the incoming export request body.
type ExportRequest struct {
	Format  string         `json:"format"`
	Filters AuditLogFilter `json:"filters"`
}

// AuditQueryService provides read and export operations for audit logs.
type AuditQueryService struct {
	db       *gorm.DB
	auditSvc *AuditService
}

// NewAuditQueryService creates a new query service.
func NewAuditQueryService(db *gorm.DB, auditSvc *AuditService) *AuditQueryService {
	return &AuditQueryService{db: db, auditSvc: auditSvc}
}

// normalizeFilter applies defaults and caps to filter values.
func normalizeFilter(f AuditLogFilter) AuditLogFilter {
	if f.Page < 1 {
		f.Page = defaultAuditPage
	}
	if f.Size < 1 {
		f.Size = defaultAuditSize
	}
	if f.Size > maxAuditSize {
		f.Size = maxAuditSize
	}
	if f.SortBy == "" {
		f.SortBy = "created_at"
	}
	if f.SortDir == "" {
		f.SortDir = "desc"
	}
	return f
}

// allowedSortFields defines which columns can be used for sorting.
var allowedAuditSortFields = map[string]bool{
	"created_at": true,
	"action":     true,
	"user_id":    true,
	"device_id":  true,
}

// ListLogs returns a paginated, tenant-scoped list of audit logs with masked sensitive data.
func (s *AuditQueryService) ListLogs(ctx context.Context, tenantID string, filter AuditLogFilter) (*AuditLogListResponse, error) {
	filter = normalizeFilter(filter)

	query := s.db.WithContext(ctx).Model(&models.AuditLog{}).Where("tenant_id = ?", tenantID)
	query = s.applyFilters(query, filter)
	query = s.applySort(query, filter.SortBy, filter.SortDir)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count audit logs: %w", err)
	}

	var logs []models.AuditLog
	offset := (filter.Page - 1) * filter.Size
	if err := query.Offset(offset).Limit(filter.Size).Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}

	masked := make([]MaskedAuditLog, len(logs))
	for i, log := range logs {
		masked[i] = s.maskLog(log, false)
	}

	return &AuditLogListResponse{
		Logs:  masked,
		Total: total,
		Page:  filter.Page,
		Size:  filter.Size,
	}, nil
}

// GetLogDetail returns a single audit log by ID with tenant verification and masked data.
func (s *AuditQueryService) GetLogDetail(ctx context.Context, tenantID, logID string) (*MaskedAuditLog, error) {
	var log models.AuditLog
	if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", logID, tenantID).First(&log).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("audit log not found")
		}
		return nil, fmt.Errorf("failed to get audit log: %w", err)
	}

	masked := s.maskLog(log, false)
	return &masked, nil
}

// ExportLogs generates an export file asynchronously and returns export metadata.
func (s *AuditQueryService) ExportLogs(ctx context.Context, tenantID, userID string, filter AuditLogFilter, format string) (*ExportResult, error) {
	exportID := uuid.New().String()
	ext := "json"
	if format == "csv" {
		ext = "csv"
	}

	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create export directory: %w", err)
	}

	filePath := filepath.Join(exportDir, exportID+"."+ext)

	query := s.db.WithContext(ctx).Model(&models.AuditLog{}).Where("tenant_id = ?", tenantID)
	query = s.applyFilters(query, filter)

	switch ext {
	case "csv":
		if err := s.exportToCSV(ctx, query, filePath); err != nil {
			return nil, fmt.Errorf("failed to export CSV: %w", err)
		}
	default:
		if err := s.exportToJSON(ctx, query, filePath); err != nil {
			return nil, fmt.Errorf("failed to export JSON: %w", err)
		}
	}

	expiresAt := time.Now().Add(exportTTL)

	if s.auditSvc != nil {
		detailJSON, _ := json.Marshal(map[string]interface{}{
			"export_id": exportID,
			"format":    format,
			"file_path": filePath,
			"filters":   filter,
		})
		_ = s.auditSvc.Log(ctx, &models.AuditLog{
			TenantID: tenantID,
			UserID:   userID,
			DeviceID: "N/A",
			Action:   "export_audit_logs",
			Detail:   datatypes.JSON(detailJSON),
		})
	}

	return &ExportResult{
		ExportID:    exportID,
		DownloadURL: "/api/v1/audit/exports/" + exportID,
		ExpiresAt:   expiresAt,
	}, nil
}

// DownloadExport streams an export file and returns the file path for the handler to serve.
// Returns the file path, content type, and any error.
func (s *AuditQueryService) DownloadExport(exportID string) (filePath, contentType string, err error) {
	for _, ext := range []string{"json", "csv"} {
		p := filepath.Join(exportDir, exportID+"."+ext)
		if _, statErr := os.Stat(p); statErr == nil {
			filePath = p
			if ext == "csv" {
				contentType = "text/csv"
			} else {
				contentType = "application/json"
			}
			return
		}
	}
	return "", "", fmt.Errorf("export not found or expired")
}

// DeleteExport removes an export file after download.
func (s *AuditQueryService) DeleteExport(exportID string) error {
	for _, ext := range []string{"json", "csv"} {
		p := filepath.Join(exportDir, exportID+"."+ext)
		_ = os.Remove(p)
	}
	return nil
}

// ---- internal helpers ----

func (s *AuditQueryService) applyFilters(query *gorm.DB, f AuditLogFilter) *gorm.DB {
	if f.UserID != nil && *f.UserID != "" {
		query = query.Where("user_id = ?", *f.UserID)
	}
	if f.DeviceID != nil && *f.DeviceID != "" {
		query = query.Where("device_id = ?", *f.DeviceID)
	}
	if f.Action != nil && *f.Action != "" {
		query = query.Where("action = ?", *f.Action)
	}
	if f.StartTime != nil {
		query = query.Where("created_at >= ?", *f.StartTime)
	}
	if f.EndTime != nil {
		query = query.Where("created_at <= ?", *f.EndTime)
	}
	return query
}

func (s *AuditQueryService) applySort(query *gorm.DB, sortBy, sortDir string) *gorm.DB {
	if !allowedAuditSortFields[sortBy] {
		sortBy = "created_at"
	}
	if sortDir != "asc" && sortDir != "desc" {
		sortDir = "desc"
	}
	return query.Order(fmt.Sprintf("%s %s", sortBy, sortDir))
}

// maskLog applies data masking rules to an audit log.
func (s *AuditQueryService) maskLog(log models.AuditLog, isAdmin bool) MaskedAuditLog {
	result := MaskedAuditLog{
		ID:        log.ID,
		TenantID:  log.TenantID,
		UserID:    log.UserID,
		DeviceID:  log.DeviceID,
		Action:    log.Action,
		CreatedAt: log.CreatedAt,
	}

	if !isAdmin {
		result.IPAddress = pkgAudit.MaskIPAddr(log.IPAddress)
	} else {
		result.IPAddress = log.IPAddress
	}

	result.Detail = s.maskDetail(log.Detail, log.Action, isAdmin)
	return result
}

// maskDetail applies masking to the JSONB detail field based on action type.
func (s *AuditQueryService) maskDetail(raw datatypes.JSON, action string, isAdmin bool) json.RawMessage {
	if len(raw) == 0 || string(raw) == "{}" {
		return json.RawMessage(raw)
	}

	var detail map[string]interface{}
	if err := json.Unmarshal(raw, &detail); err != nil {
		return json.RawMessage(raw)
	}

	switch action {
	case models.ActionExecCommand, models.ActionTerminalOpen, models.ActionTerminalClose:
		if cmd, ok := detail["command"].(string); ok && !isAdmin {
			detail["command"] = pkgAudit.MaskString(cmd)
		}
		if cmd, ok := detail["cmd"].(string); ok && !isAdmin {
			detail["cmd"] = pkgAudit.MaskString(cmd)
		}

	case models.ActionUpload, models.ActionDownload, models.ActionEdit,
		models.ActionFileRead, models.ActionFileWrite, models.ActionFileUpload:
		if _, ok := detail["content"].(string); ok && !isAdmin {
			detail["content"] = "[masked]"
		}
		if size, ok := detail["size"].(float64); ok {
			detail["content"] = pkgAudit.MaskFileContent(int(size))
		}
		if size, ok := detail["size_bytes"].(float64); ok {
			detail["content"] = pkgAudit.MaskFileContent(int(size))
		}

	case models.ActionFileChmod, models.ActionFileChown:
		// No sensitive content expected, leave as-is
	}

	if !isAdmin {
		if token, ok := detail["token"].(string); ok {
			detail["token"] = pkgAudit.MaskToken(token)
		}
		if password, ok := detail["password"].(string); ok {
			detail["password"] = pkgAudit.MaskPassword(password)
		}
	}

	masked, _ := json.Marshal(detail)
	return json.RawMessage(masked)
}

// exportToJSON streams audit logs matching the query to a JSON file.
func (s *AuditQueryService) exportToJSON(ctx context.Context, query *gorm.DB, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString("[\n")

	var logs []models.AuditLog
	cursor := uint(0)
	batchSize := 500
	first := true

	for {
		batchQuery := query.WithContext(ctx).Where("id > ?", cursor).Order("id ASC").Limit(batchSize)
		if err := batchQuery.Find(&logs).Error; err != nil {
			return err
		}
		if len(logs) == 0 {
			break
		}

		for _, log := range logs {
			if !first {
				f.WriteString(",\n")
			}
			first = false

			masked := s.maskLog(log, false)
			data, err := json.Marshal(masked)
			if err != nil {
				return err
			}
			f.Write(data)

			cursor = log.ID
		}

		if len(logs) < batchSize {
			break
		}
	}

	f.WriteString("\n]")
	return nil
}

// exportToCSV streams audit logs matching the query to a CSV file.
func (s *AuditQueryService) exportToCSV(ctx context.Context, query *gorm.DB, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString("id,tenant_id,user_id,device_id,action,detail,ip_address,created_at\n")

	var logs []models.AuditLog
	cursor := uint(0)
	batchSize := 500

	for {
		batchQuery := query.WithContext(ctx).Where("id > ?", cursor).Order("id ASC").Limit(batchSize)
		if err := batchQuery.Find(&logs).Error; err != nil {
			return err
		}
		if len(logs) == 0 {
			break
		}

		for _, log := range logs {
			masked := s.maskLog(log, false)
			detailStr := csvEscape(string(masked.Detail))
			line := fmt.Sprintf("%d,%s,%s,%s,%s,%s,%s,%s\n",
				masked.ID,
				csvEscape(masked.TenantID),
				csvEscape(masked.UserID),
				csvEscape(masked.DeviceID),
				csvEscape(masked.Action),
				detailStr,
				csvEscape(masked.IPAddress),
				masked.CreatedAt.Format(time.RFC3339),
			)
			f.WriteString(line)

			cursor = log.ID
		}

		if len(logs) < batchSize {
			break
		}
	}

	return nil
}

// csvEscape wraps a field in quotes if it contains commas, quotes, or newlines.
func csvEscape(s string) string {
	needsQuote := false
	for _, c := range s {
		if c == ',' || c == '"' || c == '\n' || c == '\r' {
			needsQuote = true
			break
		}
	}
	if !needsQuote {
		return s
	}
	return "\"" + replaceQuotes(s) + "\""
}

func replaceQuotes(s string) string {
	result := make([]byte, 0, len(s)+4)
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			result = append(result, '"', '"')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}
