package main

import (
	"github.com/hashicorp/terraform/plugin"

	"github.com/paperg/terraform-fleet/fleet"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: fleet.Provider,
	})
}
