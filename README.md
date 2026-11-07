# Nevatal Cloud 🚀
> Self-hosted cloud infrastructure and virtual server provisioning platform powered by **Proxmox VE**.

[![Platform](https://img.shields.io/badge/Platform-Proxmox%20VE-orange?logo=proxmox)](https://proxmox.com)
[![Hardware](https://img.shields.io/badge/Hardware-Dell%20PowerEdge%20R640-blue)](#)
[![Backend](https://img.shields.io/badge/Backend-Go%201.23%2B-00ADD8?logo=go)](https://golang.org)
[![Frontend](https://img.shields.io/badge/Frontend-React%2019%20%2B%20TypeScript%20%2B%20Tailwind-61DAFB?logo=react)](https://react.dev)

---

## 1. Executive Summary

**Nevatal Cloud** (`cloud.nevatal.id`) is a self-service cloud VPS platform that allows users to create, configure, deploy, and manage Linux virtual machines through a developer-friendly web control plane.

Built specifically to orchestrate hypervisor operations on a **Dell PowerEdge R640** running Proxmox VE, Nevatal Cloud abstracts away the complexity of the Proxmox administrative UI while enforcing multi-tenant isolation, user quotas, transactional resource reservations, and automated cloud-init provisioning.

---

## 2. Architecture Overview

```
                      +-----------------------------------+
                      |   Nevatal Cloud Web Dashboard     |
                      |   React 19 + TypeScript + Vite    |
                      +-----------------+-----------------+
                                        | (REST API / Bearer JWT)
                                        v
                      +-----------------+-----------------+
                      |    Nevatal API & Worker Engine    |
                      |          Go (Golang)              |
                      +----+------------------------+-----+
                           |                        |
         (Authoritative State)           (Asynchronous Hypervisor Tasks)
                           |                        |
                           v                        v
          +----------------+----+         +---------+---------+
          |  PostgreSQL 16 DB   |         |    Proxmox VE     |
          |  (Users, Quotas,    |         |   REST API v2     |
          |  Instances, Audits) |         | (Dell R640 Node)  |
          +---------------------+         +-------------------+
```

### Core Components

1. **Frontend Control Plane (`/frontend`)**:
   - Modern developer interface inspired by Hetzner Cloud and DigitalOcean.
   - Built with **React 19**, **TypeScript**, **Tailwind CSS**, and **Lucide Icons**.
   - Customer Dashboard: VM inventory, SSH keys, 5-step deployment wizard, live guest metrics, and usage bars.
   - Administrator Dashboard: User management, quota governance, hypervisor telemetry, queue inspector, and audit trail.
2. **API & Asynchronous Worker (`/backend`)**:
   - High-throughput Go service.
   - Transactional quota enforcement and resource reservation engine.
   - Asynchronous worker pipeline that clones templates, configures hardware, provisions cloud-init, starts VMs, and reconciles IP addresses.
3. **Proxmox VE REST Adapter (`internal/proxmox`)**:
   - Production HTTP client communicating with Proxmox VE REST API v2 (`/api2/json`).
   - Token authentication via `PVEAPIToken=USER@REALM!TOKENID=SECRET`.
   - **Simulation / Mock Mode**: Integrated mock hypervisor engine enabled when `PROXMOX_MOCK=true`, allowing local end-to-end development, testing, and UI evaluation without requiring immediate hypervisor connectivity.
4. **Database Migrations (`backend/migrations/`)**:
   - Versioned PostgreSQL 16 schema migrations managed via **go-migrate** (`golang-migrate/migrate`).
   - Supports automated schema versioning, rollback, and deployment automation via `./deploy.sh`.

---

## 3. Quickstart with Docker Compose (local development)

To spin up the complete platform locally (PostgreSQL, Go API, nginx + React UI) with the
simulated hypervisor and the public demo accounts:

```bash
# 1. Clone repository
git clone https://github.com/thirza258/nevatal-cloud.git
cd nevatal-cloud

# 2. Development environment (dev-only, public values)
cp .env.example .env

# 3. Build and start containers
docker compose up -d --build
```

Access the platform:
- **Web Dashboard**: [http://localhost:3080](http://localhost:3080)
- **Health (through nginx)**: [http://localhost:3080/health](http://localhost:3080/health)
- **API (direct, development only, loopback)**: [http://127.0.0.1:8090/health](http://127.0.0.1:8090/health)

Redis is not used by the application yet; it is only started with `COMPOSE_PROFILES=redis`.

### Default Seed Accounts (demo / development mode only)

In **demo** and **development** mode the platform bootstraps these public seed accounts:

| Role | Email | Password | Pre-configured Quota |
|---|---|---|---|
| **Administrator** | `admin@nevatal.id` | `AdminPass123!` | 32 vCPU, 64 GB RAM, 1000 GB SSD, 20 VMs |
| **Developer / Customer** | `demo@nevatal.id` | `DemoPass123!` | 4 vCPU, 8 GB RAM, 100 GB SSD, 3 VMs |

*(Tip: in demo mode the sidebar shows a 1-click **"Role Switcher (Demo)"** to toggle between Admin and Customer roles instantly. Production mode has no demo accounts and no role switcher: the first administrator is created from `ADMIN_EMAIL` / `ADMIN_PASSWORD`, and leftover demo accounts with public passwords are suspended.)*

### Deployment: demo vs production

Servers are deployed with [`./deploy.sh`](deploy.sh), which needs only Docker Engine + the Compose v2 plugin (>= 2.24). It validates `.env`, builds the images, runs the database migrations in a container, starts the stack and waits for `/health` through nginx.

| Mode | Use | Hypervisor | Command |
|---|---|---|---|
| **demo** | Public showcase (`demo.nevatal.id`) | Simulated, always (1 API replica) | `cp deploy/env/demo.env.example .env` then `./deploy.sh --mode demo` |
| **production** | Real service (`cloud.nevatal.id`) | Proxmox VE via a least-privilege API token | `cp deploy/env/production.env.example .env` then `./deploy.sh --mode production` |

```bash
./deploy.sh --status          # containers, schema version, runtime mode, edge + tunnel state
./deploy.sh --migrate-only    # apply migrations only
./deploy.sh --rollback        # stop the API and revert one migration
./deploy.sh --reset-demo --yes   # demo only: wipe demo data (nightly cron)
```

- **Full guide**: [docs/PROXMOX_INSTALL.md](docs/PROXMOX_INSTALL.md) - Proxmox role/token, cloud-init templates with qemu-guest-agent, control-plane VM, TLS, Cloudflare Tunnel and subdomains, firewall, backups, upgrades, troubleshooting (Proxmox VE 8.x and 9.x).
- **Load balancing**: nginx in the frontend container balances `/api` (least_conn) across `BACKEND_REPLICAS` API replicas (production default 2). Several VMs: HAProxy + keepalived in [`deploy/loadbalancer/`](deploy/loadbalancer/) with [`deploy/compose/multi-vm.yml`](deploy/compose/multi-vm.yml).
- **HTTPS**: Caddy (compose profile `tls`, [`deploy/caddy/Caddyfile`](deploy/caddy/Caddyfile)), a Cloudflare Tunnel (compose profile `tunnel`: `cloudflared` with `CLOUDFLARE_TUNNEL_TOKEN`, no public IP or inbound ports needed), or your own reverse proxy.
- **Railway-style subdomains** (production only): customers publish a VM port at `https://<name>.<PUBLIC_DOMAIN_ROOT>`. The `edge` proxy (same backend image, `/app/nevatal-edge`, compose profile `edge`) routes each name to the VM's private IP:port behind the tunnel; it needs a NIC/route into the customer VM network and `EDGE_TARGET_CIDRS` / `EDGE_DENY_IPS`. Use a separate registrable domain for tenant apps (e.g. `nevatal.app`). Set `ADCONTROL_SITE_KEY` to inject a labelled AdControl placement into the HTML pages the edge proxies (empty = off); see [docs/EDGE_ADS.md](docs/EDGE_ADS.md). Setup, security notes and Cloudflare limits: [section 20 of the guide](docs/PROXMOX_INSTALL.md#20-cloudflare-tunnel-and-railway-style-subdomains-nevatalid).
  ```bash
  # .env (production): COMPOSE_PROFILES=edge,tunnel  CLOUDFLARE_TUNNEL_TOKEN=eyJ...
  #   PUBLIC_DOMAIN_ROOT=nevatal.app  EDGE_TARGET_CIDRS=10.20.0.0/24  EDGE_DENY_IPS=10.20.0.1,10.20.0.5
  ./deploy.sh --mode production
  ```
- **Kubernetes (k3s)**: kustomize base + `demo` / `production` overlays (production adds the subdomain edge from [`deploy/k8s/components/edge`](deploy/k8s/components/edge/)) and MetalLB / cert-manager / Traefik / cloudflared add-ons in [`deploy/k8s/`](deploy/k8s/) (`kubectl apply -k deploy/k8s/overlays/production`).

---

## 4. Connecting your Proxmox VE Server

Summary of [docs/PROXMOX_INSTALL.md](docs/PROXMOX_INSTALL.md) for your physical **Dell PowerEdge R640** (or any Proxmox VE 8.x / 9.x server).

### Step 4.1: Create a least-privilege API token

Do not use `PVEVMAdmin` or a token without privilege separation. Create a dedicated role with only the privileges Nevatal calls (the guide lists every API call and its privilege). On **Proxmox VE 9.x**:

```bash
pveum role add NevatalCloud --privs "VM.Allocate VM.Audit VM.Clone VM.Config.CPU VM.Config.Cloudinit VM.Config.Disk VM.Config.Memory VM.Config.Options VM.Console VM.PowerMgmt VM.Snapshot VM.Snapshot.Rollback VM.GuestAgent.Audit Datastore.Audit Datastore.AllocateSpace Sys.Audit SDN.Use"
pveum user add nevatal@pve --comment "Nevatal Cloud control plane"
pveum user token add nevatal@pve control-plane --privsep 1   # copy the secret
```

On **Proxmox VE 8.x** replace `VM.GuestAgent.Audit` with `VM.Monitor`. The **Docker viewer** (container list and `docker logs` of a VM through the QEMU guest agent) additionally needs `VM.GuestAgent.Unrestricted` on PVE 9 (`VM.Monitor` already covers it on PVE 8): it allows running any command as root inside guests, so grant it deliberately. Then add the ACLs for the user **and** the token on `/vms`, `/storage/<storage>`, `/nodes/<node>` and `/sdn/zones/localnetwork/<bridge>` (see the guide, section 4).

### Step 4.2: Prepare Cloud-Init Templates

Nevatal provisions VMs by full-cloning templates 9000 (Ubuntu 24.04), 9001 (Debian 12), 9002 (Alpine) and 9003 (WordPress on Ubuntu). Each template needs a 20 GiB `scsi0` disk, a cloud-init drive, `--agent enabled=1` **and qemu-guest-agent installed inside the image** (used for IP discovery and the Docker viewer):

```bash
# On the Proxmox host
apt install -y libguestfs-tools
wget https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
virt-customize -a noble-server-cloudimg-amd64.img --install qemu-guest-agent --truncate /etc/machine-id
qm create 9000 --name tpl-ubuntu-2404 --ostype l26 --cores 2 --memory 2048 --cpu host \
  --net0 virtio,bridge=vmbr0 --scsihw virtio-scsi-single \
  --scsi0 local-zfs:0,import-from=$PWD/noble-server-cloudimg-amd64.img,discard=on,ssd=1 \
  --ide2 local-zfs:cloudinit --boot order=scsi0 --serial0 socket --vga serial0 --agent enabled=1
qm disk resize 9000 scsi0 20G
qm template 9000
```

### Step 4.3: Configure the production `.env`

```bash
cp deploy/env/production.env.example .env && chmod 600 .env
```

```dotenv
PROXMOX_MOCK=false
PROXMOX_HOST=192.168.1.100          # IP of your Proxmox server
PROXMOX_PORT=8006
PROXMOX_TOKEN_ID=nevatal@pve!control-plane
PROXMOX_TOKEN_SECRET=<your-token-secret>
PROXMOX_NODE_NAME=pve-dell-r640     # Proxmox node name
PROXMOX_STORAGE=local-zfs           # Target storage pool
PROXMOX_VERIFY_TLS=false            # Set true if using valid SSL
```

Fill in the remaining `CHANGE_ME` values (`JWT_SECRET`, `POSTGRES_PASSWORD`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`), then deploy:

```bash
./deploy.sh --mode production
```

---

## 5. OpenRouter AI Assistant & DevOps Copilot 🤖

Nevatal Cloud features an integrated AI Assistant powered by **OpenRouter** designed to act as your autonomous cloud copilot:

- **1-Click Container & VM Provisioning**: Prompt the assistant in natural language (e.g. *"Create an Ubuntu 24.04 server with 2 vCPUs and 4GB RAM named web-01"*), and it will automatically check quotas, select templates, and generate a 1-click **Deploy Container Now** action.
- **24/7 Platform Support & Troubleshooting**: Answers questions regarding SSH connections, cloud-init logs, network configuration, Proxmox VE Dell PowerEdge R640 architecture, and container states.
- **Global Floating Widget & Dedicated Page**: Accessible globally via the floating AI widget in the bottom-right corner or on the full dashboard page at `/dashboard/ai-assistant`.
- **OpenRouter Multi-Provider Routing**: Choose between frontier models including `deepseek/deepseek-chat`, `anthropic/claude-3.5-sonnet`, `openai/gpt-4o-mini`, `meta-llama/llama-3.3-70b-instruct`, or supply your own custom OpenRouter API key directly in Assistant Settings.
- **Zero-Config Fallback Mode**: If no OpenRouter API key is set, the assistant runs in local DevOps fallback mode, providing recommendations and 1-click container provisioning without crashing.

### Configure System OpenRouter Key in `.env`:
```dotenv
OPENROUTER_API_KEY=sk-or-v1-xxxxxxxxxxxxxxxxxxxx
OPENROUTER_DEFAULT_MODEL=openai/gpt-4o-mini
```

---

## AI Infrastructure Orchestrator

The **AI Cloud Agent** (`/dashboard/ai`) runs on an AI-native control plane: you describe what you want, the planner proposes a versioned plan, the policy engine validates it, **you** approve that exact revision, the execution engine runs it as durable tasks in PostgreSQL (resumed after crashes and rolling updates), and the verification engine checks the result on Proxmox. The model is never the authority: chat text, pasted JSON and MCP tool calls can at most lead to a plan, never approve one.

- **Phase 1** covers VPS create/start/stop/restart/resize/snapshot/delete, buckets, apps, n8n, OpenClaw, game servers and Android instances, with a live execution timeline (SSE), verification and recovery options for partial failures.
- **Approvals by risk**: `medium` plans need your approval of the exact revision, `high` plans also a risk acknowledgement, `destructive` plans (delete, snapshot restore) also the typed resource name. Approvals expire after 60 min (15 min for destructive plans).
- **Guide** for users, the security model and operators: [docs/ORCHESTRATOR.md](docs/ORCHESTRATOR.md). **API contract** (frozen for Phase 1): [docs/ORCHESTRATOR_API.md](docs/ORCHESTRATOR_API.md).

---

## 6. Local Development (Without Docker)

### Backend (Go)
```bash
cd backend
export DATABASE_URL="postgres://postgres:postgrespassword@localhost:5435/nevatal_cloud?sslmode=disable"
export PROXMOX_MOCK=true
go run ./cmd/api
```

### Frontend (React + Vite)
```bash
cd frontend
npm install
npm run dev
```

---

## 6. API Reference

All customer and administrative endpoints use the versioned prefix `/api/v1`.

### Authentication
- `POST /api/v1/auth/register` — Create new account
- `POST /api/v1/auth/login` — Sign in & receive session token
- `POST /api/v1/auth/logout` — Revoke session
- `GET /api/v1/auth/me` — Current authenticated user profile

### Instances & Lifecycle
- `GET /api/v1/dashboard` — Overview summary, quota metrics, and recent instances
- `GET /api/v1/instances` — List user's instances (supports `?status=running`)
- `POST /api/v1/instances` — Enqueue VM creation (supports `X-Idempotency-Key`, returns `202 Accepted`)
- `GET /api/v1/instances/:id` — Instance specs, IP address, and SSH command
- `POST /api/v1/instances/:id/start` — Boot stopped VM
- `POST /api/v1/instances/:id/shutdown` — Graceful ACPI guest shutdown
- `POST /api/v1/instances/:id/restart` — Graceful reboot
- `DELETE /api/v1/instances/:id` — Destroy VM & release resource quota
- `GET /api/v1/instances/:id/metrics` — Live guest CPU %, Memory MB, and Uptime
- `GET /api/v1/jobs/:id` — Poll asynchronous provisioning progress & checklist

### Catalogs & SSH Keys
- `GET /api/v1/templates` — Approved OS templates (Ubuntu 24.04 LTS, Debian 12/13, Alpine)
- `GET /api/v1/plans` — Preset plans (Nano, Starter, Developer, Pro)
- `GET /api/v1/quotas` / `GET /api/v1/usage` — Live quota breakdown
- `GET /api/v1/ssh-keys` — List registered SSH public keys
- `POST /api/v1/ssh-keys` — Add SSH public key (calculates SHA256 fingerprint)
- `DELETE /api/v1/ssh-keys/:id` — Remove SSH key

### Infrastructure Administration (Admin Role)
- `GET /api/v1/admin/overview` — Cluster health, hardware load, active/failed jobs
- `GET /api/v1/admin/users` — User directory with quota usage
- `PATCH /api/v1/admin/users/:id` — Suspend or restore user account
- `PUT /api/v1/admin/users/:id/quotas` — Update user resource limits
- `GET /api/v1/admin/instances` — Cross-tenant global VM inventory
- `GET /api/v1/admin/jobs` — Real-time queue inspector
- `POST /api/v1/admin/jobs/:id/retry` — Retry failed provisioning job
- `GET /api/v1/admin/nodes` — Dell PowerEdge R640 node telemetry & ZFS pool status
- `GET /api/v1/admin/audit-logs` — Immutable audit log stream

### Nevatal AI Cloud Agent & MCP (PRD Sections 33-48)
- `GET /api/v1/ai/conversations` — List conversation threads
- `POST /api/v1/ai/conversations` — Create new AI agent conversation
- `GET /api/v1/ai/conversations/:id` — Conversation history & state
- `DELETE /api/v1/ai/conversations/:id` — Delete conversation
- `POST /api/v1/ai/conversations/:id/messages` — Send user message and execute dynamic requirement gathering
- `GET /api/v1/ai/conversations/:id/messages` — Message history with interactive UI cards
- `GET /api/v1/ai/conversations/:id/state` — Real-time workflow state machine status
- `GET /api/v1/ai/conversations/:id/plans` — List structured multi-resource plans
- `GET /api/v1/ai/plans/:id` — Plan details and resource allocations
- `POST /api/v1/ai/plans/:id/validate` — Dry-run schema and quota validation
- `POST /api/v1/ai/plans/:id/approve` — Approve plan and initiate automated provisioning
- `POST /api/v1/ai/plans/:id/cancel` — Cancel pending plan
- `GET /api/v1/resources` — Unified inventory across compute VMs, apps, n8n, OpenClaw, and S3
- `GET /api/v1/integrations/mcp/servers` — List registered MCP servers
- `GET /api/v1/integrations/mcp/connections` — List tenant MCP connections
- `POST /api/v1/integrations/mcp/connections` — Authorize MCP server connection
- `DELETE /api/v1/integrations/mcp/connections/:id` — Disconnect MCP server
- `GET /api/v1/integrations/mcp/connections/:id/tools` — Connection tools and granted permissions
- `PATCH /api/v1/integrations/mcp/connections/:id/permissions` — Toggle tool permission
- `GET /api/v1/integrations/mcp/audit-logs` — Immutable MCP tool invocation audit trail
- `POST /api/v1/mcp/rpc` — Model Context Protocol JSON-RPC 2.0 endpoint (`tools/list`, `tools/call`)
- AI Infrastructure Orchestrator (plan revisions, version-bound approvals, executions, live events, recovery, tool registry) — see [docs/ORCHESTRATOR_API.md](docs/ORCHESTRATOR_API.md)

---

## 7. Verification & Testing

Run the automated backend test suite:
```bash
cd backend
go test -v ./...
```
Output:
```text
=== RUN   TestMockProxmoxClient
--- PASS: TestMockProxmoxClient (0.00s)
=== RUN   TestAuthTokenAndHashing
--- PASS: TestAuthTokenAndHashing (0.22s)
=== RUN   TestSSHKeyValidation
--- PASS: TestSSHKeyValidation (0.00s)
PASS
ok      github.com/nevatal/cloud/internal/proxmox   0.005s
ok      github.com/nevatal/cloud/internal/services  0.222s
```

Run frontend build verification:
```bash
cd frontend
npm run build
```

---

## 8. License & Maintenance

Developed by **Nevatal** for self-hosted cloud infrastructure deployments on Proxmox Virtual Environment.
Licensed under the MIT License.
