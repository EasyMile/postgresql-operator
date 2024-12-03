package postgres

import (
	"errors"
	"fmt"

	"github.com/lib/pq"
)

const (
	CreatePublicationSQLTemplate                = `CREATE PUBLICATION "%s" %s %s`
	DropPublicationSQLTemplate                  = `DROP PUBLICATION "%s"`
	AlterPublicationRenameSQLTemplate           = `ALTER PUBLICATION "%s" RENAME TO "%s"`
	AlterPublicationGeneralOperationSQLTemplate = `ALTER PUBLICATION "%s" SET %s`
	GetPublicationSQLTemplate                   = `SELECT
  puballtables, pubinsert, pubupdate, pubdelete, pubtruncate, pubviaroot
FROM pg_catalog.pg_publication
WHERE pubname = '%s';`
	GetReplicationSlotSQLTemplate    = `SELECT slot_name,plugin,database FROM pg_replication_slots WHERE slot_name = '%s'`
	CreateReplicationSlotSQLTemplate = `SELECT pg_create_logical_replication_slot('%s', '%s')`
	DropReplicationSlotSQLTemplate   = `SELECT pg_drop_replication_slot('%s')`
)

type PublicationResult struct {
	AllTables          bool
	Insert             bool
	Update             bool
	Delete             bool
	Truncate           bool
	PublicationViaRoot bool
}

type PublicationTableDetail struct {
	SchemaName      string
	TableName       string
	AdditionalWhere *string
	Columns         []string
}

type ReplicationSlotResult struct {
	SlotName string
	Plugin   string
	Database string
}

func (c *pg) DropReplicationSlot(name string) error {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(DropReplicationSlotSQLTemplate, name))
	if err != nil {
		return err
	}

	// Default
	return nil
}

func (c *pg) CreateReplicationSlot(dbname, name, plugin string) error {
	err := c.connect(dbname)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(CreateReplicationSlotSQLTemplate, name, plugin))
	if err != nil {
		return err
	}

	// Default
	return nil
}

func (c *pg) GetReplicationSlot(name string) (*ReplicationSlotResult, error) {
	err := c.connect(c.defaultDatabase)
	if err != nil {
		return nil, err
	}

	// Get rows
	rows, err := c.db.Query(fmt.Sprintf(GetReplicationSlotSQLTemplate, name))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var res ReplicationSlotResult

	var foundOne bool

	for rows.Next() {
		// Scan
		err = rows.Scan(&res.SlotName, &res.Plugin, &res.Database)
		// Check error
		if err != nil {
			return nil, err
		}

		// Update marker
		foundOne = true
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return nil, err
	}

	// Check if found marker isn't set
	if !foundOne {
		return nil, nil
	}

	return &res, nil
}

func (c *pg) UpdatePublication(dbname, publicationName string, builder *UpdatePublicationBuilder) (err error) {
	// Connect to db
	err = c.connect(dbname)
	if err != nil {
		return err
	}

	// Build
	builder.Build()

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			err2 := tx.Rollback()

			err = errors.Join(err, err2)
		}
	}()

	// Manage with options
	if builder.withPart != "" {
		_, err = tx.Exec(fmt.Sprintf(AlterPublicationGeneralOperationSQLTemplate, publicationName, builder.withPart))
		if err != nil {
			return err
		}
	}

	// Manage tables
	if builder.tablesPart != "" {
		_, err = tx.Exec(fmt.Sprintf(AlterPublicationGeneralOperationSQLTemplate, publicationName, builder.tablesPart))
		if err != nil {
			return err
		}
	}

	// Check rename
	// ? Note: this should be the last step
	if builder.newName != "" {
		// Rename have to be done
		_, err = tx.Exec(fmt.Sprintf(AlterPublicationRenameSQLTemplate, publicationName, builder.newName))
		if err != nil {
			return err
		}
	}

	// Commit
	err = tx.Commit()
	if err != nil {
		return err
	}

	// Default
	return nil
}

func (c *pg) CreatePublication(dbname string, builder *CreatePublicationBuilder) error {
	// Connect to db
	err := c.connect(dbname)
	if err != nil {
		return err
	}

	// Build
	builder.Build()

	_, err = c.db.Exec(fmt.Sprintf(CreatePublicationSQLTemplate, builder.name, builder.tablesPart, builder.withPart))
	if err != nil {
		return err
	}

	// Default
	return nil
}

func (c *pg) GetPublication(dbname, name string) (*PublicationResult, error) {
	err := c.connect(dbname)
	if err != nil {
		return nil, err
	}

	// Get rows
	rows, err := c.db.Query(fmt.Sprintf(GetPublicationSQLTemplate, name))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var res PublicationResult

	var foundOne bool

	for rows.Next() {
		// Scan
		err = rows.Scan(&res.AllTables, &res.Insert, &res.Update, &res.Delete, &res.Truncate, &res.PublicationViaRoot)
		// Check error
		if err != nil {
			return nil, err
		}

		// Update marker
		foundOne = true
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return nil, err
	}

	// Check if found marker isn't set
	if !foundOne {
		return nil, nil
	}

	return &res, nil
}

func (c *pg) DropPublication(dbname, name string) error {
	err := c.connect(dbname)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(DropPublicationSQLTemplate, name))
	// Error code 3D000 is returned if database doesn't exist
	if err != nil {
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != "3D000" {
			return err
		}
	}

	return nil
}

func (c *pg) RenamePublication(dbname, oldname, newname string) error {
	err := c.connect(dbname)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(AlterPublicationRenameSQLTemplate, oldname, newname))
	if err != nil {
		return err
	}

	return nil
}
