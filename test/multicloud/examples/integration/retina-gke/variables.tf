variable "project" {
  description = "The Google Cloud project where resources will be deployed."
  type        = string
  default     = "mc-retina"
}

variable "location" {
  description = "The Google Cloud location where GKE will be deployed to."
  type        = string
  default     = "eu-west2"
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

variable "retina_version" {
  description = "The tag to apply to all resources."
  type        = string
}

variable "values" {
  description = "Configuration for set blocks, this corresponds to Helm values.yaml"
  type = list(object({
    name  = string
    value = string
  }))
}
