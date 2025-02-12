resource "azurerm_resource_group" "aks_rg" {
  name     = var.resource_group_name
  location = var.location
}

resource "azurerm_virtual_network" "aks_vnet" {
  name                = "${var.prefix}-vnet"
  address_space       = var.vnet_address_space
  location            = azurerm_resource_group.aks_rg.location
  resource_group_name = azurerm_resource_group.aks_rg.name
}

resource "azurerm_subnet" "aks_subnet" {
  name                 = "${var.prefix}-subnet"
  resource_group_name  = azurerm_resource_group.aks_rg.name
  virtual_network_name = azurerm_virtual_network.aks_vnet.name
  address_prefixes     = var.subnet_address_space
}

resource "azurerm_kubernetes_cluster" "aks" {
  name                = "${var.prefix}-aks"
  location            = azurerm_resource_group.aks_rg.location
  resource_group_name = azurerm_resource_group.aks_rg.name
  dns_prefix          = "${var.prefix}-aks-dns"
  kubernetes_version  = var.kubernetes_version

  dynamic "default_node_pool" {
    for_each = [var.default_node_pool]
    content {
      name            = default_node_pool.value.name
      node_count      = default_node_pool.value.node_count
      vm_size         = default_node_pool.value.vm_size
      os_disk_size_gb = default_node_pool.value.os_disk_size_gb
      os_disk_type    = default_node_pool.value.os_disk_type
      max_pods        = default_node_pool.value.max_pods
      type            = default_node_pool.value.type
      node_labels     = default_node_pool.value.node_labels
      vnet_subnet_id  = azurerm_subnet.aks_subnet.id
    }
  }

  identity {
    type = "SystemAssigned"
  }

  dynamic "network_profile" {
    for_each = [var.network_profile]
    content {
      network_plugin      = network_profile.value.network_plugin
      network_plugin_mode = network_profile.value.network_plugin_mode
      load_balancer_profile {
        managed_outbound_ip_count = network_profile.value.load_balancer_profile.managed_outbound_ip_count
      }
      pod_cidr       = network_profile.value.pod_cidr
      service_cidr   = network_profile.value.service_cidr
      dns_service_ip = network_profile.value.dns_service_ip
      outbound_type  = network_profile.value.outbound_type
    }
  }

  tags = var.labels
}
