variable "name" {
  description = "Name for the LoadBalancer service"
  type        = string
  default     = "prometheus"
}

variable "namespace" {
  description = "Namespace for the LoadBalancer service"
  type        = string
  default     = "kube-system"
}

variable "label_selector" {
  description = "Label selector for the backend pods"
  type        = map(string)
  default = {
    "app.kubernetes.io/name" = "prometheus"
  }
}

variable "port" {
  description = "Port for the LoadBalancer service and targetPort for the pod"
  type        = number
  default     = 9090
}