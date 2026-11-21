.DEFAULT_GOAL := help

# Default platform commands
RMDIR := rm -rf

## Globals
GIT_CURRENT_BRANCH_NAME	:= $(shell git rev-parse --abbrev-ref HEAD) 

REPO_ROOT = $(shell git rev-parse --show-toplevel)
ifndef TAG
	TAG ?= $(shell git describe --tags --always)
endif
OUTPUT_DIR = $(REPO_ROOT)/output
ARTIFACTS_DIR = $(REPO_ROOT)/artifacts
OUTPUT_LOCAL ?= --output type=local,dest=$(ARTIFACTS_DIR)
BUILD_DIR = $(OUTPUT_DIR)/$(GOOS)_$(GOARCH)
RETINA_BUILD_DIR = $(BUILD_DIR)/retina
RETINA_DIR = $(REPO_ROOT)/controller
OPERATOR_DIR=$(REPO_ROOT)/operator
CAPTURE_WORKLOAD_DIR = $(REPO_ROOT)/captureworkload

KIND = /usr/local/bin/kind
KIND_CLUSTER = retina-cluster
WINVER2022   ?= "10.0.20348.1906"
APP_INSIGHTS_ID ?= ""
AGENT_IMAGE_NAME ?= ""
GENERATE_TARGET_DIRS = \
	./pkg/plugin/linuxutil

# Set agent registry to get image from when using retina-kubectl
ifneq ($(AGENT_IMAGE_NAME), "")
	EXTRA_BUILD_ARGS := "--build-arg AGENT_IMAGE_NAME=$(AGENT_IMAGE_NAME)"
endif

# Default platform is linux/amd64
GOOS			?= linux
GOARCH			?= amd64
OS				?= $(GOOS)
ARCH			?= $(GOARCH)
PLATFORM		?= $(OS)/$(ARCH)
PLATFORMS		?= linux/amd64 linux/arm64 windows/amd64
OS_VERSION		?= ltsc2022

CONTAINER_BUILDER ?= docker
CONTAINER_RUNTIME ?= docker
# Capture local-build selections before applying single-image packaging defaults.
WINDOWS_LOCAL_YEARS := $(or $(YEAR),$(WINDOWS_YEARS),2022 2025)
YEAR 			  ?= 2022

ALL_ARCH.linux = amd64 arm64
ALL_ARCH.windows = amd64

#######
# TLS #
#######
ENABLE_TLS ?= true
CERT_DIR := $(REPO_ROOT)/.certs

CERT_FILES := tls.crt:tls-client-cert-file \
              tls.key:tls-client-key-file \
              ca.crt:tls-ca-cert-files

# TAG is OS and platform agonstic, which can be used for binary version and image manifest tag,
# while RETINA_PLATFORM_TAG is platform specific, which can be used for image built for specific platforms.
RETINA_PLATFORM_TAG        ?= $(TAG)-$(subst /,-,$(PLATFORM))

# used for looping through components in container build
AGENT_TARGETS ?= init agent

WINDOWS_YEARS ?= 2022
WINDOWS_DOCKER ?= docker
REPO_PATH ?= .
BINARIES_PATH ?= output/windows_$(GOARCH)
WINDOWS_LOCAL_BINARIES ?= output/local/windows_$(GOARCH)
WINDOWS_LOCAL_TAG = $(TAG)-local-windows-ltsc$(YEAR)-amd64
WINDOWS_LOCAL_ARCHIVE ?= $(ARTIFACTS_DIR)/retina-agent-$(WINDOWS_LOCAL_TAG).tar

# for windows os, add year to the platform tag
ifeq ($(OS),windows)
RETINA_PLATFORM_TAG        = $(TAG)-windows-ltsc$(YEAR)-amd64
endif

qemu-user-static: ## Set up the host to run qemu multiplatform container builds.
	sudo $(CONTAINER_RUNTIME) run --rm --privileged tonistiigi/binfmt --install arm64

.PHONY: version
version: ## prints the root version
	@if [ "$(shell git tag --points-at HEAD)" != "" ]; then \
		export VERSION="$$(git tag --points-at HEAD)"; \
	else \
		export VERSION="$$(git rev-parse --short HEAD)"; \
	fi; \
	echo "$${VERSION}"

##@ Help 

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


##@ Tools 

GOFUMPT			= go tool mvdan.cc/gofumpt
GOLANGCI_LINT	= go tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint
CONTROLLER_GEN	= go tool sigs.k8s.io/controller-tools/cmd/controller-gen
GINKGO			= go tool github.com/onsi/ginkgo
MOCKGEN			= go tool go.uber.org/mock/mockgen
ENVTEST			= go tool sigs.k8s.io/controller-runtime/tools/setup-envtest

gofumpt: $(GOFUMPT) ## Build gofumpt

golangci-lint: $(GOLANGCI_LINT) ## Build golangci-lint

controller-gen: $(CONTROLLER_GEN) ## Build controller-gen

ginkgo: $(GINKGO) ## Build ginkgo

mockgen: $(MOCKGEN) ## Build mockgen

setup-envtest: $(ENVTEST)

all: generate

generate: generate-bpf-go
	go generate ./...
	for dir in $(GENERATE_TARGET_DIRS); do \
			make -C $$dir $@; \
	done

generate-bpf-go: ## generate ebpf wrappers for plugins for all archs
	for arch in $(ALL_ARCH.linux); do \
        GOARCH=$$arch go generate ./pkg/plugin/...; \
    done
	
.PHONY: all generate generate-bpf-go

##@ Utils 

FMT_PKG  ?= .
LINT_PKG ?= .

empty-bpf-objects: ## truncate all tracked .o files to 0 bytes
	git ls-files '*.o' | xargs truncate -s 0

fmt: ## run gofumpt on $FMT_PKG (default "retina").
	$(GOFUMPT) -w $(FMT_PKG)

lint: ## Fast lint vs default branch showing only new issues.
	$(GOLANGCI_LINT) run --new-from-rev main --timeout 10m -v $(LINT_PKG)/...

lint-existing: ## Lint the current branch in entirety.
	$(GOLANGCI_LINT) run -v $(LINT_PKG)/...

clean: ## clean build artifacts
	$(RMDIR) $(OUTPUT_DIR)

bump-images: ## update all Dockerfile base image digests to latest
	@./scripts/bump-images.sh

##@ Build Binaries

retina: ## builds retina binary
	$(MAKE) retina-binary 

retina-binary: ## build the Retina binary
	go generate ./... && \
	go build -v -o $(RETINA_BUILD_DIR)/retina$(EXE_EXT) -gcflags="-dwarflocationlists=true" -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version=$(TAG) -X github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID=$(APP_INSIGHTS_ID)" $(RETINA_DIR)/main.go

retina-capture-workload: ## build the Retina capture workload
	cd $(CAPTURE_WORKLOAD_DIR) && go build -v -o $(RETINA_BUILD_DIR)/captureworkload$(EXE_EXT) -gcflags="-dwarflocationlists=true"  -ldflags "-X main.version=$(TAG)"

##@ Containers

IMAGE_REGISTRY	?= ghcr.io
IMAGE_NAMESPACE ?= $(shell git config --get remote.origin.url | sed -E 's/.*github\.com[\/:]([^\/]+)\/([^\/.]+)(.git)?/\1\/\2/' | tr '[:upper:]' '[:lower:]')

RETINA_BUILDER_IMAGE			= $(IMAGE_NAMESPACE)/retina-builder
RETINA_TOOLS_IMAGE				= $(IMAGE_NAMESPACE)/retina-tools
RETINA_IMAGE 					= $(IMAGE_NAMESPACE)/retina-agent
RETINA_INIT_IMAGE				= $(IMAGE_NAMESPACE)/retina-init
RETINA_OPERATOR_IMAGE			= $(IMAGE_NAMESPACE)/retina-operator
RETINA_SHELL_IMAGE				= $(IMAGE_NAMESPACE)/retina-shell
KUBECTL_RETINA_IMAGE			= $(IMAGE_NAMESPACE)/kubectl-retina
KUBECTL_RETINA_SHELL_IMAGE		= $(IMAGE_NAMESPACE)/kubectl-retina-shell
RETINA_INTEGRATION_TEST_IMAGE	= $(IMAGE_NAMESPACE)/retina-integration-test
RETINA_PROTO_IMAGE				= $(IMAGE_NAMESPACE)/retina-proto-gen
RETINA_GO_GEN_IMAGE				= $(IMAGE_NAMESPACE)/retina-go-gen
KAPINGER_IMAGE 					= kapinger

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
		

manifest-skopeo-archive: # util target to export tar archive of multiarch container manifest.
	skopeo copy --all docker://$(IMAGE_REGISTRY)/$(IMAGE):$(TAG) oci-archive:$(IMAGE_ARCHIVE_DIR)/$(IMAGE)-$(TAG).tar --debug


buildx:
	if docker buildx inspect retina > /dev/null 2>&1; then \
		echo "Buildx instance retina already exists."; \
	else \
		echo "Creating buildx instance retina..."; \
		docker buildx create --name retina --use --driver-opt image=mcr.microsoft.com/oss/v2/moby/buildkit:v0.16.0-2 --platform $$(echo "$(PLATFORMS)" | tr ' ' ','); \
		docker buildx use retina; \
		echo "Buildx instance retina created."; \
	fi;


container-docker: buildx # util target to build container images using docker buildx. do not invoke directly.
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
		--build-arg VERSION=$(VERSION) $(EXTRA_BUILD_ARGS) \
		--target=$(TARGET) \
		-t $(IMAGE_REGISTRY)/$(IMAGE):$(TAG) \
		$(OUTPUT_LOCAL) \
		$(BUILDX_ACTION) \
		$(CONTEXT_DIR) 

container-docker-windows: # util target to build Windows container images without buildx. do not invoke directly.
	os=$$(echo $(PLATFORM) | cut -d'/' -f1); \
	arch=$$(echo $(PLATFORM) | cut -d'/' -f2); \
	image_name=$$(basename $(IMAGE)); \
	echo "Building $$image_name for $$os/$$arch "; \
	$(WINDOWS_DOCKER) build \
		--platform $(PLATFORM) \
		-f $(DOCKERFILE) \
		--build-arg BUILDPLATFORM=$(PLATFORM) \
		--build-arg APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
		--build-arg GOARCH=$$arch \
		--build-arg GOOS=$$os \
		--build-arg OS_VERSION=$(OS_VERSION) \
		--build-arg VERSION=$(VERSION) \
		--build-arg REPO_PATH=$(REPO_PATH) \
		--build-arg BINARIES_PATH=$(BINARIES_PATH) \
		$(EXTRA_BUILD_ARGS) \
		--target=$(TARGET) \
		-t $(IMAGE_REGISTRY)/$(IMAGE):$(TAG) \
		$(CONTEXT_DIR)

retina-image: ## build the retina linux container image.
	echo "Building for $(PLATFORM)"
	set -e; \
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

.PHONY: make-win check-windows-platform check-windows-local check-windows-build retina-image-win-local retina-image-win build-windows-binaries

make-win: BUILDX_ACTION ?= --push
make-win: ## cross-build and push both development Windows images; set YEAR for one version.
	@if [ "$(words $(WINDOWS_LOCAL_YEARS))" -gt 1 ] && [ -z "$(strip $(BUILDX_ACTION))" ] && [ "$(origin WINDOWS_LOCAL_ARCHIVE)" != "file" ]; then \
		echo "Set YEAR=2022 or YEAR=2025 when overriding WINDOWS_LOCAL_ARCHIVE." >&2; exit 1; \
	fi
	$(MAKE) check-windows-local YEAR="$(WINDOWS_LOCAL_YEARS)"
	$(MAKE) build-windows-binaries BUILD_DIR="$(REPO_ROOT)/$(WINDOWS_LOCAL_BINARIES)" APP_INSIGHTS_ID=
	@set -e; \
	for year in $(WINDOWS_LOCAL_YEARS); do \
		$(MAKE) retina-image-win-local YEAR=$$year BUILDX_ACTION="$(BUILDX_ACTION)"; \
	done

check-windows-platform:
	@set -e; \
	if [ "$(GOARCH)" != "amd64" ]; then \
		echo "Windows agent images support GOARCH=amd64 only." >&2; exit 1; \
	fi; \
	if [ -z "$(strip $(WINDOWS_YEARS))" ]; then \
		echo "WINDOWS_YEARS must contain 2022 or 2025." >&2; exit 1; \
	fi; \
	for year in $(WINDOWS_YEARS); do \
		case "$$year" in \
			2022|2025) ;; \
			*) echo "Unsupported Windows image year: $$year (use 2022 or 2025)." >&2; exit 1 ;; \
		esac; \
	done

check-windows-local:
	$(MAKE) check-windows-platform WINDOWS_YEARS="$(YEAR)"
	@$(WINDOWS_DOCKER) buildx version > /dev/null || { \
		echo "Local Windows assembly requires Docker Buildx." >&2; exit 1; \
	}

retina-image-win-local: check-windows-local ## package local Windows binaries; set BUILDX_ACTION=--push to publish.
	@set -e; \
	for binary in controller.exe captureworkload.exe; do \
		if [ ! -f "$(REPO_ROOT)/$(WINDOWS_LOCAL_BINARIES)/$$binary" ]; then \
			echo "Missing $(WINDOWS_LOCAL_BINARIES)/$$binary. Run make make-win YEAR=$(YEAR)." >&2; exit 1; \
		fi; \
	done; \
	echo "LOCAL DEVELOPMENT ONLY: unsigned binaries, no CVE-2013-3900 registry hardening; not for production." >&2
	@mkdir -p "$(dir $(WINDOWS_LOCAL_ARCHIVE))"
	$(WINDOWS_DOCKER) buildx build \
		--platform windows/amd64 \
		--provenance=false \
		--file hack/windows/Dockerfile.local \
		--build-arg OS_VERSION=ltsc$(YEAR) \
		--build-arg BINARIES_PATH=$(WINDOWS_LOCAL_BINARIES) \
		--tag $(IMAGE_REGISTRY)/$(RETINA_IMAGE):$(WINDOWS_LOCAL_TAG) \
		$(if $(strip $(BUILDX_ACTION)),$(BUILDX_ACTION),--output "type=docker,dest=$(WINDOWS_LOCAL_ARCHIVE)") \
		"$(REPO_ROOT)"
	$(if $(strip $(BUILDX_ACTION)),@echo "Built Windows development image: $(IMAGE_REGISTRY)/$(RETINA_IMAGE):$(WINDOWS_LOCAL_TAG)",@echo "Local Windows image archive: $(WINDOWS_LOCAL_ARCHIVE)")

check-windows-build: check-windows-platform
	@set -e; \
	docker_os=$$($(WINDOWS_DOCKER) info --format '{{.OSType}}') || { \
		echo "Cannot reach Windows Docker. Start a Windows-container daemon or set WINDOWS_DOCKER." >&2; exit 1; \
	}; \
	if [ "$$(printf '%s' "$$docker_os" | tr -d '\r')" != "windows" ]; then \
		echo "Windows images require a Windows Docker daemon; current daemon is $$docker_os. Switch Docker to Windows containers or use a Windows build host." >&2; exit 1; \
	fi

retina-image-win: check-windows-build ## package prebuilt Windows binaries without rebuilding or signing them.
# The Windows images are built on a corresponding Windows host without buildx.
# This is done to mitigate CVE-2013-3900.
	@set -e; \
	for binary in controller.exe captureworkload.exe; do \
		if [ ! -f "$(REPO_ROOT)/$(BINARIES_PATH)/$$binary" ]; then \
			echo "Missing $(BINARIES_PATH)/$$binary. Use make make-win for a local unsigned build, or supply prebuilt binaries via BINARIES_PATH." >&2; exit 1; \
		fi; \
	done
	set -e; \
	for year in $(WINDOWS_YEARS); do \
		tag=$(TAG)-windows-ltsc$$year-amd64; \
		echo "Building $$tag"; \
		set -e ; \
		$(MAKE) container-docker-windows \
				PLATFORM=windows/amd64 \
				DOCKERFILE=controller/Dockerfile.windows-$$year \
				REGISTRY=$(IMAGE_REGISTRY) \
				IMAGE=$(RETINA_IMAGE) \
				OS_VERSION=ltsc$$year \
				VERSION=$(TAG) \
				TAG=$$tag \
				TARGET=agent-win \
				REPO_PATH=$(REPO_PATH) \
				BINARIES_PATH=$(BINARIES_PATH) \
				CONTEXT_DIR=$(REPO_ROOT); \
	done

retina-operator-image:  ## build the retina linux operator image.
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

retina-shell-image:
	echo "Building for $(PLATFORM)"
	set -e ; \
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=shell/Dockerfile \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(RETINA_SHELL_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			OUTPUT_LOCAL= \
			CONTEXT_DIR=$(REPO_ROOT)

kubectl-retina-image:
	echo "Building for $(PLATFORM)"
	set -e ; \
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=cli/Dockerfile \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(KUBECTL_RETINA_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			CONTEXT_DIR=$(REPO_ROOT) \
			EXTRA_BUILD_ARGS=$(EXTRA_BUILD_ARGS)

kubectl-retina-shell-image:
	echo "Building shell-enabled kubectl-retina for $(PLATFORM)"
	set -e ; \
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=cli/Dockerfile \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(KUBECTL_RETINA_SHELL_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
		CONTEXT_DIR=$(REPO_ROOT) \
		TARGET=shell-target \
		EXTRA_BUILD_ARGS=$(EXTRA_BUILD_ARGS)

kapinger-image: 
	docker buildx build --builder retina --platform windows/amd64 --target windows-amd64 -t $(IMAGE_REGISTRY)/$(KAPINGER_IMAGE):$(TAG)-windows-amd64  ./hack/tools/kapinger/ --push
	docker buildx build --builder retina --platform linux/amd64 --target linux-amd64 -t $(IMAGE_REGISTRY)/$(KAPINGER_IMAGE):$(TAG)-linux-amd64  ./hack/tools/kapinger/ --push
	docker buildx build --builder retina --platform linux/arm64 --target linux-arm64 -t $(IMAGE_REGISTRY)/$(KAPINGER_IMAGE):$(TAG)-linux-arm64  ./hack/tools/kapinger/ --push
	docker buildx imagetools create -t $(IMAGE_REGISTRY)/$(KAPINGER_IMAGE):$(TAG) \
		$(IMAGE_REGISTRY)/$(KAPINGER_IMAGE):$(TAG)-windows-amd64 \
		$(IMAGE_REGISTRY)/$(KAPINGER_IMAGE):$(TAG)-linux-amd64 \
		$(IMAGE_REGISTRY)/$(KAPINGER_IMAGE):$(TAG)-linux-arm64

toolbox: 
	docker buildx build --builder retina --platform linux/amd64  -t $(IMAGE_REGISTRY)/toolbox:$(TAG)   -f ./hack/tools/toolbox/Dockerfile ./hack/tools/ --push

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

build-windows-binaries: BUILD_DIR = $(OUTPUT_DIR)/windows_$(GOARCH)
build-windows-binaries: ## Build Windows binaries
	@echo "Building Windows binaries for $(GOARCH)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=$(GOARCH) go build -v \
		-o $(BUILD_DIR)/captureworkload.exe \
		-ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version=$(TAG) \
		-X github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID=$(APP_INSIGHTS_ID)" \
		$(CAPTURE_WORKLOAD_DIR)/main.go
	CGO_ENABLED=0 GOOS=windows GOARCH=$(GOARCH) go build -v \
		-o $(BUILD_DIR)/controller.exe \
		-ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version=$(TAG) \
		-X github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID=$(APP_INSIGHTS_ID)" \
		$(RETINA_DIR)/main.go
	@echo "Windows binaries built successfully in $(BUILD_DIR)"

##@ Multiplatform

# Command that creates a multi-arch index from per-platform image references.
# The Azure DevOps pipeline has no docker CLI and passes hack/scripts/oras-index.sh.
# Plain Buildx can omit Windows os.version from index descriptors. docker-index.sh
# restores it from image configs so runtimes can distinguish WS2022 from WS2025.
# Use the helper only for Windows-containing agent manifests; explicit overrides
# and Linux-only manifests retain MANIFEST_CREATE.
ifdef MANIFEST_CREATE
WINDOWS_MANIFEST_CREATE ?= $(MANIFEST_CREATE)
else
WINDOWS_MANIFEST_CREATE ?= bash hack/scripts/docker-index.sh
endif
MANIFEST_CREATE ?= docker buildx imagetools create -t
MANIFEST_TAG ?= $(TAG)
LINUX_PLATFORMS ?= linux/amd64 linux/arm64
WINDOWS_TAG_PREFIX ?= $(TAG)

manifest-retina-image: ## create a multiplatform manifest for the retina image
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_IMAGE))
	$(eval FULL_INIT_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE))
	$(if $(strip $(WINDOWS_YEARS)),$(WINDOWS_MANIFEST_CREATE),$(MANIFEST_CREATE)) $(FULL_IMAGE_NAME):$(MANIFEST_TAG) $(foreach platform,$(LINUX_PLATFORMS), $(FULL_IMAGE_NAME):$(TAG)-$(subst /,-,$(platform))) $(foreach year,$(WINDOWS_YEARS), $(FULL_IMAGE_NAME):$(WINDOWS_TAG_PREFIX)-windows-ltsc$(year)-amd64)
	$(MANIFEST_CREATE) $(FULL_INIT_IMAGE_NAME):$(MANIFEST_TAG) $(foreach platform,$(LINUX_PLATFORMS), $(FULL_INIT_IMAGE_NAME):$(TAG)-$(subst /,-,$(platform)))

manifest-operator-image: ## create a multiplatform manifest for the operator image
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE))
	$(MANIFEST_CREATE) $(FULL_IMAGE_NAME):$(MANIFEST_TAG) $(foreach platform,$(LINUX_PLATFORMS), $(FULL_IMAGE_NAME):$(TAG)-$(subst /,-,$(platform)))

manifest-shell-image:
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_SHELL_IMAGE))
	$(MANIFEST_CREATE) $(FULL_IMAGE_NAME):$(MANIFEST_TAG) $(foreach platform,$(LINUX_PLATFORMS), $(FULL_IMAGE_NAME):$(TAG)-$(subst /,-,$(platform)))

manifest-kubectl-retina-image:
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(KUBECTL_RETINA_IMAGE))
	$(MANIFEST_CREATE) $(FULL_IMAGE_NAME):$(MANIFEST_TAG) $(foreach platform,$(LINUX_PLATFORMS), $(FULL_IMAGE_NAME):$(TAG)-$(subst /,-,$(platform)))

manifest-kubectl-retina-shell-image:
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(KUBECTL_RETINA_SHELL_IMAGE))
	$(MANIFEST_CREATE) $(FULL_IMAGE_NAME):$(MANIFEST_TAG) $(foreach platform,$(LINUX_PLATFORMS), $(FULL_IMAGE_NAME):$(TAG)-$(subst /,-,$(platform)))

manifest:
	echo "Building for $(COMPONENT)"
	if [ "$(COMPONENT)" = "retina" ]; then \
		$(MAKE) manifest-retina-image; \
	elif [ "$(COMPONENT)" = "operator" ]; then \
		$(MAKE) manifest-operator-image; \
	elif [ "$(COMPONENT)" = "shell" ]; then \
		$(MAKE) manifest-shell-image; \
	elif [ "$(COMPONENT)" = "kubectl-retina" ]; then \
		$(MAKE) manifest-kubectl-retina-image; \
	elif [ "$(COMPONENT)" = "kubectl-retina-shell" ]; then \
		$(MAKE) manifest-kubectl-retina-shell-image; \
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

.PHONY: test
test: # Run unit tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use -p path)" go test -tags=unit,dashboard -skip=TestE2E* -coverprofile=coverage.out -v ./...

.PHONY: test-windows
test-windows: # Run unit tests on Windows.
	go test -tags=unit,dashboard -skip=TestE2E* -count=1 -v ./...

.PHONY: test-ebpf
test-ebpf: # Run eBPF program tests (requires root/CAP_BPF).
	sudo $$(which go) test -tags=ebpf -v -count=1 ./pkg/plugin/...

coverage: # Code coverage.
#	go generate ./... && go test -tags=unit -coverprofile=coverage.out.tmp ./...
	cat coverage.out | grep -Ev '_bpf\.go|_bpfel_x86\.go|_bpfel_arm64\.go|_generated\.go|mock_' > coveragenew.out
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

# Fetch the latest tag from the GitHub
LATEST_TAG := $(shell curl -s https://api.github.com/repos/microsoft/retina/releases/latest | jq -r '.name')

HELM_IMAGE_TAG ?= $(LATEST_TAG)

# basic/node-level mode
helm-install: manifests
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

helm-install-with-operator: manifests
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

# advanced/pod-level mode with scale limitations, where metrics are aggregated by source and destination Pod
helm-install-advanced-remote-context: manifests
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

# advanced/pod-level mode designed for scale, where metrics are aggregated by "local" Pod (source for outgoing traffic, destination for incoming traffic)
helm-install-advanced-local-context: manifests
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
		--set operator.enableRetinaEndpoint=false \
		--set operator.repository=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE) \
		--skip-crds \
		--set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\,packetparser\]" \
		--set enablePodLevel=true \
		--set enableAnnotations=true

helm-install-hubble:
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

helm-install-without-tls: clean-certs
	$(MAKE) helm-install-hubble ENABLE_TLS=false

helm-uninstall:
	helm uninstall retina -n kube-system

.PHONY: get-certs
get-certs:
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

.PHONY: clean-certs
clean-certs:
	rm -rf $(CERT_DIR)
	$(foreach kv,$(CERT_FILES),\
		$(eval CONFIG_KEY=$(word 2,$(subst :, ,$(kv)))) \
		hubble config reset $(CONFIG_KEY);\
	)
	hubble config set tls false
	hubble config reset tls-server-name

.PHONY: docs
docs: 
	echo $(PWD)
	docker run -it -p 3000:3000 -v $(PWD):/retina -w /retina/ node:20-alpine sh ./site/start-dev.sh

.PHONY: docs-prod
docs-prod:
	docker run -i -p 3000:3000 -v $(PWD):/retina -w /retina/ node:20-alpine npm install --prefix site && npm run build --prefix site

.PHONY: quick-build
quick-build:
	$(MAKE) retina-image PLATFORM=linux/amd64 BUILDX_ACTION=--push
	$(MAKE) retina-operator-image PLATFORM=linux/amd64 BUILDX_ACTION=--push

.PHONY: quick-deploy
quick-deploy:
	$(MAKE) helm-install-advanced-local-context HELM_IMAGE_TAG=$(TAG)-linux-amd64

.PHONY: quick-deploy-hubble
quick-deploy-hubble:
	$(MAKE) helm-uninstall || true
	$(MAKE) helm-install-without-tls HELM_IMAGE_TAG=$(TAG)-linux-amd64

.PHONY: simplify-dashboards
simplify-dashboards:
	cd deploy/testutils && go test ./... -tags=dashboard,simplifydashboard -v && cd $(REPO_ROOT)

run-perf-test:
	go test -v ./test/e2e/retina_perf_test.go -timeout 2h -tags=perf -count=1  -args -image-tag=${TAG} -image-registry=${IMAGE_REGISTRY} -image-namespace=${IMAGE_NAMESPACE}

run-e2e-test:
	go test -v ./test/e2e/ -timeout 1h -tags=e2e -count=1  -args -image-tag=${TAG} -image-registry=${IMAGE_REGISTRY} -image-namespace=${IMAGE_NAMESPACE}
