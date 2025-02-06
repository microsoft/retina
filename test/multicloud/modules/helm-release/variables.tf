variable "release_name" {
  description = "The name of the Helm release."
  type        = string
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
  description = "Configuration for set blocks, this corresponds to Helm values.yaml"
  type        = list(string)
}
