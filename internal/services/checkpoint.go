package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/kathedrall/mysql-twin-backup/internal/models"
)

const checkpointFile = "migration_checkpoint.json"

type CheckpointService struct {
	state *models.MigrationState
	mutex sync.RWMutex
}

func NewCheckpointService() *CheckpointService {
	service := &CheckpointService{
		state: &models.MigrationState{
			StartTime:       time.Now(),
			LastUpdateTime:  time.Now(),
			CompletedTables: make([]string, 0),
			Checkpoints:     make(map[string]models.Checkpoint),
		},
	}

	if err := service.LoadState(); err != nil {
		log.Printf("No existing checkpoint found, starting fresh: %v", err)
	}

	return service
}

func (c *CheckpointService) LoadState() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, err := os.Stat(checkpointFile); os.IsNotExist(err) {
		return fmt.Errorf("checkpoint file does not exist")
	}

	data, err := os.ReadFile(checkpointFile)
	if err != nil {
		return fmt.Errorf("failed to read checkpoint file: %w", err)
	}

	if err := json.Unmarshal(data, c.state); err != nil {
		return fmt.Errorf("failed to unmarshal checkpoint data: %w", err)
	}

	log.Printf("Loaded checkpoint: %d tables completed, current table: %s",
		c.state.CompletedCount, c.state.CurrentTable)

	return nil
}

func (c *CheckpointService) SaveState() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.state.LastUpdateTime = time.Now()

	data, err := json.MarshalIndent(c.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint data: %w", err)
	}

	if err := os.WriteFile(checkpointFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint file: %w", err)
	}

	return nil
}

func (c *CheckpointService) SetTotalCounts(schemas, tables int) {
	c.mutex.Lock()
	c.state.TotalSchemas = schemas
	c.state.TotalTables = tables
	c.mutex.Unlock()
	c.SaveState()
}

func (c *CheckpointService) StartTable(schema, table string, totalRows int64) {
	key := fmt.Sprintf("%s.%s", schema, table)

	c.mutex.Lock()
	c.state.CurrentTable = key

	checkpoint := models.Checkpoint{
		Schema:         schema,
		Table:          table,
		TotalRows:      totalRows,
		ProcessedRows:  0,
		StartTime:      time.Now(),
		LastUpdateTime: time.Now(),
		IsCompleted:    false,
		RetryCount:     0,
	}

	if existing, exists := c.state.Checkpoints[key]; exists {
		checkpoint.LastProcessedID = existing.LastProcessedID
		checkpoint.ProcessedRows = existing.ProcessedRows
		checkpoint.RetryCount = existing.RetryCount + 1
		log.Printf("Resuming table %s from ID %d (attempt %d)", key, existing.LastProcessedID, checkpoint.RetryCount)
	}

	c.state.Checkpoints[key] = checkpoint
	c.mutex.Unlock()
	c.SaveState()
}

func (c *CheckpointService) UpdateProgress(schema, table string, lastID, processedRows int64) {
	key := fmt.Sprintf("%s.%s", schema, table)

	c.mutex.Lock()
	if checkpoint, exists := c.state.Checkpoints[key]; exists {
		checkpoint.LastProcessedID = lastID
		checkpoint.ProcessedRows = processedRows
		checkpoint.LastUpdateTime = time.Now()
		c.state.Checkpoints[key] = checkpoint
		shouldSave := processedRows%1000 == 0
		c.mutex.Unlock()
		if shouldSave {
			c.SaveState()
		}
	} else {
		c.mutex.Unlock()
	}
}

func (c *CheckpointService) CompleteTable(schema, table string) {
	key := fmt.Sprintf("%s.%s", schema, table)

	c.mutex.Lock()
	if checkpoint, exists := c.state.Checkpoints[key]; exists {
		checkpoint.IsCompleted = true
		checkpoint.LastUpdateTime = time.Now()
		c.state.Checkpoints[key] = checkpoint
	}

	c.state.CompletedTables = append(c.state.CompletedTables, key)
	c.state.CompletedCount++
	c.state.CurrentTable = ""
	completedCount := c.state.CompletedCount
	totalTables := c.state.TotalTables
	c.mutex.Unlock()

	log.Printf("Completed table %s (%d/%d tables)", key, completedCount, totalTables)
	c.SaveState()
}

func (c *CheckpointService) RecordError(schema, table string, lastID int64, errorMsg string) {
	key := fmt.Sprintf("%s.%s", schema, table)

	c.mutex.Lock()
	if checkpoint, exists := c.state.Checkpoints[key]; exists {
		checkpoint.ErrorMessage = errorMsg
		checkpoint.LastProcessedID = lastID
		checkpoint.LastUpdateTime = time.Now()
		c.state.Checkpoints[key] = checkpoint
	}

	c.state.ErrorCount++
	c.mutex.Unlock()

	log.Printf("Error in table %s at ID %d: %s", key, lastID, errorMsg)
	c.SaveState()
}

func (c *CheckpointService) IsTableCompleted(schema, table string) bool {
	key := fmt.Sprintf("%s.%s", schema, table)

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for _, completed := range c.state.CompletedTables {
		if completed == key {
			return true
		}
	}

	if checkpoint, exists := c.state.Checkpoints[key]; exists {
		return checkpoint.IsCompleted
	}

	return false
}

func (c *CheckpointService) GetLastProcessedID(schema, table string) int64 {
	key := fmt.Sprintf("%s.%s", schema, table)

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if checkpoint, exists := c.state.Checkpoints[key]; exists {
		return checkpoint.LastProcessedID
	}

	return 0
}

func (c *CheckpointService) GetProgress() (int, int, int) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.state.CompletedCount, c.state.TotalTables, c.state.ErrorCount
}

func (c *CheckpointService) CleanupCheckpoint() error {
	if _, err := os.Stat(checkpointFile); err == nil {
		if err := os.Remove(checkpointFile); err != nil {
			return fmt.Errorf("failed to remove checkpoint file: %w", err)
		}
		log.Printf("Checkpoint file removed after successful migration")
	}
	return nil
}

func (c *CheckpointService) PrintSummary() {
	c.mutex.RLock()
	duration := time.Since(c.state.StartTime)
	totalSchemas := c.state.TotalSchemas
	totalTables := c.state.TotalTables
	completedCount := c.state.CompletedCount
	errorCount := c.state.ErrorCount

	var errorTables []string
	for key, checkpoint := range c.state.Checkpoints {
		if checkpoint.ErrorMessage != "" {
			errorTables = append(errorTables, fmt.Sprintf("  - %s: %s", key, checkpoint.ErrorMessage))
		}
	}
	c.mutex.RUnlock()

	fmt.Printf("\nMigration Summary:\n")
	fmt.Printf("==============================\n")
	fmt.Printf("Duration: %v\n", duration.Round(time.Second))
	fmt.Printf("Schemas: %d\n", totalSchemas)
	fmt.Printf("Tables: %d total, %d completed\n", totalTables, completedCount)

	if errorCount > 0 {
		fmt.Printf("Errors: %d\n", errorCount)
		fmt.Printf("\nTables with errors:\n")
		for _, errorInfo := range errorTables {
			fmt.Printf("%s\n", errorInfo)
		}
	}

	if completedCount < totalTables {
		fmt.Printf("\nTo resume migration, run the same command again.\n")
	}

	fmt.Printf("==============================\n")
}
