# PostgresqlUserRole

## Description

This Custom Resource represents a PosgreSQL User Role.

## Custom Resource Definition

### kubectl names and short names

All these names are available for `kubectl`:

- postgresqluserroles.postgresql.easymile.com
- postgresqluserroles
- postgresqluserrole
- pgur

### Root fields

| Field    | Description                                                                                                                                                                                                                                                                                              | Scheme                                                                                                       | Required |
| -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ | -------- |
| metadata | Object metadata                                                                                                                                                                                                                                                                                          | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#objectmeta-v1-meta) | false    |
| spec     | Specification of the PostgreSQL User Role                                                                                                                                                                                                                                                                | [PostgresqlUserRoleSpec](#postgresqluserrolespec)                                                            | true     |
| status   | Most recent observed status of the PostgreSQL User Role. Read-only. Not included when requesting from the apiserver, only from the PostgreSQL Operator API itself. More info: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status | [PostgresqlUserRoleStatus](#postgresqluserrolestatus)                                                        | false    |

### PostgresqlUserRoleSpec

| Field                        | Description                                                                                                                                                                                                                                                                          | Scheme                                                        | Required                                 |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------- | ---------------------------------------- |
| mode                         | Mode for PostgresqlUserRole. One mode is `PROVIDED`: provide a username/password and operator will ensure the user provided will be injected with correct rights. The other mode is `MANAGED`, in that case, the operator will create a generated user/password with correct rights. | String                                                        | true                                     |
| privileges                   | Privileges list on databases                                                                                                                                                                                                                                                         | [][PostgresqlUserRolePrivilege](#postgresqluserroleprivilege) | true                                     |
| rolePrefix                   | Used as prefix in `MANAGED` mode for PostgreSQL Role generation                                                                                                                                                                                                                      | String                                                        | true in `MANAGED` mode, false otherwise  |
| importSecretName             | Used in `PROVIDED` mode to give username/password to operator to create and manage                                                                                                                                                                                                   | String                                                        | true in `PROVIDED` mode, false otherwise |
| userPasswordRotationDuration | User password rotation interval between 2 user/password rotation. This can be used only in `MANAGED` mode.                                                                                                                                                                           | String                                                        | false                                    |
| workGeneratedSecretName      | This is a secret used internally by operator. You can specify the name of this one, otherwise it will be generated                                                                                                                                                                   | String                                                        | false                                    |

### PostgresqlUserRolePrivilege

| Field               | Description                                                                                                                                                                               | Scheme            | Required |
| ------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------- | -------- |
| privilege           | User privilege on database. Enumeration is `OWNER`, `WRITER`, `READER`.                                                                                                                   | String            | true     |
| connectionType      | Connection type to be used for secret generation (Can be set to BOUNCER if wanted and supported by engine configuration). Enumeration is `PRIMARY`, `BOUNCER`. Default value is `PRIMARY` | String            | false    |
| database            | [PostgresqlDatabase](./PostgresqlDatabase.md) object reference                                                                                                                            | [CRLink](#crlink) | true     |
| generatedSecretName | Generated secret name used for secret generation.                                                                                                                                         | String            | true     |

### CRLink

| Field     | Description                                                                         | Scheme | Required |
| --------- | ----------------------------------------------------------------------------------- | ------ | -------- |
| name      | Custom resource name                                                                | String | true     |
| namespace | Custom resource namespace. Default value will be current custom resource namespace. | String | false    |

### PostgresqlUserRoleStatus

| Field                   | Description                                                                     | Scheme   | Required |
| ----------------------- | ------------------------------------------------------------------------------- | -------- | -------- |
| phase                   | Current phase of the operator                                                   | String   | true     |
| message                 | Human-readable message indicating details about current operator phase or error | String   | false    |
| ready                   | True if all resources are in a ready state and all work is done by operator     | Boolean  | false    |
| rolePrefix              | User role prefix currently used                                                 | String   | false    |
| postgresRole            | PostgreSQL role for user                                                        | String   | false    |
| oldPostgresRoles        | Old PostgreSQL roles that must be deleted but still in used                     | []String | false    |
| lastPasswordChangedTime | Last time operator has changed the user password                                | String   | false    |

## Example

### Provided mode

Here is an example of Custom Resource:

```yaml
apiVersion: postgresql.easymile.com/v1alpha1
kind: PostgresqlUserRole
metadata:
  name: postgresqluserrole-sample
spec:
  # Mode
  mode: PROVIDED
  # Privileges list
  privileges:
    - # Privilege for the selected database
      privilege: WRITER
      # Connection type to be used for secret generation (Can be set to BOUNCER if wanted and supported by engine configuration)
      connectionType: PRIMARY
      # Database link
      database:
        name: simple
      # Generated secret name with information for the selected database
      generatedSecretName: simple1
  # Import secret that will contain "USERNAME" and "PASSWORD" for provided mode
  importSecretName: provided-simple
```

with import secret:

```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: provided-simple
data:
  USERNAME: fake
  PASSWORD: fake
```

### Managed mode

Here is an example of Custom Resource:

```yaml
apiVersion: postgresql.easymile.com/v1alpha1
kind: PostgresqlUserRole
metadata:
  name: managed-simple-rotation
spec:
  # Mode
  mode: MANAGED
  # Role prefix to be used for user created in database engine
  rolePrefix: "managed-simple"
  # User password rotation duration in order to roll user/password in secret
  userPasswordRotationDuration: 720h
  # Privileges
  privileges:
    - # Privilege for the selected database
      privilege: OWNER
      # Connection type to be used for secret generation (Can be set to BOUNCER if wanted and supported by engine configuration)
      connectionType: PRIMARY
      # Database link
      database:
        name: simple
      # Generated secret name with information for the selected database
      generatedSecretName: managed-simple-rotation
```

### Generate secret

Here is an example:

```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: managed-simple-rotation
data:
  POSTGRES_URL: postgresql://fake-0:password@localhost:5432/database1
  POSTGRES_URL_ARGS: postgresql://fake-0:password@localhost:5432/database1?sslmode=require
  PASSWORD: password
  LOGIN: fake-0
  DATABASE: database1
  HOST: localhost
  PORT: "5432"
  ARGS: sslmode=require
```
