# All access groups on the server. Useful to reference an existing group
# by name instead of hardcoding its opaque ID.

terraform {
  required_providers {
    silo = {
      source = "silo-server/silo"
    }
  }
}

data "silo_access_groups" "all" {}

locals {
  family_group_id = one(
    [for g in data.silo_access_groups.all.access_groups : g.id if g.name == "Family"]
  )
}

output "family_group_id" {
  value = local.family_group_id
}
