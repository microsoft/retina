
.PHONY: longrunning
longrunning: 
	@echo "Running long running tests"
		@echo "Be sure to set CLUSTER_NAME, TAG, AZURE_LOCATION, AZURE_SUBSCRIPTION_ID"
		@if [ -z "$$CLUSTER_NAME" ]; then echo "CLUSTER_NAME is not set"; exit 1; fi
		@if [ -z "$$AZURE_LOCATION" ]; then echo "AZURE_LOCATION is not set"; exit 1; fi
		@if [ -z "$$AZURE_SUBSCRIPTION_ID" ]; then echo "AZURE_SUBSCRIPTION_ID is not set"; exit 1; fi
		@if [ -z "$$TAG" ]; then echo "TAG is not set"; exit 1; fi
	go test -v ./*.go -timeout 0 -tags=e2e -count=1  \
	-run=TestLongRunningRetina \
	-args \
	-create-infra=false \
	-delete-infra=false \
	-image-namespace=microsoft/retina \
	-image-registry=acnpublic.azurecr.io 
