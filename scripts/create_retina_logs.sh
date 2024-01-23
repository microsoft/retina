#!/bin/bash

OUTPUT_DIR=$1
TAG=$2

echo "Getting retina pods and nodes wide informations"

echo "kubectl get pods -n kube-system -o wide -l app=retina" > $OUTPUT_DIR/retina_agents_pods_$TAG.txt
kubectl get pods -n kube-system -o wide -l app=retina >> $OUTPUT_DIR/retina_agents_pods_$TAG.txt

echo "kubectl get nodes -o wide" > $OUTPUT_DIR/retina_cluster_nodes_$TAG.txt
kubectl get nodes -o wide >> $OUTPUT_DIR/retina_cluster_nodes_$TAG.txt

# get retina pods
pods=($(kubectl get pods -n kube-system -l app=retina -o jsonpath='{.items[*].metadata.name}'))

echo "Getting retina pods descriptions"
for pod in "${pods[@]}"; do
    kubectl describe pod $pod -n kube-system > $OUTPUT_DIR/${pod}_description_$TAG.txt
done

echo "Getting retina pods logs"
for pod in "${pods[@]}"; do
    kubectl logs $pod -n kube-system > $OUTPUT_DIR/${pod}_logs_$TAG.log
    #check if pod restarted and get previous logs
    number_of_restarts=$(kubectl get pods $pod -n kube-system -o jsonpath='{.status.containerStatuses[0].restartCount}')
    if [ $number_of_restarts -gt 0 ]; then
        echo "$pod has restarted"
        kubectl logs $pod -n kube-system --previous > $OUTPUT_DIR/${pod}_logs_previous_$TAG.log
    fi
done
