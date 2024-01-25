package postgres

import (
	"database/sql"
	"fmt"

	"github.com/easymile/postgresql-operator/apis/postgresql/v1alpha1"
	"github.com/go-logr/logr"
)

const MaxIdentifierLength = 63

type SetRoleOnDatabaseRoleSetting struct {
	Role     string
	Database string
}

type PG interface { //nolint:interfacebloat // This is needed
	CreateDB(dbname, username string) error
	IsDatabaseExist(dbname string) (bool, error)
	RenameDatabase(oldname, newname string) error
	CreateSchema(db, role, schema string) error
	CreateExtension(db, extension string) error
	CreateGroupRole(role string) error
	CreateUserRole(role, password string) (string, error)
	IsRoleExist(role string) (bool, error)
	RenameRole(oldname, newname string) error
	UpdatePassword(role, password string) error
	GrantRole(role, grantee string) error
	SetSchemaPrivileges(db, creator, role, schema, privs string) error
	RevokeRole(role, userRole string) error
	AlterDefaultLoginRole(role, setRole string) error
	AlterDefaultLoginRoleOnDatabase(role, setRole, database string) error
	RevokeUserSetRoleOnDatabase(role, database string) error
	DoesRoleHaveActiveSession(role string) (bool, error)
	DropDatabase(db string) error
	DropRoleAndDropAndChangeOwnedBy(role, newOwner, database string) error
	ChangeAndDropOwnedBy(role, newOwner, database string) error
	GetSetRoleOnDatabasesRoleSettings(role string) ([]*SetRoleOnDatabaseRoleSetting, error)
	DropRole(role string) error
	DropSchema(database, schema string, cascade bool) error
	DropExtension(database, extension string, cascade bool) error
	GetRoleMembership(role string) ([]string, error)
	GetUser() string
	GetHost() string
	GetPort() int
	GetDefaultDatabase() string
	GetArgs() string
	Ping() error
}

type pg struct {
	db              *sql.DB
	log             logr.Logger
	host            string
	user            string
	pass            string
	args            string
	defaultDatabase string
	name            string
	port            int
}

func NewPG(
	name,
	host,
	user,
	password,
	args,
	defaultDatabase string,
	port int,
	cloudType v1alpha1.ProviderType,
	logger logr.Logger,
) PG {
	postgres := &pg{
		log:             logger,
		host:            host,
		port:            port,
		user:            user,
		pass:            password,
		args:            args,
		defaultDatabase: defaultDatabase,
		name:            name,
	}

	switch cloudType {
	case v1alpha1.AWSProvider:
		return newAWSPG(postgres)
	case v1alpha1.AzureProvider:
		return newAzurePG(postgres)
	default:
		return postgres
	}
}

func (c *pg) GetUser() string {
	return c.user
}

func (c *pg) GetHost() string {
	return c.host
}

func (c *pg) GetArgs() string {
	return c.args
}

func (c *pg) GetPort() int {
	return c.port
}

func (c *pg) GetDefaultDatabase() string {
	return c.defaultDatabase
}

func (c *pg) GetName() string {
	return c.name
}

func (c *pg) GetPassword() string {
	return c.pass
}

func (c *pg) connect(database string) error {
	// Open or create pool
	db, err := getOrOpenPool(c, database)
	// Check error
	if err != nil {
		return err
	}
	// Save db
	c.db = db

	return nil
}

func (c *pg) Ping() error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	err = c.db.Ping()
	if err != nil {
		return err
	}

	return nil
}

func TemplatePostgresqlURLWithArgs(host, user, password, uriArgs, database string, port int) string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?%s", user, password, host, port, database, uriArgs)
}

func TemplatePostgresqlURL(host, user, password, database string, port int) string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", user, password, host, port, database)
}
