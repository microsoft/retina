output "access_token" {
  value     = module.eks.access_token
  sensitive = true
}

output "host" {
  value     = module.eks.host
  sensitive = true
}

output "cluster_ca_certificate" {
  value     = module.eks.cluster_ca_certificate
  sensitive = true
}