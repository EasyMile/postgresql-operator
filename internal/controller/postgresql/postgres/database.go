package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

const (
	CascadeKeyword                 = "CASCADE"
	RestrictKeyword                = "RESTRICT"
	CreateDBSQLTemplate            = `CREATE DATABASE "%s" WITH OWNER = "%s"`
	ChangeDBOwnerSQLTemplate       = `ALTER DATABASE "%s" OWNER TO "%s"`
	GetDatabaseOwnerSQLTemplate    = `SELECT pg_catalog.pg_get_userbyid(datdba) as owner FROM pg_database WHERE datname='%s'`
	RenameDatabaseSQLTemplate      = `ALTER DATABASE "%s" RENAME TO "%s"`
	CreateSchemaSQLTemplate        = `CREATE SCHEMA IF NOT EXISTS "%s" AUTHORIZATION "%s"`
	CreateExtensionSQLTemplate     = `CREATE EXTENSION IF NOT EXISTS "%s"`
	DropDatabaseSQLTemplate        = `DROP DATABASE "%s"`
	GetExtensionListSQLTemplate    = `SELECT extname FROM pg_extension;`
	DropExtensionSQLTemplate       = `DROP EXTENSION IF EXISTS "%s" %s`
	GetSchemaListSQLTemplate       = `SELECT schema_name FROM information_schema.schemata`
	DropSchemaSQLTemplate          = `DROP SCHEMA IF EXISTS "%s" %s`
	GrantUsageSchemaSQLTemplate    = `GRANT USAGE ON SCHEMA "%s" TO "%s"`
	GrantAllTablesSQLTemplate      = `GRANT %s ON ALL TABLES IN SCHEMA "%s" TO "%s"`
	DefaultPrivsSchemaSQLTemplate  = `ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA "%s" GRANT %s ON TABLES TO "%s"`
	GetTablesFromSchemaSQLTemplate = `SELECT tablename,tableowner FROM pg_tables WHERE schemaname = '%s'`
	ChangeTableOwnerSQLTemplate    = `ALTER TABLE IF EXISTS "%s" OWNER TO "%s"`
	ChangeTypeOwnerSQLTemplate     = `ALTER TYPE "%s"."%s" OWNER TO "%s"`
	GetColumnsFromTableSQLTemplate = `SELECT column_name FROM information_schema.columns WHERE table_schema = '%s' AND table_name = '%s'`
	// Got and edited from : https://stackoverflow.com/questions/3660787/how-to-list-custom-types-using-postgres-information-schema
	GetTypesFromSchemaSQLTemplate = `SELECT      t.typname as type, pg_catalog.pg_get_userbyid(t.typowner) as owner
FROM        pg_type t
LEFT JOIN   pg_catalog.pg_namespace n ON n.oid = t.typnamespace
WHERE       (t.typrelid = 0 OR (SELECT c.relkind = 'c' FROM pg_catalog.pg_class c WHERE c.oid = t.typrelid))
AND     NOT EXISTS(SELECT 1 FROM pg_catalog.pg_type el WHERE el.oid = t.typelem AND el.typarray = t.oid)
AND     n.nspname = '%s';`
	DuplicateDatabaseErrorCode = "42P04"
)

func (c *pg) GetColumnNamesFromTable(ctx context.Context, database string, schemaName string, tableName string) ([]string, error) {
	err := c.connect(database)
	if err != nil {
		return nil, err
	}

	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(GetColumnsFromTableSQLTemplate, schemaName, tableName))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res := []string{}

	for rows.Next() {
		it := ""
		// Scan
		err = rows.Scan(&it)
		// Check error
		if err != nil {
			return nil, err
		}
		// Save
		res = append(res, it)
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *pg) GetDatabaseOwner(ctx context.Context, dbname string) (string, error) {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return "", err
	}

	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(GetDatabaseOwnerSQLTemplate, dbname))
	if err != nil {
		return "", err
	}

	defer rows.Close()

	res := []string{}

	for rows.Next() {
		it := ""
		// Scan
		err = rows.Scan(&it)
		// Check error
		if err != nil {
			return "", err
		}
		// Save
		res = append(res, it)
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return "", err
	}

	if len(res) != 1 {
		return "", errors.New("select on database mustn't give more than one result. there is a severe issue somewhere")
	}

	// Check length
	if len(res) == 0 {
		return "", nil
	}

	return res[0], nil
}

func (c *pg) IsDatabaseExist(ctx context.Context, dbname string) (bool, error) {
	o, err := c.GetDatabaseOwner(ctx, dbname)
	// Check error
	if err != nil {
		return false, nil
	}

	return o != "", nil
}

func (c *pg) RenameDatabase(ctx context.Context, oldname, newname string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(RenameDatabaseSQLTemplate, oldname, newname))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) CreateDB(ctx context.Context, dbname, role string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(CreateDBSQLTemplate, dbname, role))
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

func (c *pg) ChangeDBOwner(ctx context.Context, dbname, owner string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(ChangeDBOwnerSQLTemplate, dbname, owner))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) CreateSchema(ctx context.Context, db, role, schema string) error {
	err := c.connect(db)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(CreateSchemaSQLTemplate, schema, role))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) GetTablesInSchema(ctx context.Context, db, schema string) ([]*TableOwnership, error) {
	err := c.connect(db)
	if err != nil {
		return nil, err
	}

	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(GetTablesFromSchemaSQLTemplate, schema))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res := []*TableOwnership{}

	for rows.Next() {
		it := &TableOwnership{}
		// Scan
		err = rows.Scan(&it.TableName, &it.Owner)
		// Check error
		if err != nil {
			return nil, err
		}
		// Save
		res = append(res, it)
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *pg) ChangeTableOwner(ctx context.Context, db, table, owner string) error {
	err := c.connect(db)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(ChangeTableOwnerSQLTemplate, table, owner))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) GetTypesInSchema(ctx context.Context, db, schema string) ([]*TypeOwnership, error) {
	err := c.connect(db)
	if err != nil {
		return nil, err
	}

	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(GetTypesFromSchemaSQLTemplate, schema))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res := []*TypeOwnership{}

	for rows.Next() {
		it := &TypeOwnership{}
		// Scan
		err = rows.Scan(&it.TypeName, &it.Owner)
		// Check error
		if err != nil {
			return nil, err
		}
		// Save
		res = append(res, it)
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *pg) ChangeTypeOwnerInSchema(ctx context.Context, db, schema, typeName, owner string) error {
	err := c.connect(db)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(ChangeTypeOwnerSQLTemplate, schema, typeName, owner))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) DropDatabase(ctx context.Context, database string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(DropDatabaseSQLTemplate, database))
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

func (c *pg) DropExtension(ctx context.Context, database, extension string, cascade bool) error {
	err := c.connect(database)
	if err != nil {
		return err
	}

	param := RestrictKeyword
	if cascade {
		param = CascadeKeyword
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(DropExtensionSQLTemplate, extension, param))
	if err != nil {
		return err
	}

	c.log.Info(fmt.Sprintf("Dropped extension %s on database %s with parameter %s", extension, database, param))

	return nil
}

func (c *pg) DropSchema(ctx context.Context, database, schema string, cascade bool) error {
	err := c.connect(database)
	if err != nil {
		return err
	}

	param := RestrictKeyword
	if cascade {
		param = CascadeKeyword
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(DropSchemaSQLTemplate, schema, param))
	if err != nil {
		return err
	}

	c.log.Info(fmt.Sprintf("Dropped schema %s on database %s with parameter %s", schema, database, param))

	return nil
}

func (c *pg) ListSchema(ctx context.Context, database string) ([]string, error) {
	err := c.connect(database)
	if err != nil {
		return nil, err
	}

	rows, err := c.db.QueryContext(ctx, GetSchemaListSQLTemplate)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res := []string{}

	for rows.Next() {
		it := ""
		// Scan
		err = rows.Scan(&it)
		// Check error
		if err != nil {
			return nil, err
		}
		// Save
		res = append(res, it)
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *pg) ListExtensions(ctx context.Context, database string) ([]string, error) {
	err := c.connect(database)
	if err != nil {
		return nil, err
	}

	rows, err := c.db.QueryContext(ctx, GetExtensionListSQLTemplate)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res := []string{}

	for rows.Next() {
		it := ""
		// Scan
		err = rows.Scan(&it)
		// Check error
		if err != nil {
			return nil, err
		}
		// Save
		res = append(res, it)
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *pg) CreateExtension(ctx context.Context, db, extension string) error {
	err := c.connect(db)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(CreateExtensionSQLTemplate, extension))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) SetSchemaPrivileges(ctx context.Context, db, creator, role, schema, privs string) error {
	err := c.connect(db)
	if err != nil {
		return err
	}

	// Grant role usage on schema
	_, err = c.db.ExecContext(ctx, fmt.Sprintf(GrantUsageSchemaSQLTemplate, schema, role))
	if err != nil {
		return err
	}

	// Grant role privs on existing tables in schema
	_, err = c.db.ExecContext(ctx, fmt.Sprintf(GrantAllTablesSQLTemplate, privs, schema, role))
	if err != nil {
		return err
	}

	// Grant role privs on future tables in schema
	_, err = c.db.ExecContext(ctx, fmt.Sprintf(DefaultPrivsSchemaSQLTemplate, creator, schema, privs, role))
	if err != nil {
		return err
	}

	return nil
}
