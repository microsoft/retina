#!/bin/bash
kubectl apply -f ./test/prometheus/service.yaml
kubectl delete configmap ama-metrics-prometheus-config-node -n kube-system
kubectl create configmap ama-metrics-prometheus-config-node --from-file=./test/prometheus/prometheus-config -n kube-system 
kubectl rollout restart ds ama-metrics-node -n kube-system
