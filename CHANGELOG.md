# Changelog

## Unreleased

- Fix permission listing against the live Drive API. The first page sent an empty `pageToken`, which Drive rejects with 403 `pageTokenExpired`. That broke the `gdrive_permissions` data source and `gdrive_permission` creation, both of which list existing grants. Verified read-only against a real Workspace Shared Drive.

## 0.1.0

Initial experimental release. Local contract and provider-protocol tests cover Terraform and OpenTofu. Live Google Workspace acceptance remains outstanding.

- Manage Shared Drives, five standard restriction flags, and protected empty-drive deletion.
- Manage Shared Drive folders, metadata, same-drive moves, and protected trash deletion.
- Manage individual direct permissions with import, role updates, and inheritance detection.
- Look up Shared Drives, file metadata, and paginated permissions.
- Authenticate through ADC, access tokens, service-account keys, or keyless impersonation and domain-wide delegation.
- Use the official Google Drive API v3 Go SDK, pinned at v0.308.0.
- Publish MPL-2.0 source and signed multi-platform release archives with dependency notices.
