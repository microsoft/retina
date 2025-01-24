resource "azurerm_resource_group" "aks_rg" {
  name     = var.resource_group_name
  location = var.location
}

resource "azurerm_kubernetes_cluster" "aks" {
  name                = "${var.prefix}-aks"
  location            = azurerm_resource_group.aks_rg.location
  resource_group_name = azurerm_resource_group.aks_rg.name
  dns_prefix          = "${var.prefix}-aks-dns"
  kubernetes_version  = "1.29.8"

  default_node_pool {
    name            = "agentpool"
    node_count      = 2
    vm_size         = "Standard_D4ds_v5"
    os_disk_size_gb = 128
    os_disk_type    = "Ephemeral"
    max_pods        = 110
    type            = "VirtualMachineScaleSets"
    node_labels     = var.labels
  }

  identity {
    type = "SystemAssigned"
  }

  network_profile {
    network_plugin      = "azure"
    network_plugin_mode = "overlay"
    load_balancer_profile {
      managed_outbound_ip_count = 1
    }
    pod_cidr       = "10.244.0.0/16"
    service_cidr   = "10.0.0.0/16"
    dns_service_ip = "10.0.0.10"
    outbound_type  = "loadBalancer"
  }

  tags = var.labels
}
