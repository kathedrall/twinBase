package services

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/kathedrall/twinBase/internal/database"
	"github.com/kathedrall/twinBase/internal/models"
)

type TableJob struct {
	Table models.Table
	Index int
	Total int
}

type TableResult struct {
	Table models.Table
	Error error
	Index int
}

type MigrationService struct {
	sourceDB      *sql.DB
	destDB        *sql.DB
	dbManager     *database.Manager
	bufferSize    int
	maxGoroutines int
	checkpoint    *CheckpointService
	sanitizer     *DataSanitizer
}

func NewMigrationService(sourceDB, destDB *sql.DB, bufferSize, maxGoroutines int) *MigrationService {
	return &MigrationService{
		sourceDB:      sourceDB,
		destDB:        destDB,
		dbManager:     nil, // Will be set via SetDatabaseManager
		bufferSize:    bufferSize,
		maxGoroutines: maxGoroutines,
		checkpoint:    NewCheckpointService(),
		sanitizer:     NewDataSanitizer(),
	}
}

func (m *MigrationService) SetDatabaseManager(dbManager *database.Manager) {
	m.dbManager = dbManager
}

func (m *MigrationService) querySource(query string, args ...interface{}) (*sql.Rows, error) {
	if m.dbManager != nil {
		return m.dbManager.QuerySourceWithLog(query, args...)
	}
	return m.sourceDB.Query(query, args...)
}

func (m *MigrationService) queryRowSource(query string, args ...interface{}) *sql.Row {
	if m.dbManager != nil {
		return m.dbManager.QueryRowSourceWithLog(query, args...)
	}
	return m.sourceDB.QueryRow(query, args...)
}

func (m *MigrationService) queryDest(query string, args ...interface{}) (*sql.Rows, error) {
	if m.dbManager != nil {
		return m.dbManager.QueryDestWithLog(query, args...)
	}
	return m.destDB.Query(query, args...)
}

func (m *MigrationService) queryRowDest(query string, args ...interface{}) *sql.Row {
	if m.dbManager != nil {
		return m.dbManager.QueryRowDestWithLog(query, args...)
	}
	return m.destDB.QueryRow(query, args...)
}

func (m *MigrationService) execDest(query string, args ...interface{}) (sql.Result, error) {
	if m.dbManager != nil {
		return m.dbManager.ExecDestWithLog(query, args...)
	}
	return m.destDB.Exec(query, args...)
}

func (m *MigrationService) MigrateData(schemas []models.Schema) error {
	var allTables []models.Table
	totalTables := 0

	for _, schema := range schemas {
		for _, table := range schema.Tables {
			allTables = append(allTables, table)
		}
		totalTables += len(schema.Tables)
	}

	m.checkpoint.SetTotalCounts(len(schemas), totalTables)
	m.printInitialDashboard(len(schemas), totalTables)

	log.Printf("Switching to PARALLEL processing for better performance...")

	if err := m.MigrateAllTablesParallel(schemas); err != nil {
		m.checkpoint.PrintSummary()
		return fmt.Errorf("parallel migration failed: %w", err)
	}

	log.Printf("Data migration completed successfully!")
	m.checkpoint.CleanupCheckpoint()
	m.checkpoint.PrintSummary()
	return nil
}

func (m *MigrationService) MigrateSchema(schema models.Schema) error {
	semaphore := make(chan struct{}, m.maxGoroutines)
	var wg sync.WaitGroup
	errChan := make(chan error, len(schema.Tables))

	for _, table := range schema.Tables {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(t models.Table) {
			defer func() {
				<-semaphore
				wg.Done()
			}()

			if err := m.MigrateTable(t); err != nil {
				errChan <- fmt.Errorf("failed to migrate table %s.%s: %w", t.Schema, t.Name, err)
				return
			}

			log.Printf("Successfully migrated table: %s.%s", t.Schema, t.Name)
		}(table)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *MigrationService) MigrateTable(table models.Table) error {
	if m.checkpoint.IsTableCompleted(table.Schema, table.Name) {
		log.Printf("Table %s.%s already completed, skipping", table.Schema, table.Name)
		return nil
	}

	log.Printf("Starting migration of table: %s.%s", table.Schema, table.Name)

	if err := m.preflightTableCount(table.Schema, table.Name); err != nil {
		log.Printf("Preflight check failed for %s.%s: %v", table.Schema, table.Name, err)
	}

	totalRows, err := m.getRowCount(table.Schema, table.Name)
	if err != nil {
		return fmt.Errorf("failed to get row count: %w", err)
	}

	if totalRows == 0 {
		log.Printf("Table %s.%s is empty, skipping", table.Schema, table.Name)
		m.checkpoint.CompleteTable(table.Schema, table.Name)
		return nil
	}

	m.checkpoint.StartTable(table.Schema, table.Name, totalRows)
	log.Printf("Table %s.%s has %d rows to migrate", table.Schema, table.Name, totalRows)
	checkpointOffset := m.checkpoint.GetLastProcessedID(table.Schema, table.Name)
	actualOffset, err := m.getLastInsertedID(table.Schema, table.Name)
	if err != nil {
		log.Printf("Warning: could not get last inserted ID, using checkpoint: %v", err)
		actualOffset = checkpointOffset
	}

	startOffset := actualOffset
	if checkpointOffset > actualOffset {
		log.Printf("Checkpoint shows %d but destination has %d, using destination value", checkpointOffset, actualOffset)
	}

	if startOffset > 0 {
		log.Printf("Resuming from offset %d (verified from destination)", startOffset)
	}

	if startOffset == 0 {
		if err := m.truncateTable(table.Schema, table.Name); err != nil {
			return fmt.Errorf("failed to truncate destination table: %w", err)
		}
	}

	var (
		offset        int64 = startOffset
		processedRows int64 = startOffset
	)

	for offset < totalRows {
		batchSize := m.bufferSize
		if offset+int64(batchSize) > totalRows {
			batchSize = int(totalRows - offset)
		}

		batch, err := m.readBatch(table, offset, batchSize)
		if err != nil {
			errMsg := fmt.Sprintf("failed to read batch at offset %d: %v", offset, err)
			m.checkpoint.RecordError(table.Schema, table.Name, offset, errMsg)
			return fmt.Errorf("failed to read batch at offset %d: %w", offset, err)
		}

		if err := m.writeBatch(batch); err != nil {
			errMsg := fmt.Sprintf("failed to write batch at offset %d: %v", offset, err)
			m.checkpoint.RecordError(table.Schema, table.Name, offset, errMsg)
			return fmt.Errorf("failed to write batch at offset %d: %w", offset, err)
		}

		processedRows += int64(len(batch.Rows))
		offset += int64(batchSize)

		m.checkpoint.UpdateProgress(table.Schema, table.Name, offset, processedRows)
		progress := float64(processedRows) / float64(totalRows) * 100
		m.printBatchProgress(table.Schema, table.Name, progress, processedRows, totalRows, offset, int64(batchSize))
		time.Sleep(10 * time.Millisecond)
	}

	log.Printf("Completed migration of table %s.%s (%d rows)", table.Schema, table.Name, processedRows)

	destCount, err := m.getDestinationRowCount(table.Schema, table.Name)
	if err != nil {
		log.Printf("Warning: Could not validate destination row count for %s.%s: %v", table.Schema, table.Name, err)
	} else {
		currentSourceCount, err := m.getRowCount(table.Schema, table.Name)
		if err != nil {
			log.Printf("Warning: Could not get current source count for validation: %v", err)
		} else {
			variance := int64(math.Max(100, float64(currentSourceCount)*0.001))

			if destCount < currentSourceCount-variance {
				log.Printf("VALIDATION WARNING: %s.%s destination has %d rows but source has %d (difference: %d)",
					table.Schema, table.Name, destCount, currentSourceCount, currentSourceCount-destCount)
				log.Printf("This table may need to be re-migrated if the difference is significant")
			} else if destCount > currentSourceCount+variance {
				log.Printf("ℹINFO: %s.%s destination has %d rows, source has %d (destination has more, possibly due to timing)",
					table.Schema, table.Name, destCount, currentSourceCount)
			} else {
				log.Printf("VALIDATION OK: %s.%s row counts match (dest: %d, source: %d)",
					table.Schema, table.Name, destCount, currentSourceCount)
			}
		}
	}
	m.checkpoint.CompleteTable(table.Schema, table.Name)
	return nil
}

func (m *MigrationService) getRowCount(schema, table string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s`.`%s`", schema, table)

	var count int64
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := m.queryRowSource(query).Scan(&count); err != nil {
			if attempt == maxRetries {
				return 0, fmt.Errorf("failed to count rows after %d attempts: %w", maxRetries, err)
			}
			log.Printf("Warning: row count attempt %d failed for %s.%s: %v", attempt, schema, table, err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		if attempt > 1 {
			log.Printf("Row count succeeded on attempt %d for %s.%s: %d rows", attempt, schema, table, count)
		}
		break
	}

	return count, nil
}

func (m *MigrationService) getRowCountWithMaxID(schema, table string) (int64, int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*), IFNULL(MAX(id), 0) FROM `%s`.`%s`", schema, table)

	var count, maxID int64
	if err := m.queryRowSource(query).Scan(&count, &maxID); err != nil {
		return 0, 0, fmt.Errorf("failed to get row count and max ID: %w", err)
	}

	return count, maxID, nil
}

func (m *MigrationService) validateTableRowCount(schema, table string, expectedRows int64) (bool, int64, error) {
	actualRows, err := m.getRowCount(schema, table)
	if err != nil {
		return false, 0, err
	}
	variance := int64(math.Max(100, float64(expectedRows)*0.001))
	if actualRows >= expectedRows-variance && actualRows <= expectedRows+variance {
		return true, actualRows, nil
	}

	log.Printf("Row count validation failed for %s.%s: expected ~%d, got %d (variance: ±%d)",
		schema, table, expectedRows, actualRows, variance)

	return false, actualRows, nil
}

func (m *MigrationService) getDestinationRowCount(schema, table string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s`.`%s`", schema, table)

	var count int64
	if err := m.queryRowDest(query).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count destination rows: %w", err)
	}

	return count, nil
}
func (m *MigrationService) truncateTable(schema, table string) error {
	query := fmt.Sprintf("TRUNCATE TABLE `%s`.`%s`", schema, table)

	if _, err := m.execDest(query); err != nil {
		return fmt.Errorf("failed to truncate table: %w", err)
	}

	return nil
}

func (m *MigrationService) readBatch(table models.Table, offset int64, limit int) (*models.DataBatch, error) {
	columnNamesWithBackticks := make([]string, len(table.Columns))
	for i, col := range table.Columns {
		columnNamesWithBackticks[i] = fmt.Sprintf("`%s`", col.Name)
	}

	columnNames := make([]string, len(table.Columns))
	for i, col := range table.Columns {
		columnNames[i] = col.Name
	}

	var query string
	if len(table.PrimaryKey) > 0 && offset > 0 {
		primaryKeyCol := table.PrimaryKey[0] // Use first primary key column
		query = fmt.Sprintf(
			"SELECT %s FROM `%s`.`%s` WHERE `%s` > %d ORDER BY `%s` LIMIT %d",
			strings.Join(columnNamesWithBackticks, ", "),
			table.Schema,
			table.Name,
			primaryKeyCol,
			offset,
			primaryKeyCol,
			limit,
		)
	} else {
		query = fmt.Sprintf(
			"SELECT %s FROM `%s`.`%s` ORDER BY %s LIMIT %d OFFSET %d",
			strings.Join(columnNamesWithBackticks, ", "),
			table.Schema,
			table.Name,
			getOrderByClause(table),
			limit,
			offset,
		)
	}

	rows, err := m.querySource(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query source data: %w", err)
	}

	defer rows.Close()

	var data [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columnNames))
		scanArgs := make([]interface{}, len(values))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		data = append(data, values)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return &models.DataBatch{
		Schema:    table.Schema,
		Table:     table.Name,
		Columns:   columnNames,
		Rows:      data,
		BatchSize: limit,
	}, nil
}

func getOrderByClause(table models.Table) string {
	if len(table.PrimaryKey) > 0 {
		orderCols := make([]string, len(table.PrimaryKey))
		for i, col := range table.PrimaryKey {
			orderCols[i] = fmt.Sprintf("`%s`", col)
		}
		return strings.Join(orderCols, ", ")
	}

	if len(table.Columns) > 0 {
		return fmt.Sprintf("`%s`", table.Columns[0].Name)
	}

	return "1"
}

func (m *MigrationService) writeBatch(batch *models.DataBatch) error {
	if len(batch.Rows) == 0 {
		return nil
	}
	columnPlaceholders := make([]string, len(batch.Columns))
	for i := range batch.Columns {
		columnPlaceholders[i] = "?"
	}

	query := fmt.Sprintf(
		"REPLACE INTO `%s`.`%s` (`%s`) VALUES (%s)",
		batch.Schema,
		batch.Table,
		strings.Join(batch.Columns, "`, `"),
		strings.Join(columnPlaceholders, ", "),
	)

	stmt, err := m.destDB.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare replace statement: %w", err)
	}
	defer stmt.Close()

	for i, row := range batch.Rows {
		sanitizedRow := m.sanitizer.SanitizeRow(row, batch.Columns)

		if _, err := stmt.Exec(sanitizedRow...); err != nil {
			log.Printf("Error replacing row %d in %s.%s: %v", i+1, batch.Schema, batch.Table, err)
			log.Printf("   Row data: %v", sanitizedRow)
			return fmt.Errorf("failed to replace row %d in %s.%s: %w", i+1, batch.Schema, batch.Table, err)
		}
	}

	return nil
}

func (m *MigrationService) GetMigrationProgress(schemas []models.Schema) ([]models.TransferProgress, error) {
	var progress []models.TransferProgress

	for _, schema := range schemas {
		for _, table := range schema.Tables {
			sourceCount, err := m.getRowCount(table.Schema, table.Name)
			if err != nil {
				progress = append(progress, models.TransferProgress{
					Schema:        table.Schema,
					Table:         table.Name,
					TotalRows:     0,
					ProcessedRows: 0,
					IsCompleted:   false,
					Error:         err,
				})
				continue
			}

			destCount, err := m.getDestRowCount(table.Schema, table.Name)
			if err != nil {
				destCount = 0
			}

			progress = append(progress, models.TransferProgress{
				Schema:        table.Schema,
				Table:         table.Name,
				TotalRows:     sourceCount,
				ProcessedRows: destCount,
				IsCompleted:   sourceCount == destCount,
				Error:         nil,
			})
		}
	}

	return progress, nil
}

func (m *MigrationService) getDestRowCount(schema, table string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s`.`%s`", schema, table)

	var count int64
	if err := m.destDB.QueryRow(query).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count destination rows: %w", err)
	}

	return count, nil
}

func (m *MigrationService) printInitialDashboard(totalSchemas, totalTables int) {
	fmt.Printf("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Printf("MYSQL TWIN BACKUP - MIGRATION DASHBOARD\n")
	fmt.Printf(strings.Repeat("=", 80) + "\n")
	fmt.Printf("Total Schemas: %d\n", totalSchemas)
	fmt.Printf("Total Tables: %d\n", totalTables)
	fmt.Printf("Buffer Size: %d rows\n", m.bufferSize)
	fmt.Printf("Max Goroutines: %d\n", m.maxGoroutines)
	fmt.Printf(strings.Repeat("=", 80) + "\n\n")
}

func (m *MigrationService) printTableProgress(currentTable models.Table, nextTable string, currentIndex, totalTables int) {
	percentage := float64(currentIndex-1) / float64(totalTables) * 100

	progressBarWidth := 40 // Create progress bar
	filledWidth := int(percentage / 100 * float64(progressBarWidth))
	emptyWidth := progressBarWidth - filledWidth

	progressBar := fmt.Sprintf("[%s%s]",
		strings.Repeat("█", filledWidth),
		strings.Repeat("░", emptyWidth))

	fmt.Printf("\n" + strings.Repeat("-", 80) + "\n")
	fmt.Printf("PROGRESS: %s %.1f%% (%d/%d tables)\n", progressBar, percentage, currentIndex-1, totalTables)
	fmt.Printf("CURRENT: Processing %s.%s\n", currentTable.Schema, currentTable.Name)

	if nextTable != "" {
		fmt.Printf("  NEXT: %s\n", nextTable)
	} else {
		fmt.Printf("  NEXT: Final table!\n")
	}
	fmt.Printf(strings.Repeat("-", 80) + "\n")
}

func (m *MigrationService) printBatchProgress(schema, table string, percentage float64, processedRows, totalRows, currentOffset, batchSize int64) {

	progressBarWidth := 20 // Create mini progress bar for this table
	filledWidth := int(percentage / 100 * float64(progressBarWidth))
	emptyWidth := progressBarWidth - filledWidth

	progressBar := fmt.Sprintf("[%s%s]",
		strings.Repeat("▓", filledWidth),
		strings.Repeat("░", emptyWidth))

	log.Printf(" %s.%s: %s %.2f%% (%d/%d rows) | Batch: %d-%d (%d rows)",
		schema, table, progressBar, percentage, processedRows, totalRows,
		currentOffset+1, currentOffset+batchSize, batchSize)
}

func (m *MigrationService) getLastInsertedID(schema, tableName string) (int64, error) {
	pkColumn, err := m.getPrimaryKeyColumn(schema, tableName)
	if err != nil {
		return 0, fmt.Errorf("failed to get primary key column: %w", err)
	}

	query := fmt.Sprintf("SELECT COALESCE(MAX(`%s`), 0) FROM `%s`.`%s`", pkColumn, schema, tableName)

	var maxID int64
	err = m.destDB.QueryRow(query).Scan(&maxID)
	if err != nil {
		return 0, fmt.Errorf("failed to query max ID: %w", err)
	}

	return maxID, nil
}

func (m *MigrationService) getPrimaryKeyColumn(schema, tableName string) (string, error) {
	query := `SELECT COLUMN_NAME 
			  FROM INFORMATION_SCHEMA.COLUMNS 
			  WHERE TABLE_SCHEMA = ? 
			   	AND TABLE_NAME = ? 
				AND COLUMN_KEY = 'PRI'
				ORDER BY ORDINAL_POSITION 
				LIMIT 1`

	var columnName string
	err := m.destDB.QueryRow(query, schema, tableName).Scan(&columnName)
	if err != nil {
		if err == sql.ErrNoRows {
			return "id", nil // Default fallback
		}
		return "", fmt.Errorf("failed to get primary key column: %w", err)
	}

	return columnName, nil
}

func (m *MigrationService) MigrateTableDirect(schemaName, tableName string) error {
	log.Printf("DIRECT migration of table: %s.%s", schemaName, tableName)

	// Create a minimal table model for direct migration
	table := models.Table{
		Schema: schemaName,
		Name:   tableName,
	}

	if err := m.populateTableInfo(&table); err != nil {
		return fmt.Errorf("failed to get table info: %w", err)
	}

	return m.MigrateTable(table) // Migrate the table
}

func (m *MigrationService) MigrateSchemaData(schemaName string) error {
	log.Printf("DIRECT migration of schema: %s", schemaName)

	// Get list of tables in schema
	tables, err := m.getTablesInSchema(schemaName)
	if err != nil {
		return fmt.Errorf("failed to get tables in schema %s: %w", schemaName, err)
	}

	log.Printf("Found %d tables in schema %s - switching to PARALLEL processing", len(tables), schemaName)

	return m.migrateTablesParallel(tables)
}

func (m *MigrationService) populateTableInfo(table *models.Table) error {
	columnsQuery := `SELECT COLUMN_NAME, 
						DATA_TYPE, 
						IS_NULLABLE, 
						COLUMN_DEFAULT, 
						EXTRA
					FROM information_schema.COLUMNS
					HERE TABLE_SCHEMA = ? 
						AND TABLE_NAME = ?
					ORDER BY ORDINAL_POSITION`

	rows, err := m.sourceDB.Query(columnsQuery, table.Schema, table.Name)
	if err != nil {
		return fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	var columns []models.Column
	for rows.Next() {
		var col models.Column
		var nullable, extra string
		var defaultValue interface{}

		err := rows.Scan(&col.Name, &col.Type, &nullable, &defaultValue, &extra)
		if err != nil {
			return fmt.Errorf("failed to scan column: %w", err)
		}

		col.IsNullable = (nullable == "YES")
		if defaultValue != nil {
			defaultStr := fmt.Sprintf("%v", defaultValue)
			col.DefaultValue = &defaultStr
		}

		columns = append(columns, col)
	}

	table.Columns = columns

	// Get primary key
	pkQuery := `SELECT COLUMN_NAME
				FROM information_schema.KEY_COLUMN_USAGE
				WHERE TABLE_SCHEMA = ? 
					AND TABLE_NAME = ? 
					AND CONSTRAINT_NAME = 'PRIMARY'
				ORDER BY ORDINAL_POSITION`

	pkRows, err := m.sourceDB.Query(pkQuery, table.Schema, table.Name)
	if err != nil {
		return fmt.Errorf("failed to query primary key: %w", err)
	}
	defer pkRows.Close()

	var primaryKey []string
	for pkRows.Next() {
		var columnName string
		if err := pkRows.Scan(&columnName); err != nil {
			return fmt.Errorf("failed to scan primary key column: %w", err)
		}
		primaryKey = append(primaryKey, columnName)
	}

	table.PrimaryKey = primaryKey

	return nil
}

func (m *MigrationService) getTablesInSchema(schemaName string) ([]models.Table, error) {
	query := `SELECT TABLE_NAME
			  FROM information_schema.TABLES
			  WHERE TABLE_SCHEMA = ? 
			  	AND TABLE_TYPE = 'BASE TABLE'
			  ORDER BY TABLE_NAME`

	rows, err := m.sourceDB.Query(query, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []models.Table
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}

		table := models.Table{
			Schema: schemaName,
			Name:   tableName,
		}

		if err := m.populateTableInfo(&table); err != nil {
			return nil, fmt.Errorf("failed to populate table info for %s: %w", tableName, err)
		}

		tables = append(tables, table)
	}

	return tables, nil
}

func (m *MigrationService) MigrateSchemaDataParallel(schemaName string) error {
	log.Printf("🚀 PARALLEL migration of schema: %s", schemaName)

	// Get list of tables in schema
	tables, err := m.getTablesInSchema(schemaName)
	if err != nil {
		return fmt.Errorf("failed to get tables in schema %s: %w", schemaName, err)
	}

	if len(tables) == 0 {
		log.Printf("No tables found in schema %s", schemaName)
		return nil
	}

	log.Printf("Found %d tables in schema %s - starting parallel migration", len(tables), schemaName)

	return m.migrateTablesParallel(tables)
}

func (m *MigrationService) MigrateAllTablesParallel(schemas []models.Schema) error {
	log.Printf("Starting PARALLEL migration of all discovered tables")

	var allTables []models.Table
	for _, schema := range schemas {
		allTables = append(allTables, schema.Tables...)
	}

	if len(allTables) == 0 {
		log.Printf("No tables found to migrate")
		return nil
	}

	log.Printf("Found %d total tables across %d schemas - starting parallel migration", len(allTables), len(schemas))

	return m.migrateTablesParallel(allTables)
}

func (m *MigrationService) migrateTablesParallel(tables []models.Table) error {
	tableCount := len(tables)
	maxWorkers := m.maxGoroutines / 10 // Use 10% of available goroutines for table-level parallelism
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	if maxWorkers > tableCount {
		maxWorkers = tableCount
	}

	log.Printf(" Using %d parallel workers for %d tables", maxWorkers, tableCount)

	// Create channels for job distribution and result collection
	jobs := make(chan TableJob, tableCount)
	results := make(chan TableResult, tableCount)

	// Start worker goroutines
	for w := 1; w <= maxWorkers; w++ {
		go m.tableWorker(w, jobs, results)
	}

	// Send jobs
	go func() {
		defer close(jobs)
		for i, table := range tables {
			jobs <- TableJob{
				Table: table,
				Index: i + 1,
				Total: tableCount,
			}
		}
	}()

	var errors []error
	completed := 0

	for completed < tableCount {
		result := <-results
		completed++

		if result.Error != nil {
			log.Printf(" Table %d/%d FAILED: %s.%s - %v",
				result.Index, tableCount, result.Table.Schema, result.Table.Name, result.Error)
			errors = append(errors, fmt.Errorf("table %s.%s: %w",
				result.Table.Schema, result.Table.Name, result.Error))
		} else {
			log.Printf(" Table %d/%d COMPLETED: %s.%s",
				result.Index, tableCount, result.Table.Schema, result.Table.Name)
		}

		percentage := float64(completed) / float64(tableCount) * 100
		log.Printf(" OVERALL PROGRESS: %.1f%% (%d/%d tables completed)",
			percentage, completed, tableCount)
	}

	close(results)

	if len(errors) > 0 {
		log.Printf("  Migration completed with %d errors out of %d tables", len(errors), tableCount)
		for _, err := range errors {
			log.Printf("Error: %v", err)
		}
		return errors[0]
	}

	log.Printf(" All %d tables migrated successfully in parallel!", tableCount)
	return nil
}

// tableWorker processes table migration jobs
func (m *MigrationService) tableWorker(workerID int, jobs <-chan TableJob, results chan<- TableResult) {
	log.Printf(" Worker %d started", workerID)

	for job := range jobs {
		log.Printf(" Worker %d: Starting table %d/%d: %s.%s",
			workerID, job.Index, job.Total, job.Table.Schema, job.Table.Name)

		start := time.Now()
		err := m.MigrateTable(job.Table)
		duration := time.Since(start)

		if err != nil {
			log.Printf(" Worker %d: FAILED table %s.%s in %v - %v",
				workerID, job.Table.Schema, job.Table.Name, duration, err)
		} else {
			log.Printf(" Worker %d: COMPLETED table %s.%s in %v",
				workerID, job.Table.Schema, job.Table.Name, duration)
		}

		results <- TableResult{
			Table: job.Table,
			Error: err,
			Index: job.Index,
		}
	}

	log.Printf(" Worker %d finished", workerID)
}

func (m *MigrationService) recountAndUpdateCheckpoint(schema, table string) error {
	currentCount, err := m.getRowCount(schema, table)
	if err != nil {
		return fmt.Errorf("failed to recount table: %w", err)
	}

	key := fmt.Sprintf("%s.%s", schema, table)

	var checkpointCount int64
	if checkpoint, exists := m.checkpoint.state.Checkpoints[key]; exists {
		checkpointCount = checkpoint.TotalRows
	} else {
		log.Printf(" No checkpoint found for %s.%s", schema, table)
		return nil
	}

	if checkpointCount != currentCount {
		log.Printf(" Row count changed for %s.%s: checkpoint has %d, actual has %d",
			schema, table, checkpointCount, currentCount)

		m.checkpoint.mutex.Lock()
		if checkpoint, exists := m.checkpoint.state.Checkpoints[key]; exists {
			checkpoint.TotalRows = currentCount
			m.checkpoint.state.Checkpoints[key] = checkpoint
		}
		m.checkpoint.mutex.Unlock()
		m.checkpoint.SaveState()

		log.Printf(" Updated checkpoint for %s.%s with new count: %d", schema, table, currentCount)
	}

	return nil
}

func (m *MigrationService) preflightTableCount(schema, table string) error {
	log.Printf(" Performing preflight count check for %s.%s", schema, table)

	if err := m.recountAndUpdateCheckpoint(schema, table); err != nil {
		return fmt.Errorf("preflight recount failed: %w", err)
	}

	return nil
}
