variable "project" {
  description = "The Google Cloud project where resources will be deployed."
  type        = string
  default     = "mc-retina"
}

variable "location" {
  description = "The Google Cloud location where GKE will be deployed to."
  type        = string
  default     = "eu-central1"
}

variable "prefix" {
  description = "A prefix to add to all resources."
  type        = string
  default     = "mc"
}

variable "machine_type" {
  description = "The machine type to use for the GKE nodes."
  type        = string
  default     = "e2-standard-4"
}
