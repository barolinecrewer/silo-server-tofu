terraform {
  required_providers {
    silo = {
      source = "silo-server/silo"
    }
  }
}

provider "silo" {}

# Credential values are never returned by the list endpoint; each key exposes
# only its non-secret prefix.
data "silo_api_keys" "all" {}

output "key_labels" {
  value = [for k in data.silo_api_keys.all.api_keys : k.label]
}
