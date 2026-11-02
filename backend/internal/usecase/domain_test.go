package usecase

import (
	"context"
	"errors"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/domainroute"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

// --- fakes ---------------------------------------------------------------

type noTx struct{}

func (noTx) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type memAudit struct {
	repository.AuditRepository
	mu      sync.Mutex
	actions []string
}

func (a *memAudit) Create(_ context.Context, e *models.AuditLog) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.actions = append(a.actions, e.Action)
	return nil
}

type memInstances struct {
	repository.InstanceRepository
	byID map[uuid.UUID]*models.Instance
}

func (m *memInstances) FindByID(_ context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.Instance, error) {
	inst, ok := m.byID[id]
	if !ok || inst.DeletedAt.Valid || (ownerID != nil && inst.OwnerID != *ownerID) {
		return nil, repository.ErrNotFound
	}
	return inst, nil
}

func (m *memInstances) ListPrivateIPs(context.Context) ([]string, error) {
	var ips []string
	for _, inst := range m.byID {
		if !inst.DeletedAt.Valid && inst.PrivateIP != nil && *inst.PrivateIP != "" {
			ips = append(ips, *inst.PrivateIP)
		}
	}
	return ips, nil
}

// memDomains mimics the SQL semantics, including the partial unique index.
type memDomains struct {
	mu        sync.Mutex
	rows      []*models.Domain
	users     map[uuid.UUID]*models.User
	instances *memInstances
}

func (m *memDomains) load(d models.Domain) models.Domain {
	d.Owner = m.users[d.OwnerID]
	if d.InstanceID != nil {
		d.Instance = m.instances.byID[*d.InstanceID]
	}
	return d
}

func (m *memDomains) active(filter func(*models.Domain) bool) []models.Domain {
	var out []models.Domain
	for _, d := range m.rows {
		if d.ReleasedAt == nil && filter(d) {
			out = append(out, m.load(*d))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (m *memDomains) ListActiveByOwner(_ context.Context, ownerID uuid.UUID) ([]models.Domain, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active(func(d *models.Domain) bool { return d.OwnerID == ownerID }), nil
}

func (m *memDomains) ListActive(context.Context) ([]models.Domain, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active(func(*models.Domain) bool { return true }), nil
}

func (m *memDomains) FindActiveByID(_ context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.Domain, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	found := m.active(func(d *models.Domain) bool { return d.ID == id && (ownerID == nil || d.OwnerID == *ownerID) })
	if len(found) == 0 {
		return nil, repository.ErrNotFound
	}
	return &found[0], nil
}

func (m *memDomains) CountActiveByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	list, _ := m.ListActiveByOwner(ctx, ownerID)
	return int64(len(list)), nil
}

func (m *memDomains) HostnameActive(_ context.Context, hostname string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.active(func(d *models.Domain) bool { return strings.EqualFold(d.Hostname, hostname) })) > 0, nil
}

func (m *memDomains) HostnameQuarantined(_ context.Context, hostname string, claimantID uuid.UUID, since time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.rows {
		if strings.EqualFold(d.Hostname, hostname) && d.ReleasedAt != nil && !d.ReleasedAt.Before(since) &&
			(d.OwnerID != claimantID || d.AdminDisabled) {
			return true, nil
		}
	}
	return false, nil
}

func (m *memDomains) LockOwner(context.Context, uuid.UUID) error { return nil }

func (m *memDomains) Create(_ context.Context, d *models.Domain) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, row := range m.rows {
		if row.ReleasedAt == nil && strings.EqualFold(row.Hostname, d.Hostname) {
			return repository.ErrHostnameTaken
		}
	}
	d.ID = uuid.New()
	d.CreatedAt = time.Now().Add(time.Duration(len(m.rows)) * time.Millisecond)
	d.UpdatedAt = d.CreatedAt
	row := *d
	m.rows = append(m.rows, &row)
	return nil
}

func (m *memDomains) update(d *models.Domain, apply func(row *models.Domain)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, row := range m.rows {
		if row.ID == d.ID {
			apply(row)
			return nil
		}
	}
	return repository.ErrNotFound
}

func (m *memDomains) UpdateSettings(_ context.Context, d *models.Domain) error {
	return m.update(d, func(row *models.Domain) {
		row.InstanceID, row.TargetPort, row.Enabled = d.InstanceID, d.TargetPort, d.Enabled
	})
}

func (m *memDomains) UpdateAdminState(_ context.Context, d *models.Domain) error {
	return m.update(d, func(row *models.Domain) { row.AdminDisabled, row.DisabledReason = d.AdminDisabled, d.DisabledReason })
}

func (m *memDomains) Release(_ context.Context, d *models.Domain, at time.Time) error {
	return m.update(d, func(row *models.Domain) { row.ReleasedAt = &at })
}

func (m *memDomains) LoadEdgeSnapshot(context.Context) (repository.EdgeSnapshot, error) {
	return repository.EdgeSnapshot{}, nil
}

// --- fixture -------------------------------------------------------------

type domainFixture struct {
	uc        *DomainUsecase
	domains   *memDomains
	instances *memInstances
	audit     *memAudit
	alice     uuid.UUID
	bob       uuid.UUID
	aliceVM   uuid.UUID
	bobVM     uuid.UUID
	clock     time.Time
}

func newDomainFixture(t *testing.T, mutate func(*config.Config)) *domainFixture {
	t.Helper()
	cfg := &config.Config{
		PublicDomainRoot:     "nevatal.id",
		ReservedSubdomains:   []string{"www", "admin", "cloud"},
		MaxDomainsPerUser:    2,
		DomainQuarantineDays: 14,
		EdgeTargetCIDRs:      []netip.Prefix{netip.MustParsePrefix("10.10.42.0/24")},
	}
	if mutate != nil {
		mutate(cfg)
	}
	f := &domainFixture{alice: uuid.New(), bob: uuid.New(), aliceVM: uuid.New(), bobVM: uuid.New(), clock: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	ip1, ip2 := "10.10.42.20", "10.10.42.21"
	f.instances = &memInstances{byID: map[uuid.UUID]*models.Instance{
		f.aliceVM: {ID: f.aliceVM, OwnerID: f.alice, Name: "alice-vm", Status: models.StateRunning, PrivateIP: &ip1},
		f.bobVM:   {ID: f.bobVM, OwnerID: f.bob, Name: "bob-vm", Status: models.StateRunning, PrivateIP: &ip2},
	}}
	f.domains = &memDomains{instances: f.instances, users: map[uuid.UUID]*models.User{
		f.alice: {ID: f.alice, Email: "alice@example.com", Status: models.UserStatusActive},
		f.bob:   {ID: f.bob, Email: "bob@example.com", Status: models.UserStatusActive},
	}}
	f.audit = &memAudit{}
	f.uc = NewDomainUsecase(noTx{}, DomainRepos{Domains: f.domains, Instances: f.instances}, cfg, NewAuditUsecase(f.audit))
	f.uc.now = func() time.Time { return f.clock }
	return f
}

func (f *domainFixture) claim(t *testing.T, user, vm uuid.UUID, sub string, port int) (*DomainView, error) {
	t.Helper()
	return f.uc.Create(context.Background(), user, false, CreateDomainInput{Subdomain: sub, InstanceID: &vm, TargetPort: &port}, "127.0.0.1")
}

func errCode(err error) string {
	var ucErr *Error
	if errors.As(err, &ucErr) {
		return ucErr.Code
	}
	if err != nil {
		return "plain: " + err.Error()
	}
	return ""
}

// --- tests ---------------------------------------------------------------

func TestClassifySubdomain(t *testing.T) {
	f := newDomainFixture(t, nil)
	cases := map[string]string{
		"myapp":                       "",
		"a1b":                         "",
		"my-app-2":                    "",
		"ab":                          DomainReasonInvalid,
		"":                            DomainReasonInvalid,
		"-ab":                         DomainReasonInvalid,
		"ab-":                         DomainReasonInvalid,
		"my_app":                      DomainReasonInvalid,
		"my.app":                      DomainReasonInvalid,
		"xn--80ak6aa":                 DomainReasonInvalid, // IDN A-label (homographs)
		"ab--cd":                      DomainReasonInvalid,
		"a" + strings.Repeat("b", 63): DomainReasonInvalid,
		"admin":                       DomainReasonReserved,
		"www":                         DomainReasonReserved,
		"mynevatal":                   DomainReasonReserved,
		"nevatal-app":                 DomainReasonReserved,
		"nevata1":                     DomainReasonReserved,
		"n3vatal":                     DomainReasonReserved,
		"neva-tal":                    DomainReasonReserved,
		"nevatai-shop":                DomainReasonReserved,
	}
	for sub, want := range cases {
		if got, _ := f.uc.classifySubdomain(normalizeSubdomain(sub)); got != want {
			t.Errorf("classifySubdomain(%q) = %q, want %q", sub, got, want)
		}
	}
	if got, _ := f.uc.classifySubdomain(normalizeSubdomain("  MyApp ")); got != "" {
		t.Errorf("input must be trimmed and lower-cased, got %q", got)
	}
}

func TestDomainsDisabled(t *testing.T) {
	for name, mutate := range map[string]func(*config.Config){
		"no root":   func(c *config.Config) { c.PublicDomainRoot = "" },
		"demo mode": func(c *config.Config) { c.DemoMode = true },
	} {
		t.Run(name, func(t *testing.T) {
			f := newDomainFixture(t, mutate)
			if _, err := f.uc.List(context.Background(), f.alice); errCode(err) != "DOMAINS_DISABLED" {
				t.Fatalf("List: %v", err)
			}
			if _, err := f.claim(t, f.alice, f.aliceVM, "myapp", 8080); errCode(err) != "DOMAINS_DISABLED" {
				t.Fatalf("Create: %v", err)
			}
		})
	}
}

func TestCreateDomain(t *testing.T) {
	f := newDomainFixture(t, nil)
	view, err := f.claim(t, f.alice, f.aliceVM, " MyApp ", 3000)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if view.Domain.Hostname != "myapp.nevatal.id" || view.Domain.Subdomain != "myapp" || !view.Domain.Enabled {
		t.Fatalf("unexpected domain %+v", view.Domain)
	}
	if view.LiveStatus != domainroute.StatusLive || view.TargetIP == nil || *view.TargetIP != "10.10.42.20" {
		t.Fatalf("expected live at 10.10.42.20, got %s %v", view.LiveStatus, view.TargetIP)
	}

	cases := map[string]struct {
		user, vm uuid.UUID
		sub      string
		port     int
		want     string
	}{
		"taken (case-insensitive)": {f.bob, f.bobVM, "MYAPP", 80, "SUBDOMAIN_TAKEN"},
		"reserved":                 {f.bob, f.bobVM, "admin", 80, "SUBDOMAIN_RESERVED"},
		"brand":                    {f.bob, f.bobVM, "nevatal-login", 80, "SUBDOMAIN_RESERVED"},
		"idn":                      {f.bob, f.bobVM, "xn--pple-43d", 80, "VALIDATION_ERROR"},
		"port 0":                   {f.bob, f.bobVM, "bobapp", 0, "VALIDATION_ERROR"},
		"port 65536":               {f.bob, f.bobVM, "bobapp", 65536, "VALIDATION_ERROR"},
		"someone else's instance":  {f.bob, f.aliceVM, "bobapp", 80, "INSTANCE_NOT_FOUND"},
		"unknown instance":         {f.bob, uuid.New(), "bobapp", 80, "INSTANCE_NOT_FOUND"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := f.claim(t, tc.user, tc.vm, tc.sub, tc.port)
			if got := errCode(err); got != tc.want {
				t.Fatalf("error code = %q, want %q (%v)", got, tc.want, err)
			}
		})
	}

	// Validation errors name their field (details.field in the API).
	_, err = f.claim(t, f.bob, f.bobVM, "bobapp", 0)
	var ucErr *Error
	if !errors.As(err, &ucErr) || ucErr.Field != "target_port" {
		t.Fatalf("port error must name target_port, got %#v", err)
	}

	// An admin may publish any instance.
	port := 8080
	if _, err := f.uc.Create(context.Background(), f.bob, true, CreateDomainInput{Subdomain: "adminpick", InstanceID: &f.aliceVM, TargetPort: &port}, ""); err != nil {
		t.Fatalf("admin claim on another tenant's instance: %v", err)
	}
}

func TestDomainLimit(t *testing.T) {
	f := newDomainFixture(t, nil)
	for _, sub := range []string{"one-app", "two-app"} {
		if _, err := f.claim(t, f.alice, f.aliceVM, sub, 80); err != nil {
			t.Fatalf("claim %s: %v", sub, err)
		}
	}
	if _, err := f.claim(t, f.alice, f.aliceVM, "three-app", 80); errCode(err) != "DOMAIN_LIMIT_REACHED" {
		t.Fatalf("third claim: %v", err)
	}
	// Releasing frees a slot.
	list, _ := f.uc.List(context.Background(), f.alice)
	if err := f.uc.Release(context.Background(), list[0].Domain.ID, f.alice, false, ""); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := f.claim(t, f.alice, f.aliceVM, "three-app", 80); err != nil {
		t.Fatalf("claim after release: %v", err)
	}
}

func TestQuarantine(t *testing.T) {
	f := newDomainFixture(t, nil)
	view, err := f.claim(t, f.alice, f.aliceVM, "shop", 80)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.uc.Release(context.Background(), view.Domain.ID, f.alice, false, ""); err != nil {
		t.Fatal(err)
	}
	if err := f.uc.Release(context.Background(), view.Domain.ID, f.alice, false, ""); errCode(err) != "DOMAIN_NOT_FOUND" {
		t.Fatalf("second release: %v", err)
	}

	check, err := f.uc.Check(context.Background(), f.bob, "shop")
	if err != nil || check.Available || check.ReasonCode != DomainReasonQuarantined {
		t.Fatalf("bob's check = %+v, %v", check, err)
	}
	if _, err := f.claim(t, f.bob, f.bobVM, "shop", 80); errCode(err) != "SUBDOMAIN_QUARANTINED" {
		t.Fatalf("bob's claim: %v", err)
	}

	// The previous owner may take it back immediately.
	check, _ = f.uc.Check(context.Background(), f.alice, "shop")
	if !check.Available {
		t.Fatalf("alice's check = %+v", check)
	}
	again, err := f.claim(t, f.alice, f.aliceVM, "shop", 80)
	if err != nil {
		t.Fatalf("alice reclaims: %v", err)
	}

	// After the window anyone may claim it.
	_ = f.uc.Release(context.Background(), again.Domain.ID, f.alice, false, "")
	f.clock = f.clock.Add(15 * 24 * time.Hour)
	if _, err := f.claim(t, f.bob, f.bobVM, "shop", 80); err != nil {
		t.Fatalf("bob after the quarantine: %v", err)
	}
}

func TestAdminDisabledReleaseQuarantinesOwnerToo(t *testing.T) {
	f := newDomainFixture(t, nil)
	view, _ := f.claim(t, f.alice, f.aliceVM, "phish", 80)
	disabled, reason := true, "  phishing report #42 "
	got, err := f.uc.AdminSetDisabled(context.Background(), f.bob, view.Domain.ID, AdminDomainInput{AdminDisabled: &disabled, DisabledReason: &reason}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.LiveStatus != domainroute.StatusDisabled || got.Domain.DisabledReason != "phishing report #42" {
		t.Fatalf("admin disable: %s %q", got.LiveStatus, got.Domain.DisabledReason)
	}
	// Releasing must not let the owner claim a fresh, enabled copy.
	if err := f.uc.Release(context.Background(), view.Domain.ID, f.alice, false, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.claim(t, f.alice, f.aliceVM, "phish", 80); errCode(err) != "SUBDOMAIN_QUARANTINED" {
		t.Fatalf("owner reclaim after an admin disable: %v", err)
	}
}

func TestLiveStatus(t *testing.T) {
	f := newDomainFixture(t, nil)
	view, _ := f.claim(t, f.alice, f.aliceVM, "status-app", 80)
	id := view.Domain.ID
	get := func() domainroute.Status {
		v, err := f.uc.view(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		return v.LiveStatus
	}
	vm := f.instances.byID[f.aliceVM]

	vm.Status = models.StateStopped
	if got := get(); got != domainroute.StatusInstanceStopped {
		t.Fatalf("stopped: %s", got)
	}
	// Disabled wins over stopped.
	off := false
	if v, _ := f.uc.Update(context.Background(), id, f.alice, false, UpdateDomainInput{Enabled: &off}, ""); v.LiveStatus != domainroute.StatusDisabled {
		t.Fatalf("owner-disabled + stopped: %s", v.LiveStatus)
	}
	on := true
	_, _ = f.uc.Update(context.Background(), id, f.alice, false, UpdateDomainInput{Enabled: &on}, "")
	vm.Status = models.StateRunning

	conflict := "10.10.42.21" // bob's VM address
	vm.PrivateIP = &conflict
	if got := get(); got != domainroute.StatusIPConflict {
		t.Fatalf("shared address: %s", got)
	}
	docker := "172.17.0.2"
	vm.PrivateIP = &docker
	if got := get(); got != domainroute.StatusIPNotRoutable {
		t.Fatalf("address outside EDGE_TARGET_CIDRS: %s", got)
	}
	vm.PrivateIP = nil
	if got := get(); got != domainroute.StatusNoIP {
		t.Fatalf("no address: %s", got)
	}
	vm.DeletedAt.Valid = true
	if got := get(); got != domainroute.StatusInstanceMissing {
		t.Fatalf("deleted instance: %s", got)
	}
}

func TestUpdateAndAdminEnable(t *testing.T) {
	f := newDomainFixture(t, nil)
	view, _ := f.claim(t, f.alice, f.aliceVM, "edit-me", 80)
	id := view.Domain.ID

	if _, err := f.uc.Update(context.Background(), id, f.alice, false, UpdateDomainInput{}, ""); errCode(err) != "VALIDATION_ERROR" {
		t.Fatalf("empty patch: %v", err)
	}
	if _, err := f.uc.Update(context.Background(), id, f.bob, false, UpdateDomainInput{TargetPort: new(int)}, ""); errCode(err) != "VALIDATION_ERROR" {
		t.Fatalf("port 0: %v", err)
	}
	port := 9000
	if _, err := f.uc.Update(context.Background(), id, f.bob, false, UpdateDomainInput{TargetPort: &port}, ""); errCode(err) != "DOMAIN_NOT_FOUND" {
		t.Fatalf("another tenant's domain: %v", err)
	}
	if _, err := f.uc.Update(context.Background(), id, f.alice, false, UpdateDomainInput{InstanceID: &f.bobVM}, ""); errCode(err) != "INSTANCE_NOT_FOUND" {
		t.Fatalf("pointing at another tenant's VM: %v", err)
	}
	updated, err := f.uc.Update(context.Background(), id, f.alice, false, UpdateDomainInput{TargetPort: &port}, "")
	if err != nil || updated.Domain.TargetPort != 9000 {
		t.Fatalf("port update: %+v %v", updated, err)
	}

	disabled, reason := true, "abuse"
	_, _ = f.uc.AdminSetDisabled(context.Background(), f.bob, id, AdminDomainInput{AdminDisabled: &disabled, DisabledReason: &reason}, "")
	enabled := false
	got, err := f.uc.AdminSetDisabled(context.Background(), f.bob, id, AdminDomainInput{AdminDisabled: &enabled}, "")
	if err != nil || got.Domain.AdminDisabled || got.Domain.DisabledReason != "" || got.LiveStatus != domainroute.StatusLive {
		t.Fatalf("admin enable must clear the reason: %+v %v", got, err)
	}
	if _, err := f.uc.AdminSetDisabled(context.Background(), f.bob, id, AdminDomainInput{}, ""); errCode(err) != "VALIDATION_ERROR" {
		t.Fatalf("missing admin_disabled: %v", err)
	}

	want := []string{"domain.create", "domain.update", "domain.admin_disable", "domain.admin_enable"}
	if strings.Join(f.audit.actions, ",") != strings.Join(want, ",") {
		t.Fatalf("audit actions = %v, want %v", f.audit.actions, want)
	}
}

func TestCheck(t *testing.T) {
	f := newDomainFixture(t, nil)
	_, _ = f.claim(t, f.alice, f.aliceVM, "taken-app", 80)
	cases := map[string]struct {
		code     string
		hostname string
	}{
		"free-app":  {"", "free-app.nevatal.id"},
		"Taken-App": {DomainReasonTaken, "taken-app.nevatal.id"},
		"cloud":     {DomainReasonReserved, "cloud.nevatal.id"},
		"x":         {DomainReasonInvalid, ""},
		"xn--abc":   {DomainReasonInvalid, ""},
	}
	for sub, tc := range cases {
		got, err := f.uc.Check(context.Background(), f.bob, sub)
		if err != nil {
			t.Fatal(err)
		}
		if got.ReasonCode != tc.code || got.Hostname != tc.hostname || got.Available != (tc.code == "") {
			t.Errorf("Check(%q) = %+v, want code %q hostname %q", sub, got, tc.code, tc.hostname)
		}
	}
}
