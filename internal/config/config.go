package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	SourceDB      DatabaseConfig
	DestDB        DatabaseConfig
	BufferSize    int
	MaxGoroutines int
	LogLevel      string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	config := &Config{
		SourceDB: DatabaseConfig{
			Host:     getEnv("MYSQL_SOURCE_HOST", "localhost"),
			Port:     getEnv("MYSQL_SOURCE_PORT", "3306"),
			User:     getEnv("MYSQL_SOURCE_USER", "root"),
			Password: getEnv("MYSQL_SOURCE_PASSWORD", ""),
			Database: getEnv("MYSQL_SOURCE_DATABASE", ""),
		},
		DestDB: DatabaseConfig{
			Host:     getEnv("MYSQL_DEST_HOST", "localhost"),
			Port:     getEnv("MYSQL_DEST_PORT", "3306"),
			User:     getEnv("MYSQL_DEST_USER", "root"),
			Password: getEnv("MYSQL_DEST_PASSWORD", ""),
			Database: getEnv("MYSQL_DEST_DATABASE", ""),
		},
		BufferSize:    getEnvAsInt("BUFFER_SIZE", 100000),
		MaxGoroutines: getEnvAsInt("MAX_GOROUTINES", 10),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

func (c *Config) Validate() error {
	if c.SourceDB.Host == "" {
		return fmt.Errorf("source database host is required")
	}
	if c.DestDB.Host == "" {
		return fmt.Errorf("destination database host is required")
	}
	if c.BufferSize <= 0 {
		return fmt.Errorf("buffer size must be greater than 0")
	}
	if c.MaxGoroutines <= 0 {
		return fmt.Errorf("max goroutines must be greater than 0")
	}
	return nil
}

func (c *Config) GetSourceDSN() string {
	return c.SourceDB.GetDSN()
}

func (c *Config) GetDestDSN() string {
	return c.DestDB.GetDSN()
}

func (db *DatabaseConfig) GetDSN() string {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/", db.User, db.Password, db.Host, db.Port)
	if db.Database != "" {
		dsn += db.Database
	}
	dsn += "?charset=utf8mb4&parseTime=True&loc=Local"
	dsn += "&timeout=30s&readTimeout=60s&writeTimeout=60s"
	dsn += "&maxAllowedPacket=67108864"
	dsn += "&collation=utf8mb4_unicode_ci"
	return dsn
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}
