PREFIX ?= retina
STACK_NAME ?= $(PREFIX)-aks

.PHONY: init plan apply quick gke aks kind destroy clean kind-kubeconfig test

plan:
	cd live/$(STACK_NAME) && \
		tofu fmt && tofu init && tofu plan

apply:
	cd live/$(STACK_NAME) && \
		tofu apply --auto-approve

quick:
	@make plan
	@make apply

gke: export STACK_NAME=$(PREFIX)-gke
gke:
	@make quick

aks: export STACK_NAME=$(PREFIX)-aks
aks:
	@make quick

kind: export STACK_NAME=$(PREFIX)-kind
kind:
	@make quick

destroy:
	cd live/$(STACK_NAME) && \
		tofu destroy --auto-approve

clean: destroy
	@cd live/$(STACK_NAME) && \
		rm -rf .terraform && rm terraform.tfstate && rm terraform.tfstate.backup

kind-kubeconfig:
	@kubectl config set-context live/$(PREFIX)-kind/mc-kind-config

test:
	@cd test && go test -v -count=1 -timeout 30m ./...
