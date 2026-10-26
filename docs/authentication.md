# Authentication

The provider requests `https://www.googleapis.com/auth/drive`. Managing arbitrary existing Shared Drives and permissions requires this scope; the per-file scope is insufficient for that administration use case. Enable Drive API in your Google Cloud project.

## Authentication modes

| Mode | Configuration | Notes |
| --- | --- | --- |
| Application Default Credentials | Empty provider block | Uses the standard ADC credential chain. Service accounts still need Drive membership; user ADC must include Drive scope. |
| Access token | `GDRIVE_ACCESS_TOKEN` or sensitive `access_token` | Short-lived and not refreshed. Cannot be combined with another authentication mode or a delegation subject. |
| Service-account key | `GDRIVE_CREDENTIALS_FILE` or `credentials_file` | A path to trusted service-account JSON, not inline JSON. Supports `subject`. |
| Keyless impersonation | `GDRIVE_IMPERSONATE_SERVICE_ACCOUNT` or `impersonate_service_account` | Uses ADC as the source identity. Supports `subject` for Workspace domain-wide delegation. |

Explicit attributes override their corresponding environment variables, including an explicitly empty string. `GDRIVE_SUBJECT` supplies the delegated Workspace user's email. Plain ADC plus a subject is rejected: configure a key or impersonation target so delegation cannot be silently ignored.

## Keyless Workspace administration

1. Create a service account for automation and enable IAM Service Account Credentials API.
2. Grant the ADC source principal Service Account Token Creator on that service account. Domain-wide delegation uses signing permissions such as `iam.serviceAccounts.signJwt`; access-token generation alone is not sufficient.
3. In the Workspace Admin Console, authorize the service account's OAuth client ID for domain-wide delegation with the Drive scope above.
4. Choose a Workspace user with the required administrative privileges. The user's organizational unit must permit Shared Drive creation. Folder operations also require content access.
5. Authenticate the source identity with ADC (or workload identity federation in CI):

```sh
gcloud auth application-default login
export GDRIVE_IMPERSONATE_SERVICE_ACCOUNT=drive-automation@YOUR_PROJECT.iam.gserviceaccount.com
export GDRIVE_SUBJECT=admin@example.com
```

```hcl
provider "gdrive" {
  use_domain_admin_access = true
}
```

The provider performs service-account impersonation itself. In this setup, do not also configure a service-account key. Domain administrator access is not a substitute for content permissions: it only applies to the Drive and permission API methods that support that flag.

## User ADC without delegation

Use an OAuth client appropriate for Drive scope and obtain ADC with that scope. A plain Cloud-only ADC login may lack Drive authorization. Follow [Google's local ADC guidance](https://cloud.google.com/docs/authentication/set-up-adc-local-dev-environment) for non-Cloud scopes. Do not assume a GCP project Owner role grants access to Workspace Drive data.

## Credential and state handling

Keep keys and tokens out of source control. Use environment variables or a secret manager, and protect state and plan files. `access_token` is marked sensitive; that redacts normal CLI output but is not encryption. Provider configuration can still be present in plan files.

API diagnostics include status and an allowlisted reason, but omit arbitrary response bodies that may contain private information. Debug logging in Terraform, OpenTofu, or SDK internals should still be treated as sensitive.

Live authentication against a real Workspace has not yet been verified in this project. Local tests verify configuration conflicts, credential type restrictions, and token refresh after the Configure RPC ends.

## References

- [Google ADC](https://cloud.google.com/docs/authentication/application-default-credentials)
- [Go impersonation SDK](https://pkg.go.dev/google.golang.org/api/impersonate)
- [Workspace domain-wide delegation](https://developers.google.com/identity/protocols/oauth2/service-account#delegatingauthority)
- [Manage Shared Drives](https://developers.google.com/workspace/drive/api/guides/manage-shareddrives)
