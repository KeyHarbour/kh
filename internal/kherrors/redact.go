package kherrors

import "regexp"

// redactPattern pairs a compiled regular expression with a replacement
// template. Capture groups in the regex are referenced as ${N} in the
// replacement so only the secret value is masked while surrounding context
// is preserved.
type redactPattern struct {
	re          *regexp.Regexp
	replacement string
}

// sensitivePatterns covers the most common ways a secret can appear in an
// error message:
//
//   - Environment-variable assignments: KH_TOKEN=abc, TF_API_TOKEN=abc
//   - HTTP Bearer credentials: "Bearer eyJ…"
//   - URL embedded passwords: https://user:pass@host
var sensitivePatterns = []redactPattern{
	{
		// KH_TOKEN=value, TF_API_TOKEN=value, TFC_TOKEN=value,
		// TF_TOKEN_app_terraform_io=value, KH_ENCRYPTION_KEY=value,
		// password=value (case-insensitive)
		re: regexp.MustCompile(
			`(?i)((?:kh_token|kh_encryption_key(?:_file)?|tf_api_token|tfc_token|tf_token_\w+|password)\s*=\s*)\S+`,
		),
		replacement: "${1}[REDACTED]",
	},
	{
		// "Bearer eyJhbGc..." or "Bearer abc123def456" (≥8 chars)
		re:          regexp.MustCompile(`(?i)(Bearer\s+)[A-Za-z0-9\-_.+/=]{8,}`),
		replacement: "${1}[REDACTED]",
	},
	{
		// https://user:password@host  →  https://user:[REDACTED]@host
		re:          regexp.MustCompile(`(https?://[^:\s@/]+:)[^@\s]+(@)`),
		replacement: "${1}[REDACTED]${2}",
	},
}

// Redact strips known secret patterns from s and returns the sanitised
// string. It is applied automatically by all ErrorDef constructors so
// callers do not need to pre-sanitise error messages.
func Redact(s string) string {
	for _, p := range sensitivePatterns {
		s = p.re.ReplaceAllString(s, p.replacement)
	}
	return s
}
