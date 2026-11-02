package usecase

import (
	"context"
	"testing"
)

func TestSSHKeyValidation(t *testing.T) {
	svc := NewSSHKeyUsecase(nil, nil)

	// Test rejection of private keys
	privateKeySample := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACDHbO22e8FkM2j9
-----END OPENSSH PRIVATE KEY-----`

	_, err := svc.CreateKey(context.Background(), [16]byte{}, "test-key", privateKeySample, "")
	if err == nil {
		t.Errorf("Expected error when providing private key, got nil")
	}

	// Test rejection of invalid public key string
	_, err = svc.CreateKey(context.Background(), [16]byte{}, "invalid-key", "this is not an ssh key", "")
	if err == nil {
		t.Errorf("Expected error for malformed key string, got nil")
	}
}
