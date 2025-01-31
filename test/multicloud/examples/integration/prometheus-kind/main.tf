module "kind" {
  source = "../../../modules/kind"
  prefix = var.prefix
}

module "prometheus" {
  depends_on = [module.kind]
  source     = "../../../modules/prometheus"
}
