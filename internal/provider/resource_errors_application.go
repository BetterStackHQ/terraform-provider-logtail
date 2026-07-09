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

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/customdiff"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var errorsPlatformTypes = []string{
	"aiohttp_errors",
	"android_errors",
	"angular_errors",
	"asgi_errors",
	"asp_dot_net_core_errors",
	"asp_dot_net_errors",
	"astro_errors",
	"aws_lambda_dot_net_errors",
	"aws_lambda_node_errors",
	"aws_lambda_python_errors",
	"azure_functions_node_errors",
	"bottle_errors",
	"bun_errors",
	"celery_errors",
	"chalice_errors",
	"cli_errors",
	"cloudflare_pages_errors",
	"connect_errors",
	"dart_errors",
	"deno_errors",
	"django_errors",
	"dot_net_errors",
	"dot_net_http_errors",
	"dot_net_maui_errors",
	"echo_errors",
	"electron_errors",
	"elixir_errors",
	"ember_errors",
	"express_errors",
	"falcon_errors",
	"fastapi_errors",
	"fasthttp_errors",
	"fastify_errors",
	"fiber_errors",
	"flask_errors",
	"flutter_errors",
	"gatsby_errors",
	"gin_errors",
	"go_errors",
	"godot_errors",
	"google_cloud_function_dot_net_errors",
	"google_cloud_function_node_errors",
	"google_cloud_function_python_errors",
	"hapi_errors",
	"ios_errors",
	"iris_errors",
	"java_errors",
	"javascript_errors",
	"koa_errors",
	"kotlin_errors",
	"laravel_errors",
	"log4j_errors",
	"logback_errors",
	"macos_errors",
	"minidump_errors",
	"native_errors",
	"negroni_errors",
	"nest_js_errors",
	"next_js_errors",
	"node_errors",
	"nuxt_errors",
	"php_errors",
	"powershell_errors",
	"pyramid_errors",
	"python_errors",
	"qt_errors",
	"quart_errors",
	"rack_middleware_errors",
	"rails_errors",
	"react_errors",
	"react_native_errors",
	"react_router_framework_errors",
	"remix_errors",
	"rq_errors",
	"ruby_errors",
	"rust_errors",
	"sanic_errors",
	"serverless_python_errors",
	"solid_errors",
	"solidstart_errors",
	"spring_boot_errors",
	"spring_errors",
	"starlette_errors",
	"svelte_errors",
	"sveltekit_errors",
	"symfony_errors",
	"tanstack_start_react_errors",
	"tornado_errors",
	"tryton_errors",
	"unity_errors",
	"unreal_engine_errors",
	"vue_errors",
	"windows_forms_errors",
	"wpf_errors",
	"wsgi_errors",
}

var errorsApplicationSchema = map[string]*schema.Schema{
	"team_name": teamNameSchema(),
	"team_id": {
		Description: "The team ID for this resource.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"id": {
		Description: "The ID of this application.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"name": {
		Description: "Application name. Must be unique within your team.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"token": {
		Description: "The token of this application. This token is used to identify and route the data you will send to Better Stack.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"table_name": {
		Description: "The table name generated for this application.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"platform": {
		Description: "The platform type for the application. This helps configure appropriate SDKs and integrations. You can't update this value later. Valid values are:\n    - `" + strings.Join(errorsPlatformTypes, "`\n    - `") + "`",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
		ValidateDiagFunc: func(v interface{}, path cty.Path) diag.Diagnostics {
			s := v.(string)
			for _, platformType := range errorsPlatformTypes {
				if s == platformType {
					return nil
				}
			}
			return diag.Diagnostics{
				diag.Diagnostic{
					AttributePath: path,
					Severity:      diag.Error,
					Summary:       `Invalid "platform"`,
					Detail:        fmt.Sprintf("Expected one of %v", errorsPlatformTypes),
				},
			}
		},
	},
	"ingesting_host": {
		Description: "The host where the errors should be sent. See documentation for your specific platform for details.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"ingesting_paused": {
		Description: "This property allows you to temporarily pause data ingesting for this application.",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"errors_retention": {
		Description:  "Error data retention period in days. Default retention is 90 days.",
		Type:         schema.TypeInt,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.IntBetween(1, 3652), // Must be between 1 day and 10 years
	},
	"created_at": {
		Description: "The time when this application was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this application was updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"data_region": {
		Description: "Data region or private cluster name to create the application in. Permitted values for most plans are: `us_east`, `germany`, `singapore`. " +
			"This value can only be set at creation time and cannot be changed afterwards. " +
			"The API returns the specific cluster name, which may differ from the value you provide (for example, `germany` may read back as `eu-nbg-2`).  \n" +
			"When importing an existing application, leave `data_region` unset in your configuration - Terraform reads it from the API. " +
			"Pinning it to an identifier that differs from the stored cluster name produces a spurious `data_region cannot be changed after application is created` error.",
		Type:     schema.TypeString,
		Optional: true,
		Computed: true,
	},
	"code_mapping_stack_root": {
		Description: "Stack trace root path prefix to match. When a stack trace file starts with this prefix, it will be replaced with the source code root to map to the correct repository path.",
		Type:        schema.TypeString,
		Optional:    true,
	},
	"code_mapping_source_root": {
		Description: "Source code root path that replaces the stack trace root prefix. Used to map container or build paths to the corresponding repository paths for git blame.",
		Type:        schema.TypeString,
		Optional:    true,
	},
	"application_group_id": {
		Description: "ID of the application group this application belongs to. Set to `0` to remove from a group.",
		Type:        schema.TypeInt,
		Optional:    true,
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			// Check if the attribute is actually set in config using raw config
			rawConfig := d.GetRawConfig()
			if !rawConfig.IsNull() && rawConfig.IsKnown() {
				val := rawConfig.GetAttr("application_group_id")
				if val.IsNull() || !val.IsKnown() {
					// null/unset in config means "don't manage" - suppress diff
					return true
				}
			}
			// 0 in config means "explicitly no group" - suppress only if state is also 0 or empty
			if new == "0" {
				return old == "0" || old == ""
			}
			return false
		},
	},
	"correlate_with_source_id": {
		Description: "ID of an existing source to correlate errors from this application with, for log and trace correlation. Cannot be changed after the application is created.",
		Type:        schema.TypeInt,
		Optional:    true,
	},
	"github_repository_name": {
		Description:   "Full name of a GitHub repository (e.g. `owner/repo`) to connect to this application for source links, git blame, and AI-assisted fixes. The repository must already be connected to your team's GitHub integration. Set to an empty string to disconnect. Mutually exclusive with `gitlab_repository_name`.",
		Type:          schema.TypeString,
		Optional:      true,
		Computed:      true,
		ConflictsWith: []string{"gitlab_repository_name"},
	},
	"gitlab_repository_name": {
		Description:   "Full name of a GitLab repository (e.g. `group/project`) to connect to this application for source links, git blame, and AI-assisted fixes. The repository must already be connected to your team's GitLab integration. Set to an empty string to disconnect. Mutually exclusive with `github_repository_name`.",
		Type:          schema.TypeString,
		Optional:      true,
		Computed:      true,
		ConflictsWith: []string{"github_repository_name"},
	},
	"custom_bucket": {
		Description: "Optional custom S3-compatible bucket configuration for the application. " +
			"Can only be set when creating the application and cannot be added, changed, or removed afterwards - recreate the application to use a different bucket. " +
			"Better Stack validates the credentials by writing and reading a test object in the bucket during creation.",
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Description: "Bucket name. Safe to omit - the bucket name will be derived from `endpoint`.",
					Type:        schema.TypeString,
					Optional:    true,
					Computed:    true,
				},
				"endpoint": {
					Description: "Bucket endpoint including the bucket name, e.g. `https://s3.us-east-1.amazonaws.com/my-bucket` or `https://my-bucket.s3.us-east-1.amazonaws.com`.",
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
}

func newErrorsApplicationResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: errorsApplicationCreate,
		ReadContext:   errorsApplicationRead,
		UpdateContext: errorsApplicationUpdate,
		DeleteContext: errorsApplicationDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		CustomizeDiff: customdiff.Sequence(validateTeamNameNotChanged, validateErrorsApplication, customizeDiffRepositoryName("github_repository_name"), customizeDiffRepositoryName("gitlab_repository_name")),
		Description:   "This resource allows you to create, modify, and delete your Errors applications. For more information about the Errors API check https://betterstack.com/docs/errors/api/applications/create/",
		Schema:        errorsApplicationSchema,
	}
}

type errorsApplication struct {
	Name                  *string             `json:"name,omitempty"`
	Token                 *string             `json:"token,omitempty"`
	TeamId                *StringOrInt        `json:"team_id,omitempty"`
	TableName             *string             `json:"table_name,omitempty"`
	Platform              *string             `json:"platform,omitempty"`
	IngestingHost         *string             `json:"ingesting_host,omitempty"`
	IngestingPaused       *bool               `json:"ingesting_paused,omitempty"`
	ErrorsRetention       *int                `json:"errors_retention,omitempty"`
	CreatedAt             *string             `json:"created_at,omitempty"`
	UpdatedAt             *string             `json:"updated_at,omitempty"`
	TeamName              *string             `json:"team_name,omitempty"`
	DataRegion            *string             `json:"data_region,omitempty"`
	CodeMappingStackRoot  *string             `json:"code_mapping_stack_root,omitempty"`
	CodeMappingSourceRoot *string             `json:"code_mapping_source_root,omitempty"`
	ApplicationGroupID    *int                `json:"application_group_id,omitempty"`
	CorrelateWithSourceID *int                `json:"correlate_with_source_id,omitempty"`
	GithubRepositoryName  *string             `json:"github_repository_name,omitempty"`
	GitlabRepositoryName  *string             `json:"gitlab_repository_name,omitempty"`
	CustomBucket          *sourceCustomBucket `json:"custom_bucket,omitempty"`
}

type errorsApplicationHTTPResponse struct {
	Data struct {
		ID         string            `json:"id"`
		Attributes errorsApplication `json:"attributes"`
	} `json:"data"`
}

func errorsApplicationRef(in *errorsApplication) []struct {
	k string
	v interface{}
} {
	return []struct {
		k string
		v interface{}
	}{
		{k: "name", v: &in.Name},
		{k: "token", v: &in.Token},
		{k: "team_id", v: &in.TeamId},
		{k: "table_name", v: &in.TableName},
		{k: "platform", v: &in.Platform},
		{k: "ingesting_host", v: &in.IngestingHost},
		{k: "ingesting_paused", v: &in.IngestingPaused},
		{k: "errors_retention", v: &in.ErrorsRetention},
		{k: "created_at", v: &in.CreatedAt},
		{k: "updated_at", v: &in.UpdatedAt},
		{k: "data_region", v: &in.DataRegion},
		{k: "code_mapping_stack_root", v: &in.CodeMappingStackRoot},
		{k: "code_mapping_source_root", v: &in.CodeMappingSourceRoot},
		{k: "application_group_id", v: &in.ApplicationGroupID},
		{k: "correlate_with_source_id", v: &in.CorrelateWithSourceID},
		{k: "github_repository_name", v: &in.GithubRepositoryName},
		{k: "gitlab_repository_name", v: &in.GitlabRepositoryName},
	}
}

func errorsApplicationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in errorsApplication
	for _, e := range errorsApplicationRef(&in) {
		if e.k == "team_id" {
			in.TeamId = StringOrIntFromResourceData(d, e.k)
		} else if e.k == "application_group_id" {
			// Use intFromResourceData to properly distinguish null vs 0
			in.ApplicationGroupID = intFromResourceData(d, e.k)
		} else if e.k == "github_repository_name" {
			// Use stringFromResourceData to distinguish null (UI-managed) from "" (disconnect)
			in.GithubRepositoryName = stringFromResourceData(d, e.k)
		} else if e.k == "gitlab_repository_name" {
			// Use stringFromResourceData to distinguish null (UI-managed) from "" (disconnect)
			in.GitlabRepositoryName = stringFromResourceData(d, e.k)
		} else {
			load(d, e.k, e.v)
		}
	}

	load(d, "team_name", &in.TeamName)

	if customBucketData, ok := d.GetOk("custom_bucket"); ok {
		customBucketList := customBucketData.([]interface{})
		if len(customBucketList) > 0 {
			customBucketMap := customBucketList[0].(map[string]interface{})
			in.CustomBucket = &sourceCustomBucket{
				Endpoint:               stringPtr(customBucketMap["endpoint"].(string)),
				AccessKeyID:            stringPtr(customBucketMap["access_key_id"].(string)),
				SecretAccessKey:        stringPtr(customBucketMap["secret_access_key"].(string)),
				KeepDataAfterRetention: boolPtr(customBucketMap["keep_data_after_retention"].(bool)),
			}
			// name is passed along when set, but the API ignores it and stores the bucket
			// name parsed out of the endpoint URL instead (state keeps the configured
			// value - see errorsApplicationCopyAttrs).
			if name := customBucketMap["name"].(string); name != "" {
				in.CustomBucket.Name = stringPtr(name)
			}
		}
	}

	var out errorsApplicationHTTPResponse
	if err := resourceCreateWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), "/api/v1/applications", &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	// Ensure platform is set in state since API doesn't return it
	if platform := d.Get("platform").(string); platform != "" {
		if err := d.Set("platform", platform); err != nil {
			return diag.FromErr(err)
		}
	}
	return errorsApplicationCopyAttrs(d, &out.Data.Attributes)
}

func errorsApplicationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out errorsApplicationHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), fmt.Sprintf("/api/v1/applications/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Force "create" on 404.
		return nil
	}
	return errorsApplicationCopyAttrs(d, &out.Data.Attributes)
}

func errorsApplicationCopyAttrs(d *schema.ResourceData, in *errorsApplication) diag.Diagnostics {
	var derr diag.Diagnostics
	for _, e := range errorsApplicationRef(in) {
		if e.k == "data_region" && d.Get("data_region").(string) != "" {
			// Don't update data region from API if it's already set - data_region can't change
			continue
		} else if e.k == "platform" && d.Get("platform").(string) != "" {
			// Don't update platform from API if it's already set - platform can't change and API doesn't return it
			continue
		} else if e.k == "team_id" {
			if err := SetStringOrIntResourceData(d, "team_id", in.TeamId); err != nil {
				derr = append(derr, diag.FromErr(err)[0])
			}
		} else if err := d.Set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface()); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	if in.CustomBucket != nil {
		customBucketData := make(map[string]interface{})
		var existingName string
		var existingSecret interface{}
		if existingCustomBucket, ok := d.GetOk("custom_bucket"); ok {
			existingCustomBucketList := existingCustomBucket.([]interface{})
			if len(existingCustomBucketList) > 0 {
				existingCustomBucketMap := existingCustomBucketList[0].(map[string]interface{})
				existingName, _ = existingCustomBucketMap["name"].(string)
				existingSecret = existingCustomBucketMap["secret_access_key"]
			}
		}
		// The API stores a bucket name parsed out of the endpoint URL, ignoring the one
		// sent to it. Keep the configured name when there is one (like data_region) and
		// only read the derived name back when name is omitted or state is fresh (import).
		if existingName != "" {
			customBucketData["name"] = existingName
		} else if in.CustomBucket.Name != nil {
			customBucketData["name"] = *in.CustomBucket.Name
		}
		if in.CustomBucket.Endpoint != nil {
			customBucketData["endpoint"] = *in.CustomBucket.Endpoint
		}
		if in.CustomBucket.AccessKeyID != nil {
			customBucketData["access_key_id"] = *in.CustomBucket.AccessKeyID
		}
		// Note: secret_access_key is never returned from API, so we preserve the existing value
		if existingSecret != nil {
			customBucketData["secret_access_key"] = existingSecret
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

func errorsApplicationUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in errorsApplication
	for _, e := range errorsApplicationRef(&in) {
		if e.k == "github_repository_name" || e.k == "gitlab_repository_name" {
			// Always evaluate: stringFromResourceData returns nil when the field is null in
			// config (UI-managed, omit from PATCH) and a pointer otherwise - including for ""
			// (explicit disconnect). Can't gate on d.HasChange because Computed: true masks
			// the change when switching to "".
			ptr := stringFromResourceData(d, e.k)
			if e.k == "github_repository_name" {
				in.GithubRepositoryName = ptr
			} else {
				in.GitlabRepositoryName = ptr
			}
			// Mirror the config value into state directly. The SDK's auto-propagation of the
			// planned new value into state relies on d.HasChange, which is unreliable here for
			// the same reason - without this, state would stay on the previous value.
			if ptr != nil {
				if err := d.Set(e.k, *ptr); err != nil {
					return diag.FromErr(err)
				}
			}
			continue
		}
		if d.HasChange(e.k) {
			if e.k == "team_id" {
				in.TeamId = StringOrIntFromResourceData(d, e.k)
			} else if e.k == "application_group_id" {
				// Use intFromResourceData to properly distinguish null vs 0
				in.ApplicationGroupID = intFromResourceData(d, e.k)
			} else {
				load(d, e.k, e.v)
			}
		}
	}
	return resourceUpdateWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), fmt.Sprintf("/api/v1/applications/%s", url.PathEscape(d.Id())), &in)
}

func errorsApplicationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDeleteWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), fmt.Sprintf("/api/v1/applications/%s", url.PathEscape(d.Id())))
}

// customizeDiffRepositoryName forces the planned value of a repository-name attribute (e.g.
// github_repository_name or gitlab_repository_name) to match whatever is in the config. Without
// this, TypeString + Optional + Computed makes Terraform treat an explicit "" the same as unset,
// so switching a previously-set value to "" produces no diff.
func customizeDiffRepositoryName(key string) schema.CustomizeDiffFunc {
	return func(ctx context.Context, diff *schema.ResourceDiff, v interface{}) error {
		rawConfig := diff.GetRawConfig()
		if rawConfig.IsNull() || !rawConfig.IsKnown() {
			return nil
		}
		val := rawConfig.GetAttr(key)
		if val.IsNull() || !val.IsKnown() {
			return nil
		}
		return diff.SetNew(key, val.AsString())
	}
}

func validateErrorsApplication(ctx context.Context, diff *schema.ResourceDiff, v interface{}) error {
	if diff.Id() != "" && diff.HasChange("correlate_with_source_id") {
		return fmt.Errorf("correlate_with_source_id cannot be changed after the application is created")
	}

	if diff.Id() != "" && diff.HasChange("data_region") {
		return fmt.Errorf("data_region cannot be changed after application is created")
	}

	if err := validateCustomBucketChange(ctx, diff, v); err != nil {
		return err
	}

	return nil
}

func newErrorsApplicationDataSource() *schema.Resource {
	s := make(map[string]*schema.Schema)
	for k, v := range errorsApplicationSchema {
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
			// ConflictsWith is invalid on computed-only attributes (data source fields), so drop it.
			cp.ConflictsWith = nil
		}
		s[k] = &cp
	}
	return &schema.Resource{
		ReadContext: errorsApplicationLookup,
		Description: "This Data Source allows you to look up existing Errors applications using their name. For more information about the Errors API check https://betterstack.com/docs/errors/api/applications/list/",
		Schema:      s,
	}
}

type errorsApplicationPageHTTPResponse struct {
	Data []struct {
		ID         string            `json:"id"`
		Attributes errorsApplication `json:"attributes"`
	} `json:"data"`
	Pagination struct {
		First string `json:"first"`
		Last  string `json:"last"`
		Prev  string `json:"prev"`
		Next  string `json:"next"`
	} `json:"pagination"`
}

func errorsApplicationLookup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	fetch := func(page int) (*errorsApplicationPageHTTPResponse, error) {
		res, err := meta.(*client).GetWithBaseURL(ctx, meta.(*client).ErrorsBaseURL(), fmt.Sprintf("/api/v1/applications?page=%d", page))
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
		var tr errorsApplicationPageHTTPResponse
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
				d.SetId(e.ID)
				return errorsApplicationCopyAttrs(d, &e.Attributes)
			}
		}
		page++
		if res.Pagination.Next == "" {
			return nil
		}
	}
}
