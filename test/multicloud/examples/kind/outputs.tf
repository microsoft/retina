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
