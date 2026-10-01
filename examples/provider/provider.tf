# Minimal provider configuration. base_url and api_key can also come from
# the SILO_BASE_URL / SILO_API_KEY environment variables.

terraform {
  required_providers {
    silo = {
      source = "registry.opentofu.org/silo-server/silo"
    }
  }
}

variable "silo_api_key" {
  type      = string
  sensitive = true
}

provider "silo" {
  base_url = "https://silo.example.org"
  api_key  = var.silo_api_key
}
