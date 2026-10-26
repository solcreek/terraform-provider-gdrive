# Implementation standards and evidence

This document maps implementation choices to official guidance. It is not a certification by HashiCorp, OpenTofu, or Google.

| Requirement | Implementation and verification |
| --- | --- |
| Use maintained provider tooling | Terraform Plugin Framework v1.19.0 and protocol 6; both CLIs run lifecycle tests through a real provider server. |
| Preserve plan/state consistency | Immutable IDs use `UseStateForUnknown`; principal identity changes require replacement. Optional computed Drive restrictions retain server defaults. Protocol tests check post-apply plans and import verification. |
| Respect null, unknown, and explicit false | Framework typed values are used throughout. Unknown restrictions are omitted from requests; explicit false uses the SDK's `ForceSendFields`. Empty folder descriptions are sent explicitly. |
| Resource disappearance vs. API failure | Resource reads remove state on 404; 403 and server failures preserve state and return diagnostics. Data sources report missing objects as errors. Drive may return 404 for inaccessible objects: verify access before applying a recreation plan. |
| Standard imports | Drive/folder IDs and permission `file_id/permission_id` imports populate state through API reads. Importing a nonfolder or Shared Drive root into the folder resource is rejected. |
| Use the official, current SDK | `google.golang.org/api/drive/v3`, module v0.308.0, checked against `@latest` on October 26, 2026. Versions are pinned; Dependabot proposes updates rather than changing builds implicitly. |
| Explicit Shared Drive support | Supported file/permission calls set `supportsAllDrives=true`. Domain administrator flags are passed only to operations that support them. Permission listing consumes every page. |
| Bounded, safe retries | Retry eligible 403 quota errors, 429, and 500/502/503/504 with bounded exponential backoff, jitter, and Retry-After. Ordinary 403/404 are not retried. Drive creation reuses a request ID within one operation. Folder/permission creation and folder moves are not automatically replayed. |
| Cancellation and credential lifetime | Resource calls use their request context; HTTP requests have a 60-second timeout. Token refresh uses a detached Configure context with its own bounded HTTP client. Tests cancel Configure before requesting a token. |
| Permission inheritance | A managed direct role is distinguished from the effective inherited role. Inherited-only permissions are not updated or deleted. Existing direct permissions require import. |
| Concurrent permission writes | A provider-client mutex serializes permission preflight and mutation. Separate aliases/processes cannot share this lock: one state owner must manage an item's ACL, and external changes must be coordinated. |
| Least disclosure | No user-configurable API endpoint. Explicit credential files must be service-account keys with a Google token endpoint. Sensitive tokens are never resource state fields. Diagnostics omit free-form API/authentication response bodies. |
| Deletion semantics | Protection defaults to true and is enforced on delete. Drives must be empty; recursive Drive deletion is never enabled. Folder deletion means trashing the folder and its descendants, not permanently deleting them. |
| Reproducible distribution | Signed checksum files, protocol manifest, pinned action revisions, dependency license checks, and license/source bundles accompany releases. |

## Official references

- [Framework plan modification and consistency rules](https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification)
- [Framework read lifecycle](https://developer.hashicorp.com/terraform/plugin/framework/resources/read)
- [Framework import](https://developer.hashicorp.com/terraform/plugin/framework/resources/import)
- [OpenTofu provider development and acceptance-test settings](https://opentofu.org/docs/language/providers/#how-to-develop-providers)
- [Drive shared-drive integration](https://developers.google.com/workspace/drive/api/guides/enable-shareddrives)
- [Drive sharing and concurrent permission operations](https://developers.google.com/workspace/drive/api/guides/manage-sharing)
- [Drive error handling](https://developers.google.com/workspace/drive/api/guides/handle-errors)
- [Shared Drive creation and request IDs](https://developers.google.com/workspace/drive/api/guides/manage-shareddrives)
- [Official Drive v3 Go SDK](https://pkg.go.dev/google.golang.org/api/drive/v3)

## Limits of local evidence

An HTTP fixture verifies our requests, state transitions, and error handling, but cannot establish Google's actual server behavior, Workspace policy behavior, or tenant authorization. Live Drive lifecycle, restrictions, folder moves, permission inheritance, and keyless domain-wide delegation must be verified in a dedicated Workspace before production use. The current live test covers empty Drive CRUD/import only; the remaining live scenarios are manual release gates.

A request ID is stable across retries within a Drive creation RPC, not across a crashed process and a fresh apply. After an interrupted creation, inspect Drive and import the result before retrying. Google does not provide an idempotency token for permission creation or Shared Drive folder creation.
