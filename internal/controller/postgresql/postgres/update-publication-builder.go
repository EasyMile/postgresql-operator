package postgres

import (
	"fmt"
	"strings"
)

type UpdatePublicationBuilder struct {
	newName    string
	withPart   string
	tablesPart string
	tables     []string
	schemaList []string
}

func NewUpdatePublicationBuilder() *UpdatePublicationBuilder {
	return &UpdatePublicationBuilder{}
}

func (b *UpdatePublicationBuilder) Build() {
	// Build
	var res string

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

func (b *UpdatePublicationBuilder) AddSetTable(name string, columns *[]string, additionalWhere *string) *UpdatePublicationBuilder {
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

func (b *UpdatePublicationBuilder) SetTablesInSchema(schemaList []string) *UpdatePublicationBuilder {
	b.schemaList = schemaList

	return b
}

func (b *UpdatePublicationBuilder) RenameTo(newName string) *UpdatePublicationBuilder {
	b.newName = newName

	return b
}

func (b *UpdatePublicationBuilder) SetWith(publish string, publishViaPartitionRoot *bool) *UpdatePublicationBuilder {
	var with string
	// Check if publish is set
	if publish != "" {
		with += "publish = '" + publish + "'"
	} else {
		// Set default for reconcile cases
		with += "publish = 'insert, update, delete, truncate'"
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
	} else {
		// Check if there is already a with set
		if with != "" {
			with += ", "
		}
		// Set default for reconcile cases
		with += "publish_via_partition_root = false"
	}

	// Save
	b.withPart = fmt.Sprintf(" (%s)", with)

	return b
}

func (b *UpdatePublicationBuilder) SetDefaultWith() *UpdatePublicationBuilder {
	fV := false
	// Call other method without parameters to inject default values
	b.SetWith("", &fV)

	return b
}
