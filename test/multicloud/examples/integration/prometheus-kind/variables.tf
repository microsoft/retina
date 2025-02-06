variable "prefix" {
  description = "A prefix to add to all resources."
  type        = string
  default     = "mc"
}

variable "prometheus_release_name" {
  description = "The name of the Helm release."
  type        = string
  default     = "prometheus"
}

variable "prometheus_repository_url" {
  description = "The URL of the Helm repository."
  type        = string
  default     = "https://prometheus-community.github.io/helm-charts"
}

variable "prometheus_chart_version" {
  description = "The version of the Helm chart to install."
  type        = string
  default     = "68.4.3"
}

variable "prometheus_chart_name" {
  description = "The name of the Helm chart to install."
  type        = string
  default     = "kube-prometheus-stack"
}

variable "prometheus_values" {
  description = "This corresponds to Helm values.yaml"
  type        = list(string)
}
