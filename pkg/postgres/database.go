package postgres

import (
	"fmt"

	"github.com/lib/pq"
)

const (
	CASCADE              = "CASCADE"
	RESTRICT             = "RESTRICT"
	CREATE_DB            = `CREATE DATABASE "%s"`
	CREATE_SCHEMA        = `CREATE SCHEMA IF NOT EXISTS "%s" AUTHORIZATION "%s"`
	CREATE_EXTENSION     = `CREATE EXTENSION IF NOT EXISTS "%s"`
	ALTER_DB_OWNER       = `ALTER DATABASE "%s" OWNER TO "%s"`
	DROP_DATABASE        = `DROP DATABASE "%s"`
	DROP_EXTENSION       = `DROP EXTENSION IF EXISTS "%s" %s`
	DROP_SCHEMA          = `DROP SCHEMA IF EXISTS "%s" %s`
	GRANT_USAGE_SCHEMA   = `GRANT USAGE ON SCHEMA "%s" TO "%s"`
	GRANT_ALL_TABLES     = `GRANT %s ON ALL TABLES IN SCHEMA "%s" TO "%s"`
	DEFAULT_PRIVS_SCHEMA = `ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA "%s" GRANT %s ON TABLES TO "%s"`
)

func (c *pg) CreateDB(dbname, role string) error {
	err := c.connect(c.default_database)
	if err != nil {
		return err
	}
	defer c.close()
	_, err = c.db.Exec(fmt.Sprintf(CREATE_DB, dbname))
	if err != nil {
		// eat DUPLICATE DATABASE ERROR
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != "42P04" {
			return err
		}
	}

	_, err = c.db.Exec(fmt.Sprintf(ALTER_DB_OWNER, dbname, role))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) CreateSchema(db, role, schema string) error {
	err := c.connect(db)
	if err != nil {
		return err
	}
	defer c.close()

	_, err = c.db.Exec(fmt.Sprintf(CREATE_SCHEMA, schema, role))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) DropDatabase(database string) error {
	err := c.connect(c.default_database)
	if err != nil {
		return err
	}
	defer c.close()
	_, err = c.db.Exec(fmt.Sprintf(DROP_DATABASE, database))
	// Error code 3D000 is returned if database doesn't exist
	if err != nil {
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != "3D000" {
			return err
		}
	}

	c.log.Info(fmt.Sprintf("Dropped database %s", database))

	return nil
}

func (c *pg) DropExtension(database, extension string, cascade bool) error {
	err := c.connect(database)
	if err != nil {
		return err
	}
	defer c.close()

	param := RESTRICT
	if cascade {
		param = CASCADE
	}
	_, err = c.db.Exec(fmt.Sprintf(DROP_EXTENSION, extension, param))
	if err != nil {
		return err
	}

	c.log.Info(fmt.Sprintf("Dropped extension %s on database %s with parameter %s", extension, database, param))

	return nil
}

func (c *pg) DropSchema(database, schema string, cascade bool) error {
	err := c.connect(database)
	if err != nil {
		return err
	}
	defer c.close()

	param := RESTRICT
	if cascade {
		param = CASCADE
	}
	_, err = c.db.Exec(fmt.Sprintf(DROP_SCHEMA, schema, param))
	if err != nil {
		return err
	}

	c.log.Info(fmt.Sprintf("Dropped schema %s on database %s with parameter %s", schema, database, param))

	return nil
}

func (c *pg) CreateExtension(db, extension string) error {
	err := c.connect(db)
	if err != nil {
		return err
	}
	defer c.close()

	_, err = c.db.Exec(fmt.Sprintf(CREATE_EXTENSION, extension))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) SetSchemaPrivileges(db, creator, role, schema, privs string) error {
	err := c.connect(db)
	if err != nil {
		return err
	}
	defer c.close()

	// Grant role usage on schema
	_, err = c.db.Exec(fmt.Sprintf(GRANT_USAGE_SCHEMA, schema, role))
	if err != nil {
		return err
	}

	// Grant role privs on existing tables in schema
	_, err = c.db.Exec(fmt.Sprintf(GRANT_ALL_TABLES, privs, schema, role))
	if err != nil {
		return err
	}

	// Grant role privs on future tables in schema
	_, err = c.db.Exec(fmt.Sprintf(DEFAULT_PRIVS_SCHEMA, creator, schema, privs, role))
	if err != nil {
		return err
	}
	return nil
}
