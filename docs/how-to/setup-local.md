# How to setup your local environment for developing ?

## Minikube (optional)

You should setup a Minikube for local development.

Download it if you haven't installed.

Command to launch it:

```bash
minikube start --kubernetes-version=v1.16.0
```

## Install local environment

- You should install CRDs first
  ```bash
  kubectl apply -f ./deploy/crds/
  ```
- Install dev resources
  ```bash
  kubectl apply -f ./deploy/dev/
  ```
