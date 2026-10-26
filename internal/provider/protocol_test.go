// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	framework "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/solcreek/terraform-provider-gdrive/internal/drive"
	"google.golang.org/api/option"
)

// Only tests inject an API endpoint. Production provider configuration never
// exposes an endpoint override or skips credential validation.
type protocolProvider struct {
	GDriveProvider
	client *configuredClient
}

func (p *protocolProvider) Configure(_ context.Context, _ framework.ConfigureRequest, res *framework.ConfigureResponse) {
	res.ResourceData = p.client
	res.DataSourceData = p.client
}

type fakeDrive struct {
	mu          sync.Mutex
	next        int
	drives      map[string]map[string]any
	files       map[string]map[string]any
	permissions map[string]map[string]map[string]any
}

func newFakeDrive() *fakeDrive {
	return &fakeDrive{
		drives:      map[string]map[string]any{"root-drive": {"id": "root-drive", "name": "Fixture", "restrictions": map[string]any{}}},
		files:       map[string]map[string]any{"other-folder": {"id": "other-folder", "name": "Other", "mimeType": drive.FolderMIME, "driveId": "root-drive", "parents": []string{"root-drive"}}},
		permissions: map[string]map[string]map[string]any{},
	}
}
func (f *fakeDrive) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	write := func(v any) {
		if err := json.NewEncoder(w).Encode(v); err != nil {
			panic(err)
		}
	}
	missing := func() {
		w.WriteHeader(404)
		write(map[string]any{"error": map[string]any{"code": 404, "message": "missing"}})
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	body := map[string]any{}
	if r.Method == "POST" || r.Method == "PATCH" {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(400)
			return
		}
	}
	id := ""
	if len(parts) > 1 {
		id = parts[1]
	}
	switch {
	case parts[0] == "drives":
		switch r.Method {
		case "POST":
			f.next++
			id = fmt.Sprintf("drive-%d", f.next)
			body["id"] = id
			if body["restrictions"] == nil {
				body["restrictions"] = map[string]any{}
			}
			f.drives[id] = body
			write(body)
		case "GET":
			if v := f.drives[id]; v != nil {
				write(v)
			} else {
				missing()
			}
		case "PATCH":
			v := f.drives[id]
			if v == nil {
				missing()
				return
			}
			for k, x := range body {
				if k == "restrictions" {
					for rk, rv := range x.(map[string]any) {
						v[k].(map[string]any)[rk] = rv
					}
				} else {
					v[k] = x
				}
			}
			write(v)
		case "DELETE":
			delete(f.drives, id)
			w.WriteHeader(204)
		default:
			w.WriteHeader(405)
		}
	case parts[0] == "files" && len(parts) >= 3 && parts[2] == "permissions":
		perms := f.permissions[id]
		if perms == nil {
			perms = map[string]map[string]any{}
			f.permissions[id] = perms
		}
		pid := ""
		if len(parts) > 3 {
			pid = parts[3]
		}
		switch r.Method {
		case "GET":
			if pid != "" {
				if v := perms[pid]; v != nil {
					write(v)
				} else {
					missing()
				}
			} else {
				// Mirrors the live API, which rejects an empty pageToken.
				if q := r.URL.Query(); q.Has("pageToken") && q.Get("pageToken") == "" {
					w.WriteHeader(403)
					write(map[string]any{"error": map[string]any{"code": 403, "message": "The specified page token has expired, and can no longer be used.", "errors": []map[string]any{{"reason": "pageTokenExpired"}}}})
					return
				}
				list := []map[string]any{}
				for _, v := range perms {
					list = append(list, v)
				}
				write(map[string]any{"permissions": list})
			}
		case "POST":
			f.next++
			pid = fmt.Sprintf("permission-%d", f.next)
			body["id"] = pid
			body["permissionDetails"] = []map[string]any{{"inherited": false, "role": body["role"]}}
			perms[pid] = body
			write(body)
		case "PATCH":
			v := perms[pid]
			if v == nil {
				missing()
				return
			}
			for k, x := range body {
				v[k] = x
			}
			v["permissionDetails"] = []map[string]any{{"inherited": false, "role": v["role"]}}
			write(v)
		case "DELETE":
			delete(perms, pid)
			w.WriteHeader(204)
		}
	case parts[0] == "files":
		switch r.Method {
		case "POST":
			f.next++
			id = fmt.Sprintf("folder-%d", f.next)
			body["id"] = id
			body["driveId"] = "root-drive"
			f.files[id] = body
			write(body)
		case "GET":
			if v := f.files[id]; v != nil {
				write(v)
			} else if v := f.drives[id]; v != nil {
				write(map[string]any{"id": id, "name": v["name"], "mimeType": drive.FolderMIME, "driveId": id})
			} else {
				missing()
			}
		case "PATCH":
			v := f.files[id]
			if v == nil {
				missing()
				return
			}
			for k, x := range body {
				v[k] = x
			}
			if p := r.URL.Query().Get("addParents"); p != "" {
				v["parents"] = []string{p}
			}
			write(v)
		default:
			w.WriteHeader(405)
		}
	default:
		w.WriteHeader(400)
	}
}
func protocolFactory(t *testing.T, f *fakeDrive) map[string]func() (tfprotov6.ProviderServer, error) {
	t.Helper()
	s := httptest.NewServer(f)
	t.Cleanup(s.Close)
	c, err := drive.NewClient(context.Background(), s.Client(), option.WithEndpoint(s.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	c.Retries = 0
	p := &protocolProvider{GDriveProvider: GDriveProvider{version: "test"}, client: &configuredClient{API: c, Admin: true}}
	return map[string]func() (tfprotov6.ProviderServer, error){"gdrive": providerserver.NewProtocol6WithError(p)}
}
func TestProtocolSharedDrive(t *testing.T) {
	f := newFakeDrive()
	factory := protocolFactory(t, f)
	config := func(name string, restricted bool) string {
		return fmt.Sprintf(`
provider "gdrive" {}
resource "gdrive_shared_drive" "test" {
 name = %q
 domain_users_only = %t
 drive_members_only = false
 deletion_protection = false
}
data "gdrive_shared_drive" "test" { id = gdrive_shared_drive.test.id }
`, name, restricted)
	}
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factory, Steps: []resource.TestStep{
		{Config: config("Initial", true), Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("gdrive_shared_drive.test", "domain_users_only", "true"), resource.TestCheckResourceAttr("data.gdrive_shared_drive.test", "name", "Initial"))},
		{ResourceName: "gdrive_shared_drive.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"deletion_protection"}},
		{Config: config("Renamed", false), Check: resource.TestCheckResourceAttr("gdrive_shared_drive.test", "domain_users_only", "false")},
	}})
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.drives) != 1 {
		t.Fatalf("managed drive was not deleted: %v", f.drives)
	}
}
func TestProtocolFolder(t *testing.T) {
	f := newFakeDrive()
	factory := protocolFactory(t, f)
	config := func(parent, desc string) string {
		return fmt.Sprintf(`
provider "gdrive" {}
resource "gdrive_folder" "test" {
 name = "Documents"
 parent_id = %q
 description = %q
 deletion_protection = false
}
data "gdrive_file" "test" { id = gdrive_folder.test.id }
`, parent, desc)
	}
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factory, Steps: []resource.TestStep{
		{Config: config("root-drive", "Initial description"), Check: resource.TestCheckResourceAttr("data.gdrive_file.test", "drive_id", "root-drive")},
		{ResourceName: "gdrive_folder.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"deletion_protection"}},
		{Config: config("other-folder", ""), Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("gdrive_folder.test", "parent_id", "other-folder"), resource.TestCheckResourceAttr("gdrive_folder.test", "description", ""))},
	}})
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, v := range f.files {
		if id != "other-folder" && v["trashed"] != true {
			t.Fatalf("folder not trashed: %v", v)
		}
	}
}
func TestProtocolPermission(t *testing.T) {
	f := newFakeDrive()
	f.permissions["root-drive"] = map[string]map[string]any{"unrelated": {"id": "unrelated", "type": "group", "emailAddress": "other@example.com", "role": "reader"}}
	factory := protocolFactory(t, f)
	config := func(role string) string {
		return fmt.Sprintf(`
provider "gdrive" {}
resource "gdrive_permission" "test" {
 file_id = "root-drive"
 type = "group"
 email_address = "team@example.com"
 role = %q
}

data "gdrive_permissions" "test" { file_id = gdrive_permission.test.file_id }
`, role)
	}
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factory, Steps: []resource.TestStep{
		{Config: config("reader"), Check: resource.TestCheckResourceAttr("gdrive_permission.test", "role", "reader")},
		{ResourceName: "gdrive_permission.test", ImportState: true, ImportStateVerify: true},
		{Config: config("writer"), Check: resource.TestCheckResourceAttr("gdrive_permission.test", "role", "writer")},
	}})
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.permissions["root-drive"]) != 1 || f.permissions["root-drive"]["unrelated"] == nil {
		t.Fatalf("unrelated ACL altered or managed permission not deleted: %v", f.permissions)
	}
}

func TestProtocolPermissionPrincipalValidation(t *testing.T) {
	for _, principal := range []string{"user", "group", "domain", "anyone"} {
		t.Run(principal, func(t *testing.T) {
			f := newFakeDrive()
			factory := protocolFactory(t, f)
			fields := ""
			switch principal {
			case "user", "group":
				fields = `email_address = "principal@example.com"`
			case "domain":
				fields = `domain = "example.com"`
			}
			config := fmt.Sprintf(`provider "gdrive" {}
resource "gdrive_permission" "test" {
 file_id = "root-drive"
 type = %q
 role = "reader"
 %s
}`, principal, fields)
			resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factory, Steps: []resource.TestStep{{Config: config}, {ResourceName: "gdrive_permission.test", ImportState: true, ImportStateVerify: true}}})
		})
	}
}

func TestProtocolMissingPrincipalRejected(t *testing.T) {
	f := newFakeDrive()
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: protocolFactory(t, f), Steps: []resource.TestStep{{Config: `provider "gdrive" {}
resource "gdrive_permission" "test" {
 file_id = "root-drive"
 type = "group"
 role = "reader"
}`, ExpectError: regexp.MustCompile("Missing principal")}}})
}
