package secret

import "testing"

func TestRoundTrip(t *testing.T) {
	c := New("a long deployment secret that only exists here")
	enc, err := c.Encrypt("s3cr3t-value")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if enc == "s3cr3t-value" {
		t.Fatal("ciphertext equals plaintext")
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if dec != "s3cr3t-value" {
		t.Fatalf("got %q", dec)
	}
}

func TestWrongKey(t *testing.T) {
	enc, err := New("key one").Encrypt("value")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := New("key two").Decrypt(enc); err == nil {
		t.Fatal("decrypt with another key succeeded")
	}
}

func TestDisabled(t *testing.T) {
	c := New("")
	if c.Enabled() {
		t.Fatal("empty master produced an enabled cipher")
	}
	if _, err := c.Encrypt("x"); err == nil {
		t.Fatal("encrypt without a key succeeded")
	}
	if _, err := c.Decrypt("x"); err == nil {
		t.Fatal("decrypt without a key succeeded")
	}
}
