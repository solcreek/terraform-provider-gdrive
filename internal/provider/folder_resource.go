// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/linyiru/terraform-provider-gdrive/internal/drive"
	api "google.golang.org/api/drive/v3"
)

type folderResource struct{ resourceBase }
type folderModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Parent      types.String `tfsdk:"parent_id"`
	DriveID     types.String `tfsdk:"drive_id"`
	Description types.String `tfsdk:"description"`
	Protection  types.Bool   `tfsdk:"deletion_protection"`
}

func NewFolderResource() resource.Resource { return &folderResource{} }
func (r *folderResource) Metadata(_ context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	res.TypeName = req.ProviderTypeName + "_folder"
}
func (r *folderResource) Schema(_ context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	res.Schema = schema.Schema{Description: "A folder inside a Shared Drive. Destroy moves it and its contents to trash; permanent deletion is not supported.", Attributes: map[string]schema.Attribute{
		"id": computedID(), "name": schema.StringAttribute{Required: true, Description: "Folder name.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
		"parent_id":           schema.StringAttribute{Required: true, Description: "Parent folder or Shared Drive root ID. Moves across drives are not supported.", Validators: idValidators()},
		"drive_id":            schema.StringAttribute{Computed: true, Description: "Shared Drive containing the folder."},
		"description":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Folder description. An empty string clears it."},
		"deletion_protection": protection(),
	}}
}
func (m *folderModel) set(v *api.File) error {
	if v.MimeType != drive.FolderMIME || v.DriveId == "" || v.Id == v.DriveId {
		return fmt.Errorf("resource must be a folder inside a Shared Drive; use gdrive_shared_drive for its root")
	}
	if len(v.Parents) != 1 {
		return fmt.Errorf("folder response must contain exactly one parent; verify content access")
	}
	m.ID = types.StringValue(v.Id)
	m.Name = types.StringValue(v.Name)
	m.Parent = types.StringValue(v.Parents[0])
	m.DriveID = types.StringValue(v.DriveId)
	m.Description = types.StringValue(v.Description)
	if m.Protection.IsNull() || m.Protection.IsUnknown() {
		m.Protection = types.BoolValue(true)
	}
	return nil
}
func (r *folderResource) parent(ctx context.Context, id string) (*api.File, error) {
	p, err := r.client.API.GetFile(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.MimeType != drive.FolderMIME || p.DriveId == "" || p.Trashed {
		return nil, fmt.Errorf("parent must be an accessible, active Shared Drive folder or root")
	}
	return p, nil
}
func (r *folderResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	var m folderModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if res.Diagnostics.HasError() {
		return
	}
	if _, err := r.parent(ctx, m.Parent.ValueString()); err != nil {
		apiError(&res.Diagnostics, "validate parent", err)
		return
	}
	v, err := r.client.API.CreateFolder(ctx, &api.File{Name: m.Name.ValueString(), Description: m.Description.ValueString(), MimeType: drive.FolderMIME, Parents: []string{m.Parent.ValueString()}})
	if err != nil {
		apiError(&res.Diagnostics, "create folder", err)
		res.Diagnostics.AddWarning("Check for an interrupted creation", "Folder creation is not automatically retried. If Google created the folder before the response failed, import its ID before applying again.")
		return
	}
	// Save the returned ID even if validation of the rest of the response fails.
	m.ID = types.StringValue(v.Id)
	if err := m.set(v); err != nil {
		res.Diagnostics.Append(res.State.SetAttribute(ctx, rootID, v.Id)...)
		apiError(&res.Diagnostics, "read created folder", err)
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &m)...)
}
func (r *folderResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	var m folderModel
	res.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if res.Diagnostics.HasError() {
		return
	}
	v, err := r.client.API.GetFile(ctx, m.ID.ValueString())
	if drive.IsNotFound(err) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&res.Diagnostics, "read folder", err)
		return
	}
	if v.Trashed {
		res.State.RemoveResource(ctx)
		return
	}
	if err := m.set(v); err != nil {
		apiError(&res.Diagnostics, "read folder", err)
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &m)...)
}
func (r *folderResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	var m, old folderModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	res.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if res.Diagnostics.HasError() {
		return
	}
	add, remove := "", ""
	if !m.Parent.Equal(old.Parent) {
		p, err := r.parent(ctx, m.Parent.ValueString())
		if err != nil {
			apiError(&res.Diagnostics, "validate new parent", err)
			return
		}
		if p.DriveId != old.DriveID.ValueString() {
			res.Diagnostics.AddError("Cross-drive move unsupported", "The new parent must belong to the same Shared Drive.")
			return
		}
		add = m.Parent.ValueString()
		remove = old.Parent.ValueString()
	}
	v, err := r.client.API.UpdateFolder(ctx, m.ID.ValueString(), &api.File{Name: m.Name.ValueString(), Description: m.Description.ValueString(), ForceSendFields: []string{"Description"}}, add, remove)
	if err != nil {
		apiError(&res.Diagnostics, "update folder", err)
		return
	}
	if err := m.set(v); err != nil {
		apiError(&res.Diagnostics, "read updated folder", err)
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &m)...)
}
func (r *folderResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	var m folderModel
	res.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if res.Diagnostics.HasError() {
		return
	}
	if m.Protection.IsNull() || m.Protection.ValueBool() {
		res.Diagnostics.AddError("Deletion protected", "Apply deletion_protection=false before destroying the folder. Destroy moves the folder and its contents to trash.")
		return
	}
	_, err := r.client.API.UpdateFolder(ctx, m.ID.ValueString(), &api.File{Trashed: true}, "", "")
	if err != nil && !drive.IsNotFound(err) {
		apiError(&res.Diagnostics, "trash folder", err)
	}
}
func (r *folderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	importID(ctx, req, res)
}
