package hasura

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceRemoteSchema() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceOrderCreate,
		ReadContext:   resourceOrderRead,
		UpdateContext: resourceOrderUpdate,
		DeleteContext: resourceOrderDelete,
		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"url": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"forward_headers": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"additional_headers": &schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
	}
}

func execute(ctx context.Context, data ProviderData, body []byte) (*http.Response, error) {
	client := &http.Client{}

	req, err := http.NewRequestWithContext(ctx, "POST", data.HasuraQeuryEndpoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hasura-Admin-Secret", data.AdminSecret)

	resp, err := client.Do(req)

	return resp, err
}

func resourceOrderCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	data := meta.(*ProviderData)

	name := d.Get("name").(string)
	url := d.Get("url").(string)
	forwardHeaders := d.Get("forward_headers").(bool)
	// additionalHeaders := d.Get("additional_headers").(map[string]interface{})

	type Definition struct {
		Url            string `json:"url"`
		ForwardHeaders bool   `json:"forward_client_headers"`
		Timeout        int    `json:"timeout"`
	}

	type Args struct {
		Name       string     `json:"name"`
		Definition Definition `json:"definition"`
	}

	type Request struct {
		Type string `json:"type"`
		Args Args   `json:"args"`
	}

	// Warning or errors can be collected in a slice type
	var diags diag.Diagnostics

	body := Request{
		Type: "add_remote_schema",
		Args: Args{
			Name: name,
			Definition: Definition{
				Url:            url,
				ForwardHeaders: forwardHeaders,
				Timeout:        30,
			},
		},
	}

	postBody, _ := json.Marshal(body)

	resp, err := execute(ctx, *data, postBody)

	if err != nil {
		return append(diags, diag.Errorf("Error making request: %s", err)...)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bytes, _ := ioutil.ReadAll(resp.Body)
		return append(diags, diag.Errorf("HTTP request error. Response code: %d; %s", resp.StatusCode, string(bytes))...)
	}

	// bytes, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	return append(diags, diag.FromErr(err)...)
	// }

	// _ = bytes

	// d.SetId(name)

	return resourceOrderRead(ctx, d, meta)
}

func resourceOrderRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	data := meta.(*ProviderData)

	name := d.Get("name").(string)

	query := fmt.Sprintf(`{"type":"select","args":{"table":{"name":"remote_schemas","schema":"hdb_catalog"},"columns":["name","definition"],"where":{"name":{"_eq":"%s"}},"limit":1}}`, name)
	postBody := []byte(query)

	type Definition struct {
		Url            string `json:"url"`
		ForwardHeaders bool   `json:"forward_client_headers"`
		Timeout        int    `json:"timeout_seconds"`
	}

	type RemoteSchema struct {
		Name       string     `json:"name"`
		Definition Definition `json:"definition"`
	}

	// Warning or errors can be collected in a slice type
	var diags diag.Diagnostics

	resp, err := execute(ctx, *data, postBody)

	if err != nil {
		return append(diags, diag.Errorf("Error making request: %s", err)...)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return append(diags, diag.Errorf("HTTP request error. Response code: %d", resp.StatusCode)...)
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	var schemas []RemoteSchema

	decodeErr := json.Unmarshal(bytes, &schemas)
	if decodeErr != nil {
		return append(diags, diag.FromErr(decodeErr)...)
	}

	if len(schemas) > 0 {
		rs := schemas[0]
		d.SetId(rs.Name)
		d.Set("name", rs.Name)
		d.Set("url", rs.Definition.Url)
		d.Set("forward_headers", rs.Definition.ForwardHeaders)
	}

	return diags
}

func resourceOrderUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	return diags
}

func resourceOrderDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	return diags
}

// func resourceOrderDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
// 	c := m.(*hc.Client)

// 	// Warning or errors can be collected in a slice type
// 	var diags diag.Diagnostics

// 	orderID := d.Id()

// 	err := c.DeleteOrder(orderID)
// 	if err != nil {
// 		return diag.FromErr(err)
// 	}

// 	// d.SetId("") is automatically called assuming delete returns no errors, but
// 	// it is added here for explicitness.
// 	d.SetId("")

// 	return diags
// }
