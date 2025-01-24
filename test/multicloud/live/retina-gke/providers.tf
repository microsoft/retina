terraform {
  required_version = ">= 1.0.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 6.17.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "2.17.0"
    }
  }
}

# Initialize the Google provider
provider "google" {
  project = var.project
  region  = var.location
}

data "google_client_config" "current" {}

# Initialize the Helm provider
provider "helm" {
  kubernetes {
    token                  = data.google_client_config.current.access_token
    host                   = module.gke.host
    cluster_ca_certificate = base64decode(module.gke.cluster_ca_certificate)
  }
}