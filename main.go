package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/provider"
)

//go:generate go run ./internal/gen
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name uapi

var version = "dev" // overridden at build time via -ldflags

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/openwrt-iac/uapi",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
