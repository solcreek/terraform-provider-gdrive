// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/solcreek/terraform-provider-gdrive/internal/drive"
	"google.golang.org/api/option"
)

func testAPI(t *testing.T, h http.HandlerFunc) *configuredClient {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	c, err := drive.NewClient(context.Background(), s.Client(), option.WithEndpoint(s.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	c.Retries = 0
	return &configuredClient{API: c, Admin: true}
}
func driveSchema(t *testing.T, r *sharedDriveResource) schema.Schema {
	t.Helper()
	var response resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &response)
	return response.Schema
}
func TestRestrictionsSendFalse(t *testing.T) {
	m := sharedDriveModel{Name: types.StringValue("drive"), DomainOnly: types.BoolValue(false), MembersOnly: types.BoolUnknown()}
	b, err := json.Marshal(m.body())
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Restrictions map[string]any `json:"restrictions"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	if value, ok := v.Restrictions["domainUsersOnly"]; !ok || value != false {
		t.Fatalf("explicit false lost: %s", b)
	}
	if _, ok := v.Restrictions["driveMembersOnly"]; ok {
		t.Fatalf("unknown value sent: %s", b)
	}
}
func TestSharedDriveReadErrorState(t *testing.T) {
	for _, code := range []int{403, 404, 500} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			r := &sharedDriveResource{resourceBase{testAPI(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) })}}
			state := tfsdk.State{Schema: driveSchema(t, r)}
			ctx := context.Background()
			if d := state.Set(ctx, &sharedDriveModel{ID: types.StringValue("drive"), Name: types.StringValue("name")}); d.HasError() {
				t.Fatal(d)
			}
			res := resource.ReadResponse{State: state}
			r.Read(ctx, resource.ReadRequest{State: state}, &res)
			if code == 404 {
				if !res.State.Raw.IsNull() || res.Diagnostics.HasError() {
					t.Fatalf("404 must remove resource: %v", res.Diagnostics)
				}
			} else {
				if res.State.Raw.IsNull() || !res.Diagnostics.HasError() {
					t.Fatal("non-404 must preserve state and report error")
				}
			}
		})
	}
}
func TestSharedDriveDeleteProtection(t *testing.T) {
	called := false
	r := &sharedDriveResource{resourceBase{testAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(204) })}}
	state := tfsdk.State{Schema: driveSchema(t, r)}
	ctx := context.Background()
	if d := state.Set(ctx, &sharedDriveModel{ID: types.StringValue("drive"), Protection: types.BoolValue(true)}); d.HasError() {
		t.Fatal(d)
	}
	res := resource.DeleteResponse{State: state}
	r.Delete(ctx, resource.DeleteRequest{State: state}, &res)
	if called || !res.Diagnostics.HasError() {
		t.Fatal("protected drive deletion was not blocked")
	}
}
