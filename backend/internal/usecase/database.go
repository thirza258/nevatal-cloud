package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
	"github.com/nevatal/cloud/internal/secret"
)

// DatabasePlan is a managed database sizing preset.
type DatabasePlan struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	VCPUs     int    `json:"vcpus"`
	MemoryMB  int    `json:"memory_mb"`
	StorageGB int    `json:"storage_gb"`
}

// AvailableDatabasePlans are the supported sizing presets.
var AvailableDatabasePlans = []DatabasePlan{
	{ID: "starter", Name: "Starter", VCPUs: 1, MemoryMB: 1024, StorageGB: 10},
	{ID: "standard", Name: "Standard", VCPUs: 2, MemoryMB: 4096, StorageGB: 50},
	{ID: "power", Name: "Power", VCPUs: 4, MemoryMB: 8192, StorageGB: 200},
}

var databaseNameRegex = regexp.MustCompile(`^[a-z][a-z0-9\-]{1,62}$`)

// DatabaseUsecase provisions and operates managed databases. The service is
// logical: it owns lifecycle, credentials, backups and health records; the
// execution engine and verification hooks decide whether a real runtime is
// observable.
type DatabaseUsecase struct {
	repos  repository.DatabaseRepository
	audit  *AuditUsecase
	quota  *QuotaUsecase
	cfg    *config.Config
	cipher *secret.Cipher
	now    func() time.Time
	// backupInterval is the automatic backup cadence (default 24 h).
	backupInterval time.Duration
}

func NewDatabaseUsecase(repos repository.DatabaseRepository, audit *AuditUsecase, quota *QuotaUsecase, cfg *config.Config, cipher *secret.Cipher) *DatabaseUsecase {
	if cfg == nil {
		cfg = &config.Config{}
	}
	interval := cfg.OrchDBBackupInterval
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &DatabaseUsecase{repos: repos, audit: audit, quota: quota, cfg: cfg, cipher: cipher,
		now: time.Now, backupInterval: interval}
}

var errDatabaseNotFound = newError(KindNotFound, "DATABASE_NOT_FOUND", "database not found")

// CreateDatabaseInput is the request to provision a database.
type CreateDatabaseInput struct {
	Name          string `json:"name"`
	Engine        string `json:"engine"`
	EngineVersion string `json:"engine_version"`
	PlanName      string `json:"plan_name"`
	VCPUs         int    `json:"vcpus"`
	MemoryMB      int    `json:"memory_mb"`
	StorageGB     int    `json:"storage_gb"`
	DatabaseName  string `json:"database_name"`
	AutoBackup    *bool  `json:"auto_backup"`
}

// DatabaseCredentials are returned once by Create and on demand by the
// owner-only credentials endpoint. They are never stored in plans or events.
type DatabaseCredentials struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	DatabaseName string `json:"database_name"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	URI          string `json:"uri"`
}

// Plans returns the sizing presets.
func (s *DatabaseUsecase) Plans() []DatabasePlan { return AvailableDatabasePlans }

// List returns the user's databases.
func (s *DatabaseUsecase) List(ctx context.Context, userID uuid.UUID) ([]models.ManagedDatabase, error) {
	return s.repos.ListByOwner(ctx, userID)
}

// Get returns one database owned by the user.
func (s *DatabaseUsecase) Get(ctx context.Context, id, userID uuid.UUID) (*models.ManagedDatabase, error) {
	db, err := s.repos.FindByID(ctx, id, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, errDatabaseNotFound
		}
		return nil, err
	}
	return db, nil
}

func normalizeDatabaseInput(in *CreateDatabaseInput) error {
	in.Name = strings.ToLower(strings.TrimSpace(in.Name))
	in.Engine = strings.ToLower(strings.TrimSpace(in.Engine))
	if !databaseNameRegex.MatchString(in.Name) {
		return ValidationError("name", "must be 2-63 lowercase letters, digits or hyphens and start with a letter")
	}
	switch in.Engine {
	case models.DatabaseEnginePostgres, models.DatabaseEngineMySQL, models.DatabaseEngineRedis, models.DatabaseEngineMongo:
	default:
		return ValidationError("engine", "must be postgresql, mysql, redis or mongodb")
	}
	if in.PlanName == "" {
		in.PlanName = "starter"
	}
	if in.PlanName != "custom" {
		found := false
		for _, p := range AvailableDatabasePlans {
			if p.ID == in.PlanName {
				in.VCPUs, in.MemoryMB, in.StorageGB = p.VCPUs, p.MemoryMB, p.StorageGB
				found = true
				break
			}
		}
		if !found {
			return ValidationError("plan_name", "must be starter, standard, power or custom")
		}
	}
	if in.VCPUs <= 0 {
		in.VCPUs = 1
	}
	if in.MemoryMB <= 0 {
		in.MemoryMB = 1024
	}
	if in.StorageGB <= 0 {
		in.StorageGB = 10
	}
	if in.VCPUs > 16 || in.MemoryMB > 65536 || in.StorageGB > 1024 {
		return ValidationError("plan_name", "the requested size exceeds the managed database limits")
	}
	if in.DatabaseName == "" {
		in.DatabaseName = strings.ReplaceAll(in.Name, "-", "_")
	}
	if !regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`).MatchString(in.DatabaseName) {
		return ValidationError("database_name", "must start with a letter and contain lowercase letters, digits or underscores")
	}
	return nil
}

func enginePort(engine string) int {
	switch engine {
	case models.DatabaseEngineMySQL:
		return 3306
	case models.DatabaseEngineRedis:
		return 6379
	case models.DatabaseEngineMongo:
		return 27017
	}
	return 5432
}

func engineVersion(engine, requested string) string {
	if requested != "" {
		return requested
	}
	switch engine {
	case models.DatabaseEngineMySQL:
		return "8.4"
	case models.DatabaseEngineRedis:
		return "7.4"
	case models.DatabaseEngineMongo:
		return "7.0"
	}
	return "16"
}

// Create provisions a database. The returned password is shown exactly once
// and is stored encrypted; the orchestrator tool discards it.
func (s *DatabaseUsecase) Create(ctx context.Context, userID uuid.UUID, in CreateDatabaseInput, clientIP string) (*models.ManagedDatabase, string, error) {
	ctx = context.WithoutCancel(ctx)
	if err := normalizeDatabaseInput(&in); err != nil {
		return nil, "", err
	}
	if existing, err := s.repos.ListByOwner(ctx, userID); err == nil {
		for _, db := range existing {
			if strings.EqualFold(db.Name, in.Name) {
				return nil, "", newError(KindConflict, "NAME_TAKEN", fmt.Sprintf("database name %q is already taken", in.Name))
			}
		}
	}
	if s.quota != nil {
		usage, err := s.quota.GetUsage(ctx, userID)
		if err != nil {
			return nil, "", err
		}
		vcpus, mem, disk, count, _ := s.repos.SumResources(ctx, userID)
		if usage.UsedVCPUs+vcpus+in.VCPUs > usage.MaxVCPUs {
			return nil, "", ValidationError("plan_name", "the database exceeds the vCPU quota")
		}
		if usage.UsedMemoryMB+mem+in.MemoryMB > usage.MaxMemoryMB {
			return nil, "", ValidationError("plan_name", "the database exceeds the memory quota")
		}
		if usage.UsedStorageGB+disk+in.StorageGB > usage.MaxStorageGB {
			return nil, "", ValidationError("plan_name", "the database exceeds the storage quota")
		}
		if usage.UsedInstances+int(count)+1 > usage.MaxInstances {
			return nil, "", ValidationError("plan_name", "the database exceeds the maximum instance count")
		}
	}
	password, err := randomPassword()
	if err != nil {
		return nil, "", err
	}
	if s.cipher == nil || !s.cipher.Enabled() {
		return nil, "", newError(KindUnavailable, "SECRETS_UNAVAILABLE", "credential encryption is not configured")
	}
	ciphertext, err := s.cipher.Encrypt(password)
	if err != nil {
		return nil, "", newError(KindUnavailable, "SECRETS_UNAVAILABLE", "credential encryption is not configured")
	}
	autoBackup := true
	if in.AutoBackup != nil {
		autoBackup = *in.AutoBackup
	}
	now := s.now()
	var nextBackup *time.Time
	if autoBackup {
		next := now.Add(s.backupInterval)
		nextBackup = &next
	}
	db := &models.ManagedDatabase{
		OwnerID: userID, Name: in.Name, Engine: in.Engine, EngineVersion: engineVersion(in.Engine, in.EngineVersion),
		Status: models.DatabaseStatusRunning, PlanName: in.PlanName,
		VCPUs: in.VCPUs, MemoryMB: in.MemoryMB, StorageGB: in.StorageGB,
		Host: fmt.Sprintf("db-%s.nevatal.internal", in.Name), Port: enginePort(in.Engine),
		DatabaseName: in.DatabaseName, Username: "ndb_" + uuid.NewString()[:8],
		PasswordCiphertext: ciphertext, AutoBackup: autoBackup, LastHealthCheckAt: &now,
		NextBackupAt: nextBackup, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repos.Create(ctx, db); err != nil {
		if err == repository.ErrDuplicate {
			return nil, "", newError(KindConflict, "NAME_TAKEN", fmt.Sprintf("database name %q is already taken", in.Name))
		}
		return nil, "", err
	}
	s.audit.Log(ctx, &userID, "database.create", "managed_database", &db.ID, "success", clientIP, map[string]any{"engine": db.Engine, "plan": db.PlanName})
	return db, password, nil
}

// Credentials decrypts the owner's database password. Every access is
// audited; the password is never written to logs or events.
func (s *DatabaseUsecase) Credentials(ctx context.Context, id, userID uuid.UUID, clientIP string) (*DatabaseCredentials, error) {
	db, err := s.Get(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	password, err := s.cipher.Decrypt(db.PasswordCiphertext)
	if err != nil {
		return nil, newError(KindUnavailable, "CREDENTIALS_UNAVAILABLE", "the stored credential cannot be decrypted")
	}
	s.audit.Log(ctx, &userID, "database.credentials", "managed_database", &db.ID, "success", clientIP, nil)
	uri := fmt.Sprintf("%s://%s:%s@%s:%d/%s", db.Engine, db.Username, password, db.Host, db.Port, db.DatabaseName)
	if db.Engine == models.DatabaseEngineRedis {
		uri = fmt.Sprintf("redis://:%s@%s:%d", password, db.Host, db.Port)
	}
	return &DatabaseCredentials{
		Host: db.Host, Port: db.Port, DatabaseName: db.DatabaseName,
		Username: db.Username, Password: password, URI: uri,
	}, nil
}

// UpdatePower starts, stops or restarts a database.
func (s *DatabaseUsecase) UpdatePower(ctx context.Context, id, userID uuid.UUID, action, clientIP string) (*models.ManagedDatabase, error) {
	ctx = context.WithoutCancel(ctx)
	db, err := s.Get(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	switch action {
	case "start", "restart":
		db.Status = models.DatabaseStatusRunning
	case "stop":
		db.Status = models.DatabaseStatusStopped
	default:
		return nil, ValidationError("action", "must be start, stop or restart")
	}
	now := s.now()
	db.LastHealthCheckAt, db.UpdatedAt = &now, now
	if err := s.repos.Save(ctx, db); err != nil {
		return nil, err
	}
	s.audit.Log(ctx, &userID, "database."+action, "managed_database", &db.ID, "success", clientIP, nil)
	return db, nil
}

// Delete soft-deletes a database.
func (s *DatabaseUsecase) Delete(ctx context.Context, id, userID uuid.UUID, clientIP string) error {
	ctx = context.WithoutCancel(ctx)
	db, err := s.Get(ctx, id, userID)
	if err != nil {
		return err
	}
	if err := s.repos.SoftDelete(ctx, id, userID); err != nil {
		return err
	}
	s.audit.Log(ctx, &userID, "database.delete", "managed_database", &db.ID, "success", clientIP, nil)
	return nil
}

// DatabaseMetrics is the observed usage of a database.
type DatabaseMetrics struct {
	CPUPercent          float64   `json:"cpu_percent"`
	MemoryUsagePercent  float64   `json:"memory_usage_percent"`
	StorageUsagePercent float64   `json:"storage_usage_percent"`
	ConnectionCount     int       `json:"connection_count"`
	Status              string    `json:"status"`
	Timestamp           time.Time `json:"timestamp"`
}

// Metrics returns usage telemetry. With the simulated platform the values
// are deterministic from the database name so dashboards stay stable.
func (s *DatabaseUsecase) Metrics(ctx context.Context, id, userID uuid.UUID) (*DatabaseMetrics, error) {
	db, err := s.Get(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	cpu, mem, disk, conns := db.CPUPercent, db.MemoryUsagePercent, db.StorageUsagePercent, db.ConnectionCount
	if s.cfg.ProxmoxMock || (cpu == 0 && mem == 0 && disk == 0) {
		h := fnv.New32a()
		_, _ = h.Write([]byte(db.Name))
		seed := float64(h.Sum32()%30) + 8
		if db.Status != models.DatabaseStatusRunning {
			seed = 0
		}
		cpu, mem, disk = seed, 20+float64(h.Sum32()%40), 15+float64(h.Sum32()%35)
		conns = int(h.Sum32() % 40)
	}
	return &DatabaseMetrics{
		CPUPercent: round1(cpu), MemoryUsagePercent: round1(mem), StorageUsagePercent: round1(disk),
		ConnectionCount: conns, Status: db.Status, Timestamp: time.Now(),
	}, nil
}

// Logs returns recent (simulated) database log lines, never credentials.
func (s *DatabaseUsecase) Logs(ctx context.Context, id, userID uuid.UUID, lines int) ([]string, error) {
	db, err := s.Get(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if lines <= 0 || lines > 2000 {
		lines = 100
	}
	now := time.Now().UTC()
	base := []string{
		fmt.Sprintf("database %s (%s %s) listening on %s:%d", db.DatabaseName, db.Engine, db.EngineVersion, db.Host, db.Port),
		"checkpoint complete: wrote data files",
		fmt.Sprintf("connection accepted from an application pool (%s)", db.Username),
		"automatic vacuum completed",
		"connection closed by client",
	}
	out := make([]string, 0, lines)
	for i := 0; i < lines; i++ {
		out = append(out, fmt.Sprintf("%s [%s] %s", now.Add(-time.Duration(lines-i)*time.Second).Format(time.RFC3339), db.Status, base[i%len(base)]))
	}
	return out, nil
}

// CreateBackup records a logical backup.
func (s *DatabaseUsecase) CreateBackup(ctx context.Context, id, userID uuid.UUID, name, clientIP string) (*models.ManagedDatabaseBackup, error) {
	ctx = context.WithoutCancel(ctx)
	db, err := s.Get(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if db.Status == models.DatabaseStatusDeleting {
		return nil, newError(KindConflict, "INVALID_STATE", "the database is being deleted")
	}
	now := s.now()
	if name == "" {
		name = fmt.Sprintf("%s-%s", db.Name, now.Format("20060102-150405"))
	}
	backup := &models.ManagedDatabaseBackup{
		DatabaseID: db.ID, Name: name, Status: "completed",
		BackupReference: fmt.Sprintf("s3://backups/databases/%s/%s.dump", db.ID, now.Format("20060102T150405")),
		SizeBytes:       int64(db.StorageGB) * 1024 * 1024 * 1024 / 20, CreatedAt: now,
	}
	if err := s.repos.CreateBackup(ctx, backup); err != nil {
		return nil, err
	}
	_ = s.repos.SetLastBackup(ctx, db.ID)
	s.audit.Log(ctx, &userID, "database.backup.create", "managed_database", &db.ID, "success", clientIP, map[string]any{"backup_id": backup.ID})
	return backup, nil
}

// ListBackups returns the database's backups.
func (s *DatabaseUsecase) ListBackups(ctx context.Context, id, userID uuid.UUID) ([]models.ManagedDatabaseBackup, error) {
	if _, err := s.Get(ctx, id, userID); err != nil {
		return nil, err
	}
	return s.repos.ListBackups(ctx, id)
}

// RestoreBackup restores the database to a backup (logical: the database
// becomes running and the restore is recorded).
func (s *DatabaseUsecase) RestoreBackup(ctx context.Context, id, backupID, userID uuid.UUID, clientIP string) error {
	ctx = context.WithoutCancel(ctx)
	db, err := s.Get(ctx, id, userID)
	if err != nil {
		return err
	}
	if _, err := s.repos.FindBackup(ctx, backupID, id); err != nil {
		return newError(KindNotFound, "BACKUP_NOT_FOUND", "backup not found")
	}
	now := s.now()
	db.Status, db.LastHealthCheckAt, db.UpdatedAt = models.DatabaseStatusRunning, &now, now
	if err := s.repos.Save(ctx, db); err != nil {
		return err
	}
	s.audit.Log(ctx, &userID, "database.backup.restore", "managed_database", &db.ID, "success", clientIP, map[string]any{"backup_id": backupID})
	return nil
}

// DeleteBackup removes one backup record.
func (s *DatabaseUsecase) DeleteBackup(ctx context.Context, id, backupID, userID uuid.UUID, clientIP string) error {
	if _, err := s.Get(ctx, id, userID); err != nil {
		return err
	}
	if err := s.repos.DeleteBackup(ctx, backupID, id); err != nil {
		if err == repository.ErrNotFound {
			return newError(KindNotFound, "BACKUP_NOT_FOUND", "backup not found")
		}
		return err
	}
	s.audit.Log(ctx, &userID, "database.backup.delete", "managed_database", &id, "success", clientIP, map[string]any{"backup_id": backupID})
	return nil
}

// SetBackupInterval changes the automatic backup cadence (tests use a short
// one).
func (s *DatabaseUsecase) SetBackupInterval(d time.Duration) {
	if d > 0 {
		s.backupInterval = d
	}
}

// StartBackups runs the automatic backup loop until ctx is cancelled. Every
// API replica may run it: due databases are claimed atomically, so a backup is
// never taken twice.
func (s *DatabaseUsecase) StartBackups(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		s.RunAutoBackups(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.RunAutoBackups(ctx)
			}
		}
	}()
}

// RunAutoBackups takes a logical backup of every due database whose
// auto_backup is enabled. A failure is logged and retried at the next
// interval; it never changes the database status.
func (s *DatabaseUsecase) RunAutoBackups(ctx context.Context) {
	due, err := s.repos.ClaimDueBackups(ctx, s.now(), s.backupInterval, 20)
	if err != nil {
		log.Printf("Warning: automatic database backups could not be claimed: %v", err)
		return
	}
	for i := range due {
		db := &due[i]
		if _, err := s.CreateBackup(ctx, db.ID, db.OwnerID, "", "scheduler"); err != nil {
			log.Printf("Warning: automatic backup of database %s failed: %v", db.ID, err)
		}
	}
}

// Health returns the recorded status and last check time.
func (s *DatabaseUsecase) Health(ctx context.Context, id, userID uuid.UUID) (string, *time.Time, error) {
	db, err := s.Get(ctx, id, userID)
	if err != nil {
		return "", nil, err
	}
	return db.Status, db.LastHealthCheckAt, nil
}

func randomPassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
