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

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type RemoteSchemaDef struct {
	Url               string   `json:"url"`
	ForwardHeaders    bool     `json:"forward_client_headers"`
	Timeout           int      `json:"timeout_seconds"`
	AdditionalHeaders []Header `json:"headers"`
}

type RemoteSchemaArgs struct {
	Name       string          `json:"name"`
	Definition RemoteSchemaDef `json:"definition"`
}

type RemoteSchemaDefRequest struct {
	Type string           `json:"type"`
	Args RemoteSchemaArgs `json:"args"`
}

type RemoteSchemaNameArgs struct {
	Name string `json:"name"`
}

type RemoteSchemaNameRequest struct {
	Type string               `json:"type"`
	Args RemoteSchemaNameArgs `json:"args"`
}

func fromTerraform(ctx context.Context, data ProviderData, plan RemoteSchema) (RemoteSchemaArgs, error) {
	var args RemoteSchemaArgs
	var definition RemoteSchemaDef

	definition.Url = plan.Url.Value
	definition.ForwardHeaders = plan.ForwardHeaders.Value
	definition.Timeout = 30

	definition.AdditionalHeaders = make([]Header, len(plan.AdditionalHeaders.Elems))

	headers := make([]Header, 0, len(plan.AdditionalHeaders.Elems))
	for key, elem := range plan.AdditionalHeaders.Elems {
		val, err := elem.ToTerraformValue(ctx)
		if err != nil {
			return args, err
		}
		headers = append(headers, Header{Name: key, Value: val.(string)})
	}

	definition.AdditionalHeaders = headers
	args.Definition = definition
	args.Name = plan.Name.Value

	return args, nil
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

	args, err := fromTerraform(ctx, *r.p.data, plan)
	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error creating Hasura request",
			Detail:   "An unexpected error was encountered while transforming schema data to a request: " + err.Error(),
		})
		return
	}

	body := RemoteSchemaDefRequest{
		Type: "add_remote_schema",
		Args: args,
	}

	res, err := executeExpect200(ctx, *r.p.data, body)
	defer res.Body.Close()

	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error registering remote schema",
			Detail:   "Could not register remote schema, unexpected error: " + err.Error(),
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
	postBody := []byte(query)

	res, err := execute(ctx, *r.p.data, postBody)
	if res != nil {
		defer res.Body.Close()
	}

	type RemoteDefinition struct {
		Url            string `json:"url"`
		ForwardHeaders bool   `json:"forward_client_headers"`
		Timeout        int    `json:"timeout_seconds"`
	}

	type ResponseSchema struct {
		Name       string           `json:"name"`
		Definition RemoteDefinition `json:"definition"`
	}

	type Response struct {
		RemoteSchemas []ResponseSchema `json:"remote_schemas"`
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

	var rs ResponseSchema
	for _, v := range response.RemoteSchemas {
		if v.Name == name {
			rs = v
			break
		}
	}

	if rs.Name != name {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  fmt.Sprintf("Remote schema '%s' does not exist", name),
			Detail:   fmt.Sprintf("Expected to find remote schema %s in Hasura, but it was not found", name),
		})
		return
	}

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

	var state RemoteSchema
	err = req.State.Get(ctx, &state)
	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error reading state",
			Detail:   "An unexpected error was encountered while reading the state: " + err.Error(),
		})
		return
	}

	plan.Name = state.Name

	args, err := fromTerraform(ctx, *r.p.data, plan)
	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error creating Hasura request",
			Detail:   "An unexpected error was encountered while transforming schema data to a request: " + err.Error(),
		})
		return
	}

	args.Name = state.Name.Value

	updateRequest := RemoteSchemaDefRequest{
		Type: "update_remote_schema",
		Args: args,
	}

	updateRes, err := executeExpect200(ctx, *r.p.data, updateRequest)
	if updateRes != nil {
		defer updateRes.Body.Close()
	}

	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error updating remote schema",
			Detail:   "Could not update remote schema, unexpected error: " + err.Error(),
		})
		return
	}

	reloadRequest := RemoteSchemaNameRequest{
		Type: "reload_remote_schema",
		Args: RemoteSchemaNameArgs{
			Name: state.Name.Value,
		},
	}

	reloadRes, err := executeExpect200(ctx, *r.p.data, reloadRequest)
	if reloadRes != nil {
		defer reloadRes.Body.Close()
	}

	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error updating remote schema",
			Detail:   "Could not update remote schema, unexpected error: " + err.Error(),
		})
		return
	}

	err = resp.State.Set(ctx, plan)
	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error setting state",
			Detail:   "Could not set state, unexpected error: " + err.Error(),
		})
		return
	}

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

	request := Request{
		Type: "remove_remote_schema",
		Args: Args{
			Name: name,
		},
	}

	res, err := executeExpect200(ctx, *r.p.data, request)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Error deleting remote schema",
			Detail:   "Could not delete remote schema, unexpected error: " + err.Error(),
		})
		return
	}

	resp.State.RemoveResource(ctx)
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

func executeExpect200(ctx context.Context, data ProviderData, body interface{}) (*http.Response, error) {
	requestBody, _ := json.Marshal(body)
	resp, err := execute(ctx, data, requestBody)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		bytes, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP request error. Response code: %d; %s", resp.StatusCode, string(bytes))
	}

	return resp, nil
}
