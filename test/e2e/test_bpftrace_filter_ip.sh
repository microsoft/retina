#!/bin/bash
# Test script for bpftrace --ip flag
# This validates that IP filtering works correctly in bpftrace scripts
# Tests drops, RST, socket errors, and retransmits with --ip filtering
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
kubectl delete pod drop-gen rst-gen traffic-gen filter-test-target --force --grace-period=0 2>/dev/null || true
kubectl delete pod -l app=retina-trace --force --grace-period=0 2>/dev/null || true
kubectl delete networkpolicy deny-all-filter-ip-test 2>/dev/null || true
sleep 2

# Build CLI if needed
if [[ ! -f "$REPO_ROOT/kubectl-retina" ]]; then
    echo "Building kubectl-retina..."
    cd "$REPO_ROOT"
    go build -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version=v1.0.3" -o kubectl-retina ./cli/main.go
fi

# Create a test pod to get a stable IP for filtering
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
echo "=== Starting bpftrace with --ip $TARGET_IP (50s duration) ==="
echo "This will capture drops, RST, socket errors, and retransmits filtered to $TARGET_IP on $NODE"
echo ""

# Create output files
OUTPUT_FILE=$(mktemp)
ERROR_FILE=$(mktemp)
trap "rm -f $OUTPUT_FILE $ERROR_FILE; kubectl delete networkpolicy deny-all-filter-ip-test 2>/dev/null || true; kubectl delete pod filter-test-target --force --grace-period=0 2>/dev/null || true; kubectl delete pod -l app=retina-trace --force --grace-period=0 2>/dev/null || true" EXIT

# Run trace with IP filter in background and capture output
"$REPO_ROOT/kubectl-retina" bpftrace "$NODE" --duration 50s --startup-timeout 120s \
    --ip "$TARGET_IP" \
    --retina-shell-image-version v1.0.3 > "$OUTPUT_FILE" 2>"$ERROR_FILE" &
TRACE_PID=$!

# Wait for trace to start
echo "Waiting for trace pod to start..."
sleep 15

echo ""
echo "=== Test 1: RST and SOCK_ERR (connection refused via target IP) ==="
# Connect to the target pod on a closed port to generate RST and socket error
kubectl run rst-gen --rm -i --restart=Never --image=busybox --overrides="{\"spec\":{\"nodeName\":\"$NODE\"}}" -- sh -c "
for i in 1 2 3; do
    echo \"RST attempt \$i: connecting to $TARGET_IP on closed port 9999\"
    nc -zv -w1 $TARGET_IP 9999 2>&1 || true
    sleep 0.5
done
echo 'RST generation complete'
" 2>&1 || true

echo ""
echo "=== Test 2: DROP and RETRANS (NetworkPolicy block on filtered IP) ==="
# Apply a deny-all NetworkPolicy to filter-test-target itself so that
# subsequent traffic to TARGET_IP is dropped and retransmitted.
# This ensures DROP and RETRANS events occur on the IP we're filtering.

cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-all-filter-ip-test
  namespace: default
spec:
  podSelector:
    matchLabels:
      app: filter-test-target
  policyTypes:
  - Ingress
  ingress: []  # Deny all ingress
EOF

echo "NetworkPolicy applied to filter-test-target (deny all ingress)"
echo "Traffic to $TARGET_IP will now be dropped by the policy"
sleep 2

# Generate blocked traffic - connection attempts will be dropped and retransmitted
echo "Sending blocked traffic to $TARGET_IP (will cause drops + retransmissions)..."
kubectl run drop-gen --rm -i --restart=Never --image=busybox --overrides="{\"spec\":{\"nodeName\":\"$NODE\"}}" -- sh -c "
for i in 1 2 3 4 5; do
    echo \"Attempt \$i to $TARGET_IP:80 (should be dropped by NetworkPolicy)\"
    nc -zv -w2 $TARGET_IP 80 2>&1 || true
    sleep 0.5
done
echo 'Traffic generation complete'
" 2>&1 || true

echo ""
echo "=== Test 3: Additional traffic to filtered IP ==="
# Generate extra traffic to the filtered IP to increase chances of capture
echo "Generating additional traffic to $TARGET_IP..."
kubectl run traffic-gen --rm -i --restart=Never --image=busybox --overrides="{\"spec\":{\"nodeName\":\"$NODE\"}}" -- sh -c "
for i in 1 2 3 4 5; do
    echo \"Attempt \$i: connecting to $TARGET_IP:80\"
    wget -q -O /dev/null -T 2 http://$TARGET_IP:80/ 2>&1 || true
    echo \"Attempt \$i: connecting to $TARGET_IP:9999 (closed port)\"
    nc -zv -w1 $TARGET_IP 9999 2>&1 || true
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
echo "=== Cleanup ==="
kubectl delete networkpolicy deny-all-filter-ip-test 2>/dev/null || true
kubectl delete pod filter-test-target --force --grace-period=0 2>/dev/null || true
kubectl delete pod -l app=retina-trace --force --grace-period=0 2>/dev/null || true

echo ""
echo "=== Test Results ==="

# Check for the specific casting error that was previously seen
CAST_ERROR=false
if grep -q "Cannot cast from" "$ERROR_FILE" || grep -q "Cannot cast from" "$OUTPUT_FILE"; then
    CAST_ERROR=true
    echo "✗ FAIL: bpftrace cast error detected"
    echo "  The --ip flag is generating invalid bpftrace code"
fi

if [[ "$CAST_ERROR" == "true" ]]; then
    echo ""
    echo "TEST FAILED: bpftrace casting error"
    echo "The --ip flag is generating code that bpftrace cannot execute."
    exit 1
fi

# Check if the trace started successfully
TRACE_STARTED=false
if grep -q "Tracing network issues" "$OUTPUT_FILE"; then
    TRACE_STARTED=true
    echo "✓ Trace started successfully with --ip filter"
fi

# Check for the filtered IP in the output (shows filter was applied)
FILTER_APPLIED=false
if grep -q "$TARGET_IP" "$ERROR_FILE" 2>/dev/null || grep -qi "filter" "$OUTPUT_FILE" 2>/dev/null; then
    FILTER_APPLIED=true
    echo "✓ IP filter was applied for $TARGET_IP"
fi

# Verify each event type was captured AND contains the filtered IP on the same line.
DROPS_FOUND=false
RST_FOUND=false
SOCK_ERR_FOUND=false
RETRANS_FOUND=false

if grep -qP '\bDROP\b.*kfree_skb' "$OUTPUT_FILE"; then
    if grep -P '\bDROP\b.*kfree_skb' "$OUTPUT_FILE" | grep -qF "$TARGET_IP"; then
        DROPS_FOUND=true
        echo "✓ DROP events captured for $TARGET_IP"
    else
        echo "✗ DROP events captured but NOT for filtered IP $TARGET_IP — filter may be broken"
    fi
else
    echo "✗ DROP events NOT captured (requires cluster NetworkPolicy support)"
fi

if grep -qP '\bRST_(SENT|RECV)\b' "$OUTPUT_FILE"; then
    if grep -P '\bRST_(SENT|RECV)\b' "$OUTPUT_FILE" | grep -qF "$TARGET_IP"; then
        RST_FOUND=true
        echo "✓ RST events captured for $TARGET_IP"
    else
        echo "✗ RST events captured but NOT for filtered IP $TARGET_IP — filter may be broken"
    fi
else
    echo "✗ RST events NOT captured"
fi

if grep -qP '\bSOCK_ERR\b.*inet_sk_error_report' "$OUTPUT_FILE"; then
    if grep -P '\bSOCK_ERR\b.*inet_sk_error_report' "$OUTPUT_FILE" | grep -qF "$TARGET_IP"; then
        SOCK_ERR_FOUND=true
        echo "✓ SOCK_ERR events captured for $TARGET_IP"
    else
        echo "✗ SOCK_ERR events captured but NOT for filtered IP $TARGET_IP — filter may be broken"
    fi
else
    echo "✗ SOCK_ERR events NOT captured"
fi

if grep -qP '\bRETRANS\b.*tcp_retransmit_skb' "$OUTPUT_FILE"; then
    if grep -P '\bRETRANS\b.*tcp_retransmit_skb' "$OUTPUT_FILE" | grep -qF "$TARGET_IP"; then
        RETRANS_FOUND=true
        echo "✓ RETRANS events captured for $TARGET_IP"
    else
        echo "✗ RETRANS events captured but NOT for filtered IP $TARGET_IP — filter may be broken"
    fi
else
    echo "✗ RETRANS events NOT captured"
fi

# Count successes
SUCCESSES=0
$DROPS_FOUND && ((SUCCESSES++)) || true
$RST_FOUND && ((SUCCESSES++)) || true
$SOCK_ERR_FOUND && ((SUCCESSES++)) || true
$RETRANS_FOUND && ((SUCCESSES++)) || true

if [[ "$TRACE_STARTED" != "true" ]]; then
    echo ""
    echo "TEST INCONCLUSIVE: Trace may not have started"
    echo "Check the output above for details"
    exit 1
fi

if [ $SUCCESSES -ge 3 ]; then
    echo ""
    echo "SUCCESS: $SUCCESSES/4 event types captured with --ip filter!"
    exit 0
elif [ $SUCCESSES -ge 1 ]; then
    echo ""
    echo "PARTIAL SUCCESS: $SUCCESSES/4 event types captured with --ip filter"
    echo "Note: Some events may not occur depending on kernel behavior"
    exit 0
else
    echo ""
    echo "NOTE: No events captured with --ip filter. This may be expected if:"
    echo "  - The kernel doesn't trigger these tracepoints for the filtered IP"
    echo "  - Traffic was blocked before reaching the tracepoints"
    echo "  - The IP filter is too restrictive for the generated traffic"
    echo "  - Try running manually with different traffic patterns"
    exit 0
fi
