package config

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Version is the application version reported by /health and /api/v1/system/config.
// Override at build time with:
//
//	go build -ldflags "-X github.com/nevatal/cloud/internal/config.Version=1.2.3"
var Version = "1.2.0"

const (
	EnvDevelopment = "development"
	EnvProduction  = "production"

	// DevJWTSecret is the public development secret. It is only used when
	// APP_ENV is not production and is rejected by Validate in production.
	DevJWTSecret = "nevatal-cloud-super-secret-key-production-ready-32-chars"

	// legacyWordPressAPIKey was hardcoded in earlier releases and in the
	// WordPress plugin. It is public, so it is never accepted in production.
	legacyWordPressAPIKey = "wp_nevatal_cloud_api_key_secret_2026"

	minJWTSecretLength     = 32
	minAdminPasswordLength = 12

	// Public demo accounts. They are seeded only when DEMO_MODE=true and are
	// suspended automatically when a database is reused with DEMO_MODE=false.
	DemoAdminEmail    = "admin@nevatal.id"
	DemoAdminPassword = "AdminPass123!"
	DemoUserEmail     = "demo@nevatal.id"
	DemoUserPassword  = "DemoPass123!"
)

// defaultTrustedProxies covers loopback and RFC 1918 private ranges, which is
// where the bundled nginx, Docker networks, Kubernetes pods and LAN load
// balancers live. Override with TRUSTED_PROXIES.
var defaultTrustedProxies = []string{"127.0.0.1/32", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}

type Config struct {
	Port        string
	Env         string // "development" or "production"
	DemoMode    bool
	AppBaseURL  string
	DatabaseURL string
	RedisURL    string
	JWTSecret   string

	// Database connection pool (per API replica; keep replicas x max_open
	// below the Postgres max_connections limit).
	DBMaxOpenConns int
	DBMaxIdleConns int

	// Bootstrap administrator (required in production when DEMO_MODE=false).
	AdminEmail    string
	AdminPassword string

	// HTTP hardening.
	CORSAllowedOrigins []string // empty in development = reflect any origin
	TrustedProxies     []string
	CookieSecure       bool

	// Local-disk object storage root (must be a persistent, shared volume when
	// running more than one API replica).
	StoragePath string

	ProxmoxHost        string
	ProxmoxPort        string
	ProxmoxTokenID     string
	ProxmoxTokenSecret string
	ProxmoxNodeName    string
	ProxmoxStorage     string
	ProxmoxMock        bool
	ProxmoxVerifyTLS   bool

	GoogleClientID     string
	GoogleClientSecret string
	GithubClientID     string
	GithubClientSecret string
	WordPressAPIKey    string // empty = WordPress integration disabled
	WordPressURL       string
	OpenRouterAPIKey   string
	OpenRouterModel    string
	OpenRouterBaseURL  string

	// Nevatal Domains: customers publish one port of a VM at
	// https://<subdomain>.<PublicDomainRoot>; nevatal-edge proxies the traffic.
	PublicDomainRoot     string         // PUBLIC_DOMAIN_ROOT, e.g. nevatal.id; empty = feature disabled
	ReservedSubdomains   []string       // built-in names + RESERVED_SUBDOMAINS + the APP_BASE_URL label
	MaxDomainsPerUser    int            // MAX_DOMAINS_PER_USER: active domains per account
	DomainQuarantineDays int            // DOMAIN_QUARANTINE_DAYS: released hostnames stay blocked for other accounts
	EdgeTargetCIDRs      []netip.Prefix // EDGE_TARGET_CIDRS: VM networks the edge may proxy to (also preferred guest IPs)
	EdgeDenyIPs          []netip.Addr   // EDGE_DENY_IPS (+ PROXMOX_HOST when it is an IP): never proxied to
	EdgePort             string         // EDGE_PORT: nevatal-edge listen port

	// Provisioning job durability: how often a running job refreshes its
	// heartbeat (JOB_HEARTBEAT_SECONDS, default 10) and after how long
	// without one the reaper fails it as JOB_STALE (JOB_STALE_AFTER_SECONDS,
	// default 300).
	JobHeartbeatInterval time.Duration
	JobStaleAfter        time.Duration

	// AI orchestrator execution engine (every API replica runs one).
	OrchConcurrency  int           // ORCH_CONCURRENCY: tasks run at once per replica (default 4)
	OrchLease        time.Duration // ORCH_LEASE_SECONDS: task lease (default 30)
	OrchHeartbeat    time.Duration // ORCH_HEARTBEAT_SECONDS: lease renewal interval (default 10, < lease/2)
	OrchPollInterval time.Duration // ORCH_POLL_MS: claim polling interval (default 1000)
	OrchRetryBase    time.Duration // ORCH_RETRY_BASE_MS: first retry backoff (default 2000)
	// OrchFaults injects task failures ("tool:N,tool:N": the first N attempts
	// of every task using tool fail). Honoured only in development with the
	// simulated hypervisor; used by the orchestrator end-to-end tests.
	OrchFaults map[string]int
	// Cost rate card used by the advisor (monthly estimates, not invoices).
	OrchCostCurrency string
	OrchCostVCPU     float64
	OrchCostMemoryGB float64
	OrchCostDiskGB   float64
	OrchCostBucketGB float64
	// MonitorInterval is how often the scheduled check loop looks for due
	// checks (MONITOR_INTERVAL_SECONDS, default 30).
	MonitorInterval time.Duration
	// OrchDBBackupInterval is how often a running managed database with
	// auto_backup enabled gets an automatic backup
	// (ORCH_DB_BACKUP_INTERVAL_SECONDS, default 86400).
	OrchDBBackupInterval time.Duration
	// NotificationRetention is how long in-app notifications are kept
	// (ORCH_NOTIFICATION_RETENTION_DAYS, default 30; 0 keeps every row).
	NotificationRetention time.Duration
	// ProxmoxMockLatency slows simulated clone/start calls
	// (PROXMOX_MOCK_LATENCY_MS); simulated hypervisor only.
	ProxmoxMockLatency time.Duration

	// Warnings collected while loading; logged by the caller.
	Warnings []string

	proxmoxMockRequested bool
	proxmoxCredsComplete bool

	// Unparsable Nevatal Domains / edge settings. Validate (production) and
	// ValidateEdge (always) refuse to start with them.
	domainProblems    []string
	domainRootInvalid bool
}

// IsProduction reports whether production hardening is active.
func (c *Config) IsProduction() bool {
	return c.Env == EnvProduction
}

// WordPressEnabled reports whether the WordPress bridge has a usable shared secret.
func (c *Config) WordPressEnabled() bool {
	return c.WordPressAPIKey != ""
}

// DomainsEnabled reports whether customers may publish VM ports under
// PublicDomainRoot. Demo mode always disables the feature: demo accounts are
// public and simulated VM addresses may belong to real hosts on the LAN.
func (c *Config) DomainsEnabled() bool {
	return c.PublicDomainRoot != "" && !c.DemoMode
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{}

	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	}
	switch env {
	case "production", "prod":
		cfg.Env = EnvProduction
	case "", "development", "dev", "local":
		cfg.Env = EnvDevelopment
	default:
		cfg.Env = EnvDevelopment
		cfg.warn("unknown APP_ENV %q; falling back to development", env)
	}

	cfg.DemoMode = getBool("DEMO_MODE", !cfg.IsProduction())

	cfg.Port = getEnv("PORT", "8090")
	cfg.AppBaseURL = strings.TrimRight(getEnv("APP_BASE_URL", "http://localhost:3080"), "/")
	cfg.DatabaseURL = getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5435/nevatal_cloud?sslmode=disable")
	cfg.RedisURL = getEnv("REDIS_URL", "")
	cfg.DBMaxOpenConns = getInt("DB_MAX_OPEN_CONNS", 15)
	cfg.DBMaxIdleConns = getInt("DB_MAX_IDLE_CONNS", 5)

	cfg.JWTSecret = os.Getenv("JWT_SECRET")
	if cfg.JWTSecret == "" && !cfg.IsProduction() {
		cfg.JWTSecret = DevJWTSecret
	}

	cfg.AdminEmail = strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
	cfg.AdminPassword = os.Getenv("ADMIN_PASSWORD")

	cfg.CORSAllowedOrigins = splitList(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if len(cfg.CORSAllowedOrigins) == 0 && cfg.IsProduction() {
		if origin := originOf(cfg.AppBaseURL); origin != "" {
			cfg.CORSAllowedOrigins = []string{origin}
		}
	}

	cfg.TrustedProxies = splitList(os.Getenv("TRUSTED_PROXIES"))
	if len(cfg.TrustedProxies) == 0 {
		cfg.TrustedProxies = append([]string(nil), defaultTrustedProxies...)
	}

	cfg.CookieSecure = getBool("COOKIE_SECURE", strings.HasPrefix(strings.ToLower(cfg.AppBaseURL), "https://"))
	cfg.StoragePath = getEnv("STORAGE_PATH", "/data/storage")

	// Proxmox VE hypervisor.
	cfg.ProxmoxHost = os.Getenv("PROXMOX_HOST")
	cfg.ProxmoxPort = getEnv("PROXMOX_PORT", "8006")
	cfg.ProxmoxTokenID = os.Getenv("PROXMOX_TOKEN_ID")
	cfg.ProxmoxTokenSecret = os.Getenv("PROXMOX_TOKEN_SECRET")
	cfg.ProxmoxNodeName = getEnv("PROXMOX_NODE_NAME", "pve-dell-r640")
	cfg.ProxmoxStorage = getEnv("PROXMOX_STORAGE", "local-zfs")
	cfg.ProxmoxVerifyTLS = getBool("PROXMOX_VERIFY_TLS", false)

	cfg.proxmoxMockRequested = getBool("PROXMOX_MOCK", true)
	cfg.proxmoxCredsComplete = cfg.ProxmoxHost != "" && cfg.ProxmoxTokenID != "" && cfg.ProxmoxTokenSecret != ""
	cfg.ProxmoxMock = cfg.proxmoxMockRequested || !cfg.proxmoxCredsComplete
	if !cfg.proxmoxMockRequested && !cfg.proxmoxCredsComplete && !cfg.IsProduction() {
		cfg.warn("PROXMOX_MOCK=false but PROXMOX_HOST/PROXMOX_TOKEN_ID/PROXMOX_TOKEN_SECRET are incomplete; using the simulated hypervisor")
	}
	if cfg.DemoMode && !cfg.ProxmoxMock {
		// Demo mode publishes admin credentials in the UI. Never let that
		// control a real hypervisor.
		cfg.ProxmoxMock = true
		cfg.warn("DEMO_MODE=true forces PROXMOX_MOCK=true (demo accounts are public). Set DEMO_MODE=false to manage a real Proxmox VE node")
	}

	cfg.GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	cfg.GoogleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	cfg.GithubClientID = os.Getenv("GITHUB_CLIENT_ID")
	cfg.GithubClientSecret = os.Getenv("GITHUB_CLIENT_SECRET")

	cfg.WordPressAPIKey = os.Getenv("WORDPRESS_API_KEY")
	if cfg.WordPressAPIKey == "" && !cfg.IsProduction() {
		// Keeps the bundled WordPress plugin working out of the box in development.
		cfg.WordPressAPIKey = legacyWordPressAPIKey
	}
	if cfg.WordPressAPIKey == legacyWordPressAPIKey && cfg.IsProduction() {
		cfg.WordPressAPIKey = ""
		cfg.warn("WORDPRESS_API_KEY is the public legacy default; WordPress integration is disabled until a new secret is set")
	}
	cfg.WordPressURL = os.Getenv("WORDPRESS_URL")

	cfg.OpenRouterAPIKey = os.Getenv("OPENROUTER_API_KEY")
	cfg.OpenRouterModel = getEnv("OPENROUTER_DEFAULT_MODEL", "openai/gpt-4o-mini")
	cfg.OpenRouterBaseURL = getEnv("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1")

	cfg.loadDomains()
	cfg.loadOrchestrator()

	return cfg
}

// loadOrchestrator reads the execution engine and job durability settings.
func (c *Config) loadOrchestrator() {
	c.JobHeartbeatInterval = time.Duration(getInt("JOB_HEARTBEAT_SECONDS", 10)) * time.Second
	c.JobStaleAfter = time.Duration(getInt("JOB_STALE_AFTER_SECONDS", 300)) * time.Second
	if c.JobStaleAfter < 3*c.JobHeartbeatInterval {
		c.JobStaleAfter = 3 * c.JobHeartbeatInterval
		c.warn("JOB_STALE_AFTER_SECONDS is below three heartbeats; using %s", c.JobStaleAfter)
	}

	c.OrchConcurrency = getInt("ORCH_CONCURRENCY", 4)
	c.OrchLease = time.Duration(getInt("ORCH_LEASE_SECONDS", 30)) * time.Second
	c.OrchHeartbeat = time.Duration(getInt("ORCH_HEARTBEAT_SECONDS", 10)) * time.Second
	if c.OrchHeartbeat*2 > c.OrchLease {
		// A lease must survive at least one missed heartbeat.
		c.OrchHeartbeat = c.OrchLease / 3
		c.warn("ORCH_HEARTBEAT_SECONDS must be below half of ORCH_LEASE_SECONDS; using %s", c.OrchHeartbeat)
	}
	c.OrchPollInterval = time.Duration(getInt("ORCH_POLL_MS", 1000)) * time.Millisecond
	c.OrchRetryBase = time.Duration(getInt("ORCH_RETRY_BASE_MS", 2000)) * time.Millisecond
	c.OrchCostCurrency = getEnv("ORCH_COST_CURRENCY", "USD")
	c.OrchCostVCPU = getFloat("ORCH_COST_VCPU_MONTH", 5)
	c.OrchCostMemoryGB = getFloat("ORCH_COST_MEMORY_GB_MONTH", 2.5)
	c.OrchCostDiskGB = getFloat("ORCH_COST_DISK_GB_MONTH", 0.1)
	c.OrchCostBucketGB = getFloat("ORCH_COST_BUCKET_GB_MONTH", 0.02)
	c.MonitorInterval = time.Duration(getInt("MONITOR_INTERVAL_SECONDS", 30)) * time.Second
	if c.MonitorInterval < 5*time.Second {
		c.MonitorInterval = 5 * time.Second
	}
	c.OrchDBBackupInterval = time.Duration(getInt("ORCH_DB_BACKUP_INTERVAL_SECONDS", 86400)) * time.Second
	if c.OrchDBBackupInterval < time.Minute {
		c.OrchDBBackupInterval = time.Minute
		c.warn("ORCH_DB_BACKUP_INTERVAL_SECONDS is below 60; using 1m")
	}
	c.NotificationRetention = time.Duration(getInt("ORCH_NOTIFICATION_RETENTION_DAYS", 30)) * 24 * time.Hour
	if c.NotificationRetention < 0 {
		c.NotificationRetention = 0
	}

	if raw := strings.TrimSpace(os.Getenv("ORCH_FAULTS")); raw != "" {
		if !c.ProxmoxMock || c.IsProduction() {
			c.warn("ORCH_FAULTS is ignored: fault injection needs APP_ENV=development and the simulated hypervisor (PROXMOX_MOCK=true)")
		} else {
			c.OrchFaults = map[string]int{}
			for _, part := range splitList(raw) {
				tool, n, ok := strings.Cut(part, ":")
				count, err := strconv.Atoi(strings.TrimSpace(n))
				if !ok || err != nil || count <= 0 || strings.TrimSpace(tool) == "" {
					c.warn("ORCH_FAULTS entry %q ignored (want tool:attempts)", part)
					continue
				}
				c.OrchFaults[strings.TrimSpace(tool)] = count
			}
		}
	}
	if ms := getInt("PROXMOX_MOCK_LATENCY_MS", 0); ms > 0 {
		if c.ProxmoxMock {
			c.ProxmoxMockLatency = time.Duration(ms) * time.Millisecond
		} else {
			c.warn("PROXMOX_MOCK_LATENCY_MS is ignored with a real hypervisor")
		}
	}
}

// Validate returns an error describing every unsafe setting that must be
// fixed before the API may start. Development configurations always pass.
func (c *Config) Validate() error {
	if !c.IsProduction() {
		return nil
	}

	var problems []string

	switch {
	case c.JWTSecret == "":
		problems = append(problems, "JWT_SECRET is required (generate one with: openssl rand -hex 32)")
	case c.JWTSecret == DevJWTSecret:
		problems = append(problems, "JWT_SECRET is the public development default; generate a new one with: openssl rand -hex 32")
	case len(c.JWTSecret) < minJWTSecretLength:
		problems = append(problems, fmt.Sprintf("JWT_SECRET must be at least %d characters", minJWTSecretLength))
	}

	if originOf(c.AppBaseURL) == "" {
		problems = append(problems, "APP_BASE_URL must be an absolute URL such as https://cloud.example.com")
	}
	for _, origin := range c.CORSAllowedOrigins {
		if origin == "*" {
			problems = append(problems, "CORS_ALLOWED_ORIGINS must list explicit origins in production (\"*\" is not allowed with credentials)")
			break
		}
	}

	if !c.DemoMode {
		if c.AdminEmail == "" || !strings.Contains(c.AdminEmail, "@") {
			problems = append(problems, "ADMIN_EMAIL is required when DEMO_MODE=false")
		}
		if len(c.AdminPassword) < minAdminPasswordLength {
			problems = append(problems, fmt.Sprintf("ADMIN_PASSWORD is required when DEMO_MODE=false and must be at least %d characters", minAdminPasswordLength))
		} else if c.AdminPassword == DemoAdminPassword || c.AdminPassword == DemoUserPassword {
			problems = append(problems, "ADMIN_PASSWORD must not be one of the public demo passwords")
		}
		if !c.proxmoxMockRequested && !c.proxmoxCredsComplete {
			problems = append(problems, "PROXMOX_MOCK=false requires PROXMOX_HOST, PROXMOX_TOKEN_ID and PROXMOX_TOKEN_SECRET")
		}
	}

	problems = append(problems, c.domainProblems...)
	if c.DomainsEnabled() && len(c.EdgeTargetCIDRs) == 0 {
		problems = append(problems, edgeCIDRsRequired)
	}

	if len(problems) == 0 {
		return nil
	}
	return errors.New("unsafe production configuration:\n  - " + strings.Join(problems, "\n  - "))
}

const edgeCIDRsRequired = "EDGE_TARGET_CIDRS is required when PUBLIC_DOMAIN_ROOT is set: list the VM network(s) the edge may proxy to, e.g. 10.10.42.0/24"

// ValidateEdge checks the settings nevatal-edge needs. Unlike Validate it
// does not require the API secrets, and it applies in every environment:
// the edge refuses demo mode, needs PUBLIC_DOMAIN_ROOT and well-formed
// EDGE_TARGET_CIDRS / EDGE_DENY_IPS, and in production at least one CIDR.
func (c *Config) ValidateEdge() error {
	var problems []string
	if c.DemoMode {
		problems = append(problems, "DEMO_MODE must be false: the edge never publishes a demo deployment (public demo accounts, simulated VM addresses)")
	}
	if c.PublicDomainRoot == "" && !c.domainRootInvalid {
		problems = append(problems, "PUBLIC_DOMAIN_ROOT is required (for example nevatal.id)")
	}
	problems = append(problems, c.domainProblems...)
	if c.IsProduction() && len(c.EdgeTargetCIDRs) == 0 {
		problems = append(problems, edgeCIDRsRequired)
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.New("unsafe edge configuration:\n  - " + strings.Join(problems, "\n  - "))
}

// builtinReservedSubdomains can never be claimed: platform and
// infrastructure hostnames, mail and client auto-discovery names (which
// clients trust implicitly), and names that make convincing phishing pages.
var builtinReservedSubdomains = []string{
	"www", "cloud", "api", "app", "apps", "admin", "demo", "docs", "doc", "status",
	"mail", "smtp", "imap", "pop", "ftp", "ns1", "ns2", "ns3", "ns4", "mx",
	"cdn", "static", "assets", "media", "img", "blog", "help", "support",
	"auth", "login", "sso", "oauth", "account", "accounts", "billing",
	"dashboard", "console", "panel", "portal", "proxmox", "pve", "edge", "tunnel", "vpn",
	"git", "grafana", "prometheus", "metrics", "monitor", "s3", "storage", "minio",
	"n8n", "wp", "wordpress", "dev", "staging", "test", "internal", "localhost", "root",
	"nevatal", "security", "abuse", "postmaster", "hostmaster", "webmaster",
	"autodiscover", "autoconfig",
	// Implicitly trusted by mail clients, browsers and OS resolvers.
	"webmail", "email", "pop3", "ns", "dns", "mta-sts", "openpgpkey", "wpad", "isatap",
	"lyncdiscover", "sip", "enterpriseregistration", "enterpriseenrollment", "msoid",
}

// loadDomains reads the Nevatal Domains and nevatal-edge settings. It runs
// after the rest of Load (it needs APP_BASE_URL, PROXMOX_HOST and DEMO_MODE).
func (c *Config) loadDomains() {
	rawRoot := strings.TrimSpace(os.Getenv("PUBLIC_DOMAIN_ROOT"))
	root := strings.Trim(strings.ToLower(rawRoot), ".")
	if root != "" && !isDomainName(root) {
		c.domainProblems = append(c.domainProblems, fmt.Sprintf("PUBLIC_DOMAIN_ROOT %q is not a valid domain name (expected something like nevatal.id)", rawRoot))
		c.domainRootInvalid = true
		root = ""
	}
	c.PublicDomainRoot = root

	reserved := append([]string(nil), builtinReservedSubdomains...)
	for _, name := range splitList(os.Getenv("RESERVED_SUBDOMAINS")) {
		reserved = append(reserved, strings.Trim(strings.ToLower(name), "."))
	}
	// The control plane's own label (cloud in cloud.nevatal.id) is never claimable.
	if root != "" {
		if u, err := url.Parse(c.AppBaseURL); err == nil {
			if label, ok := strings.CutSuffix(strings.ToLower(u.Hostname()), "."+root); ok && label != "" && !strings.Contains(label, ".") {
				reserved = append(reserved, label)
			}
		}
	}
	c.ReservedSubdomains = dedupe(reserved)

	c.MaxDomainsPerUser = getInt("MAX_DOMAINS_PER_USER", 5)
	c.DomainQuarantineDays = getNonNegativeInt("DOMAIN_QUARANTINE_DAYS", 14)

	c.EdgePort = getEnv("EDGE_PORT", "8088")
	if port, err := strconv.Atoi(c.EdgePort); err != nil || port < 1 || port > 65535 {
		c.domainProblems = append(c.domainProblems, fmt.Sprintf("EDGE_PORT %q is not a TCP port", c.EdgePort))
		c.EdgePort = "8088"
	}

	for _, raw := range splitList(os.Getenv("EDGE_TARGET_CIDRS")) {
		prefix, err := parsePrefixOrAddr(raw)
		if err != nil {
			c.domainProblems = append(c.domainProblems, fmt.Sprintf("EDGE_TARGET_CIDRS entry %q is not a CIDR or IP address", raw))
			continue
		}
		c.EdgeTargetCIDRs = append(c.EdgeTargetCIDRs, prefix)
	}
	for _, raw := range splitList(os.Getenv("EDGE_DENY_IPS")) {
		addr, err := netip.ParseAddr(raw)
		if err != nil {
			c.domainProblems = append(c.domainProblems, fmt.Sprintf("EDGE_DENY_IPS entry %q is not an IP address", raw))
			continue
		}
		c.EdgeDenyIPs = appendAddr(c.EdgeDenyIPs, addr.Unmap())
	}
	// The hypervisor itself is never a tenant target.
	if addr, err := netip.ParseAddr(strings.TrimSpace(c.ProxmoxHost)); err == nil {
		c.EdgeDenyIPs = appendAddr(c.EdgeDenyIPs, addr.Unmap())
	}

	if !c.IsProduction() {
		for _, p := range c.domainProblems {
			c.warn("%s (ignored)", p)
		}
	}
	if root != "" && c.DemoMode {
		c.warn("DEMO_MODE=true disables Nevatal Domains (PUBLIC_DOMAIN_ROOT=%s is ignored): demo accounts are public and simulated VM addresses may belong to real hosts. Set DEMO_MODE=false to publish customer domains", root)
	}
}

// isDomainName reports whether s is a lowercase DNS name of LDH labels whose
// last label is not numeric (so IP addresses are rejected).
func isDomainName(s string) bool {
	if len(s) == 0 || len(s) > 253 {
		return false
	}
	labels := strings.Split(s, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			ch := label[i]
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return false
			}
		}
	}
	if _, err := strconv.Atoi(labels[len(labels)-1]); err == nil {
		return false
	}
	return true
}

// parsePrefixOrAddr parses a CIDR, or a bare IP as a single-address prefix.
func parsePrefixOrAddr(raw string) (netip.Prefix, error) {
	if strings.Contains(raw, "/") {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return netip.Prefix{}, err
		}
		return prefix.Masked(), nil
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Prefix{}, err
	}
	addr = addr.Unmap()
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

func appendAddr(list []netip.Addr, addr netip.Addr) []netip.Addr {
	for _, existing := range list {
		if existing == addr {
			return list
		}
	}
	return append(list, addr)
}

func dedupe(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func (c *Config) warn(format string, args ...any) {
	c.Warnings = append(c.Warnings, fmt.Sprintf(format, args...))
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getBool(key string, defaultVal bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(raw)
	if err != nil {
		return defaultVal
	}
	return val
}

func getInt(key string, defaultVal int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(raw)
	if err != nil || val <= 0 {
		return defaultVal
	}
	return val
}

func getFloat(key string, defaultVal float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil || val < 0 {
		return defaultVal
	}
	return val
}

// getNonNegativeInt is getInt that also accepts 0 (for example to turn a
// waiting period off).
func getNonNegativeInt(key string, defaultVal int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(raw)
	if err != nil || val < 0 {
		return defaultVal
	}
	return val
}

func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, strings.TrimRight(part, "/"))
		}
	}
	return out
}

// originOf returns scheme://host[:port] for an absolute URL, or "".
func originOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
