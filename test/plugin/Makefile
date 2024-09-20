REPO_ROOT = $(shell git rev-parse --show-toplevel)
PLUGIN_DIR = $(REPO_ROOT)/pkg/plugin
INIT_DIR = $(REPO_ROOT)/init/retina

generate-map-bpf-obj:
	go generate $(PLUGIN_DIR)/conntrack && go generate $(PLUGIN_DIR)/filter

build-init:
	cd $(INIT_DIR) && go build .

create-config:
	sudo mkdir -p /retina/config/ && echo "enableTelemetry: false" | sudo tee /retina/config/config.yaml > /dev/null

run-init: create-config generate-map-bpf-obj build-init
	sudo $(INIT_DIR)/retina

test-packetparser: run-init
	go build -o ./packetparser/packetparser ./packetparser && sudo ./packetparser/packetparser

test-packetforward: run-init
	go build -o ./packetforward/packetforward ./packetforward && sudo ./packetforward/packetforward

test-dropreason: run-init
	go build -o ./dropreason/dropreason ./dropreason && sudo ./dropreason/dropreason

clean:
	rm -f $(INIT_DIR)/retina; \
	cd $(REPO_ROOT) && sudo find . -type f -name "*.o" -exec truncate -s 0 {} \;
