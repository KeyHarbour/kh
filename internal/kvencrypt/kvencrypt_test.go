package kvencrypt

import (
	"strings"
	"testing"
)

var testKey = [32]byte{
	0xa3, 0xf1, 0x2e, 0x84, 0x9c, 0x47, 0xb0, 0x11,
	0x23, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x12,
	0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x12,
	0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x12,
}

var wrongKey = [32]byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
	0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := "super-secret-value"
	enc, err := Encrypt(testKey, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	got, err := Decrypt(testKey, enc)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if got != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, got)
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	plaintext := "same-input"
	enc1, _ := Encrypt(testKey, plaintext)
	enc2, _ := Encrypt(testKey, plaintext)
	if enc1 == enc2 {
		t.Fatal("expected different ciphertexts due to random nonce")
	}
}

func TestEncryptHasPrefix(t *testing.T) {
	enc, err := Encrypt(testKey, "hello")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if !strings.HasPrefix(enc, "enc:v1:") {
		t.Fatalf("expected enc:v1: prefix, got: %s", enc)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	enc, _ := Encrypt(testKey, "secret")
	_, err := Decrypt(wrongKey, enc)
	if err == nil {
		t.Fatal("expected error with wrong key")
	}
	if !strings.Contains(err.Error(), "decryption failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestDecryptTruncatedCiphertext(t *testing.T) {
	_, err := Decrypt(testKey, "enc:v1:dG9vc2hvcnQ")
	if err == nil {
		t.Fatal("expected error for truncated ciphertext")
	}
}

func TestDecryptBitFlipped(t *testing.T) {
	enc, _ := Encrypt(testKey, "secret")
	// Flip a byte in the base64 payload by replacing a char
	flipped := enc[:len("enc:v1:")+5] + "X" + enc[len("enc:v1:")+6:]
	_, err := Decrypt(testKey, flipped)
	if err == nil {
		t.Fatal("expected error for bit-flipped ciphertext")
	}
}

func TestDecryptNotEncryptedFormat(t *testing.T) {
	_, err := Decrypt(testKey, "plaintext-value")
	if err == nil {
		t.Fatal("expected error for non-encrypted value")
	}
}

func TestIsEncrypted(t *testing.T) {
	enc, _ := Encrypt(testKey, "hello")
	if !IsEncrypted(enc) {
		t.Fatal("expected IsEncrypted=true for encrypted value")
	}
	if IsEncrypted("plaintext") {
		t.Fatal("expected IsEncrypted=false for plaintext")
	}
	if IsEncrypted("") {
		t.Fatal("expected IsEncrypted=false for empty string")
	}
}

func TestParseKey_Valid(t *testing.T) {
	hex64 := strings.Repeat("ab", 32) // 64 hex chars
	key, err := ParseKey(hex64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key[0] != 0xab {
		t.Fatalf("unexpected key byte: %x", key[0])
	}
}

func TestParseKey_TooShort(t *testing.T) {
	_, err := ParseKey(strings.Repeat("a", 63))
	if err == nil {
		t.Fatal("expected error for 63-char key")
	}
	if !strings.Contains(err.Error(), "64 hex characters") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestParseKey_NonHex(t *testing.T) {
	_, err := ParseKey(strings.Repeat("zz", 32))
	if err == nil {
		t.Fatal("expected error for non-hex key")
	}
	if !strings.Contains(err.Error(), "valid hex string") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
