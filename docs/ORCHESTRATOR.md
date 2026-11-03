# AI Infrastructure Orchestrator (Phases 1–4)

The AI Infrastructure Orchestrator is the engine behind the **AI Cloud Agent**.
You describe the infrastructure you want in plain language. The platform turns
it into a versioned plan, checks it, waits for **your** approval of exactly
that plan, executes it as durable tasks, verifies the result on the hypervisor
and reports what it built. Around that core, the Operations Center adds
reusable templates, managed databases, diagnostics, scheduled checks,
notifications, deterministic automation policies, cost advice, topology and
logical agent roles.

This guide describes Phases 1–4 as implemented. The Phase 1 API contract is
frozen in [ORCHESTRATOR_API.md](ORCHESTRATOR_API.md) (2026-11-02); the Phase
2–4 endpoints are documented in
[ORCHESTRATOR_PHASES_API.md](ORCHESTRATOR_PHASES_API.md). Timings, retries,
checks and limits were confirmed against the code on 2026-11-03.

| You are | Read |
|---|---|
| A user (dashboard or MCP client) | sections 1, 2 and 3 |
| The operator of a Nevatal installation | sections 3 and 4; the installation itself is in [PROXMOX_INSTALL.md](PROXMOX_INSTALL.md) |
| Writing your own client | section 5 and the two API documents |

## Contents

1. [What it is](#1-what-it-is)
2. [For users](#2-for-users)
   - [2.1 Asking for infrastructure](#21-asking-for-infrastructure)
   - [2.2 Reading a plan](#22-reading-a-plan)
   - [2.3 Risk levels and approvals](#23-risk-levels-and-approvals)
   - [2.4 Editing a plan](#24-editing-a-plan)
   - [2.5 Following an execution](#25-following-an-execution)
   - [2.6 Verification results](#26-verification-results)
   - [2.7 Partial failures and recovery](#27-partial-failures-and-recovery)
   - [2.8 Leaving and coming back](#28-leaving-and-coming-back)
   - [2.9 MCP clients (Claude Desktop, Cursor)](#29-mcp-clients-claude-desktop-cursor)
   - [2.10 Operations Center](#210-operations-center)
   - [2.11 Infrastructure templates](#211-infrastructure-templates)
   - [2.12 Managed databases](#212-managed-databases)
   - [2.13 Diagnostics and remediation](#213-diagnostics-and-remediation)
   - [2.14 Checks, notifications and automation policies](#214-checks-notifications-and-automation-policies)
   - [2.15 Advisor, costs, topology and agent roles](#215-advisor-costs-topology-and-agent-roles)
3. [Security model](#3-security-model)
4. [For operators](#4-for-operators)
   - [4.1 Where it runs](#41-where-it-runs)
   - [4.2 Durable tasks: leases and heartbeats](#42-durable-tasks-leases-and-heartbeats)
   - [4.3 Graceful shutdown (SIGTERM)](#43-graceful-shutdown-sigterm)
   - [4.4 No duplicates: idempotency keys](#44-no-duplicates-idempotency-keys)
   - [4.5 Stale-job reaper](#45-stale-job-reaper)
   - [4.6 Proxmox token privileges](#46-proxmox-token-privileges)
   - [4.7 Live events through proxies (SSE)](#47-live-events-through-proxies-sse)
   - [4.8 Monitoring](#48-monitoring)
   - [4.9 Troubleshooting](#49-troubleshooting)
   - [4.10 Phase 2–4 scheduling and settings](#410-phase-24-scheduling-and-settings)
5. [API quick reference](#5-api-quick-reference)
6. [Roadmap](#6-roadmap)

---

## 1. What it is

An AI-native control plane: the model helps you *describe* a change, and
deterministic components decide whether, when and how it happens. Every step
is stored in PostgreSQL, so nothing depends on your browser tab or on one API
replica staying alive.

```
"Create an Ubuntu 24.04 VPS web-01, 2 vCPU, 4 GB RAM, 40 GB disk"
   |
   v
planner               proposes plan revision v1: resources, tasks, estimates
   |
   v
policy engine         validates v1: quota, ownership, catalog, risk level
   |
   v
approval manager      records YOUR approval of exactly v1 (version + spec_hash)
   |
   v
execution engine      runs the task graph, retries, survives restarts
   |
   v
verification engine   checks the real state: VM exists, is running, has an IP
   |
   v
report                resources, access (ssh ubuntu@10.0.0.5), warnings, next steps
```

**The planner plans, the policy engine validates, the approval manager
authorizes, the execution engine executes, the verification engine confirms.
The model is never the authority.**

| Component | Decides | Can never |
|---|---|---|
| Planner (the AI agent) | What to propose: resources, tasks, their order | Approve, execute, or skip validation |
| Policy engine (deterministic code) | Whether a revision is allowed, and its risk level | Be overruled by the model or by chat text |
| Approval manager | Whether *you* approved *this* revision | Accept an approval from chat text, pasted JSON, tool output or an MCP client |
| Execution engine | When and where each task runs; retries | Run a revision nobody approved, or a tool outside the registry |
| Verification engine | Whether the result matches the plan | Report `verified` without checking the real hypervisor |

Where you meet it:

- **Dashboard:** *AI Cloud Agent* in the sidebar (`/dashboard/ai`): chat,
  plan and approval panel, live execution timeline. *Operations* in the
  sidebar (`/dashboard/operations`): databases, templates, checks, policies,
  advisor, topology, agent roles and diagnostics (section 2.10).
- **Notifications:** the bell in the top bar shows execution, check and
  policy findings (section 2.14).
- **REST API:** `/api/v1/ai/*`, `/api/v1/databases/*` and
  `/api/v1/notifications/*` (section 5).
- **MCP:** `POST /api/v1/mcp/rpc` for Claude Desktop, Cursor and other MCP
  clients (section 2.9).

Not in these phases (section 6): automatic cleanup/compensation after a
partial failure, real managed-database runtime provisioning and live
connection probes, and the platform-wide backup scheduler (a separate P2
feature). The managed database service is logical: it owns lifecycle,
credentials, backups and health records, while the execution engine verifies
the records and marks the runtime probe as skipped.

---

## 2. For users

### 2.1 Asking for infrastructure

Open *AI Cloud Agent* and write what you want. Name the resource, give the
sizes and the OS, and mention your SSH key. If something is missing, the agent
asks before it proposes a plan.

| Resource | Example request | Tool | Risk |
|---|---|---|---|
| VPS: create | *Create an Ubuntu 24.04 VPS named web-01 with 2 vCPU, 4 GB RAM, 40 GB disk and my SSH key* | `vps.create` | medium |
| VPS: start (MCP only) | *Start web-01* | `vps.start` | medium |
| VPS: restart (MCP only) | *Restart web-01* | `vps.restart` | medium |
| VPS: stop (MCP only) | *Stop web-01* | `vps.stop` | high |
| VPS: resize (MCP only) | *Resize web-01 to 4 vCPU and 8 GB RAM* | `vps.resize` | high |
| VPS: snapshot (MCP only) | *Take a snapshot of web-01 called before-upgrade* | `vps.snapshot.create` | medium |
| VPS: restore a snapshot (MCP only) | *Roll web-01 back to the snapshot before-upgrade* | `vps.snapshot.restore` | destructive |
| VPS: delete | *Delete the VPS web-01* | `vps.delete` | destructive |
| Bucket: create | *Create a storage bucket called assets* | `bucket.create` | medium |
| Bucket: delete (MCP only) | *Delete the bucket assets* | `bucket.delete` | destructive |
| App | *Deploy the Node.js app from https://github.com/example/shop, branch main, port 3000, as shop* | `app.create` | medium |
| n8n | *Set up an n8n instance called ops-flows with 2 GB RAM* | `n8n.create` | medium |
| OpenClaw | *Create an OpenClaw agent called support-bot that uses Anthropic* | `openclaw.create` | medium |
| Game server | *Create a Minecraft Java server called survival with 10 player slots* | `game_server.create` | medium |
| Android | *Create an Android instance called qa-phone* | `android.create` | medium |

The chat agent recognizes requests by keywords. It turns the create requests
above, VPS deletion, managed-database creation and template deployment into
plans. When you delete a VPS, call it a VPS or VM (*Delete the VPS web-01*).
The rows marked *MCP only* have no chat intent: ask for them from an MCP client
(section 2.9) or use the resource's own page. The result is the same kind of
plan, and you approve it in the dashboard.

Phases 2–4 add these chat workflows:

| Request | Example | What happens |
|---|---|---|
| Managed database | *Create a PostgreSQL database called app-db with the standard plan* | Asks for a missing engine or plan, then files a `create_database` plan. The password is generated during execution and shown once in *Operations · Databases*. |
| Template | *Deploy the web-stack template with domain=shop* | Looks up a visible template by name, asks for missing typed parameters, then files a plan in this conversation. |
| Diagnostics | *Why is web-01 slow?* / *Health check all resources* | Read-only: collects facts, ranks likely causes and lists remediation options. It never plans or changes anything. |
| Advisor | *Optimize my costs* / *What is my monthly spend?* | Read-only deterministic cost and optimization report. |

Diagnostics and advisor are per-turn read-only answers: they leave an open plan
in the conversation untouched. A database or template request is an explicit
new operation, so it starts a new active plan. *Save template* is not a chat
operation: create templates in *Operations · Templates*.

Read-only tools never need an approval. An MCP client runs them at once
(section 2.9):

| Question | Tools |
|---|---|
| *List my servers* / *Is web-01 running?* | `vps.list`, `vps.get`, `vps.status` |
| *List my applications, databases, domains...* | `service.list`, `service.get` |
| *Show the logs of app-01* | `service.logs` |
| *How much quota do I have left?* | `quota.check` |
| *Which OS templates and plans can I use?* | `catalog.templates`, `catalog.plans` |
| *Why is web-01 failing?* | `diagnostics.resource` |
| *What can I save?* | `advisor.optimize` |
| *What is connected to what?* | `infra.topology` |
| *Which templates can I deploy?* | `template.list`, `template.get` |

The dashboard chat answers such questions with the same read tools: *List
my servers*, *Show my VPS* or *What is the status of web-01* run `vps.list`
(and `vps.status` for a named VPS) and reply with a read-only answer. A
question about quota answers from your quota; a question about buckets, game
servers, Android, n8n, OpenClaw or apps points to their page. A read request
never starts or changes a plan, and an open plan in the conversation stays
as it was. A message is a read request when it asks to list, show or check
something and contains no change verb (create, deploy, start, stop, delete,
resize, restore, ...) and no troubleshooting words (why, error, fail, ...).

Good to know:

- **Resize** needs a stopped VM, and Proxmox can only grow a disk, never
  shrink it. The plan does **not** add stop and start tasks, and the
  resources of one plan run in no fixed order. Use separate plans: stop the
  VM, then resize it, then start it. A VM that is not stopped when the plan
  is validated gives the warning `INSTANCE_RUNNING`. If it still runs when the
  task runs, the task fails at once with `REQUEST_REJECTED` (no retry). A
  smaller disk is refused at validation (`DISK_SHRINK`).
- **Snapshot restore** rolls the VM back and discards everything written
  after the snapshot. That is why it is `destructive`.
- **Several resources** can be part of one plan; each gets its own resource
  entry and tasks. The chat agent and MCP calls still propose one resource per
  plan, but a template can hold several and an edited specification can too
  (section 2.4). A *Delete created* recovery plan (section 2.7) can hold
  several.
- **Wiring resources in one plan.** A resource can name another resource of
  the same specification in `depends_on`. `app.create` also understands
  `vps_key`, `database_key` and `bucket_keys`, and `database.create` /
  `n8n.create` / service creates understand `vps_key`; the policy engine adds
  the dependency automatically. The app is therefore created after its
  database, buckets and VPS, and the topology records `hosts`, `uses` and
  `routes_to` edges. A dependency cycle is refused at validation.
- **Never paste API keys or passwords into the chat.** Chat messages are kept
  in the conversation history. Plans, events and reports never contain
  secrets (section 3). `openclaw.create` takes only the name, the provider
  and the model. Its input schema has no key field, and a plan or MCP call
  that adds a field the tool does not declare (such as `api_key`) is refused
  before anything is stored. Set the provider key after the agent exists, in
  its settings under *AI Agents*
  (`PUT /api/v1/ai-agents/openclaw/instances/:id/config`). The legacy MCP
  tool `cloud.create_openclaw_instance` drops an `api_key` argument.
- **Every plan needs your approval.** Every resource action maps to a
  `medium` or riskier tool, so every Phase 1 plan has
  `requires_approval: true`. Nothing runs automatically. Only read tools
  called from an MCP client run at once, and they never become a plan.

### 2.2 Reading a plan

The agent answers with a plan card. Review it before you approve: the
approval applies to the **revision** you see (v1, v2, ...) and to nothing
else.

| Part | What it tells you |
|---|---|
| Summary and version | One line (*Create Ubuntu 24.04 VPS web-01 (2 vCPU, 4 GB, 40 GB)*) and the revision number. `created_by` says whether the planner or a person wrote this revision. |
| Resources | Every resource the plan touches: key, type (`vps`, ...), action (`create`, ...) and configuration (name, template, plan, vCPU, memory, disk, SSH keys). |
| Tasks and dependencies | The steps. Each task names its tool, its risk and the tasks it waits for (`depends_on`). |
| Estimated resources | What the plan adds: vCPUs, memory, disk, instances. Compare it with your quota. |
| Validation | `passed`, `warnings` or `failed`, with the policy engine's messages (for example `NO_SSH_KEY`). |
| Risk | The highest risk of all tasks. It decides what the approval asks for (section 2.3). |

The plan from the contract, written out:

```
Plan v3: Create Ubuntu 24.04 VPS web-01 (2 vCPU, 4 GB, 40 GB)      risk: medium

  task                    title                  risk     waits for
  quota.check             Validate quota         read     -
  vps.create:web-01       Provision VPS web-01   medium   quota.check
  vps.wait_ready:web-01   Wait until running     read     vps.create:web-01

Estimate     2 vCPU, 4096 MB RAM, 40 GB disk, 1 instance
Validation   passed
```

Validation results:

| Result | Meaning |
|---|---|
| `passed` | Nothing found. |
| `warnings` | You can approve. Read the warnings first. |
| `failed` | You cannot approve: the approve call answers `422 VALIDATION_FAILED` with the errors. Fix the cause (free quota, choose another size), then edit the plan or run the validation again. |

A plan moves through these states:

```
draft -> validating -> awaiting_approval -> approved -> executing -> verifying -> completed
ends early as:  rejected | cancelled | failed | partially_completed
```

*Reject* says "not this plan" (`rejected`, final). *Cancel* stops the plan
and also an execution that is already running.

### 2.3 Risk levels and approvals

Every tool has a fixed risk level. A plan's risk is the highest risk of its
tasks: a plan that creates one VPS and stops another is `high`.

| Risk | Tools | To approve, you... | Approval valid for |
|---|---|---|---|
| `read` | `vps.list`, `vps.get`, `vps.status`, `quota.check`, `catalog.templates`, `catalog.plans`, `vps.wait_ready`, `vps.verify`, `service.list`, `service.get`, `service.logs`, `bucket.object.list`, `domain.list`, `template.list`, `template.get`, `diagnostics.resource`, `advisor.optimize`, `infra.topology` | nothing: no approval needed | - |
| `low` | `template.save` (stores a template, never infrastructure) | nothing | - |
| `medium` | `vps.create`, `vps.start`, `vps.restart`, `vps.snapshot.create`, `bucket.create`, `app.create`, `n8n.create`, `openclaw.create`, `game_server.create`, `android.create`, `database.create`, `template.deploy`, `service.backup.create`, `app.deploy`, `bucket.update`, `domain.claim` | **approve** the revision you see | 60 min |
| `high` | `vps.stop`, `vps.resize`, `service.power`, `domain.release` | approve **and acknowledge the risks** | 60 min |
| `destructive` | `vps.delete`, `vps.snapshot.restore`, `bucket.delete`, `service.delete`, `service.backup.restore`, `bucket.object.delete` | approve, acknowledge the risks **and type the exact name(s)** of the target(s) | 15 min |

- **Version-bound.** Approving sends the revision number and its `spec_hash`
  (a fingerprint of the plan's content). If the plan changed since you opened
  it, the approval is refused (`409 PLAN_VERSION_MISMATCH`): look at the new
  revision and approve that one.
- **Typed confirmation (destructive).** The plan lists the names to type
  (`confirmation_required.expected`). Typing is case-sensitive; separate
  several names with commas: `web-01,web-02`.
- **Approve and start.** By default an approval starts the execution at once
  and is used up by it. API clients can approve with `"execute": false` and
  start later with `POST /api/v1/ai/plans/:id/execute`, before the approval
  expires. An expired approval cannot start anything: approve again.
- **Admin-only tools** need the `admin` role even in an approved plan
  (`403 ADMIN_REQUIRED`). `GET /api/v1/ai/tools` shows them
  (`requires_admin`). No Phase 1–4 tool is admin-only. The check runs at
  approval, at `/execute` and again before each task, ready for later tools.

What the dashboard sends when you approve the deletion of `web-01`:

```json
POST /api/v1/ai/plans/<plan-id>/approve
{ "version": 4, "spec_hash": "sha256:…", "acknowledge_risks": true,
  "confirmation": "web-01", "execute": true }
```

### 2.4 Editing a plan

- Editing a plan that awaits approval (in the dashboard, or
  `PATCH /api/v1/ai/plans/:id` with `base_version`) creates a **new
  revision** (`created_by: user`). The policy engine validates it again.
- Every earlier approval of the plan becomes `superseded`. Approve the new
  revision; the old approval can start nothing.
- The revision history stays available (`GET /api/v1/ai/plans/:id/revisions`)
  and the workspace shows what changed between revisions.
- If someone else (or another tab) saved a newer revision first, your edit is
  refused with `409 PLAN_VERSION_CONFLICT`: reload and edit again.
- A plan that is executing or finished cannot be edited
  (`409 PLAN_NOT_EDITABLE`): ask for a new plan.
- A plan approved with `"execute": false` (approved, not started yet) can
  still be edited. The edit creates a new revision, supersedes the approval
  and returns the plan to `awaiting_approval` (`draft` if the new revision
  fails validation). You can also reject or cancel it.
- A follow-up chat message (*make it 8 GB*) revises the open plan when the
  agent recognizes the same request (same intent) and the plan can still be
  edited. The result is a new revision with `created_by: planner`, and
  earlier approvals are superseded. A message that changes nothing in the
  specification writes no revision. In every other case (another kind of
  request, or a plan that is executing or finished) the agent creates a new
  plan. An explicit VPS delete request always starts a new plan.

Why an approval never survives an edit: it is bound to the `spec_hash`, a
SHA-256 over the plan's specification and tasks. Any change, even one
character, gives a different hash.

### 2.5 Following an execution

An execution goes `queued -> running -> verifying -> completed`. It ends as
`failed` or `partially_completed` when tasks fail, and as
`cancelling -> cancelled` after a cancel.

The timeline in the workspace shows the execution's events as they happen.
Events are numbered (`seq`, strictly increasing per execution), so a client
can resume the stream without gaps or duplicates.

| Events | Meaning |
|---|---|
| `execution.created`, `approval.recorded`, `execution.started` | The run begins; your approval is on record. |
| `task.queued`, `task.started`, `task.progress`, `task.retrying`, `task.succeeded`, `task.failed`, `task.skipped`, `task.cancelled` | The life of one task. |
| `verification.started`, `verification.result` | Checking the result (section 2.6). |
| `execution.completed`, `execution.failed`, `execution.partially_completed`, `execution.cancelled` | The outcome. |
| `execution.finished` | Always the last event. The live stream ends here. |

Task states:

| State | Meaning |
|---|---|
| `queued` | Ready; waiting for the engine to pick it up. |
| `blocked` | Waiting for the tasks it depends on. |
| `running` | In progress (`attempt` of `max_attempts`). |
| `retrying` | The last attempt failed; the next one starts at `next_attempt_at`. |
| `succeeded` | Done. `output` holds references such as `instance_id` and `job_id`. |
| `failed` | No attempts left. `error` has a code and a message. |
| `cancelled` | Stopped by a cancel. |
| `skipped` | Not run, because a task it depends on failed. |
| `compensating` | Reserved for automatic cleanup after a failure. Phase 1 never sets it: there is no automatic rollback (section 2.7). |

**Retries.** A failed attempt is retried by itself after a pause, up to the
task's `max_attempts`: 3 attempts for every idempotent Phase 1 tool, 1 for a
tool that is not idempotent (`vps.snapshot.restore`), which is **never
retried by itself**. The pause before
attempt *n*+1 is 2 s × 2^(*n*−1), so 2 s and then 4 s. It is capped at 60 s,
and ±20 % jitter is added on top. Operators can change the 2 s (section 4.1).
A manual retry (section 2.7) gives a failed task 3 more attempts (one for a
tool that is not idempotent).

- **Not retried:** requests the platform refuses (`QUOTA_EXCEEDED`,
  `RESOURCE_NOT_FOUND`, `NAME_TAKEN`, `REQUEST_REJECTED`, `INPUT_INVALID`,
  `ADMIN_REQUIRED`), and `INSTANCE_FAILED` from `vps.wait_ready`.
- **Retried:** other hypervisor or service errors (`TOOL_ERROR`,
  `NO_CAPACITY`, `LOOKUP_FAILED`), and every failed provisioning job, under the
  job's error code (for example `CLONE_FAILED` or `JOB_STALE`).
- **Timeouts:** `TOOL_TIMEOUT` is retried for every idempotent tool.
- **Tools that are not idempotent** (`idempotent: false` in the registry:
  `vps.snapshot.restore`, `app.deploy`, `service.backup.create` and
  `service.backup.restore`): no automatic retry for any error, including
  `TOOL_TIMEOUT` and `TOOL_ERROR`. When a replica dies or stops while such a
  task runs, the re-claimed attempt is not run again: the task fails with
  `INTERRUPTED_NOT_IDEMPOTENT`, because the interrupted run may already have
  rolled the VM back, deployed the commit or written the backup. Check the
  resource, then retry the task yourself if needed.

The timeout of one attempt depends on the tool (`timeout_seconds` in
`GET /api/v1/ai/tools`):

| Timeout | Tools |
|---|---|
| 900 s | `vps.create`, `database.create`, `service.backup.restore` |
| 600 s | `vps.wait_ready`, `service.backup.create`, `service.delete` |
| 300 s | `vps.start`, `vps.restart`, `vps.stop`, `vps.delete`, `vps.snapshot.create`, `vps.snapshot.restore`, `service.power`, `app.deploy` |
| 180 s | `vps.resize` |
| 120 s | `game_server.create`, `android.create`, `advisor.optimize` |
| 60 s | `app.create`, `n8n.create`, `openclaw.create`, `bucket.create`, `bucket.delete`, `vps.verify`, `service.logs`, `bucket.object.list`, `domain.list`, `domain.claim`, `domain.release`, `template.list`, `template.get`, `template.save`, `template.deploy`, `diagnostics.resource`, `infra.topology`, `bucket.update`, `bucket.object.delete` |
| 30 s | `vps.list`, `vps.get`, `vps.status`, `quota.check`, `catalog.templates`, `catalog.plans`, `service.list`, `service.get` |

Each attempt has its own idempotency key, so a retry never creates a second
copy of a resource (section 4.4).

**Cancel.** `POST /api/v1/ai/executions/:id/cancel` (or cancel in the
workspace) moves the execution to `cancelling`, then `cancelled`. Tasks that
have not started (`queued`, `blocked`, `retrying`) end as `cancelled` at once.

- **Running tasks** are interrupted. This happens at once on the API replica
  that received the cancel. On another replica it happens at the task's next
  lease heartbeat (10 s by default). The task ends as `cancelled`.
- **Work already started is not undone.** A provisioning job keeps running to
  its end: a VPS being created is still created. A hypervisor call in progress
  (resize, snapshot) finishes. The task is recorded as `cancelled` even when
  that work completes. Resources it created appear in the execution's report.
- **Finished executions:** cancelling one answers `409 EXECUTION_NOT_ACTIVE`.

Nothing is deleted automatically, and a cancelled execution offers no
recovery options. Resources that were already created stay. Remove them
with a new plan (*Delete the VPS web-01*).

### 2.6 Verification results

After the tasks finish, the execution enters `verifying`. The verification
engine compares the real state with the plan: for a VPS, that it exists, is
running and has an IP address (read from Proxmox and the QEMU guest agent).
Each task's `verification` lists its checks (`pass`, `fail`, `skip`) with
details. Only write tasks that succeeded are verified; `read` tasks such as
`quota.check` and `vps.wait_ready` are not.

| Tool | Checks | Can be `verified` |
|---|---|---|
| `vps.create`, `vps.start`, `vps.restart` | Record says `running`; hypervisor power state `running`; the guest agent reports an IPv4 address | Yes |
| `vps.stop` | Record says `stopped`; hypervisor power state `stopped` | Yes |
| `vps.delete` | Record deleted; the VM no longer exists on the hypervisor | Yes |
| `vps.snapshot.create` | Snapshot recorded; snapshot listed by the hypervisor | Yes |
| `bucket.create`, `bucket.delete` | Bucket record and its directory on the storage volume present (create) or gone (delete) | Yes |
| `vps.resize` | Recorded size equals the plan. The hypervisor configuration is not compared (`skip`) | No, always `unverified` |
| `vps.snapshot.restore` | The instance exists. A rollback cannot be observed afterwards (`skip`) | No, always `unverified` |
| `app.create`, `n8n.create`, `openclaw.create`, `game_server.create`, `android.create` | The record exists with an expected status (for example `provisioning` or `running`). No runtime probe (`skip`) | No, always `unverified` |
| `database.create` | The database record exists and is `running` (or still `provisioning`). No live connection probe (`skip`) | No, always `unverified` |
| `service.power` | The service record has the requested power state | Yes for VPS; otherwise `unverified` through the simulated/runtime skip |
| `service.delete` | The service record is gone (VPS also checked on the hypervisor) | Yes for VPS; otherwise `unverified` |
| `service.backup.create`, `service.backup.restore` | A backup id was recorded; backup contents / restore result cannot be observed (`skip`) | No, always `unverified` |
| `app.deploy` | The latest deployment exists and is not `failed`; runtime health is not probed (`skip`) | No, always `unverified` |
| `domain.claim`, `domain.release` | The domain record exists; edge routing is not checked (`skip`) | No, always `unverified` |
| `bucket.update` | The recorded visibility matches the plan | Yes |
| `bucket.object.delete` | The object record is gone | Yes |

A check that cannot reach its target (hypervisor or guest agent not
answering) is `skip`, never `pass`.

| Result | Meaning | What to do |
|---|---|---|
| `verified` | Every check passed against the real hypervisor. | Nothing. |
| `unverified` | At least one check was skipped: the platform runs the simulated hypervisor, the tool has a check that is always skipped (table above), or a check could not reach its target (for example, the guest agent does not answer). | The resource may well be fine: check it yourself (VM page, SSH). For a VPS create, start, restart, stop or delete in production, it points to a setup problem (section 4.9). |
| `failed` | A check found a difference, for example the VM is not running or has no IP address. | Read the failed check. Fix the resource (for example start the VM), or delete it with *Delete created* or a new plan. |

**The simulated hypervisor never shows `verified`.** In demo mode and with
`PROXMOX_MOCK=true` every result is `unverified`, because nothing real was
checked: the verifier adds a `simulated_platform` skip where no other check
was skipped.

**Effect on the outcome.** A `failed` verification ends the execution as
`partially_completed`, although every task succeeded, and the report lists
the failed check under warnings. Recovery then offers *Keep* and *Delete
created*, but not *Retry failed* (no task failed). `unverified` does not
change the outcome: the execution ends `completed` with a warning in the
report.

### 2.7 Partial failures and recovery

The orchestrator **never deletes a resource it created successfully**. Only
the leftovers of a failed `vps.create` attempt (the half-made VM and its
instance record) are removed, before the next attempt starts. When some
tasks succeed and others fail, the execution ends `partially_completed`, keeps
everything that worked and offers recovery options.

```
Plan v1: create VPS web-01 and web-02              execution: partially_completed

  quota.check             succeeded
  vps.create:web-01       succeeded   web-01 running, ssh ubuntu@10.0.0.5
  vps.wait_ready:web-01   succeeded
  vps.create:web-02       failed      error code + message, no attempts left
  vps.wait_ready:web-02   skipped     (waits for vps.create:web-02)

Recovery:   Retry failed  |  Diagnose  |  Keep  |  Delete created  |  Compensate
```

| Option | `action` | What happens |
|---|---|---|
| Retry failed | `retry_failed` | Re-queues the failed tasks and the tasks skipped because of them. Same as retrying each failed task (`POST /api/v1/ai/executions/:id/tasks/:taskId/retry`; only `failed` tasks, otherwise `409 TASK_NOT_RETRYABLE`). |
| Diagnose | `diagnose` | Read-only diagnostics of the resources the failed execution created or modified (up to five). Changes nothing and creates no plan. |
| Keep | `keep` | Accepts the partial result. The execution stays `partially_completed`; the created resources stay and count against your quota. |
| Delete created | `delete_created` | Offered only when the execution **created** a VPS or bucket that still exists (resources it only modified, such as a restarted VPS, do not count; once the created resources are gone the option disappears). Creates a **new destructive plan** that removes the VPS and buckets this execution created and that still exist. Phase 1 has no delete tool for apps, n8n, OpenClaw, game servers and Android: the plan's description names them, and you remove them from their own pages. You review and approve the plan like any destructive plan (typed names, 15 min). Nothing is deleted before that. |
| Compensate | `compensate` | The same approval-bound destructive plan as *Delete created*, labelled as compensation: remove what this execution created and keep everything else. Nothing is deleted before you approve it. |

Each option says whether it needs a new approval (`requires_approval`).
Options appear on `failed` and `partially_completed` executions (*Diagnose*
only when a task failed, *Keep* only on `partially_completed`). Completed and
cancelled executions have none.
*Keep* or *Delete created* where they do not apply, or *Delete created* when
nothing can be deleted by a plan, answer `409 RECOVERY_NOT_AVAILABLE`.
*Retry failed* without a failed task answers `409 TASK_NOT_RETRYABLE`. API
replies: `retry_failed` answers `202` with the execution, `keep` `200` with
the execution, `diagnose` `200` with its `diagnostics`, and `delete_created`
or `compensate` `201` with the `plan_id` of the new plan.

Every finished execution has a **report**: a summary, the resources (type,
id, name, status and access, such as `ssh ubuntu@10.0.0.5`), warnings and
suggested next actions.

### 2.8 Leaving and coming back

- Executions run on the server. Closing the tab or losing the connection does
  not stop them, and neither does a restart or a rolling update of the
  platform (section 4.2).
- When you come back, open the conversation: the timeline loads the events so
  far and continues live. A client resumes the event stream after the last
  event it saw (`Last-Event-ID` or `?after=<seq>`), so it misses nothing and
  shows nothing twice. If the live stream is not available, clients poll
  every 2 s instead (contract section 5).
- A plan points to its newest execution (`latest_execution_id`). API clients
  list executions with `GET /api/v1/ai/executions` (filters `conversation_id`,
  `plan_id`, `status`).

### 2.9 MCP clients (Claude Desktop, Cursor)

The same tool registry is served over MCP at `POST /api/v1/mcp/rpc`
(JSON-RPC 2.0: `tools/list`, `tools/call`), acting as your account.

| Tool risk | What `tools/call` does |
|---|---|
| `read` (`vps.list`, `vps.get`, `vps.status`, `quota.check`, `catalog.templates`, `catalog.plans`, `vps.wait_ready`, `vps.verify`, `service.list`, `service.get`, `service.logs`, `bucket.object.list`, `domain.list`, `template.list`, `template.get`, `diagnostics.resource`, `advisor.optimize`, `infra.topology`) | Runs at once and returns the result. |
| `low` (`template.save`) | Runs at once and stores a reusable template. It changes no infrastructure; the template is still validated and secrets are rejected. |
| `medium`, `high`, `destructive` (every other write tool) | **Never executes.** Creates a plan and returns it for approval in the Nevatal dashboard. |

The answer to a write call:

```json
{ "plan_id": "…", "version": 1, "status": "awaiting_approval", "risk_level": "destructive",
  "url": "/dashboard/ai/conversations/<conversation_id>?plan=<plan_id>" }
```

`status` is `awaiting_approval`, or `draft` when the policy engine rejected
the request (the plan then lists its validation errors). MCP-created plans
are filed in the conversation *MCP requests*. `url` is the dashboard link to
the plan (a path on your control-plane host). `/dashboard/ai?plan=<plan_id>`
also works; the workspace switches to the conversation link.

**Tool permissions.** A connection to an MCP server (*Integrations → MCP*,
or `POST /api/v1/integrations/mcp/connections`) holds one grant per tool,
set with `PATCH /api/v1/integrations/mcp/connections/<id>/permissions`
(`{"tool_id", "granted"}`; tool ids from `GET .../connections/<id>/tools` or
`GET /api/v1/integrations/mcp/servers`). A tool you disable is
refused for every `tools/call`, read tools included, with a JSON-RPC error
(`code` `-32001`, `data.code` `TOOL_PERMISSION_DENIED`): nothing runs and no
plan is created, and the refusal appears in the MCP audit log as `denied`.
Disabling a tool also disables its legacy alias (`vps.create` and
`cloud.create_vm`). Without a connection to the Nevatal Cloud server, every
tool is allowed; other MCP servers (GitHub, ...) must be connected first.
The token an MCP client uses is your session token, so these grants limit
well-behaved clients, not a client that misuses the token.

The OpenRouter assistant (*AI Assistant* page and the floating widget)
follows the same rule: its **Review plan** button turns the proposed
container into a plan in the conversation *AI assistant requests* and opens
it here. Nothing is created until you approve it.

An assistant in Claude Desktop or Cursor can therefore look around freely and
*propose* changes, but only you approve them, in the dashboard, with the same
rules as section 2.3 (typed names for destructive plans). There is no approve
tool, and nothing the assistant says or sends counts as an approval.

**Connecting a client.** What the endpoint supports today:

| | |
|---|---|
| Transport | `POST https://<your-host>/api/v1/mcp/rpc`: one JSON-RPC 2.0 request per POST, answered with one `application/json` response |
| Methods | `initialize` (protocol versions `2025-06-18`, `2025-03-26`, `2024-11-05`), `ping`, `tools/list`, `tools/call`. Notifications (`notifications/initialized`, ...) answer `202` without a body. |
| Authentication | `Authorization: Bearer <token>`, where the token is the session token from `POST /api/v1/auth/login` (section 4.7 shows how to get one). It is valid for 7 days and acts with your whole account. Phase 1 has no separate API tokens. Accounts without a password (social login) can copy the same token from the browser's local storage (`nevatal_token`) after logging in to the dashboard. The session cookie does not work here (it is accepted for GET only). |
| Not supported | OAuth and its discovery; the server-to-client event stream (`GET` on the endpoint answers `404`); `Mcp-Session-Id` sessions; JSON-RPC batches; resources and prompts |

A client therefore has to send a fixed `Authorization` header to a remote
HTTP server and use POST only. A client that reaches remote servers only
through OAuth, or only through a local (stdio) process, needs a local bridge
that forwards stdio to this URL and adds the header. No client configuration
was tested against this endpoint in this repository. Follow your client's
documentation for remote MCP servers with custom headers. When the token
expires after 7 days, log in again and replace it.

Check the endpoint and your token with curl (`API` and `TOKEN` as in
section 4.7):

```bash
curl -s -X POST "$API/mcp/rpc" -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq -r '.result.tools[].name'
```

### 2.10 Operations Center

*Operations* (`/dashboard/operations`) groups the Phase 2–4 surfaces in eight
tabs. It is a UI over the same REST API; the *AI Cloud Agent* remains the place
where plans are reviewed and approved.

| Tab | What you do there |
|---|---|
| **Databases** | Create, power, inspect, back up, restore and delete managed databases; show credentials once. |
| **Templates** | Save, edit, version and deploy reusable infrastructure specifications. |
| **Checks** | Configure recurring health, backup-freshness and quota checks; run one now. |
| **Policies** | Turn platform events into notifications or approval-bound plan proposals. |
| **Advisor** | See the estimated monthly cost, quota use and deterministic optimization advice. |
| **Topology** | See resources and the relationships recorded between them. |
| **Agent roles** | See the logical roles and the registry tools each one covers. |
| **Diagnostics** | Inspect one resource's facts and likely causes, and propose a remediation plan. |

Actions you take yourself in *Operations* are ordinary owner actions: they are
authenticated by your session and are not an AI decision. For example,
creating or powering a database, recording a backup, editing a template, or
saving a check or policy does not produce a plan. Two Operations actions do
produce plans, because they change infrastructure or compute: **deploying a
template** and **creating a remediation plan** from diagnostics. Those plans
wait for your approval like every other plan.

The bell in the top bar opens notifications (section 2.14). The unread count is
refreshed every 30 s. Clicking a notification marks it read and follows its
`action_url`: an execution goes to its conversation, a failed check goes to
*Operations · Checks*, and a policy-proposed plan goes to the plan in the AI
workspace.

Diagnostics can be opened directly. From a resource row or another Operations
tab the dashboard uses:

```
/dashboard/operations?tab=diagnostics&type=vps&id=<resource-id>&name=<name>
```

`type` is the resource type (`vps`, `app`, `n8n`, `openclaw`, `game_server`,
`android`, `database`, `bucket` or `domain`). The page ignores an unknown tab
and falls back to *Databases*.

### 2.11 Infrastructure templates

A template is a saved, parameterized version of the same specification a plan
uses: resources, actions, configuration, tasks and dependencies. Create one in
*Operations · Templates* (or call `POST /api/v1/ai/templates`). You give it:

- a **name** (2–120 characters) and an optional description;
- a **specification** with resources such as `vps`, `app`, `database`,
  `bucket`, `n8n`, `openclaw`, `game_server`, `android` or `domain`;
- optional **parameters**, each with a key, label, type (`string`, `integer`,
  `boolean` or `enum`), `required`, default, enum values and min/max; and
- **public** visibility, off by default.

Use a parameter as a whole value or inside a string:

```json
{"name": "{{name}}", "plan": "{{plan}}", "domain": "{{name}}.example.com"}
```

A whole-value placeholder keeps the parameter type (an integer stays an
integer); an embedded placeholder becomes text. Deploying asks for missing
required parameters and refuses unknown ones. Every edited template increments
its `version`; `usage_count` counts successful deployments.

Templates are deliberately not a way to store secrets. The API rejects
secret-looking configuration keys (`password`, `token`, `api_key`,
`private_key`, `credential`, ...) and values that embed credentials. Generate
or enter secrets after deployment; a template that needs one must take a
non-secret parameter or leave the field for later configuration.

Deploying a template creates a normal plan from the resolved specification. It
is validated by the same policy engine and **nothing runs until you approve
it**. From the UI or `POST /ai/templates/:id/deploy`, the plan is filed in the
*Infrastructure templates* conversation. From chat it stays in the active
conversation, so the approval card appears where you asked for it. A public
template is readable and deployable by every user, but remains owned by its
author; only the owner can edit or delete it.

The rendered specification must still use supported resource/action pairs and
must not contain unresolved `{{...}}` placeholders. Templates only save typing
and make multi-resource plans reusable; they do not bypass validation,
approval or execution (section 2.2).

### 2.12 Managed databases

*Operations · Databases* creates logical managed databases for PostgreSQL,
MySQL, Redis and MongoDB. Supported sizing presets are `starter` (1 vCPU,
1 GB RAM, 10 GB), `standard` (2 vCPU, 4 GB, 50 GB) and `power` (4 vCPU,
8 GB, 200 GB); `custom` accepts up to 16 vCPU, 64 GB RAM and 1 TB. Engine
versions default to PostgreSQL 16, MySQL 8.4, Redis 7.4 or MongoDB 7.0.

The control plane owns the lifecycle, credentials, backups and health records.
Creation checks your quota (vCPU, memory, storage and instance count) and the
unique name, generates a random password, encrypts it with AES-256-GCM and
stores only the ciphertext. The password is returned **once** in the create
response. The owner can read it again with *Show credentials*
(`GET /databases/:id/credentials`), which is audited; it is never written to a
plan, execution event, report or log. The derived host is
`db-<name>.nevatal.internal`; treat it as a control-plane identifier, not proof
that a real database server is reachable.

| Action | Endpoint | Notes |
|---|---|---|
| List / read | `GET /databases`, `GET /databases/:id` | Owner only; other users get `404 DATABASE_NOT_FOUND`. |
| Plans | `GET /databases/plans` | The three presets. |
| Create | `POST /databases` | Returns `{database, password}`. `auto_backup` defaults to `true`. |
| Start / stop / restart | `POST /databases/:id/power/:action` | `start`, `stop` or `restart`. |
| Credentials | `GET /databases/:id/credentials` | Audited; may answer `503 CREDENTIALS_UNAVAILABLE`. |
| Back up / list / restore / delete backup | `POST /databases/:id/backups`, `GET /databases/:id/backups`, `POST /databases/:id/backups/:backupId/restore`, `DELETE /databases/:id/backups/:backupId` | Logical backup records; restore marks the database running. |
| Logs / metrics | `GET /databases/:id/logs?lines=`, `GET /databases/:id/metrics` | Simulated/deterministic log lines and usage values; no credentials. |
| Delete | `DELETE /databases/:id` | Soft-deletes the control-plane record. |

Statuses are `provisioning`, `running`, `stopped`, `failed` and `deleting`.
Creation currently records `running` immediately; there is no live runtime
probe. In the same way, a backup is a control-plane record with a
`backup_reference` and size, not a dump copied from a database server, and
restore is logical. Metrics are deterministic from the database name when the
platform is simulated or no measurements were recorded; logs are illustrative
and never contain credentials.

**Automatic backups are scheduled.** `auto_backup` defaults to `true` and an
explicit `false` is honored. A running database with it enabled gets a
logical backup every `ORCH_DB_BACKUP_INTERVAL_SECONDS` (default 24 h, minimum
60 s). Every API replica runs the scheduler and claims due databases with
`FOR UPDATE SKIP LOCKED`, advancing `next_backup_at`, so a backup is taken
once, not once per replica. A newly created database first backs up one
interval after creation. A `backup.freshness` check still reports its age
truthfully, and the manual backup button remains available.

Credentials are encrypted with a key derived from the deployment's
`JWT_SECRET`. Rotating that secret makes existing database passwords
undecryptable (`503 CREDENTIALS_UNAVAILABLE`); reset them or recreate the
database. This is a current limitation, not a recovery procedure.

The same lifecycle is exposed to the AI agent as `database.create`,
`service.power`, `service.delete`, `service.backup.create` and
`service.backup.restore` (section 2.1). Those tool calls become plans when
driven by the model; the Operations UI and REST calls above act directly for
the authenticated owner.

### 2.13 Diagnostics and remediation

Diagnostics turn recorded state into evidence. For one resource, `GET
/api/v1/ai/diagnostics/:resourceType/:id` returns:

- **facts** — status, metrics, addresses and last-backup age, each marked
  `ok`, `warning`, `critical` or `info`;
- **hypotheses** — ranked possible causes (`high`, `medium`, `low`) with the
  evidence that supports them; and
- **recommendations** — actions such as start, restart, resize, inspect logs,
  back up or delete, each with its risk, whether approval is required and the
  registry `tool`/`input` it maps to.

Supported types are `vps`, `app`, `n8n`, `openclaw`, `game_server`, `android`,
`database`, `bucket` and `domain`. The rules are deterministic:

- a stopped, failed or error resource is critical; a resource that is still
  provisioning has an informational transition fact;
- CPU at or above 75% is a warning, at or above 90% critical; memory at or
  above 80% is a warning (92% is high-confidence pressure); disk at or above
  80% is a warning (90% critical when reported as a threshold fact);
- a running VPS with no private IP gets a warning, and a database, game server
  or Android instance with no backup gets a backup warning;
- missing metrics are stated as `not observable`, not guessed.

The verdict is `healthy`, `degraded`, `critical` or `unknown`. Diagnostics only
read; the model may explain the result, but it cannot invent facts or execute a
recommendation.

To act, choose a recommendation in the panel or call `POST
/api/v1/ai/diagnostics/:resourceType/:id/remediate` with an `action`
(`start`, `stop`, `restart`, `backup_create`, `delete`, `deploy` or `resize`)
and, for resize, `vcpus`, `memory_mb` and/or `disk_gb`. This creates a plan in
the *AI diagnostics* conversation; it does not run anything. Review and approve
it like any other plan. `free_space` and other cleanup advice is advice only:
there is no cleanup tool, so grow the volume or clean the resource yourself.
Diagnostics never auto-remediate, and a policy cannot turn a recommendation
into an execution.

### 2.14 Checks, notifications and automation policies

#### Scheduled checks

*Operations · Checks* configures recurring read-only observations. The check
type decides the valid resource type:

| `check_type` | `resource_type` | What it observes |
|---|---|---|
| `instance.health` | `vps` | Recorded status, then CPU/memory/disk when metrics are available. |
| `app.health` | `app` | Recorded status and, outside simulation, an HTTPS probe of the app's health path. |
| `game_server.health` | `game_server` | Status and metrics. |
| `android.health` | `android` | Status and metrics. |
| `database.health` | `database` | Status and database metrics. |
| `backup.freshness` | `game_server`, `android`, `database` | Age of the last backup (warning after 7 days or when none exists). |
| `quota.usage` | `account` | vCPU, memory, storage and instance percentages. |

Create with `POST /ai/checks` (`name`, `resource_type`, `resource_id`,
`check_type`, `interval_seconds`, `enabled`). The interval is 60–86400 seconds
and defaults to 300; a check is enabled by default. There is one check per
owner, type and resource (`409 CHECK_EXISTS` for a duplicate). `PATCH
/ai/checks/:id` changes the name, interval or enabled flag; `POST
/ai/checks/:id/run` runs it immediately. `DELETE` removes it.

The monitor loop in every API replica claims due checks from PostgreSQL
(`FOR UPDATE SKIP LOCKED`), so replicas do not duplicate a run. It looks for
work every `MONITOR_INTERVAL_SECONDS` (default 30, minimum 5). Results are
`ok`, `warning`, `critical` or `unknown`; `last_result` carries the facts and
message, and `consecutive_failures` tracks consecutive bad runs.

A warning or critical result creates an in-app notification only when the
status changes from the previous run (deduplicated per check and status), and
evaluates enabled `check.failed` policies on every bad result. Thresholds:
CPU 80% warning / 90% critical, memory 85% / 92%, disk 85% / 95%. The app
probe only follows an HTTPS URL ending in `.apps.nevatal.id`, refuses IPs and
cross-host redirects, and is reported as simulated in mock mode. Checks never
start, stop or change a resource.

#### Notifications

Notifications are per user and are read at `GET /api/v1/notifications`
(`?unread=true&limit=50`, newest first), with the unread count in
`unread_count`. `GET /notifications/unread-count` is the light poll;
`POST /notifications/:id/read` and `POST /notifications/read-all` mark them
read. Read operations are idempotent where it matters: marking an already-read
notification again is not an error.

| Emitted type | When |
|---|---|
| `execution` | An execution reaches a terminal state: completed, failed, partially completed or cancelled. |
| `check` | A check changes to warning or critical. |
| `system` | A policy produced a notification, a policy-proposed plan, or a policy failed to create its plan. |
| `advisor` | Reserved; the advisor is read-only and does not currently emit notifications. |

Every notification can carry `resource_type`, `resource_id`, `action_url` and
severity (`info`, `warning`, `critical`). One user can never read another
user's notification (`404 NOTIFICATION_NOT_FOUND`).

#### Automation policies

A policy is a deterministic rule evaluated after an event. It can notify or
propose a plan; it can never execute infrastructure, approve a plan or call an
arbitrary tool. Create policies in *Operations · Policies* or with
`POST /api/v1/ai/policies`.

| Field | Values |
|---|---|
| `trigger` | `execution.failed`, `execution.completed`, `check.failed`, `advisor.recommendation` |
| `conditions` | Optional exact matches (or comma-separated alternatives) on `severity`, `action`, `resource_type`, `category`, `check_type`, `risk`; empty or `*` matches anything. Unknown condition keys never match. |
| `action` | `notify` or `create_plan` |
| `action_config` | For `notify`: optional `title`, `message`. For `create_plan`: required `template_id` and optional `parameters`. |
| `enabled` | Default `true`; disable instead of deleting to keep history. |

`execution.failed` fires for failed and partially completed executions;
`execution.completed` fires for the other terminal states, including
cancelled. `check.failed` fires for warning or critical check results, with
`action` set to `failed`. `advisor.recommendation` fires once for each
recommendation in an advisor report, with `action` set to `recommend`,
`category` (`waste`, `capacity`, `rightsize` or `reliability`), `risk` and
`resource_type` taken from the recommendation. It is evaluated whenever a
report is generated: *Operations · Advisor*, a chat question such as *optimize
my costs*, or the `advisor.optimize` MCP tool. Because the advisor only reads,
a policy on this trigger can only `notify`; `create_plan` is refused at
validation. Each recommendation notifies at most once, so re-running the
report does not repeat it.

`notify` creates a `system` notification. `create_plan` deploys the chosen
template with the configured parameters and files the resulting plan in the
*Infrastructure templates* conversation, then notifies you with a link. The
plan still needs your approval; a policy never approves it. Trigger counts and
`last_triggered_at` are recorded, and repeated findings are deduplicated. A
policy error never breaks the execution or check that produced the event.

### 2.15 Advisor, costs, topology and agent roles

**Advisor.** `GET /api/v1/ai/advisor` returns a deterministic report:

```json
{
  "generated_at": "2026-11-03T10:00:00Z",
  "currency": "USD",
  "monthly_total": 42.50,
  "cost_resources": [{"resource_type":"vps","name":"web-01","monthly":12.5, "...":"..."}],
  "optimizations": [{"category":"rightsize","title":"Consistently idle resource","risk":"high",
                      "requires_approval":true,"tool":"vps.resize","input":{"instance_id":"..."}}],
  "notes": ["Estimates use the platform rate card; they are not an invoice."],
  "quota": {"used_vcpus":4,"max_vcpus":16,"...":"..."}
}
```

`GET /api/v1/ai/advisor/costs` returns only `currency`, `monthly_total`,
`resources` (each with `monthly`) and `notes`. The estimate covers VPS, apps,
n8n, OpenClaw, game servers, Android instances, managed databases and buckets.
Running compute pays vCPU + memory + disk; a stopped or suspended resource pays
disk only; a bucket pays its stored size. Rates come from the operator's
`ORCH_COST_*` settings and are monthly estimates, not invoices.

Optimization rules are deterministic and ranked waste → capacity → rightsize
→ reliability:

| Category | Finding | Proposed action |
|---|---|---|
| `waste` | A resource stopped for at least 7 days | Take a final backup and delete it (destructive). |
| `waste` | An empty bucket | Delete it (destructive). |
| `capacity` | Memory or disk at 85%+ | Double the resource (high risk). |
| `rightsize` | CPU below 10% and memory below 30% with over 1 GB RAM | Halve vCPU and memory (high risk). |
| `reliability` | A failed or errored resource | Diagnose it, then redeploy or restart. |

Each recommendation carries `risk`, `requires_approval`, `tool` and `input`.
The advisor never acts on them: use *Open diagnostics* to inspect the resource
and propose an approval-bound plan. A recommendation is advice, not an
execution. Generating a report also evaluates your `advisor.recommendation`
policies, which can notify you about a finding (section 2.14).

**Topology.** `GET /api/v1/ai/topology` returns `{generated_at, nodes, edges}`.
Nodes come from your live resources (VPS, app, n8n, OpenClaw, game server,
Android, database, bucket, domain) with type, name, status and a small metadata
map. Edges come from recorded `resource_relationships` rows: `created`,
`modified`, `deleted`, `uses`, `hosts`, `depends_on` and `routes_to`. For
example, an app `uses` a database and bucket, a service `hosts` on a VPS, and a
domain `routes_to` a VPS. A relationship endpoint whose live resource is gone
is shown as type `unknown` rather than silently dropped. The topology is
descriptive: it does not prove reachability and does not change anything.

**Agent roles.** `GET /api/v1/ai/agents` returns the ten logical roles —
planner, infrastructure, application, database, network, security, diagnostic,
monitoring, advisor and verification — with the registry tools covered by each.
Roles classify work; they are **not permissions**. The policy engine, approval
manager and execution engine remain the only authority, and no role grants a
tool, bypasses a quota or approves a plan.

---

## 3. Security model

1. **Only the approve endpoint approves.** An approval exists only when the
   plan's owner calls `POST /api/v1/ai/plans/:id/approve` with a Bearer token
   (the dashboard does this for you). Chat messages ("yes, go ahead",
   "approved"), JSON pasted into the chat, the output of a tool, text the
   model read in a web page or a resource name, and MCP calls can never
   approve. The session cookie is accepted for GET requests only, so another
   web page, including a tenant app on a sibling subdomain, cannot approve
   through your browser.
2. **An approval covers one exact revision.** It is bound to the plan version
   and the `spec_hash` (SHA-256 of the canonical JSON of the specification and
   tasks). An edit supersedes it. One execution consumes it, and it cannot be
   used again. It expires after 60 min (`medium`, `high`) or 15 min
   (`destructive`). Old clients that send only `{plan_id}` or `{version}` are
   accepted only for plans up to `medium` risk.
3. **Stronger proof for riskier plans.** `high` needs an explicit risk
   acknowledgement. `destructive` also needs the exact target names, typed by
   you.
4. **Checked again at execution time.** Policy and quota run again when the
   execution starts, not only when the plan was made. The transactional
   reservation in instance creation stays the final authority: two
   executions racing for the last free vCPU cannot both win.
5. **Tenant isolation.** Plans, revisions, approvals, executions, tasks and
   events belong to one user. IDs of another user answer `404`
   (`PLAN_NOT_FOUND`, `EXECUTION_NOT_FOUND`, `TASK_NOT_FOUND`), exactly like
   IDs that do not exist. The policy engine checks that you own every resource
   a plan touches.
6. **No secrets in plans, events, outputs or reports.** Error messages are
   sanitized: no secrets, no internal addresses of other tenants.
7. **Rate limit.** At most 3 active executions per user
   (`429 TOO_MANY_EXECUTIONS`) and one active execution per plan
   (`409 EXECUTION_ACTIVE`).
8. **No host shell.** The engine can only call the tools in the registry
   (`GET /api/v1/ai/tools`), each with a declared input schema, risk level,
   timeout and admin flag. No tool runs commands on the Proxmox host or on the
   control-plane servers, and the model can neither add tools nor call the
   Proxmox API itself. Underneath, the Proxmox token's ACLs still apply
   (section 4.6).
9. **One execution path.** The old synchronous provisioning inside the
   approve request is gone. Every change goes plan, approval, execution
   engine. Each approval is recorded in the execution's events
   (`approval.recorded`).
10. **Secrets stay out of plans and templates.** Template specifications and
    parameters reject secret-looking keys and credential-bearing values.
    Managed database passwords are generated server-side, encrypted with
    AES-256-GCM and returned only once at creation or by the audited
    owner-only credentials endpoint. They are never stored in a plan, task,
    event, report or notification. The encryption key is derived from
    `JWT_SECRET`; rotating that secret makes old database passwords
    undecryptable (section 2.12).
11. **Checks cannot be used as a network scanner.** The app health probe only
    targets an HTTPS hostname ending in `.apps.nevatal.id`, refuses IP
    literals and cross-host redirects, and uses the app's stored health path.
    Every other check reads recorded platform state. Checks never change a
    resource.
12. **Automation policies never execute.** A policy can only create a
    notification or propose a plan from a saved template. The plan still
    passes policy validation and requires your approval. Conditions are exact
    deterministic matches; there is no expression language, script, webhook or
    model call. `advisor.recommendation` policies can only notify: an
    optimization is a tool suggestion, not a template deployment.
13. **Operations actions are owner actions.** The Phase 2–4 Operations
    endpoints authenticate your session and act for you (creating a database,
    powering it, recording a backup, editing a template or check). They are not
    model tools, cannot be invoked by chat text, and do not accept an approval.
    Model-driven changes still go through plans and approvals. Template deploy
    and diagnostics remediation also produce plans rather than executing
    directly.
14. **Diagnostics and notifications are tenant-scoped.** A resource of another
    user answers `404 RESOURCE_NOT_FOUND` (or `DATABASE_NOT_FOUND`); a
    notification of another user answers `404 NOTIFICATION_NOT_FOUND`.
    Diagnostics returns only authorized facts and redacted messages.

**What a prompt injection can do.** Instructions hidden in a web page, a
repository README or a resource name can at worst make the planner *propose*
an unwanted plan. They cannot approve it. Read the plan before you approve,
above all the resources and the risk level. A destructive plan always names
its targets and makes you type them.

---

## 4. For operators

### 4.1 Where it runs

- **No extra service.** The orchestrator (planner, policy, approvals,
  execution and verification) runs inside every API replica (`backend`) and
  keeps its state in the same PostgreSQL database. Scaling the API
  (`BACKEND_REPLICAS`, the Kubernetes HPA) scales the engine.
- **Upgrade** as usual ([install guide, section 16](PROXMOX_INSTALL.md#16-upgrades-and-rollback)).
  Migration `000007_orchestrator` creates the orchestrator's tables. Plans
  created before the upgrade get revision 1, written by the API at startup
  (log `AI orchestrator: wrote revision 1 of N existing plan(s)`) or when the
  plan is first read. Approvals still pending from before the upgrade expire:
  approve those plans again. Plans that were `provisioning` during the upgrade
  become `failed`; old `draft` and `awaiting_confirmation` plans become
  `awaiting_approval`. Take a database backup first.
- **Phase 2–4 schema.** Migration `000008_orchestrator_phases` is additive:
  `infra_templates`, `managed_databases`, `managed_database_backups`,
  `notifications`, `infra_checks` and `automation_policies`, plus the
  `related_resource_*` columns and owner index on
  `resource_relationships`. It seeds nothing and does not rewrite Phase 1
  rows. Existing plans, approvals and executions keep working. The down
  migration drops the new tables and the added relationship columns; export
  templates, database records and policies first if you may need to roll
  back. Migration `000009_managed_database_auto_backup` adds
  `managed_databases.next_backup_at` and its due index, which the automatic
  backup loop uses (section 4.10). A live database runtime is still not
  provisioned (section 2.12).
- **Demo mode** runs one API replica with the simulated hypervisor; every
  verification is `unverified`.
- **Fixed by the contract:** 3 active executions per user, approval expiry
  60 min / 15 min, SSE heartbeat every 20 s, polling fallback every 2 s.
- **Settings.** Environment variables of the API:

  | Variable | Default | Meaning |
  |---|---|---|
  | `ORCH_CONCURRENCY` | `4` | Tasks one API replica runs at once |
  | `ORCH_LEASE_SECONDS` | `30` | Task lease: how long a claim lasts without a heartbeat |
  | `ORCH_HEARTBEAT_SECONDS` | `10` | Lease renewal interval. At most half the lease; a larger value is replaced by lease / 3 and logged as a `Config warning` |
  | `ORCH_POLL_MS` | `1000` | How often a replica looks for runnable tasks |
  | `ORCH_RETRY_BASE_MS` | `2000` | Pause before the first automatic retry; it doubles per attempt (section 2.5) |
  | `MONITOR_INTERVAL_SECONDS` | `30` | How often every replica looks for due checks; values below `5` are raised to `5` (section 2.14) |
  | `ORCH_DB_BACKUP_INTERVAL_SECONDS` | `86400` | Automatic backup cadence of a running managed database; values below `60` become `60` (section 2.12) |
  | `ORCH_NOTIFICATION_RETENTION_DAYS` | `30` | Notifications older than this are deleted hourly; `0` keeps every row (section 4.10) |
  | `ORCH_COST_CURRENCY` | `USD` | Currency label of advisor estimates (section 2.15) |
  | `ORCH_COST_VCPU_MONTH` | `5` | Monthly price per vCPU used by the advisor |
  | `ORCH_COST_MEMORY_GB_MONTH` | `2.5` | Monthly price per GB of memory |
  | `ORCH_COST_DISK_GB_MONTH` | `0.1` | Monthly price per GB of disk |
  | `ORCH_COST_BUCKET_GB_MONTH` | `0.02` | Monthly price per GB stored in a bucket |
  | `JOB_HEARTBEAT_SECONDS` | `10` | Heartbeat interval of provisioning jobs (section 4.5) |
  | `JOB_STALE_AFTER_SECONDS` | `300` | Age of a job's last heartbeat after which the reaper fails it; at least 3 heartbeats |

  With Docker Compose, set them in `.env`: `docker-compose.yml` passes the
  settings in this table to the backend (empty = the default above). On
  Kubernetes, add them to the `nevatal-config` ConfigMap.
- **Fixed in code:** 3 attempts per task (3 more per manual retry; 1 for
  tools that are not idempotent), retry
  pause cap 60 s, maintenance scan every 5 s, an attempt interrupted 5 times
  fails with `LEASE_EXHAUSTED` (section 4.2), shutdown budget 50 s for tasks
  (section 4.3).
- **For tests only:** `ORCH_FAULTS` (fault injection; ignored in production
  and with a real hypervisor), `PROXMOX_MOCK_LATENCY_MS` (simulated
  hypervisor only). `NEVATAL_TEST_DATABASE_URL` is read by the integration
  tests, not by the API.
- Per-tool timeouts are declared in the registry: `timeout_seconds` in
  `GET /api/v1/ai/tools` (table in section 2.5).

### 4.2 Durable tasks: leases and heartbeats

1. Approving creates an execution and its tasks in PostgreSQL.
2. The engine in every replica looks for runnable tasks (all dependencies
   done) and claims one with `SELECT ... FOR UPDATE SKIP LOCKED`, so exactly
   one replica gets each task.
3. The claim is a **lease**: the replica records itself as the owner with an
   expiry time and renews the lease with heartbeats while the task runs.
4. When the task ends, its result and events are written and the tasks that
   depend on it are unblocked.

| Event | What happens |
|---|---|
| A replica crashes or is killed (`docker kill`, OOM, VM reset) | Its leases are no longer renewed and expire. Another replica (or the same one after its restart) re-claims the task and **re-drives the same attempt** with the same idempotency key: it continues with the job or resource that attempt already started, instead of starting a new one (section 4.4). A tool that is not idempotent (`vps.snapshot.restore`) is not re-driven: the task fails with `INTERRUPTED_NOT_IDEMPOTENT` and waits for a manual retry. |
| Graceful stop (`SIGTERM`: `./deploy.sh`, `docker compose stop`, rolling update) | The replica stops claiming and gives its running tasks up to 50 s to finish. It then stops the rest without a result and releases their leases, so other replicas take over at once instead of waiting for expiry (section 4.3). |
| The whole stack restarts (Docker Compose restarts all replicas together) | Executions pause and continue when the API is back. |

Timings (defaults, section 4.1):

- **Lease and heartbeat:** a lease lasts 30 s and is renewed every 10 s. A
  crashed replica's task is re-claimed about 30 s after its last heartbeat.
- **Claiming:** every replica looks for runnable tasks every 1 s, and at once
  when a task ends or an approval starts an execution on that replica.
- **Maintenance scan:** every 5 s, each replica advances stalled executions:
  task graphs that finished but were not finalized because of a crash,
  verifications whose lease expired, and running tasks of a cancelled
  execution whose runner died.
- **Interrupted attempts:** an attempt interrupted 5 times (its runner
  crashed, was killed or was stopped each time) fails with `LEASE_EXHAUSTED`
  on the next claim.
- **Events:** the `task.started` event of a re-claimed attempt says
  *resumed after an interrupted run*.

This changes the behaviour described in the
[install guide, section 16](PROXMOX_INSTALL.md#16-upgrades-and-rollback),
where a replica killed in the middle of a job leaves that job failed. The
provisioning job itself still runs inside one replica. If it dies with that
replica, the reaper fails it and cleans up (section 4.5). The orchestrator
task then continues with its next attempt, if it has attempts left.

### 4.3 Graceful shutdown (SIGTERM)

When a replica receives `SIGTERM` (`backend/cmd/api/main.go`):

1. It stops claiming tasks (within 5 s) and stops its stale-job reaper. The
   reapers of the other replicas keep running.
2. The HTTP server stops accepting connections and closes the open event
   streams at once. Clients reconnect, usually to another replica, and resume
   after their last event (`Last-Event-ID`). In-flight requests get 20 s to
   finish.
3. At the same time, running tasks and verifications get up to 50 s to
   finish normally. Tasks still running after 50 s are stopped without a
   result (within 3 s). Their leases, and the leases of running
   verifications, are released. Another replica re-claims them at once, with
   the same attempt and the same idempotency key. The log shows
   `Orchestrator: shutdown interrupted N running task(s); released N lease(s) for re-claim`.
4. Provisioning jobs (the background work of a VPS create, start, stop,
   restart or delete) run until 55 s after the `SIGTERM`. Jobs still running
   then end with the process, and the log warns about them. The reaper of
   another replica fails them as `JOB_STALE` (section 4.5), and the
   orchestrator retries the task.

The shutdown is designed to end within 60 s.

Grace periods already set in this repository: Docker Compose
`stop_grace_period: 60s` for `backend`; Kubernetes a 20 s `preStop` sleep and
`terminationGracePeriodSeconds: 90`. Stop the API with `SIGTERM`
(`docker compose stop`, `./deploy.sh`, `kubectl rollout restart`). After a
`SIGKILL` its tasks wait until their leases expire (30 s).

### 4.4 No duplicates: idempotency keys

- Each task **attempt** has its own key:
  `orch:<execution_id>:<task_key>:<attempt>`, for example
  `orch:6f1c…:vps.create:web-01:1`.
- Provisioning jobs carry the key, and `provisioning_jobs(owner_id,
  idempotency_key)` is a **unique index** (for non-empty keys). The database
  refuses a second job with the same key, so a re-claimed attempt finds and
  follows the existing job.
- A **new attempt** (new key, `...:2`) starts only after the previous attempt
  reached a terminal, cleaned-up state. Two attempts of one task never run at
  the same time.
- The bundled nginx never retries POST or PATCH on another replica
  (`frontend/nginx/snippets/proxy-api.conf`), so a slow approve is not sent
  twice. A used approval cannot start another run (`409 APPROVAL_CONSUMED`),
  and a plan has at most one active execution (`409 EXECUTION_ACTIVE`).
- Migration `000007_orchestrator` de-duplicates the keys of existing jobs
  before it creates the index. Of the jobs that share an owner and a key, the
  newest (by `created_at`) keeps the key. The older ones are renamed to
  `<first 200 characters of the key>:dup:<job id>`. Then the migration
  creates the unique index `uq_jobs_owner_idempotency` on
  `(owner_id, idempotency_key) WHERE idempotency_key <> ''`.

A crash in the middle of a VM clone:

```
replica A   claims vps.create:web-01, attempt 1   key orch:E:vps.create:web-01:1
replica A   creates provisioning job J with that key; the Proxmox clone starts
replica A   is killed
            the task lease expires
replica B   re-claims attempt 1 with the same key: no second job, it follows J
            J's worker died with replica A, so J has no heartbeat:
            the reaper fails J and cleans up (half-made VM deleted, quota released)
replica B   attempt 1 is now failed and cleaned up: attempt 2 starts after the pause,
            key orch:E:vps.create:web-01:2, a new job, one new VM
```

How a re-claimed attempt continues:

- **It follows the existing job.** The tool looks up the job by the
  attempt's idempotency key and follows it (it polls every 1 s) instead of
  creating a new one. A create or power operation that already completed is
  reported as done.
- **It cannot restart a dead worker.** The provisioning job runs inside the
  replica that started it. If that replica died, the job sends no more
  heartbeats. The attempt then waits until the reaper fails the job as
  `JOB_STALE`: up to `JOB_STALE_AFTER_SECONDS` (5 min) after the last
  heartbeat, plus one reaper interval (at most 30 s). The attempt fails as
  retryable, the half-made VM is gone, and the next attempt starts after the
  pause with a new key.
- **Earlier attempts finish first.** Before an attempt starts work, the tool
  waits for the jobs of earlier attempts of the same task to end. It removes
  the leftovers of a failed create. If an earlier job completed after all,
  its result is used.
- **Tools without a job** (bucket, app, n8n, OpenClaw, game server, Android)
  adopt the resource that an interrupted run of the same task already
  created, instead of creating a second one. They find it by the recorded
  relation, or by name when it was created after the execution began.
- **Direct hypervisor calls:** `vps.resize` skips the change when the
  recorded size already matches, and `vps.snapshot.create` reuses a snapshot
  of the same name. `vps.snapshot.restore` has no such check, so the engine never runs it
  twice by itself: it is not retried automatically, and a re-claimed restore
  fails with `INTERRUPTED_NOT_IDEMPOTENT` instead of rolling the VM back a
  second time. Only a manual retry runs it again.

### 4.5 Stale-job reaper

- Provisioning jobs send heartbeats while they run. This covers every job,
  also those started from the normal dashboard pages.
- A reaper fails jobs whose heartbeat is too old and runs the existing
  failure cleanup: it releases the quota reservation and, for a create job,
  stops and deletes the partially created VM.
- Reaped jobs appear as failed in *Admin -> Jobs*.
- **Timings:** jobs send a heartbeat every 10 s (`JOB_HEARTBEAT_SECONDS`). A
  job whose last heartbeat or update is older than 300 s
  (`JOB_STALE_AFTER_SECONDS`) is stale. Every replica runs the reaper every
  30 s (a quarter of the stale age when that is shorter). `SKIP LOCKED`
  makes sure that only one replica reaps a given job.
- **Error code:** a reaped job fails with `JOB_STALE`. Its message says that
  the worker stopped reporting progress and that the reserved resources were
  released.
- **Cleanup:** the quota reservation is released. For a create job, the VM
  is stopped and deleted when its VMID is known, and the instance becomes
  `failed`. After any other operation (start, stop, restart, delete) the
  instance becomes `unknown`, because its real power state is not known.
- **Late workers:** a worker that wakes up after its job was reaped writes
  nothing. It logs a `Worker:` line that ends in `job is no longer active`.

### 4.6 Proxmox token privileges

The orchestrator calls Proxmox through the existing services, with the same
API token as the rest of the control plane. The table maps each Phase 1 tool
to its Proxmox calls (read from the code, `backend/internal/proxmox/client.go`
and its callers) and to the privileges in the
[install guide, section 4](PROXMOX_INSTALL.md#4-proxmox-api-user-role-and-token).
The privileges are derived from that table and not verified on a live host.

| Tool | Proxmox API calls | PVE 8 | PVE 9 | ACL path |
|---|---|---|---|---|
| `vps.list`, `vps.get`, `vps.wait_ready` | none (Nevatal database; the job stores the IP address) | - | - | - |
| `vps.status` | VM status `GET .../status/current` | `VM.Audit` | `VM.Audit` | `/vms` |
| `vps.verify`, and the verification of VPS tasks | VM status; IP `GET .../agent/network-get-interfaces`; for snapshots `GET .../snapshot` | `VM.Audit`, `VM.Monitor` | `VM.Audit`, `VM.GuestAgent.Audit` | `/vms` |
| `vps.create` | `GET /cluster/nextid`, full clone, `POST .../config` (CPU, memory, cloud-init, guest agent), `PUT .../resize` (only for disks above 20 GB), `POST .../status/start`, task status, IP discovery. When it fails: `POST .../status/stop` and `DELETE .../qemu/{vmid}` (`purge=1`) | `VM.Clone` (template), `VM.Allocate`, `Datastore.AllocateSpace`, `SDN.Use`, `VM.Config.CPU`, `VM.Config.Memory`, `VM.Config.Cloudinit`, `VM.Config.Options`, `VM.Config.Disk`, `VM.PowerMgmt`, `VM.Monitor` | the same, with `VM.GuestAgent.Audit` instead of `VM.Monitor` | `/vms`, `/storage/{storage}`, `/sdn/zones/localnetwork/{bridge}`, `/nodes/{node}` |
| `vps.start`, `vps.restart` | `POST .../status/start` or `POST .../status/reboot`, task status, IP refresh through the guest agent | `VM.PowerMgmt`, `VM.Monitor` | `VM.PowerMgmt`, `VM.GuestAgent.Audit` | `/vms` |
| `vps.stop` | `POST .../status/shutdown` (clean shutdown), task status | `VM.PowerMgmt` | `VM.PowerMgmt` | `/vms` |
| `vps.resize` | `POST .../config` (CPU, memory), `PUT .../resize` (disk, only when it grows). No stop or start | `VM.Config.CPU`, `VM.Config.Memory`, `VM.Config.Disk`, `Datastore.AllocateSpace` | the same | `/vms`, `/storage/{storage}` |
| `vps.snapshot.create` | `POST .../snapshot`, task status | `VM.Snapshot` | `VM.Snapshot` | `/vms` |
| `vps.snapshot.restore` | `POST .../snapshot/{name}/rollback`, task status. Nevatal does not ask Proxmox to start the VM afterwards | `VM.Snapshot.Rollback` | `VM.Snapshot.Rollback` | `/vms` |
| `vps.delete` | `POST .../status/stop` (hard stop), then `DELETE .../qemu/{vmid}` (`purge=1`). Errors of both calls are ignored; the verification checks that the VM is gone | `VM.PowerMgmt`, `VM.Allocate` | `VM.PowerMgmt`, `VM.Allocate` | `/vms` |
| `quota.check`, `catalog.templates`, `catalog.plans` | none (Nevatal database) | - | - | - |
| `bucket.create`, `bucket.delete`, `app.create`, `n8n.create`, `openclaw.create`, `game_server.create`, `android.create` | none (Nevatal database and the storage volume) | - | - | - |
| Web console (no orchestrator tool) | `POST .../termproxy` + websocket | `VM.Console` | `VM.Console` | `/vms` |

- **Every privilege above is already in the `NevatalCloud` role** of the
  [install guide, section 4.1](PROXMOX_INSTALL.md#41-create-the-role)
  (PVE 8 and PVE 9 blocks). The VPS tools need nothing beyond it. Keep
  `VM.Console` for the dashboard's web console even though no orchestrator
  tool uses it.
- What the code shows:
  - `vps.resize` does not stop the VM; it needs a VM that is already stopped
    (section 2.1). `vps.delete` stops the VM hard before it deletes it
    (`VM.PowerMgmt`, already in the role).
  - Neither `quota.check` nor plan validation calls Proxmox. Node capacity
    comes from the Nevatal database (the online node, its CPU threads × 4 and
    its memory). The orchestrator needs no `Sys.Audit` or `Datastore.Audit`
    for it.
  - Verification runs no commands inside the guest. It only reads the guest
    agent's network interfaces, so the orchestrator does not need
    `VM.GuestAgent.Unrestricted` (the optional `NevatalGuestExec` role).
  - `bucket.create`, `bucket.delete`, `app.create`, `n8n.create`,
    `openclaw.create`, `game_server.create` and `android.create` do not call
    Proxmox.
- The ACLs from the
  [install guide, section 4.2](PROXMOX_INSTALL.md#42-create-the-user-the-token-and-the-acls)
  remain a second line of defence. The policy engine only accepts resources
  the user owns in Nevatal. Even so, with `NoAccess` on the infrastructure VMs
  (e.g. VMID `150`) and clone-only templates (`9000-9003`), Proxmox itself
  refuses to stop, change or delete them.
- Upgrading the host from PVE 8 to 9: re-add guest-agent access as in
  section 4.1 of the install guide. Without it, IP discovery fails with
  `403 Permission check failed`.

### 4.7 Live events through proxies (SSE)

The timeline streams `GET /api/v1/ai/executions/:id/events` with
`Accept: text/event-stream` and a Bearer header (clients use `fetch`
streaming, not `EventSource`). Frames carry `id: <seq>`, `event: <type>` and
`data: <event JSON>`. A `: ping` comment is sent every **20 s**. A replica
streams the events of every replica: it wakes at once for its own events and
reads the database every 1 s for the others. The stream closes after
`execution.finished` and when the server shuts down. Clients
resume with `Last-Event-ID` or `?after=<seq>`, and fall back to polling the
JSON form every 2 s.

Every proxy on the way must pass the stream through without buffering or
compressing it, and keep an idle connection open for longer than 20 s.

| Hop | What it needs | In this repository |
|---|---|---|
| Bundled nginx (`frontend` container) | `proxy_buffering off`, a long `proxy_read_timeout`, HTTP/1.1 to the API, no gzip for `text/event-stream` | Already so: `proxy_buffering off` and `proxy_read_timeout 86400s` in `frontend/nginx/snippets/proxy-api.conf`; `gzip_types` in `default.conf.template` does not list `text/event-stream`. Nothing to do. |
| Your own nginx in front | `proxy_buffering off;` in the `/api/` location, `proxy_http_version 1.1;`, `proxy_read_timeout` above 20 s (the 60 s default is enough), no gzip for `text/event-stream`. The API sends `X-Accel-Buffering: no` and `Cache-Control: no-cache` on the stream, so nginx does not buffer it even where `proxy_buffering` is on, unless `proxy_ignore_headers` lists `X-Accel-Buffering`. Turn buffering off anyway. | Your configuration |
| Caddy (compose profile `tls`) | `reverse_proxy` flushes `text/event-stream` responses at once by default | Nothing to do (not verified here) |
| HAProxy (topology B) | Idle timeouts above 20 s | `timeout client 120s`, `timeout server 300s` in `deploy/loadbalancer/haproxy/haproxy.cfg`. Nothing to do. |
| Traefik (k3s) | Streams responses | Nothing to do (not verified here) |
| Cloudflare (proxied DNS or Tunnel) | Closes a proxied connection that stays silent for about 100 s (error `524` before the first byte, [install guide, section 20.9](PROXMOX_INSTALL.md#209-cloudflare-limits-that-affect-customer-apps)). The 20 s heartbeat keeps the stream open. Cloudflare also drops long-lived connections now and then; clients reconnect with `Last-Event-ID`. | Nothing to do |

Test it hop by hop (`-N` turns off curl's own buffering):

```bash
API=https://cloud.nevatal.id/api/v1          # then http://127.0.0.1:3080/api/v1 on the VM
TOKEN=$(curl -s -X POST "$API/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"..."}' | jq -r .data.token)

curl -sN -H "Authorization: Bearer $TOKEN" -H 'Accept: text/event-stream' \
  "$API/ai/executions/<execution-id>/events"
# id: 1
# event: execution.created
# data: {"seq":1,"type":"execution.created", ...}
#
# : ping      <- every 20 s while nothing else happens
```

Each event should print when it happens. Lines that arrive in bursts, or only
at the end, mean a hop buffers the stream. If the stream is live through
`127.0.0.1:3080` but not through the public name, the buffering hop sits in
front of the control-plane VM.

### 4.8 Monitoring

Phase 1 has no metrics and no metrics endpoint. Use the API logs
(`docker compose logs backend`) and the database. Log lines:

| Log line | Meaning |
|---|---|
| `AI orchestrator engine started (owner …, concurrency …, lease …, heartbeat …)` | Startup: the engine of this replica runs, with these settings. |
| `Provisioning jobs: heartbeat every …, failed as JOB_STALE after … without one` | Startup: job heartbeat settings (section 4.5). |
| `AI orchestrator: wrote revision 1 of N existing plan(s)` | First startup after the upgrade (section 4.1). |
| `Orchestrator: lost the lease of task …` | A task's lease expired while it ran (database or network stall), and another runner took it over. Alert if it repeats. |
| `Orchestrator: shutdown interrupted N running task(s); released N lease(s) for re-claim` | A graceful stop with tasks still running after 50 s (section 4.3). |
| `Job reaper: failed N stale provisioning job(s) as JOB_STALE`, and one `Job reaper: job … failed as JOB_STALE` per job | A replica died in the middle of a job. Alert. |
| A `Worker:` line ending in `job is no longer active` | A worker finished a job that the reaper had already failed; its result was dropped. |
| `Warning: provisioning jobs were still running at shutdown and were interrupted; …` | A stop interrupted jobs. Expect `JOB_STALE` reaps on other replicas. |
| `Orchestrator: claim failed: …`, `Orchestrator: maintenance scan: …`, `Orchestrator: heartbeat of task … failed: …` | Database errors in the engine. Alert. |
| `Config warning: ORCH_HEARTBEAT_SECONDS must be below half of ORCH_LEASE_SECONDS; …` | A setting was corrected at startup (section 4.1). |

Watch these signals:

| Signal | Why it matters |
|---|---|
| Executions in `running` or `verifying`, and their age | A stuck engine, or Proxmox being slow |
| Tasks in `retrying`, attempts per task | Proxmox or network errors |
| Tasks re-claimed after lease expiry | Replicas crashing or being killed (expect none outside deploys) |
| Jobs failed by the reaper | Replicas dying in the middle of a job |
| `failed` and `partially_completed` executions, by `error.code` | Capacity, quota or template problems |
| Verification results in production | `unverified` for a VPS create, start, restart, stop or delete means the platform could not check; expect `verified` there. Resize, snapshot restore and the non-VPS tools are always `unverified` in Phase 1 (section 2.6) |
| `429 TOO_MANY_EXECUTIONS`, `409 APPROVAL_*`, `409 PLAN_VERSION_MISMATCH` | Users at the limit, stale clients |
| Open event streams and reconnects | A proxy that buffers or cuts the stream |

Available today: `docker compose logs backend`, *Admin -> Jobs*, and the
user's own `GET /api/v1/ai/executions?status=running`. Phase 1 has no admin
API or dashboard view of all tenants' executions. Use SQL on the tables
`ai_executions`, `ai_execution_tasks` and `ai_execution_events`, for example
active executions with their retrying tasks and re-claims
(`docker compose exec postgres psql -U nevatal nevatal_cloud` with the
default names):

```sql
SELECT e.id, e.user_id, e.status, e.created_at,
       count(*) FILTER (WHERE t.status = 'retrying') AS retrying_tasks,
       max(t.claim_count) AS max_claims
FROM ai_executions e
JOIN ai_execution_tasks t ON t.execution_id = e.id
WHERE e.status IN ('queued', 'running', 'verifying', 'cancelling')
GROUP BY e.id
ORDER BY e.created_at;
```

### 4.9 Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Execution stays `queued` or `running`; no new events for minutes | A task waits for its next attempt (`retrying`, `next_attempt_at`); a replica was killed and the task waits for its lease to expire; no API replica is running; a slow Proxmox operation (large clone) | `GET /api/v1/ai/executions/<id>`: check each task's `status`, `attempt`, `next_attempt_at` and `error`. `docker compose ps`, `docker compose logs backend`, the VM's task log in Proxmox. Cancel the execution if it should not continue. |
| Execution stays `cancelling` | A running task is interrupted at once on its own replica and at its next heartbeat (10 s) on another one. A hypervisor call in progress (resize, snapshot) is not aborted and finishes first (up to 2 min). If the task's replica died, the task is cancelled once its lease expires (30 s, plus up to 5 s for the maintenance scan). | Wait; follow the task's events. If it lasts longer, check that an API replica is running. |
| `409 APPROVAL_EXPIRED` | The approval was not used within 60 min (`medium`, `high`) or 15 min (`destructive`) | Review the plan and approve again. |
| `409 APPROVAL_SUPERSEDED` | The plan was edited after the approval | Review the new revision and approve it. |
| `409 APPROVAL_CONSUMED` | The approval already started an execution (double click, second tab, repeated `/execute`) | Open that execution (`latest_execution_id`). Approve again only if you want a second run. |
| `409 APPROVAL_REQUIRED` | `/execute` without a valid approval (none, wrong `approval_id`) | Approve first. |
| `409 PLAN_VERSION_MISMATCH` | The approval names a revision that is no longer current (plan edited, stale page) | Reload, check what changed, approve the current revision. |
| `409 PLAN_VERSION_CONFLICT` | `PATCH` with an old `base_version` | Reload and edit again. |
| `409 PLAN_NOT_EDITABLE` | The plan is executing or finished. An approved plan that has not started can still be edited. | Ask for a new plan. |
| `409 EXECUTION_ACTIVE` | The plan already has an active execution | Follow that execution. |
| `409 EXECUTION_NOT_ACTIVE` | Cancel of an execution that has already finished | Nothing to cancel; read its report. |
| `409 TASK_NOT_RETRYABLE` | Retry of a task that is not `failed`, or *Retry failed* without a failed task | Retry only failed tasks. |
| `409 RECOVERY_NOT_AVAILABLE` | *Keep* on an execution that is not `partially_completed`; *Delete created* on one that is neither `failed` nor `partially_completed`, or that created no VPS or bucket that still exists | Use one of the execution's `recovery_options`. |
| Task error `LEASE_EXHAUSTED` | The attempt was interrupted 5 times: its replica crashed, was killed or was stopped each time | Find out why replicas restart (OOM, crash loop, deploys during long tasks), then retry the task. |
| Task error `JOB_STALE` | The provisioning job's worker stopped (its replica died or was stopped in the middle of the job); the reaper failed the job | Retried automatically. Frequent ones mean replicas die during jobs. |
| Task error `TOOL_TIMEOUT` | The attempt took longer than the tool's `timeout_seconds` (section 2.5) | Retried, except for `vps.snapshot.restore` (not idempotent: retry it manually after checking the VM). Check Proxmox for slow clones or hung tasks. |
| Task error `INTERRUPTED_NOT_IDEMPOTENT` | A replica died or stopped while a non-idempotent task (`vps.snapshot.restore`) ran; the change may already be applied | Check the VM (was it rolled back?), then retry the task manually if needed. |
| Task error `REQUEST_REJECTED` on `vps.resize` | The VM was not stopped when the resize ran | Stop the VM (a separate plan), then retry the task. |
| `422 VALIDATION_FAILED` on approve or execute | Policy failed at that moment (quota used up in the meantime, template or size no longer available); `details.errors` says why | Fix the cause, `POST /api/v1/ai/plans/<id>/validate`, approve again. |
| `422 RISK_ACK_REQUIRED` | High or destructive plan without `acknowledge_risks: true` | Acknowledge the risks. |
| `422 CONFIRMATION_REQUIRED` / `CONFIRMATION_MISMATCH` | Destructive plan without the typed names, or with wrong ones | Type exactly the names in `confirmation_required.expected`: case-sensitive, comma-separated. |
| `403 ADMIN_REQUIRED` | The plan contains an admin-only tool | Run it as an admin, or remove that step. |
| `429 TOO_MANY_EXECUTIONS` | The user already has 3 active executions | Wait for one to finish, or cancel one. |
| Verification `unverified` in production | Expected for `vps.resize`, `vps.snapshot.restore` and the app, n8n, OpenClaw, game server and Android tools (Phase 1 always skips one of their checks, section 2.6). For other tools: `PROXMOX_MOCK=true` (`/api/v1/system/config` shows `"proxmox_mock":true`); guest agent missing or not running in the VM; token without `VM.GuestAgent.Audit` (PVE 9) / `VM.Monitor` (PVE 8); Proxmox unreachable (`595`) | Production settings and the agent: [install guide, section 21](PROXMOX_INSTALL.md#21-troubleshooting). Test with `qm guest cmd <vmid> ping`. |
| Verification `failed` | The real state differs (VM not running, no IP address). The execution ends `partially_completed` although no task failed | Check the VM in Proxmox and fix it, or delete the resource with *Delete created* or a new plan. *Retry failed* is not offered. |
| Timeline only updates after a reload, or in bursts | A proxy buffers or compresses `text/event-stream` | Section 4.7: no buffering, no gzip; test with `curl -N` hop by hop. |
| Event stream reconnects every 60-100 s | A proxy holds back the 20 s heartbeat (buffering), so the connection looks idle and is closed | Turn off buffering; keep read timeouts above 20 s. |
| Own client gets `401` on the event stream | No `Authorization: Bearer` header | Send the header; use `fetch` streaming, not `EventSource` (contract section 5). |

### 4.10 Phase 2–4 scheduling and settings

**Checks.** Every API replica starts a monitor loop (log `Infrastructure
checks: monitor loop every …`). On startup and every
`MONITOR_INTERVAL_SECONDS`, each replica claims up to 20 due checks with
`FOR UPDATE SKIP LOCKED`, then advances `next_run_at` by the check's
`interval_seconds`. There is no leader election and no separate worker: run
the same number of replicas as for Phase 1, and keep their clocks and database
connection healthy. A replica that dies during a check leaves it due at its
already-advanced `next_run_at`; another replica picks it up then. Check
results keep only the latest observation and consecutive failure count, not a
history table. `POST /ai/checks/:id/run` bypasses the schedule for one check.

**Automatic database backups.** Every API replica starts the backup loop (log
`Managed databases: automatic backup cadence …`). On startup and every minute
it claims up to 20 due databases (`auto_backup = true`, status `running`,
`next_backup_at` reached) with `FOR UPDATE SKIP LOCKED`, advances
`next_backup_at` by the cadence and records a normal logical backup. A failed
backup is logged and retried at the next cadence; it never changes the
database status. No leader election is needed for the same reason as checks:
the claim is atomic, so replicas do not duplicate a backup.

**Notification retention.** With `ORCH_NOTIFICATION_RETENTION_DAYS` above zero
(default 30), each replica deletes notifications older than the window at
startup and then hourly. Pruning is best effort and removes only notification
rows: execution events, plans and audit records are kept. A failed deletion is
logged and retried on the next pass.

**Notifications and policies.** Notifications are inserted with a partial
unique index on `(user_id, dedupe_key)` where `dedupe_key <> ''`, so a repeated
finding for the same check/status or execution/status is stored once. A check
notifies on a status transition, while `check.failed` policies are evaluated
on every warning or critical run. Policy evaluation and the terminal-execution
notification run after the producing transaction commits, in a short-lived
goroutine with a 20 s timeout; a failure there is logged by its effect only
(it never changes the execution or check result). Policies cannot execute:
`notify` stores a notification, and `create_plan` deploys a template into a
plan that still needs approval. `advisor.recommendation` is evaluated when an
advisor report is generated, and those policies are limited to `notify`.

**Advisor rates.** The advisor is pure calculation over current records. Set
`ORCH_COST_*` to your own monthly rate card; estimates use the configured
currency label and are not invoices. Changing rates changes the next report,
not any stored resource. Stopped or suspended compute is estimated at
disk-only cost, and buckets by stored bytes. No recommendation executes by
itself.

**Secrets.** Managed database passwords are encrypted with AES-256-GCM using a
key derived from `JWT_SECRET`. Keep that secret stable for the life of the
database records. Rotating it (or restoring a database from a backup taken
with another secret) makes existing passwords undecryptable; the credentials
endpoint then answers `503 CREDENTIALS_UNAVAILABLE`. Creating a new database
after rotation works. Template and plan inputs reject secret-looking values,
so do not expect a template to carry a database password.

**Known Phase 2–4 limits.** The managed database service is logical: no
container or VM is provisioned for it, and there is no live connection, query
or runtime probe. An automatic backup is a control-plane record with a
`backup_reference`, not a dump copied from a server. Logs and zero/unmeasured
database metrics are simulated and must not be used for capacity decisions.
Advisor metrics are only as good as the platform's available metrics. The
advisor notifies only when a report is generated; no loop watches
recommendations in the background. Retention deletes old notification rows
without archiving them, and check results keep only the latest observation
(no history table). App health probing is limited to platform-assigned
`*.apps.nevatal.id` names and is skipped in mock mode.



Base `/api/v1`. Bearer token (the cookie works for GET only). Success
`{"data": ...}`, errors `{"error": {"code", "message", "details"}}`. The
complete contract with all fields is [ORCHESTRATOR_API.md](ORCHESTRATOR_API.md).

| Need | Call | Contract |
|---|---|---|
| Plan states, execution states, task states, risk levels | - | [section 1](ORCHESTRATOR_API.md#1-lifecycles) |
| Plan with its current revision, validation, approval | `GET /ai/plans/:id` | [section 2](ORCHESTRATOR_API.md#2-plan-extends-the-existing-plan-object) |
| Revision history; one revision | `GET /ai/plans/:id/revisions`, `GET /ai/plans/:id/revisions/:version` | [section 3](ORCHESTRATOR_API.md#3-endpoints) |
| Edit a plan | `PATCH /ai/plans/:id` `{base_version, specification}` | section 3 |
| Validate again | `POST /ai/plans/:id/validate` (answers `{plan_id, status, valid, validation}`) | section 3 |
| Approve (and start) | `POST /ai/plans/:id/approve` `{version, spec_hash, acknowledge_risks, confirmation, execute}` | section 3 |
| Start a saved approval | `POST /ai/plans/:id/execute` `{approval_id}` | section 3 |
| Reject; cancel | `POST /ai/plans/:id/reject` `{reason?}`; `POST /ai/plans/:id/cancel` | section 3 |
| List and read executions | `GET /ai/executions?conversation_id=&plan_id=&status=`; `GET /ai/executions/:id` | [section 4](ORCHESTRATOR_API.md#4-execution-object) |
| Events (JSON or SSE) | `GET /ai/executions/:id/events?after=<seq>&limit=200` | [section 5](ORCHESTRATOR_API.md#5-events) |
| Cancel an execution | `POST /ai/executions/:id/cancel` | section 3 |
| Retry one failed task | `POST /ai/executions/:id/tasks/:taskId/retry` | section 3 |
| Recovery | `POST /ai/executions/:id/recovery` `{action}`: `retry_failed` answers `202` with the execution, `diagnose` `200` with `{diagnostics}`, `keep` `200` with the execution, `delete_created` or `compensate` `201` with `{plan_id}` | section 3 |
| Tool registry | `GET /ai/tools` | [section 6](ORCHESTRATOR_API.md#6-tools-registry-also-served-to-mcp-toolslist) |
| MCP | `POST /mcp/rpc` (`initialize`, `ping`, `tools/list`, `tools/call`; setup in section 2.9) | section 6 |
| Error codes | - | [section 7](ORCHESTRATOR_API.md#7-error-codes); the implementation adds `409 EXECUTION_NOT_ACTIVE` and `409 RECOVERY_NOT_AVAILABLE` (section 4.9) |
| Engine guarantees | - | [section 8](ORCHESTRATOR_API.md#8-backend-guarantees-not-visible-in-json-required) |

Approving from a script (`API` and `TOKEN` as in section 4.7):

```bash
PLAN=<plan-id>
curl -s -H "Authorization: Bearer $TOKEN" "$API/ai/plans/$PLAN" \
  | jq '.data.revision | {version, spec_hash, risk_level, confirmation_required}'

curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"version":3,"spec_hash":"sha256:9f2c...","acknowledge_risks":false,"execute":true}' \
  "$API/ai/plans/$PLAN/approve" | jq '.data.execution.id'
```

---

## 6. Roadmap

| Phase | Scope |
|---|---|
| **1 (this guide)** | Versioned plans, policy validation, version-bound approvals, durable task engine, verification, live events, recovery options, one tool registry for the dashboard and MCP |
| **2** | Multi-service templates and relationships between resources (for example an app planned, ordered and verified together with the VPS and domain it runs on) |
| **3** | Diagnostics, monitoring and notifications |
| **4** | Optimization advisor, specialized agent roles, topology view |

**Backups** are a separate feature (P2): platform-wide automated backup and
recovery with Proxmox `vzdump`, schedules and retention, restore in place or
as a new VM, and backups of the control-plane database. Build order:
Orchestrator Phase 1, then backups, then Orchestrator Phases 2-4. Backup
operations are not orchestrator tools in Phase 1.
