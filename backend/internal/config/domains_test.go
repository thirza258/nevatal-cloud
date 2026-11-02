package config

import (
	"net/netip"
	"slices"
	"strings"
	"testing"
)

func TestDomainsDisabledByDefault(t *testing.T) {
	setEnv(t, productionEnv(nil))
	cfg := Load()
	if cfg.DomainsEnabled() || cfg.PublicDomainRoot != "" {
		t.Fatalf("without PUBLIC_DOMAIN_ROOT the feature must be off, got root %q", cfg.PublicDomainRoot)
	}
	if cfg.MaxDomainsPerUser != 5 || cfg.DomainQuarantineDays != 14 || cfg.EdgePort != "8088" {
		t.Fatalf("unexpected defaults: max=%d quarantine=%d port=%q", cfg.MaxDomainsPerUser, cfg.DomainQuarantineDays, cfg.EdgePort)
	}
	if len(cfg.EdgeTargetCIDRs) != 0 {
		t.Fatalf("EDGE_TARGET_CIDRS must default to empty (route nothing), got %v", cfg.EdgeTargetCIDRs)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("production without domains must validate: %v", err)
	}
}

func TestDomainSettingsParsed(t *testing.T) {
	setEnv(t, productionEnv(map[string]string{
		"PUBLIC_DOMAIN_ROOT":     " .Nevatal.ID. ",
		"RESERVED_SUBDOMAINS":    "Shop, billing ,  ",
		"MAX_DOMAINS_PER_USER":   "9",
		"DOMAIN_QUARANTINE_DAYS": "0",
		"EDGE_TARGET_CIDRS":      "10.10.42.0/24, 192.168.50.7",
		"EDGE_DENY_IPS":          "10.10.42.1,::ffff:10.10.42.2",
		"EDGE_PORT":              "9099",
		"APP_BASE_URL":           "https://control.nevatal.id",
		"PROXMOX_HOST":           "10.10.42.3",
	}))
	cfg := Load()
	if cfg.PublicDomainRoot != "nevatal.id" || !cfg.DomainsEnabled() {
		t.Fatalf("root must be lowercased and trimmed, got %q enabled=%v", cfg.PublicDomainRoot, cfg.DomainsEnabled())
	}
	if cfg.MaxDomainsPerUser != 9 || cfg.DomainQuarantineDays != 0 || cfg.EdgePort != "9099" {
		t.Fatalf("unexpected values: max=%d quarantine=%d port=%q", cfg.MaxDomainsPerUser, cfg.DomainQuarantineDays, cfg.EdgePort)
	}
	for _, name := range []string{"www", "cloud", "admin", "autoconfig", "shop", "billing", "control"} {
		if !slices.Contains(cfg.ReservedSubdomains, name) {
			t.Errorf("%q must be reserved, got %v", name, cfg.ReservedSubdomains)
		}
	}
	wantCIDRs := []netip.Prefix{netip.MustParsePrefix("10.10.42.0/24"), netip.MustParsePrefix("192.168.50.7/32")}
	if !slices.Equal(cfg.EdgeTargetCIDRs, wantCIDRs) {
		t.Fatalf("EDGE_TARGET_CIDRS = %v, want %v", cfg.EdgeTargetCIDRs, wantCIDRs)
	}
	wantDeny := []netip.Addr{netip.MustParseAddr("10.10.42.1"), netip.MustParseAddr("10.10.42.2"), netip.MustParseAddr("10.10.42.3")}
	if !slices.Equal(cfg.EdgeDenyIPs, wantDeny) {
		t.Fatalf("EDGE_DENY_IPS (+ PROXMOX_HOST) = %v, want %v", cfg.EdgeDenyIPs, wantDeny)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid production config, got %v", err)
	}
	if err := cfg.ValidateEdge(); err != nil {
		t.Fatalf("expected valid edge config, got %v", err)
	}
}

func TestDemoModeForcesDomainsOff(t *testing.T) {
	setEnv(t, productionEnv(map[string]string{"DEMO_MODE": "true", "PUBLIC_DOMAIN_ROOT": "nevatal.id"}))
	cfg := Load()
	if cfg.DomainsEnabled() {
		t.Fatalf("DEMO_MODE must disable Nevatal Domains")
	}
	if !slices.ContainsFunc(cfg.Warnings, func(w string) bool { return strings.Contains(w, "disables Nevatal Domains") }) {
		t.Fatalf("forcing domains off must emit a warning, got %v", cfg.Warnings)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("demo deployment with a root set must still validate: %v", err)
	}
	if err := cfg.ValidateEdge(); err == nil || !strings.Contains(err.Error(), "DEMO_MODE must be false") {
		t.Fatalf("the edge must refuse demo mode, got %v", err)
	}
}

func TestProductionDomainValidation(t *testing.T) {
	cases := map[string]struct {
		env  map[string]string
		want string
	}{
		"root without cidrs": {map[string]string{"PUBLIC_DOMAIN_ROOT": "nevatal.id"}, "EDGE_TARGET_CIDRS is required"},
		"bad cidr":           {map[string]string{"PUBLIC_DOMAIN_ROOT": "nevatal.id", "EDGE_TARGET_CIDRS": "10.10.42.0/24,10.10.42.0/99"}, `"10.10.42.0/99" is not a CIDR`},
		"bad deny ip":        {map[string]string{"PUBLIC_DOMAIN_ROOT": "nevatal.id", "EDGE_TARGET_CIDRS": "10.10.42.0/24", "EDGE_DENY_IPS": "gateway"}, `"gateway" is not an IP`},
		"bad root":           {map[string]string{"PUBLIC_DOMAIN_ROOT": "https://nevatal.id", "EDGE_TARGET_CIDRS": "10.10.42.0/24"}, "not a valid domain name"},
		"ip root":            {map[string]string{"PUBLIC_DOMAIN_ROOT": "10.0.0.1", "EDGE_TARGET_CIDRS": "10.10.42.0/24"}, "not a valid domain name"},
		"bad edge port":      {map[string]string{"PUBLIC_DOMAIN_ROOT": "nevatal.id", "EDGE_TARGET_CIDRS": "10.10.42.0/24", "EDGE_PORT": "http"}, "EDGE_PORT"},
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

func TestDevelopmentIgnoresBadEdgeEntries(t *testing.T) {
	setEnv(t, map[string]string{"DEMO_MODE": "false", "PUBLIC_DOMAIN_ROOT": "nevatal.id", "EDGE_TARGET_CIDRS": "10.10.42.0/24,nope"})
	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development must still validate: %v", err)
	}
	if len(cfg.EdgeTargetCIDRs) != 1 {
		t.Fatalf("the valid CIDR must be kept, got %v", cfg.EdgeTargetCIDRs)
	}
	if !slices.ContainsFunc(cfg.Warnings, func(w string) bool { return strings.Contains(w, `"nope"`) }) {
		t.Fatalf("a bad entry must be warned about, got %v", cfg.Warnings)
	}
	// The edge never runs with a partially understood allow/deny list.
	if err := cfg.ValidateEdge(); err == nil || !strings.Contains(err.Error(), `"nope"`) {
		t.Fatalf("ValidateEdge must reject bad entries in every environment, got %v", err)
	}
}

func TestValidateEdge(t *testing.T) {
	setEnv(t, map[string]string{"DEMO_MODE": "false"})
	if err := Load().ValidateEdge(); err == nil || !strings.Contains(err.Error(), "PUBLIC_DOMAIN_ROOT is required") {
		t.Fatalf("the edge needs a root, got %v", err)
	}

	setEnv(t, map[string]string{"DEMO_MODE": "false", "PUBLIC_DOMAIN_ROOT": "nevatal.id"})
	if err := Load().ValidateEdge(); err != nil {
		t.Fatalf("development edge without CIDRs may start (it routes nothing): %v", err)
	}

	// The edge does not need the API secrets, only its own settings.
	setEnv(t, map[string]string{"APP_ENV": "production", "DEMO_MODE": "false", "PUBLIC_DOMAIN_ROOT": "nevatal.id"})
	if err := Load().ValidateEdge(); err == nil || !strings.Contains(err.Error(), "EDGE_TARGET_CIDRS is required") {
		t.Fatalf("production edge needs CIDRs, got %v", err)
	}
	setEnv(t, map[string]string{"APP_ENV": "production", "DEMO_MODE": "false", "PUBLIC_DOMAIN_ROOT": "nevatal.id", "EDGE_TARGET_CIDRS": "10.10.42.0/24"})
	if err := Load().ValidateEdge(); err != nil {
		t.Fatalf("production edge without API secrets must validate: %v", err)
	}
}
