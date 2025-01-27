module "kind" {
  source = "../../../modules/kind"
  prefix = var.prefix
}

module "retina" {
  depends_on     = [module.kind]
  source         = "../../../modules/retina"
  retina_version = var.retina_version
}

output "host" {
  value     = module.kind.host
  sensitive = true
}

output "cluster_ca_certificate" {
  value     = module.kind.cluster_ca_certificate
  sensitive = true
}

output "client_certificate" {
  value     = module.kind.client_certificate
  sensitive = true
}

output "client_key" {
  value     = module.kind.client_key
  sensitive = true
}
