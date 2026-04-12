// Package kherrors defines the structured error taxonomy for the kh CLI.
//
// Every command failure is represented as a [*KHError] carrying a stable
// machine code (e.g. "KH-AUTH-001"), a human message, and a remediation hint.
// Root [Execute] translates all errors – including raw [khclient.APIError]
// values that bubble up from the HTTP layer – into this type before printing,
// ensuring consistent output in both table and --output json modes.
//
// # Adding a new error site
//
//  1. Pick the closest [ErrorDef] from codes.go.
//  2. Replace exitcodes.With(exitcodes.XxxError, fmt.Errorf("...")) with
//     kherrors.ErrXxx.New("...") or kherrors.ErrXxx.Wrap("...", cause).
//  3. If no existing def fits, add one to codes.go following the naming
//     convention KH-<CATEGORY_PREFIX>-<NNN>.
package kherrors

import (
	"errors"
	"fmt"

	"kh/internal/exitcodes"
)

// Category classifies errors by their operational domain.
type Category string

const (
	CategoryValidation Category = "validation"
	CategoryAuth       Category = "auth"
	CategoryNetwork    Category = "network"
	CategoryPermission Category = "permission"
	CategoryConflict   Category = "conflict"
	CategoryNotFound   Category = "not-found"
	CategoryPartial    Category = "partial"
	CategoryInternal   Category = "internal"
)

// exitCodeForCategory maps each category to its process exit code.
// The mapping preserves the existing exit-code contract documented in
// internal/exitcodes/exitcodes.go.
var exitCodeForCategory = map[Category]int{
	CategoryValidation: exitcodes.ValidationError, // 3
	CategoryAuth:       exitcodes.AuthError,        // 4
	CategoryNetwork:    exitcodes.BackendIOError,   // 5
	CategoryPermission: exitcodes.AuthError,        // 4 – HTTP 403 is auth-adjacent
	CategoryConflict:   exitcodes.LockError,        // 6
	CategoryNotFound:   exitcodes.UnknownError,     // 1 – no dedicated code yet
	CategoryPartial:    exitcodes.Partial,          // 2
	CategoryInternal:   exitcodes.UnknownError,     // 1
}

// KHError is a structured CLI error with a stable machine code.
// All fields except cause are marshaled to JSON so they can be
// embedded in the --output json error envelope.
type KHError struct {
	Code     string   `json:"code"`
	Category Category `json:"category"`
	Message  string   `json:"message"`
	Hint     string   `json:"hint,omitempty"`
	DocsURL  string   `json:"docs_url,omitempty"`
	cause    error    // unexported; not marshaled; available via Unwrap
}

// Error implements the error interface and returns the human message.
func (e *KHError) Error() string { return e.Message }

// ExitCode satisfies [exitcodes.ExitCoder] so root Execute() picks up the
// right process exit code without any special-casing.
func (e *KHError) ExitCode() int {
	if code, ok := exitCodeForCategory[e.Category]; ok {
		return code
	}
	return exitcodes.UnknownError
}

// Unwrap returns the underlying cause for errors.Is / errors.As traversal.
func (e *KHError) Unwrap() error { return e.cause }

// Classify converts any error to a *KHError suitable for structured output.
//   - If err's chain already contains a *KHError, that value is returned.
//   - Otherwise, err is wrapped as KH-INT-001 with its message redacted.
func Classify(err error) *KHError {
	var khErr *KHError
	if errors.As(err, &khErr) {
		return khErr
	}
	return ErrInternal.Wrap(Redact(err.Error()), err)
}

// ErrorDef is a reusable error descriptor. Declare one constant per error
// code in codes.go; use its methods to create *KHError instances at the
// call site.
type ErrorDef struct {
	Code     string
	Category Category
	hint     string
	docsURL  string
}

// New creates a *KHError with message (secrets are redacted automatically).
func (d ErrorDef) New(message string) *KHError {
	return &KHError{
		Code:     d.Code,
		Category: d.Category,
		Message:  Redact(message),
		Hint:     d.hint,
		DocsURL:  d.docsURL,
	}
}

// Newf is like New but accepts a format string.
func (d ErrorDef) Newf(format string, args ...any) *KHError {
	return d.New(fmt.Sprintf(format, args...))
}

// Wrap creates a *KHError with message and preserves cause for Unwrap.
func (d ErrorDef) Wrap(message string, cause error) *KHError {
	e := d.New(message)
	e.cause = cause
	return e
}

// Wrapf is like Wrap with a format string for the message.
func (d ErrorDef) Wrapf(cause error, format string, args ...any) *KHError {
	return d.Wrap(fmt.Sprintf(format, args...), cause)
}
