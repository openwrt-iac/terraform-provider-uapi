package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/raspbeguy/terraform-provider-uapi/internal/provider"
)

var version = "dev" // overridden at build time via -ldflags

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/raspbeguy/uapi",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
