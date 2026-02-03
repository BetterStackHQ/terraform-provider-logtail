package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var collectorPlatformTypes = []string{
	"docker",
	"swarm",
	"kubernetes",
	"proxy",
}

var collectorSchema = map[string]*schema.Schema{
	"id": {
		Description: "The ID of this collector.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"team_name": {
		Description: "Used to specify the team the resource should be created in when using global tokens.",
		Type:        schema.TypeString,
		Optional:    true,
		Default:     nil,
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			return d.Id() != ""
		},
	},
	"team_id": {
		Description: "The team ID for this resource.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"name": {
		Description: "The name of this collector.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"platform": {
		Description:  "The platform of this collector. This value can be set only when creating a new collector and cannot be changed later. Valid values are: `docker`, `swarm`, `kubernetes`, `proxy`.",
		Type:         schema.TypeString,
		Required:     true,
		ForceNew:     true,
		ValidateFunc: validation.StringInSlice(collectorPlatformTypes, false),
	},
	"note": {
		Description: "A description or note about this collector.",
		Type:        schema.TypeString,
		Optional:    true,
	},
	"status": {
		Description: "The current status of this collector.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"secret": {
		Description: "The secret token used to authenticate collector hosts.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
		Sensitive:   true,
	},
	"data_region": {
		Description: "Data region or private cluster name to create the collector in.",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"logs_retention": {
		Description:  "Data retention for logs in days. There might be additional charges for longer retention.",
		Type:         schema.TypeInt,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.IntBetween(1, 3652),
	},
	"metrics_retention": {
		Description:  "Data retention for metrics in days. There might be additional charges for longer retention.",
		Type:         schema.TypeInt,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.IntBetween(1, 3652),
	},
	"hosts_count": {
		Description: "The number of hosts connected to this collector.",
		Type:        schema.TypeInt,
		Optional:    false,
		Computed:    true,
	},
	"hosts_up_count": {
		Description: "The number of hosts currently online.",
		Type:        schema.TypeInt,
		Optional:    false,
		Computed:    true,
	},
	"databases_count": {
		Description: "The number of database connections configured for this collector.",
		Type:        schema.TypeInt,
		Optional:    false,
		Computed:    true,
	},
	"pinged_at": {
		Description: "The time when this collector last received data.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"created_at": {
		Description: "The time when this collector was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this collector was last updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"configuration": {
		Description: "Configuration settings for the collector.",
		Type:        schema.TypeList,
		Optional:    true,
		Computed:    true,
		MaxItems:    1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"logs_sample_rate": {
					Description:  "Sample rate for logs (0-100).",
					Type:         schema.TypeInt,
					Optional:     true,
					Computed:     true,
					ValidateFunc: validation.IntBetween(0, 100),
				},
				"traces_sample_rate": {
					Description:  "Sample rate for traces (0-100).",
					Type:         schema.TypeInt,
					Optional:     true,
					Computed:     true,
					ValidateFunc: validation.IntBetween(0, 100),
				},
				"collector_components": {
					Description: "Enable or disable specific collector components.",
					Type:        schema.TypeList,
					Optional:    true,
					Computed:    true,
					MaxItems:    1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"beyla":              {Type: schema.TypeBool, Optional: true, Computed: true},
							"beyla_basic":        {Type: schema.TypeBool, Optional: true, Computed: true},
							"beyla_full":         {Type: schema.TypeBool, Optional: true, Computed: true},
							"cluster_agent":      {Type: schema.TypeBool, Optional: true, Computed: true},
							"host_logs":          {Type: schema.TypeBool, Optional: true, Computed: true},
							"collector_internal": {Type: schema.TypeBool, Optional: true, Computed: true},
						},
					},
				},
				"monitoring_options": {
					Description: "Enable or disable specific monitoring options.",
					Type:        schema.TypeList,
					Optional:    true,
					Computed:    true,
					MaxItems:    1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"docker_json_file":      {Type: schema.TypeBool, Optional: true, Computed: true},
							"collector_kubernetes":  {Type: schema.TypeBool, Optional: true, Computed: true},
							"nginx_metrics":         {Type: schema.TypeBool, Optional: true, Computed: true},
							"apache_metrics":        {Type: schema.TypeBool, Optional: true, Computed: true},
						},
					},
				},
				"transformation": {
					Description: "VRL transformation code to modify logs before ingestion.",
					Type:        schema.TypeString,
					Optional:    true,
					Computed:    true,
				},
				"enable_ssl_certificate": {
					Description: "Enable custom SSL/TLS certificate for the collector.",
					Type:        schema.TypeBool,
					Optional:    true,
					Computed:    true,
				},
				"ssl_certificate_host": {
					Description: "Hostname for the SSL certificate.",
					Type:        schema.TypeString,
					Optional:    true,
					Computed:    true,
				},
			},
		},
	},
	"enable_http_basic_auth": {
		Description: "Enable HTTP Basic Authentication for the collector proxy.",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"http_basic_auth_username": {
		Description: "Username for HTTP Basic Authentication.",
		Type:        schema.TypeString,
		Optional:    true,
	},
	"http_basic_auth_password": {
		Description: "Password for HTTP Basic Authentication. This value is write-only and never returned by the API.",
		Type:        schema.TypeString,
		Optional:    true,
		Sensitive:   true,
	},
	"custom_bucket": {
		Description: "Optional custom bucket configuration for the collector. Once set, it cannot be removed.",
		Type:        schema.TypeList,
		Optional:    true,
		MaxItems:    1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name":                      {Description: "Bucket name.", Type: schema.TypeString, Required: true},
				"endpoint":                  {Description: "Bucket endpoint URL.", Type: schema.TypeString, Required: true},
				"access_key_id":             {Description: "Access key ID for the bucket.", Type: schema.TypeString, Required: true},
				"secret_access_key":         {Description: "Secret access key for the bucket.", Type: schema.TypeString, Required: true, Sensitive: true},
				"keep_data_after_retention": {Description: "Whether to keep data in the bucket after the retention period.", Type: schema.TypeBool, Optional: true, Default: false},
			},
		},
	},
	"databases": {
		Description: "Database connections for the collector.",
		Type:        schema.TypeList,
		Optional:    true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"id":           {Description: "The ID of this database connection (assigned by the API).", Type: schema.TypeInt, Computed: true},
				"service_type": {Description: "The type of database service.", Type: schema.TypeString, Required: true, ValidateFunc: validation.StringInSlice([]string{"postgres", "mysql", "redis", "mongodb", "memcached"}, false)},
				"host":         {Description: "The database host.", Type: schema.TypeString, Required: true},
				"port":         {Description: "The database port.", Type: schema.TypeInt, Required: true},
				"username":     {Description: "The database username.", Type: schema.TypeString, Optional: true},
				"password":     {Description: "The database password.", Type: schema.TypeString, Optional: true, Sensitive: true},
				"ssl_mode":     {Description: "SSL mode for PostgreSQL connections.", Type: schema.TypeString, Optional: true, ValidateFunc: validation.StringInSlice([]string{"disable", "require", "verify-ca", "verify-full"}, false)},
				"tls":          {Description: "TLS mode for MySQL connections.", Type: schema.TypeString, Optional: true, ValidateFunc: validation.StringInSlice([]string{"disabled", "required", "verify_ca", "verify_identity"}, false)},
			},
		},
	},
}

// Go structs for API serialization

type collectorCollectorComponents struct {
	Beyla             *bool `json:"beyla,omitempty"`
	BeylaBasic        *bool `json:"beyla_basic,omitempty"`
	BeylaFull         *bool `json:"beyla_full,omitempty"`
	ClusterAgent      *bool `json:"cluster_agent,omitempty"`
	HostLogs          *bool `json:"host_logs,omitempty"`
	CollectorInternal *bool `json:"collector_internal,omitempty"`
}

type collectorMonitoringOptions struct {
	DockerJSONFile      *bool `json:"docker_json_file,omitempty"`
	CollectorKubernetes *bool `json:"collector_kubernetes,omitempty"`
	NginxMetrics        *bool `json:"nginx_metrics,omitempty"`
	ApacheMetrics       *bool `json:"apache_metrics,omitempty"`
}

type collectorConfiguration struct {
	LogsSampleRate       *int                          `json:"logs_sample_rate,omitempty"`
	TracesSampleRate     *int                          `json:"traces_sample_rate,omitempty"`
	CollectorComponents  *collectorCollectorComponents `json:"collector_components,omitempty"`
	MonitoringOptions    *collectorMonitoringOptions   `json:"monitoring_options,omitempty"`
	Transformation       *string                       `json:"transformation,omitempty"`
	EnableSSLCertificate *bool                         `json:"enable_ssl_certificate,omitempty"`
	SSLCertificateHost   *string                       `json:"ssl_certificate_host,omitempty"`
	EnableHTTPBasicAuth  *bool                         `json:"enable_http_basic_auth,omitempty"`
}

type collectorCustomBucket struct {
	Name                   *string `json:"name,omitempty"`
	Endpoint               *string `json:"endpoint,omitempty"`
	AccessKeyID            *string `json:"access_key_id,omitempty"`
	SecretAccessKey        *string `json:"secret_access_key,omitempty"`
	KeepDataAfterRetention *bool   `json:"keep_data_after_retention,omitempty"`
}

type collectorDatabase struct {
	ID          *int    `json:"id,omitempty"`
	ServiceType *string `json:"service_type,omitempty"`
	Host        *string `json:"host,omitempty"`
	Port        *int    `json:"port,omitempty"`
	Username    *string `json:"username,omitempty"`
	Password    *string `json:"password,omitempty"`
	SSLMode     *string `json:"ssl_mode,omitempty"`
	TLS         *string `json:"tls,omitempty"`
	Destroy     *bool   `json:"_destroy,omitempty"`
}

type collector struct {
	Name                  *string                 `json:"name,omitempty"`
	Platform              *string                 `json:"platform,omitempty"`
	Note                  *string                 `json:"note,omitempty"`
	Status                *string                 `json:"status,omitempty"`
	Secret                *string                 `json:"secret,omitempty"`
	DataRegion            *string                 `json:"data_region,omitempty"`
	TeamID                *StringOrInt            `json:"team_id,omitempty"`
	TeamName              *string                 `json:"team_name,omitempty"`
	LogsRetention         *int                    `json:"logs_retention,omitempty"`
	MetricsRetention      *int                    `json:"metrics_retention,omitempty"`
	HostsCount            *int                    `json:"hosts_count,omitempty"`
	HostsUpCount          *int                    `json:"hosts_up_count,omitempty"`
	DatabasesCount        *int                    `json:"databases_count,omitempty"`
	PingedAt              *string                 `json:"pinged_at,omitempty"`
	CreatedAt             *string                 `json:"created_at,omitempty"`
	UpdatedAt             *string                 `json:"updated_at,omitempty"`
	Configuration         *collectorConfiguration `json:"configuration,omitempty"`
	CustomBucket          *collectorCustomBucket  `json:"custom_bucket,omitempty"`
	Databases             *[]collectorDatabase    `json:"databases,omitempty"`
	EnableHTTPBasicAuth   *bool                   `json:"enable_http_basic_auth,omitempty"`
	HTTPBasicAuthUsername *string                 `json:"http_basic_auth_username,omitempty"`
	HTTPBasicAuthPassword *string                 `json:"http_basic_auth_password,omitempty"`
}

type collectorHTTPResponse struct {
	Data struct {
		ID         string    `json:"id"`
		Attributes collector `json:"attributes"`
	} `json:"data"`
}

type collectorPageHTTPResponse struct {
	Data []struct {
		ID         string    `json:"id"`
		Attributes collector `json:"attributes"`
	} `json:"data"`
	Pagination struct {
		Next string `json:"next"`
	} `json:"pagination"`
}

type collectorDatabasesHTTPResponse struct {
	Data []struct {
		ID         int               `json:"id"`
		Attributes collectorDatabase `json:"attributes"`
	} `json:"data"`
}

func collectorRef(in *collector) []struct {
	k string
	v interface{}
} {
	return []struct {
		k string
		v interface{}
	}{
		{k: "name", v: &in.Name},
		{k: "platform", v: &in.Platform},
		{k: "note", v: &in.Note},
		{k: "status", v: &in.Status},
		{k: "secret", v: &in.Secret},
		{k: "data_region", v: &in.DataRegion},
		{k: "team_id", v: &in.TeamID},
		{k: "logs_retention", v: &in.LogsRetention},
		{k: "metrics_retention", v: &in.MetricsRetention},
		{k: "hosts_count", v: &in.HostsCount},
		{k: "hosts_up_count", v: &in.HostsUpCount},
		{k: "databases_count", v: &in.DatabasesCount},
		{k: "pinged_at", v: &in.PingedAt},
		{k: "created_at", v: &in.CreatedAt},
		{k: "updated_at", v: &in.UpdatedAt},
		{k: "http_basic_auth_username", v: &in.HTTPBasicAuthUsername},
	}
}

// fetchCollectorDatabases fetches databases for a collector separately.
// The collector API response doesn't include databases - only databases_count.
// Databases must be fetched via GET /api/v1/collectors/:id/databases.
func fetchCollectorDatabases(ctx context.Context, meta interface{}, collectorID string) ([]collectorDatabase, diag.Diagnostics) {
	c := meta.(*client)
	res, err := c.Get(ctx, fmt.Sprintf("/api/v1/collectors/%s/databases", url.PathEscape(collectorID)))
	if err != nil {
		return nil, diag.FromErr(err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}()

	body, err := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return nil, diag.Errorf("GET /api/v1/collectors/%s/databases returned %d: %s", collectorID, res.StatusCode, string(body))
	}
	if err != nil {
		return nil, diag.FromErr(err)
	}

	var out collectorDatabasesHTTPResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, diag.FromErr(err)
	}

	databases := make([]collectorDatabase, 0, len(out.Data))
	for _, dbData := range out.Data {
		db := dbData.Attributes
		db.ID = intPtr(dbData.ID)
		databases = append(databases, db)
	}

	return databases, nil
}

func newCollectorResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: collectorCreate,
		ReadContext:   collectorRead,
		UpdateContext: collectorUpdate,
		DeleteContext: collectorDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		CustomizeDiff: validateCollector,
		Description:   "This resource allows you to create, modify, and delete Better Stack Collectors. For more information about the Collectors API check https://betterstack.com/docs/logs/api/collectors/",
		Schema:        collectorSchema,
	}
}

func collectorCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in collector
	for _, e := range collectorRef(&in) {
		if e.k == "team_id" {
			in.TeamID = StringOrIntFromResourceData(d, e.k)
		} else {
			load(d, e.k, e.v)
		}
	}
	load(d, "team_name", &in.TeamName)

	// Load configuration (inline, following source.go pattern)
	if configData, ok := d.GetOk("configuration"); ok {
		configList := configData.([]interface{})
		if len(configList) > 0 {
			configMap := configList[0].(map[string]interface{})
			in.Configuration = &collectorConfiguration{}
			if v, ok := configMap["logs_sample_rate"].(int); ok {
				in.Configuration.LogsSampleRate = intPtr(v)
			}
			if v, ok := configMap["traces_sample_rate"].(int); ok {
				in.Configuration.TracesSampleRate = intPtr(v)
			}
			if componentsData, ok := configMap["collector_components"].([]interface{}); ok && len(componentsData) > 0 {
				cm := componentsData[0].(map[string]interface{})
				in.Configuration.CollectorComponents = &collectorCollectorComponents{
					Beyla:             boolPtrIfSet(cm, "beyla"),
					BeylaBasic:        boolPtrIfSet(cm, "beyla_basic"),
					BeylaFull:         boolPtrIfSet(cm, "beyla_full"),
					ClusterAgent:      boolPtrIfSet(cm, "cluster_agent"),
					HostLogs:          boolPtrIfSet(cm, "host_logs"),
					CollectorInternal: boolPtrIfSet(cm, "collector_internal"),
				}
			}
			if monitoringData, ok := configMap["monitoring_options"].([]interface{}); ok && len(monitoringData) > 0 {
				mm := monitoringData[0].(map[string]interface{})
				in.Configuration.MonitoringOptions = &collectorMonitoringOptions{
					DockerJSONFile:      boolPtrIfSet(mm, "docker_json_file"),
					CollectorKubernetes: boolPtrIfSet(mm, "collector_kubernetes"),
					NginxMetrics:        boolPtrIfSet(mm, "nginx_metrics"),
					ApacheMetrics:       boolPtrIfSet(mm, "apache_metrics"),
				}
			}
			if v, ok := configMap["transformation"].(string); ok && v != "" {
				in.Configuration.Transformation = stringPtr(v)
			}
			if v, ok := configMap["enable_ssl_certificate"].(bool); ok {
				in.Configuration.EnableSSLCertificate = boolPtr(v)
			}
			if v, ok := configMap["ssl_certificate_host"].(string); ok && v != "" {
				in.Configuration.SSLCertificateHost = stringPtr(v)
			}
		}
	}

	// Load HTTP Basic Auth (top-level fields sent as request params)
	if v, ok := d.GetOk("enable_http_basic_auth"); ok {
		in.EnableHTTPBasicAuth = boolPtr(v.(bool))
	}
	if v, ok := d.GetOk("http_basic_auth_username"); ok {
		in.HTTPBasicAuthUsername = stringPtr(v.(string))
	}
	if v, ok := d.GetOk("http_basic_auth_password"); ok {
		in.HTTPBasicAuthPassword = stringPtr(v.(string))
	}

	// Load custom_bucket (inline, following source.go pattern)
	if customBucketData, ok := d.GetOk("custom_bucket"); ok {
		customBucketList := customBucketData.([]interface{})
		if len(customBucketList) > 0 {
			cbm := customBucketList[0].(map[string]interface{})
			in.CustomBucket = &collectorCustomBucket{
				Name:                   stringPtr(cbm["name"].(string)),
				Endpoint:               stringPtr(cbm["endpoint"].(string)),
				AccessKeyID:            stringPtr(cbm["access_key_id"].(string)),
				SecretAccessKey:        stringPtr(cbm["secret_access_key"].(string)),
				KeepDataAfterRetention: boolPtr(cbm["keep_data_after_retention"].(bool)),
			}
		}
	}

	// Load databases
	if databasesData, ok := d.GetOk("databases"); ok {
		databasesList := databasesData.([]interface{})
		databases := make([]collectorDatabase, 0, len(databasesList))
		for _, dbData := range databasesList {
			dbMap := dbData.(map[string]interface{})
			db := collectorDatabase{
				ServiceType: stringPtrIfSet(dbMap, "service_type"),
				Host:        stringPtrIfSet(dbMap, "host"),
				Port:        intPtrIfSet(dbMap, "port"),
				Username:    stringPtrIfSet(dbMap, "username"),
				Password:    stringPtrIfSet(dbMap, "password"),
				SSLMode:     stringPtrIfSet(dbMap, "ssl_mode"),
				TLS:         stringPtrIfSet(dbMap, "tls"),
			}
			databases = append(databases, db)
		}
		in.Databases = &databases
	}

	var out collectorHTTPResponse
	if err := resourceCreate(ctx, meta, "/api/v1/collectors", &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)

	// Fetch databases separately - the API doesn't return them in the create response
	databases, derr := fetchCollectorDatabases(ctx, meta, out.Data.ID)
	if derr != nil {
		return derr
	}
	out.Data.Attributes.Databases = &databases

	return collectorCopyAttrs(d, &out.Data.Attributes)
}

func collectorRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out collectorHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v1/collectors/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Force "create" on 404.
		return nil
	}

	// Fetch databases separately - the API doesn't return them in the show response
	databases, derr := fetchCollectorDatabases(ctx, meta, d.Id())
	if derr != nil {
		return derr
	}
	out.Data.Attributes.Databases = &databases

	return collectorCopyAttrs(d, &out.Data.Attributes)
}

func collectorUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in collector

	for _, e := range collectorRef(&in) {
		if d.HasChange(e.k) {
			if e.k == "team_id" {
				in.TeamID = StringOrIntFromResourceData(d, e.k)
			} else {
				load(d, e.k, e.v)
			}
		}
	}

	// Load configuration if changed
	if d.HasChange("configuration") {
		if configData, ok := d.GetOk("configuration"); ok {
			configList := configData.([]interface{})
			if len(configList) > 0 {
				configMap := configList[0].(map[string]interface{})
				in.Configuration = &collectorConfiguration{}
				if v, ok := configMap["logs_sample_rate"].(int); ok {
					in.Configuration.LogsSampleRate = intPtr(v)
				}
				if v, ok := configMap["traces_sample_rate"].(int); ok {
					in.Configuration.TracesSampleRate = intPtr(v)
				}
				if componentsData, ok := configMap["collector_components"].([]interface{}); ok && len(componentsData) > 0 {
					cm := componentsData[0].(map[string]interface{})
					in.Configuration.CollectorComponents = &collectorCollectorComponents{
						Beyla:             boolPtrIfSet(cm, "beyla"),
						BeylaBasic:        boolPtrIfSet(cm, "beyla_basic"),
						BeylaFull:         boolPtrIfSet(cm, "beyla_full"),
						ClusterAgent:      boolPtrIfSet(cm, "cluster_agent"),
						HostLogs:          boolPtrIfSet(cm, "host_logs"),
						CollectorInternal: boolPtrIfSet(cm, "collector_internal"),
					}
				}
				if monitoringData, ok := configMap["monitoring_options"].([]interface{}); ok && len(monitoringData) > 0 {
					mm := monitoringData[0].(map[string]interface{})
					in.Configuration.MonitoringOptions = &collectorMonitoringOptions{
						DockerJSONFile:      boolPtrIfSet(mm, "docker_json_file"),
						CollectorKubernetes: boolPtrIfSet(mm, "collector_kubernetes"),
						NginxMetrics:        boolPtrIfSet(mm, "nginx_metrics"),
						ApacheMetrics:       boolPtrIfSet(mm, "apache_metrics"),
					}
				}
				if v, ok := configMap["transformation"].(string); ok && v != "" {
					in.Configuration.Transformation = stringPtr(v)
				}
				if v, ok := configMap["enable_ssl_certificate"].(bool); ok {
					in.Configuration.EnableSSLCertificate = boolPtr(v)
				}
				if v, ok := configMap["ssl_certificate_host"].(string); ok && v != "" {
					in.Configuration.SSLCertificateHost = stringPtr(v)
				}
			}
		}
	}

	// Load HTTP Basic Auth if changed (top-level fields sent as request params)
	if d.HasChange("enable_http_basic_auth") {
		if v, ok := d.GetOk("enable_http_basic_auth"); ok {
			in.EnableHTTPBasicAuth = boolPtr(v.(bool))
		} else {
			in.EnableHTTPBasicAuth = boolPtr(false)
		}
	}
	if d.HasChange("http_basic_auth_username") {
		if v, ok := d.GetOk("http_basic_auth_username"); ok {
			in.HTTPBasicAuthUsername = stringPtr(v.(string))
		}
	}
	if d.HasChange("http_basic_auth_password") {
		if v, ok := d.GetOk("http_basic_auth_password"); ok {
			in.HTTPBasicAuthPassword = stringPtr(v.(string))
		}
	}

	// Load custom_bucket if changed
	if d.HasChange("custom_bucket") {
		if customBucketData, ok := d.GetOk("custom_bucket"); ok {
			customBucketList := customBucketData.([]interface{})
			if len(customBucketList) > 0 {
				cbm := customBucketList[0].(map[string]interface{})
				in.CustomBucket = &collectorCustomBucket{
					Name:                   stringPtr(cbm["name"].(string)),
					Endpoint:               stringPtr(cbm["endpoint"].(string)),
					AccessKeyID:            stringPtr(cbm["access_key_id"].(string)),
					SecretAccessKey:        stringPtr(cbm["secret_access_key"].(string)),
					KeepDataAfterRetention: boolPtr(cbm["keep_data_after_retention"].(bool)),
				}
			}
		}
	}

	// Handle databases update with delta computation
	if d.HasChange("databases") {
		oldData, newData := d.GetChange("databases")
		in.Databases = computeDatabasesDelta(oldData.([]interface{}), newData.([]interface{}))
	}

	return resourceUpdate(ctx, meta, fmt.Sprintf("/api/v1/collectors/%s", url.PathEscape(d.Id())), &in)
}

func collectorDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDelete(ctx, meta, fmt.Sprintf("/api/v1/collectors/%s", url.PathEscape(d.Id())))
}

func collectorCopyAttrs(d *schema.ResourceData, in *collector) diag.Diagnostics {
	var derr diag.Diagnostics

	for _, e := range collectorRef(in) {
		if e.k == "data_region" && d.Get("data_region").(string) != "" {
			// Don't update data_region from API if it's already set - it can't change
			continue
		} else if e.k == "team_id" {
			if err := SetStringOrIntResourceData(d, "team_id", in.TeamID); err != nil {
				derr = append(derr, diag.FromErr(err)[0])
			}
		} else if err := d.Set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface()); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	// Copy configuration
	if in.Configuration != nil {
		configData := make(map[string]interface{})
		if in.Configuration.LogsSampleRate != nil {
			configData["logs_sample_rate"] = *in.Configuration.LogsSampleRate
		}
		if in.Configuration.TracesSampleRate != nil {
			configData["traces_sample_rate"] = *in.Configuration.TracesSampleRate
		}
		if in.Configuration.CollectorComponents != nil {
			cc := in.Configuration.CollectorComponents
			componentsData := make(map[string]interface{})
			if cc.Beyla != nil {
				componentsData["beyla"] = *cc.Beyla
			}
			if cc.BeylaBasic != nil {
				componentsData["beyla_basic"] = *cc.BeylaBasic
			}
			if cc.BeylaFull != nil {
				componentsData["beyla_full"] = *cc.BeylaFull
			}
			if cc.ClusterAgent != nil {
				componentsData["cluster_agent"] = *cc.ClusterAgent
			}
			if cc.HostLogs != nil {
				componentsData["host_logs"] = *cc.HostLogs
			}
			if cc.CollectorInternal != nil {
				componentsData["collector_internal"] = *cc.CollectorInternal
			}
			configData["collector_components"] = []interface{}{componentsData}
		}
		if in.Configuration.MonitoringOptions != nil {
			mo := in.Configuration.MonitoringOptions
			monitoringData := make(map[string]interface{})
			if mo.DockerJSONFile != nil {
				monitoringData["docker_json_file"] = *mo.DockerJSONFile
			}
			if mo.CollectorKubernetes != nil {
				monitoringData["collector_kubernetes"] = *mo.CollectorKubernetes
			}
			if mo.NginxMetrics != nil {
				monitoringData["nginx_metrics"] = *mo.NginxMetrics
			}
			if mo.ApacheMetrics != nil {
				monitoringData["apache_metrics"] = *mo.ApacheMetrics
			}
			configData["monitoring_options"] = []interface{}{monitoringData}
		}
		if in.Configuration.Transformation != nil {
			configData["transformation"] = *in.Configuration.Transformation
		}
		if in.Configuration.EnableSSLCertificate != nil {
			configData["enable_ssl_certificate"] = *in.Configuration.EnableSSLCertificate
		}
		if in.Configuration.SSLCertificateHost != nil {
			configData["ssl_certificate_host"] = *in.Configuration.SSLCertificateHost
		}
		if err := d.Set("configuration", []interface{}{configData}); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}

		// Copy enable_http_basic_auth from configuration to top-level
		// The API returns this in configuration.enable_http_basic_auth
		if in.Configuration.EnableHTTPBasicAuth != nil {
			if err := d.Set("enable_http_basic_auth", *in.Configuration.EnableHTTPBasicAuth); err != nil {
				derr = append(derr, diag.FromErr(err)[0])
			}
		}
	}

	// Preserve http_basic_auth_password from state (API never returns it)
	// We don't need to do anything here - Terraform preserves the value in state
	// as long as we don't overwrite it with d.Set()

	// Copy custom_bucket (preserve secret_access_key from state)
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
		// Preserve secret_access_key from existing state (API doesn't return it)
		if existingCustomBucket, ok := d.GetOk("custom_bucket"); ok {
			existingList := existingCustomBucket.([]interface{})
			if len(existingList) > 0 {
				existingMap := existingList[0].(map[string]interface{})
				if secretKey, ok := existingMap["secret_access_key"]; ok {
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

	// Copy databases (preserve passwords from state - API doesn't return them)
	if in.Databases != nil {
		// Build maps for password preservation: by ID and by index
		existingPasswordsByID := make(map[int]string)
		existingPasswordsByIndex := make(map[int]string)
		if existingDatabases, ok := d.GetOk("databases"); ok {
			for i, dbData := range existingDatabases.([]interface{}) {
				dbMap := dbData.(map[string]interface{})
				if password, ok := dbMap["password"].(string); ok && password != "" {
					existingPasswordsByIndex[i] = password
					if id, ok := dbMap["id"].(int); ok && id != 0 {
						existingPasswordsByID[id] = password
					}
				}
			}
		}

		databasesData := make([]interface{}, 0, len(*in.Databases))
		for i, db := range *in.Databases {
			dbData := make(map[string]interface{})
			if db.ID != nil {
				dbData["id"] = *db.ID
			}
			if db.ServiceType != nil {
				dbData["service_type"] = *db.ServiceType
			}
			if db.Host != nil {
				dbData["host"] = *db.Host
			}
			if db.Port != nil {
				dbData["port"] = *db.Port
			}
			if db.Username != nil {
				dbData["username"] = *db.Username
			}
			if db.SSLMode != nil {
				dbData["ssl_mode"] = *db.SSLMode
			}
			if db.TLS != nil {
				dbData["tls"] = *db.TLS
			}
			// Restore password: first try by ID, then by index position
			if db.ID != nil {
				if password, ok := existingPasswordsByID[*db.ID]; ok {
					dbData["password"] = password
				}
			}
			if _, hasPassword := dbData["password"]; !hasPassword {
				if password, ok := existingPasswordsByIndex[i]; ok {
					dbData["password"] = password
				}
			}
			databasesData = append(databasesData, dbData)
		}
		if err := d.Set("databases", databasesData); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	return derr
}

func validateCollector(ctx context.Context, diff *schema.ResourceDiff, v interface{}) error {
	if diff.Id() != "" && diff.HasChange("data_region") {
		return fmt.Errorf("data_region cannot be changed after collector is created")
	}

	if diff.Id() != "" && diff.HasChange("custom_bucket") {
		oldVal, newVal := diff.GetChange("custom_bucket")
		oldList := oldVal.([]interface{})
		newList := newVal.([]interface{})
		if len(oldList) > 0 && len(newList) == 0 {
			return fmt.Errorf("custom_bucket cannot be removed once set - it is a create-only field")
		}
	}

	return nil
}

// computeDatabasesDelta calculates the delta between old and new databases for update.
func computeDatabasesDelta(oldDatabases, newDatabases []interface{}) *[]collectorDatabase {
	oldByID := make(map[int]map[string]interface{})
	for _, dbData := range oldDatabases {
		dbMap := dbData.(map[string]interface{})
		if id, ok := dbMap["id"].(int); ok && id != 0 {
			oldByID[id] = dbMap
		}
	}

	presentIDs := make(map[int]bool)
	result := make([]collectorDatabase, 0, len(newDatabases)+len(oldDatabases))

	for i, dbData := range newDatabases {
		dbMap := dbData.(map[string]interface{})
		db := collectorDatabase{
			ServiceType: stringPtrIfSet(dbMap, "service_type"),
			Host:        stringPtrIfSet(dbMap, "host"),
			Port:        intPtrIfSet(dbMap, "port"),
			Username:    stringPtrIfSet(dbMap, "username"),
			Password:    stringPtrIfSet(dbMap, "password"),
			SSLMode:     stringPtrIfSet(dbMap, "ssl_mode"),
			TLS:         stringPtrIfSet(dbMap, "tls"),
		}

		// Match by position to preserve IDs
		if i < len(oldDatabases) {
			oldDbMap := oldDatabases[i].(map[string]interface{})
			if oldID, ok := oldDbMap["id"].(int); ok && oldID != 0 {
				db.ID = intPtr(oldID)
				presentIDs[oldID] = true
				// Preserve password from old state if not provided
				if db.Password == nil || *db.Password == "" {
					if oldPassword, ok := oldDbMap["password"].(string); ok && oldPassword != "" {
						db.Password = stringPtr(oldPassword)
					}
				}
			}
		}

		result = append(result, db)
	}

	// Mark removed databases for destruction
	for id := range oldByID {
		if !presentIDs[id] {
			result = append(result, collectorDatabase{
				ID:      intPtr(id),
				Destroy: boolPtr(true),
			})
		}
	}

	return &result
}

// Helper functions

func intPtr(i int) *int {
	return &i
}

func stringPtrIfSet(m map[string]interface{}, key string) *string {
	if v, ok := m[key].(string); ok && v != "" {
		return &v
	}
	return nil
}

func intPtrIfSet(m map[string]interface{}, key string) *int {
	if v, ok := m[key].(int); ok && v != 0 {
		return &v
	}
	return nil
}

func boolPtrIfSet(m map[string]interface{}, key string) *bool {
	if v, ok := m[key].(bool); ok {
		return &v
	}
	return nil
}
