# Feature and verification matrix

The initial release focuses on Shared Drive administration. It does not claim full Drive API coverage. Local protocol tests use the actual Terraform/OpenTofu engines with a simulated Google API; they do not establish real Workspace authorization or server behavior.

| Capability | Implementation | Verification |
| --- | --- | --- |
| Shared Drive create/read/rename/delete/import | Implemented | Local HTTP and both CLI protocol tests |
| Five standard Drive restriction flags | Implemented | Explicit true/false and state tests; live policy testing pending |
| Folder create/read/rename/description/import | Implemented | Local tests and both CLIs |
| Same-drive folder moves | Implemented | Parent changes and cross-drive rejection tests |
| Folder deletion | Protected trash operation | Local tests; descendants follow Google's trash semantics |
| Direct user/group/domain/anyone permissions | Implemented | All four principal types tested through both CLI protocols |
| Role and discovery changes | Implemented | Role lifecycle and explicit request fields |
| Existing and inherited permissions | Import/preflight and inheritance detection | Local preservation and inherited-delete tests |
| Drive/file/permission data sources | Implemented | Both CLIs; paginated permission HTTP test |
| ADC, access token, service-account key | Implemented | Local configuration and token tests; live tenant pending |
| Keyless impersonation and domain-wide delegation | Official SDK integration | Live tenant pending |
| Live empty Shared Drive lifecycle | Opt-in test provided | Not run |
| Newer nested download restriction fields | Deferred | Not implemented |
| Labels, organizational units, file uploads | Deferred | Not implemented |
| Permission expiration, limited-access folders, ownership transfer | Deferred | Not implemented |
| Cross-drive moves and whole-ACL replacement | Out of scope | Rejected or not exposed |

No live Drive object has been inspected, imported, or modified during initial implementation. An inaccessible `/drive/folders/` URL cannot be assumed to identify a Shared Drive root; use the `gdrive_file` data source to inspect `drive_id` and `mime_type` first.

See [implementation standards](best-practices.md) for the official guidance, design decisions, and evidence limits.
