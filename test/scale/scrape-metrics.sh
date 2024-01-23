#!/bin/bash
numPods=10
sleepSeconds=$((60*3))
csvFile=latencies.csv
tmpFile=scrape-temp.txt

echo "logs/ folders. Will fail if it exists"
set -e
mkdir logs
if [ -f $csvFile ]; then
    echo "File $csvFile already exists."
    exit 1
fi

# shuf generates random permutations
retinaPods=`kubectl get pod -n kube-system | grep retina | grep Running | awk '{print $1}' | shuf -n $numPods`
echo "OBSERVING THESE PODS"
echo $retinaPods
echo

for pod in $retinaPods; do
    echo "capturing log for $pod in background"
    kubectl logs -f $pod -n kube-system > logs/$pod.txt &
done

for pod in $retinaPods; do
    echo "installing curl on $pod"
    kubectl exec -n kube-system $pod -- bash -c "apt update && apt install -y curl"
done

round=1
while true; do
    echo "scraping node metrics"
    for pod in $retinaPods; do
        time=`date -u`
        # exec in pod, curl localhost:10093/metrics and grab metrics
    done
    echo "finished scraping node metrics for round $round"
    round=$((round+1))
    sleep $sleepSeconds
done
