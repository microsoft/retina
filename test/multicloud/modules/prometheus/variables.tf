variable "prometheus_version" {
  description = "The Prometheus version to install."
  type        = string
  default     = "68.4.3"
}

variable "values" {
  description = "Configuration for set blocks, this corresponds to Helm values.yaml"
  type = list(object({
    name  = string
    value = string
  }))
  default = [
    {
      name  = "global.prometheus.enabled"
      value = "true"
    },
    {
      name  = "global.grafana.enabled"
      value = "true"
    }
  ]
}
