#!/bin/bash
numNamespaces=1

set -e
numDeployments=`ls deployments/ | wc -l`
desiredNumPods=$(( numDeployments * numNamespaces ))

startTime=`date -u`
summary="$desiredNumPods deployments. $numDeployments deployments in $numNamespaces namespaces"
echo "creating $summary"
for (( i=1; i<=$numNamespaces; i++ )); do
    kubectl apply -n test-ns-$i -f deployments/
done

echo
echo "start time: $startTime"
echo "created $summary"
echo "waiting for $desiredNumPods deployments to come up"
while true; do
    numPods=`kubectl get pod -A | grep test-ns- | grep Running | wc -l`
    endTime=`date -u`
    if [[ $numPods == $desiredNumPods ]]; then
        break
    fi
    elapsedTime=$(( $(date -d "$endTime" '+%s') - $(date -d "$startTime" '+%s') ))
    ## 250 should be a variable/calculated. goldpinger pods have 5 replicas per deployment
    echo "$numPods pods up, want 250. Elapsed time: $(( elapsedTime / 60 )) minutes $(( elapsedTime % 60 )) seconds"
    sleep 15
done
echo
echo "DONE. All $desiredNumPods pods are running"
elapsedTime=$(( $(date -d "$endTime" '+%s') - $(date -d "$startTime" '+%s') ))
echo "Elapsed time: $(( elapsedTime / 60 )) minutes $(( elapsedTime % 60 )) seconds"
echo "start time: $startTime"
echo "end time: $endTime"
