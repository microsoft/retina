#!/bin/bash
set -e
numNamespaces=`kubectl get ns | grep test-ns- | wc -l`
numPolicies=`ls netpols/ | wc -l`
totalNetPols=$(( numPolicies * numNamespaces ))
startTime=`date -u`

echo "creating $totalNetPols network policies, $numPolicies in each of $numNamespaces namespaces"
for (( i=1; i<=$numNamespaces; i++ )); do
    kubectl -n test-ns-$i apply -f netpols/
done

endTime=`date -u`
echo
echo "DONE. All $totalNetPols policies are created"
elapsedTime=$(( $(date -d "$endTime" '+%s') - $(date -d "$startTime" '+%s') ))
echo "Elapsed time: $(( elapsedTime / 60 )) minutes $(( elapsedTime % 60 )) seconds"
echo "start time: $startTime"
echo "end time: $endTime"
