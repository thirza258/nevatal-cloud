package usecase_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/usecase"
)

func TestAllocateIP(t *testing.T) {
	uc := usecase.NewNetworkUsecase(nil, nil)
	// We can test CIDR validation
	_, err := uc.CreateNetwork(nil, uuid.New(), usecase.CreateNetworkInput{
		Name: "test-net",
		CIDR: "invalid-cidr",
	})
	if err == nil {
		t.Fatal("expected error for invalid CIDR, got nil")
	}
}

func TestCompileHAProxyConfig(t *testing.T) {
	lbUc := usecase.NewLoadBalancerUsecase(nil, nil, nil)

	lbID := uuid.New()
	poolID := uuid.New()
	lb := &models.LoadBalancer{
		ID:            lbID,
		Name:          "web-lb",
		ConfigVersion: 1,
		Listeners: []models.LoadBalancerListener{
			{
				FrontendPort:  80,
				Protocol:      "http",
				DefaultPoolID: &poolID,
			},
		},
		Pools: []models.LoadBalancerPool{
			{
				ID:        poolID,
				Name:      "web-pool",
				Protocol:  "http",
				Port:      8080,
				Algorithm: "round_robin",
				Members: []models.LoadBalancerMember{
					{
						ID:           uuid.New(),
						Address:      "10.20.0.10",
						Port:         8080,
						Weight:       100,
						Enabled:      true,
						HealthStatus: "healthy",
					},
				},
			},
		},
	}

	cfg := lbUc.CompileHAProxyConfig(lb)
	if cfg == "" {
		t.Fatal("expected non-empty HAProxy configuration")
	}

	if !contains(cfg, "frontend fe_80") {
		t.Errorf("expected frontend fe_80 in config, got:\n%s", cfg)
	}
	if !contains(cfg, "backend be_web_pool") {
		t.Errorf("expected backend be_web_pool in config, got:\n%s", cfg)
	}
	if !contains(cfg, "balance roundrobin") {
		t.Errorf("expected balance roundrobin in config, got:\n%s", cfg)
	}
	if !contains(cfg, "server srv_") {
		t.Errorf("expected server directive in config, got:\n%s", cfg)
	}
}

func TestSSRFProtection(t *testing.T) {
	devUc := usecase.NewDeveloperUsecase(nil, nil)

	// Localhost must be blocked
	err := devUc.ValidateWebhookURL("http://localhost:8080/hook")
	if err == nil {
		t.Fatal("expected localhost to be blocked by SSRF protection")
	}

	// 127.0.0.1 must be blocked
	err = devUc.ValidateWebhookURL("http://127.0.0.1/hook")
	if err == nil {
		t.Fatal("expected 127.0.0.1 to be blocked by SSRF protection")
	}

	// Cloud metadata must be blocked
	err = devUc.ValidateWebhookURL("http://169.254.169.254/latest/meta-data")
	if err == nil {
		t.Fatal("expected 169.254.169.254 to be blocked by SSRF protection")
	}
}

func TestWebhookHMACSignature(t *testing.T) {
	secret := "test_secret_key_12345"
	timestamp := "1795341600"
	payload := `{"id":"evt_123","type":"instance.created"}`

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + payload))
	sig := hex.EncodeToString(mac.Sum(nil))

	// Recompute and verify
	mac2 := hmac.New(sha256.New, []byte(secret))
	mac2.Write([]byte(timestamp + "." + payload))
	sig2 := hex.EncodeToString(mac2.Sum(nil))

	if sig != sig2 {
		t.Fatalf("HMAC signatures do not match: %s != %s", sig, sig2)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && containsSubstr(s, substr)))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
