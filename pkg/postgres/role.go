package postgres

import (
	"fmt"

	"github.com/lib/pq"
)

const (
	CREATE_GROUP_ROLE   = `CREATE ROLE "%s"`
	CREATE_USER_ROLE    = `CREATE ROLE "%s" WITH LOGIN PASSWORD '%s'`
	GRANT_ROLE          = `GRANT "%s" TO "%s"`
	ALTER_USER_SET_ROLE = `ALTER USER "%s" SET ROLE "%s"`
	REVOKE_ROLE         = `REVOKE "%s" FROM "%s"`
	UPDATE_PASSWORD     = `ALTER ROLE "%s" WITH PASSWORD '%s'`
	DROP_ROLE           = `DROP ROLE "%s"`
	DROP_OWNED_BY       = `DROP OWNED BY "%s"`
	REASIGN_OBJECTS     = `REASSIGN OWNED BY "%s" TO "%s"`
)

func (c *pg) CreateGroupRole(role string) error {
	err := c.connect(c.default_database)
	if err != nil {
		return err
	}
	defer c.close()
	_, err = c.db.Exec(fmt.Sprintf(CREATE_GROUP_ROLE, role))
	if err != nil {
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		// Error code 42710 is duplicate_object (role already exists)
		if ok && pqErr.Code == "42710" {
			return nil
		}
		return err
	}
	return nil
}

func (c *pg) CreateUserRole(role, password string) (string, error) {
	err := c.connect(c.default_database)
	if err != nil {
		return "", err
	}
	defer c.close()
	_, err = c.db.Exec(fmt.Sprintf(CREATE_USER_ROLE, role, password))
	if err != nil {
		return "", err
	}
	return role, nil
}

func (c *pg) GrantRole(role, grantee string) error {
	err := c.connect(c.default_database)
	if err != nil {
		return err
	}
	defer c.close()
	_, err = c.db.Exec(fmt.Sprintf(GRANT_ROLE, role, grantee))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) AlterDefaultLoginRole(role, setRole string) error {
	err := c.connect(c.default_database)
	if err != nil {
		return err
	}
	defer c.close()
	_, err = c.db.Exec(fmt.Sprintf(ALTER_USER_SET_ROLE, role, setRole))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) RevokeRole(role, revoked string) error {
	err := c.connect(c.default_database)
	if err != nil {
		return err
	}
	defer c.close()
	_, err = c.db.Exec(fmt.Sprintf(REVOKE_ROLE, role, revoked))
	if err != nil {
		return err
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

	_, err = c.db.Exec(fmt.Sprintf(REASIGN_OBJECTS, role, newOwner))
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil {
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != "42704" {
			return err
		}
	}

	// We previously assigned all objects to the operator's role so DROP OWNED BY will drop privileges of role
	_, err = c.db.Exec(fmt.Sprintf(DROP_OWNED_BY, role))
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil {
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != "42704" {
			return err
		}
	}

	// Close now and connect to default database
	err = c.close()
	if err != nil {
		return err
	}

	err = c.connect(c.default_database)
	if err != nil {
		return err
	}
	_, err = c.db.Exec(fmt.Sprintf(DROP_ROLE, role))
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil {
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != "42704" {
			return err
		}
	}
	return nil
}

func (c *pg) UpdatePassword(role, password string) error {
	err := c.connect(c.default_database)
	if err != nil {
		return err
	}
	defer c.close()
	_, err = c.db.Exec(fmt.Sprintf(UPDATE_PASSWORD, role, password))
	if err != nil {
		return err
	}

	return nil
}
