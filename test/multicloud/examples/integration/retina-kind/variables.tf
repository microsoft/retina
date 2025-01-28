variable "prefix" {
  description = "A prefix to add to all resources."
  type        = string
  default     = "mc"
}

variable "retina_version" {
  description = "The tag to apply to all resources."
  type        = string
  default     = "v0.0.23"
}