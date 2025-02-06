variable "prefix" {
  description = "Prefix for resource names"
  type        = string
}

variable "project" {
  description = "Project ID"
  type        = string
}

variable "location" {
  description = "Region for the GKE cluster and subnet"
  type        = string
}

variable "machine_type" {
  description = "Machine type for the GKE node pool"
  type        = string
}

variable "subnet_cidr" {
  description = "CIDR range for the subnet"
  type        = string
  default     = "10.0.0.0/24"
}

variable "inbound_protocol" {
  description = "Protocol for inbound firewall rule"
  type        = string
  default     = "tcp"
}

variable "inbound_ports" {
  description = "Ports for inbound firewall rule"
  type        = list(string)
  default     = []
}

variable "inbound_source_ranges" {
  description = "Source IP ranges for inbound firewall rule"
  type        = list(string)
  default     = []
}

variable "outbound_protocol" {
  description = "Protocol for outbound firewall rule"
  type        = string
  default     = "tcp"
}

variable "outbound_ports" {
  description = "Ports for outbound firewall rule"
  type        = list(string)
  default     = []
}

variable "outbound_destination_ranges" {
  description = "Destination IP ranges for outbound firewall rule"
  type        = list(string)
  default     = []
}