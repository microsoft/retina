#!/bin/bash
sleepSeconds=65

echo "time,pod,cpu,mem" > cpu-and-mem-pod-results.csv
echo "time,node,cpu,cpuPercent,mem,memPercent" > cpu-and-mem-node-results.csv
while true; do
    currentTime=`date -u`
    echo "running k top pod"
    lines=`kubectl top pod -n kube-system | grep retina | awk '{$1=$1;print}' | tr ' ' ',' | tr -d 'm' | tr -d 'Mi'`
    for line in $lines; do
        echo "$currentTime,$line" >> cpu-and-mem-pod-results.csv
    done

    currentTime=`date -u`
    echo "running k top node"
    lines=`kubectl top node | grep -v NAME | awk '{$1=$1;print}' | tr ' ' ',' | tr -d 'm' | tr -d 'Mi' | tr -d '%'`
    for line in $lines; do
        echo "$currentTime,$line" >> cpu-and-mem-node-results.csv
    done

    echo `date -u` >> cpu-and-mem-running-pods.out
    kubectl get pod -n kube-system | grep retina >> cpu-and-mem-running-pods.out
    echo " " >> cpu-and-mem-running-pods.out

    echo "sleeping $sleepSeconds seconds"
    sleep $sleepSeconds
done
