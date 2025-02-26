module "eks" {
    source = "../../modules/eks"
    prefix = var.prefix
    region = var.region
}