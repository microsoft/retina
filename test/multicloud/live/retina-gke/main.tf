module "gke" {
  source       = "../../modules/gke"
  location     = var.location
  prefix       = var.prefix
  project      = var.project
  machine_type = var.machine_type
}

module "retina" {
  depends_on = [module.gke]
  source     = "../../modules/retina"
}
