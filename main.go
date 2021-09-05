package main

import (
	"github.com/AlexeyRaga/terraform-provider-hasura/hasura/hasura"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: hasura.Provider})
}
