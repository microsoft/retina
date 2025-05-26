#!/bin/bash
set -e

# Default test flags
TEST_FLAGS="-test.v"

# Default to run without creating or deleting infrastructure
E2E_FLAGS="-create-infra=false -delete-infra=false"

# Allow environment variable to override default e2e flags
if [ -n "$E2E_TEST_FLAGS" ]; then
  E2E_FLAGS="$E2E_TEST_FLAGS"
fi

# Allow environment variable to override test flags
if [ -n "$GO_TEST_FLAGS" ]; then
  TEST_FLAGS="$GO_TEST_FLAGS"
fi

# Append any additional arguments passed to the script
if [ $# -gt 0 ]; then
  E2E_FLAGS="$E2E_FLAGS $@"
fi

echo "Running e2e tests with the following flags:"
echo "Test flags: $TEST_FLAGS"
echo "E2E flags: $E2E_FLAGS"

# Run the tests
/app/e2e.test $TEST_FLAGS -test.timeout=40m -test.tags=e2e -args $E2E_FLAGS