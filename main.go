package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/plugin"
	"terraform-provider-citrixblx/citrixblx"
)

func main() {
	opts := plugin.ServeOpts{
		ProviderFunc: citrixblx.Provider,
	}
	plugin.Serve(&opts)
}
