// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type resourceBase struct{ client *configuredClient }

func (b *resourceBase) Configure(_ context.Context, req resource.ConfigureRequest, r *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*configuredClient)
	if !ok {
		r.Diagnostics.AddError("Unexpected provider configuration", "Expected a Google Drive client.")
		return
	}
	b.client = c
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func idValidators() []validator.String {
	return []validator.String{stringvalidator.RegexMatches(idPattern, "must be a nonempty Google Drive ID, not a URL or path")}
}
func computedID() schema.StringAttribute {
	return schema.StringAttribute{Computed: true, Description: "Google Drive resource ID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}
}
func protection() schema.BoolAttribute {
	return schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Blocks deletion. Apply false in a separate operation before destroying this resource."}
}
func apiError(d *diag.Diagnostics, op string, err error) {
	d.AddError("Google Drive "+op+" failed", err.Error())
}
func importID(ctx context.Context, req resource.ImportStateRequest, r *resource.ImportStateResponse) {
	if !idPattern.MatchString(req.ID) {
		r.Diagnostics.AddError("Invalid import ID", "Provide a Google Drive ID, not a URL.")
		return
	}
	resource.ImportStatePassthroughID(ctx, rootID, req, r)
}
