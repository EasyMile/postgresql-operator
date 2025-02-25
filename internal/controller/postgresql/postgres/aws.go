package postgres

import (
	"context"
	"fmt"

	"github.com/lib/pq"
)

const (
	CreateDBWithoutOwnerSQLTemplate = `CREATE DATABASE "%s"`
	AlterDBOwnerSQLTemplate         = `ALTER DATABASE "%s" OWNER TO "%s"`
)

type awspg struct {
	pg
}

func newAWSPG(postgres *pg) PG {
	return &awspg{
		*postgres,
	}
}

func (c *awspg) AlterDefaultLoginRole(ctx context.Context, role, setRole string) error {
	// On AWS RDS the postgres user isn't really superuser so he doesn't have permissions
	// to ALTER USER unless he belongs to both roles
	err := c.GrantRole(ctx, role, c.user, false)
	if err != nil {
		return err
	}

	defer func() {
		err := c.RevokeRole(ctx, role, c.user)
		// Check error
		if err != nil {
			c.log.Error(err, "error in revoke role")
		}
	}()

	return c.pg.AlterDefaultLoginRole(ctx, role, setRole)
}

func (c *awspg) CreateDB(ctx context.Context, dbname, role string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(CreateDBWithoutOwnerSQLTemplate, dbname))
	if err != nil {
		// eat DUPLICATE DATABASE ERROR
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != DuplicateDatabaseErrorCode {
			return err
		}
	}

	_, err = c.db.ExecContext(ctx, fmt.Sprintf(AlterDBOwnerSQLTemplate, dbname, role))
	if err != nil {
		return err
	}

	return nil
}

func (c *awspg) DropRoleAndDropAndChangeOwnedBy(ctx context.Context, role, newOwner, database string) error {
	// On AWS RDS the postgres user isn't really superuser so he doesn't have permissions
	// to REASSIGN OWNED BY unless he belongs to both roles
	err := c.GrantRole(ctx, role, c.user, false)
	// Check error
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

	err = c.GrantRole(ctx, newOwner, c.user, false)
	// Check error
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
		err := c.RevokeRole(ctx, newOwner, c.user)
		if err != nil {
			c.log.Error(err, "error in revoke role")
		}
	}()

	return c.pg.DropRoleAndDropAndChangeOwnedBy(ctx, role, newOwner, database)
}

func (c *awspg) ChangeAndDropOwnedBy(ctx context.Context, role, newOwner, database string) error {
	// On AWS RDS the postgres user isn't really superuser so he doesn't have permissions
	// to REASSIGN OWNED BY unless he belongs to both roles
	err := c.GrantRole(ctx, role, c.user, false)
	// Check error
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

	err = c.GrantRole(ctx, newOwner, c.user, false)
	// Check error
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
		err := c.RevokeRole(ctx, newOwner, c.user)
		if err != nil {
			c.log.Error(err, "error in revoke role")
		}
	}()

	return c.pg.ChangeAndDropOwnedBy(ctx, role, newOwner, database)
}
