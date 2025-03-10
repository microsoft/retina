variable "release_name" {
  description = "The name of the Helm release."
  type        = string
}

variable "release_namespace" {
  description = "The namespace to install the Helm chart."
  type        = string
  default     = "default"
}

variable "repository_url" {
  description = "The URL of the Helm repository."
  type        = string
}

variable "chart_version" {
  description = "The version of the Helm chart to install."
  type        = string
}

variable "chart_name" {
  description = "The name of the Helm chart to install."
  type        = string
}

variable "values" {
  description = "This corresponds to Helm values.yaml"
  type        = any
}
