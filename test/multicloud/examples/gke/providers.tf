terraform {
  required_version = "1.8.3"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "6.17.0"
    }
  }
}

# Initialize the Google provider
provider "google" {
  project = var.project
  region  = var.location
}
