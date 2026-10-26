resource "gdrive_shared_drive" "engineering" {
  name                = "Engineering"
  domain_users_only   = true
  drive_members_only  = true
  deletion_protection = true
}
