package hasura

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ProviderData struct {
	HasuraQeuryEndpoint string
	AdminSecret         string
}

// Provider -
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"host": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("HASURA_HOST", nil),
			},
			"admin_secret": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("HASURA_GRAPHQL_ADMIN_SECRET", nil),
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"hasura_remote_schema": resourceRemoteSchema(),
		},
		DataSourcesMap:       map[string]*schema.Resource{},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	host := d.Get("host").(string)
	adminSecret := d.Get("admin_secret").(string)

	// Warning or errors can be collected in a slice type
	var diags diag.Diagnostics

	endpoint := fmt.Sprintf("https://%s/v1/query", host)

	data := ProviderData{
		HasuraQeuryEndpoint: endpoint,
		AdminSecret:         adminSecret,
	}

	return &data, diags
}
