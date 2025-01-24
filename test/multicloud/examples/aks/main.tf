module "aks" {
  source              = "../../modules/aks"
  location            = var.location
  resource_group_name = var.resource_group_name
  prefix              = var.prefix
  labels              = var.labels
}
