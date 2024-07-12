package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func newSourceDataSource() *schema.Resource {
	s := make(map[string]*schema.Schema)
	for k, v := range sourceSchema {
		cp := *v
		switch k {
		case "table_name":
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
		ReadContext: sourceLookup,
		Description: "This Data Source allows you to look up existing Logtail Sources using their table name. The table name is shown on the Source settings page on Logtail.com or you can list all your existing sources via the [Sources API](https://docs.logtail.com/api/sources-api#get-sources).",
		Schema:      s,
	}
}

type sourcePageHTTPResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes source `json:"attributes"`
	} `json:"data"`
	Pagination struct {
		First string `json:"first"`
		Last  string `json:"last"`
		Prev  string `json:"prev"`
		Next  string `json:"next"`
	} `json:"pagination"`
}

func sourceLookup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	fetch := func(page int) (*sourcePageHTTPResponse, error) {
		res, err := meta.(*client).Get(ctx, fmt.Sprintf("/api/v1/sources?page=%d", page))
		if err != nil {
			return nil, err
		}
		defer func() {
			// Keep-Alive.
			_, _ = io.Copy(ioutil.Discard, res.Body)
			_ = res.Body.Close()
		}()
		body, err := ioutil.ReadAll(res.Body)
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GET %s returned %d: %s", res.Request.URL.String(), res.StatusCode, string(body))
		}
		if err != nil {
			return nil, err
		}
		var tr sourcePageHTTPResponse
		return &tr, json.Unmarshal(body, &tr)
	}
	table_name := d.Get("table_name").(string)
	page := 1
	for {
		res, err := fetch(page)
		if err != nil {
			return diag.FromErr(err)
		}
		for _, e := range res.Data {
			if *e.Attributes.TableName == table_name {
				if d.Id() != "" {
					return diag.Errorf("duplicate")
				}
				d.SetId(e.ID)
				if derr := sourceCopyAttrs(d, &e.Attributes); derr != nil {
					return derr
				}
			}
		}
		page++
		if res.Pagination.Next == "" {
			return nil
		}
	}
}
