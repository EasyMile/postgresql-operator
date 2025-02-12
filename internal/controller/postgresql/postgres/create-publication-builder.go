package postgres

import (
	"fmt"
	"strings"
)

type CreatePublicationBuilder struct {
	name       string
	tablesPart string
	allTables  string
	withPart   string
	owner      string
	tables     []string
	schemaList []string
}

func NewCreatePublicationBuilder() *CreatePublicationBuilder {
	return &CreatePublicationBuilder{}
}

func (b *CreatePublicationBuilder) Build() {
	if b.allTables != "" {
		b.tablesPart = b.allTables

		return
	}

	// Build
	res := "FOR "

	// Check if tables are set
	if len(b.tables) != 0 {
		res += "TABLE " + strings.Join(b.tables, ", ")
	}

	// Check if schema are set
	if len(b.schemaList) != 0 {
		// Check if tables were added
		if len(b.tables) != 0 {
			// Append
			res += ", "
		}

		res += "TABLES IN SCHEMA " + strings.Join(b.schemaList, ", ")
	}

	// Save
	b.tablesPart = res
}

func (b *CreatePublicationBuilder) AddTable(name string, columns *[]string, additionalWhere *string) *CreatePublicationBuilder {
	res := name

	// Manage columns
	if columns != nil {
		res += " (" + strings.Join(*columns, ", ") + ")"
	}

	// Add where is set
	if additionalWhere != nil {
		res += " WHERE (" + *additionalWhere + ")"
	}

	// Save
	b.tables = append(b.tables, res)

	return b
}

func (b *CreatePublicationBuilder) SetTablesInSchema(schemaList []string) *CreatePublicationBuilder {
	b.schemaList = schemaList

	return b
}

func (b *CreatePublicationBuilder) SetForAllTables() *CreatePublicationBuilder {
	b.allTables = "FOR ALL TABLES"

	return b
}

func (b *CreatePublicationBuilder) SetOwner(n string) *CreatePublicationBuilder {
	b.owner = n

	return b
}

func (b *CreatePublicationBuilder) SetName(n string) *CreatePublicationBuilder {
	b.name = n

	return b
}

func (b *CreatePublicationBuilder) SetWith(publish string, publishViaPartitionRoot *bool) *CreatePublicationBuilder {
	var with string
	// Check if publish is set
	if publish != "" {
		with += "publish = '" + publish + "'"
	}
	// Check publish via partition root
	if publishViaPartitionRoot != nil {
		// Check if there is already a with set
		if with != "" {
			with += ", "
		}
		// Manage bool
		with += "publish_via_partition_root = "
		if *publishViaPartitionRoot {
			with += "true"
		} else {
			with += "false"
		}
	}

	// Save
	b.withPart = fmt.Sprintf("WITH (%s)", with)

	return b
}
