module "kind" {
  source = "../../modules/kind"
  prefix = var.prefix
}

module "retina" {
  depends_on = [module.kind]
  source     = "../../modules/retina"
}

# output "kubeconfig" {
#   value = module.kind.kubeconfig
# }

# resource "local_file" "kubeconfig" {
#   content  = module.kind.kubeconfig
#   filename = "${path.module}/kubeconfig.yaml"
# }
