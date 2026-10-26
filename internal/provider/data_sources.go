// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"cmp"
	"context"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/solcreek/terraform-provider-gdrive/internal/drive"
	api "google.golang.org/api/drive/v3"
)

type dataSourceBase struct{ client *configuredClient }

func (b *dataSourceBase) Configure(_ context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*configuredClient)
	if !ok {
		res.Diagnostics.AddError("Unexpected provider configuration", "Expected a Google Drive client.")
		return
	}
	b.client = c
}

type sharedDriveDataSource struct{ dataSourceBase }
type sharedDriveDataModel struct {
	ID                       types.String `tfsdk:"id"`
	Name                     types.String `tfsdk:"name"`
	AdminManaged             types.Bool   `tfsdk:"admin_managed_restrictions"`
	DomainOnly               types.Bool   `tfsdk:"domain_users_only"`
	MembersOnly              types.Bool   `tfsdk:"drive_members_only"`
	CopyRequiresWriter       types.Bool   `tfsdk:"copy_requires_writer_permission"`
	SharingRequiresOrganizer types.Bool   `tfsdk:"sharing_folders_requires_organizer_permission"`
}

func NewSharedDriveDataSource() datasource.DataSource { return &sharedDriveDataSource{} }
func (d *sharedDriveDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	res.TypeName = req.ProviderTypeName + "_shared_drive"
}
func (d *sharedDriveDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{"id": schema.StringAttribute{Required: true, Description: "Shared Drive ID.", Validators: idValidators()}, "name": schema.StringAttribute{Computed: true, Description: "Shared Drive name."}}
	for _, name := range []string{"admin_managed_restrictions", "domain_users_only", "drive_members_only", "copy_requires_writer_permission", "sharing_folders_requires_organizer_permission"} {
		attrs[name] = schema.BoolAttribute{Computed: true, Description: "Current Shared Drive restriction setting."}
	}
	res.Schema = schema.Schema{Description: "Look up a Shared Drive by ID.", Attributes: attrs}
}
func (d *sharedDriveDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	var m sharedDriveDataModel
	res.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if res.Diagnostics.HasError() {
		return
	}
	v, err := d.client.API.GetDrive(ctx, m.ID.ValueString(), d.client.Admin)
	if err != nil {
		apiError(&res.Diagnostics, "look up Shared Drive", err)
		return
	}
	normalized := sharedDriveModel{}
	normalized.set(v)
	m.Name = normalized.Name
	m.AdminManaged = normalized.AdminManaged
	m.DomainOnly = normalized.DomainOnly
	m.MembersOnly = normalized.MembersOnly
	m.CopyRequiresWriter = normalized.CopyRequiresWriter
	m.SharingRequiresOrganizer = normalized.SharingRequiresOrganizer
	res.Diagnostics.Append(res.State.Set(ctx, &m)...)
}

type fileDataSource struct{ dataSourceBase }
type fileDataModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	MIME        types.String `tfsdk:"mime_type"`
	DriveID     types.String `tfsdk:"drive_id"`
	Parents     types.List   `tfsdk:"parents"`
	Trashed     types.Bool   `tfsdk:"trashed"`
}

func NewFileDataSource() datasource.DataSource { return &fileDataSource{} }
func (d *fileDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	res.TypeName = req.ProviderTypeName + "_file"
}
func (d *fileDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	res.Schema = schema.Schema{Description: "Read file or folder metadata. drive_id identifies the containing Shared Drive; it is null for My Drive items.", Attributes: map[string]schema.Attribute{
		"id":   schema.StringAttribute{Required: true, Description: "File or folder ID.", Validators: idValidators()},
		"name": schema.StringAttribute{Computed: true, Description: "Item name."}, "description": schema.StringAttribute{Computed: true, Description: "Item description."},
		"mime_type": schema.StringAttribute{Computed: true, Description: "MIME type."}, "drive_id": schema.StringAttribute{Computed: true, Description: "Containing Shared Drive ID, if any."},
		"parents": schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Parent folder IDs."}, "trashed": schema.BoolAttribute{Computed: true, Description: "Whether the item is in the trash."},
	}}
}
func (d *fileDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	var m fileDataModel
	res.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if res.Diagnostics.HasError() {
		return
	}
	v, err := d.client.API.GetFile(ctx, m.ID.ValueString())
	if err != nil {
		apiError(&res.Diagnostics, "look up file", err)
		return
	}
	m.Name = types.StringValue(v.Name)
	m.Description = types.StringValue(v.Description)
	m.MIME = types.StringValue(v.MimeType)
	m.DriveID = types.StringNull()
	if v.DriveId != "" {
		m.DriveID = types.StringValue(v.DriveId)
	}
	m.Trashed = types.BoolValue(v.Trashed)
	parents := v.Parents
	if parents == nil {
		parents = []string{}
	}
	list, diags := types.ListValueFrom(ctx, types.StringType, parents)
	res.Diagnostics.Append(diags...)
	m.Parents = list
	res.Diagnostics.Append(res.State.Set(ctx, &m)...)
}

type permissionsDataSource struct{ dataSourceBase }
type permissionsDataModel struct {
	FileID      types.String      `tfsdk:"file_id"`
	Permissions []permissionEntry `tfsdk:"permissions"`
}
type permissionEntry struct {
	ID         types.String `tfsdk:"id"`
	Type       types.String `tfsdk:"type"`
	Role       types.String `tfsdk:"role"`
	Email      types.String `tfsdk:"email_address"`
	Domain     types.String `tfsdk:"domain"`
	Direct     types.Bool   `tfsdk:"has_direct_grant"`
	DirectRole types.String `tfsdk:"direct_role"`
}

func NewPermissionsDataSource() datasource.DataSource { return &permissionsDataSource{} }
func (d *permissionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	res.TypeName = req.ProviderTypeName + "_permissions"
}
func (d *permissionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{}
	for _, name := range []string{"id", "type", "role", "email_address", "domain", "direct_role"} {
		attrs[name] = schema.StringAttribute{Computed: true, Description: "Permission " + name + ". direct_role is null for inherited-only grants."}
	}
	attrs["has_direct_grant"] = schema.BoolAttribute{Computed: true, Description: "Whether this item has a direct grant for this principal."}
	res.Schema = schema.Schema{Description: "List all permission pages for an item, including inherited grants. This data source does not modify the ACL.", Attributes: map[string]schema.Attribute{
		"file_id":     schema.StringAttribute{Required: true, Description: "File, folder, or Shared Drive ID.", Validators: idValidators()},
		"permissions": schema.ListNestedAttribute{Computed: true, Description: "Permissions returned by the API.", NestedObject: schema.NestedAttributeObject{Attributes: attrs}},
	}}
}
func (d *permissionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	var m permissionsDataModel
	res.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if res.Diagnostics.HasError() {
		return
	}
	values, err := d.client.API.Permissions(ctx, m.FileID.ValueString(), d.client.Admin)
	if err != nil {
		apiError(&res.Diagnostics, "list permissions", err)
		return
	}
	slices.SortFunc(values, func(a, b *api.Permission) int { return cmp.Compare(a.Id, b.Id) })
	m.Permissions = []permissionEntry{}
	for _, p := range values {
		if p.Deleted {
			continue
		}
		role, direct := drive.DirectRole(p)
		entry := permissionEntry{ID: types.StringValue(p.Id), Type: types.StringValue(p.Type), Role: types.StringValue(p.Role), Email: types.StringValue(p.EmailAddress), Domain: types.StringValue(p.Domain), Direct: types.BoolValue(direct), DirectRole: types.StringNull()}
		if direct {
			entry.DirectRole = types.StringValue(role)
		}
		m.Permissions = append(m.Permissions, entry)
	}
	res.Diagnostics.Append(res.State.Set(ctx, &m)...)
}
