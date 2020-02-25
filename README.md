<h1 align="center">PostgreSQL Operator</h1>

<p align="center">
  <a href="http://godoc.org/github.com/easymile/postgresql-operator" rel="noopener noreferer" target="_blank"><img src="https://img.shields.io/badge/godoc-reference-blue.svg" alt="Go Doc" /></a>
  <a href="https://travis-ci.org/EasyMile/postgresql-operator" rel="noopener noreferer" target="_blank"><img src="https://travis-ci.org/EasyMile/postgresql-operator.svg?branch=master" alt="Travis CI" /></a>
  <a href="https://goreportcard.com/report/github.com/easymile/postgresql-operator" rel="noopener noreferer" target="_blank"><img src="https://goreportcard.com/badge/github.com/easymile/postgresql-operator" alt="Go Report Card" /></a>
  <a href="https://hub.docker.com/r/easymile/postgresql-operator" rel="noopener noreferer" target="_blank"><img src="https://img.shields.io/docker/pulls/easymile/postgresql-operator.svg" alt="Docker Pulls" /></a>
  <a href="https://github.com/easymile/postgresql-operator/blob/master/LICENSE" rel="noopener noreferer" target="_blank"><img src="https://img.shields.io/github/license/easymile/postgresql-operator" alt="GitHub license" /></a>
  <a href="https://github.com/easymile/postgresql-operator/releases" rel="noopener noreferer" target="_blank"><img src="https://img.shields.io/github/v/release/easymile/postgresql-operator" alt="GitHub release (latest by date)" /></a>
</p>

## Features

- Create or update Databases with extensions and schemas
- Create or update Users with rights (Owner, Writer or Reader)
- Connections to multiple PostgreSQL Engines
- Generate secrets for User login and password
- Allow to change User password based on time (e.g: Each 30 days)

## Concepts

When we speak about `Engines`, we speak about PostgreSQL Database Servers. It isn't the same as Databases. Databases will store tables, ...

In this operator, Users are linked to Databases and doesn't exist without it. They are "children" of databases.

Moreover, a single User can only have rights to one Database.

## Supported Custom Resources

| CustomResourceDefinition                                                    | Description                                                                        |
| --------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| [PostgresqlEngineConfiguration](docs/crds/PostgresqlEngineConfiguration.md) | Represents a PostgreSQL Engine Configuration with all necessary data to connect it |
| [PostgresqlDatabase](docs/crds/PostgresqlDatabase.md)                       | Represents a PostgreSQL Database                                                   |
| [PostgresqlUser](docs/crds/PostgresqlUser.md)                               | Represents a PostgreSQL User                                                       |

## How to deploy ?

### Using Helm 3

The project has a Helm 3 chart located in `deploy/helm/postgresql-operator`.

It will deploy the operator running the command:

```bash
helm install postgresql-operator ./deploy/helm/postgresql-operator
```

### Using Helm 2

As the chart located in `deploy/helm/postgresql-operator` uses the specific Helm 3 folder called `crd`. The chart can **only** install the operator but not the CRDs.

CRDs have to be installed manually.

The procedure is the following:

- Install CRDs
  ```bash
  kubectl apply -f ./deploy/crds/
  ```
- Install the chart
  ```bash
  helm install postgresql-operator ./deploy/helm/postgresql-operator
  ```

### Using Kubectl

The procedure is the following:

- Install CRDs
  ```bash
  kubectl apply -f ./deploy/crds/
  ```
- Install operator
  ```bash
  kubectl apply -f ./deploy/role_binding.yaml
  kubectl apply -f ./deploy/role.yaml
  kubectl apply -f ./deploy/service_account.yaml
  kubectl apply -f ./deploy/operator.yaml
  ```

## Want to contribute ?

- Read the [CONTRIBUTING guide](./CONTRIBUTING.md)
- Read how to setup your environment [here](./docs/how-to/setup-local.md)

## License

MIT (See [LICENSE](LICENSE))
