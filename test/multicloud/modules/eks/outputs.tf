output "aws_get_kubeconfig" {
  value       = "aws eks update-kubeconfig --name ${aws_eks_cluster.eks.name} --region ${var.region}"
  description = "Run this command to fetch the kubeconfig for your EKS cluster"
}

output "cluster_name" {
  value       = aws_eks_cluster.eks.name
  description = "EKS cluster name"
}

output "access_token" {
  value     = data.aws_eks_cluster_auth.eks.token
  sensitive = true
}

output "host" {
  value     = data.aws_eks_cluster.eks.endpoint
  sensitive = true
}

output "cluster_ca_certificate" {
  value     = data.aws_eks_cluster.eks.certificate_authority[0].data
  sensitive = true
}
