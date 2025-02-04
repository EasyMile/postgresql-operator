package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

const (
	CreateGroupRoleSQLTemplate             = `CREATE ROLE "%s"`
	CreateUserRoleSQLTemplate              = `CREATE ROLE "%s" WITH LOGIN PASSWORD '%s' %s`
	GrantRoleSQLTemplate                   = `GRANT "%s" TO "%s"`
	GrantRoleWithAdminOptionSQLTemplate    = `GRANT "%s" TO "%s" WITH ADMIN OPTION`
	AlterUserSetRoleSQLTemplate            = `ALTER USER "%s" SET ROLE "%s"`
	AlterUserSetRoleOnDatabaseSQLTemplate  = `ALTER ROLE "%s" IN DATABASE "%s" SET ROLE "%s"`
	RevokeUserSetRoleOnDatabaseSQLTemplate = `ALTER ROLE "%s" IN DATABASE "%s" RESET role`
	RevokeRoleSQLTemplate                  = `REVOKE "%s" FROM "%s"`
	UpdatePasswordSQLTemplate              = `ALTER ROLE "%s" WITH PASSWORD '%s'` // #nosec
	DropRoleSQLTemplate                    = `DROP ROLE "%s"`
	DropOwnedBySQLTemplate                 = `DROP OWNED BY "%s"`
	ReassignObjectsSQLTemplate             = `REASSIGN OWNED BY "%s" TO "%s"`
	IsRoleExistSQLTemplate                 = `SELECT 1 FROM pg_roles WHERE rolname='%s'`
	RenameRoleSQLTemplate                  = `ALTER ROLE "%s" RENAME TO "%s"`
	AlterRoleWithOptionSQLTemplate         = `ALTER ROLE "%s" WITH %s`
	// Source: https://dba.stackexchange.com/questions/136858/postgresql-display-role-members
	GetRoleMembershipSQLTemplate = `SELECT r1.rolname as "role" FROM pg_catalog.pg_roles r JOIN pg_catalog.pg_auth_members m ON (m.member = r.oid) JOIN pg_roles r1 ON (m.roleid=r1.oid) WHERE r.rolcanlogin AND r.rolname='%s'`
	GetRoleAttributesSQLTemplate = `select rolconnlimit, rolreplication, rolbypassrls FROM pg_roles WHERE rolname = '%s'`
	// DO NOT TOUCH THIS
	// Cannot filter on compute value so... cf line before.
	GetRoleSettingsSQLTemplate           = `SELECT pg_catalog.split_part(pg_catalog.unnest(setconfig), '=', 1) as parameter_type, pg_catalog.split_part(pg_catalog.unnest(setconfig), '=', 2) as parameter_value, d.datname as database FROM pg_catalog.pg_roles r JOIN pg_catalog.pg_db_role_setting c ON (c.setrole = r.oid) JOIN pg_catalog.pg_database d ON (d.oid = c.setdatabase) WHERE r.rolcanlogin AND r.rolname='%s'` //nolint:lll//Because
	DoesRoleHaveActiveSessionSQLTemplate = `SELECT 1 from pg_stat_activity WHERE usename = '%s' group by usename`
	DuplicateRoleErrorCode               = "42710"
	RoleNotFoundErrorCode                = "42704"
	InvalidGrantOperationErrorCode       = "0LP01"
)

var (
	DefaultAttributeConnectionLimit = -1
	DefaultAttributeReplication     = false
	DefaultAttributeBypassRLS       = false
)

type RoleAttributes struct {
	ConnectionLimit *int
	Replication     *bool
	BypassRLS       *bool
}

func (*pg) buildAttributesString(attributes *RoleAttributes) string {
	// Check nil
	if attributes == nil {
		return ""
	}

	res := make([]string, 0)

	// Connection limit case
	if attributes.ConnectionLimit != nil {
		res = append(res, fmt.Sprintf("CONNECTION LIMIT %d", *attributes.ConnectionLimit))
	}

	// Replication case
	if attributes.Replication != nil {
		if *attributes.Replication {
			res = append(res, "REPLICATION")
		} else {
			res = append(res, "NOREPLICATION")
		}
	}

	// BypassRLS case
	if attributes.BypassRLS != nil {
		if *attributes.BypassRLS {
			res = append(res, "BYPASSRLS")
		} else {
			res = append(res, "NOBYPASSRLS")
		}
	}

	return strings.Join(res, " ")
}

func (c *pg) AlterRoleAttributes(ctx context.Context, role string, attributes *RoleAttributes) error {
	// Build attributes str
	attributesSQLStr := c.buildAttributesString(attributes)
	// Check if it is empty
	if attributesSQLStr == "" {
		return nil
	}

	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(AlterRoleWithOptionSQLTemplate, role, attributesSQLStr))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) GetRoleAttributes(ctx context.Context, role string) (*RoleAttributes, error) {
	res := &RoleAttributes{
		ConnectionLimit: new(int),
		Replication:     new(bool),
		BypassRLS:       new(bool),
	}

	err := c.connect(c.defaultDatabase)
	if err != nil {
		return res, err
	}

	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(GetRoleAttributesSQLTemplate, role))
	if err != nil {
		return res, err
	}

	defer rows.Close()

	for rows.Next() {
		// Scan
		err = rows.Scan(res.ConnectionLimit, res.Replication, res.BypassRLS)
		// Check error
		if err != nil {
			return res, err
		}
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return res, err
	}

	return res, nil
}

func (c *pg) GetRoleMembership(ctx context.Context, role string) ([]string, error) {
	res := make([]string, 0)

	err := c.connect(c.defaultDatabase)
	if err != nil {
		return res, err
	}

	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(GetRoleMembershipSQLTemplate, role))
	if err != nil {
		return res, err
	}

	defer rows.Close()

	for rows.Next() {
		member := ""
		// Scan
		err = rows.Scan(&member)
		// Check error
		if err != nil {
			return res, err
		}

		// Save member
		res = append(res, member)
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return res, err
	}

	return res, nil
}

func (c *pg) CreateGroupRole(ctx context.Context, role string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(CreateGroupRoleSQLTemplate, role))
	if err != nil {
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		// Error code 42710 is duplicate_object (role already exists)
		if ok && pqErr.Code == DuplicateRoleErrorCode {
			return nil
		}

		return err
	}

	return nil
}

func (c *pg) CreateUserRole(ctx context.Context, role, password string, attributes *RoleAttributes) (string, error) {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return "", err
	}

	// Build attributes sql
	attributesSQLStr := c.buildAttributesString(attributes)

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(CreateUserRoleSQLTemplate, role, password, attributesSQLStr))
	if err != nil {
		return "", err
	}

	return role, nil
}

func (c *pg) GrantRole(ctx context.Context, role, grantee string, withAdminOption bool) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	// Select right SQL template
	tpl := GrantRoleSQLTemplate
	if withAdminOption {
		tpl = GrantRoleWithAdminOptionSQLTemplate
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(tpl, role, grantee))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) AlterDefaultLoginRole(ctx context.Context, role, setRole string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(AlterUserSetRoleSQLTemplate, role, setRole))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) AlterDefaultLoginRoleOnDatabase(ctx context.Context, role, setRole, database string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(AlterUserSetRoleOnDatabaseSQLTemplate, role, database, setRole))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) RevokeUserSetRoleOnDatabase(ctx context.Context, role, database string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(RevokeUserSetRoleOnDatabaseSQLTemplate, role, database))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) GetSetRoleOnDatabasesRoleSettings(ctx context.Context, role string) ([]*SetRoleOnDatabaseRoleSetting, error) {
	// Prepare result
	res := make([]*SetRoleOnDatabaseRoleSetting, 0)

	err := c.connect(c.defaultDatabase)
	if err != nil {
		return res, err
	}

	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(GetRoleSettingsSQLTemplate, role))
	if err != nil {
		return res, err
	}

	defer rows.Close()

	for rows.Next() {
		parameterType := ""
		parameterValue := ""
		database := ""
		// Scan
		err = rows.Scan(&parameterType, &parameterValue, &database)
		// Check error
		if err != nil {
			return res, err
		}

		// Check parameter type
		if parameterType != "role" {
			// Ignore
			continue
		}

		// Save member
		res = append(res, &SetRoleOnDatabaseRoleSetting{
			Role:     parameterValue,
			Database: database,
		})
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return res, err
	}

	return res, nil
}

func (c *pg) RevokeRole(ctx context.Context, role, revoked string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(RevokeRoleSQLTemplate, role, revoked))
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil {
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != RoleNotFoundErrorCode {
			return err
		}
	}

	return nil
}

func (c *pg) ChangeAndDropOwnedBy(ctx context.Context, role, newOwner, database string) error {
	// REASSIGN OWNED BY only works if the correct database is selected
	err := c.connect(database)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(ReassignObjectsSQLTemplate, role, newOwner))
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil {
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != RoleNotFoundErrorCode {
			return err
		}
	}

	// We previously assigned all objects to the operator's role so DROP OWNED BY will drop privileges of role
	_, err = c.db.ExecContext(ctx, fmt.Sprintf(DropOwnedBySQLTemplate, role))
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil {
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != RoleNotFoundErrorCode {
			return err
		}
	}

	// Default
	return nil
}

func (c *pg) DropRole(ctx context.Context, role string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(DropRoleSQLTemplate, role))
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil {
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != RoleNotFoundErrorCode {
			return err
		}
	}

	return nil
}

func (c *pg) DropRoleAndDropAndChangeOwnedBy(ctx context.Context, role, newOwner, database string) error {
	err := c.ChangeAndDropOwnedBy(ctx, role, newOwner, database)
	if err != nil {
		return err
	}

	return c.DropRole(ctx, role)
}

func (c *pg) UpdatePassword(ctx context.Context, role, password string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(UpdatePasswordSQLTemplate, role, password))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) IsRoleExist(ctx context.Context, role string) (bool, error) {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return false, err
	}

	res, err := c.db.ExecContext(ctx, fmt.Sprintf(IsRoleExistSQLTemplate, role))
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

func (c *pg) DoesRoleHaveActiveSession(ctx context.Context, role string) (bool, error) {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return false, err
	}

	res, err := c.db.ExecContext(ctx, fmt.Sprintf(DoesRoleHaveActiveSessionSQLTemplate, role))
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

func (c *pg) RenameRole(ctx context.Context, oldname, newname string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(RenameRoleSQLTemplate, oldname, newname))
	if err != nil {
		return err
	}

	return nil
}
