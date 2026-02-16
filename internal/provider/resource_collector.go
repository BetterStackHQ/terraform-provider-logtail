package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// normalizeVRL strips whitespace and trailing dots from VRL programs for diff comparison.
// The API normalizes VRL by appending a newline + trailing dot, so plans would show
// spurious diffs without this.
func normalizeVRL(vrl string) string {
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

// hashOptionEntry hashes a service_option or namespace_option entry by all its fields,
// used as the Set function for TypeSet schemas. All fields must be included in the hash
// so that changes to any field (not just name) are detected by the SDK.
func hashOptionEntry(v interface{}) int {
	m := v.(map[string]interface{})
	h := fnv.New32a()
	h.Write([]byte(fmt.Sprintf("%s-%d-%t", m["name"], m["log_sampling"], m["ingest_traces"])))
	return int(h.Sum32())
}

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
	"source_id": {
		Description: "The ID of the underlying source. Use this with `logtail_metric` to define metrics on this collector's data.",
		Type:        schema.TypeInt,
		Optional:    false,
		Computed:    true,
	},
	"source_group_id": {
		Description: "The ID of the source group (folder) this collector belongs to. Set to `0` to remove from a group.",
		Type:        schema.TypeInt,
		Optional:    true,
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			// Treat 0 as equivalent to unset/null
			return (old == "0" || old == "") && (new == "0" || new == "")
		},
	},
	"live_tail_pattern": {
		Description: "Freeform text template for formatting Live tail output with columns wrapped in {column} brackets. Example: \"PID: {message_json.pid} {level} {message}\"",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"data_region": {
		Description: "Data region (e.g. `eu`, `us`) or private cluster name to create the collector in. This can only be set at creation time. Note: the API may return a different identifier (the internal storage region name) than the value you provided.",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"logs_retention": {
		Description:  "Data retention for logs in days. Allowed values: 7, 30, 60, 90, 180, 365, 730, 1095, 1460, 1825. There might be additional charges for longer retention.",
		Type:         schema.TypeInt,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.IntInSlice([]int{7, 30, 60, 90, 180, 365, 730, 1095, 1460, 1825}),
	},
	"metrics_retention": {
		Description:  "Data retention for metrics in days. Allowed values: 7, 30, 60, 90, 180, 365, 730, 1095, 1460, 1825. There might be additional charges for longer retention.",
		Type:         schema.TypeInt,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.IntInSlice([]int{7, 30, 60, 90, 180, 365, 730, 1095, 1460, 1825}),
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
	"ingesting_paused": {
		Description: "Whether ingestion is paused for this collector.",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"user_vector_config": {
		Description: "Custom Vector YAML configuration for additional sources and transforms beyond the built-in component toggles. Must not contain `command:` directives.",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"source_vrl_transformation": {
		Description: "Server-side VRL transformation that runs during ingestion on Better Stack. Use this for enrichment, routing, or light normalization that doesn't involve sensitive data. For PII redaction and sensitive data filtering, prefer `configuration.vrl_transformation` which runs on the collector host and ensures raw data never leaves your network. Read more about [VRL transformations](https://betterstack.com/docs/logs/using-logtail/transforming-ingested-data/logs-vrl/).",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			return normalizeVRL(old) == normalizeVRL(new)
		},
	},
	"configuration": {
		Description: "Collector-level configuration including active components, sampling rates, batching, and VRL transformations. These settings run on the collector host inside your infrastructure.",
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
				"components": {
					Description: "Enable or disable specific collector components. Maps to the Logs, Metrics, and eBPF tabs in the collector settings UI.",
					Type:        schema.TypeList,
					Optional:    true,
					Computed:    true,
					MaxItems:    1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"logs_host":                {Description: "Collect host-level logs.", Type: schema.TypeBool, Optional: true, Computed: true},
							"logs_docker":              {Description: "Collect Docker container logs.", Type: schema.TypeBool, Optional: true, Computed: true},
							"logs_kubernetes":          {Description: "Collect Kubernetes logs.", Type: schema.TypeBool, Optional: true, Computed: true},
							"logs_collector_internals": {Description: "Collect internal collector logs.", Type: schema.TypeBool, Optional: true, Computed: true},
							"metrics_databases":        {Description: "Collect database metrics via the cluster agent.", Type: schema.TypeBool, Optional: true, Computed: true},
							"metrics_nginx":            {Description: "Collect Nginx metrics.", Type: schema.TypeBool, Optional: true, Computed: true},
							"metrics_apache":           {Description: "Collect Apache metrics.", Type: schema.TypeBool, Optional: true, Computed: true},
							"ebpf_metrics":             {Description: "Enable eBPF-based metrics collection.", Type: schema.TypeBool, Optional: true, Computed: true},
							"ebpf_tracing_basic":       {Description: "Enable basic eBPF tracing.", Type: schema.TypeBool, Optional: true, Computed: true},
							"ebpf_tracing_full":        {Description: "Enable full eBPF tracing.", Type: schema.TypeBool, Optional: true, Computed: true},
						},
					},
				},
				"vrl_transformation": {
					Description: "VRL transformation that runs on the collector host, inside your infrastructure, before data is transmitted to Better Stack. Use this for PII redaction and sensitive data filtering — raw data never leaves your network. For server-side transformations that run during ingestion on Better Stack, use the top-level `source_vrl_transformation` attribute instead. Read more about [VRL transformations](https://betterstack.com/docs/logs/using-logtail/transforming-ingested-data/logs-vrl/).",
					Type:        schema.TypeString,
					Optional:    true,
					Computed:    true,
					DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
						return normalizeVRL(old) == normalizeVRL(new)
					},
				},
				"disk_batch_size_mb": {
					Description:  "Disk buffer size in MB for outgoing requests. Minimum 256 MB.",
					Type:         schema.TypeInt,
					Optional:     true,
					Computed:     true,
					ValidateFunc: validation.IntAtLeast(256),
				},
				"memory_batch_size_mb": {
					Description:  "Memory batch size in MB for outgoing requests. Maximum 40 MB.",
					Type:         schema.TypeInt,
					Optional:     true,
					Computed:     true,
					ValidateFunc: validation.IntAtMost(40),
				},
				"service_option": {
					Description: "Per-service overrides for log sampling rate and trace ingestion. Only includes user-managed services; internal collector services (`better-stack-beyla`, `better-stack-collector`) are excluded. See `service_option_all` for the complete server state.",
					Type:        schema.TypeSet,
					Optional:    true,
					Set:         hashOptionEntry,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"name":          {Type: schema.TypeString, Required: true, Description: "Service name."},
							"log_sampling":  {Type: schema.TypeInt, Optional: true, Description: "Log sampling rate (0-100)."},
							"ingest_traces": {Type: schema.TypeBool, Optional: true, Description: "Whether to ingest traces for this service."},
						},
					},
				},
				"service_option_all": {
					Description: "All per-service overrides including server-managed internal defaults (`better-stack-beyla`, `better-stack-collector`). Read-only; to configure services, use `service_option`.",
					Type:        schema.TypeSet,
					Computed:    true,
					Set:         hashOptionEntry,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"name":          {Type: schema.TypeString, Computed: true, Description: "Service name."},
							"log_sampling":  {Type: schema.TypeInt, Computed: true, Description: "Log sampling rate (0-100)."},
							"ingest_traces": {Type: schema.TypeBool, Computed: true, Description: "Whether to ingest traces for this service."},
						},
					},
				},
				"namespace_option": {
					Description: "Per-namespace overrides for log sampling rate and trace ingestion (Kubernetes only). Order-independent; entries are identified by name.",
					Type:        schema.TypeSet,
					Optional:    true,
					Set:         hashOptionEntry,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"name":          {Type: schema.TypeString, Required: true, Description: "Namespace name."},
							"log_sampling":  {Type: schema.TypeInt, Optional: true, Description: "Log sampling rate (0-100)."},
							"ingest_traces": {Type: schema.TypeBool, Optional: true, Description: "Whether to ingest traces for this namespace."},
						},
					},
				},
			},
		},
	},
	"proxy_config": {
		Description: "Proxy settings including buffering proxy, SSL/TLS, and HTTP Basic Authentication. Only applicable to `proxy` platform collectors.",
		Type:        schema.TypeList,
		Optional:    true,
		MaxItems:    1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"enable_buffering_proxy": {
					Description: "Enable the HTTP buffering proxy for the collector.",
					Type:        schema.TypeBool,
					Optional:    true,
					Default:     false,
				},
				"buffering_proxy_listen_on": {
					Description: "Address and port for the buffering proxy to listen on.",
					Type:        schema.TypeString,
					Optional:    true,
				},
				"enable_ssl_certificate": {
					Description: "Enable custom SSL/TLS certificate for the collector.",
					Type:        schema.TypeBool,
					Optional:    true,
					Default:     false,
				},
				"ssl_certificate_host": {
					Description: "Hostname for the SSL certificate.",
					Type:        schema.TypeString,
					Optional:    true,
				},
				"enable_http_basic_auth": {
					Description: "Enable HTTP Basic Authentication for the collector proxy.",
					Type:        schema.TypeBool,
					Optional:    true,
					Default:     false,
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
			},
		},
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
				"ssl_mode":     {Description: "SSL mode for PostgreSQL connections. Valid values: `disable`, `require`, `verify-ca`.", Type: schema.TypeString, Optional: true, ValidateFunc: validation.StringInSlice([]string{"disable", "require", "verify-ca"}, false)},
				"tls":          {Description: "TLS mode for MySQL connections. Valid values: `false`, `true`, `skip-verify`, `preferred`.", Type: schema.TypeString, Optional: true, ValidateFunc: validation.StringInSlice([]string{"false", "true", "skip-verify", "preferred"}, false)},
			},
		},
	},
}

// Go structs for API serialization

// collectorComponents maps the flat API component names to their JSON keys.
type collectorComponents struct {
	LogsHost              *bool `json:"logs_host,omitempty"`
	LogsDocker            *bool `json:"logs_docker,omitempty"`
	LogsKubernetes        *bool `json:"logs_kubernetes,omitempty"`
	LogsCollectorInternal *bool `json:"logs_collector_internals,omitempty"`
	MetricsDatabases      *bool `json:"metrics_databases,omitempty"`
	MetricsNginx          *bool `json:"metrics_nginx,omitempty"`
	MetricsApache         *bool `json:"metrics_apache,omitempty"`
	EbpfMetrics           *bool `json:"ebpf_metrics,omitempty"`
	EbpfTracingBasic      *bool `json:"ebpf_tracing_basic,omitempty"`
	EbpfTracingFull       *bool `json:"ebpf_tracing_full,omitempty"`
}

type collectorEntityOption struct {
	LogSampling  *int  `json:"log_sampling,omitempty"`
	IngestTraces *bool `json:"ingest_traces,omitempty"`
}

type collectorConfiguration struct {
	LogsSampleRate    *int                             `json:"logs_sample_rate,omitempty"`
	TracesSampleRate  *int                             `json:"traces_sample_rate,omitempty"`
	Components        *collectorComponents             `json:"components,omitempty"`
	VRLTransformation *string                          `json:"vrl_transformation,omitempty"`
	DiskBatchSizeMB   *int                             `json:"disk_batch_size_mb,omitempty"`
	MemoryBatchSizeMB *int                             `json:"memory_batch_size_mb,omitempty"`
	ServicesOptions   map[string]collectorEntityOption `json:"services_options,omitempty"`
	NamespacesOptions map[string]collectorEntityOption `json:"namespaces_options,omitempty"`
}

type collectorProxyConfig struct {
	EnableBufferingProxy   *bool   `json:"enable_buffering_proxy,omitempty"`
	BufferingProxyListenOn *string `json:"buffering_proxy_listen_on,omitempty"`
	EnableSSLCertificate   *bool   `json:"enable_ssl_certificate,omitempty"`
	SSLCertificateHost     *string `json:"ssl_certificate_host,omitempty"`
	EnableHTTPBasicAuth    *bool   `json:"enable_http_basic_auth,omitempty"`
	HTTPBasicAuthUsername  *string `json:"http_basic_auth_username,omitempty"`
	HTTPBasicAuthPassword  *string `json:"http_basic_auth_password,omitempty"`
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
	Name                    *string                 `json:"name,omitempty"`
	Platform                *string                 `json:"platform,omitempty"`
	Note                    *string                 `json:"note,omitempty"`
	Status                  *string                 `json:"status,omitempty"`
	Secret                  *string                 `json:"secret,omitempty"`
	SourceID                *int                    `json:"source_id,omitempty"`
	SourceGroupID           *int                    `json:"source_group_id,omitempty"`
	LiveTailPattern         *string                 `json:"live_tail_pattern,omitempty"`
	DataRegion              *string                 `json:"data_region,omitempty"`
	TeamID                  *StringOrInt            `json:"team_id,omitempty"`
	TeamName                *string                 `json:"team_name,omitempty"`
	LogsRetention           *int                    `json:"logs_retention,omitempty"`
	MetricsRetention        *int                    `json:"metrics_retention,omitempty"`
	IngestingPaused         *bool                   `json:"ingesting_paused,omitempty"`
	HostsCount              *int                    `json:"hosts_count,omitempty"`
	HostsUpCount            *int                    `json:"hosts_up_count,omitempty"`
	DatabasesCount          *int                    `json:"databases_count,omitempty"`
	PingedAt                *string                 `json:"pinged_at,omitempty"`
	CreatedAt               *string                 `json:"created_at,omitempty"`
	UpdatedAt               *string                 `json:"updated_at,omitempty"`
	UserVectorConfig        *string                 `json:"user_vector_config,omitempty"`
	SourceVrlTransformation *string                 `json:"source_vrl_transformation,omitempty"`
	Configuration           *collectorConfiguration `json:"configuration,omitempty"`
	ProxyConfig             *collectorProxyConfig   `json:"proxy_config,omitempty"`
	CustomBucket            *collectorCustomBucket  `json:"custom_bucket,omitempty"`
	Databases               *[]collectorDatabase    `json:"databases,omitempty"`
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
		ID         json.Number       `json:"id"`
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
		{k: "source_id", v: &in.SourceID},
		{k: "source_group_id", v: &in.SourceGroupID},
		{k: "live_tail_pattern", v: &in.LiveTailPattern},
		{k: "data_region", v: &in.DataRegion},
		{k: "team_id", v: &in.TeamID},
		{k: "logs_retention", v: &in.LogsRetention},
		{k: "metrics_retention", v: &in.MetricsRetention},
		{k: "ingesting_paused", v: &in.IngestingPaused},
		{k: "hosts_count", v: &in.HostsCount},
		{k: "hosts_up_count", v: &in.HostsUpCount},
		{k: "databases_count", v: &in.DatabasesCount},
		{k: "pinged_at", v: &in.PingedAt},
		{k: "created_at", v: &in.CreatedAt},
		{k: "updated_at", v: &in.UpdatedAt},
		{k: "user_vector_config", v: &in.UserVectorConfig},
		{k: "source_vrl_transformation", v: &in.SourceVrlTransformation},
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
		if id, err := dbData.ID.Int64(); err == nil {
			db.ID = intPtr(int(id))
		}
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

// loadCollectorConfiguration reads the configuration block from Terraform state into the API struct.
func loadCollectorConfiguration(d *schema.ResourceData) *collectorConfiguration {
	configData, ok := d.GetOk("configuration")
	if !ok {
		return nil
	}
	configList := configData.([]interface{})
	if len(configList) == 0 {
		return nil
	}
	configMap := configList[0].(map[string]interface{})
	cfg := &collectorConfiguration{}

	if v, ok := configMap["logs_sample_rate"].(int); ok {
		cfg.LogsSampleRate = intPtr(v)
	}
	if v, ok := configMap["traces_sample_rate"].(int); ok {
		cfg.TracesSampleRate = intPtr(v)
	}

	// Components (flat block with 10 boolean fields)
	if componentsData, ok := configMap["components"].([]interface{}); ok && len(componentsData) > 0 {
		cm := componentsData[0].(map[string]interface{})
		cfg.Components = &collectorComponents{
			LogsHost:              boolPtrIfSet(cm, "logs_host"),
			LogsDocker:            boolPtrIfSet(cm, "logs_docker"),
			LogsKubernetes:        boolPtrIfSet(cm, "logs_kubernetes"),
			LogsCollectorInternal: boolPtrIfSet(cm, "logs_collector_internals"),
			MetricsDatabases:      boolPtrIfSet(cm, "metrics_databases"),
			MetricsNginx:          boolPtrIfSet(cm, "metrics_nginx"),
			MetricsApache:         boolPtrIfSet(cm, "metrics_apache"),
			EbpfMetrics:           boolPtrIfSet(cm, "ebpf_metrics"),
			EbpfTracingBasic:      boolPtrIfSet(cm, "ebpf_tracing_basic"),
			EbpfTracingFull:       boolPtrIfSet(cm, "ebpf_tracing_full"),
		}
	}

	if v, ok := configMap["vrl_transformation"].(string); ok && v != "" {
		cfg.VRLTransformation = stringPtr(v)
	}
	if v, ok := configMap["disk_batch_size_mb"].(int); ok && v != 0 {
		cfg.DiskBatchSizeMB = intPtr(v)
	}
	if v, ok := configMap["memory_batch_size_mb"].(int); ok && v != 0 {
		cfg.MemoryBatchSizeMB = intPtr(v)
	}

	// Load service_option set → services_options map
	if serviceOptionsSet, ok := configMap["service_option"].(*schema.Set); ok && serviceOptionsSet.Len() > 0 {
		serviceOptionsData := serviceOptionsSet.List()
		servicesOptions := make(map[string]collectorEntityOption)
		for _, soData := range serviceOptionsData {
			so := soData.(map[string]interface{})
			name := so["name"].(string)
			opt := collectorEntityOption{}
			if v, ok := so["log_sampling"].(int); ok {
				opt.LogSampling = intPtr(v)
			}
			if v, ok := so["ingest_traces"].(bool); ok {
				opt.IngestTraces = boolPtr(v)
			}
			servicesOptions[name] = opt
		}
		cfg.ServicesOptions = servicesOptions
	}

	// Load namespace_option set → namespaces_options map
	if namespaceOptionsSet, ok := configMap["namespace_option"].(*schema.Set); ok && namespaceOptionsSet.Len() > 0 {
		namespaceOptionsData := namespaceOptionsSet.List()
		namespacesOptions := make(map[string]collectorEntityOption)
		for _, noData := range namespaceOptionsData {
			no := noData.(map[string]interface{})
			name := no["name"].(string)
			opt := collectorEntityOption{}
			if v, ok := no["log_sampling"].(int); ok {
				opt.LogSampling = intPtr(v)
			}
			if v, ok := no["ingest_traces"].(bool); ok {
				opt.IngestTraces = boolPtr(v)
			}
			namespacesOptions[name] = opt
		}
		cfg.NamespacesOptions = namespacesOptions
	}

	return cfg
}

// loadCollectorProxyConfig reads the proxy_config block into the API struct.
func loadCollectorProxyConfig(d *schema.ResourceData, in *collector) {
	proxyData, ok := d.GetOk("proxy_config")
	if !ok {
		return
	}
	proxyList := proxyData.([]interface{})
	if len(proxyList) == 0 {
		return
	}
	pm := proxyList[0].(map[string]interface{})

	pc := &collectorProxyConfig{}
	if v, ok := pm["enable_buffering_proxy"].(bool); ok {
		pc.EnableBufferingProxy = boolPtr(v)
	}
	if v, ok := pm["buffering_proxy_listen_on"].(string); ok && v != "" {
		pc.BufferingProxyListenOn = stringPtr(v)
	}
	if v, ok := pm["enable_ssl_certificate"].(bool); ok {
		pc.EnableSSLCertificate = boolPtr(v)
	}
	if v, ok := pm["ssl_certificate_host"].(string); ok && v != "" {
		pc.SSLCertificateHost = stringPtr(v)
	}
	if v, ok := pm["enable_http_basic_auth"].(bool); ok {
		pc.EnableHTTPBasicAuth = boolPtr(v)
	}
	if v, ok := pm["http_basic_auth_username"].(string); ok && v != "" {
		pc.HTTPBasicAuthUsername = stringPtr(v)
	}
	if v, ok := pm["http_basic_auth_password"].(string); ok && v != "" {
		pc.HTTPBasicAuthPassword = stringPtr(v)
	}
	in.ProxyConfig = pc
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

	// Load configuration
	in.Configuration = loadCollectorConfiguration(d)

	// Load proxy config (buffering proxy, SSL, HTTP Basic Auth)
	loadCollectorProxyConfig(d, &in)

	// Load custom_bucket
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
		in.Configuration = loadCollectorConfiguration(d)
	}

	// Load proxy config if changed (buffering proxy, SSL, HTTP Basic Auth)
	if d.HasChange("proxy_config") {
		loadCollectorProxyConfig(d, &in)
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

	// Copy configuration — only set the block if there are meaningful (non-default) fields.
	if in.Configuration != nil {
		configData := make(map[string]interface{})
		if in.Configuration.LogsSampleRate != nil {
			configData["logs_sample_rate"] = *in.Configuration.LogsSampleRate
		}
		if in.Configuration.TracesSampleRate != nil {
			configData["traces_sample_rate"] = *in.Configuration.TracesSampleRate
		}

		// Copy components (flat struct)
		if in.Configuration.Components != nil {
			c := in.Configuration.Components
			componentsData := make(map[string]interface{})
			if c.LogsHost != nil {
				componentsData["logs_host"] = *c.LogsHost
			}
			if c.LogsDocker != nil {
				componentsData["logs_docker"] = *c.LogsDocker
			}
			if c.LogsKubernetes != nil {
				componentsData["logs_kubernetes"] = *c.LogsKubernetes
			}
			if c.LogsCollectorInternal != nil {
				componentsData["logs_collector_internals"] = *c.LogsCollectorInternal
			}
			if c.MetricsDatabases != nil {
				componentsData["metrics_databases"] = *c.MetricsDatabases
			}
			if c.MetricsNginx != nil {
				componentsData["metrics_nginx"] = *c.MetricsNginx
			}
			if c.MetricsApache != nil {
				componentsData["metrics_apache"] = *c.MetricsApache
			}
			if c.EbpfMetrics != nil {
				componentsData["ebpf_metrics"] = *c.EbpfMetrics
			}
			if c.EbpfTracingBasic != nil {
				componentsData["ebpf_tracing_basic"] = *c.EbpfTracingBasic
			}
			if c.EbpfTracingFull != nil {
				componentsData["ebpf_tracing_full"] = *c.EbpfTracingFull
			}
			configData["components"] = []interface{}{componentsData}
		}

		if in.Configuration.VRLTransformation != nil {
			configData["vrl_transformation"] = *in.Configuration.VRLTransformation
		}
		if in.Configuration.DiskBatchSizeMB != nil {
			configData["disk_batch_size_mb"] = *in.Configuration.DiskBatchSizeMB
		}
		if in.Configuration.MemoryBatchSizeMB != nil {
			configData["memory_batch_size_mb"] = *in.Configuration.MemoryBatchSizeMB
		}

		// Copy services_options map → service_option (user-managed, excludes better-stack-* internal
		// defaults) and service_option_all (complete server state). Follows the tags/tags_all pattern.
		if in.Configuration.ServicesOptions != nil {
			serviceOptionAll := make([]interface{}, 0, len(in.Configuration.ServicesOptions))
			serviceOptionData := make([]interface{}, 0, len(in.Configuration.ServicesOptions))
			names := make([]string, 0, len(in.Configuration.ServicesOptions))
			for name := range in.Configuration.ServicesOptions {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				opt := in.Configuration.ServicesOptions[name]
				entry := map[string]interface{}{"name": name}
				if opt.LogSampling != nil {
					entry["log_sampling"] = *opt.LogSampling
				}
				if opt.IngestTraces != nil {
					entry["ingest_traces"] = *opt.IngestTraces
				}
				serviceOptionAll = append(serviceOptionAll, entry)
				if !strings.HasPrefix(name, "better-stack-") && !strings.HasPrefix(name, "better-stack_") {
					serviceOptionData = append(serviceOptionData, entry)
				}
			}
			configData["service_option"] = serviceOptionData
			configData["service_option_all"] = serviceOptionAll
		}

		// Copy namespaces_options map → namespace_option set
		if in.Configuration.NamespacesOptions != nil {
			namespaceOptionData := make([]interface{}, 0, len(in.Configuration.NamespacesOptions))
			names := make([]string, 0, len(in.Configuration.NamespacesOptions))
			for name := range in.Configuration.NamespacesOptions {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				opt := in.Configuration.NamespacesOptions[name]
				entry := map[string]interface{}{"name": name}
				if opt.LogSampling != nil {
					entry["log_sampling"] = *opt.LogSampling
				}
				if opt.IngestTraces != nil {
					entry["ingest_traces"] = *opt.IngestTraces
				}
				namespaceOptionData = append(namespaceOptionData, entry)
			}
			configData["namespace_option"] = namespaceOptionData
		}

		// Only set the configuration block if it has user-facing fields.
		// Setting an empty configuration block would cause Terraform to fill in
		// zero-value defaults and produce a non-empty plan.
		if len(configData) > 0 {
			if err := d.Set("configuration", []interface{}{configData}); err != nil {
				derr = append(derr, diag.FromErr(err)[0])
			}
		}
	}

	// Copy proxy_config from the API response (only returned for proxy-platform collectors).
	if in.ProxyConfig != nil {
		proxyData := make(map[string]interface{})
		if in.ProxyConfig.EnableBufferingProxy != nil {
			proxyData["enable_buffering_proxy"] = *in.ProxyConfig.EnableBufferingProxy
		}
		if in.ProxyConfig.BufferingProxyListenOn != nil {
			proxyData["buffering_proxy_listen_on"] = *in.ProxyConfig.BufferingProxyListenOn
		}
		if in.ProxyConfig.EnableSSLCertificate != nil {
			proxyData["enable_ssl_certificate"] = *in.ProxyConfig.EnableSSLCertificate
		}
		if in.ProxyConfig.SSLCertificateHost != nil {
			proxyData["ssl_certificate_host"] = *in.ProxyConfig.SSLCertificateHost
		}
		if in.ProxyConfig.EnableHTTPBasicAuth != nil {
			proxyData["enable_http_basic_auth"] = *in.ProxyConfig.EnableHTTPBasicAuth
		}
		if in.ProxyConfig.HTTPBasicAuthUsername != nil {
			proxyData["http_basic_auth_username"] = *in.ProxyConfig.HTTPBasicAuthUsername
		}

		// Preserve http_basic_auth_password from existing state (API never returns it)
		if existingProxy, ok := d.GetOk("proxy_config"); ok {
			existingList := existingProxy.([]interface{})
			if len(existingList) > 0 {
				existingMap := existingList[0].(map[string]interface{})
				if password, ok := existingMap["http_basic_auth_password"]; ok {
					proxyData["http_basic_auth_password"] = password
				}
			}
		}

		if err := d.Set("proxy_config", []interface{}{proxyData}); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	} else if _, userDeclared := d.GetOk("proxy_config"); userDeclared {
		// API returned nil proxy_config (non-proxy platform) but user declared it —
		// clear it from state to avoid drift.
		if err := d.Set("proxy_config", nil); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

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
	// Reject proxy_config for non-proxy platforms at plan time
	if proxyConfig, ok := diff.GetOk("proxy_config"); ok {
		proxyList := proxyConfig.([]interface{})
		if len(proxyList) > 0 {
			platform := diff.Get("platform").(string)
			if platform != "proxy" {
				return fmt.Errorf("proxy_config is only applicable to proxy platform collectors, but platform is %q", platform)
			}
		}
	}

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
		// The API also rejects modifications to existing custom_bucket fields,
		// so block any changes when the bucket was already set.
		if len(oldList) > 0 && len(newList) > 0 {
			return fmt.Errorf("custom_bucket fields cannot be modified after creation - the bucket configuration is immutable once set")
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
