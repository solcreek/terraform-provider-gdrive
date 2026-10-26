# Terraform / OpenTofu Provider for Google Drive

[![Tests](https://github.com/solcreek/terraform-provider-gdrive/actions/workflows/test.yml/badge.svg)](https://github.com/solcreek/terraform-provider-gdrive/actions/workflows/test.yml)
[![License: MPL-2.0](https://img.shields.io/badge/License-MPL--2.0-blue)](LICENSE)

An independent provider for **Google Workspace Shared Drives**, using Google's official **Drive API v3 Go SDK**. Original source is licensed under MPL-2.0.

**Status: experimental.** Terraform and OpenTofu lifecycle tests pass against a local API fixture. Live Workspace acceptance has not been completed. The initial OpenTofu Registry registration is pending; GitHub release artifacts and local development overrides can be used before registration.

## Resources and data sources

| Kind | Name | Purpose |
| --- | --- | --- |
| Resource | `gdrive_shared_drive` | Shared Drive lifecycle, five standard restriction flags, protected empty-drive deletion. |
| Resource | `gdrive_folder` | Folders, descriptions, same-drive moves, protected trash deletion. |
| Resource | `gdrive_permission` | Individual direct user, group, domain, or anyone permissions. |
| Data source | `gdrive_shared_drive` | Drive name and restrictions. |
| Data source | `gdrive_file` | File/folder metadata, parent IDs, and containing Shared Drive ID. |
| Data source | `gdrive_permissions` | Paginated permissions with direct/inherited role information. |

All resources support import. See [the feature matrix](docs/feature-matrix.md) for the release boundary and verification status. Labels, organizational-unit placement, uploads, ownership transfer, permission expiration, and newer nested download restrictions are not implemented.

## Usage

Following [Solcreek's Gandi provider](https://github.com/solcreek/terraform-provider-gandi), the publishing target is the **OpenTofu Registry**. Use the full address below with either CLI once registration is complete; Terraform resolves short addresses against a different registry.

```hcl
terraform {
  required_providers {
    gdrive = {
      source  = "registry.opentofu.org/solcreek/gdrive"
      version = "~> 0.1"
    }
  }
}

provider "gdrive" {
  impersonate_service_account = "drive-automation@YOUR_PROJECT.iam.gserviceaccount.com"
  subject                     = "admin@example.com"
  use_domain_admin_access     = true
}

resource "gdrive_shared_drive" "engineering" {
  name                = "Engineering"
  domain_users_only   = true
  deletion_protection = true
}

resource "gdrive_permission" "engineering" {
  file_id       = gdrive_shared_drive.engineering.id
  type          = "group"
  email_address = "engineering@example.com"
  role          = "writer"
}
```

Enable Drive API and configure ADC plus Workspace domain-wide delegation for the impersonated service account. See [authentication](docs/authentication.md) for all supported modes and IAM/Workspace permission boundaries. Groups must already exist.

For an existing Drive, import before applying to avoid creating a second Drive with the same name:

```sh
tofu import gdrive_shared_drive.engineering SHARED_DRIVE_ID
tofu import gdrive_permission.engineering 'SHARED_DRIVE_ID/PERMISSION_ID'
tofu plan
```

The same commands work with `terraform`. Keep one state owner per managed resource/permission. This provider's state format is not interchangeable with other Google Drive providers; migrating requires a state backup and fresh imports.

## Lifecycle and access behavior

- Drives and folders default to deletion protection. Apply `deletion_protection = false` before destroying them. Drive deletion requires an empty Drive; folder deletion moves the folder **and its descendants** to trash.
- Permissions are additive individual grants, not an authoritative whole-ACL policy. Existing direct grants require import. Inherited-only grants belong to their originating parent.
- Google Drive uses `organizer` for Manager, `fileOrganizer` for Content manager, and `writer` for Contributor. Shared Drives do not have an individual `owner` role.
- Domain administration does not automatically grant content access. Folder operations require the delegated user's actual Drive permissions.
- Google can return 404 for inaccessible objects as well as deleted objects. Read treats 404 as absent; verify credentials and access before approving a recreation plan. Other errors retain state.
- After an interrupted creation, inspect Google Drive and import any created object before retrying. Folder and permission creation are not automatically replayed.

See [generated provider documentation](docs/index.md), [examples](examples/), and [official guidance mapped to implementation](docs/best-practices.md).

## Development and releases

```sh
make build
make test
make test-terraform
make test-tofu
make vet
make generate
make license-check
make vulncheck
```

The SDK is pinned to `google.golang.org/api v0.308.0`, verified against `@latest` on October 26, 2026. Dependabot proposes updates weekly. Go 1.26+ is required. All project documentation, comments, and diagnostics use US English.

[Development instructions](docs/development.md) cover local installation, opt-in live acceptance, signing, and registry submission. CI checks unit tests, race detection, both CLI protocols, generated docs, dependency licenses, and reachable vulnerabilities. Releases contain signed checksums, a protocol manifest, and dependency notices/source where required.

## License

[Mozilla Public License 2.0](LICENSE). Dependencies retain their original licenses; see [NOTICE](NOTICE) and the release archives' `third_party/` contents.
