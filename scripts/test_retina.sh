set -o xtrace
set -e

#create a test pod to check if retina metrics are available
if ! kubectl get pod retina-test -n kube-system >/dev/null 2>&1; then
    kubectl run retina-test -n kube-system --image=alpine --overrides='{"apiVersion": "v1", "spec": {"nodeSelector": { "kubernetes.io/os": "linux", "kubernetes.io/arch": "amd64" }}}' --command sleep infinity
fi
kubectl -n kube-system wait --for=condition=ready --timeout=300s pod -l run=retina-test

# wait for metrics to get generated
sleep 60s

# Get the list of retina pods
kubectl get pods -n kube-system -o wide -l app=retina

# Get the list of retina pods ips
ips=($(kubectl get pods -n kube-system -l app=retina -o jsonpath='{.items[*].status.podIP}'))

# check if retina metrics are available in every pods
for ip in "${ips[@]}"; do
    echo " 
    ============================================================
    =========Checking Metrics on Pod with Ip: $ip===============
    ============================================================
    "
    result=$(kubectl exec  pod/retina-test -n kube-system -- wget -qO- http://$ip:10093/metrics | grep "networkobservability_")
    
    if [ -z "$result" ]; then
        echo "==============Retina metrics not available================"
        exit 1
    else
        echo "Retina metrics available"
    fi

done

# wait for 3 minutes for logs to be generated
sleep 3m

# Get all retina pods names
pods=($(kubectl get pods -n kube-system -l app=retina -o jsonpath='{.items[*].metadata.name}'))

# Check running plugins in all pods
for pod in "${pods[@]}"; do
    echo " 
    ============================================================
    ================Checking Errors for Pod: $pod===============
    ============================================================
    "
    # get number of errors
    number_of_errors=$(kubectl logs $pod -n kube-system | awk '{print $2}'| grep -w "error"| wc -l)
    if [ $number_of_errors -gt 0 ]; then
        echo " =========================Error Found==================================="
        kubectl logs $pod -n kube-system | grep -w "error"
        echo " ======================================================================="
        exit 1
    fi

    echo "Checking restarts on pod: $pod"
    # get number of restarts
    number_of_restarts=$(kubectl get pods $pod -n kube-system -o jsonpath='{.status.containerStatuses[0].restartCount}')

    if [ $number_of_restarts -gt 0 ]; then
        if [[ $pod == *"agent-win"* ]] && kubectl describe pod $pod -n kube-system | grep -q "SandboxChanged"; then
            echo "============================= Warning! $pod has restarted because of SandboxChanged =========================="
        elif [[ $pod != *"agent-win"* ]]; then
            echo "=============================$pod has restarted=========================="
            kubectl logs $pod -n kube-system -p
            exit 1
        fi
    fi  
done
