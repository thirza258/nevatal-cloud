package integration

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
)

// nonIdempotentTool registers a test tool that is not idempotent.
func nonIdempotentTool(reg *orchestrator.Registry, id string, timeoutSeconds int, fn func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error)) {
	reg.Register(orchestrator.ToolDef{ID: id, Title: id, Description: "test tool", Risk: orchestrator.RiskDestructive, Idempotent: false,
		TimeoutSeconds: timeoutSeconds, InputSchema: map[string]any{"type": "object"}})
	reg.Bind(id, orchestrator.RunnerFunc(fn))
}

func taskErrorCode(t models.AIExecutionTask) string {
	if t.Error == nil {
		return ""
	}
	var te orchestrator.TaskError
	_ = json.Unmarshal([]byte(*t.Error), &te)
	return te.Code
}

func TestNonIdempotentToolsAreNeverRetriedAutomatically(t *testing.T) {
	if def, _ := orchestrator.DefaultRegistry().Def(orchestrator.ToolVPSSnapshotRestore); def.Idempotent || def.MaxAttempts != 1 {
		t.Fatalf("vps.snapshot.restore: idempotent=%v max_attempts=%d, want false/1", def.Idempotent, def.MaxAttempts)
	}
	var errCalls, slowCalls atomic.Int64
	s := newStack(t, stackOpts{engine: orchestrator.EngineConfig{RetryBase: 50 * time.Millisecond}, tools: func(reg *orchestrator.Registry) {
		nonIdempotentTool(reg, "test.restore_err", 30, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
			errCalls.Add(1)
			return nil, orchestrator.Retryable("PVE_BUSY", "the hypervisor was busy")
		})
		nonIdempotentTool(reg, "test.restore_slow", 1, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
			slowCalls.Add(1)
			<-ctx.Done()
			return nil, ctx.Err()
		})
	}})
	u := s.newUser()
	id := s.execution(u, []taskDef{
		{key: "err", tool: "test.restore_err", risk: orchestrator.RiskDestructive},
		{key: "slow", tool: "test.restore_slow", risk: orchestrator.RiskDestructive},
	})
	e := s.waitExecution(id, 15*time.Second)
	time.Sleep(300 * time.Millisecond) // a scheduled retry would have run by now
	tasks := s.tasks(id)
	if e.Status != models.ExecFailed || errCalls.Load() != 1 || slowCalls.Load() != 1 {
		t.Fatalf("status %s, calls err=%d slow=%d: a non-idempotent tool must run once", e.Status, errCalls.Load(), slowCalls.Load())
	}
	if tk := tasks["err"]; tk.Status != models.TaskFailed || tk.Attempt != 1 || tk.MaxAttempts != 1 || taskErrorCode(tk) != "PVE_BUSY" {
		t.Fatalf("retryable error: %s attempt %d/%d %s", tk.Status, tk.Attempt, tk.MaxAttempts, taskErrorCode(tk))
	}
	if tk := tasks["slow"]; tk.Status != models.TaskFailed || tk.Attempt != 1 || taskErrorCode(tk) != "TOOL_TIMEOUT" {
		t.Fatalf("timeout: %s attempt %d %s", tk.Status, tk.Attempt, taskErrorCode(tk))
	}
	for _, ev := range s.events(id) {
		if ev.Type == orchestrator.EventTaskRetrying {
			t.Fatalf("automatic retry event: %s", ev.Message)
		}
	}
	// The user may retry it: exactly one more run.
	if _, err := s.orch.RetryTask(context.Background(), id, tasks["err"].ID, u.id, "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	s.waitExecution(id, 15*time.Second)
	time.Sleep(300 * time.Millisecond)
	if tk := s.tasks(id)["err"]; errCalls.Load() != 2 || tk.Attempt != 2 || tk.Status != models.TaskFailed {
		t.Fatalf("manual retry: calls %d, attempt %d, %s", errCalls.Load(), tk.Attempt, tk.Status)
	}
}

func TestReclaimedNonIdempotentTaskIsNotRunAgain(t *testing.T) {
	var calls atomic.Int64
	s := newStack(t, stackOpts{noEngine: true, tools: func(reg *orchestrator.Registry) {
		nonIdempotentTool(reg, "test.rollback", 30, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
			calls.Add(1)
			return map[string]any{"rolled_back": true}, nil
		})
	}})
	u := s.newUser()
	id := s.execution(u, []taskDef{{key: "rollback", tool: "test.rollback", risk: orchestrator.RiskDestructive}})
	// A replica started the restore and died: running, lease expired.
	if err := testDB.Exec(`UPDATE ai_execution_tasks SET status = 'running', lease_owner = 'dead-replica', lease_token = ?,
		lease_expires_at = now() - interval '1 minute', claim_count = 1, started_at = now() - interval '2 minutes'
		WHERE execution_id = ?`, uuid.New(), id).Error; err != nil {
		t.Fatal(err)
	}
	testDB.Exec(`UPDATE ai_executions SET status = 'running', started_at = now() WHERE id = ?`, id)
	s.startEngine(s.newEngine(orchestrator.EngineConfig{Owner: "replica-a"}))
	e := s.waitExecution(id, 15*time.Second)
	tk := s.tasks(id)["rollback"]
	if e.Status != models.ExecFailed || calls.Load() != 0 || tk.Status != models.TaskFailed ||
		taskErrorCode(tk) != orchestrator.ErrCodeInterruptedNotIdempotent {
		t.Fatalf("status %s, calls %d, task %s %s: a re-claimed non-idempotent task must fail, not run again",
			e.Status, calls.Load(), tk.Status, taskErrorCode(tk))
	}
	if _, err := s.orch.RetryTask(context.Background(), id, tk.ID, u.id, "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if e := s.waitExecution(id, 15*time.Second); e.Status != models.ExecCompleted || calls.Load() != 1 {
		t.Fatalf("manual retry: %s, calls %d", e.Status, calls.Load())
	}
}

func recoveryActions(body map[string]any) []string {
	out := []string{}
	opts, _ := data(body)["recovery_options"].([]any)
	for _, o := range opts {
		if m, ok := o.(map[string]any); ok {
			out = append(out, m["action"].(string))
		}
	}
	return out
}

func TestDeleteCreatedOnlyOfferedWhenSomethingCanBeDeleted(t *testing.T) {
	var target atomic.Value
	s := newStack(t, stackOpts{tools: func(reg *orchestrator.Registry) {
		testTool(reg, "test.modify", orchestrator.RiskMedium, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
			id := target.Load().(uuid.UUID)
			tc.RecordResource(models.RelationModified, orchestrator.TypeVPS, id, "rec-mod")
			return map[string]any{}, nil
		})
		testTool(reg, "test.make", orchestrator.RiskMedium, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
			id := target.Load().(uuid.UUID)
			tc.RecordResource(models.RelationCreated, orchestrator.TypeVPS, id, "rec-made")
			return map[string]any{}, nil
		})
		testTool(reg, "test.boom", orchestrator.RiskMedium, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
			return nil, orchestrator.Permanent("BOOM", "failed on purpose")
		})
	}})
	u := s.newUser()
	inst := s.runningInstance(u, "rec-01")
	target.Store(inst.ID)

	// Only a modified resource: partially completed, but nothing to delete.
	id := s.execution(u, []taskDef{
		{key: "mod", tool: "test.modify", risk: orchestrator.RiskMedium},
		{key: "boom", tool: "test.boom", risk: orchestrator.RiskMedium},
	})
	if e := s.waitExecution(id, 15*time.Second); e.Status != models.ExecPartiallyCompleted {
		t.Fatalf("status %s", e.Status)
	}
	_, body := s.do("GET", "/ai/executions/"+id.String(), u, nil)
	if got := strings.Join(recoveryActions(body), ","); got != "retry_failed,keep" {
		t.Fatalf("modified-only execution offers %s", got)
	}

	// A created resource that still exists: delete_created is offered.
	id = s.execution(u, []taskDef{
		{key: "make", tool: "test.make", risk: orchestrator.RiskMedium},
		{key: "boom", tool: "test.boom", risk: orchestrator.RiskMedium},
	})
	s.waitExecution(id, 15*time.Second)
	_, body = s.do("GET", "/ai/executions/"+id.String(), u, nil)
	if got := strings.Join(recoveryActions(body), ","); got != "retry_failed,keep,delete_created" {
		t.Fatalf("execution with a created VPS offers %s", got)
	}
	// Once the resource is gone, the option disappears (and still answers 409).
	if err := testDB.Delete(&models.Instance{}, "id = ?", inst.ID).Error; err != nil {
		t.Fatal(err)
	}
	_, body = s.do("GET", "/ai/executions/"+id.String(), u, nil)
	if got := strings.Join(recoveryActions(body), ","); got != "retry_failed,keep" {
		t.Fatalf("execution whose created VPS is gone offers %s", got)
	}
	_, list := s.do("GET", "/ai/executions?plan_id="+data(body)["plan_id"].(string), u, nil)
	if items, _ := list["data"].([]any); len(items) != 1 || strings.Contains(mustJSON(items[0]), "delete_created") {
		t.Fatalf("summary still offers delete_created: %v", list)
	}
	if status, b := s.do("POST", "/ai/executions/"+id.String()+"/recovery", u, map[string]any{"action": "delete_created"}); status != 409 || errCode(b) != "RECOVERY_NOT_AVAILABLE" {
		t.Fatalf("delete_created after the resource is gone: %d %v", status, b)
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestChatReadRequestsNeverPlan(t *testing.T) {
	s := newStack(t, stackOpts{noEngine: true})
	u := s.newUser()
	s.runningInstance(u, "read-01")
	conv := s.conversation(u)
	send := func(msg string) map[string]any {
		t.Helper()
		status, body := s.do("POST", "/ai/conversations/"+conv.String()+"/messages", u, map[string]any{"message": msg})
		if status != 200 {
			t.Fatalf("%q: %d %v", msg, status, body)
		}
		return data(body)
	}
	content := func(d map[string]any) string {
		m, _ := d["message"].(map[string]any)
		c, _ := m["content"].(string)
		return c
	}
	for _, msg := range []string{"list my servers", "show my VPS", "how many VMs do I have"} {
		d := send(msg)
		if pid, _ := d["plan_id"].(string); pid != "" || d["plan_proposal"] != nil || !strings.Contains(content(d), "read-01") {
			t.Fatalf("%q: %v", msg, d)
		}
	}
	d := send("what is the status of read-01")
	if c := content(d); !strings.Contains(c, "read-01") || !strings.Contains(c, "running") {
		t.Fatalf("status answer: %q", c)
	}
	if n := countPlans(t, u.id); n != 0 {
		t.Fatalf("read requests created %d plan(s)", n)
	}
	state, err := s.agentUC.GetConversationState(context.Background(), conv, u.id)
	if err != nil || state.CurrentIntent != "" {
		t.Fatalf("read requests changed the conversation intent: %v %v", state, err)
	}
	// A create request still plans (or asks its questions), as before.
	send("Create an Ubuntu VPS, nano size, public ip")
	state, _ = s.agentUC.GetConversationState(context.Background(), conv, u.id)
	if state.CurrentIntent != "create_vps" {
		t.Fatalf("create request: intent %q", state.CurrentIntent)
	}
	before := countPlans(t, u.id)
	send("show my VPS")
	if countPlans(t, u.id) != before {
		t.Fatal("a read request in a create conversation changed its plans")
	}
	state, _ = s.agentUC.GetConversationState(context.Background(), conv, u.id)
	if state.CurrentIntent != "create_vps" {
		t.Fatalf("a read request reset the create workflow: %q", state.CurrentIntent)
	}
}
