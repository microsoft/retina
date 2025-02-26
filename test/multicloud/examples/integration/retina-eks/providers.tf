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
  }
}

# Configure the AWS Provider
provider "aws" {
  region = var.region
}

data "aws_eks_cluster_auth" "eks" {
  name = "${var.prefix}-eks"
  depends_on = [ module.eks ]
}

data "aws_eks_cluster" "eks" {
  name = "${var.prefix}-eks"
  depends_on = [ module.eks ]
}

# Initialize the Helm provider
provider "helm" {
  kubernetes {
    token                  = data.aws_eks_cluster_auth.eks.token
    host                   = data.aws_eks_cluster.eks.endpoint
    cluster_ca_certificate = base64decode(data.aws_eks_cluster.eks.certificate_authority[0].data)
  }
}