// Command terraform-provider-cookiemunch is the Terraform provider for Cookie Munch: sites, their
// configuration, cookie declarations, webhooks, partner links and connected systems as code.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/Cookie-Munch/terraform-provider-cookiemunch/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// version is set by the release build.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers")
	flag.Parse()
	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/cookie-munch/cookiemunch",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
