// SPDX-License-Identifier: MPL-2.0
package drive

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/impersonate"
)

type AuthConfig struct{ CredentialsFile, AccessToken, ImpersonateServiceAccount, Subject string }

func (a AuthConfig) Validate() error {
	if a.AccessToken != "" && (a.CredentialsFile != "" || a.ImpersonateServiceAccount != "" || a.Subject != "") {
		return errors.New("access_token cannot be combined with credentials_file, impersonate_service_account, or subject")
	}
	if a.CredentialsFile != "" && a.ImpersonateServiceAccount != "" {
		return errors.New("credentials_file and impersonate_service_account cannot be combined; use ADC as the source for impersonation")
	}
	if a.Subject != "" && a.CredentialsFile == "" && a.ImpersonateServiceAccount == "" {
		return errors.New("subject requires credentials_file (service-account key) or impersonate_service_account; plain ADC cannot silently delegate a Workspace user")
	}
	return nil
}

func AuthenticatedClient(ctx context.Context, a AuthConfig) (*http.Client, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	var ts oauth2.TokenSource
	switch {
	case a.AccessToken != "":
		ts = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: a.AccessToken})
	case a.CredentialsFile != "":
		data, err := os.ReadFile(a.CredentialsFile)
		if err != nil {
			return nil, errors.New("cannot read credentials_file")
		}
		var kind struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(data, &kind) != nil || kind.Type != "service_account" {
			return nil, errors.New("credentials_file must contain a service_account key; use ADC for other credential types")
		}
		cfg, err := google.JWTConfigFromJSON(data, Scope)
		if err != nil {
			return nil, errors.New("invalid service-account credential JSON")
		}
		// Only Google's token service is allowed for explicitly supplied key files.
		if cfg.TokenURL != "https://oauth2.googleapis.com/token" && cfg.TokenURL != "https://accounts.google.com/o/oauth2/token" {
			return nil, errors.New("unexpected service-account token_uri")
		}
		cfg.Subject = a.Subject
		ts = cfg.TokenSource(ctx)
	case a.ImpersonateServiceAccount != "":
		var err error
		ts, err = impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{TargetPrincipal: a.ImpersonateServiceAccount, Scopes: []string{Scope}, Subject: a.Subject})
		if err != nil {
			return nil, errors.New("cannot configure service-account impersonation; verify ADC and IAM credentials permissions")
		}
	default:
		creds, err := google.FindDefaultCredentials(ctx, Scope)
		if err != nil {
			return nil, errors.New("application default credentials unavailable; configure ADC or an explicit authentication method")
		}
		ts = creds.TokenSource
	}
	client := oauth2.NewClient(ctx, ts)
	client.Timeout = 60 * time.Second
	return client, nil
}
