.DEFAULT_GOAL := help

# Default platform commands
RMDIR := rm -rf

## Globals

REPO_ROOT = $(shell git rev-parse --show-toplevel)
ifndef TAG
	TAG ?= $(shell git describe --tags --always)
endif
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
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

IMAGE_REGISTRY    ?= acnpublic.azurecr.io
OS                ?= $(GOOS)
ARCH              ?= $(GOARCH)
PLATFORM          ?= $(OS)/$(ARCH)

CONTAINER_BUILDER ?= buildah
CONTAINER_RUNTIME ?= podman
YEAR 			  ?=2022

ALL_ARCH.linux = amd64 arm64
ALL_ARCH.windows = amd64

# prefer buildah, if available, but fall back to docker if that binary is not in the path.
ifeq (, $(shell which $(CONTAINER_BUILDER)))
CONTAINER_BUILDER = docker
endif
# use docker if platform is windows
ifeq ($(OS),windows)
CONTAINER_BUILDER = docker
endif
# prefer podman, if available, but fall back to docker if that binary is not in the path.
ifeq (, $(shell which $(CONTAINER_RUNTIME)))
CONTAINER_RUNTIME = docker
endif

# TAG is OS and platform agonstic, which can be used for binary version and image manifest tag,
# while RETINA_PLATFORM_TAG is platform specific, which can be used for image built for specific platforms.
RETINA_PLATFORM_TAG        ?= $(subst /,-,$(PLATFORM))-$(TAG)

# for windows os, add year to the platform tag
ifeq ($(OS),windows)
RETINA_PLATFORM_TAG        = windows-ltsc$(YEAR)-amd64-$(TAG)
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
	go generate ./... &&\
	cd $(RETINA_DIR) &&\
	CGO_ENABLED=0 &&\
	go build -v -o $(RETINA_BUILD_DIR)/retina$(EXE_EXT) -gcflags="-dwarflocationlists=true" -ldflags "-X main.version=$(TAG) -X main.applicationInsightsID=$(APP_INSIGHTS_ID)"

all-kubectl-retina: $(addprefix kubectl-retina-linux-,${ALL_ARCH.linux})  $(addprefix kubectl-retina-windows-,${ALL_ARCH.windows}) ## build kubectl plugin for all platforms

kubectl-retina: # build kubectl plugin for current platform
	if [ "$(BUILD_LOCALLY)" = "true" ]; then \
		$(MAKE) kubectl-retina-binary-$(GOOS)-$(GOARCH); \
	else \
		$(MAKE) kubectl-retina-$(GOOS)-$(GOARCH); \
	fi
	cp $(KUBECTL_RETINA_BUILD_DIR)/kubectl-retina-$(GOOS)-$(GOARCH) $(KUBECTL_RETINA_BUILD_DIR)/kubectl-gadget

kubectl-retina-binary-%: ## build kubectl plugin locally.
	export CGO_ENABLED=0 && \
	export GOOS=$(shell echo $* |cut -f1 -d-) GOARCH=$(shell echo $* |cut -f2 -d-) && \
	go build -v \
		-o $(KUBECTL_RETINA_BUILD_DIR)/kubectl-retina-$${GOOS}-$${GOARCH} \
		-gcflags="-dwarflocationlists=true" \
		-ldflags "-X github.com/microsoft/retina/cli/cmd.Version=$(TAG)" \
		github.com/microsoft/retina/cli

kubectl-retina-%: ## build kubectl plugin
	CONTAINER_BUILDER=docker $(MAKE) kubectl-retina-image
#	copy the binary from the container image.
	mkdir -p output/kubectl-retina
	docker run --rm --platform=linux/amd64 \
		--user $(shell id -u):$(shell id -g) \
		-v $(PWD):/app \
		$(IMAGE_REGISTRY)/$(KUBECTL_RETINA_IMAGE):$(RETINA_PLATFORM_TAG) \
		sh -c "cp /bin/kubectl-retina /app/output/kubectl-retina/kubectl-gadget && cp /bin/kubectl-retina /app/output/kubectl-retina/kubectl-retina-$(GOOS)-$(GOARCH)"

install-kubectl-retina: kubectl-retina ## install kubectl plugin
	chmod +x $(KUBECTL_RETINA_BUILD_DIR)/kubectl-gadget
	sudo cp $(KUBECTL_RETINA_BUILD_DIR)/kubectl-gadget /usr/local/bin/kubectl-retina
	kubectl retina --help

retina-capture-workload: ## build the Retina capture workload
	cd $(CAPTURE_WORKLOAD_DIR) && CGO_ENABLED=0 go build -v -o $(RETINA_BUILD_DIR)/captureworkload$(EXE_EXT) -gcflags="-dwarflocationlists=true"  -ldflags "-X main.version=$(TAG)"

##@ Containers

RETINA_BUILDER_IMAGE = retina-builder
RETINA_TOOLS_IMAGE = retina-tools
RETINA_IMAGE = retina-agent
RETINA_INIT_IMAGE = retina-init
KUBECTL_RETINA_IMAGE=kubectl-retina
RETINA_OPERATOR_IMAGE=retina-operator
RETINA_INTEGRATION_TEST_IMAGE=retina-integration-test
RETINA_PROTO_IMAGE=retina-proto-gen
RETINA_GO_GEN_IMAGE=retina-go-gen
KAPINGER_IMAGE = kapinger

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

# VERSION vs TAG: VERSION is the version of the binary, TAG is the version of the container image
# which may contain OS and ARCH information.
container-buildah: # util target to build container images using buildah. do not invoke directly.
	buildah bud \
		--jobs 16 \
		--platform $(PLATFORM) \
		-f $(DOCKERFILE) \
		--build-arg VERSION=$(VERSION) $(EXTRA_BUILD_ARGS) \
		--build-arg GOOS=$(GOOS) \
		--build-arg GOARCH=$(GOARCH) \
		--build-arg APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
		--build-arg builderImage=$(IMAGE_REGISTRY)/$(RETINA_BUILDER_IMAGE):$(TAG) \
		--build-arg toolsImage=$(IMAGE_REGISTRY)/$(RETINA_TOOLS_IMAGE):$(TAG) \
		-t $(IMAGE_REGISTRY)/$(IMAGE):$(TAG) \
		$(CONTEXT_DIR)

container-docker: # util target to build container images using docker buildx. do not invoke directly.
	docker buildx build \
		$(ACTION) \
		--platform $(PLATFORM) \
		-f $(DOCKERFILE) \
		--build-arg VERSION=$(VERSION) $(EXTRA_BUILD_ARGS) \
		--build-arg GOOS=$(GOOS) \
		--build-arg GOARCH=$(GOARCH) \
		--build-arg APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
		--build-arg builderImage=$(IMAGE_REGISTRY)/$(RETINA_BUILDER_IMAGE):$(TAG) \
		--build-arg toolsImage=$(IMAGE_REGISTRY)/$(RETINA_TOOLS_IMAGE):$(TAG) \
		-t $(IMAGE_REGISTRY)/$(IMAGE):$(TAG) \
		$(CONTEXT_DIR)

retina-builder-image: ## build the retina builder container image.
	echo "Building for $(PLATFORM)"
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=controller/Dockerfile.builder \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(RETINA_BUILDER_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
			CONTEXT_DIR=$(REPO_ROOT) \
			ACTION=--load

retina-builder-image-remove:
	$(CONTAINER_BUILDER) rmi $(IMAGE_REGISTRY)/$(RETINA_BUILDER_IMAGE):$(RETINA_PLATFORM_TAG)

retina-tools-image: ## build the retina container image.
	echo "Building for $(PLATFORM)"
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=controller/Dockerfile.tools \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(RETINA_TOOLS_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
			CONTEXT_DIR=$(REPO_ROOT) \
			ACTION=--load

retina-tools-image-remove:
	$(CONTAINER_BUILDER) rmi $(IMAGE_REGISTRY)/$(RETINA_TOOLS_IMAGE):$(RETINA_PLATFORM_TAG)

retina-image: ## This pulls dependecies from registry. Use retina-image-local to build dependencies first locally.
	echo "Building for $(PLATFORM)"
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=controller/Dockerfile.controller \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(RETINA_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
			CONTEXT_DIR=$(REPO_ROOT) \
			ACTION=--load

retina-init-image: ## build the retina container image.
	echo "Building for $(PLATFORM)"
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=controller/Dockerfile.init \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(RETINA_INIT_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
			CONTEXT_DIR=$(REPO_ROOT) \
			ACTION=--load

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
			CONTEXT_DIR=$(REPO_ROOT) \
			ACTION=--load

retina-operator-image:  ## build the retina operator image.
	echo "Building for $(PLATFORM)"
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=operator/Dockerfile \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(RETINA_OPERATOR_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
			CONTEXT_DIR=$(REPO_ROOT) \
			ACTION=--load

kubectl-retina-image-push: ## push kubectl-retina container image.
	$(MAKE) container-push \
		IMAGE=$(KUBECTL_RETINA_IMAGE) \
		TAG=$(RETINA_PLATFORM_TAG)

retina-image-push: ## push the retina container image.
	$(MAKE) container-push \
		IMAGE=$(RETINA_IMAGE) \
		TAG=$(RETINA_PLATFORM_TAG)

retina-builder-image-push: ## push the retina builder container image.
	$(MAKE) container-push \
		IMAGE=$(RETINA_BUILDER_IMAGE) \
		TAG=$(RETINA_PLATFORM_TAG)

retina-tools-image-push: ## push the retina tools container image.
	$(MAKE) container-push \
		IMAGE=$(RETINA_TOOLS_IMAGE) \
		TAG=$(RETINA_PLATFORM_TAG)

retina-init-image-push: ## push the retina container image.
	$(MAKE) container-push \
		IMAGE=$(RETINA_INIT_IMAGE) \
		TAG=$(RETINA_PLATFORM_TAG)

retina-operator-image-push: ## push the retina container image.
	$(MAKE) container-push \
		IMAGE=$(RETINA_OPERATOR_IMAGE) \
		TAG=$(RETINA_PLATFORM_TAG)

retina-image-win: ## build the retina Windows container image.
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=windows/amd64 \
			DOCKERFILE=controller/Dockerfile.windows-$(YEAR) \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(RETINA_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			CONTEXT_DIR=$(REPO_ROOT) \
			ACTION=--load

retina-image-win-push: ## push the retina Windows container image.
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=windows/amd64 \
			DOCKERFILE=controller/Dockerfile.windows-$(YEAR) \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(RETINA_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			CONTEXT_DIR=$(REPO_ROOT) \
			ACTION=--push

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

all-images-local:
	$(MAKE) -j4 retina-builder-image retina-tools-image retina-operator-image kubectl-retina-image
	$(MAKE) -j2 retina-image retina-init-image 

all-images-local-push:
	$(MAKE) -j3 retina-builder-image-push retina-tools-image-push retina-operator-image-push
	$(MAKE) -j3 retina-image-push retina-init-image-push kubectl-retina-image-push

base-images-remove:
	$(MAKE) -j2 retina-builder-image-remove retina-tools-image-remove

# Build images locally.
# Don't use this in pipeline, we want to pull images from registry.
retina-image-local:
	$(MAKE) -j2 retina-builder-image retina-tools-image
	$(MAKE) retina-image

retina-init-image-local:
	$(MAKE) -j2 retina-builder-image retina-tools-image
	$(MAKE) retina-init-image

##@ Tests
# Make sure the layer has only one directory.
# the test DockerFile needs to build the scratch stage with all the output files 
# and we will untar the archive and copy the files from scratch stage
retina-test-image: ## build the retina container image for testing.
	$(MAKE) container-docker \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=./test/image/Dockerfile \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(RETINA_IMAGE) \
			CONTEXT_DIR=$(REPO_ROOT) \
			TAG=$(RETINA_PLATFORM_TAG) \

	docker save -o archives.tar $(IMAGE_REGISTRY)/$(RETINA_IMAGE):$(RETINA_PLATFORM_TAG) && \
	mkdir -p archivelayers && \
	cp archives.tar archivelayers/archives.tar && \
	cd archivelayers/ && \
	pwd && \
	tar -xvf archives.tar && \
	cd `ls -d */` && \
	pwd && \
	tar -xvf layer.tar && \
	cp coverage.out ../../
	$(MAKE) retina-cc

COVER_PKG ?= .

retina-integration-test-image: # Build the retina container image for integration testing.
	docker build \
		-t $(IMAGE_REGISTRY)/$(RETINA_INTEGRATION_TEST_IMAGE):$(RETINA_PLATFORM_TAG) \
		-f test/integration/Dockerfile.integration \
		--build-arg kubeconfig=$(HOME)/.kube/config \
		--build-arg ENABLE_POD_LEVEL=$(ENABLE_POD_LEVEL) \
		.

retina-integration-docker-deploy:
	docker rm -f retina-integ-container 2> /dev/null
	docker run \
		-e RETINA_AGENT_IMAGE=$(IMAGE_REGISTRY)/$(RETINA_IMAGE):$(TAG) \
		-e ENABLE_POD_LEVEL=$(ENABLE_POD_LEVEL) \
		-v $(HOME)/.kube/config:/root/.kube/config \
		--name retina-integ-container \
		$(IMAGE_REGISTRY)/$(RETINA_INTEGRATION_TEST_IMAGE):$(RETINA_PLATFORM_TAG) \
		|| true
	docker cp retina-integ-container:/tmp/retina-integration-logs .

retina-ut: $(ENVTEST) # Run unit tests.
	go build -o test-summary ./test/utsummary/main.go
	CGO_ENABLED=0 KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use -p path)" go test -tags=unit -coverprofile=coverage.out -v -json ./... | ./test-summary --progress --verbose

retina-cc: # Code coverage.
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

retina-integration: $(GINKGO) # Integration tests.
	export ACK_GINKGO_RC=true && $(GINKGO) -keepGoing -tags=integration ./test/integration/... -v -progress -trace -cover -coverprofile=coverage.out

retina-export-logs: # Export worker node logs.
	mkdir kubernetes-logs
	kubectl get pods -A -o wide > kubernetes-logs/pods.txt
	docker cp retina-cluster-worker:/var/log kubernetes-logs

##@ kind

kind-setup: ## Deploy kind cluster.
	$(KIND) create cluster --name $(KIND_CLUSTER) --config ./test/kind/kind.yaml
#	Install NPM
	kubectl  apply -f https://raw.githubusercontent.com/Azure/azure-container-networking/master/npm/azure-npm.yaml
	sleep 5s
	kubectl -n kube-system wait --for=condition=ready --timeout=120s pod -l k8s-app=azure-npm

kind-clean: ## Delete kind cluster.
	$(KIND) delete cluster --name $(KIND_CLUSTER)

kind-load-image: ## Load local image to kind nodes.
#	$(MAKE) retina-image
	$(KIND) load docker-image --name $(KIND_CLUSTER) $(IMAGE_REGISTRY)/$(RETINA_IMAGE):$(RETINA_PLATFORM_TAG)
	$(KIND) load docker-image --name $(KIND_CLUSTER) $(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE):$(RETINA_PLATFORM_TAG)
	$(KIND) load docker-image --name $(KIND_CLUSTER) $(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE):$(RETINA_PLATFORM_TAG)

kind-install: kind-load-image # Install Retina in kind cluster.
	helm install retina ./deploy/manifests/controller/helm/retina/ \
	--set image.repository=$(IMAGE_REGISTRY)/$(RETINA_IMAGE) \
	--set image.tag=$(RETINA_PLATFORM_TAG) \
	--set operator.repository=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE) \
	--set operator.tag=$(RETINA_PLATFORM_TAG) \
	--set image.initRepository=$(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE) \
	--set image.pullPolicy=Never \
	--set logLevel=debug \
	--set os.windows=false \
	--namespace kube-system --dependency-update
	sleep 5s
# 	Wait for retina agent to be ready.
	kubectl -n kube-system wait --for=condition=ready --timeout=120s pod -l app=retina
# 	Port forward the retina api server.
	kubectl -n kube-system port-forward svc/retina-svc 8889:10093  2>&1 >/dev/null &

kind-uninstall: # Uninstall Retina from kind cluster.
	helm uninstall retina -n kube-system

## Reusable targets for building multiplat container image manifests.

IMAGE_ARCHIVE_DIR ?= $(shell pwd)

manifest-create: # util target to compose multiarch container manifests from platform specific images.
	cat /usr/share/containers/containers.conf
	$(CONTAINER_BUILDER) manifest create $(IMAGE_REGISTRY)/$(IMAGE):$(TAG)
	for PLATFORM in $(PLATFORMS); do $(MAKE) manifest-add PLATFORM=$$PLATFORM IMAGE=$(IMAGE) TAG=$(TAG); done

manifest-add:
	if [ "$(PLATFORM)" = "windows/amd64/2022" ]; then \
		echo "Adding windows/amd64/2022"; \
		$(CONTAINER_BUILDER) manifest add --os-version=$(WINVER2022) --os=windows $(IMAGE_REGISTRY)/$(IMAGE):$(TAG) docker://$(IMAGE_REGISTRY)/$(IMAGE):windows-ltsc2022-amd64-$(TAG); \
	elif [ "$(PLATFORM)" = "windows/amd64/2019" ]; then \
		echo "Adding windows/amd64/2019"; \
		$(CONTAINER_BUILDER) manifest add --os-version=$(WINVER2019) --os=windows $(IMAGE_REGISTRY)/$(IMAGE):$(TAG) docker://$(IMAGE_REGISTRY)/$(IMAGE):windows-ltsc2019-amd64-$(TAG); \
	else \
		echo "Adding $(PLATFORM)"; \
		$(CONTAINER_BUILDER) manifest add $(IMAGE_REGISTRY)/$(IMAGE):$(TAG) docker://$(IMAGE_REGISTRY)/$(IMAGE):$(subst /,-,$(PLATFORM))-$(TAG); \
	fi;

manifest-push: # util target to push multiarch container manifest.
	$(CONTAINER_BUILDER) manifest inspect $(IMAGE_REGISTRY)/$(IMAGE):$(TAG)
	$(CONTAINER_BUILDER) manifest push --all $(IMAGE_REGISTRY)/$(IMAGE):$(TAG) docker://$(IMAGE_REGISTRY)/$(IMAGE):$(TAG)

manifest-skopeo-archive: # util target to export tar archive of multiarch container manifest.
	skopeo copy --all docker://$(IMAGE_REGISTRY)/$(IMAGE):$(TAG) oci-archive:$(IMAGE_ARCHIVE_DIR)/$(IMAGE)-$(TAG).tar

## Build specific multiplat images.

retina-builder-manifest-create: ## build retina multiplat container manifest.
	$(MAKE) manifest-create \
		PLATFORMS="$(PLATFORMS)" \
		IMAGE=$(RETINA_BUILDER_IMAGE) \
		TAG=$(TAG)

retina-builder-manifest-push: ## push retina multiplat container manifest
	$(MAKE) manifest-push \
		IMAGE=$(RETINA_BUILDER_IMAGE) \
		TAG=$(TAG)

retina-builder-skopeo-archive: ## export tar archive of retina multiplat container manifest.
	$(MAKE) manifest-skopeo-archive \
		IMAGE=$(RETINA_BUILDER_IMAGE) \
		TAG=$(TAG)

retina-tools-manifest-create: ## build retina multiplat container manifest.
	$(MAKE) manifest-create \
		PLATFORMS="$(PLATFORMS)" \
		IMAGE=$(RETINA_TOOLS_IMAGE) \
		TAG=$(TAG)

retina-tools-manifest-push: ## push retina multiplat container manifest
	$(MAKE) manifest-push \
		IMAGE=$(RETINA_TOOLS_IMAGE) \
		TAG=$(TAG)

retina-tools-skopeo-archive: ## export tar archive of retina multiplat container manifest.
	$(MAKE) manifest-skopeo-archive \
		IMAGE=$(RETINA_TOOLS_IMAGE) \
		TAG=$(TAG)
		
retina-init-manifest-create: ## build retina multiplat container manifest.
	$(MAKE) manifest-create \
		PLATFORMS="$(PLATFORMS)" \
		IMAGE=$(RETINA_INIT_IMAGE) \
		TAG=$(TAG)

retina-init-manifest-push: ## push retina multiplat container manifest
	$(MAKE) manifest-push \
		IMAGE=$(RETINA_INIT_IMAGE) \
		TAG=$(TAG)

retina-init-skopeo-archive: ## export tar archive of retina multiplat container manifest.
	$(MAKE) manifest-skopeo-archive \
		IMAGE=$(RETINA_INIT_IMAGE) \
		TAG=$(TAG)

retina-agent-manifest-create: ## build retina multiplat container manifest.
	$(MAKE) manifest-create \
		PLATFORMS="$(PLATFORMS)" \
		IMAGE=$(RETINA_IMAGE) \
		TAG=$(TAG)

retina-agent-manifest-push: ## push retina multiplat container manifest
	$(MAKE) manifest-push \
		IMAGE=$(RETINA_IMAGE) \
		TAG=$(TAG)

retina-agent-skopeo-archive: ## export tar archive of retina multiplat container manifest.
	$(MAKE) manifest-skopeo-archive \
		IMAGE=$(RETINA_IMAGE) \
		TAG=$(TAG) 

retina-operator-manifest-create: ## build retina multiplat container manifest.
	$(MAKE) manifest-create \
		PLATFORMS="$(PLATFORMS)" \
		IMAGE=$(RETINA_OPERATOR_IMAGE) \
		TAG=$(TAG)

retina-operator-manifest-push: ## push retina multiplat container manifest
	$(MAKE) manifest-push \
		IMAGE=$(RETINA_OPERATOR_IMAGE) \
		TAG=$(TAG)

retina-operator-skopeo-archive: ## export tar archive of retina multiplat container manifest.
	$(MAKE) manifest-skopeo-archive \
		IMAGE=$(RETINA_OPERATOR_IMAGE) \
		TAG=$(TAG) 
		
kubectl-retina-manifest-create: ## build kubectl plugin multiplat container manifest.
	$(MAKE) manifest-create \
		PLATFORMS="$(PLATFORMS)" \
		IMAGE=$(KUBECTL_RETINA_IMAGE) \
		TAG=$(TAG)

kubectl-retina-manifest-push: ## push kubectl plugin multiplat container manifest.
	$(MAKE) manifest-push \
		IMAGE=$(KUBECTL_RETINA_IMAGE) \
		TAG=$(TAG)

kubectl-retina-skopeo-archive: ## export tar archive of retina kubectl plugin container manifest.
	$(MAKE) manifest-skopeo-archive \
		IMAGE=$(KUBECTL_RETINA_IMAGE) \
		TAG=$(TAG) 


.PHONY: manifests
manifests: 
	cd crd && make manifests && make generate

# basic/node-level mode
helm-install: manifests
	helm install retina ./deploy/manifests/controller/helm/retina/ \
		--namespace kube-system \
		--set image.repository=$(IMAGE_REGISTRY)/$(RETINA_IMAGE) \
		--set image.tag=$(RETINA_PLATFORM_TAG) \
		--set image.initRepository=$(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE) \
		--set image.pullPolicy=Always \
		--set logLevel=info \
		--set os.windows=true \
		--set operator.enabled=false \
		--set enabledPlugin_linux="[\"dropreason\"\,\"packetforward\"\,\"linuxutil\"\,\"dns\"]"

helm-install-with-operator: manifests
	helm install retina ./deploy/manifests/controller/helm/retina/ \
		--namespace kube-system \
		--set image.repository=$(IMAGE_REGISTRY)/$(RETINA_IMAGE) \
		--set image.tag=$(RETINA_PLATFORM_TAG) \
		--set image.initRepository=$(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE) \
		--set image.pullPolicy=Always \
		--set logLevel=info \
		--set os.windows=true \
		--set operator.enabled=true \
		--set operator.enableRetinaEndpoint=true \
		--set operator.tag=$(RETINA_PLATFORM_TAG) \
		--set operator.repository=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE) \
		--skip-crds \
		--set enabledPlugin_linux="[\"dropreason\"\,\"packetforward\"\,\"linuxutil\"\,\"dns\"]"

# advanced/pod-level mode with scale limitations, where metrics are aggregated by source and destination Pod
helm-install-advanced-remote-context: manifests
	helm install retina ./deploy/manifests/controller/helm/retina/ \
		--namespace kube-system \
		--set image.repository=$(IMAGE_REGISTRY)/$(RETINA_IMAGE) \
		--set image.tag=$(RETINA_PLATFORM_TAG) \
		--set image.initRepository=$(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE) \
		--set image.pullPolicy=Always \
		--set logLevel=info \
		--set os.windows=true \
		--set operator.enabled=true \
		--set operator.enableRetinaEndpoint=true \
		--set operator.tag=$(RETINA_PLATFORM_TAG) \
		--set operator.repository=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE) \
		--skip-crds \
		--set enabledPlugin_linux="[\"dropreason\"\,\"packetforward\"\,\"linuxutil\"\,\"dns\",\"packetparser\"\]" \
		--set enablePodLevel=true \
		--set remoteContext=true

# advanced/pod-level mode designed for scale, where metrics are aggregated by "local" Pod (source for outgoing traffic, destination for incoming traffic)
helm-install-advanced-local-context: manifests
	helm install retina ./deploy/manifests/controller/helm/retina/ \
		--namespace kube-system \
		--set image.repository=$(IMAGE_REGISTRY)/$(RETINA_IMAGE) \
		--set image.tag=$(RETINA_PLATFORM_TAG) \
		--set image.initRepository=$(IMAGE_REGISTRY)/$(RETINA_INIT_IMAGE) \
		--set image.pullPolicy=Always \
		--set logLevel=info \
		--set os.windows=true \
		--set operator.enabled=true \
		--set operator.enableRetinaEndpoint=true \
		--set operator.tag=$(RETINA_PLATFORM_TAG) \
		--set operator.repository=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE) \
		--skip-crds \
		--set enabledPlugin_linux="[\"dropreason\"\,\"packetforward\"\,\"linuxutil\"\,\"dns\",\"packetparser\"\]" \
		--set enablePodLevel=true \
		--set enableAnnotations=true \
		--set bypassLookupIPOfInterest=false

helm-uninstall:
	helm uninstall retina -n kube-system

.PHONY: docs
docs: 
	echo $(PWD)
	docker run -it -p 3000:3000 -v $(PWD):/retina -w /retina/ node:20-alpine ./site/start-dev.sh

.PHONY: docs-pod
docs-prod:
	docker run -i -p 3000:3000 -v $(PWD):/retina -w /retina/ node:20-alpine npm install --prefix site && npm run build --prefix site
 
kapinger-image: ## build the retina container image.
	echo "Building for $(PLATFORM)"
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=$(PLATFORM) \
			DOCKERFILE=hack/tools/kapinger/Dockerfile \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(KAPINGER_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			APP_INSIGHTS_ID=$(APP_INSIGHTS_ID) \
			CONTEXT_DIR=$(REPO_ROOT) \
			ACTION=--load

kapinger-image-push:
	$(MAKE) container-push \
		IMAGE=$(KAPINGER_IMAGE) \
		TAG=$(RETINA_PLATFORM_TAG)

kapinger-manifest-create:
	$(MAKE) manifest-create \
		PLATFORMS="$(PLATFORMS)" \
		IMAGE=$(KAPINGER_IMAGE) \
		TAG=$(TAG)

kapinger-manifest-push:
	$(MAKE) manifest-push \
		IMAGE=$(KAPINGER_IMAGE) \
		TAG=$(TAG)

kapinger-image-win-push: 
	$(MAKE) container-$(CONTAINER_BUILDER) \
			PLATFORM=windows/amd64 \
			DOCKERFILE=hack/tools/kapinger/Dockerfile.windows \
			REGISTRY=$(IMAGE_REGISTRY) \
			IMAGE=$(KAPINGER_IMAGE) \
			VERSION=$(TAG) \
			TAG=$(RETINA_PLATFORM_TAG) \
			CONTEXT_DIR=$(REPO_ROOT) \
			ACTION=--push

kapinger-skopeo-archive: 
	$(MAKE) manifest-skopeo-archive \
		IMAGE=$(KAPINGER_IMAGE) \
		TAG=$(TAG)
