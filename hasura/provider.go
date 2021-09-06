package hasura

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

type ProviderConfig struct {
	QueryUri    types.String `tfsdk:"query_uri"`
	AdminSecret types.String `tfsdk:"admin_secret"`
}

type ProviderData struct {
	QueryUri    string
	AdminSecret string
}

type Provider struct {
	configured bool
	data       *ProviderData
}

func New() tfsdk.Provider {
	return &Provider{}
}

func (p *Provider) GetSchema(_ context.Context) (schema.Schema, []*tfprotov6.Diagnostic) {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"query_uri": {
				Type:        types.StringType,
				Required:    true,
				Description: "The URI of the Hasura GraphQL API endpoint.",
			},
			"admin_secret": {
				Type:        types.StringType,
				Required:    true,
				Sensitive:   true,
				Description: "The admin secret for the Hasura GraphQL API endpoint.",
			},
		},
	}, nil
}

func (p *Provider) Configure(ctx context.Context, req tfsdk.ConfigureProviderRequest, resp *tfsdk.ConfigureProviderResponse) {
	var config ProviderConfig
	err := req.Config.Get(ctx, &config)

	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error parsing configuration",
			Detail:   "Error parsing the configuration, this is an error in the provider. Please report the following to the provider developer:\n\n" + err.Error(),
		})
		return
	}

	var queryUri string
	if config.QueryUri.Unknown {
		// Cannot connect to client with an unknown value
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityWarning,
			Summary:  "Unable to configure Hasura provider",
			Detail:   "Cannot use unknown value as hasura host",
		})
		return
	}
	queryUri = config.QueryUri.Value

	var adminSecret string
	if config.AdminSecret.Unknown {
		// Cannot connect to client with an unknown value
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityWarning,
			Summary:  "Unable to configure Hasura provider",
			Detail:   "Cannot use unknown value as admin secret",
		})
		return
	}
	adminSecret = config.AdminSecret.Value

	data := ProviderData{
		QueryUri:    queryUri,
		AdminSecret: adminSecret,
	}

	p.data = &data
	p.configured = true
}

// GetResources - Defines provider resources
func (p *Provider) GetResources(_ context.Context) (map[string]tfsdk.ResourceType, []*tfprotov6.Diagnostic) {
	return map[string]tfsdk.ResourceType{
		"hasura_remote_schema": ResourceRemoteSchemaType{},
	}, nil
}

// GetDataSources - Defines provider data sources
func (p *Provider) GetDataSources(_ context.Context) (map[string]tfsdk.DataSourceType, []*tfprotov6.Diagnostic) {
	return map[string]tfsdk.DataSourceType{}, nil
}
