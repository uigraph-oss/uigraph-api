package crypto

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	c, err := NewCipher("dev-secret-key-32-bytes-long-!!!")
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	plaintext := "figd_secret-access-token-value"
	enc, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc == plaintext {
		t.Fatal("ciphertext equals plaintext")
	}
	got, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != plaintext {
		t.Fatalf("round trip mismatch: got %q want %q", got, plaintext)
	}
}

func TestDecryptTampered(t *testing.T) {
	c, _ := NewCipher("dev-secret-key-32-bytes-long-!!!")
	enc, _ := c.Encrypt("hello")
	tampered := enc[:len(enc)-2] + "00"
	if _, err := c.Decrypt(tampered); err == nil {
		t.Fatal("expected error decrypting tampered ciphertext")
	}
}

func TestNewCipherEmptySecret(t *testing.T) {
	if _, err := NewCipher(""); err == nil {
		t.Fatal("expected error for empty secret")
	}
}
