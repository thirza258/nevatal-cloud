# AI Infrastructure Orchestrator — Phase 1 API Contract (frozen)

Status: **frozen for Phase 1** (2026-11-02; clarified the same day: approved-not-executing plans are editable; MCP `initialize`/`ping`). Source PRD: "Nevatal Cloud AI Infrastructure
Orchestrator v1.0". Backend and frontend build against this file; change it only together.

Principle: **the LLM/planner plans → the policy engine validates → the approval manager
authorizes → the execution engine executes → the verification engine confirms.**
Nothing the model, a chat message or an MCP client says can authorize execution.

Conventions: base `/api/v1`, Bearer auth (cookie auth is GET-only), success `{"data": ...}`,
error `{"error": {"code", "message", "details"?}}`. Times are RFC 3339 UTC. IDs are UUIDs.

---

## 1. Lifecycles

**Plan status:** `draft → validating → awaiting_approval → approved → executing → verifying →
completed`; terminal alternatives `rejected | cancelled | failed | partially_completed`.

**Execution status:** `queued → running → verifying → completed`; alternatives `failed |
partially_completed | cancelling → cancelled`.

**Task status:** `queued | blocked | running | retrying | succeeded | failed | cancelled |
skipped | compensating`. `blocked` = waiting on dependencies; `skipped` = a dependency failed.

**Risk levels** (task risk = tool risk; plan risk = max of its tasks):

| Risk | Examples | Approval needed |
|---|---|---|
| `read` | list/get/status, quota check | none |
| `low` | plan generation, inspect config | none |
| `medium` | create/start/restart VPS, create bucket/app/game server | approve with `version` + `spec_hash` |
| `high` | stop/resize VPS, network/exposure changes | medium + `acknowledge_risks: true` |
| `destructive` | delete VPS/bucket/backup, snapshot restore | high + `confirmation` = exact target name(s) |

Admin-only tools also require role `admin` (403 `ADMIN_REQUIRED`) even when approved.
Approval expiry: 60 min (medium/high), 15 min (destructive).

---

## 2. Plan (extends the existing plan object)

`GET /ai/plans/:id` returns the existing plan fields **unchanged** plus:

```json
{
  "current_version": 3,
  "revision": {
    "version": 3,
    "spec_hash": "sha256:9f2c…",
    "created_at": "2026-11-02T10:00:00Z",
    "created_by": "planner",
    "summary": "Create Ubuntu 24.04 VPS web-01 (2 vCPU, 4 GB, 40 GB)",
    "specification": {
      "intent": "create_vps",
      "resources": [
        { "key": "web-01", "type": "vps", "action": "create",
          "config": { "name": "web-01", "template_id": "…", "plan_id": "…", "vcpus": 2,
                      "memory_mb": 4096, "disk_gb": 40, "ssh_key_ids": ["…"] } }
      ]
    },
    "tasks": [
      { "key": "quota.check", "tool": "quota.check", "title": "Validate quota",
        "depends_on": [], "risk": "read" },
      { "key": "vps.create:web-01", "tool": "vps.create", "title": "Provision VPS web-01",
        "depends_on": ["quota.check"], "risk": "medium", "input": { "…": "…" } },
      { "key": "vps.wait_ready:web-01", "tool": "vps.wait_ready", "title": "Wait until running",
        "depends_on": ["vps.create:web-01"], "risk": "read" }
    ],
    "estimated_resources": { "vcpus": 2, "memory_mb": 4096, "disk_gb": 40, "instances": 1 },
    "risk_level": "medium",
    "requires_approval": true,
    "confirmation_required": null,
    "validation": {
      "status": "passed",
      "checked_at": "2026-11-02T10:00:01Z",
      "errors": [],
      "warnings": [ { "code": "NO_SSH_KEY", "message": "…", "task_key": "vps.create:web-01" } ]
    }
  },
  "approval": null,
  "latest_execution_id": null
}
```

- `created_by`: `planner` | `user`. `validation.status`: `passed | warnings | failed`.
- `confirmation_required` (destructive only): `{"type": "typed_name", "expected": ["web-01"]}` —
  the user must type these names (case-sensitive, comma-separated if several).
- `approval`: `null` or `{"id","version","spec_hash","status":"approved|superseded|expired|rejected|consumed","risk_level","expires_at","created_at"}`.
- `spec_hash` = `"sha256:" + hex(sha256(canonical JSON of {specification, tasks}))`.

---

## 3. Endpoints

| Method | Path | Body | Success |
|---|---|---|---|
| POST | `/ai/conversations/:id/messages` | existing | existing shape; plan cards carry `plan_id` + `version` |
| GET | `/ai/plans/:id` | — | Plan (§2) |
| GET | `/ai/plans/:id/revisions` | — | `[{version, spec_hash, created_at, created_by, summary, risk_level}]` |
| GET | `/ai/plans/:id/revisions/:version` | — | revision object (§2) |
| PATCH | `/ai/plans/:id` | `{base_version, specification}` | Plan with a new revision; earlier approvals → `superseded`; status → `awaiting_approval` (or `draft` if validation fails). Allowed in `draft`, `awaiting_approval`, `approved` (not yet executing) |
| POST | `/ai/plans/:id/validate` | — | `{validation}` (policy re-run on current revision) |
| POST | `/ai/plans/:id/approve` | see below | `{approval, execution, plan_id, status, progress_steps, result_details, message}` |
| POST | `/ai/plans/:id/reject` | `{reason?}` | Plan (`rejected`) |
| POST | `/ai/plans/:id/cancel` | existing | existing (also cancels an active execution) |
| POST | `/ai/plans/:id/execute` | `{approval_id}` | 202 Execution |
| GET | `/ai/executions` | `?conversation_id=&plan_id=&status=` | `[ExecutionSummary]` |
| GET | `/ai/executions/:id` | — | Execution (§4) |
| GET | `/ai/executions/:id/events` | `?after=<seq>&limit=200` | `{events, next_after}` or SSE (§5) |
| POST | `/ai/executions/:id/cancel` | — | 202 Execution (`cancelling`) |
| POST | `/ai/executions/:id/tasks/:taskId/retry` | — | 202 Execution |
| POST | `/ai/executions/:id/recovery` | `{action}` | see below |
| GET | `/ai/tools` | — | `[Tool]` (§6) |

**Approve body:** `{ "version": 3, "spec_hash": "sha256:…", "acknowledge_risks": false,
"confirmation": "web-01", "execute": true }`.
- `version` + `spec_hash` must match the current revision (409 `PLAN_VERSION_MISMATCH`).
  Legacy clients that send only `{plan_id}` or `{version}` are accepted **only** when plan risk ≤
  `medium`; then the current revision is used.
- `execute` defaults to `true` (the approval is consumed by a new execution, returned in
  `execution`). With `false`, call `/execute` later with `approval_id`.
- Legacy response fields stay: `plan_id`, `status` (`provisioning` when an execution started),
  `progress_steps` (initial steps), `result_details` (`{}`), `message`.

**Recovery `action`:** `retry_failed` (re-queue failed tasks and their skipped descendants,
same as per-task retry), `keep` (acknowledge partial result; execution stays
`partially_completed`), `delete_created` (creates a **new destructive plan** to remove the
resources this execution created and returns `{"plan_id"}`; never deletes directly).

---

## 4. Execution object

```json
{
  "id": "…", "plan_id": "…", "plan_version": 3, "conversation_id": "…",
  "status": "running",
  "cancel_requested": false,
  "created_at": "…", "started_at": "…", "finished_at": null,
  "tasks": [
    { "id": "…", "key": "vps.create:web-01", "tool": "vps.create", "title": "Provision VPS web-01",
      "risk": "medium", "depends_on": ["quota.check"], "status": "running",
      "attempt": 1, "max_attempts": 3, "next_attempt_at": null,
      "started_at": "…", "finished_at": null,
      "output": { "instance_id": "…", "job_id": "…" },
      "error": null,
      "verification": null }
  ],
  "report": null,
  "recovery_options": []
}
```
- `error`: `{"code", "message"}` (sanitized; no secrets, no internal IPs of other tenants).
- `verification`: `{"status": "verified|unverified|failed", "checks": [{"name", "status": "pass|fail|skip", "detail"}]}`.
  Mock hypervisor / unreachable checks report `unverified`, never `verified`.
- `report` (terminal): `{"summary", "resources": [{"type", "id", "name", "status", "access": {"ssh": "ssh ubuntu@10.0.0.5"}}], "warnings": [], "next_actions": []}`.
- `recovery_options` (when `failed` / `partially_completed`): `[{"action": "retry_failed"|"keep"|"delete_created", "label", "requires_approval": bool}]`.
- ExecutionSummary = Execution without `tasks`, plus `{"task_counts": {"succeeded": 2, "failed": 0, …}}`.

---

## 5. Events

Event: `{"seq": 42, "id": "…", "type": "task.succeeded", "severity": "info|warning|error",
"message": "…", "task_key": "vps.create:web-01", "metadata": {}, "created_at": "…"}`.
`seq` is strictly increasing per execution.

Types: `execution.created`, `execution.started`, `approval.recorded`, `task.queued`,
`task.started`, `task.progress`, `task.retrying`, `task.succeeded`, `task.failed`,
`task.skipped`, `task.cancelled`, `verification.started`, `verification.result`,
`execution.completed`, `execution.failed`, `execution.partially_completed`,
`execution.cancelled`, `execution.finished` (always last; stream terminator).

**SSE:** `GET /ai/executions/:id/events` with `Accept: text/event-stream` (Bearer header — use
fetch streaming, not `EventSource`). Frames: `id: <seq>`, `event: <type>`, `data: <event JSON>`.
Supports `Last-Event-ID` or `?after=`. Heartbeat comment `: ping` every 20 s. The stream closes
after `execution.finished` and when the server shuts down. Fallback: poll the JSON form with
`?after=<next_after>` every 2 s.

---

## 6. Tools (registry; also served to MCP `tools/list`)

`{"id", "title", "description", "risk", "resource_types", "idempotent", "timeout_seconds",
"requires_admin", "input_schema"}`.

Phase 1 tool ids: read — `vps.list`, `vps.get`, `vps.status`, `quota.check`,
`catalog.templates`, `catalog.plans`, `vps.wait_ready`, `vps.verify`; medium — `vps.create`,
`vps.start`, `vps.restart`, `vps.snapshot.create`, `bucket.create`, `app.create`, `n8n.create`,
`openclaw.create`, `game_server.create`, `android.create`; high — `vps.stop`, `vps.resize`;
destructive — `vps.delete`, `vps.snapshot.restore`, `bucket.delete`.

MCP lifecycle: `POST /api/v1/mcp/rpc` (JSON-RPC 2.0 over HTTP, Bearer auth, `application/json` responses) must also answer `initialize` (protocol version negotiation, `capabilities: {tools: {}}`, `serverInfo`), accept the `notifications/initialized` notification (no response body), and answer `ping` — desktop clients (Claude Desktop, Cursor) send `initialize` first.

MCP `tools/call`: read tools run immediately; write/high/destructive tools **never execute** —
they create a plan and return `{"plan_id", "version", "status": "awaiting_approval",
"risk_level"}` for the user to approve in the UI.

---

## 7. Error codes

| HTTP | Code | When |
|---|---|---|
| 400 | `VALIDATION_ERROR` | malformed body/params (`details.field` when known) |
| 403 | `ADMIN_REQUIRED` | admin-only tool in plan |
| 404 | `PLAN_NOT_FOUND` / `EXECUTION_NOT_FOUND` / `TASK_NOT_FOUND` | missing or not owned |
| 409 | `PLAN_VERSION_CONFLICT` | PATCH `base_version` is not current |
| 409 | `PLAN_VERSION_MISMATCH` | approve `version`/`spec_hash` not current |
| 409 | `PLAN_NOT_EDITABLE` | PATCH while `executing`/`verifying` or in a terminal state (an `approved` plan that has not started executing CAN be edited: its approval becomes `superseded` and the plan returns to `awaiting_approval`) |
| 409 | `APPROVAL_REQUIRED` / `APPROVAL_EXPIRED` / `APPROVAL_SUPERSEDED` / `APPROVAL_CONSUMED` | execute without a valid approval |
| 409 | `EXECUTION_ACTIVE` | plan already has a running execution |
| 409 | `TASK_NOT_RETRYABLE` | retry on a task that is not `failed` |
| 422 | `VALIDATION_FAILED` | policy errors (`details.errors`) at approve/execute time |
| 422 | `RISK_ACK_REQUIRED` | high/destructive without `acknowledge_risks` |
| 422 | `CONFIRMATION_REQUIRED` / `CONFIRMATION_MISMATCH` | destructive confirmation missing/wrong |
| 429 | `TOO_MANY_EXECUTIONS` | more than 3 active executions per user |

---

## 8. Backend guarantees (not visible in JSON, required)

- One execution path: the old synchronous provisioning loop in `ApprovePlan` is removed.
- Tasks are leased in Postgres (`FOR UPDATE SKIP LOCKED`, lease + heartbeat); expired leases are
  re-claimed after a crash/restart; SIGTERM stops claiming and releases unfinished leases.
- Idempotency key per task attempt: `orch:<execution_id>:<task_key>:<attempt>`. Re-claiming the
  same attempt re-drives the same job/resource; a new attempt is only made after the previous
  attempt reached a terminal, cleaned-up state. `provisioning_jobs(owner_id, idempotency_key)`
  becomes a unique index (non-empty keys).
- Stale provisioning jobs (no heartbeat) are failed by a reaper that runs the existing cleanup.
- Policy + quota are re-checked at execute time; the transactional reservation in instance
  creation remains the final authority.
- Approvals are bound to `plan_version` + `spec_hash`, consumed once, expire, and are superseded
  by any PATCH. Chat text can never approve.
- Secrets never appear in plans, events, outputs or reports.
