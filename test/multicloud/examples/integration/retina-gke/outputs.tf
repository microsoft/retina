output "host" {
  value     = module.gke.host
  sensitive = true
}

output "cluster_ca_certificate" {
  value     = module.gke.cluster_ca_certificate
  sensitive = true
}

output "access_token" {
  value     = data.google_client_config.current.access_token
  sensitive = true
}
