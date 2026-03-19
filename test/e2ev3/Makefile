TIMEOUT_KIND   ?= 60m
TIMEOUT_AZURE  ?= 120m
PROVIDER       ?= kind
KUBECONFIG     ?=
CREATE_INFRA   ?= true
DELETE_INFRA   ?= true

REPO_ROOT := $(shell git rev-parse --show-toplevel)

GO_TEST = cd $(REPO_ROOT) && go test -v -tags e2e ./test/e2ev3/ -timeout

# base flags, computed from variables above
FLAGS = -provider=$(PROVIDER) -create-infra=$(CREATE_INFRA) -delete-infra=$(DELETE_INFRA)
ifdef KUBECONFIG
FLAGS += -kubeconfig=$(KUBECONFIG)
endif

# timeout picked by provider
ifeq ($(PROVIDER),kind)
TIMEOUT = $(TIMEOUT_KIND)
else
TIMEOUT = $(TIMEOUT_AZURE)
endif

.PHONY: test-e2e test-basic-metrics test-advanced-metrics test-hubble-metrics test-capture \
        test-basic-metrics-exp test-advanced-metrics-exp help

test-e2e: ## run all e2e scenarios, defaults to kind
	$(GO_TEST) $(TIMEOUT) $(FLAGS)

test-basic-metrics: ## run basic metrics scenarios (drop, tcp, dns)
	$(GO_TEST) $(TIMEOUT) -run TestE2ERetina/BasicMetrics$$ $(FLAGS)

test-advanced-metrics: ## run advanced metrics scenarios (dns, latency)
	$(GO_TEST) $(TIMEOUT) -run TestE2ERetina/AdvancedMetrics$$ $(FLAGS)

test-hubble-metrics: ## run hubble metrics scenarios (drop, tcp, dns, flows)
	$(GO_TEST) $(TIMEOUT) -run TestE2ERetina/HubbleMetrics$$ $(FLAGS)

test-capture: ## run packet capture scenarios
	$(GO_TEST) $(TIMEOUT) -run TestE2ERetina/Capture$$ $(FLAGS)

test-basic-metrics-exp: ## run experimental basic metrics scenarios
	$(GO_TEST) $(TIMEOUT) -run TestE2ERetina/BasicMetricsExperimental$$ $(FLAGS)

test-advanced-metrics-exp: ## run experimental advanced metrics scenarios
	$(GO_TEST) $(TIMEOUT) -run TestE2ERetina/AdvancedMetricsExperimental$$ $(FLAGS)

help: ## show this help
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  %-26s %s\n", $$1, $$2}'
