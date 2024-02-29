# PostgresqlDatabase

## Description

This Custom Resource represents a PosgreSQL Database.

## Custom Resource Definition

### kubectl names and short names

All these names are available for `kubectl`:

- postgresqldatabases.postgresql.easymile.com
- postgresqldatabases
- postgresqldatabase
- pgdb

### Root fields

| Field    | Description                                                                                                                                                                                                                                                                                             | Scheme                                                                                                       | Required |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ | -------- |
| metadata | Object metadata                                                                                                                                                                                                                                                                                         | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#objectmeta-v1-meta) | false    |
| spec     | Specification of the PostgreSQL Database                                                                                                                                                                                                                                                                | [PostgresqlDatabaseSpec](#postgresqldatabasespec)                                                            | true     |
| status   | Most recent observed status of the PostgreSQL Database. Read-only. Not included when requesting from the apiserver, only from the PostgreSQL Operator API itself. More info: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status | [PostgresqlDatabaseStatus](#postgresqldatabasestatus)                                                        | false    |

### PostgresqlDatabaseSpec

| Field                       | Description                                                                                                                                                                                  | Scheme                                    | Required |
| --------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------- | -------- |
| database                    | Database name                                                                                                                                                                                | String                                    | true     |
| masterRole                  | Master role name will be used to create owner group role. Users with "owner" privilege will be put in this group role. Default is empty.                                                     | String                                    |
| dropOnDelete                | Should drop database on current Custom Resource deletion ? Default is false                                                                                                                  | Boolean                                   | false    |
| waitLinkedResourcesDeletion | Tell operator if it has to wait until all linked resources are deleted to delete current custom resource. If not, it won't be able to delete PostgresqlUser after. Default value is `false`. | Boolean                                   | false    |
| schemas                     | List of schemas to create/update. Default is empty.                                                                                                                                          | [DatabaseModuleList](#databasemodulelist) | false    |
| extensions                  | List of extensions to create/update. Default is empty.                                                                                                                                       | [DatabaseModuleList](#databasemodulelist) | false    |
| engineConfiguration         | PostgreSQL Engine Configuration reference.                                                                                                                                                   | [CRLink](#crlink)                         | true     |

### DatabaseModuleList

| Field             | Description                                                                | Scheme   | Required |
| ----------------- | -------------------------------------------------------------------------- | -------- | -------- |
| list              | Modules list. Default is empty.                                            | []String | false    |
| dropOnDelete      | Should drop module on list removal ? Default is false.                     | Boolean  | false    |
| deleteWithCascade | Should delete with cascade ? (Linked to `dropOnDelete`). Default is false. | Boolean  | false    |

### CRLink

| Field     | Description                                                                         | Scheme | Required |
| --------- | ----------------------------------------------------------------------------------- | ------ | -------- |
| name      | Custom resource name                                                                | String | true     |
| namespace | Custom resource namespace. Default value will be current custom resource namespace. | String | false    |

### PostgresqlDatabaseStatus

| Field      | Description                                                                     | Scheme                                      | Required |
| ---------- | ------------------------------------------------------------------------------- | ------------------------------------------- | -------- |
| phase      | Current phase of the operator                                                   | String                                      | true     |
| message    | Human-readable message indicating details about current operator phase or error | String                                      | false    |
| ready      | True if all resources are in a ready state and all work is done by operator     | Boolean                                     | false    |
| database   | Database created name                                                           | String                                      | false    |
| roles      | Already created group roles for database                                        | [StatusPostgresRoles](#statuspostgresroles) | false    |
| schemas    | Already created schemas                                                         | []String                                    | false    |
| extensions | Already created extensions                                                      | []String                                    | false    |

### StatusPostgresRoles

| Field  | Description  | Scheme | Required |
| ------ | ------------ | ------ | -------- |
| owner  | Owner group  | String | false    |
| reader | Reader group | String | false    |
| writer | Writer group | String | false    |

## Example

Here is an example of Custom Resource:

```yaml
apiVersion: postgresql.easymile.com/v1alpha1
kind: PostgresqlDatabase
metadata:
  name: full
spec:
  # Engine configuration link
  engineConfiguration:
    # Resource name
    name: simple
    # Resource namespace
    # Will use resource namespace if not set
    # namespace:
  # Database name
  database: databasename
  # Master role name
  # Master role name will be used to create top group role.
  # Database owner and users will be in this group role.
  # Default is ""
  masterRole: ""
  # Should drop on delete ?
  # Default set to false
  dropOnDelete: true
  # Wait for linked resource deletion to accept deletion of the current resource
  # See documentation for more information
  # Default set to false
  waitLinkedResourcesDeletion: true
  # Schemas
  schemas:
    # List of schemas to enable
    list:
      - schema1
    # Should drop on delete ?
    # Default set to false
    # If set to false, removing from list won't delete schema from database
    dropOnDelete: true
    # Delete schema with cascade
    # Default set to false
    # For all elements that have used the deleted schema
    deleteWithCascade: true
  # Extensions
  extensions:
    # List of extensions to enable
    list:
      - uuid-ossp
    # Should drop on delete ?
    # Default set to false
    # If set to false, removing from list won't delete extension from database
    dropOnDelete: true
    # Delete extension with cascade
    # Default set to false
    # For all elements that have used the deleted extension
    deleteWithCascade: true
```
