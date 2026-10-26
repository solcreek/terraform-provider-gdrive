// SPDX-License-Identifier: MPL-2.0
package drive

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/oauth2"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTokenRefreshOutlivesConfigureContext(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	data, err := json.Marshal(map[string]string{"type": "service_account", "client_email": "test@example.iam.gserviceaccount.com", "private_key": string(encoded), "token_uri": "https://oauth2.googleapis.com/token"})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "key.json")
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	h := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Context().Err() != nil {
			return nil, r.Context().Err()
		}
		calls++
		body := `{}`
		if r.URL.Host == "oauth2.googleapis.com" {
			body = `{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`
		} else if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing access token")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), oauth2.HTTPClient, h))
	client, err := AuthenticatedClient(ctx, AuthConfig{CredentialsFile: p, Subject: "admin@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	response, err := client.Get("https://www.googleapis.com/drive/v3/drives/test")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if calls != 2 {
		t.Fatalf("expected token refresh and API call, got %d", calls)
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
