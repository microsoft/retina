#!/bin/bash

kubectl delete cm ama-metrics-prometheus-config-node  -n kube-system 

kubectl create configmap ama-metrics-prometheus-config-node --from-file=./deploy/legacy/prometheus/retina-windows/prometheus-config -n kube-system 
