#!/usr/bin/env python3
"""Record the JSON *shape* (keys + JSON types, not values) of every read-only API
endpoint, so a refactor can be checked for response-contract regressions.

Usage: shape_snapshot.py <base_url> <out.json>
Requires a fresh DEMO_MODE database (demo@nevatal.id / admin@nevatal.id seeded).
"""
import json
import sys
import urllib.error
import urllib.request

BASE = sys.argv[1].rstrip("/") + "/api/v1"
OUT = sys.argv[2]


def call(method, path, token=None, body=None):
    req = urllib.request.Request(BASE + path, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    data = json.dumps(body).encode() if body is not None else None
    try:
        with urllib.request.urlopen(req, data, timeout=30) as resp:
            return resp.status, json.loads(resp.read() or b"null")
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, json.loads(raw or b"null")
        except ValueError:
            return e.code, raw.decode(errors="replace")


def shape(v, depth=0):
    if depth > 8:
        return "…"
    if isinstance(v, dict):
        return {k: shape(v[k], depth + 1) for k in sorted(v)}
    if isinstance(v, list):
        return [shape(v[0], depth + 1)] if v else []
    if v is None:
        return "null"
    if isinstance(v, bool):
        return "bool"
    if isinstance(v, (int, float)):
        return "number"
    return "string"


def first_id(token, path, key="id"):
    _, body = call("GET", path, token)
    try:
        data = body["data"]
        if isinstance(data, dict):
            for k in ("items", "instances", "servers", "buckets"):
                if isinstance(data.get(k), list):
                    data = data[k]
                    break
        return data[0][key]
    except Exception:
        return None


def login(email, password):
    _, body = call("POST", "/auth/login", body={"email": email, "password": password})
    return body["data"]["token"]


cust = login("demo@nevatal.id", "DemoPass123!")
admin = login("admin@nevatal.id", "AdminPass123!")

inst = first_id(cust, "/instances")
app = first_id(cust, "/apps")
n8n = first_id(cust, "/automation/instances")
oc = first_id(cust, "/ai-agents/openclaw/instances")
bucket = first_id(cust, "/storage/buckets")
gs = first_id(cust, "/game-servers")
andr = first_id(cust, "/android/instances")
aimg = first_id(cust, "/android/images")

customer_paths = [
    "/auth/me", "/dashboard", "/quotas", "/usage", "/activity", "/templates", "/plans",
    "/operating-systems", "/ssh-keys", "/system/config", "/resources",
    "/instances", "/instances?status=running",
    f"/instances/{inst}", f"/instances/{inst}/activity", f"/instances/{inst}/metrics",
    f"/instances/{inst}/snapshots", f"/instances/{inst}/console",
    f"/instances/{inst}/docker", f"/instances/{inst}/docker?stats=true",
    "/apps", f"/apps/{app}", f"/apps/{app}/env",
    "/automation/instances", f"/automation/instances/{n8n}", f"/automation/instances/{n8n}/logs",
    "/ai-agents/openclaw/instances", f"/ai-agents/openclaw/instances/{oc}", f"/ai-agents/openclaw/instances/{oc}/logs",
    "/ai-assistant/context", "/ai-assistant/history", "/ai-assistant/models",
    "/storage/overview", "/storage/buckets", f"/storage/buckets/{bucket}", f"/storage/buckets/{bucket}/objects",
    "/storage/access-keys", "/storage/shared-links",
    "/ai/conversations", "/integrations/mcp/servers", "/integrations/mcp/connections", "/integrations/mcp/audit-logs",
    "/game-servers", "/game-servers/templates", "/game-servers/versions", "/game-servers/plans",
    f"/game-servers/{gs}", f"/game-servers/{gs}/status", f"/game-servers/{gs}/metrics", f"/game-servers/{gs}/logs",
    f"/game-servers/{gs}/configuration", f"/game-servers/{gs}/players", f"/game-servers/{gs}/plugins",
    f"/game-servers/{gs}/mods", f"/game-servers/{gs}/worlds", f"/game-servers/{gs}/backups",
    "/android/instances", "/android/plans", "/android/images", f"/android/images/{aimg}",
    f"/android/instances/{andr}", f"/android/instances/{andr}/sessions", f"/android/instances/{andr}/metrics",
    f"/android/instances/{andr}/logs", f"/android/instances/{andr}/applications", f"/android/instances/{andr}/backups",
    "/wordpress/status", "/auth/oauth/urls",
]
admin_paths = [
    "/admin/overview", "/admin/users", "/admin/instances", "/admin/jobs", "/admin/nodes",
    "/admin/audit-logs", "/admin/templates", "/admin/plans", "/admin/isos", "/admin/android/images",
    f"/admin/instances/{inst}/docker",
]

# Normalize the ids out of the recorded path names so runs are comparable.
ids = {v: k for k, v in {"{inst}": inst, "{app}": app, "{n8n}": n8n, "{oc}": oc, "{bucket}": bucket,
                          "{gs}": gs, "{andr}": andr, "{aimg}": aimg}.items() if v}


def norm(path):
    for real, name in ids.items():
        path = path.replace(str(real), name)
    return path


result = {}
for who, token, paths in (("customer", cust, customer_paths), ("admin", admin, admin_paths)):
    for p in paths:
        status, body = call("GET", p, token)
        result[f"{who} GET {norm(p)}"] = {"status": status, "shape": shape(body)}

# A few error contracts.
for label, (method, path, token, body) in {
    "unauth": ("GET", "/instances", None, None),
    "bad-id": ("GET", "/instances/not-a-uuid", cust, None),
    "login-fail": ("POST", "/auth/login", None, {"email": "demo@nevatal.id", "password": "wrong"}),
    "admin-forbidden": ("GET", "/admin/users", cust, None),
}.items():
    status, resp = call(method, path, token, body)
    result[f"error {label}"] = {"status": status, "shape": shape(resp)}

with open(OUT, "w") as f:
    json.dump(result, f, indent=1, sort_keys=True)
print(f"wrote {len(result)} endpoint shapes to {OUT}")
