//go:build tools

// Pins build-time tooling versions. Managed by `go mod tidy` under the
// tools build tag; do not import from non-tools files.
package tools

import (
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)
