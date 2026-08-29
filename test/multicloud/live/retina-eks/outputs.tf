output "kubeconfig_command" {
  value = module.eks.aws_get_kubeconfig
}

output "cluster_name" {
  value = module.eks.cluster_name
}