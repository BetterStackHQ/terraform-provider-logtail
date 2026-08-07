package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var sourceGCPProjectSchema = map[string]*schema.Schema{
	"source_id": {
		Description: "The ID of the `logtail_source` (with `platform = \"gcp\"`) to link the GCP project to. Changing this forces a new resource, re-running the project connect against the new source.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"gcp_account_id": {
		Description:  "The ID of an already-connected GCP project to link this source to. Provide this instead of `gcp_project_id`/`gcp_project_number` to reuse a project you've already connected. Write-only: the API does not return it, so it isn't refreshed from state.",
		Type:         schema.TypeString,
		Optional:     true,
		AtLeastOneOf: []string{"gcp_account_id", "gcp_project_id"},
	},
	"gcp_project_id": {
		Description:  "The GCP project ID to connect - the `project_id` output of the Better Stack GCP Terraform module (or the value you passed to the setup script). Provide together with `gcp_project_number`. Write-only: the API does not return it, so it isn't refreshed from state.",
		Type:         schema.TypeString,
		Optional:     true,
		RequiredWith: []string{"gcp_project_number"},
	},
	"gcp_project_number": {
		Description:  "The GCP project number - the `project_number` output of the Better Stack GCP Terraform module. Provide together with `gcp_project_id`. Write-only: the API does not return it, so it isn't refreshed from state.",
		Type:         schema.TypeString,
		Optional:     true,
		RequiredWith: []string{"gcp_project_id"},
	},
}

func newSourceGCPProjectResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: sourceGCPProjectCreate,
		ReadContext:   sourceGCPProjectRead,
		UpdateContext: sourceGCPProjectUpdate,
		DeleteContext: sourceGCPProjectDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "Links a GCP project to a `gcp` platform `logtail_source` by pasting back the " +
			"project ID / project number after Workload Identity Federation has been configured by the " +
			"Better Stack GCP Terraform module or setup script (or by reusing an already-connected project).\n\n" +
			"The credentials are write-only - the API never returns them, so they aren't refreshed into " +
			"Terraform state. Destroying this resource only removes it from state; the GCP project stays " +
			"linked to the source until the source itself is destroyed.",
		Schema: sourceGCPProjectSchema,
	}
}

// sourceGCPProjectPayload is the PATCH body for the GCP project linkage. These are the
// write-only params the public Sources API consumes to run GcpIntegration::SourceManager;
// they are never returned as source attributes, so they live only on this request struct.
type sourceGCPProjectPayload struct {
	GcpAccountID     *string `json:"gcp_account_id,omitempty"`
	GcpProjectID     *string `json:"gcp_project_id,omitempty"`
	GcpProjectNumber *string `json:"gcp_project_number,omitempty"`
}

// patchSourceGCPProject PATCHes the configured GCP credentials onto the source. The full
// configured set is always sent (not just changed keys) because the connect manager needs
// the project ID / project number together - a partial PATCH would be rejected server-side.
func patchSourceGCPProject(ctx context.Context, d *schema.ResourceData, meta interface{}, sourceID string) diag.Diagnostics {
	in := sourceGCPProjectPayload{
		GcpAccountID:     stringFromResourceData(d, "gcp_account_id"),
		GcpProjectID:     stringFromResourceData(d, "gcp_project_id"),
		GcpProjectNumber: stringFromResourceData(d, "gcp_project_number"),
	}
	return resourceUpdate(ctx, meta, fmt.Sprintf("/api/v1/sources/%s", url.PathEscape(sourceID)), &in)
}

func sourceGCPProjectCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	sourceID := d.Get("source_id").(string)
	if derr := patchSourceGCPProject(ctx, d, meta, sourceID); derr != nil {
		return derr
	}
	d.SetId(sourceID)
	return sourceGCPProjectRead(ctx, d, meta)
}

func sourceGCPProjectRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out sourceHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v1/sources/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Source gone -> linkage gone.
		return nil
	}
	// The credentials are write-only (never returned by the API), so there's nothing to
	// refresh - we only confirm the underlying source still exists and keep source_id in sync.
	return diag.FromErr(d.Set("source_id", d.Id()))
}

func sourceGCPProjectUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if derr := patchSourceGCPProject(ctx, d, meta, d.Get("source_id").(string)); derr != nil {
		return derr
	}
	return sourceGCPProjectRead(ctx, d, meta)
}

func sourceGCPProjectDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// The Sources API has no disconnect endpoint: PATCHing blank GCP params is a server-side
	// no-op (the connect manager never unlinks a project). Deleting this resource therefore
	// only drops it from Terraform state; the GCP project stays linked to the source until the
	// source itself is destroyed.
	d.SetId("")
	return nil
}
