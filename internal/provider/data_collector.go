package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func newCollectorDataSource() *schema.Resource {
	s := make(map[string]*schema.Schema)
	for k, v := range collectorSchema {
		cp := *v
		switch k {
		case "name":
			// name is used for lookup
			cp.Computed = false
			cp.Optional = false
			cp.Required = true
		default:
			cp.Computed = true
			cp.Optional = false
			cp.Required = false
			cp.ValidateFunc = nil
			cp.ValidateDiagFunc = nil
			cp.Default = nil
			cp.DefaultFunc = nil
			cp.DiffSuppressFunc = nil
			cp.MaxItems = 0
		}
		s[k] = &cp
	}
	return &schema.Resource{
		ReadContext: collectorLookup,
		Description: "This Data Source allows you to look up existing Collectors by name.",
		Schema:      s,
	}
}

func collectorLookup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	fetch := func(page int) (*collectorPageHTTPResponse, error) {
		name := d.Get("name").(string)
		res, err := meta.(*client).Get(ctx, fmt.Sprintf("/api/v1/collectors?name=%s&page=%d", url.QueryEscape(name), page))
		if err != nil {
			return nil, err
		}
		defer func() {
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
		}()
		body, err := io.ReadAll(res.Body)
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GET %s returned %d: %s", res.Request.URL.String(), res.StatusCode, string(body))
		}
		if err != nil {
			return nil, err
		}
		var tr collectorPageHTTPResponse
		return &tr, json.Unmarshal(body, &tr)
	}

	name := d.Get("name").(string)
	page := 1
	for {
		res, err := fetch(page)
		if err != nil {
			return diag.FromErr(err)
		}
		for _, e := range res.Data {
			if e.Attributes.Name != nil && *e.Attributes.Name == name {
				if d.Id() != "" {
					return diag.Errorf("duplicate collector found with name %q", name)
				}
				d.SetId(e.ID)
				if derr := collectorCopyAttrs(d, &e.Attributes); derr != nil {
					return derr
				}
			}
		}
		page++
		if res.Pagination.Next == "" {
			if d.Id() == "" {
				return diag.Errorf("collector with name %q not found", name)
			}
			return nil
		}
	}
}
