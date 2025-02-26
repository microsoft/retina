output "aws_get_kubeconfig" {
  value       = module.eks.aws_get_kubeconfig
  description = "Run this command to fetch the kubeconfig for your EKS cluster"
}

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