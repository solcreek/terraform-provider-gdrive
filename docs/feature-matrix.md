# Feature and verification matrix

This matrix defines the initial release boundary. Implementation status will be updated alongside each feature commit. A passing local test is not evidence of successful authentication or authorization in a real Workspace tenant.

| Capability | Initial release scope |
| --- | --- |
| Shared Drive create, read, rename, delete, import | Required |
| Five standard Shared Drive restriction flags | Required |
| Folder create, read, rename, move within a drive, trash, import | Required |
| Direct user, group, domain, and anyone permissions | Required |
| Permission role updates, discovery setting, delete, import | Required |
| Inherited permission detection | Required |
| Shared Drive, file metadata, and permission discovery | Required |
| ADC, access token, service-account key | Required |
| ADC-backed impersonation and domain-wide delegation | Required |
| Terraform and OpenTofu protocol validation | Required |
| Live Workspace acceptance | Required before production release |
| Download restrictions added to newer Drive APIs | Deferred |
| Labels, organizational units, file uploads | Deferred |
| Permission expiration and ownership transfer | Deferred |
| Authoritative whole-ACL replacement | Out of scope |

The existing Drive URL supplied during evaluation was not accessible. No live Drive object has been inspected, imported, or modified by this project.
