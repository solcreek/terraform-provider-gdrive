# Authentication

The provider requests the `https://www.googleapis.com/auth/drive` OAuth scope. Workspace administrators must separately authorize domain-wide delegation when a service account acts on behalf of a Workspace user.

IAM service-account impersonation and Workspace domain-wide delegation are different permission boundaries. IAM access alone does not grant access to Drive content. Domain administrator access applies only to supported Drive and permission methods; folder operations still require content access.

Use ADC or keyless impersonation where possible. Never commit access tokens, service-account keys, Terraform state, or plan files. Credentials and endpoint behavior will be documented alongside the authentication implementation.
