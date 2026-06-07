package models

import "time"

type Checkpoint struct {
	Schema          string    `json:"schema"`
	Table           string    `json:"table"`
	LastProcessedID int64     `json:"last_processed_id"`
	TotalRows       int64     `json:"total_rows"`
	ProcessedRows   int64     `json:"processed_rows"`
	StartTime       time.Time `json:"start_time"`
	LastUpdateTime  time.Time `json:"last_update_time"`
	IsCompleted     bool      `json:"is_completed"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	RetryCount      int       `json:"retry_count"`
}

type MigrationState struct {
	StartTime       time.Time             `json:"start_time"`
	LastUpdateTime  time.Time             `json:"last_update_time"`
	CompletedTables []string              `json:"completed_tables"`
	CurrentTable    string                `json:"current_table"`
	Checkpoints     map[string]Checkpoint `json:"checkpoints"` // key: schema.table
	TotalSchemas    int                   `json:"total_schemas"`
	TotalTables     int                   `json:"total_tables"`
	CompletedCount  int                   `json:"completed_count"`
	ErrorCount      int                   `json:"error_count"`
}
