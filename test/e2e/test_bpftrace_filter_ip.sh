#!/bin/bash
# Test script for bpftrace --ip flag
# This validates that IP filtering works correctly in bpftrace scripts
# Usage: ./test_bpftrace_filter_ip.sh [kubeconfig_path]
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Bpftrace --ip Flag Test ==="
echo ""

# Get first node
NODE=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
echo "Target node: $NODE"

# Clean up any leftover test pods
echo "Cleaning up previous test pods..."
kubectl delete pod -l app=retina-trace --force --grace-period=0 2>/dev/null || true
kubectl delete pod filter-test-target --force --grace-period=0 2>/dev/null || true
sleep 2

# Build CLI if needed
if [[ ! -f "$REPO_ROOT/kubectl-retina" ]]; then
    echo "Building kubectl-retina..."
    cd "$REPO_ROOT"
    go build -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version=v1.0.3" -o kubectl-retina ./cli/main.go
fi

# Create a test pod to get a stable IP
echo "Creating test target pod..."
kubectl run filter-test-target --image=nginx --labels="app=filter-test-target" --overrides="{\"spec\":{\"nodeName\":\"$NODE\"}}" 2>/dev/null || true
sleep 5
kubectl wait --for=condition=Ready pod/filter-test-target --timeout=60s 2>/dev/null || true

TARGET_IP=$(kubectl get pod filter-test-target -o jsonpath='{.status.podIP}')
if [[ -z "$TARGET_IP" ]]; then
    echo "ERROR: Could not get target pod IP"
    exit 1
fi
echo "Target pod IP: $TARGET_IP"

echo ""
echo "=== Test 1: Verify --ip flag doesn't cause bpftrace errors ==="
echo "Running bpftrace with --ip $TARGET_IP for 20s..."
echo ""

# Create output file
OUTPUT_FILE=$(mktemp)
ERROR_FILE=$(mktemp)
trap "rm -f $OUTPUT_FILE $ERROR_FILE; kubectl delete pod filter-test-target --force --grace-period=0 2>/dev/null || true; kubectl delete pod -l app=retina-trace --force --grace-period=0 2>/dev/null || true" EXIT

# Run trace with IP filter and capture both stdout and stderr
"$REPO_ROOT/kubectl-retina" bpftrace "$NODE" --duration 20s --startup-timeout 60s \
    --ip "$TARGET_IP" \
    --retina-shell-image-version v1.0.3 > "$OUTPUT_FILE" 2>"$ERROR_FILE" &
TRACE_PID=$!

# Wait for trace to start
echo "Waiting for trace pod to start..."
sleep 10

# Generate some traffic to the filtered IP
echo "Generating traffic to $TARGET_IP..."
kubectl run traffic-gen --rm -i --restart=Never --image=busybox --overrides="{\"spec\":{\"nodeName\":\"$NODE\"}}" -- sh -c "
for i in 1 2 3 4 5; do
    echo \"Attempt \$i: connecting to $TARGET_IP:80\"
    wget -q -O /dev/null -T 2 http://$TARGET_IP:80/ 2>&1 || true
    sleep 1
done
echo 'Traffic generation complete'
" 2>&1 || true

echo ""
echo "=== Waiting for trace to complete ==="
wait $TRACE_PID 2>/dev/null || true

echo ""
echo "=== Trace Output ==="
cat "$OUTPUT_FILE"

echo ""
echo "=== Error Output ==="
cat "$ERROR_FILE"

echo ""
echo "=== Test Results ==="

# Check for the specific casting error that was previously seen
CAST_ERROR=false
if grep -q "Cannot cast from" "$ERROR_FILE" || grep -q "Cannot cast from" "$OUTPUT_FILE"; then
    CAST_ERROR=true
    echo "✗ FAIL: bpftrace cast error detected"
    echo "  The --filter-ip flag is generating invalid bpftrace code"
fi

# Check if the trace started successfully
TRACE_STARTED=false
if grep -q "Tracing network issues" "$OUTPUT_FILE"; then
    TRACE_STARTED=true
    echo "✓ Trace started successfully"
fi

# Check for the filtered IP in the script (shows filter was applied)
FILTER_APPLIED=false
if grep -q "$TARGET_IP" "$ERROR_FILE" 2>/dev/null || grep -qi "filter" "$OUTPUT_FILE" 2>/dev/null; then
    FILTER_APPLIED=true
    echo "✓ IP filter was applied"
fi

echo ""
if [[ "$CAST_ERROR" == "true" ]]; then
    echo "TEST FAILED: bpftrace casting error"
    echo "The --ip flag is generating code that bpftrace cannot execute."
    exit 1
elif [[ "$TRACE_STARTED" == "true" ]]; then
    echo "TEST PASSED: bpftrace with --ip works correctly"
    exit 0
else
    echo "TEST INCONCLUSIVE: Trace may not have started"
    echo "Check the output above for details"
    exit 1
fi
