terraform {
  required_version = "1.8.3"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "6.17.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "2.17.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "2.35.1"
    }
    grafana = {
      source  = "grafana/grafana"
      version = "3.18.3"
    }
  }
}

# Initialize the Google provider
provider "google" {
  project = var.project
  region  = local.location
}


# Initialize the Helm provider
provider "helm" {
  kubernetes {
    token                  = data.google_client_config.current.access_token
    host                   = module.gke.host
    cluster_ca_certificate = base64decode(module.gke.cluster_ca_certificate)
  }
}

data "google_client_config" "current" {}

# Initialize the Kubernetes provider for GKE
provider "kubernetes" {
  token                  = data.google_client_config.current.access_token
  host                   = module.gke.host
  cluster_ca_certificate = base64decode(module.gke.cluster_ca_certificate)
}

# Initialize the Grafana provider
provider "grafana" {
  url = var.grafana_url
}
