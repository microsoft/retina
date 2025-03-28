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
# Demo utils: Set Kubeconfig, Deploy/Observe/Reload pods
# -------------------------------------------------------------------

set-kubeconfig:
	@make check-env-vars
	@echo "Get kubeconfig for $(STACK_NAME)"
	@cd live/$(STACK_NAME) && \
		KUBECONFIG_CMD=$$(tofu output -json | jq -r '.kubeconfig_command.value') && \
		if [ "$$(echo "$$KUBECONFIG_CMD" | grep -c "az aks get-credentials")" -gt 0 ]; then \
			$$KUBECONFIG_CMD --overwrite-existing; \
		elif [ "$$(echo "$$KUBECONFIG_CMD" | grep -c "gcloud container clusters")" -gt 0 ]; then \
			$$KUBECONFIG_CMD; \
		else \
			$$KUBECONFIG_CMD; \
		fi
	@echo "Set kubeconfig context to $(STACK_NAME)"

# Fix the create-pods target (remove the :0)
create-pods:
	@echo "Deploy demo client and server pods"
	@NODE_NAME=$$(kubectl get no -o jsonpath='{.items[0].metadata.name}'); \
		envsubst < live/files/mc-demo-manifests.yaml | kubectl apply -f -; \
		echo "Client pod deployed on node $$NODE_NAME"

delete-pods:
	@echo "Delete demo client and  server pods"
	@NODE_NAME=$$(kubectl get no -o jsonpath='{.items[0].metadata.name}'); \
		envsubst < live/files/mc-demo-manifests.yaml | kubectl delete -f -; \
		echo "Client pod deployed on node $$NODE_NAME"

check-pods:
	@echo "Checking if demo client and server pods exist..."
	@if kubectl get pods -l app=tcp-client-0 --no-headers 2>/dev/null | grep -q "tcp-client-0" && \
        kubectl get pods -l app=tcp-server --no-headers 2>/dev/null | grep -q "tcp-server"; then \
        echo "Demo pods exist"; \
    else \
        make create-pods; \
    fi

observe-client:
	@echo "Observe client pod"
	@hubble observe --follow --to-pod default/tcp-client-0

restart-coredns:
	@echo "Restart coredns pods"
	@kubectl rollout restart deployment coredns -n kube-system

test-dns:
	@echo "Test DNS resolution for example.com"
	@POD=$$(kubectl get pods -l app=tcp-client-0 -o jsonpath='{.items[0].metadata.name}'); \
	for i in $$(seq 1 10); do \
		echo "Lookup $$i:"; \
		kubectl exec -it $$POD -- nslookup -type=AAAA example.com > /dev/null; \
		kubectl exec -it $$POD -- nslookup -type=A empty.com > /dev/null; \
		kubectl exec -it $$POD -- nslookup -type=AAAA empty.com > /dev/null; \
		kubectl exec -it $$POD -- nslookup -type=AAAA refused.com > /dev/null; \
		kubectl exec -it $$POD -- nslookup -type=A refused.com > /dev/null; \
	done
# -------------------------------------------------------------------
# Demo Scenario 1: Packets dropped by iptables (AKS)
# -------------------------------------------------------------------

drop-init:
	@STACK_NAME=retina-aks make set-kubeconfig
	@make check-pods
	@k9s

drop-add:
	@echo "Add iptables rule to deny traffic from server to client"
	@kubectl exec -it $(shell kubectl get pods -l app=tcp-client-0 -o jsonpath='{.items[0].metadata.name}') -- iptables -A INPUT -s tcp-server.default.svc.cluster.local -j DROP
	@kubectl exec -it $(shell kubectl get pods -l app=tcp-client-0 -o jsonpath='{.items[0].metadata.name}') -- iptables -L --line-numbers

drop-rm:
	@echo "Remove iptables rule to allow traffic from server to client"
	@kubectl exec -it $(shell kubectl get pods -l app=tcp-client-0 -o jsonpath='{.items[0].metadata.name}') -- iptables -D INPUT -s tcp-server.default.svc.cluster.local -j DROP
	@kubectl exec -it $(shell kubectl get pods -l app=tcp-client-0 -o jsonpath='{.items[0].metadata.name}') -- iptables -L --line-numbers

# -------------------------------------------------------------------
# Demo Scenario 2: DNS resolution failure for specific domain (EKS)
# -------------------------------------------------------------------

dns-init:
	@STACK_NAME=retina-eks make set-kubeconfig
	@make check-pods
	@k9s

dns-add:
	@echo "Backup coredns configmap"
	@kubectl get cm -n kube-system coredns -o yaml > live/files/mc-demo-eks-coredns-original.yaml
	@echo "Adding DNS failure for example.com (programmatically)"
	@kubectl replace -f live/files/mc-demo-eks-coredns-custom.yaml
	@echo "CoreDNS ConfigMap updated successfully!"
	@make restart-coredns
	@make test-dns

dns-rm:
	@echo "Restoring original coredns configmap"
	@echo "apiVersion: v1" > /tmp/coredns-reset.yaml && \
	echo "kind: ConfigMap" >> /tmp/coredns-reset.yaml && \
	echo "metadata:" >> /tmp/coredns-reset.yaml && \
	echo "  name: coredns" >> /tmp/coredns-reset.yaml && \
	echo "  namespace: kube-system" >> /tmp/coredns-reset.yaml && \
	echo "  labels:" >> /tmp/coredns-reset.yaml && \
	kubectl get cm -n kube-system coredns -o jsonpath='{.metadata.labels}' | jq -r 'to_entries[] | "    \(.key): \(.value)"' >> /tmp/coredns-reset.yaml && \
	echo "data:" >> /tmp/coredns-reset.yaml && \
	echo "  Corefile: |" >> /tmp/coredns-reset.yaml && \
	echo "    .:53 {" >> /tmp/coredns-reset.yaml && \
	echo "        errors" >> /tmp/coredns-reset.yaml && \
	echo "        health {" >> /tmp/coredns-reset.yaml && \
	echo "            lameduck 5s" >> /tmp/coredns-reset.yaml && \
	echo "        }" >> /tmp/coredns-reset.yaml && \
	echo "        ready" >> /tmp/coredns-reset.yaml && \
	echo "        kubernetes cluster.local in-addr.arpa ip6.arpa {" >> /tmp/coredns-reset.yaml && \
	echo "            pods insecure" >> /tmp/coredns-reset.yaml && \
	echo "            fallthrough in-addr.arpa ip6.arpa" >> /tmp/coredns-reset.yaml && \
	echo "        }" >> /tmp/coredns-reset.yaml && \
	echo "        prometheus :9153" >> /tmp/coredns-reset.yaml && \
	echo "        forward . /etc/resolv.conf" >> /tmp/coredns-reset.yaml && \
	echo "        cache 30" >> /tmp/coredns-reset.yaml && \
	echo "        loop" >> /tmp/coredns-reset.yaml && \
	echo "        reload" >> /tmp/coredns-reset.yaml && \
	echo "        loadbalance" >> /tmp/coredns-reset.yaml && \
	echo "    }" >> /tmp/coredns-reset.yaml && \
	kubectl replace -f /tmp/coredns-reset.yaml
	@echo "CoreDNS ConfigMap restored successfully!"
	@make restart-coredns
	@make test-dns

# -------------------------------------------------------------------
# Demo Scenario 3: Retina capture (GKE)
# -------------------------------------------------------------------

capture-init:
	@STACK_NAME=retina-gke make set-kubeconfig
	@make check-pods
	@k9s

capture-run:
	@echo "Capture packets from client to server"
	@kubectl retina capture create \
		--host-path /tmp/retina-capture \
		--pod-selectors="app=tcp-client-0" \
		--duration 15s -n default
	
capture-copy-file:
	@echo "Copy capture file to local machine"
	@kubectl cp default/tcp-client-0:/host/tmp/retina-capture ./retina-capture
	@echo "Untar the file on localmachine and analyze capture file with Wireshark"
