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
BUILD_DIR = $(OUTPUT_DIR)/$(GOOS)_$(GOARCH)
RETINA_BUILD_DIR = $(BUILD_DIR)/retina
RETINA_DIR = $(REPO_ROOT)/controller
OPERATOR_DIR=$(REPO_ROOT)/operator
CAPTURE_WORKLOAD_DIR = $(REPO_ROOT)/captureworkload

KIND = /usr/local/bin/kind
KIND_CLUSTER = retina-cluster
WINVER2022   ?= "10.0.20348.1906"
WINVER2019   ?= "10.0.17763.4737"
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
OS_VERSION		?= ltsc2019

HUBBLE_VERSION ?= v1.18.3

CONTAINER_BUILDER ?= docker
CONTAINER_RUNTIME ?= docker
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

WINDOWS_YEARS ?= "2019 2022"

# for windows os, add year to the platform tag
ifeq ($(OS),windows)
RETINA_PLATFORM_TAG        = $(TAG)-windows-ltsc$(YEAR)-amd64
endif

qemu-user-static: ## Set up the host to run qemu multiplatform container builds.
	sudo $(CONTAINER_RUNTIME) run --rm --privileged multiarch/qemu-user-static --reset -p yes

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
GOLANGCI_LINT	= go tool github.com/golangci/golangci-lint/cmd/golangci-lint
GORELEASER		= go tool github.com/goreleaser/goreleaser
CONTROLLER_GEN	= go tool sigs.k8s.io/controller-tools/cmd/controller-gen
GINKGO			= go tool github.com/onsi/ginkgo
MOCKGEN			= go tool go.uber.org/mock/mockgen
ENVTEST			= go tool sigs.k8s.io/controller-runtime/tools/setup-envtest

gofumpt: $(GOFUMPT) ## Build gofumpt

golangci-lint: $(GOLANGCI_LINT) ## Build golangci-lint

goreleaser: $(GORELEASER) ## Build goreleaser

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

fmt: ## run gofumpt on $FMT_PKG (default "retina").
	$(GOFUMPT) -w $(FMT_PKG)

lint: ## Fast lint vs default branch showing only new issues.
	$(GOLANGCI_LINT) run --new-from-rev main --timeout 10m -v $(LINT_PKG)/...

lint-existing: ## Lint the current branch in entirety.
	$(GOLANGCI_LINT) run -v $(LINT_PKG)/...

clean: ## clean build artifacts
	$(RMDIR) $(OUTPUT_DIR)

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
		--build-arg HUBBLE_VERSION=$(HUBBLE_VERSION) \
		--build-arg VERSION=$(VERSION) $(EXTRA_BUILD_ARGS) \
		--target=$(TARGET) \
		-t $(IMAGE_REGISTRY)/$(IMAGE):$(TAG) \
		--output type=local,dest=$(ARTIFACTS_DIR) \
		$(BUILDX_ACTION) \
		$(CONTEXT_DIR) 


retina-image: ## build the retina linux container image.
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

retina-image-win: ## build the retina Windows container image.
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

build-windows-binaries:
	GOOS=windows GOARCH=$(GOARCH) go build -v -o /go/bin/retina/captureworkload -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version=$(TAG) -X github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID=$(APP_INSIGHTS_ID)" captureworkload/main.go
	GOOS=windows GOARCH=$(GOARCH) go build -x -v -o /go/bin/retina/controller -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version=$(TAG) -X github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID=$(APP_INSIGHTS_ID)" controller/main.go

##@ Multiplatform

manifest-retina-image: ## create a multiplatform manifest for the retina image
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_IMAGE):$(TAG))
	$(eval FULL_INIT_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE):$(TAG))
	docker buildx imagetools create -t $(FULL_IMAGE_NAME) $(foreach platform,linux/amd64 linux/arm64 windows-ltsc2019-amd64 windows-ltsc2022-amd64, $(FULL_IMAGE_NAME)-$(subst /,-,$(platform)))
	docker buildx imagetools create -t $(FULL_INIT_IMAGE_NAME) $(foreach platform,linux/amd64 linux/arm64, $(FULL_INIT_IMAGE_NAME)-$(subst /,-,$(platform)))

manifest-operator-image: ## create a multiplatform manifest for the operator image
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE):$(TAG))
	docker buildx imagetools create -t $(FULL_IMAGE_NAME) $(foreach platform,linux/amd64 linux/arm64, $(FULL_IMAGE_NAME)-$(subst /,-,$(platform)))

manifest-shell-image:
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(RETINA_SHELL_IMAGE):$(TAG))
	docker buildx imagetools create -t $(FULL_IMAGE_NAME) $(foreach platform,linux/amd64 linux/arm64, $(FULL_IMAGE_NAME)-$(subst /,-,$(platform)))

manifest-kubectl-retina-image:
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(KUBECTL_RETINA_IMAGE):$(TAG))
	docker buildx imagetools create -t $(FULL_IMAGE_NAME) $(foreach platform,linux/amd64 linux/arm64, $(FULL_IMAGE_NAME)-$(subst /,-,$(platform)))

manifest-kubectl-retina-shell-image:
	$(eval FULL_IMAGE_NAME=$(IMAGE_REGISTRY)/$(KUBECTL_RETINA_SHELL_IMAGE):$(TAG))
	docker buildx imagetools create -t $(FULL_IMAGE_NAME) $(foreach platform,linux/amd64 linux/arm64, $(FULL_IMAGE_NAME)-$(subst /,-,$(platform)))

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
	go build -o test-summary ./test/utsummary/main.go
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use -p path)" go test -tags=unit,dashboard -skip=TestE2E* -coverprofile=coverage.out -v -json ./... | ./test-summary --progress --verbose

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

.PHONY: update-hubble
update-hubble:
	@echo "Checking for Hubble updates..."
	@latest_version=$$(curl -s https://api.github.com/repos/cilium/hubble/releases/latest | jq -r .tag_name); \
    echo "Latest Hubble version: $$latest_version"; \
    current_version=$$(grep -oP '(?<=ARG HUBBLE_VERSION=).*' controller/Dockerfile); \
    echo "Current Hubble version: $$current_version"; \
    if [ "$$latest_version" = "$$current_version" ]; then \
        echo "Hubble version is up to date. No update needed."; \
    else \
        echo "Updating Hubble version from $$current_version to $$latest_version"; \
        sed -i "s/^ARG HUBBLE_VERSION=.*/ARG HUBBLE_VERSION=$$latest_version/" controller/Dockerfile; \
        sed -i "s/^HUBBLE_VERSION ?=.*/HUBBLE_VERSION ?= $$latest_version/" Makefile; \
        echo ""; \
        echo "Updated Hubble version in controller/Dockerfile and Makefile."; \
        echo "Please create a branch and commit these changes:"; \
        echo "  git checkout -b deps/update-hubble-to-$$latest_version"; \
        echo "  git commit -am \"deps: bump Hubble version from $$current_version to $$latest_version\""; \
        echo "  git push origin deps/update-hubble-to-$$latest_version"; \
        echo "Then create a pull request on GitHub."; \
    fi
