terraform {
  required_version = "1.8.3"
  required_providers {
    kind = {
      source  = "tehcyx/kind"
      version = "0.7.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "2.17.0"
    }
  }
}

# Initialize the kind provider
provider "kind" {}

# Initialize the Helm provider
provider "helm" {
  kubernetes {
    host                   = module.kind.host
    client_certificate     = module.kind.client_certificate
    client_key             = module.kind.client_key
    cluster_ca_certificate = module.kind.cluster_ca_certificate
  }
}