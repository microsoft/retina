#!/bin/bash

kubectl delete cm ama-metrics-prometheus-config-node  -n kube-system 
kubectl create configmap ama-metrics-prometheus-config-node --from-file=./deploy/prometheus/cilium/prometheus-config -n kube-system 
k rollout restart ds ama-metrics-node -n kube-system
