# Terraform / OpenTofu Provider for Google Drive

An independently implemented, MPL-2.0 provider focused on Google Workspace Shared Drive administration.

This project is under development and has not been released to either registry. The development source address is `registry.opentofu.org/solcreek/gdrive`; it does not imply a published package.

## Scope

The initial scope is Shared Drives, folders, direct permissions, import, and read-only discovery. Authentication includes Application Default Credentials (ADC), service-account keys, and keyless service-account impersonation with optional Workspace domain-wide delegation.

Read [the feature matrix](docs/feature-matrix.md) for implementation and verification status. Labels, organization-unit placement, file content upload, and authoritative ACL replacement are outside the initial scope. Full Drive API coverage is not claimed.

## License and implementation provenance

Original source is licensed under [MPL-2.0](LICENSE). Implementation is based on Google's public API specifications and the Terraform Plugin Framework. No source code from the GPL-licensed `hanneshayashi/gdrive` provider is included. The earlier evaluation inspected that provider's public documentation; this project does not claim a formally isolated clean-room process.

Dependencies retain their own licenses. Do not relicense dependencies or copy code from another provider without reviewing its license. A reproducible dependency inventory and license checks are part of the release checklist.

## Development

All repository documentation, comments, and user-facing diagnostics use US English. Changes should be small, independently reviewable commits with appropriate tests. Google Workspace credentials are not required for local HTTP contract tests; live acceptance tests must use a dedicated test Shared Drive.

See [development](docs/development.md) and [authentication](docs/authentication.md) for setup instructions as the implementation lands.

## API references

- [Shared Drives](https://developers.google.com/workspace/drive/api/reference/rest/v3/drives)
- [Files and folders](https://developers.google.com/workspace/drive/api/reference/rest/v3/files)
- [Permissions](https://developers.google.com/workspace/drive/api/reference/rest/v3/permissions)
- [Role mapping](https://developers.google.com/workspace/drive/api/guides/ref-roles)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
