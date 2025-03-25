output "kubeconfig" {
  value     = kind_cluster.kind.kubeconfig
  sensitive = true
}

output "cluster_name" {
  value     = kind_cluster.kind.name
}

output "host" {
  value     = kind_cluster.kind.endpoint
  sensitive = true
}

output "client_certificate" {
  value     = kind_cluster.kind.client_certificate
  sensitive = true
}

output "client_key" {
  value     = kind_cluster.kind.client_key
  sensitive = true
}

output "cluster_ca_certificate" {
  value     = kind_cluster.kind.cluster_ca_certificate
  sensitive = true
}