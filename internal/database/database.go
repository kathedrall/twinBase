package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/kathedrall/mysql-twin-backup/internal/config"
)

type Manager struct {
	SourceDB *sql.DB
	DestDB   *sql.DB
	config   *config.Config
}

func NewManager(cfg *config.Config) (*Manager, error) {
	manager := &Manager{
		config: cfg,
	}

	sourceDB, err := sql.Open("mysql", cfg.GetSourceDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to source database: %w", err)
	}

	if err := sourceDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping source database: %w", err)
	}

	sourceDB.SetMaxOpenConns(20)                 // Max 20 concurrent connections
	sourceDB.SetMaxIdleConns(5)                  // Keep 5 idle connections
	sourceDB.SetConnMaxLifetime(5 * time.Minute) // Recycle connections every 5 minutes

	manager.SourceDB = sourceDB
	log.Printf("Connected to source database: %s:%s (max_conns: 20)", cfg.SourceDB.Host, cfg.SourceDB.Port)

	destDB, err := sql.Open("mysql", cfg.GetDestDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to destination database: %w", err)
	}

	if err := destDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping destination database: %w", err)
	}

	destDB.SetMaxOpenConns(50)                  // Max 50 concurrent connections locally
	destDB.SetMaxIdleConns(10)                  // Keep 10 idle connections
	destDB.SetConnMaxLifetime(10 * time.Minute) // Recycle connections every 10 minutes

	manager.DestDB = destDB
	log.Printf("Connected to destination database: %s:%s (max_conns: 50)", cfg.DestDB.Host, cfg.DestDB.Port)

	return manager, nil
}

func (m *Manager) Close() error {
	var errs []error

	if m.SourceDB != nil {
		if err := m.SourceDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close source database: %w", err))
		}
	}

	if m.DestDB != nil {
		if err := m.DestDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close destination database: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing databases: %v", errs)
	}

	return nil
}
func (m *Manager) GetSourceDB() *sql.DB {
	return m.SourceDB
}

func (m *Manager) GetDestDB() *sql.DB {
	return m.DestDB
}

func (m *Manager) SetConnectionLimits(maxOpen, maxIdle int) {
	if m.SourceDB != nil {
		m.SourceDB.SetMaxOpenConns(maxOpen)
		m.SourceDB.SetMaxIdleConns(maxIdle)
	}
	if m.DestDB != nil {
		m.DestDB.SetMaxOpenConns(maxOpen)
		m.DestDB.SetMaxIdleConns(maxIdle)
	}
}

func (m *Manager) QuerySourceWithLog(query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	rows, err := m.SourceDB.Query(query, args...)
	duration := time.Since(start)
	if err != nil {
		log.Printf("[TIMEOUT/ERROR] MYSQL_SOURCE_HOST (%s:%s) query failed after %v: %v",
			m.config.SourceDB.Host, m.config.SourceDB.Port, duration, err)
		return nil, err
	}
	if duration > 5*time.Second {
		log.Printf("[SLOW] MYSQL_SOURCE_HOST (%s:%s) query took %v",
			m.config.SourceDB.Host, m.config.SourceDB.Port, duration)
	}
	return rows, nil
}

func (m *Manager) QueryRowSourceWithLog(query string, args ...interface{}) *sql.Row {
	start := time.Now()
	row := m.SourceDB.QueryRow(query, args...)
	duration := time.Since(start)
	if duration > 5*time.Second {
		log.Printf("[SLOW] MYSQL_SOURCE_HOST (%s:%s) queryrow took %v",
			m.config.SourceDB.Host, m.config.SourceDB.Port, duration)
	}
	return row
}

func (m *Manager) QueryDestWithLog(query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	rows, err := m.DestDB.Query(query, args...)
	duration := time.Since(start)
	if err != nil {
		log.Printf("[TIMEOUT/ERROR] MYSQL_DEST_HOST (%s:%s) query failed after %v: %v",
			m.config.DestDB.Host, m.config.DestDB.Port, duration, err)
		return nil, err
	}
	if duration > 5*time.Second {
		log.Printf("[SLOW] MYSQL_DEST_HOST (%s:%s) query took %v",
			m.config.DestDB.Host, m.config.DestDB.Port, duration)
	}
	return rows, nil
}

func (m *Manager) QueryRowDestWithLog(query string, args ...interface{}) *sql.Row {
	start := time.Now()
	row := m.DestDB.QueryRow(query, args...)
	duration := time.Since(start)
	if duration > 5*time.Second {
		log.Printf("[SLOW] MYSQL_DEST_HOST (%s:%s) queryrow took %v",
			m.config.DestDB.Host, m.config.DestDB.Port, duration)
	}
	return row
}

func (m *Manager) ExecDestWithLog(query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := m.DestDB.Exec(query, args...)
	duration := time.Since(start)
	if err != nil {
		log.Printf("[TIMEOUT/ERROR] MYSQL_DEST_HOST (%s:%s) exec failed after %v: %v",
			m.config.DestDB.Host, m.config.DestDB.Port, duration, err)
		return nil, err
	}
	if duration > 5*time.Second {
		log.Printf("[SLOW] MYSQL_DEST_HOST (%s:%s) exec took %v",
			m.config.DestDB.Host, m.config.DestDB.Port, duration)
	}
	return result, nil
}
