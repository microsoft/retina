variable "location" {
  description = "The VM location."
  type        = string
  default     = "UK South"
}

variable "resource_group_name" {
  description = "The name of the resource group."
  type        = string
}

variable "prefix" {
  description = "A prefix to add to all resources."
  type        = string
  default     = "example-vm"
}

variable "labels" {
  description = "A map of labels to add to all resources."
  type        = map(string)
  default     = {}
}

variable "network_profile" {
  description = "Network profile configuration"
  type = object({
    network_plugin      = string
    network_plugin_mode = string
    load_balancer_profile = object({
      managed_outbound_ip_count = number
    })
    pod_cidr       = string
    service_cidr   = string
    dns_service_ip = string
    outbound_type  = string
  })
  default = {
    network_plugin      = "azure"
    network_plugin_mode = "overlay"
    load_balancer_profile = {
      managed_outbound_ip_count = 1
    }
    pod_cidr       = "10.244.0.0/16"
    service_cidr   = "10.0.0.0/16"
    dns_service_ip = "10.0.0.10"
    outbound_type  = "loadBalancer"
  }
}

variable "default_node_pool" {
  description = "Default node pool configuration"
  type = object({
    name            = string
    node_count      = number
    vm_size         = string
    os_disk_size_gb = number
    os_disk_type    = string
    max_pods        = number
    type            = string
    node_labels     = map(string)
  })
  default = {
    name            = "agentpool"
    node_count      = 2
    vm_size         = "Standard_D4ds_v5"
    os_disk_size_gb = 128
    os_disk_type    = "Ephemeral"
    max_pods        = 110
    type            = "VirtualMachineScaleSets"
    node_labels     = {}
  }
}

variable "kubernetes_version" {
  description = "The version of Kubernetes to use for the AKS cluster."
  type        = string
  default     = "1.29.8"
}
