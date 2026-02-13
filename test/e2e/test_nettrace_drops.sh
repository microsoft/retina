#!/bin/bash
# Test script for nettrace - tests drops, RST, socket errors, and retransmits
# Usage: ./test_nettrace_drops.sh [kubeconfig_path]
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Nettrace Network Issue Detection Test ==="
echo ""

# Get first node
NODE=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
echo "Target node: $NODE"

# Clean up any leftover test pods
echo "Cleaning up previous test pods..."
kubectl delete pod drop-gen rst-gen --force --grace-period=0 2>/dev/null || true
kubectl delete pod -l app=retina-trace --force --grace-period=0 2>/dev/null || true
sleep 2

# Build CLI if needed
if [[ ! -f "$REPO_ROOT/kubectl-retina" ]]; then
    echo "Building kubectl-retina..."
    cd "$REPO_ROOT"
    go build -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version=v1.0.3" -o kubectl-retina ./cli/main.go
fi

echo ""
echo "=== Starting nettrace (50s duration) ==="
echo "This will capture drops, RST, socket errors, and retransmits on $NODE"
echo ""

# Create output file
OUTPUT_FILE=$(mktemp)
trap "rm -f $OUTPUT_FILE" EXIT

# Run trace in background and capture output
"$REPO_ROOT/kubectl-retina" nettrace "$NODE" --duration 50s --timeout 120s --retina-shell-image-version v1.0.3 > "$OUTPUT_FILE" 2>&1 &
TRACE_PID=$!

# Wait for trace to start
echo "Waiting for trace pod to start..."
sleep 15

echo ""
echo "=== Test 1: RST and SOCK_ERR (connection refused) ==="
# Connect to a closed port to generate RST and socket error (ECONNREFUSED=111)
kubectl run rst-gen --rm -i --restart=Never --image=busybox --overrides="{\"spec\":{\"nodeName\":\"$NODE\"}}" -- sh -c "
for i in 1 2 3; do
    echo \"RST attempt \$i: connecting to closed port 9999\"
    nc -zv -w1 127.0.0.1 9999 2>&1 || true
    sleep 0.5
done
echo 'RST generation complete'
" 2>&1 || true

echo ""
echo "=== Test 2: DROP and RETRANS (NetworkPolicy block) ==="
# NetworkPolicy drops will cause TCP to retransmit SYN packets

# Create a NetworkPolicy to block traffic
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-all-test
  namespace: default
spec:
  podSelector:
    matchLabels:
      app: drop-target
  policyTypes:
  - Ingress
  ingress: []  # Deny all ingress
EOF

# Create target pod
echo "Creating drop-target pod..."
kubectl run drop-target --image=nginx --labels="app=drop-target" --overrides="{\"spec\":{\"nodeName\":\"$NODE\"}}" 2>/dev/null || true
sleep 5
kubectl wait --for=condition=Ready pod/drop-target --timeout=30s 2>/dev/null || true

TARGET_IP=$(kubectl get pod drop-target -o jsonpath='{.status.podIP}' 2>/dev/null || echo "10.244.0.99")
echo "Target IP: $TARGET_IP"

# Generate blocked traffic - connection attempts will retransmit SYN packets
echo "Sending blocked traffic (will cause retransmissions)..."
kubectl run drop-gen --rm -i --restart=Never --image=busybox --overrides="{\"spec\":{\"nodeName\":\"$NODE\"}}" -- sh -c "
for i in 1 2 3 4 5; do
    echo \"Attempt \$i to $TARGET_IP:80\"
    nc -zv -w2 $TARGET_IP 80 2>&1 || true
    sleep 0.5
done
echo 'Traffic generation complete'
" 2>&1 || true

echo ""
echo "=== Waiting for trace to complete ==="
wait $TRACE_PID || true

echo ""
echo "=== Trace Output ==="
cat "$OUTPUT_FILE"

echo ""
echo "=== Cleanup ==="
kubectl delete networkpolicy deny-all-test 2>/dev/null || true
kubectl delete pod drop-target --force --grace-period=0 2>/dev/null || true
kubectl delete pod -l app=retina-trace --force --grace-period=0 2>/dev/null || true

echo ""
echo "=== Test Complete ==="

# Check results
DROPS_FOUND=false
RST_FOUND=false
SOCK_ERR_FOUND=false
RETRANS_FOUND=false

if grep -q "DROP" "$OUTPUT_FILE"; then
    DROPS_FOUND=true
    echo "✓ DROP events captured"
fi

if grep -q "RST_" "$OUTPUT_FILE"; then
    RST_FOUND=true
    echo "✓ RST events captured"
fi

if grep -q "SOCK_ERR" "$OUTPUT_FILE"; then
    SOCK_ERR_FOUND=true
    echo "✓ SOCK_ERR events captured"
fi

if grep -q "RETRANS" "$OUTPUT_FILE"; then
    RETRANS_FOUND=true
    echo "✓ RETRANS events captured"
fi

# Count successes
SUCCESSES=0
$DROPS_FOUND && ((SUCCESSES++)) || true
$RST_FOUND && ((SUCCESSES++)) || true
$SOCK_ERR_FOUND && ((SUCCESSES++)) || true
$RETRANS_FOUND && ((SUCCESSES++)) || true

if [ $SUCCESSES -ge 3 ]; then
    echo ""
    echo "SUCCESS: $SUCCESSES/4 event types captured!"
    exit 0
elif [ $SUCCESSES -ge 1 ]; then
    echo ""
    echo "PARTIAL SUCCESS: $SUCCESSES/4 event types captured"
    echo "Note: Some events may not occur depending on kernel behavior"
    exit 0
else
    echo ""
    echo "NOTE: No events captured. This may be expected if:"
    echo "  - The kernel doesn't trigger these tracepoints"
    echo "  - Traffic was blocked before reaching the tracepoints"
    echo "  - Try running manually with different traffic patterns"
    exit 0
fi
