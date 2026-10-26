// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/linyiru/terraform-provider-gdrive/internal/drive"
	api "google.golang.org/api/drive/v3"
)

var rootID = path.Root("id")

type sharedDriveResource struct{ resourceBase }
type sharedDriveModel struct {
	ID                       types.String `tfsdk:"id"`
	Name                     types.String `tfsdk:"name"`
	Protection               types.Bool   `tfsdk:"deletion_protection"`
	AdminManaged             types.Bool   `tfsdk:"admin_managed_restrictions"`
	DomainOnly               types.Bool   `tfsdk:"domain_users_only"`
	MembersOnly              types.Bool   `tfsdk:"drive_members_only"`
	CopyRequiresWriter       types.Bool   `tfsdk:"copy_requires_writer_permission"`
	SharingRequiresOrganizer types.Bool   `tfsdk:"sharing_folders_requires_organizer_permission"`
}

func NewSharedDriveResource() resource.Resource { return &sharedDriveResource{} }
func (r *sharedDriveResource) Metadata(_ context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	res.TypeName = req.ProviderTypeName + "_shared_drive"
}
func (r *sharedDriveResource) Schema(_ context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	a := map[string]schema.Attribute{
		"id": computedID(), "name": schema.StringAttribute{Required: true, Description: "Shared Drive name.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}}, "deletion_protection": protection(),
	}
	for name, description := range map[string]string{
		"admin_managed_restrictions": "Require administrative privileges to modify restrictions.", "domain_users_only": "Restrict access to domain users.", "drive_members_only": "Restrict item access to drive members.", "copy_requires_writer_permission": "Disable copy, print, and download for readers and commenters.", "sharing_folders_requires_organizer_permission": "Require Manager role to share folders.",
	} {
		a[name] = schema.BoolAttribute{Optional: true, Computed: true, Description: description + " Omit to preserve the server setting."}
	}
	res.Schema = schema.Schema{Description: "A Shared Drive. Destruction only supports empty drives and requires deletion_protection=false.", Attributes: a}
}
func (m sharedDriveModel) body() *api.Drive {
	restrictions := &api.DriveRestrictions{}
	fields := []struct {
		name   string
		value  types.Bool
		target *bool
	}{
		{"AdminManagedRestrictions", m.AdminManaged, &restrictions.AdminManagedRestrictions},
		{"DomainUsersOnly", m.DomainOnly, &restrictions.DomainUsersOnly},
		{"DriveMembersOnly", m.MembersOnly, &restrictions.DriveMembersOnly},
		{"CopyRequiresWriterPermission", m.CopyRequiresWriter, &restrictions.CopyRequiresWriterPermission},
		{"SharingFoldersRequiresOrganizerPermission", m.SharingRequiresOrganizer, &restrictions.SharingFoldersRequiresOrganizerPermission},
	}
	for _, f := range fields {
		if !f.value.IsNull() && !f.value.IsUnknown() {
			*f.target = f.value.ValueBool()
			restrictions.ForceSendFields = append(restrictions.ForceSendFields, f.name)
		}
	}
	v := &api.Drive{Name: m.Name.ValueString()}
	if len(restrictions.ForceSendFields) > 0 {
		v.Restrictions = restrictions
	}
	return v
}
func (m *sharedDriveModel) set(v *api.Drive) {
	m.ID = types.StringValue(v.Id)
	m.Name = types.StringValue(v.Name)
	restrictions := v.Restrictions
	if restrictions == nil {
		restrictions = &api.DriveRestrictions{}
	}
	m.AdminManaged = types.BoolValue(restrictions.AdminManagedRestrictions)
	m.DomainOnly = types.BoolValue(restrictions.DomainUsersOnly)
	m.MembersOnly = types.BoolValue(restrictions.DriveMembersOnly)
	m.CopyRequiresWriter = types.BoolValue(restrictions.CopyRequiresWriterPermission)
	m.SharingRequiresOrganizer = types.BoolValue(restrictions.SharingFoldersRequiresOrganizerPermission)
	if m.Protection.IsNull() || m.Protection.IsUnknown() {
		m.Protection = types.BoolValue(true)
	}
}
func (r *sharedDriveResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	var m sharedDriveModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if res.Diagnostics.HasError() {
		return
	}
	// One requestId for all retries in this Create operation.
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		apiError(&res.Diagnostics, "request ID generation", err)
		return
	}
	v, err := r.client.API.CreateDrive(ctx, hex.EncodeToString(token[:]), m.body())
	if err != nil {
		apiError(&res.Diagnostics, "create Shared Drive", err)
		return
	}
	m.set(v)
	res.Diagnostics.Append(res.State.Set(ctx, &m)...)
}
func (r *sharedDriveResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	var m sharedDriveModel
	res.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if res.Diagnostics.HasError() {
		return
	}
	v, err := r.client.API.GetDrive(ctx, m.ID.ValueString(), r.client.Admin)
	if drive.IsNotFound(err) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&res.Diagnostics, "read Shared Drive", err)
		return
	}
	m.set(v)
	res.Diagnostics.Append(res.State.Set(ctx, &m)...)
}
func (r *sharedDriveResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	var m sharedDriveModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if res.Diagnostics.HasError() {
		return
	}
	v, err := r.client.API.UpdateDrive(ctx, m.ID.ValueString(), m.body(), r.client.Admin)
	if err != nil {
		apiError(&res.Diagnostics, "update Shared Drive", err)
		return
	}
	m.set(v)
	res.Diagnostics.Append(res.State.Set(ctx, &m)...)
}
func (r *sharedDriveResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	var m sharedDriveModel
	res.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if res.Diagnostics.HasError() {
		return
	}
	if m.Protection.ValueBool() || m.Protection.IsNull() {
		res.Diagnostics.AddError("Deletion protected", "Apply deletion_protection=false before destroying the Shared Drive.")
		return
	}
	// Never set allowItemDeletion: recursive permanent deletion is not supported.
	err := r.client.API.DeleteDrive(ctx, m.ID.ValueString(), r.client.Admin)
	if err != nil && !drive.IsNotFound(err) {
		apiError(&res.Diagnostics, "delete Shared Drive", err)
	}
}
func (r *sharedDriveResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	importID(ctx, req, res)
}
