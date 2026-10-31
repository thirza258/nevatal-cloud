---
name: nevatal-cloud
description: Discover and orchestrate Nevatal Cloud virtual compute, game servers, Android containers, and S3 storage
---

# Nevatal Cloud Agent Skill

Nevatal Cloud is a developer-first bare-metal cloud platform at https://cloud.nevatal.id.
It provides virtual machines (KVM), game servers (Minecraft Java/Bedrock, Terraria), containerized Android (Waydroid LXC with WebRTC streaming and ADB), Git app hosting, n8n automations, S3 object storage, and Model Context Protocol (MCP) integrations.

## Discovery Documents

- Documentation index: https://cloud.nevatal.id/docs
- Full LLM text corpus: https://cloud.nevatal.id/llms-full.txt
- Machine-readable llms.txt: https://cloud.nevatal.id/llms.txt
- Sitemap: https://cloud.nevatal.id/sitemap.xml
- API catalog: https://cloud.nevatal.id/.well-known/api-catalog
- A2A Agent card: https://cloud.nevatal.id/.well-known/agent-card.json
- MCP Server card: https://cloud.nevatal.id/.well-known/mcp/server-card.json
- AI Catalog: https://cloud.nevatal.id/.well-known/ai-catalog.json

## Authentication & API Access

The control plane REST API is hosted under `https://cloud.nevatal.id/api/v1/`.
Obtain a JWT token via:

```bash
curl -X POST https://cloud.nevatal.id/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"developer@example.com","password":"your-password"}'
```

Pass the token in the `Authorization: Bearer <TOKEN>` header on all subsequent requests.

## Key API Endpoints

- Health: `GET /api/v1/health`
- Compute VMs: `GET /api/v1/instances` | `POST /api/v1/instances`
- VM Power: `POST /api/v1/instances/:id/power` (`action`: "start" | "stop" | "reboot")
- Game Servers: `GET /api/v1/game-servers` | `POST /api/v1/game-servers`
- Android Cloud: `GET /api/v1/android` | `POST /api/v1/android`
- S3 Buckets: `GET /api/v1/storage/buckets`
- Quotas: `GET /api/v1/quotas`
- MCP Server: `https://cloud.nevatal.id/api/v1/mcp`
