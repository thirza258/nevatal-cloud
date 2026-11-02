// Package integration holds Postgres-backed tests of the AI orchestrator:
// task leasing (FOR UPDATE SKIP LOCKED), idempotency under concurrency, the
// stale-job reaper, approvals and the HTTP contract.
//
// The tests use NEVATAL_TEST_DATABASE_URL (a URL of any database on the
// server; default: the QA container postgres://postgres:smokepw@127.0.0.1:55433)
// and create a throwaway database nevatal_orch_it_<pid>. They are skipped
// when the server is unreachable.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/database"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
	"github.com/nevatal/cloud/internal/proxmox"
	"github.com/nevatal/cloud/internal/repository"
	"github.com/nevatal/cloud/internal/usecase"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const defaultTestDB = "postgres://postgres:smokepw@127.0.0.1:55433/postgres?sslmode=disable&connect_timeout=3"

var (
	testDB     *gorm.DB
	skipReason string
	storageDir string
	userSeq    atomic.Int64
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(run(m))
}

func run(m *testing.M) int {
	base := os.Getenv("NEVATAL_TEST_DATABASE_URL")
	if base == "" {
		base = defaultTestDB
	}
	quiet := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(base), quiet)
	if err == nil {
		var sqlDB interface{ Ping() error }
		if sqlDB, err = admin.DB(); err == nil {
			err = sqlDB.Ping()
		}
	}
	if err != nil {
		skipReason = fmt.Sprintf("Postgres not reachable at NEVATAL_TEST_DATABASE_URL (%v)", err)
		return m.Run()
	}
	name := fmt.Sprintf("nevatal_orch_it_%d", os.Getpid())
	admin.Exec("DROP DATABASE IF EXISTS " + name)
	if err := admin.Exec("CREATE DATABASE " + name).Error; err != nil {
		fmt.Fprintf(os.Stderr, "create test database: %v\n", err)
		return 1
	}
	defer admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")

	u, _ := url.Parse(base)
	u.Path = "/" + name
	testDB, err = database.Connect(u.String(), 40, 10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		return 1
	}
	testDB.Logger = logger.Default.LogMode(logger.Silent)
	if err := database.RunMigrations(testDB); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		return 1
	}
	storageDir, _ = os.MkdirTemp("", "nevatal-it-storage")
	defer os.RemoveAll(storageDir)
	if err := database.Seed(testDB, baseConfig()); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		return 1
	}
	code := m.Run()
	if sqlDB, err := testDB.DB(); err == nil {
		_ = sqlDB.Close()
	}
	return code
}

func baseConfig() *config.Config {
	return &config.Config{
		Env:             config.EnvDevelopment,
		DemoMode:        false,
		ProxmoxMock:     true,
		ProxmoxNodeName: "pve-dell-r640",
		ProxmoxStorage:  "local-zfs",
		AdminEmail:      "admin@it.local",
		AdminPassword:   "ItAdminPassword123!",
		JWTSecret:       "integration-test-secret-0123456789abcdef",
		StoragePath:     storageDir,
		AppBaseURL:      "http://localhost",
	}
}

func needDB(t *testing.T) {
	t.Helper()
	if testDB == nil {
		t.Skip(skipReason)
	}
}

// stack is the API composed like cmd/api/main.go, on the test database.
type stack struct {
	t        *testing.T
	cfg      *config.Config
	tx       repository.Transactor
	pve      proxmox.ProxmoxClient
	reg      *orchestrator.Registry
	orchRepo repository.OrchestratorRepository
	jobRepo  repository.JobRepository
	instRepo repository.InstanceRepository
	agent    repository.AIAgentRepository
	quota    *usecase.QuotaUsecase
	auth     *usecase.AuthUsecase
	worker   *usecase.WorkerUsecase
	instance *usecase.InstanceUsecase
	storage  *usecase.StorageUsecase
	orch     *usecase.OrchestratorUsecase
	mcp      *usecase.MCPUsecase
	agentUC  *usecase.AIAgentUsecase
	messages repository.AIAssistantRepository
	server   *httptest.Server
	engines  []*orchestrator.Engine
}

type stackOpts struct {
	engine      orchestrator.EngineConfig
	mockLatency time.Duration
	tools       func(reg *orchestrator.Registry)
	noEngine    bool
}

// resetOrchestrator removes every execution so engines of one test never
// see tasks of another.
func resetOrchestrator(t *testing.T) {
	t.Helper()
	if err := testDB.Exec("TRUNCATE ai_executions CASCADE").Error; err != nil {
		t.Fatal(err)
	}
}

func newStack(t *testing.T, opts stackOpts) *stack {
	needDB(t)
	resetOrchestrator(t)
	cfg := baseConfig()
	cfg.ProxmoxMockLatency = opts.mockLatency
	db := testDB
	tx := repository.NewTransactor(db)
	auditRepo := repository.NewAuditRepository(db)
	userRepo := repository.NewUserRepository(db)
	quotaRepo := repository.NewQuotaRepository(db)
	reservationRepo := repository.NewReservationRepository(db)
	instanceRepo := repository.NewInstanceRepository(db)
	jobRepo := repository.NewJobRepository(db)
	templateRepo := repository.NewTemplateRepository(db)
	planRepo := repository.NewPlanRepository(db)
	sshKeyRepo := repository.NewSSHKeyRepository(db)
	nodeRepo := repository.NewNodeRepository(db)
	snapshotRepo := repository.NewSnapshotRepository(db)
	storageRepo := repository.NewStorageRepository(db)
	portRepo := repository.NewPortRepository(db)
	gameServerRepo := repository.NewGameServerRepository(db)
	androidRepo := repository.NewAndroidRepository(db)
	appRepo := repository.NewAppRepository(db)
	automationRepo := repository.NewAutomationRepository(db)
	openClawRepo := repository.NewOpenClawRepository(db)
	mcpRepo := repository.NewMCPRepository(db)
	aiAgentRepo := repository.NewAIAgentRepository(db)
	orchRepo := repository.NewOrchestratorRepository(db)

	pve := proxmox.NewClient(cfg)
	auditUC := usecase.NewAuditUsecase(auditRepo)
	authUC := usecase.NewAuthUsecase(tx, userRepo, quotaRepo, auditUC, cfg)
	quotaUC := usecase.NewQuotaUsecase(quotaRepo, instanceRepo, reservationRepo)
	workerUC := usecase.NewWorkerUsecase(tx, usecase.WorkerRepos{Jobs: jobRepo, Instances: instanceRepo, Templates: templateRepo, Nodes: nodeRepo}, cfg, pve, quotaUC)
	instanceUC := usecase.NewInstanceUsecase(tx, usecase.InstanceRepos{
		Instances: instanceRepo, Jobs: jobRepo, Templates: templateRepo, Plans: planRepo, SSHKeys: sshKeyRepo,
		Nodes: nodeRepo, Snapshots: snapshotRepo, Audits: auditRepo,
	}, quotaUC, auditUC, workerUC, pve)
	appUC := usecase.NewAppUsecase(tx, appRepo, auditUC)
	automationUC := usecase.NewAutomationUsecase(automationRepo, auditUC)
	openClawUC := usecase.NewOpenClawUsecase(openClawRepo, auditUC)
	storageUC := usecase.NewStorageUsecase(storageRepo, cfg.StoragePath)
	portUC := usecase.NewPortUsecase(portRepo)
	gameServerUC := usecase.NewGameServerUsecase(gameServerRepo, cfg, portUC, auditUC, quotaUC, pve)
	androidUC := usecase.NewAndroidUsecase(androidRepo, cfg, auditUC, quotaUC, pve)

	reg := orchestrator.DefaultRegistry()
	if opts.tools != nil {
		opts.tools(reg)
	}
	ecfg := opts.engine
	if ecfg.Lease == 0 {
		ecfg.Lease = 3 * time.Second
	}
	if ecfg.Heartbeat == 0 {
		ecfg.Heartbeat = 500 * time.Millisecond
	}
	if ecfg.PollInterval == 0 {
		ecfg.PollInterval = 100 * time.Millisecond
	}
	if ecfg.MaintenanceInterval == 0 {
		ecfg.MaintenanceInterval = 300 * time.Millisecond
	}
	if ecfg.RetryBase == 0 {
		ecfg.RetryBase = 100 * time.Millisecond
	}
	orchUC := usecase.NewOrchestratorUsecaseWith(tx, usecase.OrchestratorRepos{
		Orch: orchRepo, Agent: aiAgentRepo, Users: userRepo, Templates: templateRepo, Plans: planRepo,
		SSHKeys: sshKeyRepo, Nodes: nodeRepo, Instances: instanceRepo, Snapshots: snapshotRepo,
		Jobs: jobRepo, Android: androidRepo, Storage: storageRepo,
	}, usecase.OrchestratorServices{
		Instance: instanceUC, App: appUC, Automation: automationUC, OpenClaw: openClawUC, Storage: storageUC,
		GameServer: gameServerUC, Android: androidUC, Quota: quotaUC, Audit: auditUC,
	}, cfg, pve, reg, ecfg)
	orchUC.SetJobPollInterval(100 * time.Millisecond)
	mcpUC := usecase.NewMCPUsecase(usecase.MCPRepos{
		MCP: mcpRepo, Templates: templateRepo, Plans: planRepo, Nodes: nodeRepo, Instances: instanceRepo,
		Audits: auditRepo, Jobs: jobRepo,
	}, instanceUC, appUC, automationUC, openClawUC, storageUC, gameServerUC, androidUC, quotaUC, orchUC, pve)
	assistantRepo := repository.NewAIAssistantRepository(db)
	assistantUC := usecase.NewAIAssistantUsecase(usecase.AIAssistantRepos{
		Messages: assistantRepo, Users: userRepo, Templates: templateRepo, Plans: planRepo, SSHKeys: sshKeyRepo,
	}, cfg, instanceUC, quotaUC, auditUC, orchUC)
	agentUC := usecase.NewAIAgentUsecase(usecase.AIAgentRepos{
		Agent: aiAgentRepo, Templates: templateRepo, SSHKeys: sshKeyRepo, Android: androidRepo,
	}, cfg, instanceUC, appUC, automationUC, openClawUC, storageUC, gameServerUC, androidUC, quotaUC, mcpUC, orchUC, pve)

	router := api.SetupRouter(api.RouterParams{
		Cfg: cfg, Auth: authUC, Quota: quotaUC, Audit: auditUC, Instance: instanceUC, Storage: storageUC,
		MCP: mcpUC, AIAgent: agentUC, AIAssistant: assistantUC, Orchestrator: orchUC, Streams: context.Background(),
	})
	s := &stack{
		t: t, cfg: cfg, tx: tx, pve: pve, reg: reg, orchRepo: orchRepo, jobRepo: jobRepo, instRepo: instanceRepo,
		agent: aiAgentRepo, quota: quotaUC, auth: authUC, worker: workerUC, instance: instanceUC,
		storage: storageUC, orch: orchUC, mcp: mcpUC, agentUC: agentUC, messages: assistantRepo,
		server: httptest.NewServer(router),
	}
	if !opts.noEngine {
		s.startEngine(orchUC.Engine())
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, e := range s.engines {
			e.Shutdown(ctx)
		}
		s.server.Close()
		workerUC.Wait(ctx)
	})
	return s
}

func (s *stack) startEngine(e *orchestrator.Engine) {
	e.Start()
	s.engines = append(s.engines, e)
}

// newEngine returns a second engine (another replica) on the same database.
func (s *stack) newEngine(cfg orchestrator.EngineConfig) *orchestrator.Engine {
	if cfg.Lease == 0 {
		cfg.Lease = 3 * time.Second
	}
	if cfg.Heartbeat == 0 {
		cfg.Heartbeat = 500 * time.Millisecond
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 100 * time.Millisecond
	}
	if cfg.MaintenanceInterval == 0 {
		cfg.MaintenanceInterval = 300 * time.Millisecond
	}
	return orchestrator.NewEngine(s.orchRepo, s.tx, s.reg, s.orch, s.orch.Notifier(), cfg)
}

type user struct {
	id    uuid.UUID
	token string
}

func (s *stack) newUser() user {
	s.t.Helper()
	email := fmt.Sprintf("it-%d-%d@example.com", os.Getpid(), userSeq.Add(1))
	u, token, err := s.auth.Register(context.Background(), email, "Password123!", "IT user", "127.0.0.1")
	if err != nil {
		s.t.Fatal(err)
	}
	return user{id: u.ID, token: token}
}

func (s *stack) admin() user {
	s.t.Helper()
	u, token, err := s.auth.Login(context.Background(), "admin@it.local", "ItAdminPassword123!", "127.0.0.1")
	if err != nil {
		s.t.Fatal(err)
	}
	return user{id: u.ID, token: token}
}

func (s *stack) conversation(u user) uuid.UUID {
	s.t.Helper()
	conv, err := s.agentUC.CreateConversation(context.Background(), u.id, "test")
	if err != nil {
		s.t.Fatal(err)
	}
	return conv.ID
}

// do performs an API request and decodes the JSON reply.
func (s *stack) do(method, path string, u user, body any) (int, map[string]any) {
	s.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, s.server.URL+"/api/v1"+path, r)
	req.Header.Set("Content-Type", "application/json")
	if u.token != "" {
		req.Header.Set("Authorization", "Bearer "+u.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func data(m map[string]any) map[string]any {
	d, _ := m["data"].(map[string]any)
	return d
}

func errCode(m map[string]any) string {
	e, _ := m["error"].(map[string]any)
	c, _ := e["code"].(string)
	return c
}

func (s *stack) template() uuid.UUID {
	s.t.Helper()
	var tpl models.OSTemplate
	if err := testDB.Where("enabled = true").Order("name").First(&tpl).Error; err != nil {
		s.t.Fatalf("no OS template seeded: %v", err)
	}
	return tpl.ID
}

func vpsSpec(name string, tpl uuid.UUID, memoryMB int) orchestrator.Specification {
	return orchestrator.Specification{Intent: "create_vps", Resources: []orchestrator.ResourceSpec{{
		Key: name, Type: orchestrator.TypeVPS, Action: orchestrator.ActionCreate,
		Config: map[string]any{"name": name, "template_id": tpl.String(), "vcpus": 1, "memory_mb": memoryMB, "disk_gb": 20},
	}}}
}

// plan creates a plan through the planner entry point.
func (s *stack) plan(u user, spec orchestrator.Specification) (uuid.UUID, *orchestrator.Revision) {
	s.t.Helper()
	p, rev, err := s.orch.CreatePlan(context.Background(), usecase.PlanDraft{
		ConversationID: s.conversation(u), UserID: u.id, Intent: spec.Intent, Spec: spec,
	})
	if err != nil {
		s.t.Fatal(err)
	}
	return p.ID, rev
}

func (s *stack) waitExecution(id uuid.UUID, timeout time.Duration) *models.AIExecution {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		e, err := s.orchRepo.FindExecution(context.Background(), id, nil)
		if err != nil {
			s.t.Fatal(err)
		}
		switch e.Status {
		case models.ExecCompleted, models.ExecFailed, models.ExecPartiallyCompleted, models.ExecCancelled:
			return e
		}
		if time.Now().After(deadline) {
			tasks, _ := s.orchRepo.ListTasks(context.Background(), id)
			for _, t := range tasks {
				s.t.Logf("task %s: %s attempt %d err %v", t.Key, t.Status, t.Attempt, ptr(t.Error))
			}
			s.t.Fatalf("execution %s still %s after %s", id, e.Status, timeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func ptr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *stack) tasks(execID uuid.UUID) map[string]models.AIExecutionTask {
	s.t.Helper()
	list, err := s.orchRepo.ListTasks(context.Background(), execID)
	if err != nil {
		s.t.Fatal(err)
	}
	out := map[string]models.AIExecutionTask{}
	for _, t := range list {
		out[t.Key] = t
	}
	return out
}

func (s *stack) events(execID uuid.UUID) []models.AIExecutionEvent {
	s.t.Helper()
	evs, err := s.orchRepo.ListEvents(context.Background(), execID, 0, 500)
	if err != nil {
		s.t.Fatal(err)
	}
	return evs
}

// taskDef describes a task of a hand-built execution.
type taskDef struct {
	key, tool string
	deps      []string
	risk      string
	input     map[string]any
}

// execution creates an execution directly (no plan approval), for engine
// tests with test tools.
func (s *stack) execution(u user, defs []taskDef) uuid.UUID {
	s.t.Helper()
	ctx := context.Background()
	conv := s.conversation(u)
	plan := models.AIResourcePlan{ConversationID: conv, UserID: u.id, Version: 1, Intent: "test", Status: models.PlanStatusApproved,
		Title: "engine test", PlanJSON: "{}", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.agent.CreatePlan(ctx, &plan); err != nil {
		s.t.Fatal(err)
	}
	exec := models.AIExecution{PlanID: plan.ID, PlanVersion: 1, ConversationID: conv, UserID: u.id, Status: models.ExecQueued,
		RecoveryOptions: "[]", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.orchRepo.CreateExecution(ctx, &exec); err != nil {
		s.t.Fatal(err)
	}
	var tasks []models.AIExecutionTask
	for i, d := range defs {
		status := models.TaskQueued
		if len(d.deps) > 0 {
			status = models.TaskBlocked
		}
		deps := d.deps
		if deps == nil {
			deps = []string{}
		}
		risk := d.risk
		if risk == "" {
			risk = orchestrator.RiskRead
		}
		in := d.input
		if in == nil {
			in = map[string]any{}
		}
		def, _ := s.reg.Def(d.tool)
		tasks = append(tasks, models.AIExecutionTask{
			ExecutionID: exec.ID, Position: i, Key: d.key, Tool: d.tool, Title: d.key, Risk: risk,
			DependsOn: orchestrator.MustJSON(deps), Input: orchestrator.MustJSON(in), Status: status,
			Attempt: 1, MaxAttempts: def.MaxAttempts, TimeoutSeconds: def.TimeoutSeconds,
			IdempotencyKey: orchestrator.IdempotencyKey(exec.ID, d.key, 1),
			CreatedAt:      exec.CreatedAt.Add(time.Duration(i) * time.Microsecond), UpdatedAt: time.Now(),
		})
	}
	if err := s.orchRepo.CreateTasks(ctx, tasks); err != nil {
		s.t.Fatal(err)
	}
	for _, e := range s.engines {
		e.Nudge()
	}
	return exec.ID
}

// recorder is a test tool that records its runs.
type recorder struct {
	mu    sync.Mutex
	runs  map[string][]toolRun
	calls atomic.Int64
}

type toolRun struct {
	start, end time.Time
	attempt    int
	key        string
}

func newRecorder() *recorder { return &recorder{runs: map[string][]toolRun{}} }

func (r *recorder) add(task string, rn toolRun) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs[task] = append(r.runs[task], rn)
}

func (r *recorder) get(task string) []toolRun {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]toolRun(nil), r.runs[task]...)
}

func testTool(reg *orchestrator.Registry, id, risk string, fn func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error)) {
	reg.Register(orchestrator.ToolDef{ID: id, Title: id, Description: "test tool", Risk: risk, Idempotent: true,
		TimeoutSeconds: 30, InputSchema: map[string]any{"type": "object"}})
	reg.Bind(id, orchestrator.RunnerFunc(fn))
}

type sseFrame struct {
	id    int64
	event string
	data  string
}

// readSSE reads an event stream until the server closes it.
func readSSE(url, token string, lastEventID int64, timeout time.Duration) ([]sseFrame, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")
	if lastEventID > 0 {
		req.Header.Set("Last-Event-ID", fmt.Sprint(lastEventID))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/event-stream" || resp.Header.Get("X-Accel-Buffering") != "no" {
		return nil, fmt.Errorf("status %d, content type %q, X-Accel-Buffering %q", resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("X-Accel-Buffering"))
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var frames []sseFrame
	for _, block := range bytes.Split(raw, []byte("\n\n")) {
		var f sseFrame
		for _, line := range bytes.Split(block, []byte("\n")) {
			switch {
			case bytes.HasPrefix(line, []byte("id: ")):
				fmt.Sscan(string(line[4:]), &f.id)
			case bytes.HasPrefix(line, []byte("event: ")):
				f.event = string(line[7:])
			case bytes.HasPrefix(line, []byte("data: ")):
				f.data = string(line[6:])
			}
		}
		if f.event != "" {
			frames = append(frames, f)
		}
	}
	return frames, nil
}
