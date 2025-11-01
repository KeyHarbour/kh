package state

import "testing"

func TestSHA256Hex(t *testing.T) {
	cases := []struct {
		in   []byte
		want string
	}{
		{[]byte(""), "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{[]byte("hello"), "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
	}
	for _, tc := range cases {
		got := SHA256Hex(tc.in)
		if got != tc.want {
			t.Fatalf("SHA256Hex(%q) = %s, want %s", string(tc.in), got, tc.want)
		}
	}
}
