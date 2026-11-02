package config

import (
	"strings"
	"testing"
)

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	keys := []string{"APP_ENV", "ENV", "DEMO_MODE", "JWT_SECRET", "APP_BASE_URL", "CORS_ALLOWED_ORIGINS",
		"ADMIN_EMAIL", "ADMIN_PASSWORD", "PROXMOX_MOCK", "PROXMOX_HOST", "PROXMOX_TOKEN_ID",
		"PROXMOX_TOKEN_SECRET", "WORDPRESS_API_KEY", "COOKIE_SECURE", "TRUSTED_PROXIES",
		"PUBLIC_DOMAIN_ROOT", "RESERVED_SUBDOMAINS", "MAX_DOMAINS_PER_USER", "DOMAIN_QUARANTINE_DAYS",
		"EDGE_TARGET_CIDRS", "EDGE_DENY_IPS", "EDGE_PORT"}
	for _, k := range keys {
		t.Setenv(k, "")
	}
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

const strongSecret = "0123456789abcdef0123456789abcdef0123456789abcdef"

func productionEnv(extra map[string]string) map[string]string {
	env := map[string]string{
		"APP_ENV":              "production",
		"DEMO_MODE":            "false",
		"JWT_SECRET":           strongSecret,
		"APP_BASE_URL":         "https://cloud.example.com",
		"ADMIN_EMAIL":          "ops@example.com",
		"ADMIN_PASSWORD":       "a-long-admin-password",
		"PROXMOX_MOCK":         "false",
		"PROXMOX_HOST":         "10.0.0.2",
		"PROXMOX_TOKEN_ID":     "nevatal@pve!api",
		"PROXMOX_TOKEN_SECRET": "secret",
	}
	for k, v := range extra {
		env[k] = v
	}
	return env
}

func TestDevelopmentDefaults(t *testing.T) {
	setEnv(t, nil)
	cfg := Load()
	if cfg.IsProduction() || !cfg.DemoMode || !cfg.ProxmoxMock {
		t.Fatalf("development should default to demo + mock, got prod=%v demo=%v mock=%v", cfg.IsProduction(), cfg.DemoMode, cfg.ProxmoxMock)
	}
	if cfg.JWTSecret != DevJWTSecret {
		t.Fatalf("development should fall back to the dev JWT secret")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development config must validate: %v", err)
	}
}

func TestLegacyEnvVariableStillWorks(t *testing.T) {
	setEnv(t, productionEnv(map[string]string{"APP_ENV": "", "ENV": "production"}))
	if cfg := Load(); !cfg.IsProduction() {
		t.Fatalf("ENV=production must still select production")
	}
}

func TestProductionValid(t *testing.T) {
	setEnv(t, productionEnv(nil))
	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid production config, got %v", err)
	}
	if cfg.ProxmoxMock || cfg.DemoMode {
		t.Fatalf("production must use the real hypervisor and no demo mode")
	}
	if !cfg.CookieSecure {
		t.Fatalf("https APP_BASE_URL must enable secure cookies")
	}
	if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "https://cloud.example.com" {
		t.Fatalf("CORS must default to the APP_BASE_URL origin, got %v", cfg.CORSAllowedOrigins)
	}
}

func TestProductionRejectsUnsafeSettings(t *testing.T) {
	cases := map[string]struct {
		env  map[string]string
		want string
	}{
		"missing jwt":        {map[string]string{"JWT_SECRET": ""}, "JWT_SECRET is required"},
		"default jwt":        {map[string]string{"JWT_SECRET": DevJWTSecret}, "public development default"},
		"short jwt":          {map[string]string{"JWT_SECRET": "short"}, "at least 32"},
		"missing admin":      {map[string]string{"ADMIN_EMAIL": ""}, "ADMIN_EMAIL is required"},
		"short admin pw":     {map[string]string{"ADMIN_PASSWORD": "short"}, "ADMIN_PASSWORD is required"},
		"demo admin pw":      {map[string]string{"ADMIN_PASSWORD": DemoAdminPassword}, "public demo passwords"},
		"incomplete proxmox": {map[string]string{"PROXMOX_TOKEN_SECRET": ""}, "PROXMOX_MOCK=false requires"},
		"wildcard cors":      {map[string]string{"CORS_ALLOWED_ORIGINS": "*"}, "explicit origins"},
		"relative base url":  {map[string]string{"APP_BASE_URL": "cloud.example.com"}, "APP_BASE_URL"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			setEnv(t, productionEnv(tc.env))
			err := Load().Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestDemoModeForcesMockHypervisor(t *testing.T) {
	setEnv(t, productionEnv(map[string]string{"DEMO_MODE": "true", "ADMIN_EMAIL": "", "ADMIN_PASSWORD": ""}))
	cfg := Load()
	if !cfg.ProxmoxMock {
		t.Fatalf("DEMO_MODE must force the simulated hypervisor")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("demo deployment (production hardening, demo accounts) must validate: %v", err)
	}
	if len(cfg.Warnings) == 0 {
		t.Fatalf("forcing mock mode must emit a warning")
	}
}

func TestProductionDropsLegacyWordPressKey(t *testing.T) {
	setEnv(t, productionEnv(map[string]string{"WORDPRESS_API_KEY": legacyWordPressAPIKey}))
	cfg := Load()
	if cfg.WordPressEnabled() {
		t.Fatalf("the public legacy WordPress key must disable the integration in production")
	}

	setEnv(t, nil)
	if !Load().WordPressEnabled() {
		t.Fatalf("development keeps the bundled plugin key working")
	}
}
