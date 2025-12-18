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

func newSourceGroupDataSource() *schema.Resource {
	s := make(map[string]*schema.Schema)

	for k, v := range sourceGroupSchema {
		cp := *v
		switch k {
		case "name":
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
		}
		s[k] = &cp
	}

	return &schema.Resource{
		ReadContext: sourceGroupLookup,
		Description: "Retrieve details of an existing log source group by name. Useful for accessing source group configurations and settings for programmatic management of log sources. [Learn more](https://betterstack.com/docs/logs/api/listing-sources-in-source-group/).",
		Schema:      s,
	}
}

type sourceGroupsHTTPResponse struct {
	Data []struct {
		ID         string      `json:"id"`
		Attributes sourceGroup `json:"attributes"`
	} `json:"data"`
	Pagination struct {
		Next *string `json:"next"`
	} `json:"pagination"`
}

func sourceGroupLookup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	name := d.Get("name").(string)

	fetch := func(u string) (*sourceGroupsHTTPResponse, error) {
		res, err := meta.(*client).Get(ctx, u)
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
		var out sourceGroupsHTTPResponse
		return &out, json.Unmarshal(body, &out)
	}

	page := "/api/v1/source-groups?page=1"
	for {
		out, err := fetch(page)
		if err != nil {
			return diag.FromErr(err)
		}

		for _, item := range out.Data {
			if item.Attributes.Name != nil && *item.Attributes.Name == name {
				d.SetId(item.ID)
				return sourceGroupCopyAttrs(d, &item.Attributes)
			}
		}

		if out.Pagination.Next == nil {
			break
		}

		if u, err := url.Parse(*out.Pagination.Next); err != nil {
			return diag.FromErr(err)
		} else {
			page = u.RequestURI()
		}
	}

	return diag.Errorf("Source group with name %q not found", name)
}
