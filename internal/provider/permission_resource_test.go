// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPermissionInheritedDeleteDoesNotWrite(t *testing.T) {
	r := &permissionResource{resourceBase{testAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("attempted to mutate inherited permission")
		}
		fmt.Fprint(w, `{"id":"p","role":"reader","permissionDetails":[{"inherited":true,"role":"reader"}]}`)
	})}}
	ctx := context.Background()
	var sr resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	state := tfsdk.State{Schema: sr.Schema}
	if d := state.Set(ctx, &permissionModel{ID: types.StringValue("f/p"), FileID: types.StringValue("f"), PermissionID: types.StringValue("p")}); d.HasError() {
		t.Fatal(d)
	}
	res := resource.DeleteResponse{State: state}
	r.Delete(ctx, resource.DeleteRequest{State: state}, &res)
	if res.Diagnostics.HasError() {
		t.Fatal(res.Diagnostics)
	}
}
func TestPermissionImportRejectsMalformedIDs(t *testing.T) {
	r := &permissionResource{}
	ctx := context.Background()
	var sr resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	for _, id := range []string{"", "p", "f/", "/p", "f/p/extra", "https://drive.google.com/f/p"} {
		res := resource.ImportStateResponse{State: tfsdk.State{Schema: sr.Schema}}
		r.ImportState(ctx, resource.ImportStateRequest{ID: id}, &res)
		if !res.Diagnostics.HasError() {
			t.Errorf("accepted %q", id)
		}
	}
}

func TestPermissionCreateRequiresImportOfExistingGrant(t *testing.T) {
	calls := 0
	r := &permissionResource{resourceBase{testAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" {
			t.Error("existing grant was mutated")
		}
		fmt.Fprint(w, `{"permissions":[{"id":"p","type":"group","emailAddress":"team@example.com","role":"reader"}]}`)
	})}}
	ctx := context.Background()
	var sr resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	plan := tfsdk.Plan{Schema: sr.Schema}
	if d := plan.Set(ctx, &permissionModel{FileID: types.StringValue("drive"), Type: types.StringValue("group"), Email: types.StringValue("team@example.com"), Role: types.StringValue("writer")}); d.HasError() {
		t.Fatal(d)
	}
	res := resource.CreateResponse{State: tfsdk.State{Schema: sr.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, &res)
	if !res.Diagnostics.HasError() || calls != 1 {
		t.Fatalf("existing grant was not rejected: %v", res.Diagnostics)
	}
}
