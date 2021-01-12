package main

import (
	"github.com/cofyc/terraform-provider-dnspod/dnspod"
	"github.com/hashicorp/terraform-plugin-sdk/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: dnspod.Provider,
	})
}
