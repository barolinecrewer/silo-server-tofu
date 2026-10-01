# Access groups are the policy bundles that limit which libraries member
# accounts can see and how they may stream, transcode, download, and
# submit requests. Fields left unset take the server's defaults.

terraform {
  required_providers {
    silo = {
      source = "silo-server/silo"
    }
  }
}

resource "silo_access_group" "family" {
  name             = "Family"
  description      = "Household members, all libraries"
  download_allowed = true
  requests_allowed = true
  max_streams      = 4
}

resource "silo_access_group" "guests" {
  name                           = "Guests"
  description                    = "Limited streaming, no downloads, no requests"
  transcode_allowed              = true
  max_streams                    = 1
  max_remote_stream_bitrate_kbps = 4096
}
