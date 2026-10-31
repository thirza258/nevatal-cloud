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
4. **Data Store (`scripts/init-db.sql`)**:
   - PostgreSQL 16 schema strictly following PRD Section 19.

---

## 3. Quickstart with Docker Compose

To spin up the complete platform (PostgreSQL, Redis, Go API/Worker, and Frontend):

```bash
# 1. Clone repository
git clone https://github.com/nevatal/cloud.git
cd cloud

# 2. Review and configure environment
cp .env.example .env

# 3. Start containers
docker compose up -d --build
```

Access the platform:
- **Web Dashboard**: [http://localhost:3080](http://localhost:3080)
- **API Endpoint**: [http://localhost:8090/health](http://localhost:8090/health)

### Default Seed Accounts

The platform automatically bootstraps with initial seed accounts:

| Role | Email | Password | Pre-configured Quota |
|---|---|---|---|
| **Administrator** | `admin@nevatal.id` | `AdminPass123!` | 32 vCPU, 64 GB RAM, 1000 GB SSD, 20 VMs |
| **Developer / Customer** | `demo@nevatal.id` | `DemoPass123!` | 4 vCPU, 8 GB RAM, 100 GB SSD, 3 VMs |

*(Tip: In the web interface, use the 1-click **"Role Switcher (Demo)"** in the sidebar to toggle between Admin and Customer roles instantly).*

---

## 4. Connecting your Proxmox VE Server

When you are ready to connect to your physical **Dell PowerEdge R640** (or any Proxmox VE server):

### Step 4.1: Create an API Token in Proxmox VE
1. Log into your Proxmox Web GUI (`https://<proxmox-ip>:8006`).
2. Go to **Datacenter** -> **Permissions** -> **Users** -> **Add**:
   - User: `nevatal-cloud`
   - Realm: `Proxmox VE authentication server` (pve)
3. Go to **Datacenter** -> **Permissions** -> **API Tokens** -> **Add**:
   - User: `nevatal-cloud@pve`
   - Token ID: `api-token`
   - **Uncheck** "Privilege Separation" (or configure explicit permissions below).
   - Copy the generated **Secret**.
4. Go to **Datacenter** -> **Permissions** -> **Add** -> **User Permission**:
   - Path: `/`
   - User: `nevatal-cloud@pve`
   - Role: `PVEVMAdmin` and `PVEDatastoreUser`

### Step 4.2: Prepare a Cloud-Init Template
Nevatal Cloud provisions VMs by cloning a validated Proxmox template with Cloud-Init:
```bash
# On Proxmox host via SSH:
wget https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
qm create 9000 --name "ubuntu-2404-template" --memory 2048 --cores 2 --net0 virtio,bridge=vmbr0
qm importdisk 9000 noble-server-cloudimg-amd64.img local-zfs
qm set 9000 --scsihw virtio-scsi-pci --scsi0 local-zfs:vm-9000-disk-0
qm set 9000 --boot c --bootdisk scsi0
qm set 9000 --ide2 local-zfs:cloudinit
qm set 9000 --serial0 socket --vga serial0
qm set 9000 --agent enabled=1
qm template 9000
```

### Step 4.3: Configure `.env`
Update `.env` on your Nevatal Cloud host:
```dotenv
PROXMOX_MOCK=false
PROXMOX_HOST=192.168.1.100          # IP of your Proxmox server
PROXMOX_PORT=8006
PROXMOX_TOKEN_ID=nevatal-cloud@pve!api-token
PROXMOX_TOKEN_SECRET=<your-token-secret>
PROXMOX_NODE_NAME=pve-dell-r640     # Proxmox node name
PROXMOX_STORAGE=local-zfs           # Target storage pool
PROXMOX_VERIFY_TLS=false            # Set true if using valid SSL
```

Restart backend:
```bash
docker compose restart backend
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
