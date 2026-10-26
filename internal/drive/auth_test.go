// SPDX-License-Identifier: MPL-2.0
package drive

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAuthenticationConflicts(t *testing.T) {
	for _, a := range []AuthConfig{
		{AccessToken: "secret", Subject: "admin@example.com"},
		{AccessToken: "secret", CredentialsFile: "key.json"},
		{AccessToken: "secret", ImpersonateServiceAccount: "sa@example.com"},
		{CredentialsFile: "key.json", ImpersonateServiceAccount: "sa@example.com"},
		{Subject: "admin@example.com"},
	} {
		if a.Validate() == nil {
			t.Errorf("invalid configuration accepted: %+v", a)
		}
	}
	for _, a := range []AuthConfig{{}, {AccessToken: "secret"}, {CredentialsFile: "key.json", Subject: "admin@example.com"}, {ImpersonateServiceAccount: "sa@example.com", Subject: "admin@example.com"}} {
		if err := a.Validate(); err != nil {
			t.Error(err)
		}
	}
}

func TestExplicitCredentialsRejectUntrustedType(t *testing.T) {
	p := filepath.Join(t.TempDir(), "key.json")
	if err := os.WriteFile(p, []byte(`{"type":"external_account","credential_source":{"url":"http://untrusted"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := AuthenticatedClient(context.Background(), AuthConfig{CredentialsFile: p}); err == nil {
		t.Fatal("accepted unsupported credential type")
	}
}
