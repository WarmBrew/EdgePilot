package middleware

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	minPageSize     = 1
)

var (
	uuidRegex          = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	nullByteRegex      = regexp.MustCompile(`\x00`)
	pathTraversalRegex = regexp.MustCompile(`\.\.[\\/]`)
)

// Validator defines a function that validates a single string value.
// Returns an error message if validation fails, or nil if valid.
type Validator func(value string) error

// ValidateUUID validates that the value is a valid UUID format.
func ValidateUUID(value string) error {
	if value == "" {
		return fmt.Errorf("device_id is required")
	}
	if !uuidRegex.MatchString(value) {
		return fmt.Errorf("device_id must be a valid UUID format")
	}
	return nil
}

// ValidateFilePath validates that the file path does not contain path traversal or null bytes.
func ValidateFilePath(value string) error {
	if value == "" {
		return fmt.Errorf("file path is required")
	}
	if nullByteRegex.MatchString(value) {
		return fmt.Errorf("file path contains invalid characters")
	}
	if pathTraversalRegex.MatchString(value) {
		return fmt.Errorf("file path contains invalid traversal sequence")
	}
	if strings.HasPrefix(value, "/") && len(value) > 1 && strings.HasPrefix(value, "/..") {
		return fmt.Errorf("file path contains invalid traversal sequence")
	}
	return nil
}

// ValidatePageSize validates and normalizes page size query parameter.
func ValidatePageSize(value string) error {
	return nil
}

// ParseAndValidatePageSize parses and normalizes page size, returning the validated value.
func ParseAndValidatePageSize(value string) (int, error) {
	if value == "" {
		return defaultPageSize, nil
	}

	var pageSize int
	if _, err := fmt.Sscanf(value, "%d", &pageSize); err != nil {
		return 0, fmt.Errorf("page_size must be a valid number")
	}

	if pageSize < minPageSize {
		return minPageSize, nil
	}
	if pageSize > maxPageSize {
		return maxPageSize, nil
	}
	return pageSize, nil
}

// ValidateSortField validates that the sort field is in the allowed whitelist.
func ValidateSortField(allowedFields []string) Validator {
	allowed := make(map[string]bool, len(allowedFields))
	for _, field := range allowedFields {
		allowed[strings.ToLower(field)] = true
	}

	return func(value string) error {
		if value == "" {
			return nil
		}
		field := strings.ToLower(strings.TrimSpace(value))
		if !allowed[field] {
			return fmt.Errorf("sort field '%s' is not allowed, allowed fields: %s", value, strings.Join(allowedFields, ", "))
		}
		return nil
	}
}

// ValidateQueryParams creates a gin middleware that validates query parameters
// against a map of parameter name to validator functions.
func ValidateQueryParams(rules map[string]Validator) gin.HandlerFunc {
	return func(c *gin.Context) {
		for paramName, validator := range rules {
			value := c.Query(paramName)
			if err := validator(value); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":     "INVALID_QUERY_PARAM",
					"message":   fmt.Sprintf("invalid parameter '%s': %s", paramName, err.Error()),
					"parameter": paramName,
				})
				c.Abort()
				return
			}
		}

		if validator, exists := rules["page_size"]; exists && validator != nil {
			value := c.Query("page_size")
			if value != "" {
				if _, err := ParseAndValidatePageSize(value); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"error":     "INVALID_QUERY_PARAM",
						"message":   fmt.Sprintf("invalid parameter 'page_size': %s", err.Error()),
						"parameter": "page_size",
					})
					c.Abort()
					return
				}
			}
		}

		c.Next()
	}
}

// DefaultValidators provides commonly used validator presets.
var DefaultValidators = struct {
	DeviceID Validator
	FilePath Validator
	PageNum  Validator
	PageSize Validator
}{
	DeviceID: ValidateUUID,
	FilePath: ValidateFilePath,
	PageNum: func(value string) error {
		if value == "" {
			return nil
		}
		var pageNum int
		if _, err := fmt.Sscanf(value, "%d", &pageNum); err != nil {
			return fmt.Errorf("page must be a valid number")
		}
		if pageNum < 1 {
			return fmt.Errorf("page must be greater than 0")
		}
		return nil
	},
	PageSize: func(value string) error {
		if value == "" {
			return nil
		}
		_, err := ParseAndValidatePageSize(value)
		return err
	},
}
