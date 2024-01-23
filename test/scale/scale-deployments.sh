#!/bin/bash
numReplicas=100
# 50 deployments * 100 replicas = 5000 pods

set -e
numNamespaces=`kubectl get ns | grep test-ns- | wc -l`
numDeployments=`ls deployments/ | wc -l`
desiredNumPods=$(( numDeployments * numNamespaces * numReplicas ))

startTime=`date -u`
summary="$numDeployments deployments per $numNamespaces namespaces to $numReplicas replicas each. Total of $desiredNumPods pods"
echo "scaling $summary"
for (( i=1; i<=$numNamespaces; i++ )); do
    for (( j=1; j<=$numDeployments; j++ )); do
        kubectl scale -n test-ns-$i deployment/test-deployment-$j --replicas=$numReplicas
    done
done

## the rest is copied from create-deployments.sh
echo
echo "start time: $startTime"
echo "scaled $summary"
echo "waiting for $desiredNumPods pods to come up"
while true; do
    numPods=`kubectl get pod -A | grep test-ns- | grep Running | wc -l`
    endTime=`date -u`
    if [[ $numPods == $desiredNumPods ]]; then
        break
    fi
    elapsedTime=$(( $(date -d "$endTime" '+%s') - $(date -d "$startTime" '+%s') ))
    echo "$numPods pods up, want $desiredNumPods. Elapsed time: $(( elapsedTime / 60 )) minutes $(( elapsedTime % 60 )) seconds"
    sleep 15
done
echo
echo "DONE. All $desiredNumPods pods are running"
elapsedTime=$(( $(date -d "$endTime" '+%s') - $(date -d "$startTime" '+%s') ))
echo "Elapsed time: $(( elapsedTime / 60 )) minutes $(( elapsedTime % 60 )) seconds"
echo "start time: $startTime"
echo "end time: $endTime"
