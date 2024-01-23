#!/bin/bash
set -e
numNamespaces=`kubectl get ns | grep test-ns- | wc -l`
numServiceAccounts=`ls serviceaccounts/ | wc -l`
startTime=`date -u`

echo "creating $numServiceAccounts service accounts"
kubectl apply -f serviceaccounts/

endTime=`date -u`
echo
echo "DONE. All $numServiceAccounts service accounts are created"
elapsedTime=$(( $(date -d "$endTime" '+%s') - $(date -d "$startTime" '+%s') ))
echo "Elapsed time: $(( elapsedTime / 60 )) minutes $(( elapsedTime % 60 )) seconds"
echo "start time: $startTime"
echo "end time: $endTime"
