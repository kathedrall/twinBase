package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/kathedrall/mysql-twin-backup/internal/config"
	"github.com/kathedrall/mysql-twin-backup/internal/database"
	"github.com/kathedrall/mysql-twin-backup/internal/models"
	"github.com/kathedrall/mysql-twin-backup/internal/services"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("MySQL Twin Backup - Database Migration Tool")
	fmt.Println("============================================")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	fmt.Printf("Source DB: %s:%s\n", cfg.SourceDB.Host, cfg.SourceDB.Port)
	fmt.Printf("Destination DB: %s:%s\n", cfg.DestDB.Host, cfg.DestDB.Port)
	fmt.Printf("Buffer Size: %d\n", cfg.BufferSize)
	fmt.Printf("Max Goroutines: %d\n", cfg.MaxGoroutines)
	fmt.Println()

	dbManager, err := database.NewManager(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database connections: %v", err)
	}
	defer func() {
		if err := dbManager.Close(); err != nil {
			log.Printf("Error closing database connections: %v", err)
		}
	}()

	dbManager.SetConnectionLimits(cfg.MaxGoroutines*2, cfg.MaxGoroutines)
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	command := os.Args[1]
	noDiscovery := false
	target := ""

	fmt.Printf("DEBUG - All arguments: %v\n", os.Args)
	fmt.Printf("DEBUG - Arguments after command: %v\n", os.Args[2:])

	for _, arg := range os.Args[2:] {
		fmt.Printf("DEBUG - Processing arg: '%s'\n", arg)
		if arg == "--no-discovery" {
			noDiscovery = true
			fmt.Printf("DEBUG - Set noDiscovery = true\n")
		} else if target == "" && !strings.HasPrefix(arg, "--") {
			target = arg
			fmt.Printf("DEBUG - Set target = '%s'\n", target)
		}
	}
	fmt.Printf("DEBUG - Final values: noDiscovery=%v, target='%s'\n", noDiscovery, target)

	switch command {
	case "discover":
		runDiscovery(dbManager)
	case "replicate":
		if noDiscovery && target != "" {
			runDirectReplication(dbManager, cfg, target)
		} else {
			runReplication(dbManager, cfg)
		}
	case "migrate":
		if target != "" {
			if noDiscovery {
				runDirectMigration(dbManager, cfg, target)
			} else {
				runTargetMigration(dbManager, cfg, target)
			}
		} else {
			runMigration(dbManager, cfg)
		}
	case "users":
		runUserMigration(dbManager, cfg)
	case "full":
		if noDiscovery && target != "" {
			runDirectFullMigration(dbManager, cfg, target)
		} else {
			runFullMigration(dbManager, cfg)
		}
	case "progress":
		runProgressCheck(dbManager, cfg)
	case "help", "--help":
		printUsage()
		os.Exit(0)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: mysql-twin-backup <command> [options] [target]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  discover              - Discover schemas and tables from source database")
	fmt.Println("  replicate [target]    - Create schema and table structure in destination")
	fmt.Println("  migrate [target]      - Migrate data from source to destination")
	fmt.Println("  users                 - Migrate database users, passwords and privileges")
	fmt.Println("  full [target]         - Run complete migration (discover + replicate + migrate)")
	fmt.Println("  progress              - Check migration progress")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --no-discovery        - Skip discovery phase (faster for specific targets)")
	fmt.Println()
	fmt.Println("Target formats:")
	fmt.Println("  schema_name           - Process entire schema")
	fmt.Println("  schema.table          - Process specific table")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Fast migration of specific table (RECOMMENDED for large tables)")
	fmt.Println("  ./twinbase migrate --no-discovery platform_event.trace")
	fmt.Println()
	fmt.Println("  # Fast migration of entire schema")
	fmt.Println("  ./twinbase migrate --no-discovery platform_event")
	fmt.Println()
	fmt.Println("  # Traditional full discovery (slower)")
	fmt.Println("  ./twinbase full")
	fmt.Println()
	fmt.Println("Environment variables (.env file):")
	fmt.Println("  MYSQL_SOURCE_HOST, MYSQL_SOURCE_PORT, MYSQL_SOURCE_USER")
	fmt.Println("  MYSQL_SOURCE_PASSWORD, MYSQL_SOURCE_DATABASE")
	fmt.Println("  MYSQL_DEST_HOST, MYSQL_DEST_PORT, MYSQL_DEST_USER")
	fmt.Println("  MYSQL_DEST_PASSWORD, MYSQL_DEST_DATABASE")
	fmt.Println("  BUFFER_SIZE (default: 100000)")
	fmt.Println("  MAX_GOROUTINES (default: 10)")
}

func runDiscovery(dbManager *database.Manager) {
	fmt.Println("Starting schema discovery...")

	discoveryService := services.NewDiscoveryService(dbManager.GetSourceDB())

	schemas, err := discoveryService.DiscoverSchemas()
	if err != nil {
		log.Fatalf("Discovery failed: %v", err)
	}

	fmt.Printf("\nDiscovery completed! Found %d schemas:\n", len(schemas))
	for _, schema := range schemas {
		fmt.Printf("Schema: %s (%d tables)\n", schema.Name, len(schema.Tables))
		for _, table := range schema.Tables {
			fmt.Printf("Table: %s (%d columns)\n", table.Name, len(table.Columns))
		}
	}
}

func runReplication(dbManager *database.Manager, cfg *config.Config) {
	fmt.Println("Starting schema replication...")
	discoveryService := services.NewDiscoveryService(dbManager.GetSourceDB())
	schemas, err := discoveryService.DiscoverSchemas()
	if err != nil {
		log.Fatalf("Discovery failed: %v", err)
	}

	replicationService := services.NewReplicationService(dbManager.GetDestDB())
	if err := replicationService.ReplicateSchemas(schemas); err != nil {
		log.Fatalf("Replication failed: %v", err)
	}
	fmt.Printf("\nSchema replication completed! Created %d schemas in destination.\n", len(schemas))
}

func runMigration(dbManager *database.Manager, cfg *config.Config) {
	fmt.Println("Starting data migration...")
	discoveryService := services.NewDiscoveryService(dbManager.GetSourceDB())
	schemas, err := discoveryService.DiscoverSchemas()
	if err != nil {
		log.Fatalf("Discovery failed: %v", err)
	}

	migrationService := services.NewMigrationService(
		dbManager.GetSourceDB(),
		dbManager.GetDestDB(),
		cfg.BufferSize,
		cfg.MaxGoroutines,
	)
	migrationService.SetDatabaseManager(dbManager)

	startTime := time.Now()

	if err := migrationService.MigrateData(schemas); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("\nData migration completed in %v!\n", duration)
}

func runFullMigration(dbManager *database.Manager, cfg *config.Config) {
	fmt.Println("Starting full migration (discover + replicate + migrate)...")

	startTime := time.Now()
	fmt.Println("\nStep 1: Discovering source database structure...")
	discoveryService := services.NewDiscoveryService(dbManager.GetSourceDB())
	schemas, err := discoveryService.DiscoverSchemas()
	if err != nil {
		log.Fatalf("Discovery failed: %v", err)
	}

	fmt.Printf("Discovered %d schemas\n", len(schemas))
	fmt.Println("\nStep 2: Replicating structure to destination...")
	replicationService := services.NewReplicationService(dbManager.GetDestDB())

	if err := replicationService.ReplicateSchemas(schemas); err != nil {
		log.Fatalf("Replication failed: %v", err)
	}

	fmt.Println("Structure replication completed")
	fmt.Println("\nStep 3: Migrating data...")
	migrationService := services.NewMigrationService(
		dbManager.GetSourceDB(),
		dbManager.GetDestDB(),
		cfg.BufferSize,
		cfg.MaxGoroutines,
	)
	migrationService.SetDatabaseManager(dbManager)

	if err := migrationService.MigrateData(schemas); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("\nFull migration completed successfully in %v!\n", duration)
	showFinalStats(migrationService, schemas)
}

func runProgressCheck(dbManager *database.Manager, cfg *config.Config) {
	fmt.Println("Checking migration progress...")
	discoveryService := services.NewDiscoveryService(dbManager.GetSourceDB())
	schemas, err := discoveryService.DiscoverSchemas()
	if err != nil {
		log.Fatalf("Discovery failed: %v", err)
	}

	migrationService := services.NewMigrationService(
		dbManager.GetSourceDB(),
		dbManager.GetDestDB(),
		cfg.BufferSize,
		cfg.MaxGoroutines,
	)
	migrationService.SetDatabaseManager(dbManager)

	progress, err := migrationService.GetMigrationProgress(schemas)
	if err != nil {
		log.Fatalf("Failed to get progress: %v", err)
	}

	fmt.Printf("\nMigration Progress Report:\n")
	fmt.Println("=============================")

	var totalRows, totalProcessed int64
	var completedTables, totalTables int

	for _, p := range progress {
		status := "In Progress"
		if p.IsCompleted {
			status = "Completed"
			completedTables++
		} else if p.Error != nil {
			status = "Error"
		}

		percentage := float64(0)
		if p.TotalRows > 0 {
			percentage = float64(p.ProcessedRows) / float64(p.TotalRows) * 100
		}
		fmt.Printf("%s %s.%s: %d/%d rows (%.1f%%) %s\n",
			status, p.Schema, p.Table, p.ProcessedRows, p.TotalRows, percentage, "")

		if p.Error != nil {
			fmt.Printf("    Error: %v\n", p.Error)
		}
		totalRows += p.TotalRows
		totalProcessed += p.ProcessedRows
		totalTables++
	}

	fmt.Println("=============================")
	overallPercentage := float64(0)
	if totalRows > 0 {
		overallPercentage = float64(totalProcessed) / float64(totalRows) * 100
	}

	fmt.Printf("Overall Progress: %d/%d tables completed (%.1f%%)\n",
		completedTables, totalTables, float64(completedTables)/float64(totalTables)*100)
	fmt.Printf("Data Progress: %d/%d rows migrated (%.1f%%)\n",
		totalProcessed, totalRows, overallPercentage)
}

func showFinalStats(migrationService *services.MigrationService, schemas []models.Schema) {
	fmt.Println("\nFinal Migration Statistics:")
	fmt.Println("==============================")

	var totalTables, totalColumns int
	for _, schema := range schemas {
		totalTables += len(schema.Tables)
		for _, table := range schema.Tables {
			totalColumns += len(table.Columns)
		}
	}
	fmt.Printf("Schemas migrated: %d\n", len(schemas))
	fmt.Printf("Tables migrated: %d\n", totalTables)
	fmt.Printf("Columns migrated: %d\n", totalColumns)
	fmt.Println("==============================")
}

func runUserMigration(dbManager *database.Manager, cfg *config.Config) {
	fmt.Println("Starting user and privilege migration...")
	userMigrationService := services.NewUserMigrationService(
		dbManager.GetSourceDB(),
		dbManager.GetDestDB(),
	)

	startTime := time.Now()
	summary, err := userMigrationService.MigrateUsers()
	if err != nil {
		log.Fatalf("User migration failed: %v", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("\nUser migration completed in %v!\n", duration)

	if len(summary.Errors) > 0 {
		fmt.Printf("Migration completed with %d errors. Check the logs above for details.\n", len(summary.Errors))
	}
}

func runTargetMigration(dbManager *database.Manager, cfg *config.Config, target string) {
	fmt.Printf("Starting migration of target: %s (WITH DISCOVERY)\n", target)

	startTime := time.Now()
	discoveryService := services.NewDiscoveryService(dbManager.GetSourceDB())
	migrationService := services.NewMigrationService(
		dbManager.GetSourceDB(),
		dbManager.GetDestDB(),
		cfg.BufferSize,
		cfg.MaxGoroutines,
	)
	migrationService.SetDatabaseManager(dbManager)

	if strings.Contains(target, ".") {
		parts := strings.Split(target, ".")
		if len(parts) != 2 {
			log.Fatalf("Invalid table format. Use: schema.table")
		}
		schema := parts[0]
		table := parts[1]
		fmt.Printf("Discovering table: %s.%s\n", schema, table)

		tables, err := discoveryService.DiscoverTables(schema)
		if err != nil {
			log.Fatalf("Failed to discover tables in schema %s: %v", schema, err)
		}
		var targetTable *models.Table
		for _, t := range tables {
			if t.Name == table {
				targetTable = &t
				break
			}
		}

		if targetTable == nil {
			log.Fatalf("Table '%s.%s' not found", schema, table)
		}
		fmt.Printf("Migrating TABLE: %s.%s\n", schema, table)

		if err := migrationService.MigrateTable(*targetTable); err != nil {
			log.Fatalf("Table migration failed: %v", err)
		}

	} else {
		fmt.Printf("Discovering schema: %s\n", target)
		tables, err := discoveryService.DiscoverTables(target)
		if err != nil {
			log.Fatalf("Failed to discover tables in schema %s: %v", target, err)
		}

		if len(tables) == 0 {
			log.Fatalf("No tables found in schema '%s'", target)
		}

		targetSchema := models.Schema{
			Name:   target,
			Tables: tables,
		}
		fmt.Printf("Migrating SCHEMA: %s (%d tables)\n", target, len(tables))

		if err := migrationService.MigrateData([]models.Schema{targetSchema}); err != nil {
			log.Fatalf("Schema migration failed: %v", err)
		}
	}

	duration := time.Since(startTime)
	fmt.Printf("\nTarget migration completed in %v!\n", duration)
}

func runDirectMigration(dbManager *database.Manager, cfg *config.Config, target string) {
	fmt.Printf("Starting DIRECT migration of: %s (NO DISCOVERY)\n", target)

	startTime := time.Now()
	migrationService := services.NewMigrationService(
		dbManager.GetSourceDB(),
		dbManager.GetDestDB(),
		cfg.BufferSize,
		cfg.MaxGoroutines,
	)
	migrationService.SetDatabaseManager(dbManager)

	if strings.Contains(target, ".") {
		parts := strings.Split(target, ".")
		if len(parts) != 2 {
			log.Fatalf("Invalid table format. Use: schema.table")
		}

		schema := parts[0]
		table := parts[1]
		fmt.Printf("Migrating TABLE: %s.%s\n", schema, table)
		if err := migrationService.MigrateTableDirect(schema, table); err != nil {
			log.Fatalf("Table migration failed: %v", err)
		}

	} else {
		fmt.Printf("Migrating SCHEMA: %s\n", target)
		if err := migrationService.MigrateSchemaData(target); err != nil {
			log.Fatalf("Schema migration failed: %v", err)
		}
	}

	duration := time.Since(startTime)
	fmt.Printf("\nDIRECT migration completed in %v! (MUCH FASTER!)\n", duration)
}

func runDirectReplication(dbManager *database.Manager, cfg *config.Config, target string) {
	fmt.Printf("Starting DIRECT replication of: %s (NO DISCOVERY)\n", target)
	replicationService := services.NewReplicationService(dbManager.GetDestDB())

	if strings.Contains(target, ".") {
		parts := strings.Split(target, ".")
		schema := parts[0]
		table := parts[1]

		fmt.Printf("Creating TABLE structure: %s.%s\n", schema, table)
		if err := replicationService.ReplicateTableDirect(schema, table); err != nil {
			log.Fatalf("Table replication failed: %v", err)
		}

	} else {
		fmt.Printf("Creating SCHEMA structure: %s\n", target)
		if err := replicationService.ReplicateSchemaDirect(target); err != nil {
			log.Fatalf("Schema replication failed: %v", err)
		}
	}

	fmt.Println("DIRECT replication completed!")
}

func runDirectFullMigration(dbManager *database.Manager, cfg *config.Config, target string) {
	fmt.Printf("Starting DIRECT FULL migration of: %s (NO DISCOVERY)\n", target)
	startTime := time.Now()
	fmt.Printf("Step 1: Creating structure for %s...\n", target)
	runDirectReplication(dbManager, cfg, target)
	fmt.Printf("Step 2: Migrating data for %s...\n", target)
	runDirectMigration(dbManager, cfg, target)
	duration := time.Since(startTime)
	fmt.Printf("\nDIRECT FULL migration completed in %v! (ULTRA FAST!)\n", duration)
}
