# Contributor instructions

- Write all repository documentation, comments, examples, and diagnostics in US English.
- License original source under MPL-2.0. Preserve dependency licenses and notices, and review license compatibility before adding code or dependencies.
- Use the official Google Drive API v3 Go SDK. Pin dependency versions and verify updates against official releases.
- Follow Terraform Plugin Framework plan/state consistency, import, and diagnostic conventions. Test Terraform and OpenTofu behavior through the provider protocol.
- Keep commits atomic and commit completed, verified work frequently.
- Run the relevant unit or protocol tests before each code commit. Run `make generate` when schema or examples change.
- Do not claim live Workspace verification from local fixtures. Real API tests require explicit opt-in and a dedicated test environment.
- Follow the Solcreek Gandi provider's OpenTofu Registry release layout. Preserve published version tags and the registered Solcreek signing identity.
