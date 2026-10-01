// The silo provider binary. OpenTofu/Terraform starts it and speaks
// protocol 6 over stdio; all logic lives in internal/.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/barolinecrewer/silo-server-tofu/internal/provider"
)

// providerAddress matches the registry entry in README.md and docs.
const providerAddress = "registry.opentofu.org/silo-server/silo"

// Set at build time: -ldflags "-X main.version=..."
var version = "dev"

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	if err := providerserver.Serve(
		context.Background(),
		provider.New(version),
		providerserver.ServeOpts{
			Address: providerAddress,
			Debug:   debug,
		},
	); err != nil {
		log.Fatal(err.Error())
	}
}
