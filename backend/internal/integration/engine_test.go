package integration

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
)

// sleepTool records each run and sleeps input.ms milliseconds.
func sleepTool(rec *recorder) func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	return func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
		start := time.Now()
		rec.calls.Add(1)
		select {
		case <-time.After(time.Duration(orchestrator.Int(tc.Input, "ms", 0)) * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		rec.add(tc.TaskKey, toolRun{start: start, end: time.Now(), attempt: tc.Attempt, key: tc.IdempotencyKey})
		return map[string]any{"slept": orchestrator.Int(tc.Input, "ms", 0)}, nil
	}
}

func TestEngineDAGOrderingAndParallelism(t *testing.T) {
	rec := newRecorder()
	s := newStack(t, stackOpts{tools: func(reg *orchestrator.Registry) {
		testTool(reg, "test.sleep", orchestrator.RiskRead, sleepTool(rec))
	}})
	u := s.newUser()
	id := s.execution(u, []taskDef{
		{key: "root", tool: "test.sleep", input: map[string]any{"ms": 100}},
		{key: "left", tool: "test.sleep", deps: []string{"root"}, input: map[string]any{"ms": 600}},
		{key: "right", tool: "test.sleep", deps: []string{"root"}, input: map[string]any{"ms": 600}},
		{key: "join", tool: "test.sleep", deps: []string{"left", "right"}, input: map[string]any{"ms": 50}},
	})
	e := s.waitExecution(id, 20*time.Second)
	if e.Status != models.ExecCompleted {
		t.Fatalf("status = %s", e.Status)
	}
	root, left, right, join := rec.get("root")[0], rec.get("left")[0], rec.get("right")[0], rec.get("join")[0]
	if left.start.Before(root.end) || right.start.Before(root.end) {
		t.Error("a dependent started before its dependency finished")
	}
	if !(left.start.Before(right.end) && right.start.Before(left.end)) {
		t.Errorf("independent tasks did not run in parallel: left %s-%s right %s-%s", left.start, left.end, right.start, right.end)
	}
	if join.start.Before(left.end) || join.start.Before(right.end) {
		t.Error("join started before both branches finished")
	}
	if rec.calls.Load() != 4 {
		t.Errorf("tool calls = %d, want 4", rec.calls.Load())
	}
	// Events: strictly increasing seq starting at 1, finished last.
	evs := s.events(id)
	for i, ev := range evs {
		if ev.Seq != int64(i+1) {
			t.Fatalf("event %d has seq %d", i, ev.Seq)
		}
	}
	if last := evs[len(evs)-1]; last.Type != orchestrator.EventExecutionFinished {
		t.Errorf("last event = %s", last.Type)
	}
}

func TestEngineRetryWithBackoff(t *testing.T) {
	var flakyCalls atomic.Int64
	starts := make(chan time.Time, 10)
	s := newStack(t, stackOpts{
		engine: orchestrator.EngineConfig{RetryBase: 300 * time.Millisecond},
		tools: func(reg *orchestrator.Registry) {
			testTool(reg, "test.flaky", orchestrator.RiskRead, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
				starts <- time.Now()
				if flakyCalls.Add(1) < 3 {
					return nil, orchestrator.Retryable("TRANSIENT", "try again")
				}
				return map[string]any{"attempt": tc.Attempt, "key": tc.IdempotencyKey}, nil
			})
			testTool(reg, "test.down", orchestrator.RiskRead, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
				return nil, orchestrator.Retryable("DOWN", "always down")
			})
		},
	})
	u := s.newUser()
	id := s.execution(u, []taskDef{{key: "flaky", tool: "test.flaky"}})
	e := s.waitExecution(id, 20*time.Second)
	if e.Status != models.ExecCompleted {
		t.Fatalf("status = %s", e.Status)
	}
	task := s.tasks(id)["flaky"]
	if task.Attempt != 3 || flakyCalls.Load() != 3 {
		t.Fatalf("attempt = %d, calls = %d", task.Attempt, flakyCalls.Load())
	}
	if task.IdempotencyKey != orchestrator.IdempotencyKey(id, "flaky", 3) {
		t.Errorf("idempotency key = %s", task.IdempotencyKey)
	}
	t1, t2, t3 := <-starts, <-starts, <-starts
	if d := t2.Sub(t1); d < 240*time.Millisecond {
		t.Errorf("first backoff %s < 0.8 x 300ms", d)
	}
	if d := t3.Sub(t2); d < 480*time.Millisecond {
		t.Errorf("second backoff %s < 0.8 x 600ms", d)
	}
	retrying := 0
	for _, ev := range s.events(id) {
		if ev.Type == orchestrator.EventTaskRetrying {
			retrying++
		}
	}
	if retrying != 2 {
		t.Errorf("task.retrying events = %d, want 2", retrying)
	}

	// Bounded: a task that keeps failing stops after max_attempts.
	id = s.execution(u, []taskDef{{key: "down", tool: "test.down"}})
	e = s.waitExecution(id, 20*time.Second)
	down := s.tasks(id)["down"]
	if e.Status != models.ExecFailed || down.Status != models.TaskFailed || down.Attempt != down.MaxAttempts {
		t.Fatalf("status %s, task %s attempt %d/%d", e.Status, down.Status, down.Attempt, down.MaxAttempts)
	}
}

func TestLeaseExpiryReclaimedExactlyOnce(t *testing.T) {
	var calls atomic.Int64
	s := newStack(t, stackOpts{noEngine: true, tools: func(reg *orchestrator.Registry) {
		testTool(reg, "test.once", orchestrator.RiskRead, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
			calls.Add(1)
			return map[string]any{"resumed": tc.Resumed, "attempt": tc.Attempt}, nil
		})
	}})
	u := s.newUser()
	id := s.execution(u, []taskDef{{key: "once", tool: "test.once"}})
	// A replica claimed the task and died: running, lease expired.
	dead := uuid.New()
	if err := testDB.Exec(`UPDATE ai_execution_tasks SET status = 'running', lease_owner = 'dead-replica', lease_token = ?,
		lease_expires_at = now() - interval '1 minute', claim_count = 1, started_at = now() - interval '2 minutes'
		WHERE execution_id = ?`, dead, id).Error; err != nil {
		t.Fatal(err)
	}
	testDB.Exec(`UPDATE ai_executions SET status = 'running', started_at = now() WHERE id = ?`, id)

	// Two replicas compete for it.
	a := s.newEngine(orchestrator.EngineConfig{Owner: "replica-a"})
	b := s.newEngine(orchestrator.EngineConfig{Owner: "replica-b"})
	s.startEngine(a)
	s.startEngine(b)
	e := s.waitExecution(id, 15*time.Second)
	if e.Status != models.ExecCompleted {
		t.Fatalf("status = %s", e.Status)
	}
	task := s.tasks(id)["once"]
	if calls.Load() != 1 {
		t.Fatalf("tool ran %d times, want exactly once", calls.Load())
	}
	if task.Attempt != 1 || task.ClaimCount != 2 || task.IdempotencyKey != orchestrator.IdempotencyKey(id, "once", 1) {
		t.Fatalf("attempt %d, claims %d, key %s: a re-claim must keep the attempt and its key", task.Attempt, task.ClaimCount, task.IdempotencyKey)
	}
	var out map[string]any
	_ = json.Unmarshal([]byte(*task.Output), &out)
	if out["resumed"] != true {
		t.Errorf("the tool was not told it resumed an interrupted attempt: %v", out)
	}
}

func TestLeaseNeverExpiresUnderLiveRunner(t *testing.T) {
	var calls atomic.Int64
	s := newStack(t, stackOpts{
		engine: orchestrator.EngineConfig{Owner: "replica-a", Lease: time.Second, Heartbeat: 250 * time.Millisecond},
		tools: func(reg *orchestrator.Registry) {
			testTool(reg, "test.long", orchestrator.RiskRead, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
				calls.Add(1)
				select {
				case <-time.After(3500 * time.Millisecond): // 3.5 leases
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return map[string]any{}, nil
			})
		},
	})
	other := s.newEngine(orchestrator.EngineConfig{Owner: "replica-b", Lease: time.Second, Heartbeat: 250 * time.Millisecond})
	s.startEngine(other)
	u := s.newUser()
	id := s.execution(u, []taskDef{{key: "long", tool: "test.long"}})
	e := s.waitExecution(id, 15*time.Second)
	if e.Status != models.ExecCompleted || calls.Load() != 1 {
		t.Fatalf("status %s, calls %d: the heartbeat must keep the lease while the tool runs", e.Status, calls.Load())
	}
	if c := s.tasks(id)["long"].ClaimCount; c != 1 {
		t.Fatalf("claim count = %d, want 1", c)
	}
}

func TestShutdownReleasesLeases(t *testing.T) {
	release := make(chan struct{})
	var calls atomic.Int64
	var started atomic.Bool
	s := newStack(t, stackOpts{
		engine: orchestrator.EngineConfig{Owner: "replica-a", Lease: 30 * time.Second},
		tools: func(reg *orchestrator.Registry) {
			testTool(reg, "test.block", orchestrator.RiskRead, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
				calls.Add(1)
				started.Store(true)
				select {
				case <-release:
					return map[string]any{"attempt": tc.Attempt, "key": tc.IdempotencyKey}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			})
		},
	})
	u := s.newUser()
	id := s.execution(u, []taskDef{{key: "block", tool: "test.block"}})
	deadline := time.Now().Add(10 * time.Second)
	for !started.Load() {
		if time.Now().After(deadline) {
			t.Fatal("task never started")
		}
		time.Sleep(20 * time.Millisecond)
	}
	// SIGTERM: the shutdown budget runs out while the task is still running.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	s.orch.Engine().Shutdown(ctx)

	task := s.tasks(id)["block"]
	if task.Status != models.TaskRunning || task.LeaseExpiresAt == nil || task.LeaseExpiresAt.After(time.Now()) {
		t.Fatalf("after shutdown: status %s, lease until %v (want running with a released lease)", task.Status, task.LeaseExpiresAt)
	}
	if task.Error != nil {
		t.Fatalf("an interrupted task must not record an error: %s", *task.Error)
	}
	// Another replica re-claims it immediately, with the same attempt.
	close(release)
	s.startEngine(s.newEngine(orchestrator.EngineConfig{Owner: "replica-b", Lease: 30 * time.Second}))
	e := s.waitExecution(id, 10*time.Second)
	task = s.tasks(id)["block"]
	if e.Status != models.ExecCompleted || task.Attempt != 1 || calls.Load() != 2 {
		t.Fatalf("status %s, attempt %d, calls %d", e.Status, task.Attempt, calls.Load())
	}
}

func TestCancelExecution(t *testing.T) {
	var cancelled atomic.Bool
	s := newStack(t, stackOpts{tools: func(reg *orchestrator.Registry) {
		testTool(reg, "test.wait", orchestrator.RiskRead, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
			<-ctx.Done()
			cancelled.Store(true)
			return nil, ctx.Err()
		})
		testTool(reg, "test.never", orchestrator.RiskRead, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
			t.Error("a task after a cancel ran")
			return nil, nil
		})
	}})
	u := s.newUser()
	id := s.execution(u, []taskDef{
		{key: "wait", tool: "test.wait"},
		{key: "after", tool: "test.never", deps: []string{"wait"}},
	})
	deadline := time.Now().Add(10 * time.Second)
	for s.tasks(id)["wait"].Status != models.TaskRunning {
		if time.Now().After(deadline) {
			t.Fatal("task never started")
		}
		time.Sleep(20 * time.Millisecond)
	}
	status, body := s.do("POST", "/ai/executions/"+id.String()+"/cancel", u, nil)
	if status != 202 || data(body)["status"] != models.ExecCancelling {
		t.Fatalf("cancel: %d %v", status, body)
	}
	e := s.waitExecution(id, 10*time.Second)
	tasks := s.tasks(id)
	if e.Status != models.ExecCancelled || tasks["wait"].Status != models.TaskCancelled || tasks["after"].Status != models.TaskCancelled || !cancelled.Load() {
		t.Fatalf("exec %s, wait %s, after %s, tool cancelled %v", e.Status, tasks["wait"].Status, tasks["after"].Status, cancelled.Load())
	}
	evs := s.events(id)
	if evs[len(evs)-1].Type != orchestrator.EventExecutionFinished || evs[len(evs)-2].Type != orchestrator.EventExecutionCancelled {
		t.Errorf("closing events: %s, %s", evs[len(evs)-2].Type, evs[len(evs)-1].Type)
	}
	// Cancelling a finished execution is refused.
	if status, body := s.do("POST", "/ai/executions/"+id.String()+"/cancel", u, nil); status != 409 {
		t.Errorf("second cancel: %d %v", status, body)
	}
}

func TestPartialFailureSkipsDescendantsAndRetries(t *testing.T) {
	var fixed atomic.Bool
	s := newStack(t, stackOpts{tools: func(reg *orchestrator.Registry) {
		testTool(reg, "test.write", orchestrator.RiskMedium, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		})
		testTool(reg, "test.broken", orchestrator.RiskMedium, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
			if !fixed.Load() {
				return nil, orchestrator.Permanent("BROKEN", "permanently broken")
			}
			return map[string]any{"ok": true}, nil
		})
	}})
	u := s.newUser()
	id := s.execution(u, []taskDef{
		{key: "created", tool: "test.write", risk: orchestrator.RiskMedium},
		{key: "broken", tool: "test.broken", risk: orchestrator.RiskMedium},
		{key: "child", tool: "test.write", risk: orchestrator.RiskMedium, deps: []string{"broken"}},
		{key: "grandchild", tool: "test.write", risk: orchestrator.RiskMedium, deps: []string{"child"}},
	})
	e := s.waitExecution(id, 15*time.Second)
	tasks := s.tasks(id)
	if e.Status != models.ExecPartiallyCompleted {
		t.Fatalf("status = %s", e.Status)
	}
	if tasks["created"].Status != models.TaskSucceeded || tasks["broken"].Status != models.TaskFailed ||
		tasks["child"].Status != models.TaskSkipped || tasks["grandchild"].Status != models.TaskSkipped {
		t.Fatalf("statuses: %s %s %s %s", tasks["created"].Status, tasks["broken"].Status, tasks["child"].Status, tasks["grandchild"].Status)
	}
	if tasks["broken"].Attempt != 1 {
		t.Errorf("a permanent failure must not be retried (attempt %d)", tasks["broken"].Attempt)
	}
	status, body := s.do("GET", "/ai/executions/"+id.String(), u, nil)
	if status != 200 {
		t.Fatalf("get: %d", status)
	}
	var actions []string
	for _, o := range data(body)["recovery_options"].([]any) {
		actions = append(actions, o.(map[string]any)["action"].(string))
	}
	if len(actions) < 3 || actions[0] != "retry_failed" || actions[1] != "diagnose" || actions[2] != "keep" {
		t.Fatalf("recovery options = %v", actions)
	}
	if data(body)["report"] == nil {
		t.Fatal("no report on a finished execution")
	}
	// Retry a task that is not failed: refused.
	if status, body := s.do("POST", "/ai/executions/"+id.String()+"/tasks/"+tasks["child"].ID.String()+"/retry", u, nil); status != 409 || errCode(body) != "TASK_NOT_RETRYABLE" {
		t.Fatalf("retry skipped task: %d %v", status, body)
	}
	// Retry the failed one once it is fixed: the skipped descendants run too.
	fixed.Store(true)
	if status, body := s.do("POST", "/ai/executions/"+id.String()+"/tasks/"+tasks["broken"].ID.String()+"/retry", u, nil); status != 202 {
		t.Fatalf("retry: %d %v", status, body)
	}
	e = s.waitExecution(id, 15*time.Second)
	tasks = s.tasks(id)
	if e.Status != models.ExecCompleted || tasks["grandchild"].Status != models.TaskSucceeded || tasks["broken"].Attempt != 2 {
		t.Fatalf("after retry: %s, grandchild %s, broken attempt %d", e.Status, tasks["grandchild"].Status, tasks["broken"].Attempt)
	}
}

func TestEventsFeedAndSSE(t *testing.T) {
	s := newStack(t, stackOpts{tools: func(reg *orchestrator.Registry) {
		testTool(reg, "test.quick", orchestrator.RiskRead, func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
			time.Sleep(200 * time.Millisecond)
			return map[string]any{}, nil
		})
	}})
	u := s.newUser()
	id := s.execution(u, []taskDef{{key: "a", tool: "test.quick"}, {key: "b", tool: "test.quick", deps: []string{"a"}}})
	frames, err := readSSE(s.server.URL+"/api/v1/ai/executions/"+id.String()+"/events", u.token, 0, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) == 0 || frames[len(frames)-1].event != orchestrator.EventExecutionFinished {
		t.Fatalf("stream did not end with execution.finished: %v", frames)
	}
	for i, f := range frames {
		if f.id != int64(i+1) {
			t.Fatalf("frame %d has id %d", i, f.id)
		}
	}
	// Resume after the 3rd event with Last-Event-ID and with ?after=.
	again, err := readSSE(s.server.URL+"/api/v1/ai/executions/"+id.String()+"/events", u.token, 3, 10*time.Second)
	if err != nil || len(again) != len(frames)-3 || again[0].id != 4 {
		t.Fatalf("resumed stream: %v %v", again, err)
	}
	status, body := s.do("GET", "/ai/executions/"+id.String()+"/events?after=2&limit=2", u, nil)
	page := data(body)
	if status != 200 || len(page["events"].([]any)) != 2 || page["next_after"].(float64) != 4 {
		t.Fatalf("JSON page: %d %v", status, body)
	}
	// Another user cannot read the events.
	other := s.newUser()
	if status, body := s.do("GET", "/ai/executions/"+id.String()+"/events", other, nil); status != 404 || errCode(body) != "EXECUTION_NOT_FOUND" {
		t.Fatalf("foreign events: %d %v", status, body)
	}
	if errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
