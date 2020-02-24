## Copied and modified from Keycloak-operator

# Other contants
NAMESPACE=postgresql
PROJECT=postgresql-operator
PKG=github.com/easymile/postgresql-operator
OPERATOR_SDK_VERSION=v0.15.2
OPERATOR_SDK_DOWNLOAD_URL=https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk-$(OPERATOR_SDK_VERSION)-x86_64-linux-gnu
MINIKUBE_DOWNLOAD_URL=https://storage.googleapis.com/minikube/releases/v1.7.3/minikube-linux-amd64
KUBECTL_DOWNLOAD_URL=https://storage.googleapis.com/kubernetes-release/release/v1.16.0/bin/linux/amd64/kubectl

# Compile constants
COMPILE_TARGET=./tmp/_output/bin/$(PROJECT)
GOOS=linux
GOARCH=amd64
CGO_ENABLED=0

.DEFAULT_GOAL := code/check

##############################
# Release                    #
##############################

.PHONY: release/olm-catalog
release/olm-catalog:
	@echo Making OLM Catalog for release
	@operator-sdk generate csv --csv-channel alpha --csv-version $(version) --update-crds

.PHONY: release/docker
release/docker: code/docker
	@echo Releasing docker image
	@docker push easymile/postgresql-operator:$(version)

##############################
# Operator Management        #
##############################
.PHONY: cluster/prepare
cluster/prepare:
	@echo Preparing cluster
	@kubectl apply -f deploy/crds/ || true
	@kubectl create namespace $(NAMESPACE) || true
	@kubectl apply -f deploy/role.yaml -n $(NAMESPACE) || true
	@kubectl apply -f deploy/role_binding.yaml -n $(NAMESPACE) || true
	@kubectl apply -f deploy/service_account.yaml -n $(NAMESPACE) || true

.PHONY: cluster/clean
cluster/clean:
	@echo Cleaning cluster
	# Remove all roles, rolebindings and service accounts with the name postgresql-operator
	@kubectl get roles,rolebindings,serviceaccounts postgresql-operator -n $(NAMESPACE) --no-headers=true -o name | xargs kubectl delete -n $(NAMESPACE)
	# Remove all CRDS with postgresql.easymile.com in the name
	@kubectl get crd --no-headers=true -o name | awk '/postgresql.easymile.com/{print $1}' | xargs kubectl delete
	@kubectl delete namespace $(NAMESPACE)

.PHONY: cluster/create/examples
cluster/create/examples:
	@echo Setup examples
	@kubectl create -f deploy/examples/engineconfiguration/engineconfigurationsecret.yaml -n $(NAMESPACE)
	@kubectl create -f deploy/examples/engineconfiguration/simple.yaml -n $(NAMESPACE)
	@kubectl create -f deploy/examples/database/simple.yaml -n $(NAMESPACE)
	@kubectl create -f deploy/examples/user/simple.yaml -n $(NAMESPACE)

##############################
# Tests                      #
##############################
# FOR THE MOMENT, NO TESTS
# .PHONY: test/unit
# test/unit:
# 	@echo Running tests:
# 	@go test -v -tags=unit -coverpkg ./... -coverprofile cover-unit.coverprofile -covermode=count ./pkg/...

# .PHONY: test/e2e
# test/e2e: cluster/prepare
# 	@echo Running tests:
# 	@touch deploy/empty-init.yaml
# 	# This is not recommended way or running the tests (see https://github.com/operator-framework/operator-sdk/blob/master/doc/test-framework/writing-e2e-tests.md#running-go-test-directly-not-recommended)
# 	# However, this way we will have a consistent way of running tests on Travis and locally. The downside
# 	# is that Operator testing harness downloads things manually using `go mod` when executing the tests.
# 	# Here is a corresponding Operator SDK call:
# 	# operator-sdk test  local --go-test-flags "-tags=integration -coverpkg ./... -coverprofile cover-e2e.coverprofile -covermode=count" --namespace ${NAMESPACE} --up-local --debug --verbose ./test/e2e
# 	go test -tags=integration -coverpkg ./... -coverprofile cover-e2e.coverprofile -covermode=count -mod=vendor ./test/e2e/... -root=$(PWD) -kubeconfig=$(HOME)/.kube/config -globalMan deploy/empty-init.yaml -namespacedMan deploy/empty-init.yaml -v -singleNamespace -parallel=1 -localOperator

# .PHONY: test/coverage/prepare
# test/coverage/prepare:
# 	@echo Preparing coverage file:
# 	@echo "mode: count" > cover-all.coverprofile
# 	@tail -n +2 cover-unit.coverprofile >> cover-all.coverprofile
# 	@tail -n +2 cover-e2e.coverprofile >> cover-all.coverprofile
# 	@echo Running test coverage generation:
# 	@which cover 2>/dev/null ; if [ $$? -eq 1 ]; then \
# 		go get golang.org/x/tools/cmd/cover; \
# 	fi
# 	@go tool cover -html=cover-all.coverprofile -o cover.html

# .PHONY: test/coverage
# test/coverage: test/coverage/prepare
# 	@go tool cover -html=cover-all.coverprofile -o cover.html

##############################
# Local Development          #
##############################
.PHONY: setup
setup: setup/mod code/gen

.PHONY: setup/mod
setup/mod:
	@echo Adding vendor directory
	go mod vendor
	@echo setup complete

.PHONY: setup/operator-sdk
setup/operator-sdk:
	@echo Installing Operator SDK
	@curl -Lo operator-sdk ${OPERATOR_SDK_DOWNLOAD_URL} && chmod +x operator-sdk && sudo mv operator-sdk /usr/local/bin/

.PHONY: code/run
code/run:
	@echo Running code using operator-sdk
	@operator-sdk run --local --namespace=${NAMESPACE}

.PHONY: code/compile
code/compile:
	@echo Compiling code using go
	@GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=${CGO_ENABLED} go build -o=$(COMPILE_TARGET) -mod=vendor ./cmd/manager

.PHONY: code/docker
code/docker:
	@echo Building docker image
	@operator-sdk build easymile/postgresql-operator:$(version)

.PHONY: code/gen
code/gen:
	@echo Generating CRD and Kubernetes code using operator-sdk
	operator-sdk generate k8s
	operator-sdk generate crds
	cp deploy/crds/* deploy/helm/postgresql-operator/crds/

.PHONY: code/check
code/check:
	@echo Running go fmt
	go fmt $$(go list ./... | grep -v /vendor/)

HAS_GOLANGCI_LINT := $(shell command -v golangci-lint;)
.PHONY: code/lint
code/lint:
	@echo Running golangci-lint
ifndef HAS_GOLANGCI_LINT
	@echo "=> Installing golangci-lint tool"
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.23.6
endif
	golangci-lint run

##############################
# CI                         #
##############################
.PHONY: setup/travis
setup/travis: setup/operator-sdk
	@echo Installing Kubectl
	@curl -Lo kubectl ${KUBECTL_DOWNLOAD_URL} && chmod +x kubectl && sudo mv kubectl /usr/local/bin/
	@echo Installing Minikube
	@curl -Lo minikube ${MINIKUBE_DOWNLOAD_URL} && chmod +x minikube && sudo mv minikube /usr/local/bin/
	@echo Booting Minikube up, see Travis env. variables for more information
	@mkdir -p $HOME/.kube $HOME/.minikube
	@touch $KUBECONFIG
	@sudo minikube start --vm-driver=none --kubernetes-version=v1.16.0
	@sudo chown -R travis: /home/travis/.minikube/

# NO TEST FOR THE MOMENT
# .PHONY: test/goveralls
# test/goveralls: test/coverage/prepare
# 	@echo "Preparing goveralls file"
# 	go get -u github.com/mattn/goveralls
# 	@echo "Running goveralls"
# 	@goveralls -v -coverprofile=cover-all.coverprofile -service=travis-ci
