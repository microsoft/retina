# Unmanaged Prometheus/Grafana
<!-- markdownlint-disable MD029 -->

## Pre-Requisites

1. Create a Kubernetes cluster.
2. Install Retina DaemonSet (see [Quick Installation](./setup.md)).

## Configuring Prometheus

1. In this example, we will install Prometheus via the community supported helm chart. If you already have this chart deployed, skip to step 3.

  ```shell
  helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
  helm repo update
  ```

2. Install the Prometheus chart

  ```shell
  helm install prometheus -n kube-system -f deploy/prometheus/values.yaml prometheus-community/kube-prometheus-stack
  ```

3. Save the Prometheus values below to `deploy/prometheus/values.yaml`

  ```yaml
  prometheus:
    prometheusSpec:
      additionalScrapeConfigs: |
        - job_name: "retina-pods"
          kubernetes_sd_configs:
            - role: pod
          relabel_configs:
            - source_labels: [__meta_kubernetes_pod_container_name]
              action: keep
              regex: retina(.*)
            - source_labels:
                [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
              separator: ":"
              regex: ([^:]+)(?::\d+)?
              target_label: __address__
              replacement: ${1}:${2}
              action: replace
            - source_labels: [__meta_kubernetes_pod_node_name]
              action: replace
              target_label: instance
          metric_relabel_configs:
            - source_labels: [__name__]
              action: keep
              regex: (.*)
  ```

4. Upgrade deployment with

  ```shell
  helm upgrade prometheus -n kube-system -f deploy/prometheus/values.yaml prometheus-community/kube-prometheus-stack
  ```

5. Install Retina

  ```shell
  helm install retina ./deploy/manifests/controller/helm/retina/ --namespace kube-system --dependency-update
  ```

6. Verify that the Retina Pods are being scraped by port-forwarding the Prometheus server:

  ```shell
  kubectl port-forward --namespace kube-system svc/prometheus-operated 9090
  ```

7. Then go to [http://localhost:9090/targets](http://localhost:9090/targets) to see the Retina Pods being discovered and scraped:

![alt text](img/prometheus-retina-pods.png)

## Configuring Grafana

Create a Grafana instance at [grafana.com](https://www.grafana.com) and follow [Configuring Grafana](./configuring-grafana.md).
