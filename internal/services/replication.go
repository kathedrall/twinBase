package services

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/kathedrall/twinBase/internal/models"
)

type ReplicationService struct {
	destDB *sql.DB
}

func NewReplicationService(destDB *sql.DB) *ReplicationService {
	return &ReplicationService{
		destDB: destDB,
	}
}

func (s *ReplicationService) ReplicateSchemas(schemas []models.Schema) error {
	for _, schema := range schemas {
		log.Printf("Creating schema: %s", schema.Name)

		if err := s.CreateSchema(schema.Name); err != nil {
			return fmt.Errorf("failed to create schema %s: %w", schema.Name, err)
		}

		for _, table := range schema.Tables {
			log.Printf("Creating table: %s.%s", schema.Name, table.Name)

			if err := s.CreateTable(table); err != nil {
				return fmt.Errorf("failed to create table %s.%s: %w", schema.Name, table.Name, err)
			}

			if err := s.CreateIndexes(table); err != nil {
				log.Printf("Warning: failed to create indexes for table %s.%s: %v", schema.Name, table.Name, err)
			}
		}
	}

	return nil
}

func (s *ReplicationService) CreateSchema(schemaName string) error {
	query := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS `%s`", schemaName)

	if _, err := s.destDB.Exec(query); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

func (s *ReplicationService) CreateTable(table models.Table) error {
	query := s.buildCreateTableQuery(table)

	if _, err := s.destDB.Exec(query); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

func (s *ReplicationService) CreateIndexes(table models.Table) error {
	for _, index := range table.Indexes {
		// Skip primary key as it's already created with the table
		if index.IsPrimary {
			continue
		}

		// Check if index already exists
		exists, err := s.indexExists(table.Schema, table.Name, index.Name)
		if err != nil {
			log.Printf("Warning: Failed to check if index %s exists: %v", index.Name, err)
		}

		if exists {
			log.Printf("Index %s already exists on %s.%s, skipping", index.Name, table.Schema, table.Name)
			continue
		}

		query := s.buildCreateIndexQuery(table.Schema, table.Name, index)

		if _, err := s.destDB.Exec(query); err != nil {
			// Log warning but don't fail - continue with other indexes
			log.Printf("Warning: Failed to create index %s: %v", index.Name, err)
			continue
		}

		log.Printf("Created index: %s on %s.%s", index.Name, table.Schema, table.Name)
	}

	return nil
}

func (s *ReplicationService) buildCreateTableQuery(table models.Table) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s`.`%s` (", table.Schema, table.Name))

	// Add columns
	var columnDefs []string
	for _, column := range table.Columns {
		columnDef := s.buildColumnDefinition(column)
		columnDefs = append(columnDefs, columnDef)
	}

	// Add primary key
	if len(table.PrimaryKey) > 0 {
		pkColumns := make([]string, len(table.PrimaryKey))
		for i, col := range table.PrimaryKey {
			pkColumns[i] = fmt.Sprintf("`%s`", col)
		}
		primaryKeyDef := fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkColumns, ", "))
		columnDefs = append(columnDefs, primaryKeyDef)
	}

	parts = append(parts, strings.Join(columnDefs, ",\n  "))
	parts = append(parts, ") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci")

	return strings.Join(parts, "\n  ")
}

func (s *ReplicationService) buildColumnDefinition(column models.Column) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("`%s`", column.Name))
	parts = append(parts, column.Type)

	if column.CharacterSet != nil && *column.CharacterSet != "" {
		parts = append(parts, fmt.Sprintf("CHARACTER SET %s", *column.CharacterSet))
	}

	if column.Collation != nil && *column.Collation != "" {
		parts = append(parts, fmt.Sprintf("COLLATE %s", *column.Collation))
	}

	if !column.IsNullable {
		parts = append(parts, "NOT NULL")
	}

	if column.IsAutoIncrement {
		parts = append(parts, "AUTO_INCREMENT")
	}

	if column.DefaultValue != nil {
		if *column.DefaultValue == "CURRENT_TIMESTAMP" {
			parts = append(parts, "DEFAULT CURRENT_TIMESTAMP")
		} else if *column.DefaultValue == "NULL" {
			parts = append(parts, "DEFAULT NULL")
		} else {
			parts = append(parts, fmt.Sprintf("DEFAULT '%s'", *column.DefaultValue))
		}
	}

	return strings.Join(parts, " ")
}

func (s *ReplicationService) buildCreateIndexQuery(schema, table string, index models.Index) string {
	indexType := "INDEX"
	if index.IsUnique {
		indexType = "UNIQUE INDEX"
	}

	columns := make([]string, len(index.Columns))
	for i, col := range index.Columns {
		columns[i] = fmt.Sprintf("`%s`", col)
	}

	return fmt.Sprintf(
		"CREATE %s `%s` ON `%s`.`%s` (%s)",
		indexType,
		index.Name,
		schema,
		table,
		strings.Join(columns, ", "),
	)
}

func (s *ReplicationService) indexExists(schema, table, indexName string) (bool, error) {
	query := `SELECT COUNT(*) 
			  FROM information_schema.statistics 
			  WHERE table_schema = ? 
				AND table_name = ? 
				AND index_name = ?`

	var count int
	err := s.destDB.QueryRow(query, schema, table, indexName).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (s *ReplicationService) DropSchema(schemaName string) error {
	query := fmt.Sprintf("DROP SCHEMA IF EXISTS `%s`", schemaName)

	if _, err := s.destDB.Exec(query); err != nil {
		return fmt.Errorf("failed to drop schema: %w", err)
	}

	return nil
}

func (s *ReplicationService) TableExists(schema, table string) (bool, error) {
	query := `SELECT COUNT(*) 
			  FROM information_schema.TABLES 
			  WHERE TABLE_SCHEMA = ? 
			  	AND TABLE_NAME = ?`

	var count int
	err := s.destDB.QueryRow(query, schema, table).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check table existence: %w", err)
	}

	return count > 0, nil
}

func (s *ReplicationService) ReplicateTableDirect(schemaName, tableName string) error {
	log.Printf("  DIRECT replication of table: %s.%s", schemaName, tableName)
	return fmt.Errorf("DIRECT replication not fully implemented yet. Use: ./twinbase replicate (without --no-discovery)")
}

func (s *ReplicationService) ReplicateSchemaDirect(schemaName string) error {
	log.Printf("  DIRECT replication of schema: %s", schemaName)
	return fmt.Errorf("DIRECT replication not fully implemented yet. Use: ./twinbase replicate (without --no-discovery)")
}
