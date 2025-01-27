module "kind" {
  source = "../../../modules/kind"
  prefix = var.prefix
}

module "retina" {
  depends_on = [module.kind]
  source     = "../../../modules/retina"
  retina_version = var.retina_version
}

output "host" {
  value = module.kind.host
}

output "cluster_ca_certificate" {
  value = module.kind.cluster_ca_certificate
}

output "client_certificate" {
  value = module.kind.client_certificate
}

output "client_key" {
  value = module.kind.client_key
}
