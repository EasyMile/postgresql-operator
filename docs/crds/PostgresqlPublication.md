# PostgresqlPublication

## Description

This Custom Resource represents a PosgreSQL Publication.

This will create and manage PostgreSQL Publication. See here: https://www.postgresql.org/docs/current/sql-createpublication.html

## Custom Resource Definition

### kubectl names and short names

All these names are available for `kubectl`:

- postgresqlpublications.postgresql.easymile.com
- postgresqlpublications
- postgresqlpublication
- pgpub

### Root fields

| Field    | Description                                                                                                                                                                                                                                                                                             | Scheme                                                                                                       | Required |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ | -------- |
| metadata | Object metadata                                                                                                                                                                                                                                                                                         | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#objectmeta-v1-meta) | false    |
| spec     | Specification of the PostgreSQL Database                                                                                                                                                                                                                                                                | [PostgresqlPublicationSpec](#postgresqlpublicationspec)                                                      | true     |
| status   | Most recent observed status of the PostgreSQL Database. Read-only. Not included when requesting from the apiserver, only from the PostgreSQL Operator API itself. More info: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status | [PostgresqlPublicationStatus](#postgresqlpublicationstatus)                                                  | false    |

### PostgresqlPublicationSpec

| Field          | Description                                                                                    | Scheme                                                      | Required |
| -------------- | ---------------------------------------------------------------------------------------------- | ----------------------------------------------------------- | -------- |
| database       | PostgreSQL Database reference.                                                                 | [CRLink](#crlink)                                           | true     |
| name           | Publication name in PostgreSQL                                                                 | String                                                      | true     |
| dropOnDelete   | Should drop publication on current Custom Resource deletion ? Default is false                 | Boolean                                                     | false    |
| allTables      | Publication for all tables. Note: This is mutually exclusive with "tablesInSchema" & "tables". | Boolean                                                     | false    |
| tablesInSchema | Publication all tables in specific schema list. Note: This is a list of schema                 | []String                                                    | false    |
| tables         | Publication for selected tables                                                                | [][PostgresqlPublicationTable](#postgresqlpublicationtable) | false    |
| withParameters | Publication parameters                                                                         | [PostgresqlPublicationWith](#postgresqlpublicationwith)     | false    |

### PostgresqlPublicationTable

| Field           | Description                                                                 | Scheme   | Required |
| --------------- | --------------------------------------------------------------------------- | -------- | -------- |
| tableName       | Table name on which publication should be created                           | String   | true     |
| columns         | Columns to select for the publication (Empty array will select all columns) | []String | false    |
| additionalWhere | WHERE clause for the publication on selected table                          | String   | false    |

### PostgresqlPublicationWith

| Field                   | Description                                                                                                                              | Scheme  | Required |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| publish                 | Publish options (See here: https://www.postgresql.org/docs/current/sql-createpublication.html#SQL-CREATEPUBLICATION-PARAMS-WITH-PUBLISH) | String  | false    |
| publishViaPartitionRoot | Publish options (See here: https://www.postgresql.org/docs/current/sql-createpublication.html#SQL-CREATEPUBLICATION-PARAMS-WITH-PUBLISH) | Boolean | false    |

### CRLink

| Field     | Description                                                                         | Scheme | Required |
| --------- | ----------------------------------------------------------------------------------- | ------ | -------- |
| name      | Custom resource name                                                                | String | true     |
| namespace | Custom resource namespace. Default value will be current custom resource namespace. | String | false    |

### PostgresqlPublicationStatus

| Field     | Description                                                                     | Scheme    | Required |
| --------- | ------------------------------------------------------------------------------- | --------- | -------- |
| phase     | Current phase of the operator                                                   | String    | true     |
| message   | Human-readable message indicating details about current operator phase or error | String    | false    |
| ready     | True if all resources are in a ready state and all work is done by operator     | Boolean   | false    |
| name      | Publication created name                                                        | String    | false    |
| allTables | Flag to save if publication was created for all tables                          | \*Boolean | false    |
| hash      | Resource spec hash for internal needs                                           | String    | false    |

## Example

Here is an example of Custom Resource:

```yaml
apiVersion: postgresql.easymile.com/v1alpha1
kind: PostgresqlPublication
metadata:
  name: full
spec:
  # Database custom resource reference
  database:
    name: postgresqldatabase-sample
  # Publication name in PostgreSQL
  name: my-publication
  # Drop on delete
  dropOnDelete: false
  # Enable publication for all tables in database
  allTables: false
  # Tables in schema to select for publication
  tablesInSchema:
    []
    # - table1
    # - table2
  # Table selection for publication
  tables:
    - # Table name
      tableName: table1
      # Columns to select (Empty array will select all of them)
      columns:
        - id
        - created_at
        # - updated_at
        - number1
      # WHERE clause on selected table
      additionalWhere: number1 > 5
  # Publication with parameters
  withParameters:
    # Publish param
    # See here: https://www.postgresql.org/docs/current/sql-createpublication.html#SQL-CREATEPUBLICATION-PARAMS-WITH-PUBLISH
    # publish: 'TRUNCATE'
    # Publish via partition root param
    # See here: https://www.postgresql.org/docs/current/sql-createpublication.html#SQL-CREATEPUBLICATION-PARAMS-WITH-PUBLISH
    publishViaPartitionRoot: false
```
