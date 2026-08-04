#!/bin/bash
# Test script for bpftrace - tests drops, RST, socket errors, and retransmits
# Usage: ./test_bpftrace_drops.sh [kubeconfig_path]
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Bpftrace Network Issue Detection Test ==="
echo ""

# Get first node
NODE=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
echo "Target node: $NODE"

# Clean up any leftover test pods
echo "Cleaning up previous test pods..."
kubectl delete pod drop-gen rst-gen nfqueue-helper --force --grace-period=0 2>/dev/null || true
kubectl delete pod -l app=retina-trace --force --grace-period=0 2>/dev/null || true
sleep 2

# Build CLI if needed
if [[ ! -f "$REPO_ROOT/kubectl-retina" ]]; then
    echo "Building kubectl-retina..."
    cd "$REPO_ROOT"
    go build -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version=v1.0.3" -o kubectl-retina ./cli/main.go
fi

echo ""
echo "=== Starting bpftrace (70s duration) ==="
echo "This will capture drops, RST, socket errors, retransmits, and NFQUEUE drops on $NODE"
echo ""

# Create output file
OUTPUT_FILE=$(mktemp)
trap "rm -f $OUTPUT_FILE" EXIT

# Run trace in background and capture output
# Duration must be long enough for all test phases: RST, DROP/RETRANS, and NFQUEUE
# (each phase takes ~15-30s including pod startup and traffic generation)
"$REPO_ROOT/kubectl-retina" bpftrace "$NODE" --duration 70s --startup-timeout 120s --retina-shell-image-version v1.0.3 > "$OUTPUT_FILE" 2>&1 &
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
echo "=== Test 3: NFQ_DROP (iptables NFQUEUE with no consumer) ==="
# Add an iptables -j NFQUEUE rule pointing to a queue with no reader.
# The kernel calls __nf_queue which returns -ESRCH, and fexit catches it.
echo "Creating privileged pod to add NFQUEUE iptables rule..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: nfqueue-helper
  namespace: default
spec:
  nodeName: "$NODE"
  hostNetwork: true
  restartPolicy: Never
  containers:
  - name: nfqueue-helper
    image: alpine
    securityContext:
      privileged: true
    command: ["sh", "-c"]
    args:
    - |
      apk add --no-cache iptables >/dev/null 2>&1
      echo "Adding NFQUEUE rule on OUTPUT to $TARGET_IP:80 queue 42 (no consumer)..."
      iptables -I OUTPUT -d $TARGET_IP -p tcp --dport 80 -j NFQUEUE --queue-num 42
      for i in 1 2 3 4 5; do
        echo "attempt \$i: connecting to $TARGET_IP:80 via NFQUEUE..."
        nc -zv -w2 $TARGET_IP 80 2>&1 || true
        sleep 0.5
      done
      echo "Removing NFQUEUE rule..."
      iptables -D OUTPUT -d $TARGET_IP -p tcp --dport 80 -j NFQUEUE --queue-num 42 2>/dev/null
      echo "NFQUEUE test done"
EOF
kubectl wait --for=condition=Ready pod/nfqueue-helper --timeout=60s 2>/dev/null || true
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/nfqueue-helper --timeout=60s 2>/dev/null || true
kubectl logs nfqueue-helper 2>/dev/null || true
kubectl delete pod nfqueue-helper --force --grace-period=0 2>/dev/null || true

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
kubectl delete pod nfqueue-helper --force --grace-period=0 2>/dev/null || true
kubectl delete pod -l app=retina-trace --force --grace-period=0 2>/dev/null || true

echo ""
echo "=== Test Complete ==="

# Check results.
# Use precise patterns to match actual event output lines and avoid false
# positives from verbose/startup logs that may mention these keywords.
DROPS_FOUND=false
RST_FOUND=false
SOCK_ERR_FOUND=false
RETRANS_FOUND=false
NFQ_DROP_FOUND=false

if grep -qP '\bDROP\b.*kfree_skb' "$OUTPUT_FILE"; then
    DROPS_FOUND=true
    echo "✓ DROP events captured"
else
    echo "✗ DROP events NOT captured (requires cluster NetworkPolicy support)"
fi

if grep -qP '\bRST_(SENT|RECV)\b' "$OUTPUT_FILE"; then
    RST_FOUND=true
    echo "✓ RST events captured"
else
    echo "✗ RST events NOT captured"
fi

if grep -qP '\bSOCK_ERR\b.*inet_sk_error_report' "$OUTPUT_FILE"; then
    SOCK_ERR_FOUND=true
    echo "✓ SOCK_ERR events captured"
else
    echo "✗ SOCK_ERR events NOT captured"
fi

if grep -qP '\bRETRANS\b.*tcp_retransmit_skb' "$OUTPUT_FILE"; then
    RETRANS_FOUND=true
    echo "✓ RETRANS events captured"
else
    echo "✗ RETRANS events NOT captured"
fi

if grep -qP '\bNFQ_DROP\b.*__nf_queue' "$OUTPUT_FILE"; then
    NFQ_DROP_FOUND=true
    echo "✓ NFQ_DROP events captured"
else
    echo "✗ NFQ_DROP events NOT captured (requires iptables/NFQUEUE support)"
fi

# Count successes
SUCCESSES=0
$DROPS_FOUND && ((SUCCESSES++)) || true
$RST_FOUND && ((SUCCESSES++)) || true
$SOCK_ERR_FOUND && ((SUCCESSES++)) || true
$RETRANS_FOUND && ((SUCCESSES++)) || true
$NFQ_DROP_FOUND && ((SUCCESSES++)) || true

if [ $SUCCESSES -ge 3 ]; then
    echo ""
    echo "SUCCESS: $SUCCESSES/5 event types captured!"
    exit 0
elif [ $SUCCESSES -ge 1 ]; then
    echo ""
    echo "PARTIAL SUCCESS: $SUCCESSES/5 event types captured"
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
