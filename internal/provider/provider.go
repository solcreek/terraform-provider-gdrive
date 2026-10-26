// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/linyiru/terraform-provider-gdrive/internal/drive"
)

type GDriveProvider struct{ version string }
type providerModel struct {
	CredentialsFile types.String `tfsdk:"credentials_file"`
	AccessToken     types.String `tfsdk:"access_token"`
	Impersonate     types.String `tfsdk:"impersonate_service_account"`
	Subject         types.String `tfsdk:"subject"`
	Admin           types.Bool   `tfsdk:"use_domain_admin_access"`
}
type configuredClient struct {
	API   *drive.Client
	Admin bool
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &GDriveProvider{version: version} }
}
func (p *GDriveProvider) Metadata(_ context.Context, _ provider.MetadataRequest, r *provider.MetadataResponse) {
	r.TypeName = "gdrive"
	r.Version = p.version
}
func (p *GDriveProvider) Schema(_ context.Context, _ provider.SchemaRequest, r *provider.SchemaResponse) {
	r.Schema = schema.Schema{Description: "Manage Google Workspace Shared Drives, folders, and direct permissions.", Attributes: map[string]schema.Attribute{
		"credentials_file":            schema.StringAttribute{Optional: true, Description: "Path to a service-account key. GDRIVE_CREDENTIALS_FILE fallback. Prefer ADC or keyless impersonation."},
		"access_token":                schema.StringAttribute{Optional: true, Sensitive: true, Description: "Short-lived OAuth access token. GDRIVE_ACCESS_TOKEN fallback; no automatic refresh."},
		"impersonate_service_account": schema.StringAttribute{Optional: true, Description: "Service-account email to impersonate using ADC. GDRIVE_IMPERSONATE_SERVICE_ACCOUNT fallback."},
		"subject":                     schema.StringAttribute{Optional: true, Description: "Workspace user for domain-wide delegation. GDRIVE_SUBJECT fallback. Requires key or impersonation."},
		"use_domain_admin_access":     schema.BoolAttribute{Optional: true, Description: "Use domain administrator access for Drive and permission operations. Defaults to false; does not grant access to file contents."},
	}}
}
func (p *GDriveProvider) Configure(ctx context.Context, req provider.ConfigureRequest, r *provider.ConfigureResponse) {
	var m providerModel
	r.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if r.Diagnostics.HasError() {
		return
	}
	if m.CredentialsFile.IsUnknown() || m.AccessToken.IsUnknown() || m.Impersonate.IsUnknown() || m.Subject.IsUnknown() || m.Admin.IsUnknown() {
		r.Diagnostics.AddError("Unknown provider configuration", "Authentication and administrator access must be known before the provider can be configured.")
		return
	}
	env := func(v types.String, name string) string {
		if v.IsNull() {
			return os.Getenv(name)
		}
		return v.ValueString()
	}
	a := drive.AuthConfig{CredentialsFile: env(m.CredentialsFile, "GDRIVE_CREDENTIALS_FILE"), AccessToken: env(m.AccessToken, "GDRIVE_ACCESS_TOKEN"), ImpersonateServiceAccount: env(m.Impersonate, "GDRIVE_IMPERSONATE_SERVICE_ACCOUNT"), Subject: env(m.Subject, "GDRIVE_SUBJECT")}
	h, err := drive.AuthenticatedClient(ctx, a)
	if err != nil {
		r.Diagnostics.AddError("Authentication configuration failed", err.Error())
		return
	}
	api, err := drive.NewClient(ctx, h)
	if err != nil {
		r.Diagnostics.AddError("Drive SDK initialization failed", "Unable to initialize the Google Drive v3 SDK.")
		return
	}
	c := &configuredClient{API: api, Admin: m.Admin.ValueBool()}
	r.ResourceData = c
	r.DataSourceData = c
}
func (p *GDriveProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewSharedDriveResource, NewFolderResource, NewPermissionResource}
}
func (p *GDriveProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewSharedDriveDataSource, NewFileDataSource, NewPermissionsDataSource}
}
