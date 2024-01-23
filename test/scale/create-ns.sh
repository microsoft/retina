#!/bin/bash
numNamespaces=1

set -e

startTime=`date -u`
summary="creating $numNamespaces namespaces"
echo "$summary"
for (( i=1; i<=$numNamespaces; i++ )); do
    kubectl create namespace test-ns-$i
done
