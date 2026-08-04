package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var sourceAzureAccountSchema = map[string]*schema.Schema{
	"source_id": {
		Description: "The ID of the `logtail_source` (with `platform = \"azure\"`) to link the Azure tenant to. Changing this forces a new resource, re-running the account connect against the new source.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"azure_account_id": {
		Description: "The ID of a connected Azure tenant to link this source to. Unlike AWS and GCP there is no credential paste-back: connecting a new tenant requires the interactive Microsoft admin consent in the Better Stack UI (done once per tenant), after which this resource links it to any number of sources. Write-only: the API does not return it, so it isn't refreshed from state.",
		Type:        schema.TypeString,
		Required:    true,
	},
}

func newSourceAzureAccountResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: sourceAzureAccountCreate,
		ReadContext:   sourceAzureAccountRead,
		UpdateContext: sourceAzureAccountUpdate,
		DeleteContext: sourceAzureAccountDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "Links a connected Azure tenant to an `azure` platform `logtail_source`.\n\n" +
			"Unlike AWS and GCP there is no credential paste-back: new tenants are connected via the " +
			"interactive Microsoft admin consent in the Better Stack UI (once per tenant); this resource " +
			"then links the connected tenant to sources by its account ID.\n\n" +
			"The account ID is write-only - the API never returns it, so it isn't refreshed into " +
			"Terraform state. Destroying this resource only removes it from state; the Azure tenant stays " +
			"linked to the source until the source itself is destroyed.",
		Schema: sourceAzureAccountSchema,
	}
}

// sourceAzureAccountPayload is the PATCH body for the Azure tenant linkage. This is the
// write-only param the public Sources API consumes to run AzureIntegration::SourceManager;
// it is never returned as a source attribute, so it lives only on this request struct.
type sourceAzureAccountPayload struct {
	AzureAccountID *string `json:"azure_account_id,omitempty"`
}

func patchSourceAzureAccount(ctx context.Context, d *schema.ResourceData, meta interface{}, sourceID string) diag.Diagnostics {
	in := sourceAzureAccountPayload{
		AzureAccountID: stringFromResourceData(d, "azure_account_id"),
	}
	return resourceUpdate(ctx, meta, fmt.Sprintf("/api/v1/sources/%s", url.PathEscape(sourceID)), &in)
}

func sourceAzureAccountCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	sourceID := d.Get("source_id").(string)
	if derr := patchSourceAzureAccount(ctx, d, meta, sourceID); derr != nil {
		return derr
	}
	d.SetId(sourceID)
	return sourceAzureAccountRead(ctx, d, meta)
}

func sourceAzureAccountRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out sourceHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v1/sources/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Source gone -> linkage gone.
		return nil
	}
	// The account ID is write-only (never returned by the API), so there's nothing to
	// refresh - we only confirm the underlying source still exists and keep source_id in sync.
	return diag.FromErr(d.Set("source_id", d.Id()))
}

func sourceAzureAccountUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if derr := patchSourceAzureAccount(ctx, d, meta, d.Get("source_id").(string)); derr != nil {
		return derr
	}
	return sourceAzureAccountRead(ctx, d, meta)
}

func sourceAzureAccountDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// The Sources API has no disconnect endpoint: PATCHing a blank Azure param is a server-side
	// no-op (the connect manager never unlinks an account). Deleting this resource therefore
	// only drops it from Terraform state; the Azure tenant stays linked to the source until the
	// source itself is destroyed.
	d.SetId("")
	return nil
}
