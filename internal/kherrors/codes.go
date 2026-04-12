package kherrors

// Operator runbook
//
// Use this table when diagnosing a failed kh invocation. In --output json
// mode the "error.code" field matches the Code column exactly.
//
// ┌──────────────┬────────────┬──────┬────────────────────────────────────────────────┬───────────────────────────────────────────────┐
// │ Code         │ Category   │ Exit │ Meaning                                        │ Next action                                   │
// ├──────────────┼────────────┼──────┼────────────────────────────────────────────────┼───────────────────────────────────────────────┤
// │ KH-VAL-001   │ validation │  3   │ A required flag or argument is missing         │ Add the missing flag; see kh help <command>   │
// │ KH-VAL-002   │ validation │  3   │ A flag or argument has an invalid value        │ Fix the value; check kh help <command>        │
// │ KH-VAL-003   │ validation │  3   │ Workspace name contains invalid characters     │ Use alphanumeric names with hyphens only      │
// │ KH-VAL-004   │ validation │  3   │ Two mutually exclusive flags were both set     │ Remove one of the conflicting flags           │
// │ KH-VAL-005   │ validation │  3   │ Resource already exists (conflict on create)   │ Use the update command instead                │
// │ KH-AUTH-001  │ auth       │  4   │ No API token is configured                     │ Run kh login or set KH_TOKEN                  │
// │ KH-AUTH-002  │ auth       │  4   │ API token was rejected (HTTP 401)              │ Re-authenticate; check token has not expired  │
// │ KH-PERM-001  │ permission │  4   │ Token lacks permission for this operation (403)│ Request access from your administrator        │
// │ KH-NET-001   │ network    │  5   │ Backend I/O or connection failure              │ Check connectivity and retry                  │
// │ KH-NET-002   │ network    │  5   │ KeyHarbour API returned a server error (5xx)   │ Check server health; retry after a delay      │
// │ KH-NF-001    │ not-found  │  1   │ The requested resource does not exist          │ Verify the UUID or name and check it exists   │
// │ KH-CONF-001  │ conflict   │  6   │ State lock held by another process             │ Wait or force-unlock with kh tf unlock        │
// │ KH-PART-001  │ partial    │  2   │ Some operations in a batch failed              │ Inspect failure details and retry failed items│
// │ KH-INT-001   │ internal   │  1   │ Unexpected internal error                      │ Report at github.com/keyharbour/cli/issues    │
// │ KH-INT-002   │ internal   │  1   │ CLI configuration failed to load               │ Check ~/.kh/config and environment variables  │
// └──────────────┴────────────┴──────┴────────────────────────────────────────────────┴───────────────────────────────────────────────┘

// Validation errors

// ErrMissingFlag is used when a required flag or argument is absent.
var ErrMissingFlag = ErrorDef{
	Code:     "KH-VAL-001",
	Category: CategoryValidation,
	hint:     "run 'kh help <command>' to see required flags",
}

// ErrInvalidValue is used when a flag or argument value is malformed.
var ErrInvalidValue = ErrorDef{
	Code:     "KH-VAL-002",
	Category: CategoryValidation,
	hint:     "check the value format and try again; run 'kh help <command>' for usage",
}

// ErrInvalidWorkspaceName is used when a workspace name contains
// characters that KeyHarbour does not allow.
var ErrInvalidWorkspaceName = ErrorDef{
	Code:     "KH-VAL-003",
	Category: CategoryValidation,
	hint:     "workspace names must be alphanumeric (a-z, 0-9, hyphens)",
}

// ErrConflictingFlags is used when two mutually exclusive flags are both set.
var ErrConflictingFlags = ErrorDef{
	Code:     "KH-VAL-004",
	Category: CategoryValidation,
	hint:     "only one of the conflicting flags may be provided at a time",
}

// ErrResourceConflict is used when a create operation fails because the
// resource already exists.
var ErrResourceConflict = ErrorDef{
	Code:     "KH-VAL-005",
	Category: CategoryValidation,
	hint:     "the resource already exists; use the update command to modify it",
}

// Auth errors

// ErrMissingToken is used when no API token is present in the config or
// environment.
var ErrMissingToken = ErrorDef{
	Code:     "KH-AUTH-001",
	Category: CategoryAuth,
	hint:     "run 'kh login --token <TOKEN>' or set the KH_TOKEN environment variable",
}

// ErrTokenInvalid is used when the API returns HTTP 401.
var ErrTokenInvalid = ErrorDef{
	Code:     "KH-AUTH-002",
	Category: CategoryAuth,
	hint:     "check that your token is valid and has not expired; run 'kh login' to re-authenticate",
}

// Permission errors

// ErrForbidden is used when the API returns HTTP 403.
var ErrForbidden = ErrorDef{
	Code:     "KH-PERM-001",
	Category: CategoryPermission,
	hint:     "your token does not have permission for this operation; contact your administrator",
}

// Network / backend IO errors

// ErrBackendIO is used for connection failures and general I/O errors when
// reading from or writing to a backend.
var ErrBackendIO = ErrorDef{
	Code:     "KH-NET-001",
	Category: CategoryNetwork,
	hint:     "check your network connection and try again",
}

// ErrAPIError is used when the KeyHarbour API returns a 5xx response.
var ErrAPIError = ErrorDef{
	Code:     "KH-NET-002",
	Category: CategoryNetwork,
	hint:     "the KeyHarbour API returned a server error; check server health and retry",
}

// Not found

// ErrNotFound is used when a resource (project, workspace, statefile) does
// not exist.
var ErrNotFound = ErrorDef{
	Code:     "KH-NF-001",
	Category: CategoryNotFound,
	hint:     "verify the UUID or name and confirm the resource exists",
}

// Conflict / lock

// ErrStateLocked is used when another process holds the state lock (HTTP
// 409 or 423).
var ErrStateLocked = ErrorDef{
	Code:     "KH-CONF-001",
	Category: CategoryConflict,
	hint:     "another process holds the state lock; wait for it to finish or force-unlock with 'kh tf unlock'",
}

// Partial failure

// ErrPartialFailure is used when a batch operation succeeds for some items
// but fails for others.
var ErrPartialFailure = ErrorDef{
	Code:     "KH-PART-001",
	Category: CategoryPartial,
	hint:     "check the failure details above and retry the failed items",
}

// Internal errors

// ErrInternal is the fallback for unexpected errors that do not fit any
// other category.
var ErrInternal = ErrorDef{
	Code:     "KH-INT-001",
	Category: CategoryInternal,
	hint:     "this is an unexpected error; please report it at https://github.com/keyharbour/cli/issues",
}

// ErrConfigLoad is used when the CLI configuration cannot be read.
var ErrConfigLoad = ErrorDef{
	Code:     "KH-INT-002",
	Category: CategoryInternal,
	hint:     "check your ~/.kh/config file and environment variables (KH_ENDPOINT, KH_TOKEN)",
}
