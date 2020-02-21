package postgres

import (
	"fmt"

	"github.com/lib/pq"
)

const (
	CreateGroupRoleSQLTemplate     = `CREATE ROLE "%s"`
	CreateUserRoleSQLTemplate      = `CREATE ROLE "%s" WITH LOGIN PASSWORD '%s'`
	GrantRoleSQLTemplate           = `GRANT "%s" TO "%s"`
	AlterUserSetRoleSQLTemplate    = `ALTER USER "%s" SET ROLE "%s"`
	RevokeRoleSQLTemplate          = `REVOKE "%s" FROM "%s"`
	UpdatePasswordSQLTemplate      = `ALTER ROLE "%s" WITH PASSWORD '%s'` // #nosec
	DropRoleSQLTemplate            = `DROP ROLE "%s"`
	DropOwnedBySQLTemplate         = `DROP OWNED BY "%s"`
	ReassignObjectsSQLTemplate     = `REASSIGN OWNED BY "%s" TO "%s"`
	IsRoleExistSQLTemplate         = `SELECT 1 FROM pg_roles WHERE rolname='%s'`
	RenameRoleSQLTemplate          = `ALTER ROLE "%s" RENAME TO "%s"`
	DuplicateRoleErrorCode         = "42710"
	RoleNotFoundErrorCode          = "42704"
	InvalidGrantOperationErrorCode = "0LP01"
)

func (c *pg) CreateGroupRole(role string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}
	defer c.close()
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
	defer c.close()
	_, err = c.db.Exec(fmt.Sprintf(CreateUserRoleSQLTemplate, role, password))
	if err != nil {
		return "", err
	}
	return role, nil
}

func (c *pg) GrantRole(role, grantee string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}
	defer c.close()
	_, err = c.db.Exec(fmt.Sprintf(GrantRoleSQLTemplate, role, grantee))
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
	defer c.close()
	_, err = c.db.Exec(fmt.Sprintf(AlterUserSetRoleSQLTemplate, role, setRole))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) RevokeRole(role, revoked string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}
	defer c.close()
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

func (c *pg) DropRole(role, newOwner, database string) error {
	// REASSIGN OWNED BY only works if the correct database is selected
	err := c.connect(database)
	if err != nil {
		return err
	}
	defer c.close()

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

	// Close now and connect to default database
	err = c.close()
	if err != nil {
		return err
	}

	err = c.connect(c.defaultDatabase)
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

func (c *pg) UpdatePassword(role, password string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}
	defer c.close()
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
	defer c.close()
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

func (c *pg) RenameRole(oldname, newname string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}
	defer c.close()
	_, err = c.db.Exec(fmt.Sprintf(RenameRoleSQLTemplate, oldname, newname))
	if err != nil {
		return err
	}

	return nil
}
