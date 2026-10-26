resource "gdrive_permission" "engineering" {
  file_id       = "SHARED_DRIVE_ID"
  type          = "group"
  email_address = "engineering@example.com"
  role          = "writer"
}
