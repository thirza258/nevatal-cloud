# Backend architecture

The Nevatal Cloud API (`github.com/nevatal/cloud`) is organised in three
layers: **repository → usecase → API**. Each layer only knows the one below
it, so business rules can be read, tested and changed without touching HTTP
or SQL.

```
cmd/api/main.go            composition root: config → db → repositories → use cases → router
cmd/edge/main.go           second composition root: nevatal-edge (Nevatal Domains reverse proxy)
internal/
  repository/              persistence: one interface + GORM implementation per aggregate
  usecase/                 business logic: one XUsecase per domain
  domainroute/             pure routing policy shared by the API and the edge (no I/O)
  edge/                    nevatal-edge: route snapshot + httputil.ReverseProxy
  api/
    router.go              route table (SetupRouter)
    middleware/            auth (JWT / session cookie), admin guard, CORS
    handler/               thin gin handlers
    request/               request DTOs (binding tags) + param helpers
    response/              envelope helpers + response DTOs
  models/                  GORM models shared by all layers (database schema)
  proxmox/                 Proxmox VE client (real + simulated)
  config/, database/       configuration, connection, migrations, seeding
```

## Layers

### repository

* Persistence only: no business rules, no HTTP. A method is a query — its
  `WHERE`, preloads, ordering, limits and row locks live here.
* One file per aggregate (`user.go`, `instance.go`, `storage.go`, …), each
  with an interface (what use cases may ask for), a GORM implementation and a
  compile-time check `var _ XRepository = (*xRepository)(nil)`.
* Every method takes `ctx context.Context` first and runs on `b.conn(ctx)`:
  the transaction stored in `ctx` if there is one, otherwise the pool bound
  to `ctx`.
* `repository.go` holds the shared pieces:
  * `ErrNotFound` — returned instead of `gorm.ErrRecordNotFound`. Its text is
    the same (`record not found`) because some endpoints echo it.
  * `Transactor.WithinTransaction(ctx, fn)` — runs `fn` with a context that
    carries a transaction; repository calls made with that context join it.
    A nested call joins the outer transaction.
* Queries are not merged even when they look alike: a different preload set
  or ordering changes the JSON clients receive.

### usecase

* Business logic, one `XUsecase` per domain, built with `NewXUsecase(...)`.
* Depends on repository interfaces, `proxmox.ProxmoxClient`, `*config.Config`
  and other use cases — never on gin or `gorm.io/*`.
* Use cases with many repositories take an `XRepos` struct (for example
  `InstanceRepos`) so the composition root stays readable.
* Errors:
  * `*usecase.Error{Kind, Code, Message}` when an endpoint answers different
    conditions with different status codes. `Kind` (invalid, unauthorized,
    forbidden, not found, conflict, unprocessable, unavailable, bad gateway)
    is mapped to a status by the API layer.
  * Plain errors otherwise; the handler reports them with the endpoint's
    default status and code.
  * `usecase.ValidationError(field, message)` is a 400 `VALIDATION_ERROR`
    whose `Field` the API reports as `details.field` (used by Nevatal
    Domains so the SPA can attach the message to a form field).
* Ownership: methods that take `userID, isAdmin` scope lookups to the owner
  unless `isAdmin` (helper `ownerScope`). A resource of another tenant is
  reported exactly like a missing one.
* Contexts:
  * Reads use the request context.
  * Methods that change state start with `ctx = context.WithoutCancel(ctx)`:
    a client disconnect must not leave a half-applied change (job created but
    instance not updated, snapshot taken but not recorded, missing audit
    entry). Hypervisor calls get their own timeout derived from that context.
  * Work that outlives the request never uses the request context. Background
    jobs go through `WorkerUsecase.Go` (tracked for graceful shutdown) and
    build their contexts from `context.Background()`.
* `AuditUsecase.Log` never fails the caller and is detached from
  cancellation.

### api

* `handler/` — thin gin handlers: parse params, bind the request DTO, call one
  or more use case methods, write the response. No database, no
  repositories, no `gin.H`.
* `request/` — request DTOs with `binding` tags, one file per domain, and
  `ToUsecase()` conversions. A DTO keeps the Go type name the API bound into
  historically, because validation and JSON errors embed it
  (`Key: 'CreateInstanceInput.Name' …`). Bodies that used to be anonymous
  structs are aliases of unnamed structs (`type X = struct{…}`) for the same
  reason.
* `response/` — `OK` / `Created` / `Accepted` / `NoContent` write
  `{"data": …}`; `Error` writes `{"error": {"code", "message"}}`
  (`ErrorWithDetails` adds `details`); `ErrorMessage` writes the code-less
  `{"error": {"message"}}` used by the game server, Android, MCP and AI agent
  endpoints; `JSON` writes a body as is for the few legacy non-enveloped
  endpoints (health, WordPress bridge, some admin/storage acknowledgements).
  Composite payloads are typed DTOs whose fields are declared in alphabetical
  JSON-key order, so the encoded bytes match the map-based bodies the API
  returned before. Models are returned directly as `data` where the API
  always did so.
* `middleware/` — `AuthRequired` (Bearer token or `nevatal_token` cookie),
  `AdminOnly`, CORS allowlist.

## Dependency rule

```
api/handler ──▶ usecase ──▶ repository ──▶ gorm / models
     │             │
     └─ request,   └─ proxmox, config
        response
```

* `internal/usecase` and `internal/api` must not import `gorm.io/*`.
* `internal/api/handler` must not import `internal/repository`.
* Only `cmd/api/main.go` constructs repositories and use cases; `cmd/edge/main.go`
  constructs the one repository the edge reads.
* `internal/domainroute` imports only the standard library and `models`. Both
  `usecase.DomainUsecase` (the `live_status` of a domain) and `internal/edge`
  (whether to proxy) call `domainroute.Policy.Evaluate`, so the dashboard and
  the proxy cannot disagree.
* `internal/edge` sits at use-case level: it may import `repository` (through
  the small `edge.Source` interface) and `domainroute`, never `api` or `usecase`.

## Nevatal Domains and nevatal-edge

```
browser → Cloudflare (*.PUBLIC_DOMAIN_ROOT, TLS) → Cloudflare Tunnel → cloudflared
        → nevatal-edge (EDGE_PORT) → customer VM private_ip:target_port
```

* The API owns the claim rules (`usecase/domain.go`): subdomain syntax,
  reserved names, per-user limit (advisory lock per owner), quarantine of
  released hostnames. Hostname uniqueness comes from the partial unique index
  `uq_domains_hostname_active` (migration 000006), mapped to `SUBDOMAIN_TAKEN`.
* The edge never migrates or seeds. It reloads `DomainRepository.LoadEdgeSnapshot`
  every 2 s (and at most once per second on an unknown hostname), keeps the last
  good snapshot when the database is unreachable, and only dials addresses inside
  `EDGE_TARGET_CIDRS` that are not in `EDGE_DENY_IPS` (checked again in the dialer).
* The edge is configured by `config.Load()` + `Config.ValidateEdge()` (not
  `Validate()`, which requires the API secrets) and refuses to run with
  `DEMO_MODE=true`.

Checks:

```
grep -rn '"gorm.io' internal/usecase internal/api          # nothing
grep -rn 'gin.H' internal/api/handler                      # nothing
grep -rln 'repository"' internal/api/handler               # nothing
go test ./internal/api -run TestRouteTable                 # route table unchanged
```

## How to add an endpoint

1. **Repository** (if new data access is needed): add a method to the
   aggregate's interface and implement it in the same file, `ctx` first,
   using `r.conn(ctx)` and returning `mapErr(err)` for single-row lookups.
   Keep preloads and ordering explicit.
2. **Use case**: add a method `func (s *XUsecase) DoThing(ctx context.Context, …)`.
   Enforce ownership with `ownerScope(userID, isAdmin)`. Start with
   `ctx = context.WithoutCancel(ctx)` if it changes state. Wrap multi-row
   writes in `s.tx.WithinTransaction(ctx, func(txCtx context.Context) error { … })`
   and use `txCtx` inside. Audit with `s.auditService.Log(ctx, …)`. Return a
   `*usecase.Error` when the endpoint needs a specific status/code.
3. **Request DTO**: add the body struct with `json` and `binding` tags to
   `internal/api/request/<domain>.go`, plus a `ToUsecase()` conversion.
4. **Response DTO** (only for composite bodies): add a struct to
   `internal/api/response/<domain>.go` with fields in alphabetical JSON-key
   order.
5. **Handler**: in `internal/api/handler/<domain>.go`, parse params with
   `request.UUIDParam`, bind with `request.BindJSON`, pass
   `c.Request.Context()`, `middleware.GetCurrentUserID(c)`,
   `middleware.IsAdmin(c)` and `c.ClientIP()` to the use case, then write
   with `response.*` (or `writeError` for use case errors).
6. **Route**: register it in `internal/api/router.go`; new dependencies go
   into `RouterParams` and are wired in `cmd/api/main.go`.
7. **Tests**: `go test ./...`. The route table test compares against
   `internal/api/testdata/routes.golden`; regenerate it deliberately with
   `go test ./internal/api -run TestRouteTable -update-routes` and review the
   diff.

## AI infrastructure orchestrator

Contract: `docs/ORCHESTRATOR_API.md`. Principle: the planner (chat agent, MCP)
only proposes; the policy engine validates; `POST /ai/plans/:id/approve` is the
only approval; the engine executes; verification confirms.

```
internal/orchestrator/   no gorm; persistence through repository.OrchestratorRepository
  registry.go            tool definitions (GET /ai/tools = MCP tools/list), type/action -> tool
  spec.go, canonical.go  specification -> task DAG (Compile), spec_hash (canonical JSON, Go only)
  policy.go, schema.go   deterministic policy engine (schema, ownership, catalog, quota, capacity, risk)
  engine.go              leased task runner, retries, DAG progression, verification, finalization
internal/usecase/orchestrator*.go  approval manager + API, tool implementations (call the existing
                                    use cases), policy environment, verification and report hooks
internal/repository/orchestrator.go  revisions, approvals, executions, tasks, events, relationships
migrations/000007_orchestrator     tables, approval binding columns, unique job idempotency index
```

* **Execution:** every API replica runs one engine (`cmd/api/main.go`). Tasks are claimed with
  `FOR UPDATE SKIP LOCKED` and leased (`ORCH_LEASE_SECONDS`, renewed every
  `ORCH_HEARTBEAT_SECONDS`); every write is fenced by the claim's `lease_token`. An expired lease
  is re-claimed with the same attempt and idempotency key `orch:<execution>:<task>:<attempt>`.
  SIGTERM: stop claiming, let running tasks finish within the budget, release unfinished leases.
* **Tools** live in package usecase and drive the existing use cases; they are idempotent per
  attempt (`InstanceUsecase.Create`/`QueueOperation` with the key; adoption of resources a crashed
  run created) and start a new attempt only after the previous attempt's job is terminal and its
  leftovers are discarded. Outputs are whitelisted fields, never secrets.
* **Non-idempotent tools** (`ToolDef.Idempotent == false`, Phase 1: `vps.snapshot.restore`):
  `Registry.Register` sets `MaxAttempts` to 1, `Engine.handleFailure` never schedules an automatic
  retry for them, and `Engine.runTask` fails a re-claimed attempt (`claim_count > 1`) with
  `INTERRUPTED_NOT_IDEMPOTENT` instead of running the tool again. Only a manual retry re-runs it.
* **Recovery options:** `delete_created` is offered when `Hooks.CanDeleteCreated` finds `created`
  relations (VPS, bucket) without a `deleted` one whose resource still exists — the same set
  `deleteCreatedPlan` uses — computed at finalize and re-checked when an execution is read.
* **Chat read requests** (`AIAgentUsecase.ProcessUserMessage`, `isReadRequest`): list/show/status
  questions without a change verb are answered through `OrchestratorUsecase.CallTool` read tools
  (`vps.list`, `vps.status`) and never reach the create workflows or change the conversation intent.
* **Jobs:** provisioning jobs heartbeat (`JOB_HEARTBEAT_SECONDS`); the reaper in every replica fails
  jobs without one for `JOB_STALE_AFTER_SECONDS` as `JOB_STALE` and cleans up. Terminal job writes
  are conditional (`WorkerUsecase.finishJob`), so a reaped job is never resurrected.
* **Events:** `ai_execution_events.seq` comes from a counter row-locked in the same transaction,
  so seq order is commit order and `?after=` pollers never skip an event. SSE streams end on
  `execution.finished` and when the server shuts down (`RouterParams.Streams`).
* **Planner entry points:** the chat agent, MCP write tools (`MCPUsecase.callRegistryTool` →
  `OrchestratorUsecase.CallTool`) and the OpenRouter assistant's action cards
  (`AIAssistantUsecase.ExecuteAction` → `OrchestratorUsecase.ProposeFromAssistant`) only create
  plans. MCP and the assistant share `planFromTool` (one plan per tool call, filed in the
  "MCP requests" / "AI assistant requests" conversation, audited `ai.plan.create_from_mcp` /
  `ai.plan.create_from_assistant`) and return the workspace deep link `PlanURL`
  (`/dashboard/ai/conversations/<conversation>?plan=<plan>`). Nothing outside the engine calls
  `InstanceUsecase.Create` for a planner.
* **MCP tool permissions** (`MCPUsecase.authorizeToolCall`, run for every `tools/call` before a
  tool runs or a plan is created):
  * Caller = the Bearer-token user of `POST /api/v1/mcp/rpc`. Their connection = their
    `mcp_connections` row for the server that owns the tool — `nevatal-cloud` for orchestrator
    registry tools and legacy `cloud.*` names, the tool's `mcp_tools.server_id` otherwise. A user
    has at most one connection per server (`ConnectServer` reuses it).
  * The registry tools are `mcp_tools` rows of `nevatal-cloud` (seeded from
    `orchestrator.DefaultRegistry()` on every start, risk mapped to read/write/destructive), so
    `mcp_permissions` can grant or revoke them; `SetPermission` creates a missing row.
  * Decision: no connection to `nevatal-cloud` → implicit first-party connection, every tool
    granted; no connection to another server → refused (`MCP_SERVER_NOT_CONNECTED`);
    connection status not `connected` → refused (`MCP_CONNECTION_INACTIVE`); a `granted=false`
    row for the tool **or one of its aliases** (registry tool ↔ legacy name in `legacyToolMap`)
    → refused (`TOOL_PERMISSION_DENIED`); a tool without a row → granted (connecting grants
    every tool).
  * A refusal is a JSON-RPC error `-32001` with `data.code`, audited in `mcp_audit_logs` as
    `denied`; nothing runs and no plan is created. Write/high/destructive tools that pass still
    only create plans.
  * Limit: the MCP client authenticates with the user's own JWT, which can also call the REST
    API (including the permissions endpoint). Permissions restrict well-behaved agents; scoped
    per-connection MCP tokens are the follow-up that makes them a hard boundary.
* **Tests:** `internal/orchestrator` (unit) and `internal/integration` (Postgres, uses
  `NEVATAL_TEST_DATABASE_URL`, default the QA container; skipped when unreachable);
  end to end: `scripts/qa/orchestrator_e2e.py`.
