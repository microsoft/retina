variable "prefix" {
  description = "Prefix for resource names"
  type        = string
}

variable "inbound_firewall_rule" {
  description = "Configuration for inbound firewall rule"
  type = object({
    protocol           = string
    ports              = list(string)
    source_ranges      = list(string)
    destination_ranges = list(string)
  })
  default = {
    protocol           = "tcp"
    ports              = []
    source_ranges      = []
    destination_ranges = []
  }
}

variable "outbound_firewall_rule" {
  description = "Configuration for outbound firewall rule"
  type = object({
    protocol           = string
    ports              = list(string)
    source_ranges      = list(string)
    destination_ranges = list(string)
  })
  default = {
    protocol           = "tcp"
    ports              = []
    source_ranges      = []
    destination_ranges = []
  }
}
