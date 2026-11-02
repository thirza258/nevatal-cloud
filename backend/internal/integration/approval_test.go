package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
	"github.com/nevatal/cloud/internal/usecase"
)

func countExecutions(t *testing.T, planID uuid.UUID) int64 {
	t.Helper()
	var n int64
	testDB.Model(&models.AIExecution{}).Where("plan_id = ?", planID).Count(&n)
	return n
}

func TestApprovalBinding(t *testing.T) {
	s := newStack(t, stackOpts{noEngine: true})
	u := s.newUser()
	tpl := s.template()
	planID, rev := s.plan(u, vpsSpec("bind-01", tpl, 1024))
	path := "/ai/plans/" + planID.String()

	// Approve without executing: the plan is approved, still editable.
	status, body := s.do("POST", path+"/approve", u, map[string]any{"version": rev.Version, "spec_hash": rev.SpecHash, "execute": false})
	if status != 200 || data(body)["execution"] != nil {
		t.Fatalf("approve: %d %v", status, body)
	}
	approval1 := data(body)["approval"].(map[string]any)
	if approval1["status"] != "approved" {
		t.Fatalf("approval status %v", approval1["status"])
	}
	if _, plan := s.do("GET", path, u, nil); data(plan)["status"] != models.PlanStatusApproved {
		t.Fatalf("plan status %v", data(plan)["status"])
	}

	// PATCH (allowed while approved and not executing) supersedes it.
	edited := vpsSpec("bind-01", tpl, 2048)
	status, body = s.do("PATCH", path, u, map[string]any{"base_version": 1, "specification": edited})
	if status != 200 || data(body)["current_version"].(float64) != 2 || data(body)["status"] != models.PlanStatusAwaitingApproval {
		t.Fatalf("patch: %d %v", status, body)
	}
	rev2 := data(body)["revision"].(map[string]any)
	if data(body)["approval"].(map[string]any)["status"] != "superseded" {
		t.Fatalf("approval after patch: %v", data(body)["approval"])
	}
	if rev2["spec_hash"] == rev.SpecHash || rev2["created_by"] != "user" {
		t.Fatalf("revision 2: %v", rev2)
	}
	// Stale base version.
	if status, body = s.do("PATCH", path, u, map[string]any{"base_version": 1, "specification": edited}); status != 409 || errCode(body) != "PLAN_VERSION_CONFLICT" {
		t.Fatalf("stale patch: %d %v", status, body)
	}
	// Any PATCH supersedes, even one that changes nothing.
	status, body = s.do("POST", path+"/approve", u, map[string]any{"version": 2, "spec_hash": rev2["spec_hash"], "execute": false})
	if status != 200 {
		t.Fatalf("approve v2: %d %v", status, body)
	}
	status, body = s.do("PATCH", path, u, map[string]any{"base_version": 2, "specification": edited})
	if status != 200 || data(body)["current_version"].(float64) != 3 || data(body)["approval"].(map[string]any)["status"] != "superseded" {
		t.Fatalf("identical patch: %d %v", status, body)
	}
	rev2 = data(body)["revision"].(map[string]any)
	// Undeclared config fields are rejected before anything is stored.
	bad := vpsSpec("bind-01", tpl, 2048)
	bad.Resources[0].Config["root_password"] = "hunter2-sentinel"
	if status, body = s.do("PATCH", path, u, map[string]any{"base_version": 3, "specification": bad}); status != 400 || errCode(body) != "VALIDATION_ERROR" {
		t.Fatalf("patch with an undeclared field: %d %v", status, body)
	}
	// The superseded approval cannot execute.
	if status, body = s.do("POST", path+"/execute", u, map[string]any{"approval_id": approval1["id"]}); status != 409 || errCode(body) != "APPROVAL_SUPERSEDED" {
		t.Fatalf("execute superseded: %d %v", status, body)
	}
	// Wrong hash (current version with the hash of version 1).
	if status, body = s.do("POST", path+"/approve", u, map[string]any{"version": 3, "spec_hash": rev.SpecHash}); status != 409 || errCode(body) != "PLAN_VERSION_MISMATCH" {
		t.Fatalf("wrong hash: %d %v", status, body)
	}
	// Old version number.
	if status, body = s.do("POST", path+"/approve", u, map[string]any{"version": 1, "spec_hash": rev2["spec_hash"]}); status != 409 || errCode(body) != "PLAN_VERSION_MISMATCH" {
		t.Fatalf("old version: %d %v", status, body)
	}
	// Expired approval.
	status, body = s.do("POST", path+"/approve", u, map[string]any{"version": 3, "spec_hash": rev2["spec_hash"], "execute": false})
	if status != 200 {
		t.Fatalf("approve v2: %d %v", status, body)
	}
	approval2 := data(body)["approval"].(map[string]any)
	expires, _ := time.Parse(time.RFC3339Nano, approval2["expires_at"].(string))
	if d := time.Until(expires); d < 59*time.Minute || d > 61*time.Minute {
		t.Errorf("medium-risk approval expires in %s, want 60m", d)
	}
	testDB.Exec("UPDATE ai_approvals SET expires_at = now() - interval '1 minute' WHERE id = ?", approval2["id"])
	if status, body = s.do("POST", path+"/execute", u, map[string]any{"approval_id": approval2["id"]}); status != 409 || errCode(body) != "APPROVAL_EXPIRED" {
		t.Fatalf("execute expired: %d %v", status, body)
	}
	// A valid approval executes once; reusing it is refused.
	status, body = s.do("POST", path+"/approve", u, map[string]any{"version": 3, "spec_hash": rev2["spec_hash"], "execute": false})
	approval3 := data(body)["approval"].(map[string]any)
	status, body = s.do("POST", path+"/execute", u, map[string]any{"approval_id": approval3["id"]})
	if status != 202 || data(body)["status"] != models.ExecQueued || data(body)["plan_version"].(float64) != 3 {
		t.Fatalf("execute: %d %v", status, body)
	}
	if status, body = s.do("POST", path+"/execute", u, map[string]any{"approval_id": approval3["id"]}); status != 409 || errCode(body) != "APPROVAL_CONSUMED" {
		t.Fatalf("execute twice: %d %v", status, body)
	}
	if status, body = s.do("PATCH", path, u, map[string]any{"base_version": 3, "specification": edited}); status != 409 || errCode(body) != "PLAN_NOT_EDITABLE" {
		t.Fatalf("patch while executing: %d %v", status, body)
	}
	if status, body = s.do("POST", path+"/approve", u, map[string]any{"version": 3, "spec_hash": rev2["spec_hash"]}); status != 409 || errCode(body) != "EXECUTION_ACTIVE" {
		t.Fatalf("approve while executing: %d %v", status, body)
	}
	if n := countExecutions(t, planID); n != 1 {
		t.Fatalf("%d executions, want 1", n)
	}
	// Revisions are listed and individually readable.
	status, body = s.do("GET", path+"/revisions", u, nil)
	if revs, _ := body["data"].([]any); status != 200 || len(revs) != 3 {
		t.Fatalf("revisions: %d %v", status, body)
	}
	if status, body = s.do("GET", path+"/revisions/1", u, nil); status != 200 || data(body)["spec_hash"] != rev.SpecHash {
		t.Fatalf("revision 1: %d %v", status, body)
	}
	// Another tenant sees nothing.
	other := s.newUser()
	if status, body = s.do("GET", path, other, nil); status != 404 || errCode(body) != "PLAN_NOT_FOUND" {
		t.Fatalf("foreign plan: %d %v", status, body)
	}
}

// runningInstance provisions a VPS outside the orchestrator and waits for it.
func (s *stack) runningInstance(u user, name string) *models.Instance {
	s.t.Helper()
	job, err := s.instance.Create(context.Background(), u.id, usecase.CreateInstanceInput{
		Name: name, TemplateID: s.template(), VCPUs: 1, MemoryMB: 1024, DiskGB: 20,
	}, "127.0.0.1")
	if err != nil {
		s.t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		inst, err := s.instRepo.FindByID(context.Background(), *job.InstanceID, nil)
		if err == nil && inst.Status == models.StateRunning {
			return inst
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("instance %s never ran", name)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestRiskGates(t *testing.T) {
	s := newStack(t, stackOpts{})
	u := s.newUser()
	inst := s.runningInstance(u, "gate-01")
	ref := map[string]any{"instance_id": inst.ID.String(), "name": "spoofed"}

	// High risk: acknowledge_risks required.
	stopID, stopRev := s.plan(u, orchestrator.Specification{Intent: "stop", Resources: []orchestrator.ResourceSpec{{
		Key: "gate-01", Type: orchestrator.TypeVPS, Action: orchestrator.ActionStop, Config: ref}}})
	if stopRev.RiskLevel != orchestrator.RiskHigh {
		t.Fatalf("stop risk = %s", stopRev.RiskLevel)
	}
	stopPath := "/ai/plans/" + stopID.String() + "/approve"
	if status, body := s.do("POST", stopPath, u, map[string]any{"version": 1, "spec_hash": stopRev.SpecHash}); status != 422 || errCode(body) != "RISK_ACK_REQUIRED" {
		t.Fatalf("high without ack: %d %v", status, body)
	}
	if status, body := s.do("POST", stopPath, u, nil); status != 400 || errCode(body) != "VALIDATION_ERROR" {
		t.Fatalf("legacy approve of a high-risk plan: %d %v", status, body)
	}

	// Destructive: typed confirmation of the database name (not the name
	// written in the specification).
	delID, delRev := s.plan(u, orchestrator.Specification{Intent: "delete", Resources: []orchestrator.ResourceSpec{{
		Key: "gate-01", Type: orchestrator.TypeVPS, Action: orchestrator.ActionDelete, Config: ref}}})
	if delRev.RiskLevel != orchestrator.RiskDestructive || delRev.ConfirmationRequired == nil || delRev.ConfirmationRequired.Expected[0] != "gate-01" {
		t.Fatalf("delete revision: %+v %+v", delRev.RiskLevel, delRev.ConfirmationRequired)
	}
	delPath := "/ai/plans/" + delID.String() + "/approve"
	base := map[string]any{"version": 1, "spec_hash": delRev.SpecHash, "acknowledge_risks": true}
	if status, body := s.do("POST", delPath, u, base); status != 422 || errCode(body) != "CONFIRMATION_REQUIRED" {
		t.Fatalf("missing confirmation: %d %v", status, body)
	}
	for _, typed := range []string{"spoofed", "GATE-01", "gate-01,other"} {
		base["confirmation"] = typed
		if status, body := s.do("POST", delPath, u, base); status != 422 || errCode(body) != "CONFIRMATION_MISMATCH" {
			t.Fatalf("confirmation %q: %d %v", typed, status, body)
		}
	}
	if n := countExecutions(t, delID); n != 0 {
		t.Fatalf("a refused approval started %d execution(s)", n)
	}
	base["confirmation"] = "gate-01"
	status, body := s.do("POST", delPath, u, base)
	if status != 200 || data(body)["status"] != "provisioning" {
		t.Fatalf("destructive approve: %d %v", status, body)
	}
	expires, _ := time.Parse(time.RFC3339Nano, data(body)["approval"].(map[string]any)["expires_at"].(string))
	if d := time.Until(expires); d > 16*time.Minute {
		t.Errorf("destructive approval lasts %s, want 15m", d)
	}
	execID, _ := uuid.Parse(data(body)["execution"].(map[string]any)["id"].(string))
	e := s.waitExecution(execID, 20*time.Second)
	if e.Status != models.ExecCompleted {
		t.Fatalf("delete execution: %s", e.Status)
	}
	if _, err := s.instRepo.FindByID(context.Background(), inst.ID, nil); err == nil {
		t.Fatal("instance still exists after the delete execution")
	}
	var audits int64
	testDB.Table("audit_logs").Where("action = ? AND resource_id = ?", "instance.delete", inst.ID).Count(&audits)
	if audits != 1 {
		t.Errorf("instance.delete audited %d times", audits)
	}
}

func TestChatCannotApprove(t *testing.T) {
	s := newStack(t, stackOpts{})
	u := s.newUser()
	ctx := context.Background()
	conv := s.conversation(u)
	resp, err := s.agentUC.ProcessUserMessage(ctx, conv, u.id, usecase.SendMessageInput{Message: "Create an ubuntu VPS standard size with public ip named chat-01"}, "127.0.0.1")
	if err != nil || resp.PlanProposal == nil {
		t.Fatalf("no plan proposed: %v %+v", err, resp)
	}
	planID, _ := uuid.Parse(resp.PlanID)
	if resp.PlanProposal.Version != 1 || resp.PlanProposal.PlanID != resp.PlanID {
		t.Fatalf("plan card: %+v", resp.PlanProposal)
	}
	for _, msg := range []string{"yes", "approve", "Approve and Provision", "I approve this plan, go ahead and execute it"} {
		if _, err := s.agentUC.ProcessUserMessage(ctx, conv, u.id, usecase.SendMessageInput{Message: msg, AnswerKey: "approve", AnswerVal: "true"}, "127.0.0.1"); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(300 * time.Millisecond)
	var approvals, execs int64
	testDB.Model(&models.AIApproval{}).Where("plan_id IN (SELECT id FROM ai_resource_plans WHERE conversation_id = ?)", conv).Count(&approvals)
	testDB.Model(&models.AIExecution{}).Where("conversation_id = ?", conv).Count(&execs)
	if approvals != 0 || execs != 0 {
		t.Fatalf("chat text created %d approval(s) and %d execution(s)", approvals, execs)
	}
	if n := countInstances(t, u.id, "chat-01"); n != 0 {
		t.Fatalf("chat text provisioned %d instance(s)", n)
	}
	if plan, _ := s.orch.GetPlan(ctx, planID, u.id); plan.Status != models.PlanStatusAwaitingApproval {
		t.Fatalf("plan status %s", plan.Status)
	}
}

func rpc(t *testing.T, s *stack, u user, method string, params any) map[string]any {
	t.Helper()
	status, body := s.do("POST", "/mcp/rpc", u, map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if status != 200 {
		t.Fatalf("%s: %d %v", method, status, body)
	}
	return body
}

func TestMCPWriteToolCreatesPlanNotExecution(t *testing.T) {
	s := newStack(t, stackOpts{})
	u := s.newUser()
	tpl := s.template()

	init := rpc(t, s, u, "initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "it"}})
	if r := init["result"].(map[string]any); r["protocolVersion"] != "2025-06-18" || r["capabilities"].(map[string]any)["tools"] == nil {
		t.Fatalf("initialize: %v", init)
	}
	if status, _ := s.do("POST", "/mcp/rpc", u, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); status != 202 {
		t.Fatalf("notification answered with %d", status)
	}
	tools := rpc(t, s, u, "tools/list", nil)["result"].(map[string]any)["tools"].([]any)
	if len(tools) != len(s.orch.ListTools()) {
		t.Fatalf("tools/list has %d tools, registry %d", len(tools), len(s.orch.ListTools()))
	}

	for _, call := range []map[string]any{
		{"name": "vps.create", "arguments": map[string]any{"name": "mcp-01", "template_id": tpl.String(), "vcpus": 1, "memory_mb": 1024, "disk_gb": 20}, "confirmed": true},
		{"name": "cloud.create_vm", "arguments": map[string]any{"name": "mcp-02", "template_id": tpl.String()}, "confirmed": true},
	} {
		res := rpc(t, s, u, "tools/call", call)["result"].(map[string]any)
		out := res["structuredContent"].(map[string]any)
		if res["isError"] == true || out["status"] != "awaiting_approval" || out["risk_level"] != "medium" || out["version"].(float64) != 1 {
			t.Fatalf("tools/call %v: %v", call["name"], res)
		}
		planID, _ := uuid.Parse(out["plan_id"].(string))
		if n := countExecutions(t, planID); n != 0 {
			t.Fatalf("MCP call started %d execution(s)", n)
		}
		plan, err := s.orch.GetPlan(context.Background(), planID, u.id)
		if err != nil || plan.Status != models.PlanStatusAwaitingApproval {
			t.Fatalf("plan: %v %v", plan, err)
		}
	}
	time.Sleep(300 * time.Millisecond)
	if n := countInstances(t, u.id, "mcp-01") + countInstances(t, u.id, "mcp-02"); n != 0 {
		t.Fatalf("MCP provisioned %d instance(s)", n)
	}
	// Destructive and unmapped legacy write tools never run either.
	inst := s.runningInstance(u, "mcp-keep")
	res := rpc(t, s, u, "tools/call", map[string]any{"name": "cloud.delete_vm", "arguments": map[string]any{"id": inst.ID.String(), "confirm_name": "mcp-keep"}, "confirmed": true})["result"].(map[string]any)
	if res["structuredContent"].(map[string]any)["risk_level"] != "destructive" {
		t.Fatalf("delete over MCP: %v", res)
	}
	res = rpc(t, s, u, "tools/call", map[string]any{"name": "cloud.restart_openclaw_instance", "arguments": map[string]any{"id": uuid.NewString()}, "confirmed": true})["result"].(map[string]any)
	var denied map[string]any
	_ = json.Unmarshal([]byte(res["content"].([]any)[0].(map[string]any)["text"].(string)), &denied)
	if res["isError"] != true || denied["status"] != "denied" {
		t.Fatalf("legacy write tool: %v", res)
	}
	time.Sleep(200 * time.Millisecond)
	if got, err := s.instRepo.FindByID(context.Background(), inst.ID, nil); err != nil || got.Status != models.StateRunning {
		t.Fatalf("the instance was touched over MCP: %v %v", got, err)
	}
	// Read tools run immediately.
	res = rpc(t, s, u, "tools/call", map[string]any{"name": "vps.list", "arguments": map[string]any{}})["result"].(map[string]any)
	if res["isError"] == true {
		t.Fatalf("read tool: %v", res)
	}
}

func TestVerificationUnverifiedInMockMode(t *testing.T) {
	s := newStack(t, stackOpts{})
	u := s.newUser()
	planID, rev := s.plan(u, vpsSpec("verify-01", s.template(), 1024))
	status, body := s.do("POST", "/ai/plans/"+planID.String()+"/approve", u, map[string]any{"version": rev.Version, "spec_hash": rev.SpecHash})
	if status != 200 {
		t.Fatalf("approve: %d %v", status, body)
	}
	execID, _ := uuid.Parse(data(body)["execution"].(map[string]any)["id"].(string))
	e := s.waitExecution(execID, 20*time.Second)
	if e.Status != models.ExecCompleted {
		t.Fatalf("status = %s", e.Status)
	}
	task := s.tasks(execID)["vps.create:verify-01"]
	var v orchestrator.Verification
	if task.Verification == nil || json.Unmarshal([]byte(*task.Verification), &v) != nil {
		t.Fatal("no verification stored")
	}
	if v.Status != orchestrator.Unverified {
		t.Fatalf("verification = %s, want unverified with a simulated hypervisor", v.Status)
	}
	skipped := false
	for _, c := range v.Checks {
		if c.Name == "hypervisor_power_state" && c.Status == "skip" {
			skipped = true
		}
		if c.Name == "instance_record" && c.Status != "pass" {
			t.Errorf("instance_record = %s", c.Status)
		}
	}
	if !skipped {
		t.Fatalf("hypervisor check not skipped: %+v", v.Checks)
	}
	var report orchestrator.Report
	_ = json.Unmarshal([]byte(*e.Report), &report)
	if len(report.Resources) != 1 || report.Resources[0].Access["ssh"] == "" || len(report.Warnings) == 0 {
		t.Fatalf("report: %+v", report)
	}
	// The conversation got the result.
	msgs, _ := s.agent.ListMessages(context.Background(), e.ConversationID)
	if last := msgs[len(msgs)-1]; last.Role != "assistant" || last.UIComponents == "" {
		t.Fatalf("last message: %+v", last)
	}
}

func TestTooManyActiveExecutions(t *testing.T) {
	s := newStack(t, stackOpts{noEngine: true})
	u := s.newUser()
	tpl := s.template()
	for i := 0; i < usecase.MaxActiveExecutions+1; i++ {
		planID, rev := s.plan(u, vpsSpec("limit-"+string(rune('a'+i)), tpl, 512))
		status, body := s.do("POST", "/ai/plans/"+planID.String()+"/approve", u, map[string]any{"version": rev.Version, "spec_hash": rev.SpecHash})
		if i < usecase.MaxActiveExecutions {
			if status != 200 {
				t.Fatalf("approve %d: %d %v", i, status, body)
			}
			continue
		}
		if status != 429 || errCode(body) != "TOO_MANY_EXECUTIONS" {
			t.Fatalf("approve %d: %d %v", i, status, body)
		}
	}
}

func TestBackfillLegacyPlan(t *testing.T) {
	s := newStack(t, stackOpts{noEngine: true})
	u := s.newUser()
	ctx := context.Background()
	conv := s.conversation(u)
	legacy := models.AIResourcePlan{ConversationID: conv, UserID: u.id, Version: 1, Intent: "create_storage", Status: models.PlanStatusAwaitingApproval,
		Title: "legacy", PlanJSON: "{}", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_ = s.agent.CreatePlan(ctx, &legacy)
	_ = s.agent.CreatePlanResource(ctx, &models.AIPlanResource{PlanID: legacy.ID, ResourceType: "storage_bucket", ResourceName: "legacy-bucket",
		ConfigJSON: `{"name":"legacy-bucket","is_public":false}`, Status: "pending", ExecutionOrder: 1})
	if n, err := s.orch.BackfillRevisions(ctx); err != nil || n < 1 {
		t.Fatalf("backfill: %d %v", n, err)
	}
	plan, err := s.orch.GetPlan(ctx, legacy.ID, u.id)
	if err != nil || plan.Revision == nil || plan.Revision.Version != 1 || plan.Revision.Tasks[0].Tool != orchestrator.ToolBucketCreate {
		t.Fatalf("backfilled plan: %+v %v", plan, err)
	}
	if hash, _ := orchestrator.SpecHash(plan.Revision.Specification, plan.Revision.Tasks); hash != plan.Revision.SpecHash {
		t.Fatal("backfilled hash does not match the stored revision")
	}
	if n, _ := s.orch.BackfillRevisions(ctx); n != 0 {
		t.Fatalf("second backfill wrote %d revision(s)", n)
	}
}

func TestChatFollowUpRevisesOpenPlan(t *testing.T) {
	s := newStack(t, stackOpts{noEngine: true})
	u := s.newUser()
	ctx := context.Background()
	conv := s.conversation(u)
	first, err := s.agentUC.ProcessUserMessage(ctx, conv, u.id, usecase.SendMessageInput{Message: "Create an ubuntu VPS nano size with public ip named rev-01"}, "127.0.0.1")
	if err != nil || first.PlanProposal == nil {
		t.Fatalf("first message: %v %+v", err, first)
	}
	second, err := s.agentUC.ProcessUserMessage(ctx, conv, u.id, usecase.SendMessageInput{Message: "actually make it the standard size"}, "127.0.0.1")
	if err != nil || second.PlanProposal == nil {
		t.Fatalf("second message: %v %+v", err, second)
	}
	if second.PlanID != first.PlanID || second.PlanProposal.Version != 2 {
		t.Fatalf("follow-up created plan %s v%d, want a revision of %s", second.PlanID, second.PlanProposal.Version, first.PlanID)
	}
	planID, _ := uuid.Parse(first.PlanID)
	plan, _ := s.orch.GetPlan(ctx, planID, u.id)
	if plan.Revision.Version != 2 || plan.Revision.EstimatedResources.MemoryMB != 4096 || plan.Revision.CreatedBy != "planner" {
		t.Fatalf("revision: %+v", plan.Revision)
	}
	// An unchanged follow-up does not add a revision.
	third, _ := s.agentUC.ProcessUserMessage(ctx, conv, u.id, usecase.SendMessageInput{Message: "standard is fine"}, "127.0.0.1")
	if third.PlanID != first.PlanID || third.PlanProposal.Version != 2 {
		t.Fatalf("unchanged follow-up: plan %s v%d", third.PlanID, third.PlanProposal.Version)
	}
}

func TestSecretsNeverStored(t *testing.T) {
	s := newStack(t, stackOpts{noEngine: true})
	u := s.newUser()
	const sentinel = "sk-sentinel-4f1e9c"
	tpl := s.template()
	for _, call := range []map[string]any{
		{"name": "openclaw.create", "arguments": map[string]any{"name": "claw-secret", "api_key": sentinel}},
		{"name": "vps.create", "arguments": map[string]any{"name": "vm-secret", "template_id": tpl.String(), "vcpus": 1, "memory_mb": 1024, "disk_gb": 20, "root_password": sentinel}},
		{"name": "cloud.create_openclaw_instance", "arguments": map[string]any{"name": "claw-legacy", "api_key": sentinel}},
	} {
		rpc(t, s, u, "tools/call", call)
	}
	planID, _ := s.plan(u, vpsSpec("vm-ok", tpl, 1024))
	bad := vpsSpec("vm-ok", tpl, 1024)
	bad.Resources[0].Config["ssh_password"] = sentinel
	if status, _ := s.do("PATCH", "/ai/plans/"+planID.String(), u, map[string]any{"base_version": 1, "specification": bad}); status != 400 {
		t.Fatalf("PATCH with a secret field: %d", status)
	}
	for _, q := range []string{
		"SELECT count(*) FROM ai_resource_plans WHERE plan_json LIKE ?",
		"SELECT count(*) FROM ai_plan_resources WHERE config_json LIKE ?",
		"SELECT count(*) FROM ai_plan_revisions WHERE specification LIKE ? OR tasks LIKE ?",
		"SELECT count(*) FROM mcp_audit_logs WHERE input_summary LIKE ?",
		"SELECT count(*) FROM ai_tool_invocations WHERE input_json LIKE ? OR output_json LIKE ?",
	} {
		var n int64
		args := []any{"%" + sentinel + "%"}
		if len(q) > 0 && countPlaceholders(q) == 2 {
			args = append(args, "%"+sentinel+"%")
		}
		if err := testDB.Raw(q, args...).Scan(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("secret stored: %s -> %d row(s)", q, n)
		}
	}
	// The legacy name without the key still becomes a plan (api_key dropped).
	var plans int64
	testDB.Raw("SELECT count(*) FROM ai_plan_resources WHERE resource_name = 'claw-legacy'").Scan(&plans)
	if plans != 1 {
		t.Fatalf("legacy openclaw request: %d plan resources", plans)
	}
}

func countPlaceholders(q string) int {
	n := 0
	for _, c := range q {
		if c == '?' {
			n++
		}
	}
	return n
}

func TestChatResourceTypesExecuteThroughEngine(t *testing.T) {
	s := newStack(t, stackOpts{})
	ctx := context.Background()
	var image models.AndroidImage
	if err := testDB.Where("is_published = true").First(&image).Error; err != nil {
		t.Fatalf("no published Android image: %v", err)
	}
	bucket := fmt.Sprintf("it-assets-%d", time.Now().UnixNano()%1000000)
	cases := []struct {
		name, message, tool, key string
		answers                  map[string]any
	}{
		{"app", "Deploy my website from github named site-01", orchestrator.ToolAppCreate, "site-01",
			map[string]any{"git_repo": "https://github.com/nevatal/go-api-service", "framework": "go"}},
		{"n8n", "Set up an n8n automation instance named flows-01", orchestrator.ToolN8nCreate, "flows-01", nil},
		{"openclaw", "Deploy an openclaw agent named claw-01", orchestrator.ToolOpenClawCreate, "claw-01", nil},
		{"bucket", "Create an s3 bucket named " + bucket, orchestrator.ToolBucketCreate, bucket, nil},
		{"game_server", "Create a minecraft server named mc-01", orchestrator.ToolGameServerCreate, "mc-01",
			map[string]any{"server_software": "paper", "difficulty": "normal", "gamemode": "survival"}},
		{"android", "Create an android instance named droid-01", orchestrator.ToolAndroidCreate, "droid-01",
			map[string]any{"image_id": image.ID.String(), "plan": "standard", "display_resolution": "720x1280"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u := s.newUser()
			conv := s.conversation(u)
			resp, err := s.agentUC.ProcessUserMessage(ctx, conv, u.id, usecase.SendMessageInput{Message: c.message, Answers: c.answers}, "127.0.0.1")
			if err != nil || resp.PlanProposal == nil {
				t.Fatalf("no plan: %v %+v", err, resp)
			}
			planID, _ := uuid.Parse(resp.PlanID)
			plan, err := s.orch.GetPlan(ctx, planID, u.id)
			if err != nil || plan.Status != models.PlanStatusAwaitingApproval {
				t.Fatalf("plan %s: %v %+v", plan.Status, err, plan.Revision.Validation)
			}
			status, body := s.do("POST", "/ai/plans/"+resp.PlanID+"/approve", u, map[string]any{"version": plan.Revision.Version, "spec_hash": plan.Revision.SpecHash})
			if status != 200 {
				t.Fatalf("approve: %d %v", status, body)
			}
			execID, _ := uuid.Parse(data(body)["execution"].(map[string]any)["id"].(string))
			e := s.waitExecution(execID, 30*time.Second)
			if e.Status != models.ExecCompleted {
				t.Fatalf("execution %s", e.Status)
			}
			task := s.tasks(execID)[c.tool+":"+c.key]
			if task.Status != models.TaskSucceeded || task.Verification == nil {
				t.Fatalf("task %s verification %v", task.Status, task.Verification)
			}
			var v orchestrator.Verification
			_ = json.Unmarshal([]byte(*task.Verification), &v)
			if v.Status != orchestrator.Unverified {
				t.Fatalf("verification %s in mock mode", v.Status)
			}
			out := strings.ToLower(ptr(task.Output) + ptr(e.Report))
			for _, secret := range []string{"access_url", "session", "token", "password", "rcon", "gw_sec_", "encryption"} {
				if strings.Contains(out, secret) {
					t.Fatalf("%q in output/report: %s", secret, out)
				}
			}
			var report orchestrator.Report
			_ = json.Unmarshal([]byte(*e.Report), &report)
			found := false
			for _, r := range report.Resources {
				if r.Name == c.key && r.Status != "deleted" {
					found = true
				}
			}
			if !found {
				t.Fatalf("report does not list %s: %+v", c.key, report.Resources)
			}
			var row models.AIPlanResource
			testDB.Where("plan_id = ? AND resource_name = ?", planID, c.key).First(&row)
			if row.Status != "completed" || row.ProvisionedID == "" {
				t.Fatalf("legacy resource row: %s %q", row.Status, row.ProvisionedID)
			}
		})
	}
}
