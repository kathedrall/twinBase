package services

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/kathedrall/mysql-twin-backup/internal/models"
)

type DiscoveryService struct {
	db *sql.DB
}

func NewDiscoveryService(db *sql.DB) *DiscoveryService {
	return &DiscoveryService{
		db: db,
	}
}

func (s *DiscoveryService) DiscoverSchemas() ([]models.Schema, error) {
	query := `SELECT SCHEMA_NAME 
				FROM information_schema.SCHEMATA 
				WHERE SCHEMA_NAME NOT IN ('information_schema', 'performance_schema', 'mysql', 'sys')`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query schemas: %w", err)
	}
	defer rows.Close()

	var schemas []models.Schema
	for rows.Next() {
		var schemaName string
		if err := rows.Scan(&schemaName); err != nil {
			return nil, fmt.Errorf("failed to scan schema name: %w", err)
		}

		schema := models.Schema{
			Name: schemaName,
		}

		tables, err := s.DiscoverTables(schemaName)
		if err != nil {
			log.Printf("Warning: failed to discover tables for schema %s: %v", schemaName, err)
			continue
		}

		schema.Tables = tables
		schemas = append(schemas, schema)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating schemas: %w", err)
	}

	log.Printf("Discovered %d schemas", len(schemas))
	return schemas, nil
}

func (s *DiscoveryService) DiscoverTables(schemaName string) ([]models.Table, error) {
	query := `SELECT TABLE_NAME 
				FROM information_schema.TABLES 
				WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'`

	rows, err := s.db.Query(query, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables for schema %s: %w", schemaName, err)
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

		columns, err := s.DiscoverColumns(schemaName, tableName)
		if err != nil {
			log.Printf("Warning: failed to discover columns for table %s.%s: %v", schemaName, tableName, err)
			continue
		}
		table.Columns = columns

		indexes, err := s.DiscoverIndexes(schemaName, tableName)
		if err != nil {
			log.Printf("Warning: failed to discover indexes for table %s.%s: %v", schemaName, tableName, err)
		} else {
			table.Indexes = indexes
		}

		primaryKey, err := s.DiscoverPrimaryKey(schemaName, tableName)
		if err != nil {
			log.Printf("Warning: failed to discover primary key for table %s.%s: %v", schemaName, tableName, err)
		} else {
			table.PrimaryKey = primaryKey
		}

		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tables: %w", err)
	}

	log.Printf("Discovered %d tables in schema %s", len(tables), schemaName)
	return tables, nil
}

func (s *DiscoveryService) DiscoverColumns(schemaName, tableName string) ([]models.Column, error) {
	query := `SELECT COLUMN_NAME,
					COLUMN_TYPE,
					IS_NULLABLE,
					COLUMN_DEFAULT,
					EXTRA,
					CHARACTER_SET_NAME,
					COLLATION_NAME
			   	FROM information_schema.COLUMNS 
				WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
				ORDER BY ORDINAL_POSITION`

	rows, err := s.db.Query(query, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	var columns []models.Column
	for rows.Next() {
		var column models.Column
		var isNullable string
		var extra sql.NullString

		err := rows.Scan(
			&column.Name,
			&column.Type,
			&isNullable,
			&column.DefaultValue,
			&extra,
			&column.CharacterSet,
			&column.Collation,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}

		column.IsNullable = isNullable == "YES"
		column.IsAutoIncrement = extra.Valid && extra.String == "auto_increment"

		columns = append(columns, column)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating columns: %w", err)
	}

	return columns, nil
}

func (s *DiscoveryService) DiscoverIndexes(schemaName, tableName string) ([]models.Index, error) {
	query := `SELECT INDEX_NAME,
					COLUMN_NAME,
					NON_UNIQUE
				FROM information_schema.STATISTICS 
				WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
				ORDER BY INDEX_NAME, SEQ_IN_INDEX`

	rows, err := s.db.Query(query, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer rows.Close()

	indexMap := make(map[string]*models.Index)
	for rows.Next() {
		var indexName, columnName string
		var nonUnique int

		if err := rows.Scan(&indexName, &columnName, &nonUnique); err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}

		if index, exists := indexMap[indexName]; exists {
			index.Columns = append(index.Columns, columnName)
		} else {
			indexMap[indexName] = &models.Index{
				Name:      indexName,
				Columns:   []string{columnName},
				IsUnique:  nonUnique == 0,
				IsPrimary: indexName == "PRIMARY",
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating indexes: %w", err)
	}

	var indexes []models.Index
	for _, index := range indexMap {
		indexes = append(indexes, *index)
	}

	return indexes, nil
}

func (s *DiscoveryService) DiscoverPrimaryKey(schemaName, tableName string) ([]string, error) {
	query := `SELECT COLUMN_NAME 
			  FROM information_schema.KEY_COLUMN_USAGE 
			  WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND CONSTRAINT_NAME = 'PRIMARY'
			  ORDER BY ORDINAL_POSITION`

	rows, err := s.db.Query(query, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query primary key: %w", err)
	}
	defer rows.Close()

	var primaryKey []string
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, fmt.Errorf("failed to scan primary key column: %w", err)
		}
		primaryKey = append(primaryKey, columnName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating primary key columns: %w", err)
	}

	return primaryKey, nil
}
