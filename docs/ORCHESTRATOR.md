# AI Infrastructure Orchestrator (Phase 1)

The AI Infrastructure Orchestrator is the engine behind the **AI Cloud Agent**.
You describe the infrastructure you want in plain language. The platform turns
it into a versioned plan, checks it, waits for **your** approval of exactly
that plan, executes it as durable tasks, verifies the result on the hypervisor
and reports what it built.

This guide describes Phase 1 from its frozen API contract,
[ORCHESTRATOR_API.md](ORCHESTRATOR_API.md) (2026-11-02). Details that only the
implementation decides are marked **TBD (confirm after implementation)**.

| You are | Read |
|---|---|
| A user (dashboard or MCP client) | sections 1, 2 and 3 |
| The operator of a Nevatal installation | sections 3 and 4; the installation itself is in [PROXMOX_INSTALL.md](PROXMOX_INSTALL.md) |
| Writing your own client | section 5 and the contract |

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
  plan and approval panel, live execution timeline.
- **REST API:** `/api/v1/ai/*` (section 5).
- **MCP:** `POST /api/v1/mcp/rpc` for Claude Desktop, Cursor and other MCP
  clients (section 2.9).

Not in Phase 1 (section 6): templates of several services that depend on each
other, diagnostics, monitoring and notifications, optimization advice,
backups.

---

## 2. For users

### 2.1 Asking for infrastructure

Open *AI Cloud Agent* and write what you want. Name the resource, give the
sizes and the OS, and mention your SSH key. If something is missing, the agent
asks before it proposes a plan.

| Resource | Example request | Tool | Risk |
|---|---|---|---|
| VPS: create | *Create an Ubuntu 24.04 VPS named web-01 with 2 vCPU, 4 GB RAM, 40 GB disk and my SSH key* | `vps.create` | medium |
| VPS: start | *Start web-01* | `vps.start` | medium |
| VPS: restart | *Restart web-01* | `vps.restart` | medium |
| VPS: stop | *Stop web-01* | `vps.stop` | high |
| VPS: resize | *Resize web-01 to 4 vCPU and 8 GB RAM* | `vps.resize` | high |
| VPS: snapshot | *Take a snapshot of web-01 called before-upgrade* | `vps.snapshot.create` | medium |
| VPS: restore a snapshot | *Roll web-01 back to the snapshot before-upgrade* | `vps.snapshot.restore` | destructive |
| VPS: delete | *Delete web-01* | `vps.delete` | destructive |
| Bucket: create | *Create a storage bucket called assets* | `bucket.create` | medium |
| Bucket: delete | *Delete the bucket assets* | `bucket.delete` | destructive |
| App | *Deploy the Node.js app from https://github.com/example/shop, branch main, port 3000, as shop* | `app.create` | medium |
| n8n | *Set up an n8n instance called ops-flows with 2 GB RAM* | `n8n.create` | medium |
| OpenClaw | *Create an OpenClaw agent called support-bot that uses Anthropic* | `openclaw.create` | medium |
| Game server | *Create a Minecraft Java server called survival with 10 player slots* | `game_server.create` | medium |
| Android | *Create an Android instance called qa-phone* | `android.create` | medium |

Questions that only read run without any approval:

| Question | Tools |
|---|---|
| *List my servers* / *Is web-01 running?* | `vps.list`, `vps.get`, `vps.status` |
| *How much quota do I have left?* | `quota.check` |
| *Which OS templates and plans can I use?* | `catalog.templates`, `catalog.plans` |

Good to know:

- **Resize** needs a stopped VM, and Proxmox can only grow a disk, never
  shrink it. Whether the plan adds the stop and start tasks for you:
  TBD (confirm after implementation).
- **Snapshot restore** rolls the VM back and discards everything written
  after the snapshot. That is why it is `destructive`.
- **Several resources** can be part of one plan (*Create two VPS, web-01 and
  web-02*); each gets its own resource entry and tasks. Services wired to each
  other (an app plus the VPS and domain it runs on) come in Phase 2.
- **Never paste API keys or passwords into the chat.** Chat messages are kept
  in the conversation history. Plans, events and reports never contain
  secrets (section 3). How `openclaw.create` receives the AI provider key:
  TBD (confirm after implementation).
- Plans that contain only `read` and `low` tasks need no approval
  (`requires_approval: false`). Whether the agent runs them at once or shows
  them as a plan first: TBD (confirm after implementation).

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

| Risk | Phase 1 tools | To approve, you... | Approval valid for |
|---|---|---|---|
| `read` | `vps.list`, `vps.get`, `vps.status`, `quota.check`, `catalog.templates`, `catalog.plans`, `vps.wait_ready`, `vps.verify` | nothing: no approval needed | - |
| `low` | none (planning itself, inspecting configuration) | nothing | - |
| `medium` | `vps.create`, `vps.start`, `vps.restart`, `vps.snapshot.create`, `bucket.create`, `app.create`, `n8n.create`, `openclaw.create`, `game_server.create`, `android.create` | **approve** the revision you see | 60 min |
| `high` | `vps.stop`, `vps.resize` | approve **and acknowledge the risks** | 60 min |
| `destructive` | `vps.delete`, `vps.snapshot.restore`, `bucket.delete` | approve, acknowledge the risks **and type the exact name(s)** of the target(s) | 15 min |

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
  (`requires_admin`). Which Phase 1 tools are admin-only:
  TBD (confirm after implementation).

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
  (`409 PLAN_NOT_EDITABLE`): ask for a new plan. Whether a plan approved with
  `"execute": false` (approved, not started yet) can still be edited, or must
  be rejected or cancelled first: TBD (confirm after implementation).
- Whether a follow-up chat message (*make it 8 GB*) revises the open plan or
  creates a new plan: TBD (confirm after implementation).

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
| `compensating` | Cleaning up after a failure. Which tools compensate, and how: TBD (confirm after implementation). |

**Retries.** A failed attempt is retried by itself after a pause, up to the
task's `max_attempts`. The number of attempts and the pauses per tool:
TBD (confirm after implementation). Each attempt has its own idempotency key,
so a retry never creates a second copy of a resource (section 4.4).

**Cancel.** `POST /api/v1/ai/executions/:id/cancel` (or cancel in the
workspace) moves the execution to `cancelling`, then `cancelled`. Tasks that
have not started end as `cancelled`. How a task that is already running ends
(it finishes its current step, or is interrupted):
TBD (confirm after implementation). Nothing is deleted automatically:
resources that were already created stay. Remove them with a new plan
(*Delete web-01*).

### 2.6 Verification results

After the tasks finish, the execution enters `verifying`. The verification
engine compares the real state with the plan: for a VPS, that it exists, is
running and has an IP address (read from Proxmox and the QEMU guest agent).
Each task's `verification` lists its checks (`pass`, `fail`, `skip`) with
details. The full list of checks per tool: TBD (confirm after implementation).

| Result | Meaning | What to do |
|---|---|---|
| `verified` | Every check passed against the real hypervisor. | Nothing. |
| `unverified` | The checks could not confirm the state: the platform runs the simulated hypervisor, or a check could not reach its target (for example, the guest agent does not answer). | The resource may well be fine: check it yourself (VM page, SSH). In production this points to a setup problem (section 4.9). |
| `failed` | A check found a difference, for example the VM is not running or has no IP address. | Read the failed check. Retry, or delete the resource with a new plan. |

**The simulated hypervisor never shows `verified`.** In demo mode and with
`PROXMOX_MOCK=true` every result is `unverified`, because nothing real was
checked. How a `failed` verification changes the execution's final status:
TBD (confirm after implementation).

### 2.7 Partial failures and recovery

The orchestrator **never deletes what it created on its own**. When some
tasks succeed and others fail, the execution ends `partially_completed`, keeps
everything that worked and offers recovery options.

```
Plan v1: create VPS web-01 and web-02              execution: partially_completed

  quota.check             succeeded
  vps.create:web-01       succeeded   web-01 running, ssh ubuntu@10.0.0.5
  vps.wait_ready:web-01   succeeded
  vps.create:web-02       failed      error code + message, no attempts left
  vps.wait_ready:web-02   skipped     (waits for vps.create:web-02)

Recovery:   Retry failed  |  Keep  |  Delete created
```

| Option | `action` | What happens |
|---|---|---|
| Retry failed | `retry_failed` | Re-queues the failed tasks and the tasks skipped because of them. Same as retrying each failed task (`POST /api/v1/ai/executions/:id/tasks/:taskId/retry`; only `failed` tasks, otherwise `409 TASK_NOT_RETRYABLE`). |
| Keep | `keep` | Accepts the partial result. The execution stays `partially_completed`; the created resources stay and count against your quota. |
| Delete created | `delete_created` | Creates a **new destructive plan** that removes the resources this execution created. You review and approve it like any destructive plan (typed names, 15 min). Nothing is deleted before that. |

Each option says whether it needs a new approval (`requires_approval`).
Options appear on `failed` and `partially_completed` executions.

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
| `read` (`vps.list`, `vps.get`, `vps.status`, `quota.check`, `catalog.templates`, `catalog.plans`, `vps.wait_ready`, `vps.verify`) | Runs at once and returns the result. |
| `medium`, `high`, `destructive` (every other tool) | **Never executes.** Creates a plan and returns it for approval in the Nevatal dashboard. |

The answer to a write call:

```json
{ "plan_id": "…", "version": 1, "status": "awaiting_approval", "risk_level": "destructive" }
```

An assistant in Claude Desktop or Cursor can therefore look around freely and
*propose* changes, but only you approve them, in the dashboard, with the same
rules as section 2.3 (typed names for destructive plans). There is no approve
tool, and nothing the assistant says or sends counts as an approval.

Connecting a client (transport, which token to use, the configuration file
for Claude Desktop and Cursor): TBD (confirm after implementation).

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
  created before the upgrade get revision 1. Take a database backup first.
- **Demo mode** runs one API replica with the simulated hypervisor; every
  verification is `unverified`.
- **Fixed by the contract:** 3 active executions per user, approval expiry
  60 min / 15 min, SSE heartbeat every 20 s, polling fallback every 2 s.
- **Settings** (environment variables for lease timings, retries, limits):
  TBD (confirm after implementation).
- Per-tool timeouts are declared in the registry: `timeout_seconds` in
  `GET /api/v1/ai/tools`.

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
| A replica crashes or is killed (`docker kill`, OOM, VM reset) | Its leases are no longer renewed and expire. Another replica (or the same one after its restart) re-claims the task and **re-drives the same attempt** with the same idempotency key: it continues with the job or resource that attempt already started, instead of starting a new one (section 4.4). |
| Graceful stop (`SIGTERM`: `./deploy.sh`, `docker compose stop`, rolling update) | The replica stops claiming and releases its unfinished leases, so other replicas take over at once instead of waiting for expiry (section 4.3). |
| The whole stack restarts (Docker Compose restarts all replicas together) | Executions pause and continue when the API is back. |

Lease duration, heartbeat interval and how often idle replicas look for
work: TBD (confirm after implementation).

This changes the behaviour described in the
[install guide, section 16](PROXMOX_INSTALL.md#16-upgrades-and-rollback),
where a replica killed in the middle of a job leaves that job failed. The
provisioning job itself still runs inside one replica. If it dies with that
replica, the reaper fails it and cleans up (section 4.5). The orchestrator
task then continues with its next attempt, if it has attempts left.

### 4.3 Graceful shutdown (SIGTERM)

When a replica receives `SIGTERM`:

1. it stops claiming tasks;
2. it releases the leases of its unfinished tasks, so another replica
   continues them;
3. it closes its open event streams. Clients reconnect, usually to another
   replica, and resume after their last event (`Last-Event-ID`);
4. it drains HTTP requests and running jobs as before (existing values in
   `backend/cmd/api/main.go`: 20 s for HTTP, 40 s for jobs).

The order of these steps, and how long a running task keeps working before
its lease is released: TBD (confirm after implementation).

Grace periods already set in this repository: Docker Compose
`stop_grace_period: 60s` for `backend`; Kubernetes a 20 s `preStop` sleep and
`terminationGracePeriodSeconds: 90`. Stop the API with `SIGTERM`
(`docker compose stop`, `./deploy.sh`, `kubectl rollout restart`). After a
`SIGKILL` its tasks wait until their leases expire.

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
  before it creates the index. How (which jobs keep their key):
  TBD (confirm after implementation).

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

How exactly a re-claimed attempt resumes a job or waits for the reaper:
TBD (confirm after implementation).

### 4.5 Stale-job reaper

- Provisioning jobs send heartbeats while they run. This covers every job,
  also those started from the normal dashboard pages.
- A reaper fails jobs whose heartbeat is too old and runs the existing
  failure cleanup: it releases the quota reservation and, for a create job,
  stops and deletes the partially created VM.
- Reaped jobs appear as failed in *Admin -> Jobs*.
- Heartbeat interval, the age after which a job counts as stale, the error
  code shown for reaped jobs, and the state an instance gets after a reaped
  non-create operation: TBD (confirm after implementation).

### 4.6 Proxmox token privileges

The orchestrator calls Proxmox through the existing services, with the same
API token as the rest of the control plane. The table maps each Phase 1 tool
to the calls and privileges in the
[install guide, section 4](PROXMOX_INSTALL.md#4-proxmox-api-user-role-and-token).
It is derived from that table and not verified on a live host.

| Tool | Proxmox API calls | PVE 8 | PVE 9 | ACL path |
|---|---|---|---|---|
| `vps.list`, `vps.get`, `vps.status` | VM status `GET .../status/current` | `VM.Audit` | `VM.Audit` | `/vms` |
| `vps.wait_ready`, `vps.verify` | VM status; IP discovery `GET .../agent/network-get-interfaces` | `VM.Audit`, `VM.Monitor` | `VM.Audit`, `VM.GuestAgent.Audit` | `/vms` |
| `vps.create` | `GET /cluster/nextid`, full clone, `POST .../config`, `PUT .../resize`, start, task status, IP discovery | `VM.Clone` (template), `VM.Allocate`, `Datastore.AllocateSpace`, `SDN.Use`, `VM.Config.CPU`, `VM.Config.Memory`, `VM.Config.Cloudinit`, `VM.Config.Options`, `VM.Config.Disk`, `VM.PowerMgmt`, `VM.Monitor` | the same, with `VM.GuestAgent.Audit` instead of `VM.Monitor` | `/vms`, `/storage/{storage}`, `/sdn/zones/localnetwork/{bridge}`, `/nodes/{node}` |
| `vps.start`, `vps.stop`, `vps.restart` | `POST .../status/*` | `VM.PowerMgmt` | `VM.PowerMgmt` | `/vms` |
| `vps.resize` | `POST .../config` (CPU, memory), `PUT .../resize` (disk) | `VM.Config.CPU`, `VM.Config.Memory`, `VM.Config.Disk`, `Datastore.AllocateSpace` | the same | `/vms`, `/storage/{storage}` |
| `vps.snapshot.create` | snapshot create | `VM.Snapshot` | `VM.Snapshot` | `/vms` |
| `vps.snapshot.restore` | snapshot rollback | `VM.Snapshot.Rollback` | `VM.Snapshot.Rollback`; 9.2+ also checks `VM.PowerMgmt` when the VM starts after a rollback | `/vms` |
| `vps.delete` | `DELETE .../qemu/{vmid}` (`purge=1`) | `VM.Allocate` | `VM.Allocate` | `/vms` |
| `quota.check`, `catalog.templates`, `catalog.plans` | none expected (Nevatal database) | - | - | - |
| Web console (no orchestrator tool) | `POST .../termproxy` + websocket | `VM.Console` | `VM.Console` | `/vms` |

- **Every privilege above is already in the `NevatalCloud` role** of the
  [install guide, section 4.1](PROXMOX_INSTALL.md#41-create-the-role)
  (PVE 8 and PVE 9 blocks). The VPS tools need nothing beyond it. Keep
  `VM.Console` for the dashboard's web console even though no orchestrator
  tool uses it.
- The exact Proxmox calls of each tool: TBD (confirm after implementation).
  In particular:
  - whether `vps.resize` and `vps.delete` stop the VM first (`VM.PowerMgmt`,
    already in the role);
  - whether `quota.check` or plan validation read node capacity
    (`Sys.Audit` on `/nodes/{node}`, `Datastore.Audit` on
    `/storage/{storage}`, already in the role);
  - whether verification runs commands inside the guest through the QEMU
    guest agent (for example app checks that reuse the Docker viewer). That
    would need `VM.GuestAgent.Unrestricted` (the optional `NevatalGuestExec`
    role) on PVE 9, and `VM.Monitor` on PVE 8;
  - whether `bucket.create`, `bucket.delete`, `app.create`, `n8n.create`,
    `openclaw.create`, `game_server.create` and `android.create` call
    Proxmox at all.
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
`data: <event JSON>`. A `: ping` comment is sent every **20 s**. The stream
closes after `execution.finished` and when the server shuts down. Clients
resume with `Last-Event-ID` or `?after=<seq>`, and fall back to polling the
JSON form every 2 s.

Every proxy on the way must pass the stream through without buffering or
compressing it, and keep an idle connection open for longer than 20 s.

| Hop | What it needs | In this repository |
|---|---|---|
| Bundled nginx (`frontend` container) | `proxy_buffering off`, a long `proxy_read_timeout`, HTTP/1.1 to the API, no gzip for `text/event-stream` | Already so: `proxy_buffering off` and `proxy_read_timeout 86400s` in `frontend/nginx/snippets/proxy-api.conf`; `gzip_types` in `default.conf.template` does not list `text/event-stream`. Nothing to do. |
| Your own nginx in front | `proxy_buffering off;` in the `/api/` location, `proxy_http_version 1.1;`, `proxy_read_timeout` above 20 s (the 60 s default is enough), no gzip for `text/event-stream`. Whether the API also sends `X-Accel-Buffering: no`: TBD (confirm after implementation). | Your configuration |
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

Metric names, a metrics endpoint and the log lines to alert on:
TBD (confirm after implementation). Watch these signals:

| Signal | Why it matters |
|---|---|
| Executions in `running` or `verifying`, and their age | A stuck engine, or Proxmox being slow |
| Tasks in `retrying`, attempts per task | Proxmox or network errors |
| Tasks re-claimed after lease expiry | Replicas crashing or being killed (expect none outside deploys) |
| Jobs failed by the reaper | Replicas dying in the middle of a job |
| `failed` and `partially_completed` executions, by `error.code` | Capacity, quota or template problems |
| Verification results in production | `unverified` means the platform could not check; expect `verified` for VPS work |
| `429 TOO_MANY_EXECUTIONS`, `409 APPROVAL_*`, `409 PLAN_VERSION_MISMATCH` | Users at the limit, stale clients |
| Open event streams and reconnects | A proxy that buffers or cuts the stream |

Available today: `docker compose logs backend`, *Admin -> Jobs*, and the
user's own `GET /api/v1/ai/executions?status=running`. A view of all
tenants' executions for admins (API or SQL): TBD (confirm after implementation).

### 4.9 Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Execution stays `queued` or `running`; no new events for minutes | A task waits for its next attempt (`retrying`, `next_attempt_at`); a replica was killed and the task waits for its lease to expire; no API replica is running; a slow Proxmox operation (large clone) | `GET /api/v1/ai/executions/<id>`: check each task's `status`, `attempt`, `next_attempt_at` and `error`. `docker compose ps`, `docker compose logs backend`, the VM's task log in Proxmox. Cancel the execution if it should not continue. |
| Execution stays `cancelling` | A running task ends its current step first (TBD (confirm after implementation)) | Wait; follow the task's events. |
| `409 APPROVAL_EXPIRED` | The approval was not used within 60 min (`medium`, `high`) or 15 min (`destructive`) | Review the plan and approve again. |
| `409 APPROVAL_SUPERSEDED` | The plan was edited after the approval | Review the new revision and approve it. |
| `409 APPROVAL_CONSUMED` | The approval already started an execution (double click, second tab, repeated `/execute`) | Open that execution (`latest_execution_id`). Approve again only if you want a second run. |
| `409 APPROVAL_REQUIRED` | `/execute` without a valid approval (none, wrong `approval_id`) | Approve first. |
| `409 PLAN_VERSION_MISMATCH` | The approval names a revision that is no longer current (plan edited, stale page) | Reload, check what changed, approve the current revision. |
| `409 PLAN_VERSION_CONFLICT` | `PATCH` with an old `base_version` | Reload and edit again. |
| `409 PLAN_NOT_EDITABLE` | The plan is approved, executing or finished | Ask for a new plan. |
| `409 EXECUTION_ACTIVE` | The plan already has an active execution | Follow that execution. |
| `409 TASK_NOT_RETRYABLE` | Retry of a task that is not `failed` | Retry only failed tasks. |
| `422 VALIDATION_FAILED` on approve or execute | Policy failed at that moment (quota used up in the meantime, template or size no longer available); `details.errors` says why | Fix the cause, `POST /api/v1/ai/plans/<id>/validate`, approve again. |
| `422 RISK_ACK_REQUIRED` | High or destructive plan without `acknowledge_risks: true` | Acknowledge the risks. |
| `422 CONFIRMATION_REQUIRED` / `CONFIRMATION_MISMATCH` | Destructive plan without the typed names, or with wrong ones | Type exactly the names in `confirmation_required.expected`: case-sensitive, comma-separated. |
| `403 ADMIN_REQUIRED` | The plan contains an admin-only tool | Run it as an admin, or remove that step. |
| `429 TOO_MANY_EXECUTIONS` | The user already has 3 active executions | Wait for one to finish, or cancel one. |
| Verification `unverified` in production | `PROXMOX_MOCK=true` (`/api/v1/system/config` shows `"proxmox_mock":true`); guest agent missing or not running in the VM; token without `VM.GuestAgent.Audit` (PVE 9) / `VM.Monitor` (PVE 8); Proxmox unreachable (`595`) | Production settings and the agent: [install guide, section 21](PROXMOX_INSTALL.md#21-troubleshooting). Test with `qm guest cmd <vmid> ping`. |
| Verification `failed` | The real state differs (VM not running, no IP address) | Check the VM in Proxmox; retry the failed task or delete the resource with a new plan. |
| Timeline only updates after a reload, or in bursts | A proxy buffers or compresses `text/event-stream` | Section 4.7: no buffering, no gzip; test with `curl -N` hop by hop. |
| Event stream reconnects every 60-100 s | A proxy holds back the 20 s heartbeat (buffering), so the connection looks idle and is closed | Turn off buffering; keep read timeouts above 20 s. |
| Own client gets `401` on the event stream | No `Authorization: Bearer` header | Send the header; use `fetch` streaming, not `EventSource` (contract section 5). |

---

## 5. API quick reference

Base `/api/v1`. Bearer token (the cookie works for GET only). Success
`{"data": ...}`, errors `{"error": {"code", "message", "details"}}`. The
complete contract with all fields is [ORCHESTRATOR_API.md](ORCHESTRATOR_API.md).

| Need | Call | Contract |
|---|---|---|
| Plan states, execution states, task states, risk levels | - | [section 1](ORCHESTRATOR_API.md#1-lifecycles) |
| Plan with its current revision, validation, approval | `GET /ai/plans/:id` | [section 2](ORCHESTRATOR_API.md#2-plan-extends-the-existing-plan-object) |
| Revision history; one revision | `GET /ai/plans/:id/revisions`, `GET /ai/plans/:id/revisions/:version` | [section 3](ORCHESTRATOR_API.md#3-endpoints) |
| Edit a plan | `PATCH /ai/plans/:id` `{base_version, specification}` | section 3 |
| Validate again | `POST /ai/plans/:id/validate` | section 3 |
| Approve (and start) | `POST /ai/plans/:id/approve` `{version, spec_hash, acknowledge_risks, confirmation, execute}` | section 3 |
| Start a saved approval | `POST /ai/plans/:id/execute` `{approval_id}` | section 3 |
| Reject; cancel | `POST /ai/plans/:id/reject` `{reason?}`; `POST /ai/plans/:id/cancel` | section 3 |
| List and read executions | `GET /ai/executions?conversation_id=&plan_id=&status=`; `GET /ai/executions/:id` | [section 4](ORCHESTRATOR_API.md#4-execution-object) |
| Events (JSON or SSE) | `GET /ai/executions/:id/events?after=<seq>&limit=200` | [section 5](ORCHESTRATOR_API.md#5-events) |
| Cancel an execution | `POST /ai/executions/:id/cancel` | section 3 |
| Retry one failed task | `POST /ai/executions/:id/tasks/:taskId/retry` | section 3 |
| Recovery | `POST /ai/executions/:id/recovery` `{action}` | section 3 |
| Tool registry | `GET /ai/tools` | [section 6](ORCHESTRATOR_API.md#6-tools-registry-also-served-to-mcp-toolslist) |
| MCP | `POST /mcp/rpc` (`tools/list`, `tools/call`) | section 6 |
| Error codes | - | [section 7](ORCHESTRATOR_API.md#7-error-codes) |
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
