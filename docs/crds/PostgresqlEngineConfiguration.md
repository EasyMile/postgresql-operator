# PostgresqlEngineConfiguration

## Description

This Custom Resource represents a PosgreSQL Engine Configuration with all necessary data to connect it.

## Custom Resource Definition

### kubectl names and short names

All these names are available for `kubectl`:
- postgresqlengineconfigurations.postgresql.easymile.com
- postgresqlengineconfigurations
- postgresqlengineconfiguration
- pgengcfg
- pgec

### Root fields

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata | Object metadata | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#objectmeta-v1-meta) | false |
| spec | Specification of the PostgreSQL Engine configuration. | [PostgresqlEngineConfigurationSpec](#postgresqlengineconfigurationspec) | true |
| status | Most recent observed status of the PostgreSQL Engine Configuration. Read-only. Not included when requesting from the apiserver, only from the PostgreSQL Operator API itself. More info: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status | [PostgresqlEngineConfigurationStatus](#postgresqlengineconfigurationstatus) | false |

### PostgresqlEngineConfigurationSpec

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| provider | PostgreSQL Provider. This can be "", "AWS" or "AZURE". **Note**: AWS and Azure aren't well tested and might not work. This support is imported from [movetokube/postgres-operator](https://github.com/movetokube/postgres-operator) | String | false |
| host | PostgreSQL Hostname | String | true |
| port | PostgreSQL Port. Default value is `5432` | Integer | false |
| uriArgs | PostgreSQL URI arguments like `sslmode=disabled` | String | false |
| defaultDatabase | Default database to connect for administration commands. Default is `postgres`. | String | false |
| checkInterval | Interval between 2 connectivity check. Default is `30s`. | String | false |
| waitLinkedResourcesDeletion | Tell operator if it has to wait until all linked resources are deleted to delete current custom resource. If not, it won't be able to delete PostgresqlDatabase and PostgresqlUser after. Default value is `false`. | Boolean | false |
| secretName | Secret name in the same namespace has the current custom resource that contains user and password to be used to connect PostgreSQL engine. An example can be found [here](../../deploy/examples/engineconfiguration/engineconfigurationsecret.yaml) | String | true |
| userConnections | User connections used for secret generation. That will be used to generate secret with primary server as url or to use the pg bouncer one. Note: Operator won't check those values. | [UserConnections](#userconnections) | false |

### UserConnections

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| primaryConnection | Primary connection is referring to the primary node connection. If not being set, all values will be set from spec (host, port, uriArgs) | [GenericUserConnection](#genericuserconnection) | false |
| bouncerConnection | Bouncer connection is referring to a pg bouncer node. The default port will be 6432 if other fields are filled but not port. | [GenericUserConnection](#genericuserconnection) | false |

### GenericUserConnection

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| host | PostgreSQL Hostname | String | true |
| port | PostgreSQL Port. | Integer | false |
| uriArgs | PostgreSQL URI arguments like `sslmode=disabled` | String | false |

### PostgresqlEngineConfigurationStatus

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| phase | Current phase of the operator on the current custom resource | String | true |
| message | Human-readable message indicating details about current operator phase or error | String | false |
| ready | True if all resources are in a ready state and all work is done by operator | Boolean | false |
| lastValidatedTime | Last time the operator has successfully connected to the PostgreSQL engine | String | false |
| hash | Resource spec hash for internal needs | String | false |

## Example

Here is an example of Custom Resource:

```yaml
apiVersion: postgresql.easymile.com/v1alpha1
kind: PostgresqlEngineConfiguration
metadata:
  name: full-example
spec:
  # Provider type
  # Default to ""
  provider: ""
  # PostgreSQL Hostname
  host: postgres
  # PostgreSQL Port
  # Default to 5432
  port: 5432
  # Secret name in the current namespace to find "user" and "password"
  secretName: pgenginesecrets
  # URI args to add for PostgreSQL URL
  # Default to ""
  uriArgs: sslmode=disabled
  # Default database name
  # Default to "postgres"
  defaultDatabase: postgres
  # Check interval
  # Default to 30s
  checkInterval: 30s
  # Wait for linked resource to be deleted
  # Default to false
  waitLinkedResourcesDeletion: true
  # User connections used for secret generation
  # That will be used to generate secret with primary server as url or
  # to use the pg bouncer one.
  # Note: Operator won't check those values.
  userConnections:
    # Primary connection is referring to the primary node connection.
    primaryConnection:
      host: localhost
      uriArgs: sslmode=disable
      port: 5432
    # Bouncer connection is referring to a pg bouncer node.
    # bouncerConnection:
    #   host: localhost
    #   uriArgs: sslmode=disable
    #   port: 6432
```
