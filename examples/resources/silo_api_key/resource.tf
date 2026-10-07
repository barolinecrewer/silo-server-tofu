terraform {
  required_providers {
    silo = {
      source = "silo-server/silo"
    }
  }
}

provider "silo" {}

# The credential is disclosed only at creation and is stored in sensitive
# state. Label, scopes, and the owning account cannot change after creation;
# only rate_tier is updatable in place.
resource "silo_api_key" "ci" {
  label     = "ci"
  rate_tier = "standard"
}

output "ci_key" {
  value     = silo_api_key.ci.key
  sensitive = true
}
