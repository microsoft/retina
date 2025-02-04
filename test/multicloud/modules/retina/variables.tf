variable "retina_version" {
  description = "The Retina version to install."
  type        = string
  default     = "v0.0.23"
}

variable "values" {
  description = "Configuration for set blocks, this corresponds to Helm values.yaml"
  type = list(object({
    name  = string
    value = string
  }))
  default = [
    {
      name  = "image.tag"
      value = "v0.0.23"
    },
    {
      name  = "operator.tag"
      value = "v0.0.23"
    },
    {
      name  = "logLevel"
      value = "info"
    }
  ]
}
