package config

import "testing"

func TestNormalizeEndpoint(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://app.keyharbour.ca", "https://app.keyharbour.ca/api/v2"},
		{"https://app.keyharbour.ca/", "https://app.keyharbour.ca/api/v2"},
		{"https://app.keyharbour.ca/api/v2", "https://app.keyharbour.ca/api/v2"},
		{"https://app.keyharbour.ca/api/v2/", "https://app.keyharbour.ca/api/v2"},
		{"https://infra.acme.com/kh", "https://infra.acme.com/kh/api/v2"},
		{"https://infra.acme.com/kh/api/v2", "https://infra.acme.com/kh/api/v2"},
	}
	for _, tc := range tests {
		got := normalizeEndpoint(tc.input)
		if got != tc.want {
			t.Errorf("normalizeEndpoint(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
