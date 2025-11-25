# KeyHarbour Backend Bug: Workspace UUIDs Match Project UUID

## Issue

The KeyHarbour API is returning workspace UUIDs that are identical to the project UUID, causing statefile uploads to fail with 404 errors.

## Evidence

```bash
$ curl -H "Authorization: Bearer $KH_TOKEN" \
  "$KH_ENDPOINT/v1/projects/639d060e-be34-4b1e-9879-1008418886ea/workspaces"

[
    {
        "uuid": "639d060e-be34-4b1e-9879-1008418886ea",
        "name": "workspace"
    },
    {
        "uuid": "639d060e-be34-4b1e-9879-1008418886ea",
        "name": "test-migration"
    }
]
```

Note that both workspaces have UUID `639d060e-be34-4b1e-9879-1008418886ea`, which is the same as the project UUID.

## Impact

When the CLI tries to upload statefiles using the resolved workspace UUID:

```
POST /v1/projects/639d060e-be34-4b1e-9879-1008418886ea/workspaces/639d060e-be34-4b1e-9879-1008418886ea/statefiles
```

The API returns 404 Not Found, even though:
- The project UUID is valid (GET /v1/projects/{uuid} returns 200)
- The workspace is listed (GET /v1/projects/{uuid}/workspaces returns the workspace)
- The authentication token is valid

## Expected Behavior

Each workspace should have its own unique UUID, distinct from the project UUID. For example:

```json
[
    {
        "uuid": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
        "name": "workspace"
    },
    {
        "uuid": "ffffffff-gggg-hhhh-iiii-jjjjjjjjjjjj",
        "name": "test-migration"
    }
]
```

## Workaround

None available. The issue must be fixed on the KeyHarbour backend.

## Related Test

`integration-tests/tfc_to_kh_migration_test.go` fails at the statefile upload step with:

```
failed to upload statefile: create statefile: api error (404): Not Found
```

## Date Identified

November 24, 2025
