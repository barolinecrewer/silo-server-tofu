terraform {
  required_providers {
    silo = {
      source = "silo-server/silo"
    }
  }
}

provider "silo" {}

data "silo_accounts" "all" {}

output "account_names" {
  value = [for account in data.silo_accounts.all.accounts : account.username]
}
