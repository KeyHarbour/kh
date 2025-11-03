package exitcodes

const (
	OK              = 0
	Partial         = 2
	ValidationError = 3
	AuthError       = 4
	BackendIOError  = 5
	LockError       = 6
	UnknownError    = 1
)

// ExitCoder allows an error to carry a specific process exit code.
type ExitCoder interface{ ExitCode() int }

// codedError wraps an error with an exit code.
type codedError struct {
	code int
	err  error
}

func (e codedError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return ""
}
func (e codedError) Unwrap() error { return e.err }
func (e codedError) ExitCode() int { return e.code }

// With wraps err to return the provided exit code from root Execute().
func With(code int, err error) error { return codedError{code: code, err: err} }
