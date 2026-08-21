package provider

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var errorsReleaseSchema = map[string]*schema.Schema{
	"id": {
		Description: "The ID of this release.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"application_id": {
		Description: "The ID of the Errors application this release belongs to. The API resolves the value to the canonical application ID, which may differ from the one you provide when the application has sibling sources - state always keeps the canonical ID.",
		Type:        schema.TypeInt,
		Required:    true,
		ForceNew:    true,
	},
	"version": {
		Description: "Release version or commit ref, e.g. `v1.2.3` or a commit SHA.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"environments": {
		Description: "Environments the release was deployed to, e.g. `[\"production\"]`. The server keeps merging in environments the release is later seen in, so state always keeps the configured value.",
		Type:        schema.TypeList,
		Optional:    true,
		ForceNew:    true,
		Elem: &schema.Schema{
			Type: schema.TypeString,
		},
	},
	"first_seen_at": {
		Description: "The time when this release was first seen.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"origin": {
		Description: "How this release was detected, e.g. `reported` for releases registered via the API.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
}

func newErrorsReleaseResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: errorsReleaseCreate,
		ReadContext:   errorsReleaseRead,
		DeleteContext: errorsReleaseDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "This resource allows you to register and remove releases of your Errors applications. Creating a release registers it the moment it deploys, before its first error arrives. Destroying a release is a hard delete: a release that is still receiving error events is re-detected automatically by the platform (about a minute after its next event).",
		Schema:      errorsReleaseSchema,
	}
}

type errorsRelease struct {
	Version       *string `json:"version,omitempty"`
	ApplicationID *int    `json:"application_id,omitempty"`
	FirstSeenAt   *string `json:"first_seen_at,omitempty"`
	Origin        *string `json:"origin,omitempty"`
}

type errorsReleaseHTTPResponse struct {
	Data struct {
		ID         string        `json:"id"`
		Attributes errorsRelease `json:"attributes"`
	} `json:"data"`
}

type errorsReleaseCreateHTTPRequest struct {
	Version      string    `json:"version"`
	Applications []int     `json:"applications"`
	Environment  *[]string `json:"environment,omitempty"`
}

type errorsReleaseCreateHTTPResponse struct {
	Releases []struct {
		ID            int `json:"id"`
		ApplicationID int `json:"application_id"`
	} `json:"releases"`
}

// environments is deliberately missing here: the server keeps merging in environments the
// release is later seen in, so reading it back would cause perpetual diffs - state keeps
// the configured value instead.
func errorsReleaseRef(in *errorsRelease) []struct {
	k string
	v interface{}
} {
	return []struct {
		k string
		v interface{}
	}{
		{k: "version", v: &in.Version},
		{k: "application_id", v: &in.ApplicationID},
		{k: "first_seen_at", v: &in.FirstSeenAt},
		{k: "origin", v: &in.Origin},
	}
}

func errorsReleaseCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	in := errorsReleaseCreateHTTPRequest{
		Version:      d.Get("version").(string),
		Applications: []int{d.Get("application_id").(int)},
	}
	load(d, "environments", &in.Environment)

	var out errorsReleaseCreateHTTPResponse
	if err := resourceCreateWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), "/api/v1/releases", &in, &out); err != nil {
		return err
	}
	if len(out.Releases) != 1 {
		return diag.Errorf("POST /api/v1/releases: expected exactly 1 release in the response, got %d", len(out.Releases))
	}
	d.SetId(strconv.Itoa(out.Releases[0].ID))
	// The create response carries only the release ID and the canonical application ID;
	// read the release back to populate the computed attributes.
	return errorsReleaseRead(ctx, d, meta)
}

func errorsReleaseRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out errorsReleaseHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), fmt.Sprintf("/api/v1/releases/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Force "create" on 404.
		return nil
	}
	return errorsReleaseCopyAttrs(d, &out.Data.Attributes)
}

func errorsReleaseCopyAttrs(d *schema.ResourceData, in *errorsRelease) diag.Diagnostics {
	var derr diag.Diagnostics
	for _, e := range errorsReleaseRef(in) {
		if err := d.Set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface()); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	return derr
}

func errorsReleaseDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDeleteWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), fmt.Sprintf("/api/v1/releases/%s", url.PathEscape(d.Id())))
}
