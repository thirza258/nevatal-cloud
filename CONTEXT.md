# Nevatal Cloud — Working Context (handoff)

Last updated: 2026-11-02. Purpose: let a new session continue without the old chat.
**Nothing is committed.** All work below is uncommitted on `main` (`git status` shows it).

---

## 0. Checkpoint policy (user instruction, 2026-11-02)

**When the Claude usage limit is hit** (or a "usage limit reached / grace allowance"
notice appears), push all work to `main` so nothing is lost:

```bash
NO_CHECKS=1 scripts/checkpoint.sh "<what was in progress>"   # fastest (grace window)
scripts/checkpoint.sh "<what was in progress>"               # with go build + tsc status
```

- The user authorized committing and pushing to `origin/main` in this situation.
  Any session or agent may run it. The user can also run it: `! scripts/checkpoint.sh "note"`.
- If time allows, first update §2/§5 of this file with what was in progress (one line is enough).
- The script stages everything, refuses secret-looking files (`.env`, `secrets.env`,
  `*.pem`/`*.key`, private keys), notes the build status in the commit message, commits
  with the Claude co-author line, and pushes. It **never force-pushes**. If `main` moved on
  the remote, it pushes to `checkpoint/<timestamp>` instead (merge later).
- Checkpoint commits can be WIP or even fail to build (the message says which).
  **Do not deploy from `main` without running the §4 gates first.**
- Outside the limit situation, still commit/push only when the user asks.

---

## 1. Project in one paragraph

Self-hosted VPS control plane for **Proxmox VE** on a **Dell PowerEdge R640**.
Go 1.26 API (`backend/`, gin + GORM + Postgres 16) + React 19/Vite SPA
(`frontend/`, served by nginx). Public domain **nevatal.id**; the operator
exposes services through a **Cloudflare Tunnel** (no public IP).
Control plane URL: `cloud.nevatal.id`.

---

## 2. Requests from the user (in order) and status

| # | Request | Status |
|---|---|---|
| 1 | Fix landing page with the *right* R640 design | **Done** |
| 2 | Demo + production configuration, ready to deploy on Proxmox | **Done** |
| 3 | Tutorial `.md` for installing on Proxmox | **Done** — `docs/PROXMOX_INSTALL.md` |
| 4 | Load balancing + Kubernetes configuration | **Done** |
| 5 | Docker logs / Docker viewer for a selected VM (customer + admin) | **Done** |
| 6 | Backend split: repository / usecase; API split: request / response / handler | **Done** (merged, all gates green) |
| 7 | Railway-style subdomains `<name>.nevatal.id` with Cloudflare Tunnel + Proxmox | **Done** (backend, frontend, infra verified — see §5) |
| 8 | P2: platform-wide automated backup & recovery | **Designed, not started** (see §6) |
| 9 | PRD "AI Infrastructure Orchestrator" (pasted by user, v1.0, 2026-11-02) | **Phase 1 in progress** — frozen contract `docs/ORCHESTRATOR_API.md`; backend + frontend agents running; see §6b |

**Build order:** domains (finish + verify) → Orchestrator Phase 1 (P0) → backups (P2) →
Orchestrator Phases 2–4. Reserve migrations: `000006` domains, `000007` orchestrator,
`000008` backups. Run one backend agent at a time (shared router/main/usecase package).

---

## 3. What was built (done)

### 3.1 Landing page (R640)
- Old `frontend/public/dell-r640-blueprint.png` (+ identical copy) was a noisy AI image → **deleted**.
- New vector drawing `frontend/src/components/landing/R640Chassis.tsx`, drawn from Dell's
  R640 Installation & Service Manual, Fig. 4 (10 × 2.5″): 2 rows × 5 horizontal carriers,
  vent strip, VGA, left panel (health/ID bar + 5 status LEDs), right panel (power, USB 2.0,
  iDRAC Direct micro-USB). Variants **Blueprint** (default; dimensions 482.0 mm × 42.8 mm/1U,
  callouts ①–④) and **Front panel** (rendered, blinking activity LEDs, honors reduced motion).
  Hover/focus a bay → caption. Mobile: horizontal scroll inside the figure.
- Wired in `frontend/src/pages/LandingPage.tsx` (segmented toggle replaces the CSS-filter hack).

### 3.2 Demo vs production (backend `internal/config/config.go`)
- `APP_ENV` (`development|production`, falls back to legacy `ENV`), `DEMO_MODE`
  (default true unless production). Demo **forces `PROXMOX_MOCK=true`**.
- Production `Validate()` fails fast: `JWT_SECRET` (≥32, not the public dev default),
  `ADMIN_EMAIL`/`ADMIN_PASSWORD` (≥12, not a demo password) when `DEMO_MODE=false`,
  Proxmox creds when `PROXMOX_MOCK=false`, absolute `APP_BASE_URL`, no `*` CORS.
- Other env: `CORS_ALLOWED_ORIGINS`, `TRUSTED_PROXIES`, `COOKIE_SECURE`, `STORAGE_PATH`,
  `DB_MAX_OPEN_CONNS` (15), `DB_MAX_IDLE_CONNS` (5), `WORDPRESS_API_KEY` (legacy public key
  rejected in prod → integration disabled), `config.Version` (ldflags `-X`).
- Seeding (`internal/database/seed.go`, `bootstrap.go`): catalogs always; demo users/data only
  in demo; admin from `ADMIN_*`; reused demo DB in prod → public demo accounts **suspended**,
  configured admin still on a demo password → **rotated**. Migrations + seed run under a
  **Postgres advisory lock** (safe with many replicas). Single-node `PROXMOX_NODE_NAME` is
  authoritative (renames the seeded node).
- `GET /api/v1/system/config` (public) → `environment, demo_mode, proxmox_mock, version,
  features, auth` — the SPA hides role switcher / quick logins / social buttons unless demo
  (fail-closed `frontend/src/context/SystemConfigContext.tsx`).

### 3.3 Security fixes (important)
- **Critical (pre-existing):** `POST /api/v1/auth/oauth/{google,github}` with
  `{"mock_email":"admin@..."}` minted an admin session; `/auth/oauth/wordpress` same without a
  key. Now: mock only in demo; prod needs real code exchange (+ verified email, GitHub
  primary verified email), `redirect_uri` under `APP_BASE_URL`; WordPress SSO needs
  `X-Nevatal-Key` (constant-time) and cannot sign in admins.
- CORS allow-list (was reflect-any-origin with credentials); trusted proxies for `ClientIP()`.
- Session cookie: `__Host-nevatal_token` when Secure (legacy `nevatal_token` on plain HTTP);
  **cookie auth only for GET/HEAD**, mutations require Bearer (tenant apps on sibling
  subdomains are same-site → CSRF). Smoke checks cover it.
- Refactor-time authz fixes: `GET /instances/:id/activity` ownership; game-server
  sub-resources (metrics/players/plugins/mods/worlds/backups/restore) ownership;
  `POST /storage/shared-links` ownership; MCP presigned download + game-server tools.
- Later fixes: vCPU quota never enforced (`ResourceTotals` column tags), AI-agent SSH key
  query (`owner_id`), WordPress `list_instances` unknown email returned all tenants,
  unpublished Android images visible to customers, Preload `LIMIT` capped across parents.
- Other pre-existing bugs fixed: demo VM never seeded (template IDs on loop copies),
  `openclaw_instances.ai_provider` / `storage_objects.etag` column mismatches, game-server
  backup checksum too long, Proxmox `DELETE` params dropped (purge never sent).

### 3.4 Docker viewer (via QEMU guest agent)
- Proxmox: `internal/proxmox/agent_exec.go` (`AgentExec`, `AgentExecStatus`,
  `RunAgentCommand`, error classification; `APIError` reads PVE reason phrase),
  `mock_agent.go` (fake Docker engine for demo/mock).
- API: `GET /api/v1/instances/:id/docker?stats=true`,
  `GET /api/v1/instances/:id/docker/containers/:container/logs?tail=200&since=1h`,
  admin twins under `/api/v1/admin/instances/:id/docker...`. Fixed argv only, container ref
  regex `^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`, tail 1–5000, since `^[0-9]{1,4}[smh]$`;
  audit de-duplicated (10 min).
- Needs `qemu-guest-agent` in the guest + token privilege **PVE 8: `VM.Monitor`,
  PVE 9: `VM.GuestAgent.Unrestricted`** (PVE 9 dropped `VM.Monitor`).
- UI: `frontend/src/components/docker/*`, Docker tab in `InstanceDetails.tsx`,
  `frontend/src/pages/admin/AdminDocker.tsx` (`/admin/docker?instance=<uuid>`).

### 3.5 Backend architecture (refactor, merged)
- `internal/repository` (GORM, `Transactor` with tx in ctx, `ErrNotFound`),
  `internal/usecase` (no gorm imports), `internal/api/{handler,request,response,middleware}`,
  `cmd/api/main.go` composition root + graceful shutdown (drains HTTP 20 s, jobs 40 s).
  Read `backend/ARCHITECTURE.md` before adding endpoints.
- Old `internal/services`, `internal/api/handlers` deleted.

### 3.6 Deployment (infra)
- `backend/Dockerfile` multi-stage (golang:1.26-alpine → alpine:3.24, uid 10001),
  `frontend/Dockerfile` (node:22 → nginxinc/nginx-unprivileged:1.30, port 8080),
  nginx template `frontend/nginx/default.conf.template` (+ `snippets/`): `resolve`-based
  least_conn upstream over scaled backends, never retries non-idempotent requests,
  upload limits, `/nginx-health`.
- Compose: base `docker-compose.yml` + `deploy/compose/{demo,production,multi-vm}.yml`;
  env templates `deploy/env/{demo,production}.env.example` (set `COMPOSE_FILE`,
  `COMPOSE_PROJECT_NAME` nevatal-demo / nevatal-prod); Caddy TLS profile `deploy/caddy/`.
- `deploy.sh --mode demo|production` (validation, build, migrate, health, `--status`,
  `--rollback`, `--migrate-only`, `--reset-demo --yes`).
- LB for multi-VM: `deploy/loadbalancer/haproxy/haproxy.cfg`, `keepalived/keepalived.conf`.
- Kubernetes: `deploy/k8s/base` + `overlays/{demo,production}` + `addons/{metallb,cert-manager,traefik}`
  (headless backend Service, HPA 2–6 prod, PDBs, NetworkPolicies, Postgres StatefulSet).
  Images `ghcr.io/thirza258/nevatal-cloud-{backend,frontend}:1.2.0` are **not published yet**.
- Tutorial `docs/PROXMOX_INSTALL.md` (~1050 lines): least-privilege token (PVE 8/9 table),
  cloud-init templates with qemu-guest-agent, control-plane VM, demo/prod deploy, TLS,
  firewall, demo→prod migration, backups, upgrades, LB, k3s, troubleshooting.

### 3.7 Frontend performance
- Lazy routes + vendor chunks: entry JS 872 kB → ~80 kB (gzip ~21 kB); no chunk > 500 kB.

---

## 4. Verification gates (all were green at handoff for done work)

QA scripts are in `scripts/qa/` (copied from the session scratchpad). They need a throwaway
Postgres:

```bash
docker run -d --rm --name nevatal-smoke-pg -e POSTGRES_PASSWORD=smokepw \
  -p 127.0.0.1:55433:5432 postgres:16-alpine
```

```bash
cd backend
go build ./... && go vet ./... && go test -count=1 ./...
gofmt -l internal cmd                                   # must print nothing
grep -rn '"gorm.io' internal/usecase internal/api       # must print nothing
grep -rn 'gin.H' internal/api/handler                   # must print nothing
go test ./internal/api -run TestRouteTable              # route golden (internal/api/testdata/routes.golden)
go build -o ../scripts/qa/nevatal-api ./cmd/api && bash ../scripts/qa/smoke.sh   # expect RESULT: 45 passed, 0 failed
bash ../scripts/qa/capture_shapes.sh "$PWD" ../scripts/qa/shapes-now.json && \
  diff <(python3 -m json.tool ../scripts/qa/shapes-golden.json) <(python3 -m json.tool ../scripts/qa/shapes-now.json)
```
- Route golden: regenerate with `-update-routes` only when adding routes; diff must be additions only.
- Shapes golden was captured **before** the domains feature: after domains lands, only
  `customer GET /system/config` may differ (new `domains`/`features.domains` keys).
- Frontend: `cd frontend && npm run build` (zero TS errors).
- Infra: `docker compose config -q` per mode, `kubectl kustomize deploy/k8s/overlays/{demo,production}`,
  shellcheck `deploy.sh`. Full e2e: copy repo to a temp dir, `cp deploy/env/<mode>.env.example .env`,
  fill secrets, `./deploy.sh --mode <mode> --yes` (done for demo and production, 2 replicas — passed).

---

## 5. IN PROGRESS at handoff: Railway-style domains (`<name>.nevatal.id`)

Status:
- **Backend: DONE + independently verified.** Build/vet/tests green (new pkgs `internal/edge`,
  `internal/domainroute`); route golden = exactly 7 added routes; smoke 45/45; shapes differ only
  in `customer GET /system/config`; domains e2e 81/81 (script was in the session scratchpad
  `domains/e2e.py`: claim/check/patch/release, quarantine, parallel-claim race → one 201,
  limit, stop→503, ip_conflict/out-of-CIDR→502, admin disable/enable, edge 404/502/503, demo refusal).
  Edge binary: `go build -o nevatal-edge ./cmd/edge`; health `/__edge/health`; env: `DATABASE_URL`,
  `PUBLIC_DOMAIN_ROOT`, `DEMO_MODE=false` (explicit!), `EDGE_TARGET_CIDRS`, `EDGE_DENY_IPS`,
  `EDGE_PORT`, `PROXMOX_HOST` (IP literal auto-denied). API and edge must share the same
  CIDR/deny settings. Only cloudflared may reach the edge port (it trusts `Cf-*` headers).
- **Frontend: DONE** (build green; agent headless harness 124/124).
- **Infra/docs: DONE + verified.** Dockerfile builds `/app/nevatal-edge`; compose services `edge`
  (profile `edge`; refuses to start without root or in demo) and `cloudflared` (profile `tunnel`,
  `cloudflare/cloudflared:2026.10.7`, token `CLOUDFLARE_TUNNEL_TOKEN`); optional
  `deploy/compose/edge-publish.yml`; `deploy/cloudflared/config.yml.example` (ordered ingress);
  k8s `deploy/k8s/components/edge` (in production overlay) + `addons/cloudflared`; deploy.sh validates
  domain/tunnel settings and removes containers of disabled profiles. Tutorial §20 (20.1–20.10),
  troubleshooting moved to §21. Verified: compose config (prod+edge+tunnel), kustomize
  production 26 / demo 18 / cloudflared 4 objects, shellcheck; agent e2e served a claimed name
  through the edge in a scratch prod deploy. Not verified: real tunnel token, live k8s apply,
  dashboard wildcard hostname, Proxmox firewall ipfilter commands.
- Follow-up idea: read-only DB role for the edge (`EDGE_DATABASE_URL`).
- Follow-up: add the tenant domain to the Public Suffix List; ip_conflict can be triggered by a
  tenant reporting another VM's IP (both domains 502) until Proxmox ipfilter/static IPs exist.

### Architecture
Browser → Cloudflare (DNS `CNAME * <TUNNEL_UUID>.cfargotunnel.com`, proxied; Universal SSL
covers only `nevatal.id` + first-level `*.nevatal.id`) → Cloudflare Tunnel → `cloudflared`
ingress (order: `cloud.nevatal.id` → frontend `:8080`; `*.nevatal.id` → edge `:8088`;
`http_status:404`) → **`nevatal-edge`** (Go reverse proxy, DB routes) → customer VM
private IP:port. Edge host needs a NIC/route into the customer VM bridge/VLAN.

### Contract
- Env: `PUBLIC_DOMAIN_ROOT` (empty = off; **forced off in demo**), `RESERVED_SUBDOMAINS`,
  `MAX_DOMAINS_PER_USER` (5), `DOMAIN_QUARANTINE_DAYS` (14), `EDGE_TARGET_CIDRS` (required in
  prod when root set), `EDGE_DENY_IPS`, `EDGE_PORT` (8088), `CLOUDFLARE_TUNNEL_TOKEN` (compose profile `tunnel`).
- Migration `000006_add_domains`: `domains` table, unique index on `lower(hostname)`
  `WHERE released_at IS NULL`; release = soft (`released_at`) + quarantine for other owners.
- API: `GET/POST /api/v1/domains`, `GET /api/v1/domains/check?subdomain=`,
  `PATCH/DELETE /api/v1/domains/:id`, `GET /api/v1/admin/domains`,
  `PATCH /api/v1/admin/domains/:id {admin_disabled, disabled_reason}`.
  Domain JSON includes `live_status` ∈ live | disabled | instance_missing | instance_stopped |
  no_ip | ip_not_routable | ip_conflict. Errors: VALIDATION_ERROR, SUBDOMAIN_RESERVED,
  SUBDOMAIN_TAKEN, SUBDOMAIN_QUARANTINED, DOMAIN_LIMIT_REACHED, INSTANCE_NOT_FOUND,
  DOMAIN_NOT_FOUND, DOMAINS_DISABLED (503).
  `system/config` adds `domains: {enabled, root, max_per_user}` + `features.domains`.
- Subdomain rules: 3–63, `^[a-z0-9](?:[a-z0-9-]*[a-z0-9])$`, no `xn--`, no "nevatal", not reserved.
- Edge: refresh routes every 2 s; proxy only if instance running, IP set, IP ∈ `EDGE_TARGET_CIDRS`,
  ∉ `EDGE_DENY_IPS`, not loopback/link-local, and no other live instance shares the IP
  (guest-agent IPs are guest-controlled → SSRF/spoof defense). Keep public Host;
  X-Forwarded-For from `Cf-Connecting-Ip`; branded 404/502/503 pages; WebSockets OK.
- Worker IP selection: prefer IPv4 inside `EDGE_TARGET_CIDRS`, skip docker*/br-*/veth*/etc.;
  refresh `private_ip` after start/restart.
- Frontend: `/dashboard/domains` (claim with debounced availability check, instance + port,
  live status, re-point, release modal), domains block in `InstanceDetails`, `/admin/domains`.
- Infra: Dockerfile builds `./cmd/edge` → `/app/nevatal-edge`; compose `edge` service +
  `cloudflared` (profile `tunnel`); k8s edge + `addons/cloudflared`; tutorial section
  "Cloudflare Tunnel and Railway-style subdomains".

### Decision pending with the user
Tenant apps on `*.nevatal.id` share the registrable domain with `cloud.nevatal.id`
(phishing on one tenant can get the whole domain blocklisted; parent-domain cookies leak).
Recommended: a separate domain for tenant apps (e.g. `nevatal.app`) — config-only switch
(`PUBLIC_DOMAIN_ROOT`). Control plane is already hardened either way (§3.3).

---

## 6. NEXT: P2 platform-wide backup & recovery (designed, not started)

Start only after the domains work is verified (same files: router, migrations, nav, compose).
- **VM engine:** Proxmox `vzdump` (`POST /nodes/{node}/vzdump`, mode snapshot, zstd) to
  `PROXMOX_BACKUP_STORAGE` (PBS recommended; NFS/dir ok; not the same ZFS pool as VMs).
  List via storage content (`content=backup&vmid=`), delete volume, protect flag.
  Restore via `POST /nodes/{node}/qemu` with `archive` — **in place** (`force=1`, stop → restore
  → start, automatic pre-restore backup) or **as new** (new VMID, `unique=1`, quota-checked).
  Verify exact params/privileges against the PVE API docs.
- **Unified catalog** `backups` (resource_type: instance | game_server | android | app |
  automation | openclaw | bucket; provider; status; trigger manual/scheduled/pre_restore;
  size; location; protected) with adapters for existing game-server/Android backups;
  buckets → real `tar.gz` under `BACKUP_LOCAL_PATH`.
- **Policies** `backup_policies`: hourly/daily/weekly at UTC time, retention keep_last /
  daily / weekly / monthly; pruning never touches manual/protected; scheduler in API claims
  due rows with `FOR UPDATE SKIP LOCKED` (exactly-once across replicas); per-resource and
  per-user limits; jobs reuse provisioning-job progress.
- **API:** `/api/v1/backups` (list/create 202/get/restore/patch/delete),
  `/api/v1/backup-policies`, admin overview (all tenants, failures, storage usage).
- **UI:** Backups page, Backups tab on VM details (next to Snapshots), restore modal
  (in-place vs as-new), admin backup health page.
- **Control plane DR:** nightly `pg_dump` sidecar (compose) + k8s CronJob with retention,
  optional off-site rclone to Cloudflare R2/S3, restore runbook in the tutorial; token
  privileges (`VM.Backup`, `Datastore.*` on backup storage) for PVE 8/9.
- **Gates:** route golden additions only, unchanged shapes for existing endpoints, smoke 45/45,
  new tests (retention, scheduler exactly-once, restore with mocked vzdump/storage listing).
- Open question for the user: where backups live (PBS datastore vs extra disks/NFS).

---

## 6b. AI Infrastructure Orchestrator (PRD) — gap analysis + Phase 1 plan

Principle from PRD: *LLM plans → policy engine validates → approval manager authorizes →
execution engine executes → verification engine confirms.* The model is never the authority.

**What exists today** (`backend/internal/usecase/ai_agent.go` ~2.1k lines, `models/ai_agent.go`,
`usecase/mcp.go`, routes under `/api/v1/ai/*` and `/api/v1/mcp/rpc`, UI `frontend/src/pages/ai/AICloudAgent.tsx`):
conversations, messages with UI cards, state machine (`AIConversationState`), rule-based
intent handlers (`handleVPSWorkflow`, website, n8n, openclaw, storage, game server, android),
`AIResourcePlan` + `AIPlanResource`, `AIApproval` (24 h expiry), `AIToolInvocation`, `AIAgentRun`,
MCP JSON-RPC with per-connection tool permissions and risk levels.

**Gaps vs PRD Phase 1:**
- `ApprovePlan` provisions **synchronously inside the HTTP request** (`for _, res := range plan.Resources`) — must become an async, durable task engine.
- No plan versioning; approval not bound to `plan_version`; no invalidation on edit.
- No task DAG (`InfrastructureTask` with dependencies/state/retries/idempotency keys); no recovery after restart; no cancellation.
- No post-execution verification (desired vs actual state); success = tool returned.
- No execution events stream (SSE) / timeline UI; no final execution report.
- No uniform tool registry (id, schemas, risk level, permissions, idempotency, timeout).
- No destructive-operation flow with strong confirmation (Scenario C) or partial-failure recovery options (Scenario D).

**Phase 1 status:** API contract frozen in `docs/ORCHESTRATOR_API.md` (single source for backend +
frontend). Four agents were launched: backend (engine), frontend (AI workspace), QA (independent
adversarial suite `scripts/qa/orchestrator_contract_test.py` written from the contract only — rerun
it after the backend lands; failures there are real findings), docs (`docs/ORCHESTRATOR.md` — **done**, 759 lines; 23 markers
`TBD (confirm after implementation)` to fill from the backend report: grep that string; README got an
"AI Infrastructure Orchestrator" section; PROXMOX_INSTALL intro links it).
Contract clarified 2026-11-02 (agents notified): an `approved` plan not yet executing IS editable
(PATCH supersedes the approval → `awaiting_approval`); `PLAN_NOT_EDITABLE` only for
executing/verifying/terminal. MCP endpoint must also handle `initialize`, `notifications/initialized`,
`ping` (desktop clients send `initialize` first). Expected artifacts: migration
`000007_orchestrator`, orchestrator engine/policy/registry/verification, new `/api/v1/ai/*` routes,
`scripts/qa/orchestrator_e2e.py`, upgraded AI workspace UI. Verify with §4 gates (shapes golden was
refreshed to post-domains state, so the shapes diff must now be empty) + the orchestrator e2e
(Scenarios A, C, D, restart-resume, SSE).
Hard requirements given to the backend (must check): single execution path (old synchronous loop
removed), leased + heartbeated tasks re-claimed after crash, per-attempt idempotency keys + UNIQUE
index on `provisioning_jobs(owner_id, idempotency_key)` (dedupe first), job heartbeats + stale-job
reaper with cleanup, approvals bound to version + spec_hash (superseded on PATCH, consumed once,
expiry), typed confirmation for destructive, policy re-run at execute, MCP write tools → plan only,
`unverified` in mock mode, SSE heartbeat + shutdown-safe, backfill revision 1 for old plans.

**Phase 1 plan (P0), migration `000007_orchestrator`:**
- Tables: `infra_plans` (or extend `ai_resource_plans`: `plan_version`, `specification` JSONB, `dependency_graph`, `estimated_resources`, `validation_result`), `infra_plan_revisions`, `infra_executions`, `infra_tasks` (deps, status queued/running/succeeded/retrying/failed/cancelled/skipped/compensating, retry_count, idempotency_key unique, input_snapshot, output_reference, error_details, lease_owner/lease_expires_at), `infra_approvals` (plan_id + plan_version + risk_level + expires_at + status), `infra_execution_events`, `resource_relationships`.
- Plan lifecycle `draft → validating → awaiting_approval → approved → executing → verifying → completed` (+ rejected/cancelled/failed/partially_completed).
- Policy engine (deterministic Go): quota, ownership, template/plan catalog, node capacity, risk classification (read/low/medium/high/destructive), destructive requires typed confirmation (resource name).
- Execution engine in Go: DAG scheduler, Postgres-leased tasks (`FOR UPDATE SKIP LOCKED` + lease expiry → resume after restart/replica crash), idempotency keys per task, bounded retries with backoff, timeouts, cancel, partial completion (no auto-destroy; offer retry/diagnose/keep/delete-with-approval).
- Adapters/tool registry over existing usecases (instance create/start/stop/delete, game server, android, n8n, openclaw, bucket, domain) with declared schema/risk/idempotency/timeout; MCP gateway reuses the same registry server-side.
- Verification engine: instance exists + running + IP (Proxmox status / guest agent), Docker viewer reuse for app checks, HTTP health for domains; results persisted; unverified marked.
- API (keep existing `/api/v1/ai/*`, add): `POST/GET/PATCH /ai/plans/:id`, `/validate`, `/approve` (version-bound), `/reject`, `/execute`, `GET /ai/executions/:id`, `/events` (JSON + SSE), `/cancel`, `POST /ai/executions/:id/tasks/:taskId/retry`.
- UI: AI workspace (history | chat + execution timeline | plan/approval panel), plan diff on revision, approval modal with risk + consequences, typed confirmation for destructive, live timeline via SSE, final report.
- Acceptance: PRD Scenarios A, C, D end to end + restart-resume test + duplicate-retry test; existing gates (routes additions only, shapes unchanged for old endpoints, smoke).
- Phases 2–4 (multi-service templates, diagnostics, monitoring/notifications, optimization, multi-agent roles) after Phase 1 + backups.

---

## 7. Known limits / follow-ups
- Not tested against a real Proxmox host or Kubernetes cluster (commands marked in the tutorial).
- Provisioning jobs run in-process: a crashed/killed replica fails its running job (retry in Admin → Jobs).
  Compose has no rolling update; k8s uses preStop + 90 s grace.
- When a replica dies abruptly, one in-flight request can get 502/504.
- Real OAuth redirect flow is not implemented in the UI (social login is demo-only).
- `FileManager` download URL appends `?token=` (ignored by the API; cookie GET auth is used) — remove.
- WordPress bridge with a valid key can control any instance (by design; key = full trust).
- Guest-agent IP spoofing: real fix is Proxmox firewall `ipfilter` + static per-VM IPs (IPAM).
- `GetNextVMID` race across replicas could make a clone fail (retry).

## 8. Suggested commit split (when the user asks to commit)
(If §0 checkpoints already landed on `main`, this split only matters for a cleaned-up history.)
1. `fix(security)`: OAuth/WordPress gating, CORS, cookies, authz fixes.
2. `feat(config)`: demo/production modes, validation, seeding, advisory lock.
3. `feat(docker)`: guest-agent exec + Docker viewer (API + UI).
4. `refactor(backend)`: repository/usecase/request/response/handler layers.
5. `feat(web)`: R640 landing drawing, demo gating, code splitting.
6. `feat(deploy)`: Dockerfiles, compose modes, deploy.sh, LB, k8s, tutorial.
7. `feat(domains)` and later `feat(backups)` as separate commits.
Commit messages end with: `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
