package services

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

type DataSanitizer struct {
	defaultDate time.Time
}

func NewDataSanitizer() *DataSanitizer {
	return &DataSanitizer{
		defaultDate: time.Now(), // Use current date as default for invalid dates
	}
}

func (ds *DataSanitizer) SanitizeRow(row []interface{}, columns []string) []interface{} {
	sanitized := make([]interface{}, len(row))

	for i, value := range row {
		sanitized[i] = ds.sanitizeValue(value, columns[i])
	}

	return sanitized
}

func (ds *DataSanitizer) sanitizeValue(value interface{}, columnName string) interface{} {
	if value == nil {
		return nil
	}

	if strValue, ok := value.(string); ok {
		// Check for invalid datetime values
		if ds.isInvalidDateTime(strValue) {
			return ds.fixDateTime(strValue, columnName)
		}

		// Check for invalid date values
		if ds.isInvalidDate(strValue) {
			return ds.fixDate(strValue, columnName)
		}
	}

	// Handle byte arrays (common in MySQL drivers)
	if byteValue, ok := value.([]byte); ok {
		strValue := string(byteValue)

		if ds.isInvalidDateTime(strValue) {
			return ds.fixDateTime(strValue, columnName)
		}

		if ds.isInvalidDate(strValue) {
			return ds.fixDate(strValue, columnName)
		}
		return strValue
	}

	// Handle time.Time values
	if timeValue, ok := value.(time.Time); ok {
		// Check if it's zero time or invalid
		if timeValue.IsZero() || timeValue.Year() < 1000 {
			return ds.defaultDate.Format("2006-01-02 15:04:05")
		}
	}

	return value
}

func (ds *DataSanitizer) isInvalidDateTime(value string) bool {
	invalidPatterns := []string{
		"0000-00-00 00:00:00",
		"0000-00-00",
		"1000-01-01 00:00:00", // MySQL minimum datetime
	}

	for _, pattern := range invalidPatterns {
		if strings.HasPrefix(value, pattern) {
			return true
		}
	}

	return false
}

func (ds *DataSanitizer) isInvalidDate(value string) bool {
	invalidPatterns := []string{
		"0000-00-00",
		"1000-01-01", // MySQL minimum date
	}

	for _, pattern := range invalidPatterns {
		if value == pattern {
			return true
		}
	}

	return false
}

func (ds *DataSanitizer) fixDateTime(value, columnName string) string {
	if ds.isCreatedAtField(columnName) {
		return ds.defaultDate.Format("2006-01-02 15:04:05")
	}

	if ds.isUpdatedAtField(columnName) {
		return ds.defaultDate.Format("2006-01-02 15:04:05")
	}

	return "1970-01-01 00:00:01"
}

func (ds *DataSanitizer) fixDate(value, columnName string) string {
	// Use current date for created/updated fields
	if ds.isCreatedAtField(columnName) || ds.isUpdatedAtField(columnName) {
		return ds.defaultDate.Format("2006-01-02")
	}

	// For other date fields, use a reasonable default
	return "1970-01-01"
}

func (ds *DataSanitizer) isCreatedAtField(columnName string) bool {
	lowerName := strings.ToLower(columnName)
	patterns := []string{"created_at", "createdat", "created", "date_created", "creation_date"}

	for _, pattern := range patterns {
		if strings.Contains(lowerName, pattern) {
			return true
		}
	}

	return false
}

func (ds *DataSanitizer) isUpdatedAtField(columnName string) bool {
	lowerName := strings.ToLower(columnName)
	patterns := []string{"updated_at", "updatedat", "updated", "date_updated", "modification_date", "last_modified"}

	for _, pattern := range patterns {
		if strings.Contains(lowerName, pattern) {
			return true
		}
	}

	return false
}

func (ds *DataSanitizer) LogDataIssue(schema, table, column string, originalValue, fixedValue interface{}) {
	fmt.Printf(" Data sanitized in %s.%s.%s: '%v' → '%v'\n",
		schema, table, column, originalValue, fixedValue)
}

func (ds *DataSanitizer) ValidateRowData(row []interface{}, schema, table string, rowID int64) error {
	for i, value := range row {
		if err := ds.validateValue(value, schema, table, fmt.Sprintf("column_%d", i), rowID); err != nil {
			return err
		}
	}
	return nil
}

func (ds *DataSanitizer) validateValue(value interface{}, schema, table, column string, rowID int64) error {
	if value == nil {
		return nil
	}

	if strValue, ok := value.(string); ok {
		if len(strValue) > 65535 { // TEXT field limit
			return fmt.Errorf("value too long in %s.%s.%s at row ID %d: %d characters",
				schema, table, column, rowID, len(strValue))
		}
	}

	// Check for extremely large numbers
	if reflect.TypeOf(value).Kind() == reflect.Float64 {
		if floatValue := value.(float64); floatValue > 1e15 {
			return fmt.Errorf("number too large in %s.%s.%s at row ID %d: %v",
				schema, table, column, rowID, floatValue)
		}
	}

	return nil
}
