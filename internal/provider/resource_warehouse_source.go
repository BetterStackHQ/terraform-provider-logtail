package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var warehouseSourceSchema = map[string]*schema.Schema{
	"team_name": {
		Description: "Used to specify the team the resource should be created in when using global tokens.",
		Type:        schema.TypeString,
		Optional:    true,
		Default:     nil,
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			return d.Id() != ""
		},
	},
	"id": {
		Description: "The ID of this warehouse source.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"name": {
		Description: "The name of the new Warehouse source. Can contain letters, numbers, spaces, and special characters. Source names do not need to be unique.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"token": {
		Description: "The token of this warehouse source. This token is used to identify and route the data you will send to Better Stack.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"table_name": {
		Description: "The table name generated for this warehouse source.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"ingesting_host": {
		Description: "The host where the data should be sent. See documentation for details.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"ingesting_paused": {
		Description: "This property allows you to temporarily pause data ingesting for this source.",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"events_retention": {
		Description:  "The retention period for event data in days. Default is 9999999 days (effectively infinite).",
		Type:         schema.TypeInt,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.IntBetween(1, 9999999),
	},
	"time_series_retention": {
		Description:  "The retention period for time series data in days. Default is 9999999 days (effectively infinite).",
		Type:         schema.TypeInt,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.IntBetween(1, 9999999),
	},
	"live_tail_pattern": {
		Description: "A display template for live tail messages. Default is `\"{status} {message}\"`.",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"created_at": {
		Description: "The time when this warehouse source was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this warehouse source was updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"data_region": {
		Description: strings.ReplaceAll(`The data region or cluster name where the source's data will be stored.
Possible values include `+"`us_east`, `us_west`, `germany`, `singapore`, or a specific cluster name like `us-east-9`"+`.
The actual region created may differ slightly due to dynamic load balancing.`, "**", "`"),
		Type:     schema.TypeString,
		Optional: true,
		Computed: true,
	},
	"warehouse_source_group_id": {
		Description: "The ID of the warehouse source group this source belongs to.",
		Type:        schema.TypeInt,
		Required:    true,
		ForceNew:    true,
	},
	"custom_bucket": {
		Description: "Optional custom bucket configuration for the source. When provided, all fields (name, endpoint, access_key_id, secret_access_key) are required.",
		Type:        schema.TypeList,
		Optional:    true,
		MaxItems:    1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Description: "Bucket name",
					Type:        schema.TypeString,
					Required:    true,
				},
				"endpoint": {
					Description: "Bucket endpoint",
					Type:        schema.TypeString,
					Required:    true,
				},
				"access_key_id": {
					Description: "Access key ID",
					Type:        schema.TypeString,
					Required:    true,
				},
				"secret_access_key": {
					Description: "Secret access key",
					Type:        schema.TypeString,
					Required:    true,
					Sensitive:   true,
				},
				"keep_data_after_retention": {
					Description: "Whether we should keep data in the bucket after the retention period.",
					Type:        schema.TypeBool,
					Optional:    true,
					Default:     false,
				},
			},
		},
	},
	"vrl_transformation": {
		Description: "A VRL program for real-time data transformation. Read more about [VRL transformations](https://betterstack.com/docs/logs/using-logtail/transforming-ingested-data/logs-vrl/).",
		Type:        schema.TypeString,
		Optional:    true,
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			normalizeVRL := func(vrl string) string {
				if vrl == "" {
					return ""
				}
				lines := strings.Split(vrl, "\n")
				var normalized []string
				for _, line := range lines {
					normalizedLine := strings.TrimSpace(line)
					normalizedLine = strings.TrimSuffix(normalizedLine, ".")
					normalizedLine = strings.TrimSpace(normalizedLine)
					if normalizedLine != "" {
						normalized = append(normalized, normalizedLine)
					}
				}
				return strings.Join(normalized, "\n")
			}

			return normalizeVRL(old) == normalizeVRL(new)
		},
	},
}

func newWarehouseSourceResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: warehouseSourceCreate,
		ReadContext:   warehouseSourceRead,
		UpdateContext: warehouseSourceUpdate,
		DeleteContext: warehouseSourceDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		CustomizeDiff: validateWarehouseSource,
		Description:   "Create data sources in Better Stack Warehouse for ingesting time-series events. Define schemas for operational data, user behavior metrics, or business analytics to enable powerful querying and visualization. [Learn more](https://betterstack.com/docs/warehouse/start/).",
		Schema:        warehouseSourceSchema,
	}
}

type warehouseSource struct {
	Name                   *string             `json:"name,omitempty"`
	Token                  *string             `json:"token,omitempty"`
	TableName              *string             `json:"table_name,omitempty"`
	IngestingHost          *string             `json:"ingesting_host,omitempty"`
	IngestingPaused        *bool               `json:"ingesting_paused,omitempty"`
	EventsRetention        *int                `json:"events_retention,omitempty"`
	TimeSeriesRetention    *int                `json:"time_series_retention,omitempty"`
	LiveTailPattern        *string             `json:"live_tail_pattern,omitempty"`
	CreatedAt              *string             `json:"created_at,omitempty"`
	UpdatedAt              *string             `json:"updated_at,omitempty"`
	TeamName               *string             `json:"team_name,omitempty"`
	DataRegion             *string             `json:"data_region,omitempty"`
	WarehouseSourceGroupID *int                `json:"warehouse_source_group_id,omitempty"`
	CustomBucket           *sourceCustomBucket `json:"custom_bucket,omitempty"`
	VrlTransformation      *string             `json:"vrl_transformation,omitempty"`
}

type warehouseSourceHTTPResponse struct {
	Data struct {
		ID         string          `json:"id"`
		Attributes warehouseSource `json:"attributes"`
	} `json:"data"`
}

func warehouseSourceRef(in *warehouseSource) []struct {
	k string
	v interface{}
} {
	return []struct {
		k string
		v interface{}
	}{
		{k: "name", v: &in.Name},
		{k: "token", v: &in.Token},
		{k: "table_name", v: &in.TableName},
		{k: "ingesting_host", v: &in.IngestingHost},
		{k: "ingesting_paused", v: &in.IngestingPaused},
		{k: "events_retention", v: &in.EventsRetention},
		{k: "time_series_retention", v: &in.TimeSeriesRetention},
		{k: "live_tail_pattern", v: &in.LiveTailPattern},
		{k: "created_at", v: &in.CreatedAt},
		{k: "updated_at", v: &in.UpdatedAt},
		{k: "data_region", v: &in.DataRegion},
		{k: "warehouse_source_group_id", v: &in.WarehouseSourceGroupID},
		{k: "vrl_transformation", v: &in.VrlTransformation},
	}
}

func warehouseSourceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in warehouseSource
	for _, e := range warehouseSourceRef(&in) {
		load(d, e.k, e.v)
	}

	load(d, "team_name", &in.TeamName)

	if customBucketData, ok := d.GetOk("custom_bucket"); ok {
		customBucketList := customBucketData.([]interface{})
		if len(customBucketList) > 0 {
			customBucketMap := customBucketList[0].(map[string]interface{})
			in.CustomBucket = &sourceCustomBucket{
				Name:                   stringPtr(customBucketMap["name"].(string)),
				Endpoint:               stringPtr(customBucketMap["endpoint"].(string)),
				AccessKeyID:            stringPtr(customBucketMap["access_key_id"].(string)),
				SecretAccessKey:        stringPtr(customBucketMap["secret_access_key"].(string)),
				KeepDataAfterRetention: boolPtr(customBucketMap["keep_data_after_retention"].(bool)),
			}
		}
	}

	var out warehouseSourceHTTPResponse
	if err := resourceCreateWithBaseURL(ctx, meta, meta.(*client).WarehouseBaseURL(), "/api/v1/sources", &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	return warehouseSourceCopyAttrs(d, &out.Data.Attributes)
}

func warehouseSourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out warehouseSourceHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/sources/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Force "create" on 404.
		return nil
	}
	return warehouseSourceCopyAttrs(d, &out.Data.Attributes)
}

func warehouseSourceCopyAttrs(d *schema.ResourceData, in *warehouseSource) diag.Diagnostics {
	var derr diag.Diagnostics
	for _, e := range warehouseSourceRef(in) {
		if e.k == "data_region" && d.Get("data_region").(string) != "" {
			// Don't update data region from API if it's already set - data_region can't change
			continue
		} else if err := d.Set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface()); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	if in.CustomBucket != nil {
		customBucketData := make(map[string]interface{})
		if in.CustomBucket.Name != nil {
			customBucketData["name"] = *in.CustomBucket.Name
		}
		if in.CustomBucket.Endpoint != nil {
			customBucketData["endpoint"] = *in.CustomBucket.Endpoint
		}
		if in.CustomBucket.AccessKeyID != nil {
			customBucketData["access_key_id"] = *in.CustomBucket.AccessKeyID
		}
		// Note: secret_access_key is never returned from API, so we preserve the existing value
		if existingCustomBucket, ok := d.GetOk("custom_bucket"); ok {
			existingCustomBucketList := existingCustomBucket.([]interface{})
			if len(existingCustomBucketList) > 0 {
				existingCustomBucketMap := existingCustomBucketList[0].(map[string]interface{})
				if secretKey, ok := existingCustomBucketMap["secret_access_key"]; ok {
					customBucketData["secret_access_key"] = secretKey
				}
			}
		}
		if in.CustomBucket.KeepDataAfterRetention != nil {
			customBucketData["keep_data_after_retention"] = *in.CustomBucket.KeepDataAfterRetention
		}
		if err := d.Set("custom_bucket", []interface{}{customBucketData}); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	return derr
}

func warehouseSourceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in warehouseSource
	for _, e := range warehouseSourceRef(&in) {
		if d.HasChange(e.k) {
			load(d, e.k, e.v)
		}
	}
	return resourceUpdateWithBaseURL(ctx, meta, meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/sources/%s", url.PathEscape(d.Id())), &in)
}

func warehouseSourceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDeleteWithBaseURL(ctx, meta, meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/sources/%s", url.PathEscape(d.Id())))
}

func validateWarehouseSource(ctx context.Context, diff *schema.ResourceDiff, v interface{}) error {
	if err := validateCustomBucketRemoval(ctx, diff, v); err != nil {
		return err
	}

	return nil
}

func newWarehouseSourceDataSource() *schema.Resource {
	s := make(map[string]*schema.Schema)
	for k, v := range warehouseSourceSchema {
		cp := *v
		switch k {
		case "name":
			cp.Computed = false
			cp.Optional = false
			cp.Required = true
		case "custom_bucket":
			cp.Computed = true
			cp.Optional = false
			cp.Required = false
			cp.ValidateFunc = nil
			cp.ValidateDiagFunc = nil
			cp.Default = nil
			cp.DefaultFunc = nil
			cp.DiffSuppressFunc = nil
			cp.MaxItems = 0
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
		ReadContext: warehouseSourceLookup,
		Description: "This Data Source allows you to look up existing Warehouse sources using their name. For more information about the Warehouse API check https://betterstack.com/docs/warehouse/api/sources/index/",
		Schema:      s,
	}
}

type warehouseSourcePageHTTPResponse struct {
	Data []struct {
		ID         string          `json:"id"`
		Attributes warehouseSource `json:"attributes"`
	} `json:"data"`
	Pagination struct {
		First string `json:"first"`
		Last  string `json:"last"`
		Prev  string `json:"prev"`
		Next  string `json:"next"`
	} `json:"pagination"`
}

func warehouseSourceLookup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	fetch := func(page int) (*warehouseSourcePageHTTPResponse, error) {
		res, err := meta.(*client).GetWithBaseURL(ctx, meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/sources?page=%d", page))
		if err != nil {
			return nil, err
		}
		defer func() {
			// Keep-Alive.
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
		var tr warehouseSourcePageHTTPResponse
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
			if *e.Attributes.Name == name {
				if d.Id() != "" {
					return diag.Errorf("duplicate")
				}
				d.SetId(e.ID)
				if derr := warehouseSourceCopyAttrs(d, &e.Attributes); derr != nil {
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
