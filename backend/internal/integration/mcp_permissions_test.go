package integration

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/usecase"
)

var planURLPattern = regexp.MustCompile(`^/dashboard/ai/conversations/[0-9a-f-]{36}\?plan=([0-9a-f-]{36})$`)

func countPlans(t *testing.T, owner uuid.UUID) int64 {
	t.Helper()
	var n int64
	testDB.Model(&models.AIResourcePlan{}).Where("user_id = ?", owner).Count(&n)
	return n
}

func countMCPAudit(t *testing.T, owner uuid.UUID, tool, status string) int64 {
	t.Helper()
	var n int64
	testDB.Model(&models.MCPAuditLog{}).Where("user_id = ? AND tool_name = ? AND result_status = ?", owner, tool, status).Count(&n)
	return n
}

// connectServer connects u to an MCP server and returns the connection id
// and its tool ids by name.
func (s *stack) connectServer(u user, serverName string) (string, map[string]string) {
	s.t.Helper()
	var server models.MCPServer
	if err := testDB.Where("name = ?", serverName).First(&server).Error; err != nil {
		s.t.Fatalf("server %s not seeded: %v", serverName, err)
	}
	status, body := s.do("POST", "/integrations/mcp/connections", u, map[string]any{"server_id": server.ID.String(), "config_json": "{}"})
	if status != 201 && status != 200 {
		s.t.Fatalf("connect: %d %v", status, body)
	}
	connID := data(body)["id"].(string)
	var tools []models.MCPTool
	testDB.Where("server_id = ?", server.ID).Find(&tools)
	ids := map[string]string{}
	for _, t := range tools {
		ids[t.Name] = t.ID.String()
	}
	return connID, ids
}

func (s *stack) setPermission(u user, connID, toolID string, granted bool) {
	s.t.Helper()
	status, body := s.do("PATCH", "/integrations/mcp/connections/"+connID+"/permissions", u, map[string]any{"tool_id": toolID, "granted": granted})
	if status != 200 {
		s.t.Fatalf("set permission: %d %v", status, body)
	}
}

// rpcError calls a tool and returns the JSON-RPC error (nil when the call
// succeeded).
func rpcError(t *testing.T, s *stack, u user, name string, args map[string]any) map[string]any {
	t.Helper()
	body := rpc(t, s, u, "tools/call", map[string]any{"name": name, "arguments": args, "confirmed": true})
	e, _ := body["error"].(map[string]any)
	if e == nil {
		if _, ok := body["result"].(map[string]any); !ok {
			t.Fatalf("tools/call %s: neither result nor error: %v", name, body)
		}
	}
	return e
}

func deniedCode(e map[string]any) string {
	d, _ := e["data"].(map[string]any)
	c, _ := d["code"].(string)
	return c
}

func TestMCPToolPermissionsEnforced(t *testing.T) {
	s := newStack(t, stackOpts{noEngine: true})
	u := s.newUser()
	tpl := s.template()
	create := map[string]any{"name": "perm-01", "template_id": tpl.String(), "vcpus": 1, "memory_mb": 1024, "disk_gb": 20}

	// No connection to the first-party server: implicit, every tool granted.
	if e := rpcError(t, s, u, "vps.list", map[string]any{}); e != nil {
		t.Fatalf("vps.list without a connection: %v", e)
	}

	connID, tools := s.connectServer(u, usecase.FirstPartyMCPServer)
	for _, name := range []string{"vps.create", "vps.list", "vps.delete", "cloud.create_vm"} {
		if tools[name] == "" {
			t.Fatalf("tool %s is not an mcp_tools row of the first-party server", name)
		}
	}

	// A disabled write tool neither runs nor becomes a plan, under its
	// registry name or its legacy alias.
	s.setPermission(u, connID, tools["vps.create"], false)
	for _, name := range []string{"vps.create", "cloud.create_vm"} {
		e := rpcError(t, s, u, name, create)
		if e == nil || e["code"].(float64) != -32001 || deniedCode(e) != usecase.MCPDeniedToolDisabled {
			t.Fatalf("%s with vps.create disabled: %v", name, e)
		}
		if d := e["data"].(map[string]any); d["connection_id"] != connID || d["tool"] != name {
			t.Fatalf("%s denial data: %v", name, d)
		}
	}
	if n := countPlans(t, u.id); n != 0 {
		t.Fatalf("a disabled tool created %d plan(s)", n)
	}
	if countMCPAudit(t, u.id, "vps.create", "denied") != 1 || countMCPAudit(t, u.id, "cloud.create_vm", "denied") != 1 {
		t.Fatal("denials are not audited")
	}

	// Disabling the legacy alias refuses the registry tool too.
	s.setPermission(u, connID, tools["vps.create"], true)
	s.setPermission(u, connID, tools["cloud.create_vm"], false)
	if e := rpcError(t, s, u, "vps.create", create); deniedCode(e) != usecase.MCPDeniedToolDisabled {
		t.Fatalf("vps.create with cloud.create_vm disabled: %v", e)
	}
	s.setPermission(u, connID, tools["cloud.create_vm"], true)

	// Read tools are enforced too.
	s.setPermission(u, connID, tools["vps.list"], false)
	for _, name := range []string{"vps.list", "cloud.list_vms"} {
		if e := rpcError(t, s, u, name, map[string]any{}); deniedCode(e) != usecase.MCPDeniedToolDisabled {
			t.Fatalf("%s disabled: %v", name, e)
		}
	}
	s.setPermission(u, connID, tools["vps.list"], true)
	if e := rpcError(t, s, u, "vps.list", map[string]any{}); e != nil {
		t.Fatalf("vps.list re-enabled: %v", e)
	}

	// Granted again: the write tool creates a plan and links to it.
	res := rpc(t, s, u, "tools/call", map[string]any{"name": "vps.create", "arguments": create})["result"].(map[string]any)
	out := res["structuredContent"].(map[string]any)
	m := planURLPattern.FindStringSubmatch(out["url"].(string))
	if res["isError"] == true || out["status"] != "awaiting_approval" || m == nil || m[1] != out["plan_id"] {
		t.Fatalf("vps.create granted: %v", res)
	}
	var text map[string]any
	_ = json.Unmarshal([]byte(res["content"].([]any)[0].(map[string]any)["text"].(string)), &text)
	if text["url"] != out["url"] {
		t.Fatalf("text content lacks the url: %v", text)
	}
	planID, _ := uuid.Parse(out["plan_id"].(string))
	plan, err := s.orch.GetPlan(context.Background(), planID, u.id)
	if err != nil || out["url"] != usecase.PlanURL(plan.ConversationID, planID) {
		t.Fatalf("url %v does not point at the plan's conversation: %v %v", out["url"], plan, err)
	}

	// A permission for a tool of another server is refused.
	var gh models.MCPTool
	if err := testDB.Where("name = ?", "github.list_repos").First(&gh).Error; err != nil {
		t.Fatal(err)
	}
	if status, _ := s.do("PATCH", "/integrations/mcp/connections/"+connID+"/permissions", u, map[string]any{"tool_id": gh.ID.String(), "granted": false}); status != 404 {
		t.Fatalf("permission for another server's tool: %d", status)
	}

	// An inactive connection refuses every tool.
	testDB.Model(&models.MCPConnection{}).Where("id = ?", connID).Update("status", "disconnected")
	if e := rpcError(t, s, u, "vps.list", map[string]any{}); deniedCode(e) != usecase.MCPDeniedConnectionStopped {
		t.Fatalf("inactive connection: %v", e)
	}

	// External servers need a connection, and their permissions apply too.
	if e := rpcError(t, s, u, "github.list_repos", map[string]any{}); deniedCode(e) != usecase.MCPDeniedNotConnected {
		t.Fatalf("github tool without a connection: %v", e)
	}
	ghConn, ghTools := s.connectServer(u, "github")
	if e := rpcError(t, s, u, "github.list_repos", map[string]any{}); e != nil {
		t.Fatalf("github tool connected: %v", e)
	}
	s.setPermission(u, ghConn, ghTools["github.list_repos"], false)
	if e := rpcError(t, s, u, "github.list_repos", map[string]any{}); deniedCode(e) != usecase.MCPDeniedToolDisabled {
		t.Fatalf("disabled github tool: %v", e)
	}
	time.Sleep(100 * time.Millisecond)
	if n := countInstances(t, u.id, "perm-01"); n != 0 {
		t.Fatalf("MCP provisioned %d instance(s)", n)
	}
}

func TestAIAssistantExecuteCreatesPlanOnly(t *testing.T) {
	s := newStack(t, stackOpts{noEngine: true})
	u := s.newUser()
	tpl := s.template()
	params := map[string]any{
		"name": "assist-01", "hostname": "assist-01", "template_id": tpl.String(), "template_name": "shown only",
		"vcpus": 1, "memory_mb": 1024, "disk_gb": 20, "ssh_key_id": "<EXACT_SSH_KEY_UUID_OR_EMPTY>", "reason": "test",
	}
	payload, _ := json.Marshal(params)
	msg := models.AIAssistantMessage{UserID: u.id, Role: "assistant", Content: "proposal", ActionType: "create_instance", ActionPayload: string(payload), CreatedAt: time.Now()}
	if err := s.messages.Create(context.Background(), &msg); err != nil {
		t.Fatal(err)
	}

	status, body := s.do("POST", "/ai-assistant/execute", u, map[string]any{"message_id": msg.ID.String(), "action": "create_instance", "params": params})
	d := data(body)
	if status != 200 || d["status"] != "awaiting_approval" || d["risk_level"] != "medium" || d["version"].(float64) != 1 || d["name"] != "assist-01" {
		t.Fatalf("execute: %d %v", status, body)
	}
	for _, gone := range []string{"job_id", "instance_id"} {
		if _, ok := d[gone]; ok {
			t.Fatalf("response still has %s: %v", gone, d)
		}
	}
	m := planURLPattern.FindStringSubmatch(d["url"].(string))
	if m == nil || m[1] != d["plan_id"] {
		t.Fatalf("url: %v", d)
	}
	planID, _ := uuid.Parse(d["plan_id"].(string))
	plan, err := s.orch.GetPlan(context.Background(), planID, u.id)
	if err != nil || plan.Status != models.PlanStatusAwaitingApproval || plan.ConversationID.String() != d["conversation_id"] {
		t.Fatalf("plan: %v %v", plan, err)
	}
	if n := countExecutions(t, planID); n != 0 {
		t.Fatalf("assistant started %d execution(s)", n)
	}
	time.Sleep(200 * time.Millisecond)
	if n := countInstances(t, u.id, "assist-01"); n != 0 {
		t.Fatalf("assistant provisioned %d instance(s)", n)
	}
	var stored models.AIAssistantMessage
	if err := testDB.First(&stored, "id = ?", msg.ID).Error; err != nil || !stored.ActionExecuted {
		t.Fatalf("message not marked: %v %v", stored, err)
	}

	// The same card again opens the same plan.
	status, body = s.do("POST", "/ai-assistant/execute", u, map[string]any{"message_id": msg.ID.String(), "action": "create_instance", "params": params})
	if status != 200 || data(body)["plan_id"] != d["plan_id"] || data(body)["url"] != d["url"] {
		t.Fatalf("second execute: %d %v", status, body)
	}
	if n := countPlans(t, u.id); n != 1 {
		t.Fatalf("%d plans after two clicks", n)
	}

	// A proposal the policy engine rejects becomes a draft, not a resource.
	bad := map[string]any{"name": "assist-02", "template_id": uuid.NewString(), "vcpus": 1, "memory_mb": 1024, "disk_gb": 20}
	status, body = s.do("POST", "/ai-assistant/execute", u, map[string]any{"action": "create_instance", "params": bad})
	if status != 200 || data(body)["status"] != "draft" {
		t.Fatalf("invalid template: %d %v", status, body)
	}

	status, body = s.do("POST", "/ai-assistant/execute", u, map[string]any{"action": "delete_everything", "params": map[string]any{}})
	if status != 422 || errCode(body) != "EXECUTE_ACTION_FAILED" {
		t.Fatalf("unsupported action: %d %v", status, body)
	}
}
