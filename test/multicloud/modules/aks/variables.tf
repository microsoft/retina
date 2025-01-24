variable "location" {
  description = "The VM location."
  type        = string
  default     = "UK South"
}

variable "resource_group_name" {
  description = "The name of the resource group."
  type        = string
}

variable "prefix" {
  description = "A prefix to add to all resources."
  type        = string
  default     = "example-vm"
}

variable "labels" {
  description = "A map of labels to add to all resources."
  type        = map(string)
  default     = {}
}
