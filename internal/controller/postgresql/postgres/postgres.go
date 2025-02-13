package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
	"github.com/go-logr/logr"
)

const MaxIdentifierLength = 63

type SetRoleOnDatabaseRoleSetting struct {
	Role     string
	Database string
}

type TableOwnership struct {
	TableName string
	Owner     string
}

type TypeOwnership struct {
	TypeName string
	Owner    string
}

type PG interface { //nolint:interfacebloat // This is needed
	CreateDB(ctx context.Context, dbname, username string) error
	GetDatabaseOwner(ctx context.Context, dbname string) (string, error)
	ChangeDBOwner(ctx context.Context, dbname, owner string) error
	IsDatabaseExist(ctx context.Context, dbname string) (bool, error)
	RenameDatabase(ctx context.Context, oldname, newname string) error
	CreateSchema(ctx context.Context, db, role, schema string) error
	CreateExtension(ctx context.Context, db, extension string) error
	CreateGroupRole(ctx context.Context, role string) error
	CreateUserRole(ctx context.Context, role, password string, attributes *RoleAttributes) (string, error)
	AlterRoleAttributes(ctx context.Context, role string, attributes *RoleAttributes) error
	GetRoleAttributes(ctx context.Context, role string) (*RoleAttributes, error)
	IsRoleExist(ctx context.Context, role string) (bool, error)
	RenameRole(ctx context.Context, oldname, newname string) error
	UpdatePassword(ctx context.Context, role, password string) error
	GrantRole(ctx context.Context, role, grantee string, withAdminOption bool) error
	SetSchemaPrivileges(ctx context.Context, db, creator, role, schema, privs string) error
	RevokeRole(ctx context.Context, role, userRole string) error
	AlterDefaultLoginRole(ctx context.Context, role, setRole string) error
	AlterDefaultLoginRoleOnDatabase(ctx context.Context, role, setRole, database string) error
	RevokeUserSetRoleOnDatabase(ctx context.Context, role, database string) error
	DoesRoleHaveActiveSession(ctx context.Context, role string) (bool, error)
	DropDatabase(ctx context.Context, db string) error
	DropRoleAndDropAndChangeOwnedBy(ctx context.Context, role, newOwner, database string) error
	ChangeAndDropOwnedBy(ctx context.Context, role, newOwner, database string) error
	GetSetRoleOnDatabasesRoleSettings(ctx context.Context, role string) ([]*SetRoleOnDatabaseRoleSetting, error)
	DropRole(ctx context.Context, role string) error
	DropSchema(ctx context.Context, database, schema string, cascade bool) error
	ListSchema(ctx context.Context, database string) ([]string, error)
	ListExtensions(ctx context.Context, database string) ([]string, error)
	DropExtension(ctx context.Context, database, extension string, cascade bool) error
	GetRoleMembership(ctx context.Context, role string) ([]string, error)
	GetTablesInSchema(ctx context.Context, db, schema string) ([]*TableOwnership, error)
	ChangeTableOwner(ctx context.Context, db, table, owner string) error
	GetTypesInSchema(ctx context.Context, db, schema string) ([]*TypeOwnership, error)
	ChangeTypeOwnerInSchema(ctx context.Context, db, schema, typeName, owner string) error
	DropPublication(ctx context.Context, dbname, name string) error
	RenamePublication(ctx context.Context, dbname, oldname, newname string) error
	GetPublication(ctx context.Context, dbname, name string) (*PublicationResult, error)
	CreatePublication(ctx context.Context, dbname string, builder *CreatePublicationBuilder) error
	UpdatePublication(ctx context.Context, dbname, publicationName string, builder *UpdatePublicationBuilder) error
	ChangePublicationOwner(ctx context.Context, dbname string, publicationName string, owner string) error
	GetPublicationTablesDetails(ctx context.Context, db, publicationName string) ([]*PublicationTableDetail, error)
	DropReplicationSlot(ctx context.Context, name string) error
	CreateReplicationSlot(ctx context.Context, dbname, name, plugin string) error
	GetReplicationSlot(ctx context.Context, name string) (*ReplicationSlotResult, error)
	GetColumnNamesFromTable(ctx context.Context, database string, schemaName string, tableName string) ([]string, error)
	GetUser() string
	GetHost() string
	GetPort() int
	GetDefaultDatabase() string
	GetArgs() string
	Ping(ctx context.Context) error
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

func (c *pg) Ping(ctx context.Context) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	err = c.db.PingContext(ctx)
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
