package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
	"github.com/nevatal/cloud/internal/repository"
)

// MonitoringUsecase runs user-configured infrastructure checks. Checks are
// claimed from Postgres, so every API replica can run the monitor loop
// without duplicating work. Findings become in-app notifications and can
// trigger automation policies; checks never change infrastructure.
type MonitoringUsecase struct {
	checks        repository.InfraCheckRepository
	notifications *NotificationUsecase
	policies      *AutomationPolicyUsecase
	instances     *InstanceUsecase
	apps          *AppUsecase
	gameServers   *GameServerUsecase
	android       *AndroidUsecase
	databases     *DatabaseUsecase
	quota         *QuotaUsecase
	cfg           *config.Config
	interval      time.Duration
	httpClient    *http.Client
}

func NewMonitoringUsecase(checks repository.InfraCheckRepository, notifications *NotificationUsecase,
	policies *AutomationPolicyUsecase, instances *InstanceUsecase, apps *AppUsecase,
	gameServers *GameServerUsecase, android *AndroidUsecase, databases *DatabaseUsecase,
	quota *QuotaUsecase, cfg *config.Config) *MonitoringUsecase {
	if cfg == nil {
		cfg = &config.Config{}
	}
	return &MonitoringUsecase{
		checks: checks, notifications: notifications, policies: policies, instances: instances,
		apps: apps, gameServers: gameServers, android: android, databases: databases, quota: quota,
		cfg: cfg, interval: 30 * time.Second,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SetInterval changes the monitor tick (tests use a short interval).
func (s *MonitoringUsecase) SetInterval(d time.Duration) {
	if d > 0 {
		s.interval = d
	}
}

// CheckView is the API view of an infra check.
type CheckView struct {
	models.InfraCheck
	LastResult map[string]any `json:"last_result"`
}

func checkView(c *models.InfraCheck) CheckView {
	v := CheckView{InfraCheck: *c, LastResult: map[string]any{}}
	_ = json.Unmarshal([]byte(c.LastResult), &v.LastResult)
	if v.LastResult == nil {
		v.LastResult = map[string]any{}
	}
	return v
}

// CreateCheckInput is the body of POST /ai/checks.
type CreateCheckInput struct {
	Name            string     `json:"name"`
	ResourceType    string     `json:"resource_type"`
	ResourceID      *uuid.UUID `json:"resource_id"`
	CheckType       string     `json:"check_type"`
	IntervalSeconds int        `json:"interval_seconds"`
	Enabled         *bool      `json:"enabled"`
}

var checkTypeResources = map[string][]string{
	models.CheckTypeInstanceHealth: {"vps"},
	models.CheckTypeAppHealth:      {"app"},
	models.CheckTypeGameHealth:     {"game_server"},
	models.CheckTypeAndroidHealth:  {"android"},
	models.CheckTypeDatabaseHealth: {"database"},
	models.CheckTypeBackupFresh:    {"game_server", "android", "database"},
	models.CheckTypeQuotaUsage:     {"account"},
}

func validateCheck(in *CreateCheckInput, userID uuid.UUID) *Error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		in.Name = in.CheckType
	}
	if len(in.Name) > 120 {
		return ValidationError("name", "must be at most 120 characters")
	}
	allowed, ok := checkTypeResources[in.CheckType]
	if !ok {
		return ValidationError("check_type", "unsupported check type")
	}
	valid := false
	for _, typ := range allowed {
		if typ == in.ResourceType {
			valid = true
			break
		}
	}
	if !valid {
		return ValidationError("resource_type", "is not valid for this check type")
	}
	if in.ResourceType == "account" {
		id := userID
		in.ResourceID = &id
	}
	if in.ResourceID == nil || *in.ResourceID == uuid.Nil {
		return ValidationError("resource_id", "is required")
	}
	if in.IntervalSeconds == 0 {
		in.IntervalSeconds = 300
	}
	if in.IntervalSeconds < 60 || in.IntervalSeconds > 86400 {
		return ValidationError("interval_seconds", "must be 60-86400")
	}
	return nil
}

// CreateCheck registers a recurring check.
func (s *MonitoringUsecase) CreateCheck(ctx context.Context, userID uuid.UUID, in CreateCheckInput, clientIP string) (*CheckView, error) {
	if e := validateCheck(&in, userID); e != nil {
		return nil, e
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	now := time.Now()
	c := &models.InfraCheck{
		OwnerID: userID, Name: in.Name, ResourceType: in.ResourceType, ResourceID: *in.ResourceID,
		CheckType: in.CheckType, IntervalSeconds: in.IntervalSeconds, Enabled: enabled,
		Status: models.CheckStatusUnknown, LastResult: "{}", NextRunAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.checks.Create(ctx, c); err != nil {
		if err == repository.ErrDuplicate {
			return nil, newError(KindConflict, "CHECK_EXISTS", "a check of this type already exists for this resource")
		}
		return nil, err
	}
	v := checkView(c)
	return &v, nil
}

// ListChecks returns the user's checks.
func (s *MonitoringUsecase) ListChecks(ctx context.Context, userID uuid.UUID) ([]CheckView, error) {
	items, err := s.checks.ListByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]CheckView, 0, len(items))
	for i := range items {
		out = append(out, checkView(&items[i]))
	}
	return out, nil
}

// GetCheck returns one check.
func (s *MonitoringUsecase) GetCheck(ctx context.Context, id, userID uuid.UUID) (*CheckView, error) {
	c, err := s.checks.FindByID(ctx, id, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, newError(KindNotFound, "CHECK_NOT_FOUND", "check not found")
		}
		return nil, err
	}
	v := checkView(c)
	return &v, nil
}

// UpdateCheckInput changes a check's mutable fields.
type UpdateCheckInput struct {
	Name            *string `json:"name"`
	IntervalSeconds *int    `json:"interval_seconds"`
	Enabled         *bool   `json:"enabled"`
}

// UpdateCheck changes a check.
func (s *MonitoringUsecase) UpdateCheck(ctx context.Context, id, userID uuid.UUID, in UpdateCheckInput, clientIP string) (*CheckView, error) {
	c, err := s.checks.FindByID(ctx, id, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, newError(KindNotFound, "CHECK_NOT_FOUND", "check not found")
		}
		return nil, err
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" || len(name) > 120 {
			return nil, ValidationError("name", "must be 1-120 characters")
		}
		c.Name = name
	}
	if in.IntervalSeconds != nil {
		if *in.IntervalSeconds < 60 || *in.IntervalSeconds > 86400 {
			return nil, ValidationError("interval_seconds", "must be 60-86400")
		}
		c.IntervalSeconds = *in.IntervalSeconds
	}
	if in.Enabled != nil {
		c.Enabled = *in.Enabled
		if c.Enabled {
			c.NextRunAt = time.Now()
		}
	}
	c.UpdatedAt = time.Now()
	if err := s.checks.Save(ctx, c); err != nil {
		return nil, err
	}
	v := checkView(c)
	return &v, nil
}

// DeleteCheck removes a check.
func (s *MonitoringUsecase) DeleteCheck(ctx context.Context, id, userID uuid.UUID, clientIP string) error {
	if err := s.checks.Delete(ctx, id, userID); err != nil {
		if err == repository.ErrNotFound {
			return newError(KindNotFound, "CHECK_NOT_FOUND", "check not found")
		}
		return err
	}
	return nil
}

// RunNow runs one check immediately and returns the updated record.
func (s *MonitoringUsecase) RunNow(ctx context.Context, id, userID uuid.UUID) (*CheckView, error) {
	c, err := s.checks.FindByID(ctx, id, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, newError(KindNotFound, "CHECK_NOT_FOUND", "check not found")
		}
		return nil, err
	}
	s.run(ctx, c)
	fresh, err := s.checks.FindByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	v := checkView(fresh)
	return &v, nil
}

// Start runs the monitor loop until ctx is cancelled. Every replica may run
// it: checks are claimed atomically.
func (s *MonitoringUsecase) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		s.RunDue(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.RunDue(ctx)
			}
		}
	}()
}

// RunDue claims and runs the next due checks.
func (s *MonitoringUsecase) RunDue(ctx context.Context) {
	checks, err := s.checks.ClaimDue(ctx, 20)
	if err != nil {
		return
	}
	for i := range checks {
		s.run(ctx, &checks[i])
	}
}

// CheckResult is one observation.
type CheckResult struct {
	Status     string         `json:"status"`
	Message    string         `json:"message"`
	Facts      map[string]any `json:"facts"`
	ObservedAt time.Time      `json:"observed_at"`
}

func (s *MonitoringUsecase) run(ctx context.Context, c *models.InfraCheck) {
	result := s.observe(ctx, c)
	failures := c.ConsecutiveFailures
	if result.Status == models.CheckStatusCritical || result.Status == models.CheckStatusWarning {
		failures++
	} else {
		failures = 0
	}
	data, _ := json.Marshal(result)
	_ = s.checks.MarkRun(ctx, c.ID, result.Status, string(data), failures)

	bad := result.Status == models.CheckStatusCritical || result.Status == models.CheckStatusWarning
	if bad && c.Status != result.Status {
		severity := models.NotificationSeverityWarning
		if result.Status == models.CheckStatusCritical {
			severity = models.NotificationSeverityCritical
		}
		s.notifications.Notify(ctx, NotificationInput{
			UserID: c.OwnerID, Type: models.NotificationTypeCheck, Severity: severity,
			Title:   fmt.Sprintf("%s: %s", c.Name, result.Status),
			Message: result.Message, ResourceType: c.ResourceType, ResourceID: &c.ResourceID,
			ActionURL: "/dashboard/operations?tab=checks", DedupeKey: "check:" + c.ID.String() + ":" + result.Status,
		})
	}
	if bad && s.policies != nil {
		severity := "warning"
		if result.Status == models.CheckStatusCritical {
			severity = "critical"
		}
		s.policies.Evaluate(ctx, c.OwnerID, PolicyEvent{
			Trigger: models.PolicyTriggerCheckFailed, Severity: severity, Action: "failed",
			ResourceType: c.ResourceType, ResourceID: c.ResourceID.String(), CheckType: c.CheckType,
			Message: result.Message, CheckID: c.ID.String(),
		})
	}
}

func (s *MonitoringUsecase) observe(ctx context.Context, c *models.InfraCheck) CheckResult {
	now := time.Now()
	ok := func(msg string, facts map[string]any) CheckResult {
		return CheckResult{Status: models.CheckStatusOK, Message: msg, Facts: facts, ObservedAt: now}
	}
	warn := func(msg string, facts map[string]any) CheckResult {
		return CheckResult{Status: models.CheckStatusWarning, Message: msg, Facts: facts, ObservedAt: now}
	}
	crit := func(msg string, facts map[string]any) CheckResult {
		return CheckResult{Status: models.CheckStatusCritical, Message: msg, Facts: facts, ObservedAt: now}
	}
	unknown := func(msg string, facts map[string]any) CheckResult {
		return CheckResult{Status: models.CheckStatusUnknown, Message: msg, Facts: facts, ObservedAt: now}
	}

	switch c.CheckType {
	case models.CheckTypeInstanceHealth:
		inst, err := s.instances.Get(ctx, c.ResourceID, c.OwnerID, false)
		if err != nil {
			return crit("instance not found or access denied", nil)
		}
		facts := map[string]any{"status": inst.Status, "vcpus": inst.VCPUs, "memory_mb": inst.MemoryMB}
		if inst.Status != models.StateRunning {
			return crit(fmt.Sprintf("%s is %s", inst.Name, inst.Status), facts)
		}
		m, err := s.instances.GetMetrics(ctx, c.ResourceID, c.OwnerID, false)
		if err != nil || m == nil || !m.Available || m.Status == nil {
			return warn("instance is running but metrics are unavailable", facts)
		}
		cpu := m.Status.CPUUsage * 100
		mem := percent(m.Status.MemoryUsed, m.Status.MemoryMax)
		disk := percent(m.Status.DiskUsed, m.Status.DiskMax)
		facts["cpu_percent"], facts["memory_percent"], facts["disk_percent"] = round1(cpu), round1(mem), round1(disk)
		return thresholdResult(inst.Name, cpu, mem, disk, facts, ok, warn, crit)
	case models.CheckTypeAppHealth:
		app, err := s.apps.Get(ctx, c.ResourceID, c.OwnerID, false)
		if err != nil {
			return crit("application not found or access denied", nil)
		}
		facts := map[string]any{"status": app.Status, "url": "https://" + app.AssignedSubdomain}
		if app.Status != "running" && app.Status != "active" {
			return crit(fmt.Sprintf("%s is %s", app.Name, app.Status), facts)
		}
		if s.cfg.ProxmoxMock {
			facts["probe"] = "simulated"
			return ok("application is recorded as running (simulated platform)", facts)
		}
		code, err := s.probeApp(ctx, app)
		if err != nil {
			return warn("application health endpoint is unreachable: "+sanitize(err.Error()), facts)
		}
		facts["http_status"] = code
		switch {
		case code >= 500:
			return crit(fmt.Sprintf("application returns HTTP %d", code), facts)
		case code >= 400:
			return warn(fmt.Sprintf("application returns HTTP %d", code), facts)
		}
		return ok(fmt.Sprintf("application returns HTTP %d", code), facts)
	case models.CheckTypeGameHealth:
		gs, err := s.gameServers.Get(ctx, c.ResourceID, c.OwnerID, false)
		if err != nil {
			return crit("game server not found or access denied", nil)
		}
		facts := map[string]any{"status": gs.Status, "players": gs.OnlinePlayerCount}
		if gs.Status != models.GameServerStatusRunning {
			return crit(fmt.Sprintf("%s is %s", gs.Name, gs.Status), facts)
		}
		m, err := s.gameServers.GetMetrics(ctx, c.ResourceID, c.OwnerID, false)
		if err != nil {
			return warn("game server metrics are unavailable", facts)
		}
		facts["cpu_percent"], facts["memory_percent"] = round1(m.CPUUsagePercent), round1(m.MemoryUsagePercent)
		facts["tps"] = m.TPS
		return thresholdResult(gs.Name, m.CPUUsagePercent, m.MemoryUsagePercent, diskPercent(m.DiskUsedGB, m.DiskTotalGB), facts, ok, warn, crit)
	case models.CheckTypeAndroidHealth:
		a, err := s.android.Get(ctx, c.ResourceID, c.OwnerID, false)
		if err != nil {
			return crit("Android instance not found or access denied", nil)
		}
		facts := map[string]any{"status": a.Status}
		if a.Status != models.AndroidStatusRunning {
			return crit(fmt.Sprintf("%s is %s", a.Name, a.Status), facts)
		}
		m, err := s.android.GetMetrics(ctx, c.ResourceID, c.OwnerID, false)
		if err != nil {
			return warn("Android metrics are unavailable", facts)
		}
		facts["cpu_percent"], facts["memory_percent"] = round1(m.CPUUsagePercent), round1(m.MemoryUsagePercent)
		return thresholdResult(a.Name, m.CPUUsagePercent, m.MemoryUsagePercent, diskPercent(m.DiskUsedGB, m.DiskTotalGB), facts, ok, warn, crit)
	case models.CheckTypeDatabaseHealth:
		db, err := s.databases.Get(ctx, c.ResourceID, c.OwnerID)
		if err != nil {
			return crit("database not found or access denied", nil)
		}
		facts := map[string]any{"status": db.Status, "engine": db.Engine}
		if db.Status != models.DatabaseStatusRunning {
			return crit(fmt.Sprintf("%s is %s", db.Name, db.Status), facts)
		}
		m, err := s.databases.Metrics(ctx, c.ResourceID, c.OwnerID)
		if err != nil {
			return warn("database metrics are unavailable", facts)
		}
		facts["cpu_percent"], facts["memory_percent"], facts["storage_percent"] = round1(m.CPUPercent), round1(m.MemoryUsagePercent), round1(m.StorageUsagePercent)
		return thresholdResult(db.Name, m.CPUPercent, m.MemoryUsagePercent, m.StorageUsagePercent, facts, ok, warn, crit)
	case models.CheckTypeBackupFresh:
		return s.checkBackupFreshness(ctx, c, now, ok, warn, crit)
	case models.CheckTypeQuotaUsage:
		usage, err := s.quota.GetUsage(ctx, c.OwnerID)
		if err != nil {
			return unknown("quota is unavailable", nil)
		}
		cpu := percent(int64(usage.UsedVCPUs), int64(usage.MaxVCPUs))
		mem := percent(int64(usage.UsedMemoryMB), int64(usage.MaxMemoryMB))
		disk := percent(int64(usage.UsedStorageGB), int64(usage.MaxStorageGB))
		inst := percent(int64(usage.UsedInstances), int64(usage.MaxInstances))
		facts := map[string]any{"vcpu_percent": round1(cpu), "memory_percent": round1(mem), "storage_percent": round1(disk), "instances_percent": round1(inst)}
		return thresholdResult("quota", cpu, mem, disk, facts, ok, warn, crit)
	}
	return unknown("unsupported check type", nil)
}

func (s *MonitoringUsecase) checkBackupFreshness(ctx context.Context, c *models.InfraCheck, now time.Time,
	ok, warn, crit func(string, map[string]any) CheckResult) CheckResult {
	var last *time.Time
	var name string
	switch c.ResourceType {
	case orchestrator.TypeGameServer:
		g, err := s.gameServers.Get(ctx, c.ResourceID, c.OwnerID, false)
		if err != nil {
			return crit("game server not found or access denied", nil)
		}
		name, last = g.Name, g.LastBackupAt
	case orchestrator.TypeAndroid:
		a, err := s.android.Get(ctx, c.ResourceID, c.OwnerID, false)
		if err != nil {
			return crit("Android instance not found or access denied", nil)
		}
		name, last = a.Name, a.LastBackupAt
	case orchestrator.TypeDatabase:
		db, err := s.databases.Get(ctx, c.ResourceID, c.OwnerID)
		if err != nil {
			return crit("database not found or access denied", nil)
		}
		name, last = db.Name, db.LastBackupAt
	}
	facts := map[string]any{"resource": name}
	if last == nil {
		facts["last_backup"] = "never"
		return warn("no backup has ever completed", facts)
	}
	age := now.Sub(*last)
	facts["last_backup"] = last.UTC().Format(time.RFC3339)
	facts["age_hours"] = int(age.Hours())
	if age > 7*24*time.Hour {
		return warn(fmt.Sprintf("the last backup is %d hours old", int(age.Hours())), facts)
	}
	return ok("backup is fresh", facts)
}

func (s *MonitoringUsecase) probeApp(ctx context.Context, app *models.Application) (int, error) {
	host := strings.TrimSpace(app.AssignedSubdomain)
	if host == "" {
		return 0, fmt.Errorf("the application has no assigned hostname")
	}
	if strings.ContainsAny(host, " /\\") || strings.HasPrefix(host, "-") {
		return 0, fmt.Errorf("invalid application hostname")
	}
	if net.ParseIP(host) != nil {
		return 0, fmt.Errorf("refusing to probe an IP address")
	}
	// Only platform-assigned application hostnames are probed: this prevents
	// the health check from becoming an SSRF primitive.
	if !strings.HasSuffix(host, ".apps.nevatal.id") {
		return 0, fmt.Errorf("refusing to probe a host outside the application domain")
	}
	path := app.HealthCheckPath
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	target := "https://" + host + path
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "https" || u.Host != host {
		return 0, fmt.Errorf("invalid application URL")
	}
	client := *s.httpClient
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		if req.URL.Host != host {
			return fmt.Errorf("redirected to another host")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "nevatal-monitor/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func percent(used, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}

func diskPercent(usedGB float64, totalGB int) float64 {
	if totalGB <= 0 {
		return 0
	}
	return usedGB / float64(totalGB) * 100
}

func thresholdResult(name string, cpu, mem, disk float64, facts map[string]any,
	ok, warn, crit func(string, map[string]any) CheckResult) CheckResult {
	switch {
	case cpu >= 90 || mem >= 92 || disk >= 95:
		return crit(fmt.Sprintf("%s: CPU %.1f%%, memory %.1f%%, disk %.1f%%", name, cpu, mem, disk), facts)
	case cpu >= 80 || mem >= 85 || disk >= 85:
		return warn(fmt.Sprintf("%s: CPU %.1f%%, memory %.1f%%, disk %.1f%%", name, cpu, mem, disk), facts)
	}
	return ok(fmt.Sprintf("%s is healthy", name), facts)
}
