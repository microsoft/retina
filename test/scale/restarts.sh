#!/bin/bash

## Script to get the previous logs of pods that have restarted on the cluster

echo "pod,restarts,error" > pod-restarts.out
lines=`kubectl get pod -n kube-system | awk '{$1=$1;print}' | grep retina | tr ' ' ','`
for line in $lines; do
    IFS=','
    read -ra values <<< "$line"
    pod="${values[0]}"
    restarts="${values[3]}"
    echo "Getting previous logs for $pod with $restarts restarts"
    if [ $restarts -gt 0 ]; then
        err=`kubectl logs -n kube-system $pod --previous`
        echo "$pod,$restarts,$err" >> ./results/pod-restarts.out 
    fi
    IFS=' '
done
