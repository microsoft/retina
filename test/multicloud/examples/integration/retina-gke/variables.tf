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

variable "retina_release_name" {
  description = "The name of the Helm release."
  type        = string
  default     = "retina"
}

variable "retina_repository_url" {
  description = "The URL of the Helm repository."
  type        = string
  default     = "oci://ghcr.io/microsoft/retina/charts"
}

variable "retina_chart_version" {
  description = "The version of the Helm chart to install."
  type        = string
  default     = "v0.0.24"
}

variable "retina_chart_name" {
  description = "The name of the Helm chart to install."
  type        = string
  default     = "retina"
}

variable "retina_values" {
  description = "Configuration for set blocks, this corresponds to Helm values.yaml"
  type = list(object({
    name  = string
    value = string
  }))
  default = [
    {
      name  = "image.tag"
      value = "v0.0.24"
    },
    {
      name  = "operator.tag"
      value = "v0.0.24"
    },
    {
      name  = "logLevel"
      value = "info"
    }
  ]
}
