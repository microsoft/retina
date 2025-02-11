PREFIX ?= retina
STACK_NAME ?= $(PREFIX)-aks

.PHONY: init plan apply quick gke aks kind destroy clean kind-kubeconfig test

plan:
	cd live/$(STACK_NAME) && \
		tofu init && tofu plan

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

# !! DANGER remove state, backup, kubeconfig files and .terraform directories
clean:
	@find . -name '*.tfstate*' -delete
	@find . -name '*-kind-config*' -delete
	@find . -name '*.terraform' -type d -exec rm -rf {} +

kind-kubeconfig:
	@kubectl config set-context live/$(PREFIX)-kind/mc-kind-config

# For now we only want to run the retina-kind integration
# since we do not have credentials for the other cloud providers
# Once we do this targets will be updated to
# @cd test && go test -v -count=1 -timeout 30m ./...
test:
	@cd test/integration && go test -run TestRetinaKindIntegration -count=1 -timeout 20m

fmt:
	@tofu fmt -recursive
	@cd test && go fmt ./...
