package integration

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
	"github.com/nevatal/cloud/internal/usecase"
)

func countInstances(t *testing.T, owner uuid.UUID, name string) int64 {
	t.Helper()
	var n int64
	testDB.Model(&models.Instance{}).Where("owner_id = ? AND name = ?", owner, name).Count(&n)
	return n
}

func countJobs(t *testing.T, owner uuid.UUID, key string) int64 {
	t.Helper()
	var n int64
	testDB.Model(&models.ProvisioningJob{}).Where("owner_id = ? AND idempotency_key = ?", owner, key).Count(&n)
	return n
}

func TestConcurrentCreateSameKeyCreatesOneInstance(t *testing.T) {
	s := newStack(t, stackOpts{noEngine: true})
	u := s.newUser()
	tpl := s.template()
	const key = "client-key-123"
	var wg sync.WaitGroup
	ids := make([]uuid.UUID, 8)
	errs := make([]error, 8)
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			job, err := s.instance.Create(context.Background(), u.id, usecase.CreateInstanceInput{
				Name: "same-key-vm", TemplateID: tpl, VCPUs: 1, MemoryMB: 1024, DiskGB: 20, IdempotencyKey: key,
			}, "127.0.0.1")
			errs[i] = err
			if job != nil {
				ids[i] = job.ID
			}
		}(i)
	}
	wg.Wait()
	for i := range ids {
		if errs[i] != nil {
			t.Fatalf("create %d: %v", i, errs[i])
		}
		if ids[i] != ids[0] {
			t.Fatalf("create %d returned job %s, want %s", i, ids[i], ids[0])
		}
	}
	if n := countInstances(t, u.id, "same-key-vm"); n != 1 {
		t.Fatalf("%d instances, want 1", n)
	}
	if n := countJobs(t, u.id, key); n != 1 {
		t.Fatalf("%d jobs, want 1", n)
	}
	var reservations int64
	testDB.Model(&models.ResourceReservation{}).Where("job_id = ?", ids[0]).Count(&reservations)
	if reservations != 1 {
		t.Fatalf("%d reservations, want 1", reservations)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.worker.Wait(ctx)
}

func TestReclaimedAttemptRedrivesSameJob(t *testing.T) {
	// The simulated hypervisor clone takes 2s, so the task is mid-job when
	// its replica "crashes".
	s := newStack(t, stackOpts{noEngine: true, mockLatency: 2 * time.Second})
	u := s.newUser()
	planID, rev := s.plan(u, vpsSpec("redrive-01", s.template(), 1024))
	res, err := s.orch.ApprovePlan(context.Background(), planID, u.id, false, usecase.ApproveInput{Version: &rev.Version, SpecHash: rev.SpecHash}, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	execID := res.Execution.ID
	key := orchestrator.IdempotencyKey(execID, "vps.create:redrive-01", 1)

	a := s.newEngine(orchestrator.EngineConfig{Owner: "replica-a", Lease: 30 * time.Second})
	a.Start()
	deadline := time.Now().Add(15 * time.Second)
	for countJobs(t, u.id, key) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the provisioning job was never created")
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Replica A dies with the task running (its lease is released the way
	// an expiry would free it; the job keeps running in the worker).
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	a.Shutdown(ctx)
	cancel()
	if task := s.tasks(execID)["vps.create:redrive-01"]; task.Status != models.TaskRunning {
		t.Fatalf("task is %s, want running (interrupted)", task.Status)
	}

	// Replica B re-claims the same attempt and re-drives the same job.
	s.startEngine(s.newEngine(orchestrator.EngineConfig{Owner: "replica-b", Lease: 30 * time.Second}))
	e := s.waitExecution(execID, 30*time.Second)
	if e.Status != models.ExecCompleted {
		t.Fatalf("status = %s", e.Status)
	}
	task := s.tasks(execID)["vps.create:redrive-01"]
	if task.Attempt != 1 || task.ClaimCount != 2 {
		t.Fatalf("attempt %d, claims %d", task.Attempt, task.ClaimCount)
	}
	if n := countJobs(t, u.id, key); n != 1 {
		t.Fatalf("%d jobs for %s, want 1", n, key)
	}
	if n := countInstances(t, u.id, "redrive-01"); n != 1 {
		t.Fatalf("%d instances, want 1", n)
	}
}

func TestStaleJobReaperFailsAndCleansUp(t *testing.T) {
	s := newStack(t, stackOpts{noEngine: true})
	u := s.newUser()
	ctx := context.Background()
	tpl := s.template()
	var node models.Node
	if err := testDB.First(&node).Error; err != nil {
		t.Fatal(err)
	}
	// A job whose worker died mid-provisioning: in progress, VM 4242 cloned,
	// reservation held, last heartbeat 10 minutes ago.
	job := models.ProvisioningJob{OwnerID: u.id, IdempotencyKey: "stale-" + uuid.NewString(), OperationType: models.OpCreate,
		Status: models.JobStatusInProgress, CurrentStep: "cloning_template", ProgressPercent: 30}
	if err := s.jobRepo.Create(ctx, &job); err != nil {
		t.Fatal(err)
	}
	inst := models.Instance{OwnerID: u.id, NodeID: node.ID, OSTemplateID: tpl, ProxmoxVMID: 4242, Name: "stale-vm", Hostname: "stale-vm",
		Status: models.StateProvisioning, VCPUs: 1, MemoryMB: 1024, DiskGB: 20}
	if err := s.instRepo.Create(ctx, &inst); err != nil {
		t.Fatal(err)
	}
	if err := s.jobRepo.SetInstance(ctx, &job, inst.ID); err != nil {
		t.Fatal(err)
	}
	usage, _ := s.quota.GetUsage(ctx, u.id)
	if _, err := s.quota.Reserve(ctx, usage, u.id, job.ID, 1, 1024, 20); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pve.CloneVM(ctx, node.ProxmoxNodeName, 9000, 4242, "stale-vm", "local-zfs"); err != nil {
		t.Fatal(err)
	}
	testDB.Exec(`UPDATE provisioning_jobs SET heartbeat_at = now() - interval '10 minutes', updated_at = now() - interval '10 minutes' WHERE id = ?`, job.ID)

	// A healthy job (fresh heartbeat) is never reaped.
	fresh := models.ProvisioningJob{OwnerID: u.id, IdempotencyKey: "fresh-" + uuid.NewString(), OperationType: models.OpStart,
		Status: models.JobStatusInProgress}
	_ = s.jobRepo.Create(ctx, &fresh)
	_, _ = s.jobRepo.Heartbeat(ctx, fresh.ID)

	s.worker.SetJobTimings(time.Second, 5*time.Minute)
	n, err := s.worker.ReapStaleJobs(ctx)
	if err != nil || n < 1 {
		t.Fatalf("reaped %d, %v", n, err)
	}
	got, _ := s.jobRepo.FindByID(ctx, job.ID, nil)
	if got.Status != models.JobStatusFailed || got.ErrorCode == nil || *got.ErrorCode != models.JobErrorStale {
		t.Fatalf("job %s / %v", got.Status, got.ErrorCode)
	}
	if f, _ := s.jobRepo.FindByID(ctx, fresh.ID, nil); f.Status != models.JobStatusInProgress {
		t.Fatalf("a job with a fresh heartbeat was reaped")
	}
	var reservation models.ResourceReservation
	testDB.Where("job_id = ?", job.ID).First(&reservation)
	if reservation.Status != models.ReservationReleased {
		t.Fatalf("reservation is %s, want released", reservation.Status)
	}
	instAfter, _ := s.instRepo.FindByID(ctx, inst.ID, nil)
	if instAfter.Status != models.StateFailed {
		t.Fatalf("instance is %s, want failed", instAfter.Status)
	}
	if _, err := s.pve.GetVMStatus(ctx, node.ProxmoxNodeName, 4242); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("the partially created VM was not deleted: %v", err)
	}
	// Safe to run again (and from every replica): nothing left to reap.
	if n, _ := s.worker.ReapStaleJobs(ctx); n != 0 {
		t.Fatalf("second pass reaped %d", n)
	}
	// A reaped job is never resurrected by a late worker write.
	if alive, _ := s.jobRepo.Heartbeat(ctx, job.ID); alive {
		t.Fatal("heartbeat accepted for a reaped job")
	}
}

func TestReapedJobRetriedWithNextAttempt(t *testing.T) {
	// A replica created the job of attempt 1 and died: the job stays in
	// progress with no heartbeat. The re-claimed task re-drives that job,
	// the reaper fails it as JOB_STALE (cleaning up), and the task retries
	// with attempt 2, which provisions normally.
	s := newStack(t, stackOpts{noEngine: true, engine: orchestrator.EngineConfig{RetryBase: 100 * time.Millisecond}})
	ctx := context.Background()
	u := s.newUser()
	tpl := s.template()
	planID, rev := s.plan(u, vpsSpec("stuck-01", tpl, 1024))
	res, err := s.orch.ApprovePlan(ctx, planID, u.id, false, usecase.ApproveInput{Version: &rev.Version, SpecHash: rev.SpecHash}, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	execID := res.Execution.ID
	key1 := orchestrator.IdempotencyKey(execID, "vps.create:stuck-01", 1)

	var node models.Node
	testDB.First(&node)
	job1 := models.ProvisioningJob{OwnerID: u.id, IdempotencyKey: key1, OperationType: models.OpCreate,
		Status: models.JobStatusInProgress, CurrentStep: "cloning_template", ProgressPercent: 30}
	if err := s.jobRepo.Create(ctx, &job1); err != nil {
		t.Fatal(err)
	}
	inst1 := models.Instance{OwnerID: u.id, NodeID: node.ID, OSTemplateID: tpl, ProxmoxVMID: 4343, Name: "stuck-01", Hostname: "stuck-01",
		Status: models.StateProvisioning, VCPUs: 1, MemoryMB: 1024, DiskGB: 20}
	_ = s.instRepo.Create(ctx, &inst1)
	_ = s.jobRepo.SetInstance(ctx, &job1, inst1.ID)
	usage, _ := s.quota.GetUsage(ctx, u.id)
	_, _ = s.quota.Reserve(ctx, usage, u.id, job1.ID, 1, 1024, 20)
	testDB.Exec(`UPDATE provisioning_jobs SET heartbeat_at = now() - interval '10 minutes', updated_at = now() - interval '10 minutes' WHERE id = ?`, job1.ID)

	s.startEngine(s.orch.Engine())
	// The task is waiting on the stuck job; the reaper (every replica runs
	// one) fails it.
	s.worker.SetJobTimings(time.Second, 5*time.Minute)
	deadline := time.Now().Add(15 * time.Second)
	for {
		if n, _ := s.worker.ReapStaleJobs(ctx); n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the stuck job was never reaped")
		}
		time.Sleep(100 * time.Millisecond)
	}
	e := s.waitExecution(execID, 30*time.Second)
	if e.Status != models.ExecCompleted {
		t.Fatalf("status = %s", e.Status)
	}
	task := s.tasks(execID)["vps.create:stuck-01"]
	if task.Attempt != 2 {
		t.Fatalf("attempt = %d, want 2", task.Attempt)
	}
	key2 := orchestrator.IdempotencyKey(execID, "vps.create:stuck-01", 2)
	if countJobs(t, u.id, key1) != 1 || countJobs(t, u.id, key2) != 1 {
		t.Fatal("want exactly one job per attempt key")
	}
	job1After, _ := s.jobRepo.FindByIdempotencyKey(ctx, u.id, key1)
	if job1After.Status != models.JobStatusFailed || *job1After.ErrorCode != models.JobErrorStale {
		t.Fatalf("job 1: %s %v", job1After.Status, job1After.ErrorCode)
	}
	// Attempt 1's instance was discarded (soft-deleted, not counted in the
	// quota); attempt 2's instance is the only live one.
	var live int64
	testDB.Model(&models.Instance{}).Where("owner_id = ? AND name = ? AND deleted_at IS NULL", u.id, "stuck-01").Count(&live)
	if live != 1 {
		t.Fatalf("%d live instances, want 1", live)
	}
	if got, err := s.instRepo.FindByID(ctx, inst1.ID, nil); err == nil {
		t.Fatalf("attempt 1 instance still live (%s)", got.Status)
	}
	var retried bool
	for _, ev := range s.events(execID) {
		if ev.Type == orchestrator.EventTaskRetrying && strings.Contains(ev.Message, models.JobErrorStale) {
			retried = true
		}
	}
	if !retried {
		t.Error("no task.retrying event naming JOB_STALE")
	}
}
