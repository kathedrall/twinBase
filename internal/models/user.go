package models

type MySQLUser struct {
	User               string
	Host               string
	Password           string
	PasswordHash       string
	SSLType            string
	SSLCipher          string
	X509Issuer         string
	X509Subject        string
	MaxQuestions       int64
	MaxUpdates         int64
	MaxConnections     int64
	MaxUserConnections int64
	PluginName         string
	AuthString         string
	PasswordExpired    string
	PasswordLifetime   int64
	AccountLocked      string
}

type MySQLPrivilege struct {
	User          string
	Host          string
	Database      string
	Table         string
	Column        string
	PrivilegeType string
	IsGrantable   bool

	SelectPriv         string
	InsertPriv         string
	UpdatePriv         string
	DeletePriv         string
	CreatePriv         string
	DropPriv           string
	ReloadPriv         string
	ShutdownPriv       string
	ProcessPriv        string
	FilePriv           string
	GrantPriv          string
	ReferencesPriv     string
	IndexPriv          string
	AlterPriv          string
	ShowDBPriv         string
	SuperPriv          string
	CreateTmpTablePriv string
	LockTablesPriv     string
	ExecutePriv        string
	ReplSlavePriv      string
	ReplClientPriv     string
	CreateViewPriv     string
	ShowViewPriv       string
	CreateRoutinePriv  string
	AlterRoutinePriv   string
	CreateUserPriv     string
	EventPriv          string
	TriggerPriv        string
}

type MySQLRole struct {
	User        string
	Host        string
	IsRole      bool
	DefaultRole bool
}

type UserMigrationSummary struct {
	TotalUsers         int
	MigratedUsers      int
	SkippedUsers       int
	ErrorUsers         int
	TotalPrivileges    int
	MigratedPrivileges int
	ErrorPrivileges    int
	Errors             []string
}
