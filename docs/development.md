# Development

Use Go 1.26 or later; local verification used Go 1.27.3. Terraform 1.5.7 and OpenTofu 1.12.5 are the current protocol-test baselines. All comments, diagnostics, and documentation use US English. Commit small, independently reviewable changes with appropriate checks.

```sh
make build           # bin/terraform-provider-gdrive
make test            # local HTTP/unit tests, no credentials or CLI required
make test-terraform  # full protocol lifecycle tests against a local API fixture
make test-tofu       # same suite through OpenTofu
make vet
make fmt-check
make generate        # generated registry docs from actual schema and examples
make license-check   # runtime dependency license allowlist
make licenses        # dependency notices/source for release archives
make vulncheck       # symbol-level vulnerability analysis
```

Tool versions are pinned in the Makefile; GitHub Actions are pinned to commit IDs. `go.mod` and `go.sum` pin module versions. The Google SDK was resolved against `@latest` on October 26, 2026: `google.golang.org/api v0.308.0` (Drive API v3). Dependabot proposes dependency updates weekly.

## Local provider installation

Create a temporary CLI configuration with your absolute build path:

```hcl
provider_installation {
  dev_overrides {
    "registry.opentofu.org/solcreek/gdrive" = "/absolute/path/to/terraform-provider-gdrive/bin"
  }
  direct {}
}
```

Set `TF_CLI_CONFIG_FILE` to that file for both CLIs. With a development override, run `validate` or `plan` directly; `init` still attempts registry resolution. Do not commit your personal CLI configuration. `validate` does not contact Google or verify authorization.

## Protocol tests

The suite uses Terraform Plugin Testing and an in-process HTTP fixture. Tests include create/read/update/delete, post-apply plan consistency, import verification, explicit false restrictions, empty descriptions, same-drive moves, and preservation of unrelated ACL entries. Fixture endpoints can only be injected from Go tests, not provider configuration.

OpenTofu requires the testing framework's host and namespace environment variables. `scripts/test-protocol.sh` sets them explicitly; merely pointing `TF_ACC_TERRAFORM_PATH` at OpenTofu is not sufficient.

## Live acceptance

No live API tests run in regular CI. Drive has no equivalent of a disposable provider sandbox, so use a dedicated Workspace and an authorized test identity. The following explicitly creates, renames, imports, and deletes an empty Shared Drive:

```sh
TF_ACC=1 GDRIVE_ACC_CREATE_DRIVE=1 \
TF_ACC_TERRAFORM_PATH="$(command -v tofu)" \
TF_ACC_PROVIDER_NAMESPACE=solcreek \
TF_ACC_PROVIDER_HOST=registry.opentofu.org \
go test -count=1 -run '^TestAccSharedDrive$' -v ./internal/provider
```

Credentials come from the documented provider environment variables or ADC. Live acceptance was not run during initial implementation. Folder, permission, restriction, and keyless delegation behavior still requires dedicated-tenant verification before production use.

## Release

The release layout follows `solcreek/terraform-provider-gandi`: per-platform ZIPs, SHA256SUMS with detached GPG signature, and a protocol-6 manifest. Release archives also include third-party license notices and source when required. A release tag runs the same CI checks before producing a draft GitHub release; publish the draft only after verifying assets and signatures.

The signing fingerprint must match the key already registered for Solcreek in the OpenTofu registry: `2DF6EA201129324B19FFCCD6CF721E16E07047D5`. Never replace it without updating the registry key registration.

The initial experimental release documents the outstanding live tests. A production-ready release must pass those live gates. Once a version is published and indexed, do not move its tag or replace artifacts; issue a new version instead.

The first OpenTofu registration must be submitted using [the registry's GitHub issue form](https://github.com/opentofu/registry/issues/new?template=provider.yml&title=Provider%3A%20solcreek%2Fgdrive&repository=solcreek%2Fterraform-provider-gdrive). The form's processing pipeline explicitly requires the browser form rather than an API-created issue. Subsequent releases are discovered automatically after registration.
