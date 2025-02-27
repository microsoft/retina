PREFIX ?= retina
STACK_NAME ?= $(PREFIX)-aks

.PHONY: init plan apply quick gke aks kind destroy clean kind-kubeconfig test

plan:
	cd live/$(STACK_NAME) && \
		tofu init && tofu plan

apply:
	cd live/$(STACK_NAME) && \
		tofu apply --auto-approve

check-env-vars:
	@if [ -z "$(GRAFANA_AUTH)" ]; then echo "GRAFANA_AUTH is not set"; exit 1; fi
	@if [ -z "$(STACK_NAME)" ]; then echo "STACK_NAME is not set"; exit 1; fi
	@if [ "$(STACK_NAME)" = "retina-gke" ] && [ -z "$(GOOGLE_APPLICATION_CREDENTIALS)" ]; then echo "GOOGLE_APPLICATION_CREDENTIALS is not set"; exit 1; fi
	@if [ "$(STACK_NAME)" = "retina-eks" ] && [ -z "$(AWS_SECRET_ACCESS_KEY)" ]; then echo "AWS_SECRET_ACCESS_KEY is not set"; exit 1; fi
	@if [ "$(STACK_NAME)" = "retina-eks" ] && [ -z "$(AWS_ACCESS_KEY_ID)" ]; then echo "AWS_ACCESS_KEY_ID is not set"; exit 1; fi

quick:
	@make check-env-vars
	@make plan
	@make apply

gke: export STACK_NAME=$(PREFIX)-gke
gke:
	@make quick

aks: export STACK_NAME=$(PREFIX)-aks
aks:
	@make quick

eks: export STACK_NAME=$(PREFIX)-eks
eks:
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
