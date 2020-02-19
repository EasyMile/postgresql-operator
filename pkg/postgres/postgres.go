package postgres

import (
	"database/sql"
	"fmt"

	"github.com/easymile/postgresql-operator/pkg/apis/postgresql/v1alpha1"
	"github.com/go-logr/logr"
)

type PG interface {
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
	DropDatabase(db string) error
	DropRole(role, newOwner, database string) error
	DropSchema(database, schema string, cascade bool) error
	DropExtension(database, extension string, cascade bool) error
	GetUser() string
	GetHost() string
	GetPort() int
	GetDefaultDatabase() string
	Ping() error
}

type pg struct {
	db               *sql.DB
	log              logr.Logger
	host             string
	port             int
	user             string
	pass             string
	args             string
	default_database string
}

func NewPG(host, user, password, uri_args, default_database string, port int, cloud_type v1alpha1.ProviderType, logger logr.Logger) PG {
	postgres := &pg{
		log:              logger,
		host:             host,
		port:             port,
		user:             user,
		pass:             password,
		args:             uri_args,
		default_database: default_database,
	}

	switch cloud_type {
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

func (c *pg) GetPort() int {
	return c.port
}

func (c *pg) GetDefaultDatabase() string {
	return c.default_database
}

func (c *pg) connect(database string) error {
	pgUrl := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?%s", c.user, c.pass, c.host, c.port, database, c.args)
	db, err := sql.Open("postgres", pgUrl)
	if err != nil {
		return err
	}
	c.db = db
	return nil
}

func (c *pg) Ping() error {
	err := c.connect(c.default_database)
	if err != nil {
		return err
	}
	err = c.db.Ping()
	if err != nil {
		return err
	}
	return c.close()
}

func (c *pg) close() error {
	return c.db.Close()
}
