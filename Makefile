.DEFAULT_GOAL := help

# Default platform commands
RMDIR := rm -rf

##########################################
# Globals
##########################################
GIT_CURRENT_BRANCH_NAME	:= $(shell git rev-parse --abbrev-ref HEAD) 

REPO_ROOT = $(shell git rev-parse --show-toplevel)
ifndef TAG
	TAG ?= $(shell git describe --tags --always)
endif
OUTPUT_DIR = $(REPO_ROOT)/output
ARTIFACTS_DIR = $(REPO_ROOT)/artifacts
BUILD_DIR = $(OUTPUT_DIR)/$(GOOS)_$(GOARCH)
RETINA_BUILD_DIR = $(BUILD_DIR)/retina
RETINA_DIR = $(REPO_ROOT)/controller
OPERATOR_DIR=$(REPO_ROOT)/operator
CAPTURE_WORKLOAD_DIR = $(REPO_ROOT)/captureworkload
CLI_DIR = $(REPO_ROOT)/cli
BIN_DIR = $(REPO_ROOT)/bin

KIND = /usr/local/bin/kind
KIND_CLUSTER = retina-cluster
WINVER2022   ?= "10.0.20348.1906"
WINVER2019   ?= "10.0.17763.4737"
APP_INSIGHTS_ID ?= ""
GENERATE_TARGET_DIRS = \
	./pkg/plugin/linuxutil

# Default platform is linux/amd64
GOOS			?= linux
GOARCH			?= amd64
OS				?= $(GOOS)
ARCH			?= $(GOARCH)
PLATFORM		?= $(OS)/$(ARCH)
PLATFORMS		?= linux/amd64 linux/arm64 windows/amd64
OS_VERSION		?= ltsc2019

# This may be modified via the update-hubble GitHub Action
HUBBLE_VERSION ?= v1.16.6 

CONTAINER_BUILDER ?= docker
CONTAINER_RUNTIME ?= docker
YEAR 			  ?= 2022

ALL_ARCH.linux = amd64 arm64
ALL_ARCH.windows = amd64

# TLS
ENABLE_TLS ?= true
CERT_DIR := $(REPO_ROOT)/.certs

CERT_FILES := tls.crt:tls-client-cert-file \
              tls.key:tls-client-key-file \
              ca.crt:tls-ca-cert-files

# TAG is OS and platform agonstic, which can be used for binary version and image manifest tag,
# while RETINA_PLATFORM_TAG is platform specific, which can be used for image built for specific platforms.
RETINA_PLATFORM_TAG        ?= $(TAG)-$(subst /,-,$(PLATFORM))

# Used for looping through components in container build
AGENT_TARGETS ?= init agent

WINDOWS_YEARS ?= "2019 2022"

# For Windows OS, add year to the platform tag
ifeq ($(OS),windows)
RETINA_PLATFORM_TAG        = $(TAG)-windows-ltsc$(YEAR)-amd64
endif

##########################################
##@ Help
##########################################

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##########################################
##@ Tools
##########################################

TOOLS_DIR		= $(REPO_ROOT)/hack/tools
TOOLS_BIN_DIR	= $(TOOLS_DIR)/bin

GOFUMPT			= $(TOOLS_BIN_DIR)/gofumpt
GOLANGCI_LINT	= $(TOOLS_BIN_DIR)/golangci-lint
GORELEASER		= $(TOOLS_BIN_DIR)/goreleaser
CONTROLLER_GEN	= $(TOOLS_BIN_DIR)/controller-gen
GINKGO			= $(TOOLS_BIN_DIR)/ginkgo
MOCKGEN			= $(TOOLS_BIN_DIR)/mockgen
ENVTEST			= $(TOOLS_BIN_DIR)/setup-envtest

$(TOOLS_DIR)/go.mod:
	cd $(TOOLS_DIR); go mod init && go mod tidy

$(GOFUMPT): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go mod download; go build -tags=tools -o $(BIN_DIR)/gofumpt mvdan.cc/gofumpt

.PHONY: gofumpt
gofumpt: $(GOFUMPT) ## Build gofumpt

$(GOLANGCI_LINT): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go mod download; go build -tags=tools -o $(BIN_DIR)/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Build golangci-lint

$(CONTROLLER_GEN): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go mod download; go build -tags=tools -o $(BIN_DIR)/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

.PHONY: goreleaser
goreleaser: $(GORELEASER) ## Build goreleaser

$(GORELEASER): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go mod download; go build -tags=tools -o $(BIN_DIR)/goreleaser github.com/goreleaser/goreleaser

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Build controller-gen

$(GINKGO): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go mod download; go build -tags=tools -o $(BIN_DIR)/ginkgo github.com/onsi/ginkgo/ginkgo

.PHONY: ginkgo
ginkgo: $(GINKGO) ## Build ginkgo

$(MOCKGEN): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go mod download; go build -tags=$(TOOL_TAG) -o $(BIN_DIR)/mockgen go.uber.org/mock/mockgen

.PHONY: mockgen
mockgen: $(MOCKGEN) ## Build mockgen

$(ENVTEST): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go mod download; go build -tags=$(TOOL_TAG) -o $(BIN_DIR)/setup-envtest sigs.k8s.io/controller-runtime/tools/setup-envtest

.PHONY: setup-envtest
setup-envtest: $(ENVTEST) ## Build setup-envtest

##########################################
##@ Utils 
##########################################

FMT_PKG  ?= .
LINT_PKG ?= .

.PHONY: quick-build
quick-build: ## Builds Retina agent and operator image for Linux-AMD64 and push to configured Container Registry
	$(MAKE) retina-image PLATFORM=linux/amd64 BUILDX_ACTION=--push
	$(MAKE) retina-operator-image PLATFORM=linux/amd64 BUILDX_ACTION=--push

.PHONY: quick-deploy
quick-deploy: ## Deploys Retina agent and operator with Helm for Linux-AMD64 (Standard Control Plane)
	$(MAKE) helm-install-advanced-local-context HELM_IMAGE_TAG=$(TAG)-linux-amd64

.PHONY: quick-deploy-hubble
quick-deploy-hubble: ## Deploys Retina agent and operator with Helm for Linux-AMD64 (Hubble Control Plane)
	$(MAKE) helm-uninstall || true
	$(MAKE) helm-install-without-tls HELM_IMAGE_TAG=$(TAG)-linux-amd64

.PHONY: version
version: ## Prints the tag pointing to the current commit, or the short commit hash if no tag is present
	@if [ "$(shell git tag --points-at HEAD)" != "" ]; then \
		export VERSION="$$(git tag --points-at HEAD)"; \
	else \
		export VERSION="$$(git rev-parse --short HEAD)"; \
	fi; \
	echo "$${VERSION}"

.PHONY: fmt
fmt: $(GOFUMPT) ## Run gofumpt on $FMT_PKG (default "retina")
	$(GOFUMPT) -w $(FMT_PKG)

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Fast lint on default branch showing only new issues
	$(GOLANGCI_LINT) run --new-from-rev main --timeout 10m -v $(LINT_PKG)/...

.PHONY: lint-existing
lint-existing: $(GOLANGCI_LINT) ## Lint the current branch in entirety
	$(GOLANGCI_LINT) run -v $(LINT_PKG)/...

.PHONY: clean
clean: ## Clean build artifacts
	$(RMDIR) $(OUTPUT_DIR)

.PHONY: simplify-dashboards
simplify-dashboards: ## Simplify Grafana dashboards
	cd deploy/testutils && go test ./... -tags=dashboard,simplifydashboard -v && cd $(REPO_ROOT)

##########################################
##@ Generate (Host)
##########################################

.PHONY: all
all: generate ## Generate eBPF wrappers and Go code

.PHONY: generate
generate: generate-bpf-go
	go generate ./...
	for dir in $(GENERATE_TARGET_DIRS); do \
			make -C $$dir $@; \
	done

.PHONY: generate-bpf-go
generate-bpf-go: ## Generate eBPF wrappers for plugins for all archs
	for arch in $(ALL_ARCH.linux); do \
        GOARCH=$$arch go generate ./pkg/plugin/...; \
    done

##########################################
##@ Generate (Docker)
##########################################

.PHONY: all-gen
all-gen: ## Generate Protobuf and Go code
	$(MAKE) proto-gen
	$(MAKE) go-gen

.PHONY: proto-gen
proto-gen: ## Generate Protobuf code
	docker build --platform=linux/amd64 \
		-t $(IMAGE_REGISTRY)/$(RETINA_PROTO_IMAGE):$(RETINA_PLATFORM_TAG) \
		-f controller/Dockerfile.proto .
	docker run --rm --platform=linux/amd64 \
		--user $(shell id -u):$(shell id -g) \
		-v $(PWD):/app $(IMAGE_REGISTRY)/$(RETINA_PROTO_IMAGE):$(RETINA_PLATFORM_TAG)

.PHONY: go-gen
go-gen: ## Generates Go code
	docker build -t $(IMAGE_REGISTRY)/$(RETINA_GO_GEN_IMAGE):$(RETINA_PLATFORM_TAG) \
		--build-arg GOOS=$(GOOS) \
		--build-arg GOARCH=$(GOARCH) \
		-f controller/Dockerfile.gogen .
	docker run --rm --user $(shell id -u):$(shell id -g) -v $(PWD):/app $(IMAGE_REGISTRY)/$(RETINA_GO_GEN_IMAGE):$(RETINA_PLATFORM_TAG)

##########################################
##@ Build Binaries
##########################################

retina: ## Builds Retina binary
	$(MAKE) retina-binary 

retina-binary:
	go generate ./... && \
	go build -v -o $(RETINA_BUILD_DIR)/retina$(EXE_EXT) -gcflags="-dwarflocationlists=true" -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version=$(TAG) -X github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID=$(APP_INSIGHTS_ID)" $(RETINA_DIR)/main.go

retina-capture-workload: ## Build the Retina capture workload
	cd $(CAPTURE_WORKLOAD_DIR) && go build -v -o $(RETINA_BUILD_DIR)/captureworkload$(EXE_EXT) -gcflags="-dwarflocationlists=true"  -ldflags "-X main.version=$(TAG)"

retina-cli: ## Build the Retina CLI
	go build -o $(BIN_DIR)/kubectl-retina $(CLI_DIR)/main.go

##########################################
##@ Containers
##########################################

IMAGE_REGISTRY	?= ghcr.io
IMAGE_NAMESPACE ?= $(shell git config --get remote.origin.url | sed -E 's/.*github\.com[\/:]([^\/]+)\/([^\/.]+)(.git)?/\1\/\2/' | tr '[:upper:]' '[:lower:]')

RETINA_BUILDER_IMAGE			= $(IMAGE_NAMESPACE)/retina-builder
RETINA_TOOLS_IMAGE				= $(IMAGE_NAMESPACE)/retina-tools
RETINA_IMAGE 					= $(IMAGE_NAMESPACE)/retina-agent
RETINA_INIT_IMAGE				= $(IMAGE_NAMESPACE)/retina-init
RETINA_OPERATOR_IMAGE			= $(IMAGE_NAMESPACE)/retina-operator
RETINA_SHELL_IMAGE				= $(IMAGE_NAMESPACE)/retina-shell
KUBECTL_RETINA_IMAGE			= $(IMAGE_NAMESPACE)/kubectl-retina
RETINA_INTEGRATION_TEST_IMAGE	= $(IMAGE_NAMESPACE)/retina-integration-test
RETINA_PROTO_IMAGE				= $(IMAGE_NAMESPACE)/retina-proto-gen
RETINA_GO_GEN_IMAGE				= $(IMAGE_NAMESPACE)/retina-go-gen
KAPINGER_IMAGE 					= kapinger

container-push: # Util target to publish container image. (Do not invoke directly)
	$(CONTAINER_BUILDER) push \
		$(IMAGE_REGISTRY)/$(IMAGE):$(TAG)

container-pull: ## Util target to pull container image
	$(CONTAINER_BUILDER) pull \
		$(IMAGE_REGISTRY)/$(IMAGE):$(TAG)

.PHONY: qemu-user-static
qemu-user-static: ## Set up the host to run QEMU
	sudo $(CONTAINER_RUNTIME) run --rm --privileged multiarch/qemu-user-static --reset -p yes

skopeo-export: ## Util target to copy a container from containers-storage to the docker daemon
	skopeo copy \
		containers-storage:$(REF) \
		docker-daemon:$(REF)

retina-skopeo-export: ## Util target to copy a container from containers-storage to the docker daemon
	$(MAKE) skopeo-export \
		REF=$(IMAGE_REGISTRY)/$(RETINA_IMAGE):$(RETINA_PLATFORM_TAG) \
		IMG=$(RETINA_IMAGE)
		TAG=$(RETINA_PLATFORM_TAG)

manifest-skopeo-archive: ## Util target to export tar archive of multiarch container manifest
	skopeo copy --all docker://$(IMAGE_REGISTRY)/$(IMAGE):$(TAG) oci-archive:$(IMAGE_ARCHIVE_DIR)/$(IMAGE)-$(TAG).tar --debug

buildx: ## Create a Retina buildx instance 
	if docker buildx inspect retina > /dev/null 2>&1; then \
		echo "Buildx instance retina already exists."; \
	else \
		echo "Creating buildx instance retina..."; \
		docker buildx create --name retina --use --driver-opt image=mcr.microsoft.com/oss/v2/moby/buildkit:v0.16.0-2 --platform $$(echo "$(PLATFORMS)" | tr ' ' ','); \
		docker buildx use retina; \
		echo "Buildx instance retina created."; \
	fi;

container-docker: buildx # Util target to build container images using docker buildx. (Do not invoke directly)
	os=$$(echo $(PLATFORM) | cut -d'/' -f1); \
	arch=$$(echo $(PLATFORM) | cut -d'/' -f2); \
	image_name=$$(basename $(IMAGE)); \
	image_metadata_filename="image-metadata-$$image_name-$(TAG).json"; \
	touch $$image_metadata_filename; \
	echo "Building $$image_name for $$os/$$arch "; \
	mkdir -p $(ARTIFACTS_DIR); \
	docker buildx build \
		--platform $(PLATFORM) \
		--metadata-file=$$image_metadata_filename \
		-f $(DOCKERFILE) \
		--build-arg APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
		--build-arg GOARCH=$$arch \
		--build-arg GOOS=$$os \
		--build-arg OS_VERSION=$(OS_VERSION) \
		--build-arg HUBBLE_VERSION=$(HUBBLE_VERSION) \
		--build-arg VERSION=$(VERSION) $(EXTRA_BUILD_ARGS) \
		--target=$(TARGET) \
		-t $(IMAGE_REGISTRY)/$(IMAGE):$(TAG) \
		--output type=local,dest=$(ARTIFACTS_DIR) \
		$(BUILDX_ACTION) \
		$(CONTEXT_DIR) 

retina-image: ## Build the Retina Linux container image
	echo "Building for $(PLATFORM)"
	for target in $(AGENT_TARGETS); do \
		echo "Building for $$target"; \
		if [ "$$target" = "init" ]; then \
			image_name=$(RETINA_INIT_IMAGE); \
		else \
			image_name=$(RETINA_IMAGE); \
		fi; \
		$(MAKE) container-$(CONTAINER_BUILDER) \
				PLATFORM=$(PLATFORM) \
				DOCKERFILE=controller/Dockerfile \
				REGISTRY=$(IMAGE_REGISTRY) \
				IMAGE=$$image_name \
				VERSION=$(TAG) \
				TAG=$(RETINA_PLATFORM_TAG) \
				APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
				CONTEXT_DIR=$(REPO_ROOT) \
				TARGET=$$target; \
	done

retina-image-win: ## Build the Retina Windows container image
	for year in $(WINDOWS_YEARS); do \
		tag=$(TAG)-windows-ltsc$$year-amd64; \
		echo "Building $(RETINA_PLATFORM_TAG)"; \
		set -e ; \
		$(MAKE) container-$(CONTAINER_BUILDER) \
				PLATFORM=windows/amd64 \
				DOCKERFILE=controller/Dockerfile \
				REGISTRY=$(IMAGE_REGISTRY) \
				IMAGE=$(RETINA_IMAGE) \
				OS_VERSION=ltsc$$year \
				VERSION=$(TAG) \
				TAG=$$tag \
				TARGET=agent-win \
				CONTEXT_DIR=$(REPO_ROOT); \
	done

retina-operator-image:  ## Build the Retina Linux operator image
	echo "Building for $(PLATFORM)"
	set -e ; \
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=operator/Dockerfile \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(RETINA_OPERATOR_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
			CONTEXT_DIR=$(REPO_ROOT)

retina-shell-image: ## Build the Retina Linux shell image
	echo "Building for $(PLATFORM)"
	set -e ; \
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=shell/Dockerfile \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(RETINA_SHELL_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			CONTEXT_DIR=$(REPO_ROOT)

kubectl-retina-image: ## Build the kubectl-retina image
	echo "Building for $(PLATFORM)"
	set -e ; \
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=cli/Dockerfile \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(KUBECTL_RETINA_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			CONTEXT_DIR=$(REPO_ROOT)

kapinger-image: ## Build the kapinger image
	docker buildx build --builder retina --platform windows/amd64 --target windows-amd64 -t $(IMAGE_REGISTRY)/$(KAPINGER_IMAGE):$(TAG)-windows-amd64  ./hack/tools/kapinger/ --push
	docker buildx build --builder retina --platform linux/amd64 --target linux-amd64 -t $(IMAGE_REGISTRY)/$(KAPINGER_IMAGE):$(TAG)-linux-amd64  ./hack/tools/kapinger/ --push
	docker buildx build --builder retina --platform linux/arm64 --target linux-arm64 -t $(IMAGE_REGISTRY)/$(KAPINGER_IMAGE):$(TAG)-linux-arm64  ./hack/tools/kapinger/ --push
	docker buildx imagetools create -t $(IMAGE_REGISTRY)/$(KAPINGER_IMAGE):$(TAG) \
		$(IMAGE_REGISTRY)/$(KAPINGER_IMAGE):$(TAG)-windows-amd64 \
		$(IMAGE_REGISTRY)/$(KAPINGER_IMAGE):$(TAG)-linux-amd64 \
		$(IMAGE_REGISTRY)/$(KAPINGER_IMAGE):$(TAG)-linux-arm64

toolbox: ## Build the toolbox image
	docker buildx build --builder retina --platform linux/amd64  -t $(IMAGE_REGISTRY)/toolbox:$(TAG)   -f ./hack/tools/toolbox/Dockerfile ./hack/tools/ --push

##########################################
##@ Multiplatform manifests
##########################################

manifest-retina-image: ## Create a multiplatform manifest for the Retina image
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_IMAGE):$(TAG))
	$(eval FULL_INIT_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE):$(TAG))
	docker buildx imagetools create -t $(FULL_IMAGE_NAME) $(foreach platform,linux/amd64 linux/arm64 windows-ltsc2019-amd64 windows-ltsc2022-amd64, $(FULL_IMAGE_NAME)-$(subst /,-,$(platform)))
	docker buildx imagetools create -t $(FULL_INIT_IMAGE_NAME) $(foreach platform,linux/amd64 linux/arm64, $(FULL_INIT_IMAGE_NAME)-$(subst /,-,$(platform)))

manifest-operator-image: ## Create a multiplatform manifest for the operator image
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE):$(TAG))
	docker buildx imagetools create -t $(FULL_IMAGE_NAME) $(foreach platform,linux/amd64, $(FULL_IMAGE_NAME)-$(subst /,-,$(platform)))

manifest-shell-image: ## Create a multiplatform manifest for the shell image
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_SHELL_IMAGE):$(TAG))
	docker buildx imagetools create -t $(FULL_IMAGE_NAME) $(foreach platform,linux/amd64 linux/arm64, $(FULL_IMAGE_NAME)-$(subst /,-,$(platform)))

manifest-kubectl-retina-image: ## Create a multiplatform manifest for the kubectl-retina image
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(KUBECTL_RETINA_IMAGE):$(TAG))
	docker buildx imagetools create -t $(FULL_IMAGE_NAME) $(foreach platform,linux/amd64 linux/arm64, $(FULL_IMAGE_NAME)-$(subst /,-,$(platform)))

manifest: ## Create a multiplatform manifest for $COMPONENT
	echo "Building for $(COMPONENT)"
	if [ "$(COMPONENT)" = "retina" ]; then \
		$(MAKE) manifest-retina-image; \
	elif [ "$(COMPONENT)" = "operator" ]; then \
		$(MAKE) manifest-operator-image; \
	elif [ "$(COMPONENT)" = "shell" ]; then \
		$(MAKE) manifest-shell-image; \
	elif [ "$(COMPONENT)" = "kubectl-retina" ]; then \
		$(MAKE) manifest-kubectl-retina-image; \
	fi

.PHONY: manifests
manifests: ## Create multiplatform manifests
	cd crd && make manifests && make generate

##########################################
##@ Tests
##########################################

COVER_PKG ?= .

# Make sure the layer has only one directory.
# The test DockerFile needs to build the scratch stage with all the output files 
# and we will untar the archive and copy the files from scratch stage
test-image: ## Build the retina container image for testing
	$(MAKE) container-docker \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=./test/image/Dockerfile \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(RETINA_IMAGE) \
			CONTEXT_DIR=$(REPO_ROOT) \
			TAG=$(RETINA_PLATFORM_TAG)

test: $(ENVTEST) ## Run unit tests
	go build -o test-summary ./test/utsummary/main.go
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use -p path)" go test -tags=unit,dashboard -skip=TestE2E* -coverprofile=coverage.out -v -json ./... | ./test-summary --progress --verbose

.PHONY: run-perf-test
run-perf-test: ## Run performance tests
	go test -v ./test/e2e/retina_perf_test.go -timeout 2h -tags=perf -count=1  -args -image-tag=${TAG} -image-registry=${IMAGE_REGISTRY} -image-namespace=${IMAGE_NAMESPACE}

coverage: ## Code coverage
#	go generate ./... && go test -tags=unit -coverprofile=coverage.out.tmp ./...
	cat coverage.out | grep -v "_bpf.go\|_bpfel_x86.go\|_bpfel_arm64.go|_generated.go|mock_" | grep -v mock > coveragenew.out
	go tool cover -html coveragenew.out -o coverage.html
	go tool cover -func=coveragenew.out -o coverageexpanded.out
	ls -al
	rm coverage.out
	mv coveragenew.out coverage.out
	if [ "$(GIT_CURRENT_BRANCH_NAME)" != "main" ]; then \
		python3 scripts/coverage/get_coverage.py; \
		go tool cover -func=mainbranchcoverage/coverage.out -o maincoverageexpanded.out; \
		python3 scripts/coverage/compare_cov.py; \
	fi;

##########################################
##@ Docs
##########################################

.PHONY: docs
docs: ## Generate docs and host on localhost:3000
	echo $(PWD)
	docker run -it -p 3000:3000 -v $(PWD):/retina -w /retina/ node:20-alpine sh ./site/start-dev.sh

.PHONY: docs-prod
docs-prod: ## Generate a production build of the docs
	docker run -i -p 3000:3000 -v $(PWD):/retina -w /retina/ node:20-alpine npm install --prefix site && npm run build --prefix site

.PHONY: docs-prod-serve
docs-prod-serve: ## Serve a production build of the docs on localhost:3000
	cd site && npm run serve

##########################################
##@ Helm
##########################################

# Fetch the latest tag from the GitHub
LATEST_TAG := $(shell curl -s https://api.github.com/repos/microsoft/retina/releases | jq -r '.[0].name')
HELM_IMAGE_TAG ?= $(LATEST_TAG)

helm-install: manifests ## Basic / node-level mode
	helm upgrade --install retina ./deploy/standard/manifests/controller/helm/retina/ \
		--namespace kube-system \
		--set image.repository=$(IMAGE_REGISTRY)/$(RETINA_IMAGE) \
		--set image.initRepository=$(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE) \
		--set image.tag=$(HELM_IMAGE_TAG) \
		--set operator.tag=$(HELM_IMAGE_TAG) \
		--set image.pullPolicy=Always \
		--set logLevel=info \
		--set os.windows=true \
		--set operator.enabled=false \
		--set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\]"

helm-install-with-operator: manifests ## Basic / node-level mode with operator
	helm upgrade --install retina ./deploy/standard/manifests/controller/helm/retina/ \
		--namespace kube-system \
		--set image.repository=$(IMAGE_REGISTRY)/$(RETINA_IMAGE) \
		--set image.initRepository=$(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE) \
		--set image.tag=$(HELM_IMAGE_TAG) \
		--set operator.tag=$(HELM_IMAGE_TAG) \
		--set image.pullPolicy=Always \
		--set logLevel=info \
		--set os.windows=true \
		--set operator.enabled=true \
		--set operator.enableRetinaEndpoint=true \
		--set operator.repository=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE) \
		--skip-crds \
		--set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\,packetparser\]"

helm-install-advanced-remote-context: manifests ## Advanced / pod-level mode with scale limitations, where metrics are aggregated by source and destination Pod
	helm upgrade --install retina ./deploy/standard/manifests/controller/helm/retina/ \
		--namespace kube-system \
		--set image.repository=$(IMAGE_REGISTRY)/$(RETINA_IMAGE) \
		--set image.initRepository=$(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE) \
		--set image.tag=$(HELM_IMAGE_TAG) \
		--set operator.tag=$(HELM_IMAGE_TAG) \
		--set image.pullPolicy=Always \
		--set logLevel=info \
		--set os.windows=true \
		--set operator.enabled=true \
		--set operator.enableRetinaEndpoint=true \
		--set operator.repository=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE) \
		--skip-crds \
		--set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\,packetparser\]" \
		--set enablePodLevel=true \
		--set remoteContext=true

helm-install-advanced-local-context: manifests ## Advanced / pod-level mode designed for scale, where metrics are aggregated by "local" Pod (source for outgoing traffic, destination for incoming traffic)
	helm upgrade --install retina ./deploy/standard/manifests/controller/helm/retina/ \
		--namespace kube-system \
		--set image.repository=$(IMAGE_REGISTRY)/$(RETINA_IMAGE) \
		--set image.initRepository=$(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE) \
		--set image.tag=$(HELM_IMAGE_TAG) \
		--set operator.tag=$(HELM_IMAGE_TAG) \
		--set image.pullPolicy=Always \
		--set logLevel=info \
		--set os.windows=true \
		--set operator.enabled=true \
		--set operator.enableRetinaEndpoint=true \
		--set operator.repository=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE) \
		--skip-crds \
		--set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\,packetparser\]" \
		--set enablePodLevel=true \
		--set enableAnnotations=true

helm-install-hubble: ## Install Hubble
	helm upgrade --install retina ./deploy/hubble/manifests/controller/helm/retina/ \
		--namespace kube-system \
		--set os.windows=true \
		--set operator.enabled=true \
		--set operator.repository=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE) \
		--set operator.tag=$(HELM_IMAGE_TAG) \
		--set agent.enabled=true \
		--set agent.repository=$(IMAGE_REGISTRY)/$(RETINA_IMAGE) \
		--set agent.tag=$(HELM_IMAGE_TAG) \
		--set agent.init.enabled=true \
		--set agent.init.repository=$(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE) \
		--set agent.init.tag=$(HELM_IMAGE_TAG) \
		--set logLevel=info \
		--set hubble.tls.enabled=$(ENABLE_TLS) \
		--set hubble.relay.tls.server.enabled=$(ENABLE_TLS) \
		--set hubble.tls.auto.enabled=$(ENABLE_TLS) \
		--set hubble.tls.auto.method=cronJob \
		--set hubble.tls.auto.certValidityDuration=1 \
		--set hubble.tls.auto.schedule="*/10 * * * *"	

helm-install-without-tls: clean-certs ## Install Hubble without TLS
	$(MAKE) helm-install-hubble ENABLE_TLS=false

helm-uninstall: ## Uninstall Retina with Helm
	helm uninstall retina -n kube-system

.PHONY: clean-certs
clean-certs: ## Clean certs
	rm -rf $(CERT_DIR)
	$(foreach kv,$(CERT_FILES),\
		$(eval CONFIG_KEY=$(word 2,$(subst :, ,$(kv)))) \
		hubble config reset $(CONFIG_KEY);\
	)
	hubble config set tls false
	hubble config reset tls-server-name

.PHONY: get-certs
get-certs: ## Get certs
	mkdir -p $(CERT_DIR)
	$(foreach kv,$(CERT_FILES),\
			$(eval FILE=$(word 1,$(subst :, ,$(kv)))) \
			$(eval CONFIG_KEY=$(word 2,$(subst :, ,$(kv)))) \
			kubectl get secret $(TLS_SECRET_NAME) \
				-n kube-system \
				-o jsonpath="{.data['$(call escape_dot,$(FILE))']}" \
			| base64 -d > $(CERT_DIR)/$(FILE);\
			hubble config set $(CONFIG_KEY) $(CERT_DIR)/$(FILE);\
		)
	hubble config set tls true
	hubble config set tls-server-name instance.hubble-relay.cilium.io

# Replaces every '.' in $(1) with '\.'
escape_dot = $(subst .,\.,$(1))
