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
	"github.com/solcreek/terraform-provider-gdrive/internal/drive"
	api "google.golang.org/api/drive/v3"
)

func TestFolderRejectsNonSharedDriveFolder(t *testing.T) {
	for _, v := range []*api.File{
		{Id: "file", MimeType: "text/plain", DriveId: "drive", Parents: []string{"drive"}},
		{Id: "folder", MimeType: drive.FolderMIME, Parents: []string{"parent"}},
		{Id: "drive", MimeType: drive.FolderMIME, DriveId: "drive"},
	} {
		var m folderModel
		if m.set(v) == nil {
			t.Fatalf("accepted invalid folder: %+v", v)
		}
	}
}
func TestFolderCrossDriveMoveDoesNotWrite(t *testing.T) {
	calls := 0
	r := &folderResource{resourceBase{testAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" {
			t.Error("unexpected write")
		}
		fmt.Fprint(w, `{"id":"new","mimeType":"application/vnd.google-apps.folder","driveId":"other"}`)
	})}}
	ctx := context.Background()
	var sr resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	old := folderModel{ID: types.StringValue("folder"), Name: types.StringValue("folder"), DriveID: types.StringValue("drive"), Parent: types.StringValue("old"), Description: types.StringValue(""), Protection: types.BoolValue(false)}
	state := tfsdk.State{Schema: sr.Schema}
	if d := state.Set(ctx, &old); d.HasError() {
		t.Fatal(d)
	}
	next := old
	next.Parent = types.StringValue("new")
	plan := tfsdk.Plan{Schema: sr.Schema}
	if d := plan.Set(ctx, &next); d.HasError() {
		t.Fatal(d)
	}
	res := resource.UpdateResponse{State: state}
	r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, &res)
	if !res.Diagnostics.HasError() || calls != 1 {
		t.Fatalf("cross-drive move not blocked: %v", res.Diagnostics)
	}
}
