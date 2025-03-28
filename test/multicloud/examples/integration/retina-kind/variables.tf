variable "prefix" {
  description = "A prefix to add to all resources."
  type        = string
  default     = "mc"
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
  description = "This corresponds to Helm values.yaml"
  type        = any
}
