package hasura

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"

	"github.com/hashicorp/terraform-plugin-framework/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

type ResourceRemoteSchemaType struct{}

type ResourceRemoteSchema struct {
	p Provider
}

type RemoteSchema struct {
	Name              types.String `tfsdk:"name"`
	Url               types.String `tfsdk:"url"`
	ForwardHeaders    types.Bool   `tfsdk:"forward_headers"`
	AdditionalHeaders types.Map    `tfsdk:"additional_headers"`
}

func (r ResourceRemoteSchemaType) GetSchema(_ context.Context) (schema.Schema, []*tfprotov6.Diagnostic) {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": {
				Type:     types.StringType,
				Required: true,
			},
			"url": {
				Type:     types.StringType,
				Required: true,
			},
			"forward_headers": {
				Type:     types.BoolType,
				Optional: true,
				Computed: true,
			},
			"additional_headers": {
				Optional: true,
				Type: types.MapType{
					ElemType: types.StringType,
				},
			},
		}}, nil
}

func (r ResourceRemoteSchemaType) NewResource(_ context.Context, p tfsdk.Provider) (tfsdk.Resource, []*tfprotov6.Diagnostic) {
	return ResourceRemoteSchema{
		p: *(p.(*Provider)),
	}, nil
}

func (r ResourceRemoteSchema) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	if !r.p.configured {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Provider not configured",
			Detail:   "The provider hasn't been configured before apply, likely because it depends on an unknown value from another resource. This leads to weird stuff happening, so we'd prefer if you didn't do that. Thanks!",
		})
		return
	}

	var plan RemoteSchema
	err := req.Plan.Get(ctx, &plan)
	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error reading plan",
			Detail:   "An unexpected error was encountered while reading the plan: " + err.Error(),
		})
		return
	}

	type Header struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}

	type Definition struct {
		Url               string   `json:"url"`
		ForwardHeaders    bool     `json:"forward_client_headers"`
		Timeout           int      `json:"timeout_seconds"`
		AdditionalHeaders []Header `json:"headers"`
	}

	type Args struct {
		Name       string     `json:"name"`
		Definition Definition `json:"definition"`
	}

	type Request struct {
		Type string `json:"type"`
		Args Args   `json:"args"`
	}

	headers := make([]Header, 0, len(plan.AdditionalHeaders.Elems))
	for key, elem := range plan.AdditionalHeaders.Elems {
		val, err := elem.ToTerraformValue(ctx)
		if err != nil {
			resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "Error processing additional headers",
				Detail:   "Error getting Terraform value for element: " + err.Error(),
			})
			return
		}
		headers = append(headers, Header{Name: key, Value: val.(string)})
	}

	body := Request{
		Type: "add_remote_schema",
		Args: Args{
			Name: plan.Name.Value,
			Definition: Definition{
				Url:               plan.Url.Value,
				ForwardHeaders:    plan.ForwardHeaders.Value,
				Timeout:           30,
				AdditionalHeaders: headers,
			},
		},
	}

	postBody, _ := json.Marshal(body)

	res, err := execute(ctx, *r.p.data, postBody)
	defer res.Body.Close()

	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error registering remote schema",
			Detail:   "Could not register remote schema, unexpected error: " + err.Error(),
		})
		return
	}

	if res.StatusCode != 200 {
		bytes, _ := ioutil.ReadAll(res.Body)
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error registering remote schema",
			Detail:   fmt.Sprintf("HTTP request error. Response code: %d; %s", res.StatusCode, string(bytes)),
		})
		return
	}

	var result = RemoteSchema{
		Name:              types.String{Value: plan.Name.Value},
		Url:               types.String{Value: plan.Url.Value},
		ForwardHeaders:    types.Bool{Value: plan.ForwardHeaders.Value},
		AdditionalHeaders: plan.AdditionalHeaders,
	}

	err = resp.State.Set(ctx, result)
	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error setting state",
			Detail:   "Could not set state, unexpected error: " + err.Error(),
		})
		return
	}
}

func (r ResourceRemoteSchema) Read(ctx context.Context, req tfsdk.ReadResourceRequest, resp *tfsdk.ReadResourceResponse) {
	var state RemoteSchema
	err := req.State.Get(ctx, &state)
	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error reading state",
			Detail:   "An unexpected error was encountered while reading the state: " + err.Error(),
		})
		return
	}

	name := state.Name.Value

	query := `{
    "type": "export_metadata",
    "version": 1,
    "args": {}
	}`
	// query := fmt.Sprintf(`{"type":"select","args":{"table":{"name":"remote_schemas","schema":"hdb_catalog"},"columns":["name","definition"],"where":{"name":{"_eq":"%s"}},"limit":1}}`, name)
	postBody := []byte(query)

	res, err := execute(ctx, *r.p.data, postBody)
	defer res.Body.Close()

	type Definition struct {
		Url            string `json:"url"`
		ForwardHeaders bool   `json:"forward_client_headers"`
		Timeout        int    `json:"timeout_seconds"`
	}

	type ResponseItem struct {
		Name       string     `json:"name"`
		Definition Definition `json:"definition"`
	}

	type Response struct {
		RemoteSchemas []ResponseItem `json:"remote_schemas"`
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  fmt.Sprintf("Error reading remote schema '%s'", name),
			Detail:   "An unexpected error was encountered while reading the Hasura HTTP response body: " + err.Error(),
		})
		return
	}

	if res.StatusCode != 200 {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  fmt.Sprintf("Error reading remote schema '%s'", name),
			Detail:   fmt.Sprintf("HTTP request error. Response code: %d; %s", res.StatusCode, string(bytes)),
		})
		return
	}

	var response Response

	decodeErr := json.Unmarshal(bytes, &response)
	if decodeErr != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  fmt.Sprintf("Error reading remote schema '%s'", name),
			Detail:   "Unable to decode Hasura response: " + decodeErr.Error(),
		})
		return
	}

	if len(response.RemoteSchemas) == 0 {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  fmt.Sprintf("Remote schema '%s' does not exist", name),
			Detail:   fmt.Sprintf("Expected to find remote schema %s in Hasura, but it was not found", name),
		})
		return
	}

	rs := response.RemoteSchemas[0]
	state.Url = types.String{Value: rs.Definition.Url}
	state.ForwardHeaders = types.Bool{Value: rs.Definition.ForwardHeaders}
	// state.AdditionalHeaders = rs.Definition.AdditionalHeaders

	err = resp.State.Set(ctx, &state)
	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  fmt.Sprintf("Error setting state for remote schema '%s'", name),
			Detail:   "Unexpected error encountered trying to set new state: " + err.Error(),
		})
		return
	}
}

func (r ResourceRemoteSchema) Update(ctx context.Context, req tfsdk.UpdateResourceRequest, resp *tfsdk.UpdateResourceResponse) {
}

func (r ResourceRemoteSchema) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	var state RemoteSchema
	err := req.State.Get(ctx, &state)
	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error reading state",
			Detail:   "An unexpected error was encountered while reading the state: " + err.Error(),
		})
		return
	}

	name := state.Name.Value

	type Args struct {
		Name string `json:"name"`
	}

	type Request struct {
		Type string `json:"type"`
		Args Args   `json:"args"`
	}

	body := Request{
		Type: "remove_remote_schema",
		Args: Args{
			Name: name,
		},
	}

	postBody, _ := json.Marshal(body)

	res, err := execute(ctx, *r.p.data, postBody)
	defer res.Body.Close()

	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error deleting remote schema",
			Detail:   "Could not delete remote schema, unexpected error: " + err.Error(),
		})
		return
	}

	if res.StatusCode != 200 {
		bytes, _ := ioutil.ReadAll(res.Body)
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error deleting remote schema",
			Detail:   fmt.Sprintf("HTTP request error. Response code: %d; %s", res.StatusCode, string(bytes)),
		})
		return
	}

}

func execute(ctx context.Context, data ProviderData, body []byte) (*http.Response, error) {
	client := &http.Client{}

	req, err := http.NewRequestWithContext(ctx, "POST", data.QueryUri, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hasura-Admin-Secret", data.AdminSecret)

	line, _ := httputil.DumpRequest(req, true)

	log.Printf("[WARN] %s", line)

	resp, err := client.Do(req)

	return resp, err
}
