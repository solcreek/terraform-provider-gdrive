terraform {
  required_version = ">= 1.5.0"
  required_providers {
    gdrive = {
      source  = "registry.opentofu.org/solcreek/gdrive"
      version = "~> 0.1"
    }
  }
}

provider "gdrive" {
  impersonate_service_account = var.service_account
  subject                     = var.workspace_admin
  use_domain_admin_access     = true
}

variable "service_account" {
  description = "Service account authorized for Workspace domain-wide delegation. ADC supplies the source identity."
  type        = string
}

variable "workspace_admin" {
  description = "Workspace administrator to impersonate. Folder operations also require content access."
  type        = string
}

variable "drive_name" {
  description = "For import, use the existing Shared Drive name."
  type        = string
}

variable "group_roles" {
  description = "Existing Google Group email addresses mapped to Shared Drive roles."
  type        = map(string)
  default     = {}
  validation {
    condition     = alltrue([for role in values(var.group_roles) : contains(["organizer", "fileOrganizer", "writer", "commenter", "reader"], role)])
    error_message = "Use organizer, fileOrganizer, writer, commenter, or reader."
  }
}

resource "gdrive_shared_drive" "this" {
  name                = var.drive_name
  deletion_protection = true
}

resource "gdrive_permission" "group" {
  for_each      = var.group_roles
  file_id       = gdrive_shared_drive.this.id
  type          = "group"
  email_address = each.key
  role          = each.value
}

output "drive_id" {
  value = gdrive_shared_drive.this.id
}
