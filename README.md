<h1 align="center">PostgreSQL Operator</h1>

<p align="center">
  <a href="http://godoc.org/github.com/easymile/postgresql-operator" rel="noopener noreferer" target="_blank"><img src="https://img.shields.io/badge/godoc-reference-blue.svg" alt="Go Doc" /></a>
  <a href="https://github.com/easymile/postgresql-operator/actions/workflows/ci.yml" rel="noopener noreferer" target="_blank"><img src="https://github.com/easymile/postgresql-operator/actions/workflows/ci.yml/badge.svg" alt="Github Actions" /></a>
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
| [PostgresqlUserRole](docs/crds/PostgresqlUserRole.md)                       | Represents a PostgreSQL User Role                                                  |

## How to deploy ?

### Using Helm

#### From EasyMile Helm Chart Repository

```bash
helm repo add easymile https://easymile.github.io/helm-charts/
```

And then deploy:

```bash
helm install postgresql-operator easymile/postgresql-operator
```

#### From Git

The project has a Helm 3 chart located in `deploy/helm/postgresql-operator`.

It will deploy the operator running the command:

```bash
helm install postgresql-operator ./helm/postgresql-operator
```

## Getting Started

You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

Read how to setup your environment [here](./docs/how-to/setup-local.md)

### Running on the cluster

1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/postgresql-operator:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/postgresql-operator:tag
```

### Uninstall CRDs

To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller

UnDeploy the controller to the cluster:

```sh
make undeploy
```

## Contributing

Read the [CONTRIBUTING guide](./CONTRIBUTING.md)

### How it works

This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/)
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster

### Test It Out

1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions

If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
