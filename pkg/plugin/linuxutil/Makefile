REPO_ROOT = $(shell git rev-parse --show-toplevel)
TOOLS_BIN_DIR = $(REPO_ROOT)/hack/tools/bin
MOCKGEN = $(TOOLS_BIN_DIR)/mockgen

.PHONY: generate

generate: $(MOCKGEN) ## Generate mock clients
	$(MOCKGEN) -source=$(REPO_ROOT)/pkg/plugin/linuxutil/types_linux.go -copyright_file=$(REPO_ROOT)/pkg/lib/ignore_headers.txt -package=linuxutil > linuxutil_mock_generated.go

$(MOCKGEN):
	@make -C $(REPO_ROOT) $(MOCKGEN)
