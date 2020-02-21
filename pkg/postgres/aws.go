package postgres

import (
	"fmt"

	"github.com/lib/pq"
)

type awspg struct {
	pg
}

func newAWSPG(postgres *pg) PG {
	return &awspg{
		*postgres,
	}
}

func (c *awspg) AlterDefaultLoginRole(role, setRole string) error {
	// On AWS RDS the postgres user isn't really superuser so he doesn't have permissions
	// to ALTER USER unless he belongs to both roles
	err := c.GrantRole(role, c.user)
	if err != nil {
		return err
	}
	defer func() {
		err := c.RevokeRole(role, c.user)
		c.log.Error(err, "error in revoke role")
	}()

	return c.pg.AlterDefaultLoginRole(role, setRole)
}

func (c *awspg) DropRole(role, newOwner, database string) error {
	// On AWS RDS the postgres user isn't really superuser so he doesn't have permissions
	// to REASSIGN OWNED BY unless he belongs to both roles
	err := c.GrantRole(role, c.user)
	if err != nil {
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok {
			return err
		}
		if pqErr.Code == RoleNotFoundErrorCode {
			return nil
		}
		if pqErr.Code != InvalidGrantOperationErrorCode {
			return err
		}
	}
	err = c.GrantRole(newOwner, c.user)
	if err != nil {
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok {
			return err
		}
		if pqErr.Code == RoleNotFoundErrorCode {
			// The group role does not exist, no point of granting roles
			c.log.Info(fmt.Sprintf("not granting %s to %s as %s does not exist", role, newOwner, newOwner))
			return nil
		}
		if pqErr.Code != InvalidGrantOperationErrorCode {
			return err
		}
	}
	defer func() {
		err := c.RevokeRole(newOwner, c.user)
		c.log.Error(err, "error in revoke role")
	}()

	return c.pg.DropRole(role, newOwner, database)
}
