#!/bin/bash
set -e
numNamespaces=`kubectl get ns | grep test-ns- | wc -l`
numClusterrolebindings=`ls clusterrolebindings/ | wc -l`
startTime=`date -u`

echo "creating $numClusterrolebindings clusterrolebindings"
for (( i=1; i<=$numNamespaces; i++ )); do
    kubectl -n test-ns-$i apply -f clusterrolebindings/
done

endTime=`date -u`
echo
echo "DONE. All $numClusterrolebindings clusterrolebindings are created"
elapsedTime=$(( $(date -d "$endTime" '+%s') - $(date -d "$startTime" '+%s') ))
echo "Elapsed time: $(( elapsedTime / 60 )) minutes $(( elapsedTime % 60 )) seconds"
echo "start time: $startTime"
echo "end time: $endTime"
