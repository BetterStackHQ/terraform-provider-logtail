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

func newExplorationDataSource() *schema.Resource {
	s := make(map[string]*schema.Schema)

	for k, v := range explorationSchema {
		cp := *v
		switch k {
		case "name":
			// Name is used for lookup - make it required
			cp.Computed = false
			cp.Optional = false
			cp.Required = true
		default:
			// All other fields become computed (read-only)
			cp.Computed = true
			cp.Optional = false
			cp.Required = false
			cp.ValidateFunc = nil
			cp.ValidateDiagFunc = nil
			cp.Default = nil
			cp.DefaultFunc = nil
			cp.DiffSuppressFunc = nil
			cp.MinItems = 0
			cp.MaxItems = 0
		}
		s[k] = &cp
	}

	return &schema.Resource{
		ReadContext: explorationLookup,
		Description: "This data source allows you to get information about an Exploration in Better Stack Telemetry.",
		Schema:      s,
	}
}

// Summary response for list endpoint (no chart/queries/variables)
type explorationsHTTPResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Name               *string `json:"name,omitempty"`
			DateRangeFrom      *string `json:"date_range_from,omitempty"`
			DateRangeTo        *string `json:"date_range_to,omitempty"`
			ExplorationGroupID *int    `json:"exploration_group_id,omitempty"`
			CreatedAt          *string `json:"created_at,omitempty"`
			UpdatedAt          *string `json:"updated_at,omitempty"`
		} `json:"attributes"`
	} `json:"data"`
	Pagination struct {
		Next *string `json:"next"`
	} `json:"pagination"`
}

func explorationLookup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	name := d.Get("name").(string)
	c := meta.(*client)

	// Fetch function for listing explorations
	fetchList := func(u string) (*explorationsHTTPResponse, error) {
		res, err := c.Get(ctx, u)
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
		var out explorationsHTTPResponse
		return &out, json.Unmarshal(body, &out)
	}

	// First, find the exploration by name using the list endpoint
	var foundID string
	page := "/api/v2/explorations?page=1"
	for {
		out, err := fetchList(page)
		if err != nil {
			return diag.FromErr(err)
		}

		for _, item := range out.Data {
			if item.Attributes.Name != nil && *item.Attributes.Name == name {
				foundID = item.ID
				break
			}
		}

		if foundID != "" {
			break
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

	if foundID == "" {
		return diag.Errorf("Exploration with name %q not found", name)
	}

	// Now fetch the full exploration details
	var out explorationHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, c.TelemetryBaseURL(), fmt.Sprintf("/api/v2/explorations/%s", url.PathEscape(foundID)), &out); err != nil {
		return err
	} else if !ok {
		return diag.Errorf("Exploration with ID %s not found", foundID)
	}

	d.SetId(foundID)
	return explorationCopyAttrs(d, &out.Data.Attributes)
}
