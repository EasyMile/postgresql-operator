package postgres

import (
	"fmt"

	"github.com/lib/pq"
)

const (
	CreateGroupRoleSQLTemplate             = `CREATE ROLE "%s"`
	CreateUserRoleSQLTemplate              = `CREATE ROLE "%s" WITH LOGIN PASSWORD '%s'`
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
	// Source: https://dba.stackexchange.com/questions/136858/postgresql-display-role-members
	GetRoleMembershipSQLTemplate = `SELECT r1.rolname as "role" FROM pg_catalog.pg_roles r JOIN pg_catalog.pg_auth_members m ON (m.member = r.oid) JOIN pg_roles r1 ON (m.roleid=r1.oid) WHERE r.rolcanlogin AND r.rolname='%s'`
	// DO NOT TOUCH THIS
	// Cannot filter on compute value so... cf line before.
	GetRoleSettingsSQLTemplate           = `SELECT pg_catalog.split_part(pg_catalog.unnest(setconfig), '=', 1) as parameter_type, pg_catalog.split_part(pg_catalog.unnest(setconfig), '=', 2) as parameter_value, d.datname as database FROM pg_catalog.pg_roles r JOIN pg_catalog.pg_db_role_setting c ON (c.setrole = r.oid) JOIN pg_catalog.pg_database d ON (d.oid = c.setdatabase) WHERE r.rolcanlogin AND r.rolname='%s'` //nolint:lll//Because
	DoesRoleHaveActiveSessionSQLTemplate = `SELECT 1 from pg_stat_activity WHERE usename = '%s' group by usename`
	DuplicateRoleErrorCode               = "42710"
	RoleNotFoundErrorCode                = "42704"
	InvalidGrantOperationErrorCode       = "0LP01"
)

func (c *pg) GetRoleMembership(role string) ([]string, error) {
	res := make([]string, 0)

	err := c.connect(c.defaultDatabase)
	if err != nil {
		return res, err
	}

	rows, err := c.db.Query(fmt.Sprintf(GetRoleMembershipSQLTemplate, role))
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

func (c *pg) CreateGroupRole(role string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(CreateGroupRoleSQLTemplate, role))
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

func (c *pg) CreateUserRole(role, password string) (string, error) {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return "", err
	}

	_, err = c.db.Exec(fmt.Sprintf(CreateUserRoleSQLTemplate, role, password))
	if err != nil {
		return "", err
	}

	return role, nil
}

func (c *pg) GrantRole(role, grantee string, withAdminOption bool) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	// Select right SQL template
	tpl := GrantRoleSQLTemplate
	if withAdminOption {
		tpl = GrantRoleWithAdminOptionSQLTemplate
	}

	_, err = c.db.Exec(fmt.Sprintf(tpl, role, grantee))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) AlterDefaultLoginRole(role, setRole string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(AlterUserSetRoleSQLTemplate, role, setRole))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) AlterDefaultLoginRoleOnDatabase(role, setRole, database string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(AlterUserSetRoleOnDatabaseSQLTemplate, role, database, setRole))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) RevokeUserSetRoleOnDatabase(role, database string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(RevokeUserSetRoleOnDatabaseSQLTemplate, role, database))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) GetSetRoleOnDatabasesRoleSettings(role string) ([]*SetRoleOnDatabaseRoleSetting, error) {
	// Prepare result
	res := make([]*SetRoleOnDatabaseRoleSetting, 0)

	err := c.connect(c.defaultDatabase)
	if err != nil {
		return res, err
	}

	rows, err := c.db.Query(fmt.Sprintf(GetRoleSettingsSQLTemplate, role))
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

func (c *pg) RevokeRole(role, revoked string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(RevokeRoleSQLTemplate, role, revoked))
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

func (c *pg) ChangeAndDropOwnedBy(role, newOwner, database string) error {
	// REASSIGN OWNED BY only works if the correct database is selected
	err := c.connect(database)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(ReassignObjectsSQLTemplate, role, newOwner))
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil {
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != RoleNotFoundErrorCode {
			return err
		}
	}

	// We previously assigned all objects to the operator's role so DROP OWNED BY will drop privileges of role
	_, err = c.db.Exec(fmt.Sprintf(DropOwnedBySQLTemplate, role))
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

func (c *pg) DropRole(role string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(DropRoleSQLTemplate, role))
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

func (c *pg) DropRoleAndDropAndChangeOwnedBy(role, newOwner, database string) error {
	err := c.ChangeAndDropOwnedBy(role, newOwner, database)
	if err != nil {
		return err
	}

	return c.DropRole(role)
}

func (c *pg) UpdatePassword(role, password string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(UpdatePasswordSQLTemplate, role, password))
	if err != nil {
		return err
	}

	return nil
}

func (c *pg) IsRoleExist(role string) (bool, error) {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return false, err
	}

	res, err := c.db.Exec(fmt.Sprintf(IsRoleExistSQLTemplate, role))
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

func (c *pg) DoesRoleHaveActiveSession(role string) (bool, error) {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return false, err
	}

	res, err := c.db.Exec(fmt.Sprintf(DoesRoleHaveActiveSessionSQLTemplate, role))
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

func (c *pg) RenameRole(oldname, newname string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(RenameRoleSQLTemplate, oldname, newname))
	if err != nil {
		return err
	}

	return nil
}
