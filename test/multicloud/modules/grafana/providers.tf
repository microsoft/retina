# Terraform Grafana provider configuration
terraform {
  required_version = "1.8.3"
  required_providers {
    grafana = {
      source  = "grafana/grafana"
      version = "3.18.3"
    }
  }
}
