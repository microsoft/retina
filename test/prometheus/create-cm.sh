#!/bin/bash
kubectl apply -f ./test/prometheus/service.yaml

kubectl delete configmap ama-metrics-prometheus-config-node -n kube-system
kubectl create configmap ama-metrics-prometheus-config-node --from-file=./test/prometheus/ama-metrics-node/prometheus-config -n kube-system 

kubectl delete configmap ama-metrics-prometheus-config -n kube-system
# kubectl create configmap ama-metrics-prometheus-config --from-file=./test/prometheus/ama-metrics/prometheus-config -n kube-system 
kubectl apply -f ./test/prometheus/ama-metrics/prometheus-config -n kube-system

kubectl rollout restart ds ama-metrics-node -n kube-system
kubectl rollout restart deploy ama-metrics -n kube-system
