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

# -------------------------------------------------------------------
# Demo Prep: Set Kubeconfig, Deploy client and server pods
# -------------------------------------------------------------------

demo-set-kubeconfig:
	@make check-env-vars
	@echo "Get kubeconfig for $(STACK_NAME)"
	@cd live/$(STACK_NAME) && \
		tofu output -json | jq -r '.kubeconfig_command.value' | bash
	@echo "Set kubeconfig context to $(STACK_NAME)"

demo-create-pods:
	@echo "Deploy demo client and server pods"
	@NODE_NAME=$$(kubectl get no -o jsonpath='{.items[0].metadata.name}'); \
		envsubst < live/files/mc-demo-manifests.yaml | kubectl apply -f -; \
		echo "Client pod deployed on node $$NODE_NAME"
demo-delete-pods:
	@echo "Delete demo client and  server pods"
	@NODE_NAME=$$(kubectl get no -o jsonpath='{.items[0].metadata.name}'); \
		envsubst < live/files/mc-demo-manifests.yaml | kubectl delete -f -; \
		echo "Client pod deployed on node $$NODE_NAME"

demo-observe-client:
	@echo "Observe client pod"
	@hubble observe --follow --to-pod default/tcp-client-0

# -------------------------------------------------------------------
# Demo Scenario 1: Packets dropped by iptables (AKS)
# -------------------------------------------------------------------

demo-drop-add:
	@echo "Add iptables rule to deny traffic from server to client"
	@kubectl exec -it $(shell kubectl get pods -l app=tcp-client-0 -o jsonpath='{.items[0].metadata.name}') -- iptables -A INPUT -s tcp-server.default.svc.cluster.local -j DROP
	@kubectl exec -it $(shell kubectl get pods -l app=tcp-client-0 -o jsonpath='{.items[0].metadata.name}') -- iptables -L --line-numbers

demo-drop-rm:
	@echo "Remove iptables rule to allow traffic from server to client"
	@kubectl exec -it $(shell kubectl get pods -l app=tcp-client-0 -o jsonpath='{.items[0].metadata.name}') -- iptables -D INPUT -s tcp-server.default.svc.cluster.local -j DROP
	@kubectl exec -it $(shell kubectl get pods -l app=tcp-client-0 -o jsonpath='{.items[0].metadata.name}') -- iptables -L --line-numbers

# -------------------------------------------------------------------
# Demo Scenario 2: DNS resolution failure for specific domain (EKS)
# -------------------------------------------------------------------

demo-dns-add-failure:
	@echo "Backup coredns configmap"
	@kubectl get cm -n kube-system coredns -o yaml > live/files/mc-demo-eks-coredns-original.yaml
	@echo "Add DNS failure for example.com"
	@cat live/files/mc-demo-eks-coredns-custom.yaml
	@echo "k edit cm -n kube-system coredns"

demo-dns-test:
	@echo "Test DNS resolution for example.com"
	@POD=$$(kubectl get pods -l app=tcp-client-0 -o jsonpath='{.items[0].metadata.name}'); \
	for i in $$(seq 1 10); do \
		echo "Lookup $$i:"; \
		kubectl exec -it $$POD -- nslookup -type=AAAA example.com; \
		kubectl exec -it $$POD -- nslookup -type=A empty.com; \
		kubectl exec -it $$POD -- nslookup -type=AAAA empty.com; \
		kubectl exec -it $$POD -- nslookup -type=AAAA refused.com; \
		kubectl exec -it $$POD -- nslookup -type=A refused.com; \
	done

demo-dns-rm-failure:
	@echo "Restore coredns configmap"
	@cat live/files/mc-demo-eks-coredns-original.yaml
	@echo "k edit cm -n kube-system coredns"
	@echo "Now you can reload DNS pods [make demo-dns-restart-coredns]"

demo-dns-restart-coredns:
	@echo "Restart coredns pods"
	@kubectl rollout restart deployment coredns -n kube-system

# -------------------------------------------------------------------
# Demo Scenario 3: Retina capture (GKE)
# -------------------------------------------------------------------

demo-capture-run:
	@echo "Capture packets from client to server"
	@kubectl retina capture create \
		--host-path /tmp/retina-capture \
		--pod-selectors="app=tcp-client-0" \
		--duration 15s -n default
	
demo-capture-copy-file:
	@echo "Copy capture file to local machine"
	@kubectl cp default/tcp-client-0:/host/tmp/retina-capture ./retina-capture
	@echo "Untar the file on localmachine and analyze capture file with Wireshark"
