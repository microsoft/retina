REPO_ROOT = $(shell git rev-parse --show-toplevel)
PLUGIN_DIR = $(REPO_ROOT)/pkg/plugin
INIT_DIR = $(REPO_ROOT)/init/retina
CONFIG_DIR = /retina/config
PACKETPARSER_DIR = $(REPO_ROOT)/test/plugin/packetparser
PACKETFORWARD_DIR = $(REPO_ROOT)/test/plugin/packetforward
DROPREASON_DIR = $(REPO_ROOT)/test/plugin/dropreason

.PHONY : generate-map-bpf-obj
generate-map-bpf-obj:
	go generate $(PLUGIN_DIR)/conntrack && go generate $(PLUGIN_DIR)/filter

.PHONY : build-init
build-init:
	cd $(INIT_DIR) && go build .

.PHONY : create-config
create-config:
	sudo mkdir -p $(CONFIG_DIR) && echo "enableTelemetry: false" | sudo tee $(CONFIG_DIR)/config.yaml > /dev/null

.PHONY : run-init
run-init: create-config generate-map-bpf-obj build-init
	sudo $(INIT_DIR)/retina

.PHONY : test-packetparser
test-packetparser: run-init
	trap 'rm -f $(PACKETPARSER_DIR)/packetparser' INT TERM EXIT; \
	go build -o $(PACKETPARSER_DIR)/packetparser $(PACKETPARSER_DIR) && sudo $(PACKETPARSER_DIR)/packetparser

.PHONY : test-packetforward
test-packetforward: run-init
	trap 'rm -f $(PACKETFORWARD_DIR)/packetforward' INT TERM EXIT; \
	go build -o $(PACKETFORWARD_DIR)/packetforward $(PACKETFORWARD_DIR) && sudo $(PACKETFORWARD_DIR)/packetforward

.PHONY : test-dropreason
test-dropreason: run-init
	trap 'rm -f $(DROPREASON_DIR)/dropreason' INT TERM EXIT; \
	go build -o $(DROPREASON_DIR)/dropreason $(DROPREASON_DIR) && sudo $(DROPREASON_DIR)/dropreason

.PHONY : clean
clean:
	sudo rm -rf $(CONFIG_DIR); \
	rm -f $(INIT_DIR)/retina; \
	cd $(REPO_ROOT) && sudo find . -type f -name "*.o" -exec truncate -s 0 {} \;
