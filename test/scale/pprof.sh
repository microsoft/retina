#!/bin/bash

## Script to get high mem usage pods and get retina pprof outputs

## Memory limit before we get the pprof of the running pod
limit=100
## Time duration to collect the cpu profile
profsec=60
## Time duration to collect the traces profile
tracesec=60
echo "Getting pprof output of pods above mem: $limit"
lines=`kubectl top pod -n kube-system | grep retina | awk '{$1=$1;print}' | tr ' ' ',' | tr -d 'm'`
for line in $lines; do
    IFS=','
    read -ra values <<< "$line"
    pod="${values[0]}"
    mem=`echo ${values[2]} | tr -d 'Mi'`
    if [ $mem -gt $limit ]; then
        echo "Getting pprof for $pod with mem $mem"
        kubectl exec -it -n kube-system $pod -- bash -c "apt update && apt install -y curl"
        kubectl exec -it -n kube-system $pod -- bash -c "curl localhost:10093/debug/pprof/trace?seconds=$tracesec -o trace.out && curl localhost:10093/debug/pprof/profile?seconds=$profsec -o profile.out && curl localhost:10093/debug/pprof/heap -o heap.out"
        kubectl cp -n kube-system $pod:/heap.out ./results/$pod/heap.out
        kubectl cp -n kube-system $pod:/trace.out ./results/$pod/trace.out
        kubectl cp -n kube-system $pod:/profile.out ./results/$pod/profile.out
    fi
    IFS=' '
done
