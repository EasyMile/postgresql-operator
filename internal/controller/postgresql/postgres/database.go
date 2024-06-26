package postgres

import (
	"fmt"

	"github.com/lib/pq"
)

const (
	CascadeKeyword                 = "CASCADE"
	RestrictKeyword                = "RESTRICT"
	CreateDBSQLTemplate            = `CREATE DATABASE "%s" WITH OWNER = "%s"`
	ChangeDBOwnerSQLTemplate       = `ALTER DATABASE "%s" OWNER TO "%s"`
	IsDatabaseExistSQLTemplate     = `SELECT 1 FROM pg_database WHERE datname='%s'`
	RenameDatabaseSQLTemplate      = `ALTER DATABASE "%s" RENAME TO "%s"`
	CreateSchemaSQLTemplate        = `CREATE SCHEMA IF NOT EXISTS "%s" AUTHORIZATION "%s"`
	CreateExtensionSQLTemplate     = `CREATE EXTENSION IF NOT EXISTS "%s"`
	DropDatabaseSQLTemplate        = `DROP DATABASE "%s"`
	DropExtensionSQLTemplate       = `DROP EXTENSION IF EXISTS "%s" %s`
	DropSchemaSQLTemplate          = `DROP SCHEMA IF EXISTS "%s" %s`
	GrantUsageSchemaSQLTemplate    = `GRANT USAGE ON SCHEMA "%s" TO "%s"`
	GrantAllTablesSQLTemplate      = `GRANT %s ON ALL TABLES IN SCHEMA "%s" TO "%s"`
	DefaultPrivsSchemaSQLTemplate  = `ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA "%s" GRANT %s ON TABLES TO "%s"`
	GetTablesFromSchemaSQLTemplate = `SELECT table_name FROM information_schema.tables WHERE table_schema = '%s'`
	ChangeTableOwnerSQLTemplate    = `ALTER TABLE IF EXISTS "%s" OWNER TO "%s"`
	ChangeTypeOwnerSQLTemplate     = `ALTER TYPE "%s"."%s" OWNER TO "%s"`
	// Got and edited from : https://stackoverflow.com/questions/3660787/how-to-list-custom-types-using-postgres-information-schema
	GetTypesFromSchemaSQLTemplate = `SELECT      t.typname as type
FROM        pg_type t
LEFT JOIN   pg_catalog.pg_namespace n ON n.oid = t.typnamespace
WHERE       (t.typrelid = 0 OR (SELECT c.relkind = 'c' FROM pg_catalog.pg_class c WHERE c.oid = t.typrelid))
AND     NOT EXISTS(SELECT 1 FROM pg_catalog.pg_type el WHERE el.oid = t.typelem AND el.typarray = t.oid)
AND     n.nspname = '%s';`
	DuplicateDatabaseErrorCode = "42P04"
)

func (c *pg) IsDatabaseExist(dbname string) (bool, error) {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return false, err
	}

	res, err := c.db.Exec(fmt.Sprintf(IsDatabaseExistSQLTemplate, dbname))
	if err != nil {
		return false, err
	}
	// Get affected rows
	nb, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return nb == 1, nil
}

func (c *pg) RenameDatabase(oldname, newname string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(RenameDatabaseSQLTemplate, oldname, newname))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) CreateDB(dbname, role string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(CreateDBSQLTemplate, dbname, role))
	if err != nil {
		// eat DUPLICATE DATABASE ERROR
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != DuplicateDatabaseErrorCode {
			return err
		}
	}

	return nil
}

func (c *pg) ChangeDBOwner(dbname, owner string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(ChangeDBOwnerSQLTemplate, dbname, owner))
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

	_, err = c.db.Exec(fmt.Sprintf(CreateSchemaSQLTemplate, schema, role))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) GetTablesInSchema(db, schema string) ([]string, error) {
	err := c.connect(db)
	if err != nil {
		return nil, err
	}

	rows, err := c.db.Query(fmt.Sprintf(GetTablesFromSchemaSQLTemplate, schema))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res := []string{}

	for rows.Next() {
		tableName := ""
		// Scan
		err = rows.Scan(&tableName)
		// Check error
		if err != nil {
			return nil, err
		}
		// Save
		res = append(res, tableName)
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *pg) ChangeTableOwner(db, table, owner string) error {
	err := c.connect(db)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(ChangeTableOwnerSQLTemplate, table, owner))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) GetTypesInSchema(db, schema string) ([]string, error) {
	err := c.connect(db)
	if err != nil {
		return nil, err
	}

	rows, err := c.db.Query(fmt.Sprintf(GetTypesFromSchemaSQLTemplate, schema))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res := []string{}

	for rows.Next() {
		typeName := ""
		// Scan
		err = rows.Scan(&typeName)
		// Check error
		if err != nil {
			return nil, err
		}
		// Save
		res = append(res, typeName)
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *pg) ChangeTypeOwnerInSchema(db, schema, typeName, owner string) error {
	err := c.connect(db)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(ChangeTypeOwnerSQLTemplate, schema, typeName, owner))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) DropDatabase(database string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(DropDatabaseSQLTemplate, database))
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

	param := RestrictKeyword
	if cascade {
		param = CascadeKeyword
	}

	_, err = c.db.Exec(fmt.Sprintf(DropExtensionSQLTemplate, extension, param))
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

	param := RestrictKeyword
	if cascade {
		param = CascadeKeyword
	}

	_, err = c.db.Exec(fmt.Sprintf(DropSchemaSQLTemplate, schema, param))
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

	_, err = c.db.Exec(fmt.Sprintf(CreateExtensionSQLTemplate, extension))
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

	// Grant role usage on schema
	_, err = c.db.Exec(fmt.Sprintf(GrantUsageSchemaSQLTemplate, schema, role))
	if err != nil {
		return err
	}

	// Grant role privs on existing tables in schema
	_, err = c.db.Exec(fmt.Sprintf(GrantAllTablesSQLTemplate, privs, schema, role))
	if err != nil {
		return err
	}

	// Grant role privs on future tables in schema
	_, err = c.db.Exec(fmt.Sprintf(DefaultPrivsSchemaSQLTemplate, creator, schema, privs, role))
	if err != nil {
		return err
	}

	return nil
}
