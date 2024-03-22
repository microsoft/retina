.DEFAULT_GOAL := help

# Default platform commands
RMDIR := rm -rf

## Globals

REPO_ROOT = $(shell git rev-parse --show-toplevel)
ifndef TAG
	TAG ?= $(shell git describe --tags --always)
endif
OUTPUT_DIR = $(REPO_ROOT)/output
BUILD_DIR = $(OUTPUT_DIR)/$(GOOS)_$(GOARCH)
RETINA_BUILD_DIR = $(BUILD_DIR)/retina
RETINA_DIR = $(REPO_ROOT)/controller
KUBECTL_RETINA_BUILD_DIR = $(OUTPUT_DIR)/kubectl-retina
OPERATOR_DIR=$(REPO_ROOT)/operator
CAPTURE_WORKLOAD_DIR = $(REPO_ROOT)/captureworkload

KIND = /usr/local/bin/kind
KIND_CLUSTER = retina-cluster
WINVER2022   ?= "10.0.20348.1906"
WINVER2019   ?= "10.0.17763.4737"
APP_INSIGHTS_ID ?= ""
GENERATE_TARGET_DIRS = \
	./pkg/plugin/linuxutil

DESTINATION ?= local

# Default platform is linux/amd64
GOOS			?= linux
GOARCH			?= amd64
OS				?= $(GOOS)
ARCH			?= $(GOARCH)
PLATFORM		?= $(OS)/$(ARCH)
PLATFORMS		?= linux/amd64 linux/arm64 windows/amd64

CONTAINER_BUILDER ?= docker
CONTAINER_RUNTIME ?= docker
YEAR 			  ?= 2022

ALL_ARCH.linux = amd64 arm64
ALL_ARCH.windows = amd64

# TAG is OS and platform agonstic, which can be used for binary version and image manifest tag,
# while RETINA_PLATFORM_TAG is platform specific, which can be used for image built for specific platforms.
RETINA_PLATFORM_TAG        ?= $(TAG)-$(subst /,-,$(PLATFORM))

# for windows os, add year to the platform tag
ifeq ($(OS),windows)
RETINA_PLATFORM_TAG        = $(TAG)-windows-ltsc$(YEAR)-amd64
endif

qemu-user-static: ## Set up the host to run qemu multiplatform container builds.
	sudo $(CONTAINER_RUNTIME) run --rm --privileged multiarch/qemu-user-static --reset -p yes

version: ## prints the root version
	@echo $(TAG)

##@ Help 

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


##@ Tools 

TOOLS_DIR     = $(REPO_ROOT)/hack/tools
TOOLS_BIN_DIR = $(TOOLS_DIR)/bin
GOFUMPT       := $(TOOLS_BIN_DIR)/gofumpt
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/golangci-lint
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/controller-gen
GINKGO 		  := $(TOOLS_BIN_DIR)/ginkgo
MOCKGEN         := $(TOOLS_BIN_DIR)/mockgen
ENVTEST         := $(TOOLS_BIN_DIR)/setup-envtest
GIT_CURRENT_BRANCH_NAME := $(shell git rev-parse --abbrev-ref HEAD) 

$(TOOLS_DIR)/go.mod:
	cd $(TOOLS_DIR); go mod init && go mod tidy

$(GOFUMPT): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go mod download; go build -tags=tools -o bin/gofumpt mvdan.cc/gofumpt

gofumpt: $(GOFUMPT) ## Build gofumpt

$(GOLANGCI_LINT): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go mod download; go build -tags=tools -o bin/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint

golangci-lint: $(GOLANGCI_LINT) ## Build golangci-lint

$(CONTROLLER_GEN): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go mod download; go build -tags=tools -o bin/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

controller-gen: $(CONTROLLER_GEN) ## Build controller-gen

$(GINKGO): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go mod download; go build -tags=tools -o bin/ginkgo github.com/onsi/ginkgo/ginkgo

ginkgo: $(GINKGO) ## Build ginkgo

$(MOCKGEN): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go mod download; go build -tags=$(TOOL_TAG) -o bin/mockgen github.com/golang/mock/mockgen

mockgen: $(MOCKGEN) ## Build mockgen

$(ENVTEST): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go mod download; go build -tags=$(TOOL_TAG) -o bin/setup-envtest sigs.k8s.io/controller-runtime/tools/setup-envtest

setup-envtest: $(ENVTEST)

all: generate

generate: generate-bpf-go
	CGO_ENABLED=0 go generate ./...
	for dir in $(GENERATE_TARGET_DIRS); do \
			make -C $$dir $@; \
	done

generate-bpf-go: ## generate ebpf wrappers for plugins for all archs
	for arch in $(ALL_ARCH.linux); do \
        CGO_ENABLED=0 GOARCH=$$arch go generate ./pkg/plugin/...; \
    done
	
.PHONY: all generate generate-bpf-go

##@ Utils 

FMT_PKG  ?= .
LINT_PKG ?= .

fmt: $(GOFUMPT) ## run gofumpt on $FMT_PKG (default "retina").
	$(GOFUMPT) -w $(FMT_PKG)

lint: $(GOLANGCI_LINT) ## Fast lint vs default branch showing only new issues.
	$(GOLANGCI_LINT) run --new-from-rev main --timeout 10m -v $(LINT_PKG)/...

lint-existing: $(GOLANGCI_LINT) ## Lint the current branch in entirety.
	$(GOLANGCI_LINT) run -v $(LINT_PKG)/...

clean: ## clean build artifacts
	$(RMDIR) $(OUTPUT_DIR)

##@ Build Binaries

retina: ## builds both retina and kapctl binaries
	$(MAKE) retina-binary kubectl-retina

retina-binary: ## build the Retina binary
	go generate ./...
	export CGO_ENABLED=0
	go build -v -o $(RETINA_BUILD_DIR)/retina$(EXE_EXT) -gcflags="-dwarflocationlists=true" -ldflags "-X main.version=$(TAG) -X main.applicationInsightsID=$(APP_INSIGHTS_ID)" $(RETINA_DIR)/main.go

kubectl-retina-binary-%: ## build kubectl plugin locally.
	export CGO_ENABLED=0 && \
	export GOOS=$(shell echo $* |cut -f1 -d-) GOARCH=$(shell echo $* |cut -f2 -d-) && \
	go build -v \
		-o $(KUBECTL_RETINA_BUILD_DIR)/kubectl-retina-$${GOOS}-$${GOARCH} \
		-gcflags="-dwarflocationlists=true" \
		-ldflags "-X github.com/microsoft/retina/cli/cmd.Version=$(TAG)" \
		github.com/microsoft/retina/cli

retina-capture-workload: ## build the Retina capture workload
	cd $(CAPTURE_WORKLOAD_DIR) && CGO_ENABLED=0 go build -v -o $(RETINA_BUILD_DIR)/captureworkload$(EXE_EXT) -gcflags="-dwarflocationlists=true"  -ldflags "-X main.version=$(TAG)"

##@ Containers

ifeq ($(DESTINATION), acr)
IMAGE_REGISTRY	?= acnpublic.azurecr.io
else
IMAGE_REGISTRY ?= ghcr.io 
endif

IMAGE_NAMESPACE ?= $(shell git config --get remote.origin.url | sed -E 's/.*github\.com[\/:]([^\/]+)\/([^\/.]+)(.git)?/\1\/\2/' | tr '[:upper:]' '[:lower:]')

RETINA_BUILDER_IMAGE			= $(IMAGE_NAMESPACE)/retina-builder
RETINA_TOOLS_IMAGE				= $(IMAGE_NAMESPACE)/retina-tools
RETINA_IMAGE 					= $(IMAGE_NAMESPACE)/retina-agent
RETINA_INIT_IMAGE				= $(IMAGE_NAMESPACE)/retina-init
RETINA_OPERATOR_IMAGE			= $(IMAGE_NAMESPACE)/retina-operator
RETINA_INTEGRATION_TEST_IMAGE	= $(IMAGE_NAMESPACE)/retina-integration-test
RETINA_PROTO_IMAGE				= $(IMAGE_NAMESPACE)/retina-proto-gen
RETINA_GO_GEN_IMAGE				= $(IMAGE_NAMESPACE)/retina-go-gen
KAPINGER_IMAGE 					= $(IMAGE_NAMESPACE)/kapinger

skopeo-export: # util target to copy a container from containers-storage to the docker daemon.
	skopeo copy \
		containers-storage:$(REF) \
		docker-daemon:$(REF)
		

container-push: # util target to publish container image. do not invoke directly.
	$(CONTAINER_BUILDER) push \
		$(IMAGE_REGISTRY)/$(IMAGE):$(TAG)

container-pull: # util target to pull container image.
	$(CONTAINER_BUILDER) pull \
		$(IMAGE_REGISTRY)/$(IMAGE):$(TAG)

retina-skopeo-export: 
	$(MAKE) skopeo-export \
		REF=$(IMAGE_REGISTRY)/$(RETINA_IMAGE):$(RETINA_PLATFORM_TAG) \
		IMG=$(RETINA_IMAGE)
		TAG=$(RETINA_PLATFORM_TAG)

buildx:
	if docker buildx inspect retina > /dev/null 2>&1; then \
		echo "Buildx instance retina already exists."; \
	else \
		echo "Creating buildx instance retina..."; \
		docker buildx create --name retina --use --platform $$(echo "$(PLATFORMS)" | tr ' ' ','); \
		docker buildx use retina; \
		echo "Buildx instance retina created."; \
	fi;

container-docker: buildx # util target to build container images using docker buildx. do not invoke directly.
	os=$$(echo $(PLATFORM) | cut -d'/' -f1); \
	arch=$$(echo $(PLATFORM) | cut -d'/' -f2); \
	echo "Building for $$os/$$arch"; \
	if [ "$(DESTINATION)" = "acr" ]; then \
		docker buildx build \
			$(BUILDX_ACTION) \
			--platform $(PLATFORM) \
			-t $(IMAGE_REGISTRY)/$(IMAGE):$(TAG) \
			-f $(DOCKERFILE) \
			--build-arg VERSION=$(VERSION) $(EXTRA_BUILD_ARGS) \
			--build-arg GOOS=$$os \
			--build-arg GOARCH=$$arch \
			--build-arg APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
			--target=$(TARGET) \
			--push \
			$(CONTEXT_DIR); \
	else \
		docker buildx build \
			$(BUILDX_ACTION) \
			--platform $(PLATFORM) \
			-f $(DOCKERFILE) \
			--build-arg VERSION=$(VERSION) $(EXTRA_BUILD_ARGS) \
			--build-arg GOOS=$$os \
			--build-arg GOARCH=$$arch \
			--build-arg APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
			--target=$(TARGET) \
			$(CONTEXT_DIR); \
	fi;

retina-image: ## build the retina linux container image.
	echo "Building for $(PLATFORM)"
	for target in init agent; do \
		echo "Building for $$target"; \
		if [ "$$target" = "init" ]; then \
			image_name=$(RETINA_INIT_IMAGE); \
		else \
			image_name=$(RETINA_IMAGE); \
		fi; \
		$(MAKE) container-$(CONTAINER_BUILDER) \
				PLATFORM=$(PLATFORM) \
				DOCKERFILE=controller/Dockerfile.controller \
				REGISTRY=$(IMAGE_REGISTRY) \
				IMAGE=$$image_name \
				VERSION=$(TAG) \
				TAG=$(RETINA_PLATFORM_TAG) \
				APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
				CONTEXT_DIR=$(REPO_ROOT) \
				TARGET=$$target; \
				DESTINATION=$(DESTINATION); \
	done

retina-image-win: ## build the retina Windows container image.
	for year in 2019 2022; do \
		tag=$(TAG)-windows-ltsc$$year-amd64; \
		echo "Building $(RETINA_PLATFORM_TAG)"; \
		$(MAKE) container-$(CONTAINER_BUILDER) \
				PLATFORM=windows/amd64 \
				DOCKERFILE=controller/Dockerfile.windows-$$year \
				REGISTRY=$(IMAGE_REGISTRY) \
				IMAGE=$(RETINA_IMAGE) \
				VERSION=$(TAG) \
				TAG=$$tag \
				CONTEXT_DIR=$(REPO_ROOT); \
				DESTINATION=$(DESTINATION); \
	done

retina-operator-image:  ## build the retina linux operator image.
	echo "Building for $(PLATFORM)"
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=operator/Dockerfile \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(RETINA_OPERATOR_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
			CONTEXT_DIR=$(REPO_ROOT)
			DESTINATION=$(DESTINATION)

kubectl-retina-image: ## build the kubectl-retina image. 
	echo "Building for $(PLATFORM)"
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=cli/Dockerfile.kubectl-retina \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(KUBECTL_RETINA_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
			CONTEXT_DIR=$(REPO_ROOT)
			DESTINATION=$(DESTINATION)

proto-gen: ## generate protobuf code
	docker build --platform=linux/amd64 \
		-t $(IMAGE_REGISTRY)/$(RETINA_PROTO_IMAGE):$(RETINA_PLATFORM_TAG) \
		-f controller/Dockerfile.proto .
	docker run --rm --platform=linux/amd64 \
		--user $(shell id -u):$(shell id -g) \
		-v $(PWD):/app $(IMAGE_REGISTRY)/$(RETINA_PROTO_IMAGE):$(RETINA_PLATFORM_TAG)

go-gen: ## run go generate at the repository root
	docker build -t $(IMAGE_REGISTRY)/$(RETINA_GO_GEN_IMAGE):$(RETINA_PLATFORM_TAG) \
		--build-arg GOOS=$(GOOS) \
		--build-arg GOARCH=$(GOARCH) \
		-f controller/Dockerfile.gogen .
	docker run --rm --user $(shell id -u):$(shell id -g) -v $(PWD):/app $(IMAGE_REGISTRY)/$(RETINA_GO_GEN_IMAGE):$(RETINA_PLATFORM_TAG)

all-gen: ## generate all code
	$(MAKE) proto-gen
	$(MAKE) go-gen

##@ Multiplatform

manifest-retina-image: ## create a multiplatform manifest for the retina image
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_IMAGE):$(TAG))
	$(eval FULL_INIT_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE):$(TAG))
	docker buildx imagetools create -t $(FULL_IMAGE_NAME) $(foreach platform,linux/amd64 linux/arm64 windows-ltsc2019-amd64 windows-ltsc2022-amd64, $(FULL_IMAGE_NAME)-$(subst /,-,$(platform)))
	docker buildx imagetools create -t $(FULL_INIT_IMAGE_NAME) $(foreach platform,linux/amd64 linux/arm64, $(FULL_INIT_IMAGE_NAME)-$(subst /,-,$(platform)))

manifest-operator-image: ## create a multiplatform manifest for the operator image
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE):$(TAG))
	docker buildx imagetools create -t $(FULL_IMAGE_NAME) $(foreach platform,linux/amd64, $(FULL_IMAGE_NAME)-$(subst /,-,$(platform)))

manifest-kubectl-retina-image: ## create a multiplatform manifest for the kubectl-retina image
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(KUBECTL_RETINA_IMAGE):$(TAG))
	docker buildx imagetools create -t $(FULL_IMAGE_NAME) $(foreach platform,linux/amd64 linux/arm64, $(FULL_IMAGE_NAME)-$(subst /,-,$(platform)))

manifest:
	echo "Building for $(COMPONENT)"
	if [ "$(COMPONENT)" = "retina" ]; then \
		$(MAKE) manifest-retina-image; \
	elif [ "$(COMPONENT)" = "operator" ]; then \
		$(MAKE) manifest-operator-image; \
	elif [ "$(COMPONENT)" = "kubectl-retina" ]; then \
		$(MAKE) manifest-kubectl-retina-image; \
	fi

##@ Tests
# Make sure the layer has only one directory.
# the test DockerFile needs to build the scratch stage with all the output files 
# and we will untar the archive and copy the files from scratch stage
test-image: ## build the retina container image for testing.
	$(MAKE) container-docker \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=./test/image/Dockerfile \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(RETINA_IMAGE) \
			CONTEXT_DIR=$(REPO_ROOT) \
			TAG=$(RETINA_PLATFORM_TAG)

COVER_PKG ?= .

test: $(ENVTEST) # Run unit tests.
	go build -o test-summary ./test/utsummary/main.go
	CGO_ENABLED=0 KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use -p path)" go test -tags=unit -skip=TestE2E* -coverprofile=coverage.out -v -json ./... | ./test-summary --progress --verbose

coverage: # Code coverage.
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

## Reusable targets for building multiplat container image manifests.

.PHONY: manifests
manifests: 
	cd crd && make manifests && make generate

HELM_IMAGE_TAG ?= v0.0.1

# basic/node-level mode
helm-install: manifests
	helm upgrade --install retina ./deploy/manifests/controller/helm/retina/ \
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

helm-install-with-operator: manifests
	helm upgrade --install retina ./deploy/manifests/controller/helm/retina/ \
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

# advanced/pod-level mode with scale limitations, where metrics are aggregated by source and destination Pod
helm-install-advanced-remote-context: manifests
	helm upgrade --install retina ./deploy/manifests/controller/helm/retina/ \
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

# advanced/pod-level mode designed for scale, where metrics are aggregated by "local" Pod (source for outgoing traffic, destination for incoming traffic)
helm-install-advanced-local-context: manifests
	helm upgrade --install retina ./deploy/manifests/controller/helm/retina/ \
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

helm-uninstall:
	helm uninstall retina -n kube-system

.PHONY: docs
docs: 
	echo $(PWD)
	docker run -it -p 3000:3000 -v $(PWD):/retina -w /retina/ node:20-alpine ./site/start-dev.sh

.PHONY: docs-pod
docs-prod:
	docker run -i -p 3000:3000 -v $(PWD):/retina -w /retina/ node:20-alpine npm install --prefix site && npm run build --prefix site
