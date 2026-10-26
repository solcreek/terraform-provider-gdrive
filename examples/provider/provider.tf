terraform {
  required_providers {
    gdrive = {
      source  = "registry.opentofu.org/solcreek/gdrive"
      version = "~> 0.1"
    }
  }
}

# Use Application Default Credentials.
provider "gdrive" {}

# Keyless domain-wide delegation uses ADC to impersonate a service account,
# then acts as the specified Workspace user.
provider "gdrive" {
  alias                       = "admin"
  impersonate_service_account = "drive-automation@example-project.iam.gserviceaccount.com"
  subject                     = "admin@example.com"
  use_domain_admin_access     = true
}
