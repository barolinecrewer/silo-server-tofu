terraform {
  required_providers {
    silo = {
      source = "silo-server/silo"
    }
  }
}

provider "silo" {}

variable "initial_password" {
  type      = string
  sensitive = true
}

resource "silo_account" "example" {
  username               = "tofu-example"
  email                  = "tofu-example@example.test"
  password               = var.initial_password
  role                   = "user"
  create_default_profile = false
}
