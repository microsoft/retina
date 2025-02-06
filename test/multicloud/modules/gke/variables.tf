variable "prefix" {
  description = "Prefix for resource names"
  type        = string
}

variable "project" {
  description = "Project ID"
  type        = string
}

variable "location" {
  description = "Region for the GKE cluster and subnet"
  type        = string
}

variable "machine_type" {
  description = "Machine type for the GKE node pool"
  type        = string
}

variable "subnet_cidr" {
  description = "CIDR range for the subnet"
  type        = string
  default     = "10.0.0.0/24"
}
