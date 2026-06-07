package services

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/kathedrall/mysql-twin-backup/internal/models"
)

type UserMigrationService struct {
	sourceDB *sql.DB
	destDB   *sql.DB
}

func NewUserMigrationService(sourceDB, destDB *sql.DB) *UserMigrationService {
	return &UserMigrationService{
		sourceDB: sourceDB,
		destDB:   destDB,
	}
}

func (ums *UserMigrationService) MigrateUsers() (*models.UserMigrationSummary, error) {
	summary := &models.UserMigrationSummary{
		Errors: make([]string, 0),
	}

	log.Printf(" Starting MySQL user and privilege migration...")

	// 1. Discover all users
	users, err := ums.discoverUsers()
	if err != nil {
		return summary, fmt.Errorf("failed to discover users: %w", err)
	}

	summary.TotalUsers = len(users)
	log.Printf(" Discovered %d users in source database", len(users))

	// 2. Migrate users
	for _, user := range users {
		if err := ums.migrateUser(user); err != nil {
			errMsg := fmt.Sprintf("Failed to migrate user %s@%s: %v", user.User, user.Host, err)
			summary.Errors = append(summary.Errors, errMsg)
			summary.ErrorUsers++
			log.Printf(" %s", errMsg)
			continue
		}
		summary.MigratedUsers++
		log.Printf(" Migrated user: %s@%s", user.User, user.Host)
	}

	// 3. Discover and migrate global privileges
	globalPrivs, err := ums.discoverGlobalPrivileges()
	if err != nil {
		return summary, fmt.Errorf("failed to discover global privileges: %w", err)
	}

	summary.TotalPrivileges += len(globalPrivs)

	for _, priv := range globalPrivs {
		if err := ums.migrateGlobalPrivilege(priv); err != nil {
			errMsg := fmt.Sprintf("Failed to migrate global privilege for %s@%s: %v", priv.User, priv.Host, err)
			summary.Errors = append(summary.Errors, errMsg)
			summary.ErrorPrivileges++
			log.Printf(" %s", errMsg)
			continue
		}
		summary.MigratedPrivileges++
	}

	// 4. Discover and migrate database privileges
	dbPrivs, err := ums.discoverDatabasePrivileges()
	if err != nil {
		return summary, fmt.Errorf("failed to discover database privileges: %w", err)
	}

	summary.TotalPrivileges += len(dbPrivs)

	for _, priv := range dbPrivs {
		if err := ums.migrateDatabasePrivilege(priv); err != nil {
			errMsg := fmt.Sprintf("Failed to migrate database privilege for %s@%s on %s: %v", priv.User, priv.Host, priv.Database, err)
			summary.Errors = append(summary.Errors, errMsg)
			summary.ErrorPrivileges++
			log.Printf(" %s", errMsg)
			continue
		}
		summary.MigratedPrivileges++
	}

	// 5. Discover and migrate table privileges
	tablePrivs, err := ums.discoverTablePrivileges()
	if err != nil {
		return summary, fmt.Errorf("failed to discover table privileges: %w", err)
	}

	summary.TotalPrivileges += len(tablePrivs)

	for _, priv := range tablePrivs {
		if err := ums.migrateTablePrivilege(priv); err != nil {
			errMsg := fmt.Sprintf("Failed to migrate table privilege for %s@%s on %s.%s: %v", priv.User, priv.Host, priv.Database, priv.Table, err)
			summary.Errors = append(summary.Errors, errMsg)
			summary.ErrorPrivileges++
			log.Printf(" %s", errMsg)
			continue
		}
		summary.MigratedPrivileges++
	}

	// 6. Flush privileges
	if err := ums.flushPrivileges(); err != nil {
		return summary, fmt.Errorf("failed to flush privileges: %w", err)
	}

	log.Printf(" User migration completed!")
	ums.printSummary(summary)

	return summary, nil
}

func (ums *UserMigrationService) discoverUsers() ([]models.MySQLUser, error) {
	query := `SELECT User, 
					Host, 
					authentication_string as password_hash,
					ssl_type, 
					ssl_cipher, 
					x509_issuer, 
					x509_subject,
					max_questions, 
					max_updates, 
					max_connections, 
					max_user_connections,
					plugin, 
					authentication_string,
					password_expired, 
					password_lifetime, 
					account_locked
				FROM mysql.user 
				WHERE User != '' 
					AND User NOT IN ('mysql.sys', 'mysql.session', 'mysql.infoschema')
					AND Host NOT IN ('localhost')
				ORDER BY User, Host`

	rows, err := ums.sourceDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []models.MySQLUser
	for rows.Next() {
		var user models.MySQLUser
		var sslType, sslCipher, x509Issuer, x509Subject sql.NullString
		var maxQuestions, maxUpdates, maxConnections, maxUserConnections sql.NullInt64
		var pluginName, authString, passwordExpired, accountLocked sql.NullString
		var passwordLifetime sql.NullInt64

		err := rows.Scan(
			&user.User, &user.Host, &user.PasswordHash,
			&sslType, &sslCipher, &x509Issuer, &x509Subject,
			&maxQuestions, &maxUpdates, &maxConnections, &maxUserConnections,
			&pluginName, &authString,
			&passwordExpired, &passwordLifetime, &accountLocked,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		// Handle nullable fields
		user.SSLType = sslType.String
		user.SSLCipher = sslCipher.String
		user.X509Issuer = x509Issuer.String
		user.X509Subject = x509Subject.String
		user.MaxQuestions = maxQuestions.Int64
		user.MaxUpdates = maxUpdates.Int64
		user.MaxConnections = maxConnections.Int64
		user.MaxUserConnections = maxUserConnections.Int64
		user.PluginName = pluginName.String
		user.AuthString = authString.String
		user.PasswordExpired = passwordExpired.String
		user.PasswordLifetime = passwordLifetime.Int64
		user.AccountLocked = accountLocked.String

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

func (ums *UserMigrationService) migrateUser(user models.MySQLUser) error {
	// Skip root and system users
	if user.User == "root" || strings.HasPrefix(user.User, "mysql.") {
		return nil
	}

	// Create user with the same password hash
	var createQuery string
	if user.PasswordHash != "" {
		createQuery = fmt.Sprintf(
			"CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED WITH %s AS '%s'",
			user.User, user.Host,
			ums.getPluginName(user.PluginName),
			user.PasswordHash,
		)
	} else {
		createQuery = fmt.Sprintf(
			"CREATE USER IF NOT EXISTS '%s'@'%s'",
			user.User, user.Host,
		)
	}

	if _, err := ums.destDB.Exec(createQuery); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Set resource limits if they exist
	if user.MaxQuestions > 0 || user.MaxUpdates > 0 || user.MaxConnections > 0 || user.MaxUserConnections > 0 {
		alterQuery := fmt.Sprintf(
			"ALTER USER '%s'@'%s' WITH MAX_QUERIES_PER_HOUR %d MAX_UPDATES_PER_HOUR %d MAX_CONNECTIONS_PER_HOUR %d MAX_USER_CONNECTIONS %d",
			user.User, user.Host,
			user.MaxQuestions, user.MaxUpdates, user.MaxConnections, user.MaxUserConnections,
		)

		if _, err := ums.destDB.Exec(alterQuery); err != nil {
			log.Printf("Warning: failed to set resource limits for %s@%s: %v", user.User, user.Host, err)
		}
	}

	// Set SSL requirements if they exist
	if user.SSLType != "" {
		sslQuery := fmt.Sprintf("ALTER USER '%s'@'%s' REQUIRE SSL", user.User, user.Host)
		if _, err := ums.destDB.Exec(sslQuery); err != nil {
			log.Printf("Warning: failed to set SSL requirements for %s@%s: %v", user.User, user.Host, err)
		}
	}

	return nil
}

func (ums *UserMigrationService) getPluginName(plugin string) string {
	switch plugin {
	case "mysql_native_password":
		return "mysql_native_password"
	case "caching_sha2_password":
		return "caching_sha2_password"
	case "sha256_password":
		return "sha256_password"
	default:
		return "mysql_native_password" // Default fallback
	}
}

func (ums *UserMigrationService) discoverGlobalPrivileges() ([]models.MySQLPrivilege, error) {
	query := `SELECT User, 
				Host,
				Select_priv, 
				Insert_priv, 
				Update_priv,
				Delete_priv, 
				Create_priv, 
				Drop_priv,
				Reload_priv, 
				Shutdown_priv, 
				Process_priv, 
				File_priv, 
				Grant_priv, 
				References_priv,
				Index_priv, 
				Alter_priv, 
				Show_db_priv, 
				Super_priv, 
				Create_tmp_table_priv,
				Lock_tables_priv, 
				Execute_priv, 
				Repl_slave_priv, 
				Repl_client_priv,
				Create_view_priv, 
				Show_view_priv, 
				Create_routine_priv, 
				Alter_routine_priv,
				Create_user_priv, 
				Event_priv, 
				Trigger_priv
			FROM mysql.user 
			WHERE User != '' 
				AND User NOT IN ('mysql.sys', 'mysql.session', 'mysql.infoschema', 'root')`

	rows, err := ums.sourceDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query global privileges: %w", err)
	}
	defer rows.Close()

	var privileges []models.MySQLPrivilege
	for rows.Next() {
		var priv models.MySQLPrivilege
		err := rows.Scan(
			&priv.User, &priv.Host,
			&priv.SelectPriv, &priv.InsertPriv, &priv.UpdatePriv, &priv.DeletePriv,
			&priv.CreatePriv, &priv.DropPriv, &priv.ReloadPriv, &priv.ShutdownPriv,
			&priv.ProcessPriv, &priv.FilePriv, &priv.GrantPriv, &priv.ReferencesPriv,
			&priv.IndexPriv, &priv.AlterPriv, &priv.ShowDBPriv, &priv.SuperPriv,
			&priv.CreateTmpTablePriv, &priv.LockTablesPriv, &priv.ExecutePriv,
			&priv.ReplSlavePriv, &priv.ReplClientPriv, &priv.CreateViewPriv,
			&priv.ShowViewPriv, &priv.CreateRoutinePriv, &priv.AlterRoutinePriv,
			&priv.CreateUserPriv, &priv.EventPriv, &priv.TriggerPriv,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan global privilege: %w", err)
		}

		privileges = append(privileges, priv)
	}

	return privileges, nil
}

func (ums *UserMigrationService) migrateGlobalPrivilege(priv models.MySQLPrivilege) error {
	var grantedPrivs []string

	// Check each privilege and add to list if granted
	if priv.SelectPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "SELECT")
	}
	if priv.InsertPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "INSERT")
	}
	if priv.UpdatePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "UPDATE")
	}
	if priv.DeletePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "DELETE")
	}
	if priv.CreatePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "CREATE")
	}
	if priv.DropPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "DROP")
	}
	if priv.ReloadPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "RELOAD")
	}
	if priv.ProcessPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "PROCESS")
	}
	if priv.FilePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "FILE")
	}
	if priv.IndexPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "INDEX")
	}
	if priv.AlterPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "ALTER")
	}
	if priv.ShowDBPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "SHOW DATABASES")
	}
	if priv.CreateTmpTablePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "CREATE TEMPORARY TABLES")
	}
	if priv.LockTablesPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "LOCK TABLES")
	}
	if priv.ExecutePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "EXECUTE")
	}
	if priv.ReplSlavePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "REPLICATION SLAVE")
	}
	if priv.ReplClientPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "REPLICATION CLIENT")
	}
	if priv.CreateViewPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "CREATE VIEW")
	}
	if priv.ShowViewPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "SHOW VIEW")
	}
	if priv.CreateRoutinePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "CREATE ROUTINE")
	}
	if priv.AlterRoutinePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "ALTER ROUTINE")
	}
	if priv.CreateUserPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "CREATE USER")
	}
	if priv.EventPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "EVENT")
	}
	if priv.TriggerPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "TRIGGER")
	}

	// Grant privileges if any exist
	if len(grantedPrivs) > 0 {
		grantQuery := fmt.Sprintf(
			"GRANT %s ON *.* TO '%s'@'%s'",
			strings.Join(grantedPrivs, ", "),
			priv.User, priv.Host,
		)

		if priv.GrantPriv == "Y" {
			grantQuery += " WITH GRANT OPTION"
		}

		if _, err := ums.destDB.Exec(grantQuery); err != nil {
			return fmt.Errorf("failed to grant global privileges: %w", err)
		}
	}

	return nil
}

func (ums *UserMigrationService) discoverDatabasePrivileges() ([]models.MySQLPrivilege, error) {
	query := `SELECT User, 
				Host, 
				Db as Database_name,
				Select_priv, 
				Insert_priv, 
				Update_priv, 
				Delete_priv, 
				Create_priv, 
				Drop_priv,
				Grant_priv, 
				References_priv, 
				Index_priv, 
				Alter_priv,
				Create_tmp_table_priv, 
				Lock_tables_priv, 
				Create_view_priv, 
				Show_view_priv,
				Create_routine_priv, 
				Alter_routine_priv, 
				Execute_priv, 
				Event_priv, 
				Trigger_priv
			   FROM mysql.db 
			   WHERE User != '' 
				AND User NOT IN ('mysql.sys', 'mysql.session', 'mysql.infoschema', 'root')`

	rows, err := ums.sourceDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query database privileges: %w", err)
	}
	defer rows.Close()

	var privileges []models.MySQLPrivilege
	for rows.Next() {
		var priv models.MySQLPrivilege
		err := rows.Scan(
			&priv.User, &priv.Host, &priv.Database,
			&priv.SelectPriv, &priv.InsertPriv, &priv.UpdatePriv, &priv.DeletePriv,
			&priv.CreatePriv, &priv.DropPriv, &priv.GrantPriv, &priv.ReferencesPriv,
			&priv.IndexPriv, &priv.AlterPriv, &priv.CreateTmpTablePriv,
			&priv.LockTablesPriv, &priv.CreateViewPriv, &priv.ShowViewPriv,
			&priv.CreateRoutinePriv, &priv.AlterRoutinePriv, &priv.ExecutePriv,
			&priv.EventPriv, &priv.TriggerPriv,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan database privilege: %w", err)
		}

		privileges = append(privileges, priv)
	}

	return privileges, nil
}

func (ums *UserMigrationService) migrateDatabasePrivilege(priv models.MySQLPrivilege) error {
	var grantedPrivs []string

	// Build privilege list similar to global privileges
	if priv.SelectPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "SELECT")
	}
	if priv.InsertPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "INSERT")
	}
	if priv.UpdatePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "UPDATE")
	}
	if priv.DeletePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "DELETE")
	}
	if priv.CreatePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "CREATE")
	}
	if priv.DropPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "DROP")
	}
	if priv.IndexPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "INDEX")
	}
	if priv.AlterPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "ALTER")
	}
	if priv.CreateTmpTablePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "CREATE TEMPORARY TABLES")
	}
	if priv.LockTablesPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "LOCK TABLES")
	}
	if priv.CreateViewPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "CREATE VIEW")
	}
	if priv.ShowViewPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "SHOW VIEW")
	}
	if priv.CreateRoutinePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "CREATE ROUTINE")
	}
	if priv.AlterRoutinePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "ALTER ROUTINE")
	}
	if priv.ExecutePriv == "Y" {
		grantedPrivs = append(grantedPrivs, "EXECUTE")
	}
	if priv.EventPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "EVENT")
	}
	if priv.TriggerPriv == "Y" {
		grantedPrivs = append(grantedPrivs, "TRIGGER")
	}

	// Grant privileges if any exist
	if len(grantedPrivs) > 0 {
		grantQuery := fmt.Sprintf(
			"GRANT %s ON `%s`.* TO '%s'@'%s'",
			strings.Join(grantedPrivs, ", "),
			priv.Database, priv.User, priv.Host,
		)

		if priv.GrantPriv == "Y" {
			grantQuery += " WITH GRANT OPTION"
		}

		if _, err := ums.destDB.Exec(grantQuery); err != nil {
			return fmt.Errorf("failed to grant database privileges: %w", err)
		}
	}

	return nil
}

func (ums *UserMigrationService) discoverTablePrivileges() ([]models.MySQLPrivilege, error) {
	query := `SELECT User, 
				Host, 
				Db as Database_name, 
				Table_name,
				Table_priv, 
				Column_priv
			  FROM mysql.tables_priv 
			WHERE User != '' 
				AND User NOT IN ('mysql.sys', 'mysql.session', 'mysql.infoschema', 'root')`

	rows, err := ums.sourceDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query table privileges: %w", err)
	}
	defer rows.Close()

	var privileges []models.MySQLPrivilege
	for rows.Next() {
		var priv models.MySQLPrivilege
		var tablePriv, columnPriv string

		err := rows.Scan(
			&priv.User, &priv.Host, &priv.Database, &priv.Table,
			&tablePriv, &columnPriv,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan table privilege: %w", err)
		}

		priv.PrivilegeType = tablePriv
		privileges = append(privileges, priv)
	}

	return privileges, nil
}

func (ums *UserMigrationService) migrateTablePrivilege(priv models.MySQLPrivilege) error {
	if priv.PrivilegeType == "" {
		return nil
	}

	// Parse the privilege type (comma-separated values)
	privs := strings.Split(priv.PrivilegeType, ",")
	var grantedPrivs []string

	for _, p := range privs {
		p = strings.TrimSpace(p)
		switch strings.ToUpper(p) {
		case "SELECT":
			grantedPrivs = append(grantedPrivs, "SELECT")
		case "INSERT":
			grantedPrivs = append(grantedPrivs, "INSERT")
		case "UPDATE":
			grantedPrivs = append(grantedPrivs, "UPDATE")
		case "DELETE":
			grantedPrivs = append(grantedPrivs, "DELETE")
		case "CREATE":
			grantedPrivs = append(grantedPrivs, "CREATE")
		case "DROP":
			grantedPrivs = append(grantedPrivs, "DROP")
		case "INDEX":
			grantedPrivs = append(grantedPrivs, "INDEX")
		case "ALTER":
			grantedPrivs = append(grantedPrivs, "ALTER")
		}
	}

	if len(grantedPrivs) > 0 {
		grantQuery := fmt.Sprintf(
			"GRANT %s ON `%s`.`%s` TO '%s'@'%s'",
			strings.Join(grantedPrivs, ", "),
			priv.Database, priv.Table, priv.User, priv.Host,
		)

		if _, err := ums.destDB.Exec(grantQuery); err != nil {
			return fmt.Errorf("failed to grant table privileges: %w", err)
		}
	}

	return nil
}

func (ums *UserMigrationService) flushPrivileges() error {
	if _, err := ums.destDB.Exec("FLUSH PRIVILEGES"); err != nil {
		return fmt.Errorf("failed to flush privileges: %w", err)
	}
	return nil
}

func (ums *UserMigrationService) printSummary(summary *models.UserMigrationSummary) {
	fmt.Printf("\n User Migration Summary:\n")
	fmt.Printf("==============================\n")
	fmt.Printf(" Users: %d total, %d migrated, %d errors\n",
		summary.TotalUsers, summary.MigratedUsers, summary.ErrorUsers)
	fmt.Printf(" Privileges: %d total, %d migrated, %d errors\n",
		summary.TotalPrivileges, summary.MigratedPrivileges, summary.ErrorPrivileges)

	if len(summary.Errors) > 0 {
		fmt.Printf("\n Errors encountered:\n")
		for _, err := range summary.Errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	fmt.Printf("==============================\n")
}
