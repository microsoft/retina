#!/bin/bash

kubectl create configmap ama-metrics-prometheus-config-node --from-file=./deploy/standard/prometheus/retina/prometheus-config -n kube-system 
