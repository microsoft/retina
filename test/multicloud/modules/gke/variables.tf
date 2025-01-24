variable "project" {
  description = "The Google Cloud project where resources will be deployed."
  type        = string
}

variable "location" {
  description = "The Google Cloud location where GKE will be deployed to."
  type        = string
}

variable "prefix" {
  description = "A prefix to add to all resources."
  type        = string
}

variable "machine_type" {
  description = "The machine type to use for the GKE nodes."
  type        = string
}