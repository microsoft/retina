variable "subscription_id" {
  description = "The subscription ID for the Azure account."
  type        = string
}

variable "tenant_id" {
  description = "The tenant ID for the Azure account."
  type        = string
}

variable "location" {
  description = "The Azure Cloud location where AKS will be deployed to."
  type        = string
  default     = "uksouth"
}

variable "resource_group_name" {
  description = "The name of the resource group."
  type        = string
  default     = "mc-rg"
}

variable "prefix" {
  description = "A prefix to add to all resources."
  type        = string
  default     = "mc"
}

variable "labels" {
  description = "A map of labels to add to all resources."
  type        = map(string)
  default = {
    "env" = "test"
  }
}
