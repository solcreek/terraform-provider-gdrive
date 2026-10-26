resource "gdrive_folder" "documents" {
  name                = "Documents"
  parent_id           = "SHARED_DRIVE_ID"
  description         = "Engineering documentation"
  deletion_protection = true
}
