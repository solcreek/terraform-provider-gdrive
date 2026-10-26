// SPDX-License-Identifier: MPL-2.0
package drive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	api "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	c, err := NewClient(context.Background(), s.Client(), option.WithEndpoint(s.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	c.Retries = 2
	c.Wait = func(context.Context, time.Duration) error { return nil }
	return c
}
func TestRetryPolicy(t *testing.T) {
	for _, tc := range []struct {
		name   string
		code   int
		reason string
		safe   bool
		calls  int
	}{
		{"rate limit", 403, "rateLimitExceeded", true, 3}, {"quota", 429, "", true, 3}, {"unavailable", 503, "", true, 3},
		{"access denied", 403, "insufficientFilePermissions", true, 1}, {"not found", 404, "notFound", true, 1}, {"non-idempotent create", 503, "", false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.code)
				fmt.Fprintf(w, `{"error":{"code":%d,"message":"SECRET","errors":[{"reason":%q}]}}`, tc.code, tc.reason)
			})
			var err error
			if tc.safe {
				_, err = c.GetDrive(context.Background(), "id", false)
			} else {
				_, err = c.CreateFolder(context.Background(), &api.File{Name: "test"})
			}
			if err == nil || strings.Contains(err.Error(), "SECRET") {
				t.Fatalf("unsafe or missing error: %v", err)
			}
			if calls != tc.calls {
				t.Fatalf("calls=%d want %d", calls, tc.calls)
			}
		})
	}
}
func TestCreateDriveRetryKeepsRequestID(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("requestId") != "fixed" {
			t.Error("request ID changed")
		}
		if calls == 1 {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, `{"id":"drive","name":"test"}`)
	})
	v, err := c.CreateDrive(context.Background(), "fixed", &api.Drive{Name: "test"})
	if err != nil || v.Id != "drive" || calls != 2 {
		t.Fatalf("v=%v err=%v calls=%d", v, err, calls)
	}
}
func TestRetryHonorsCancellation(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "20"); w.WriteHeader(503) })
	c.Wait = func(_ context.Context, d time.Duration) error {
		if d < 20*time.Second {
			t.Errorf("Retry-After ignored: %s", d)
		}
		return context.Canceled
	}
	_, err := c.GetDrive(context.Background(), "id", false)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestPermissionsPagination(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if q.Get("supportsAllDrives") != "true" || q.Get("useDomainAdminAccess") != "true" {
			t.Error("missing Shared Drive/admin flags")
		}
		if calls == 1 {
			fmt.Fprint(w, `{"permissions":[{"id":"a"}],"nextPageToken":"next"}`)
		} else {
			if q.Get("pageToken") != "next" {
				t.Error("missing page token")
			}
			fmt.Fprint(w, `{"permissions":[{"id":"b"}]}`)
		}
	})
	p, err := c.Permissions(context.Background(), "drive", true)
	if err != nil || len(p) != 2 || p[1].Id != "b" {
		t.Fatalf("%v %v", p, err)
	}
}
func TestDirectRole(t *testing.T) {
	p := &api.Permission{Role: "organizer", PermissionDetails: []*api.PermissionPermissionDetails{{Inherited: true, Role: "organizer"}}}
	if _, ok := DirectRole(p); ok {
		t.Fatal("inherited grant considered direct")
	}
	p.PermissionDetails = append(p.PermissionDetails, &api.PermissionPermissionDetails{Role: "reader"})
	if role, ok := DirectRole(p); !ok || role != "reader" {
		t.Fatalf("role=%s direct=%t", role, ok)
	}
}
func TestSDKExplicitFalseAndEmptyValues(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("supportsAllDrives") != "true" {
			t.Error("missing supportsAllDrives")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if v, ok := body["description"]; !ok || v != "" {
			t.Error("empty description omitted")
		}
		fmt.Fprint(w, `{"id":"folder"}`)
	})
	_, err := c.UpdateFolder(context.Background(), "folder", &api.File{Description: "", ForceSendFields: []string{"Description"}}, "", "")
	if err != nil {
		t.Fatal(err)
	}
}
func TestDeleteDriveNeverDeletesContents(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("allowItemDeletion") != "false" {
			t.Error("recursive deletion enabled")
		}
		if r.URL.Query().Get("useDomainAdminAccess") != "true" {
			t.Error("admin flag missing")
		}
		w.WriteHeader(204)
	})
	if err := c.DeleteDrive(context.Background(), "drive", true); err != nil {
		t.Fatal(err)
	}
}
