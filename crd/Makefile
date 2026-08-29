.DEFAULT_GOAL = all 

REPO_ROOT = $(shell git rev-parse --show-toplevel)
CONTROLLER_GEN = go tool sigs.k8s.io/controller-tools/cmd/controller-gen
HELM_CRD_DIR = $(REPO_ROOT)/deploy/standard/manifests/controller/helm/retina/crds

.PHONY: generate manifests

all: generate manifests

generate:
	$(CONTROLLER_GEN) object paths="./api/..."

manifests:
	rm -rf $(HELM_CRD_DIR)
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=$(HELM_CRD_DIR)

