// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"testing"

	framework "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestProviderEnvironmentConflictAndExplicitOverride(t *testing.T) {
	for _, name := range []string{"GDRIVE_CREDENTIALS_FILE", "GDRIVE_IMPERSONATE_SERVICE_ACCOUNT", "GDRIVE_ACCESS_TOKEN", "GDRIVE_SUBJECT"} {
		t.Setenv(name, "")
	}
	t.Setenv("GDRIVE_ACCESS_TOKEN", "test-token")
	t.Setenv("GDRIVE_SUBJECT", "admin@example.com")
	ctx := context.Background()
	p := &GDriveProvider{}
	var sr framework.SchemaResponse
	p.Schema(ctx, framework.SchemaRequest{}, &sr)
	configure := func(m providerModel) framework.ConfigureResponse {
		state := tfsdk.State{Schema: sr.Schema}
		if d := state.Set(ctx, &m); d.HasError() {
			t.Fatal(d)
		}
		var res framework.ConfigureResponse
		p.Configure(ctx, framework.ConfigureRequest{Config: tfsdk.Config{Schema: sr.Schema, Raw: state.Raw}}, &res)
		return res
	}
	if res := configure(providerModel{}); !res.Diagnostics.HasError() {
		t.Fatal("conflicting token/subject environment accepted")
	}
	if res := configure(providerModel{Subject: types.StringValue("")}); res.Diagnostics.HasError() || res.ResourceData == nil {
		t.Fatalf("explicit empty subject must override environment: %v", res.Diagnostics)
	}
	if res := configure(providerModel{AccessToken: types.StringUnknown()}); !res.Diagnostics.HasError() {
		t.Fatal("unknown auth value accepted")
	}
}
