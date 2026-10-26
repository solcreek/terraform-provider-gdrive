// SPDX-License-Identifier: MPL-2.0
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/linyiru/terraform-provider-gdrive/internal/provider"
)

var version = "dev"

func main() {
	debug := flag.Bool("debug", false, "Enable debugger support")
	flag.Parse()
	if err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/linyiru/gdrive", Debug: *debug,
	}); err != nil {
		log.Fatal(err)
	}
}
