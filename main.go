package main

import (
	"context"

	"github.com/AlexeyRaga/terraform-provider-hasura/hasura/hasura"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

func main() {
	tfsdk.Serve(context.Background(), hasura.New, tfsdk.ServeOpts{
		Name: "hasura",
	})
}
