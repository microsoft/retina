terraform {
  required_version = "1.8.3"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.88.0"
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

# Configure the AWS Provider
provider "aws" {
  region = local.region
}

data "aws_eks_cluster_auth" "eks" {
  name       = "${local.prefix}-eks"
  depends_on = [module.eks]
}

data "aws_eks_cluster" "eks" {
  name       = "${local.prefix}-eks"
  depends_on = [module.eks]
}

# Initialize the Helm provider
provider "helm" {
  kubernetes {
    token                  = data.aws_eks_cluster_auth.eks.token
    host                   = data.aws_eks_cluster.eks.endpoint
    cluster_ca_certificate = base64decode(data.aws_eks_cluster.eks.certificate_authority[0].data)
  }
}

# Initialize the Kubernetes provider for GKE
provider "kubernetes" {
  token                  = data.aws_eks_cluster_auth.eks.token
  host                   = data.aws_eks_cluster.eks.endpoint
  cluster_ca_certificate = base64decode(data.aws_eks_cluster.eks.certificate_authority[0].data)
}

# Initialize the Grafana provider
provider "grafana" {
  url = var.grafana_url
}

