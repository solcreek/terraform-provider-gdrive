// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/solcreek/terraform-provider-gdrive/internal/drive"
	api "google.golang.org/api/drive/v3"
)

type permissionResource struct{ resourceBase }
type permissionModel struct {
	ID           types.String `tfsdk:"id"`
	FileID       types.String `tfsdk:"file_id"`
	PermissionID types.String `tfsdk:"permission_id"`
	Type         types.String `tfsdk:"type"`
	Role         types.String `tfsdk:"role"`
	Email        types.String `tfsdk:"email_address"`
	Domain       types.String `tfsdk:"domain"`
	Discovery    types.Bool   `tfsdk:"allow_file_discovery"`
	Notify       types.Bool   `tfsdk:"send_notification_email"`
}

func NewPermissionResource() resource.Resource { return &permissionResource{} }
func (r *permissionResource) Metadata(_ context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	res.TypeName = req.ProviderTypeName + "_permission"
}
func (r *permissionResource) Schema(_ context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	res.Schema = schema.Schema{Description: "One direct permission. Existing grants must be imported. Inherited grants cannot be managed here; unrelated ACL entries are never replaced.", Attributes: map[string]schema.Attribute{
		"id": computedID(), "permission_id": computedID(),
		"file_id":                 schema.StringAttribute{Required: true, Description: "File, folder, or Shared Drive ID.", Validators: idValidators(), PlanModifiers: replace},
		"type":                    schema.StringAttribute{Required: true, Description: "Principal type.", Validators: []validator.String{stringvalidator.OneOf("user", "group", "domain", "anyone")}, PlanModifiers: replace},
		"role":                    schema.StringAttribute{Required: true, Description: "Drive API role. Ownership transfer is not supported.", Validators: []validator.String{stringvalidator.OneOf("organizer", "fileOrganizer", "writer", "commenter", "reader")}},
		"email_address":           schema.StringAttribute{Optional: true, Description: "Canonical lowercase user or group email.", Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[^A-Z@\s]+@[^A-Z@\s]+$`), "must be a lowercase email address")}, PlanModifiers: replace},
		"domain":                  schema.StringAttribute{Optional: true, Description: "Lowercase domain for a domain grant.", Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`), "must be a lowercase domain")}, PlanModifiers: replace},
		"allow_file_discovery":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Allow discovery for domain or anyone grants."},
		"send_notification_email": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Send an email on creation of a user or group grant. Changing this does not resend an email."},
	}}
}
func (r *permissionResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, res *resource.ValidateConfigResponse) {
	var m permissionModel
	res.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if res.Diagnostics.HasError() || m.Type.IsUnknown() || m.Type.IsNull() {
		return
	}
	user := m.Type.ValueString() == "user" || m.Type.ValueString() == "group"
	if user && m.Email.IsNull() {
		res.Diagnostics.AddAttributeError(path.Root("email_address"), "Missing principal", "User and group permissions require email_address.")
	}
	if !user && !m.Email.IsNull() {
		res.Diagnostics.AddAttributeError(path.Root("email_address"), "Unexpected principal", "Only user and group permissions accept email_address.")
	}
	if m.Type.ValueString() == "domain" && m.Domain.IsNull() {
		res.Diagnostics.AddAttributeError(path.Root("domain"), "Missing domain", "Domain permissions require domain.")
	}
	if m.Type.ValueString() != "domain" && !m.Domain.IsNull() {
		res.Diagnostics.AddAttributeError(path.Root("domain"), "Unexpected domain", "Only domain permissions accept domain.")
	}
	if user && m.Discovery.ValueBool() {
		res.Diagnostics.AddAttributeError(path.Root("allow_file_discovery"), "Invalid discovery setting", "Discovery applies only to domain and anyone grants.")
	}
	if !user && m.Notify.ValueBool() {
		res.Diagnostics.AddAttributeError(path.Root("send_notification_email"), "Invalid notification setting", "Notifications apply only to user and group grants.")
	}
}
func (m permissionModel) body(create bool) *api.Permission {
	v := &api.Permission{Role: m.Role.ValueString()}
	if create {
		v.Type = m.Type.ValueString()
		v.EmailAddress = m.Email.ValueString()
		v.Domain = m.Domain.ValueString()
	}
	if m.Type.ValueString() == "domain" || m.Type.ValueString() == "anyone" {
		v.AllowFileDiscovery = m.Discovery.ValueBool()
		v.ForceSendFields = []string{"AllowFileDiscovery"}
	}
	return v
}
func (m permissionModel) matches(p *api.Permission) bool {
	if p.Deleted || m.Type.ValueString() != p.Type {
		return false
	}
	switch p.Type {
	case "user", "group":
		return strings.EqualFold(m.Email.ValueString(), p.EmailAddress)
	case "domain":
		return strings.EqualFold(m.Domain.ValueString(), p.Domain)
	case "anyone":
		return true
	}
	return false
}
func (m *permissionModel) set(p *api.Permission) bool {
	role, direct := drive.DirectRole(p)
	if !direct || p.Deleted {
		return false
	}
	m.PermissionID = types.StringValue(p.Id)
	m.ID = types.StringValue(m.FileID.ValueString() + "/" + p.Id)
	m.Type = types.StringValue(p.Type)
	m.Role = types.StringValue(role)
	if p.Type == "user" || p.Type == "group" {
		m.Email = types.StringValue(strings.ToLower(p.EmailAddress))
	} else {
		m.Email = types.StringNull()
	}
	if p.Type == "domain" {
		m.Domain = types.StringValue(strings.ToLower(p.Domain))
	} else {
		m.Domain = types.StringNull()
	}
	m.Discovery = types.BoolValue(p.AllowFileDiscovery)
	if m.Notify.IsNull() || m.Notify.IsUnknown() {
		m.Notify = types.BoolValue(false)
	}
	return true
}
func (r *permissionResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	var m permissionModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if res.Diagnostics.HasError() {
		return
	}
	r.client.API.PermissionMu.Lock()
	defer r.client.API.PermissionMu.Unlock()
	existing, err := r.client.API.Permissions(ctx, m.FileID.ValueString(), r.client.Admin)
	if err != nil {
		apiError(&res.Diagnostics, "list existing permissions", err)
		return
	}
	for _, p := range existing {
		if _, direct := drive.DirectRole(p); direct && m.matches(p) {
			res.Diagnostics.AddError("Permission already exists", "Import the existing direct permission using file_id/permission_id before managing it.")
			return
		}
	}
	p, err := r.client.API.CreatePermission(ctx, m.FileID.ValueString(), m.body(true), r.client.Admin, m.Notify.ValueBool())
	if err != nil {
		apiError(&res.Diagnostics, "create permission", err)
		res.Diagnostics.AddWarning("Check for an interrupted creation", "Permission creation is not automatically retried. Inspect the ACL and import any created grant before applying again.")
		return
	}
	if !m.set(p) {
		res.Diagnostics.AddError("Unexpected inherited permission", "Google did not return a direct grant. Inspect permissions on the originating parent.")
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &m)...)
}
func (r *permissionResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	var m permissionModel
	res.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if res.Diagnostics.HasError() {
		return
	}
	p, err := r.client.API.GetPermission(ctx, m.FileID.ValueString(), m.PermissionID.ValueString(), r.client.Admin)
	if drive.IsNotFound(err) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&res.Diagnostics, "read permission", err)
		return
	}
	if !m.set(p) {
		res.State.RemoveResource(ctx)
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &m)...)
}
func (r *permissionResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	var m permissionModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if res.Diagnostics.HasError() {
		return
	}
	r.client.API.PermissionMu.Lock()
	defer r.client.API.PermissionMu.Unlock()
	current, err := r.client.API.GetPermission(ctx, m.FileID.ValueString(), m.PermissionID.ValueString(), r.client.Admin)
	if err != nil {
		apiError(&res.Diagnostics, "read permission before update", err)
		return
	}
	if _, direct := drive.DirectRole(current); !direct {
		res.Diagnostics.AddError("Inherited permission", "Modify the permission on its originating parent instead.")
		return
	}
	p, err := r.client.API.UpdatePermission(ctx, m.FileID.ValueString(), m.PermissionID.ValueString(), m.body(false), r.client.Admin)
	if err != nil {
		apiError(&res.Diagnostics, "update permission", err)
		return
	}
	if !m.set(p) {
		res.Diagnostics.AddError("Direct permission missing", "Google did not return the updated direct grant.")
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &m)...)
}
func (r *permissionResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	var m permissionModel
	res.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if res.Diagnostics.HasError() {
		return
	}
	r.client.API.PermissionMu.Lock()
	defer r.client.API.PermissionMu.Unlock()
	p, err := r.client.API.GetPermission(ctx, m.FileID.ValueString(), m.PermissionID.ValueString(), r.client.Admin)
	if drive.IsNotFound(err) {
		return
	}
	if err != nil {
		apiError(&res.Diagnostics, "read permission before delete", err)
		return
	}
	if _, direct := drive.DirectRole(p); !direct || p.Deleted {
		return
	}
	err = r.client.API.DeletePermission(ctx, m.FileID.ValueString(), m.PermissionID.ValueString(), r.client.Admin)
	if err != nil && !drive.IsNotFound(err) {
		apiError(&res.Diagnostics, "delete permission", err)
	}
}
func (r *permissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || !idPattern.MatchString(parts[0]) || !idPattern.MatchString(parts[1]) {
		res.Diagnostics.AddError("Invalid import ID", "Use file_id/permission_id.")
		return
	}
	res.Diagnostics.Append(res.State.SetAttribute(ctx, rootID, req.ID)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root("file_id"), parts[0])...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root("permission_id"), parts[1])...)
}
