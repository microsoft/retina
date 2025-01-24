module "kind" {
  source = "../../modules/kind"
  prefix = var.prefix
}

module "retina" {
  depends_on = [module.kind]
  source     = "../../modules/retina"
}
