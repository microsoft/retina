locals {
  location            = "uksouth"
  resource_group_name = "mc-rg"
  prefix              = "mc"

  retina_release_name      = "retina"
  retina_release_namespace = "kube-system"
  retina_repository_url    = "oci://ghcr.io/microsoft/retina/charts"
  retina_chart_version     = "v0.0.29"
  retina_chart_name        = "retina-hubble"
  retina_values            = yamldecode(file("../files/retina-hubble.yaml"))

  prometheus_release_name      = "prometheus"
  prometheus_release_namespace = "kube-system"
  prometheus_repository_url    = "https://prometheus-community.github.io/helm-charts"
  prometheus_chart_version     = "68.4.3"
  prometheus_chart_name        = "kube-prometheus-stack"
  prometheus_values            = yamldecode(file("../../../../deploy/hubble/prometheus/values.yaml"))

  aks_security_rules = [
    {
      name                       = "Allow_Prometheus_Inbound"
      priority                   = 100
      direction                  = "Inbound"
      access                     = "Allow"
      protocol                   = "Tcp"
      source_port_range          = "*"
      source_address_prefix      = "*"
      destination_port_range     = "9090"
      destination_address_prefix = module.prometheus_lb_aks.ip
    },
    {
      name                       = "Allow_Prometheus_Outbound"
      priority                   = 100
      direction                  = "Outbound"
      access                     = "Allow"
      protocol                   = "Tcp"
      source_port_range          = "9090"
      source_address_prefix      = module.prometheus_lb_aks.ip
      destination_port_range     = "*"
      destination_address_prefix = "*"
    },
  ]

  default_node_pool = {
    name            = "agentpool"
    node_count      = 3
    vm_size         = "standard_a2_v2"
    os_disk_size_gb = 128
    os_disk_type    = "Managed"
    max_pods        = 110
    type            = "VirtualMachineScaleSets"
    node_labels     = {}
  }

  # Make sure dashboards are deployed only once
  # if anything is passed here, then
  # live/retina-eks and live/retina-gke 
  # cannot have the same values since we are using
  # a single Grafana instance
  dashboards = {
    "clusters"                   = "clusters.json"
    "hubble-dns"                 = "hubble-dns.json"
    "hubble-pod-flows-namespace" = "hubble-pod-flows-namespace.json"
    "hubble-pod-flows-workload"  = "hubble-pod-flows-workload.json"
    "standard-dns"               = "standard-dns.json"
    "standard-pod-level"         = "standard-pod-level.json"
  }
}