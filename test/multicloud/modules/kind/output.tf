output "kubeconfig" {
  value = kind_cluster.kind.kubeconfig
}

output "host" {
  value = kind_cluster.kind.endpoint
}

output "client_certificate" {
  value = kind_cluster.kind.client_certificate
}

output "client_key" {
  value = kind_cluster.kind.client_key
}

output "cluster_ca_certificate" {
  value = kind_cluster.kind.cluster_ca_certificate
}