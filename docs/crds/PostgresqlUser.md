# PostgresqlUser

## Description

This Custom Resource represents a PosgreSQL User.

## Custom Resource Definition

### kubectl names and short names

All these names are available for `kubectl`:
- postgresqlusers.postgresql.easymile.com
- postgresqlusers
- postgresqluser
- pgu

### Root fields

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata | Object metadata | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#objectmeta-v1-meta) | false |
| spec | Specification of the PostgreSQL User | [PostgresqlUserSpec](#postgresqluserspec) | true |
| status | Most recent observed status of the PostgreSQL User. Read-only. Not included when requesting from the apiserver, only from the PostgreSQL Operator API itself. More info: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status | [PostgresqlUserStatus](#postgresqluserstatus) | false |

### PostgresqlUserSpec

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| rolePrefix | User role prefix | String | true |
| database | PostgreSQL Database reference | [CRLink](#crlink) | true |
| generatedSecretNamePrefix | Generated secret name prefix used for secret generation. The generated name will be `${PREFIX}-${CURRENT_CR_NAME}`. | String | true |
| privileges | User privileges on database. Enumeration is `OWNER`, `WRITER`, `READER`. | String | true |
| userPasswordRotationDuration | User password rotation interval between 2 changes. Default is empty. | String | false |

### CRLink

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Custom resource name | String | true |
| namespace | Custom resource namespace. Default value will be current custom resource namespace. | String | false |

### PostgresqlUserStatus

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| phase | Current phase of the operator | String | true |
| message | Human-readable message indicating details about current operator phase or error | String | false |
| ready | True if all resources are in a ready state and all work is done by operator | Boolean | false |
| rolePrefix | User role prefix actually used | String | false |
| postgresRole | PostgreSQL role for user | String | false |
| postgresLogin | PostgreSQL login for user | String | false |
| postgresGroup | PostgreSQL group for user | String | false |
| postgresDatabaseName | PostgreSQL database name for which user is created | String | false |
| lastPasswordChangedTime | Last time operator has changed the user password | String | false |

## Example

Here is an example of Custom Resource:

```yaml
apiVersion: postgresql.easymile.com/v1alpha1
kind: PostgresqlUser
metadata:
  name: full
spec:
  # Database link
  database:
    # Resource name
    name: simple
    # Resource namespace
    # Will use resource namespace if not set
    # namespace:
  # Generated Secret name prefix
  generatedSecretNamePrefix: secret1
  # User role prefix
  rolePrefix: pguser1
  # Privileges for user role
  privileges: OWNER
  # User password rotation duration
  # Default set to ""
  userPasswordRotationDuration: 720h
```
