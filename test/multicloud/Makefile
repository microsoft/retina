PREFIX ?= retina
STACK_NAME ?= $(PREFIX)-aks

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
	@if [ $(STACK_NAME) == $(PREFIX)-kind ]; then \
		rm -rf live/$(STACK_NAME)/mc-kind-config; \
	fi

kind-kubeconfig:
	@kubectl config set-context live/$(PREFIX)-kind/mc-kind-config
