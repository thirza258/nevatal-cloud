#!/usr/bin/env python3
"""AI Infrastructure Orchestrator (Phase 1) end-to-end test.

Builds the API, runs it against throwaway databases nevatal_orch_e2e_* on the
QA Postgres container (nevatal-smoke-pg, 127.0.0.1:55433) with the simulated
hypervisor (PROXMOX_MOCK=true, DEMO_MODE=false, ADMIN_* set) on ports
18290-18299, and checks PRD scenarios end to end over HTTP:

  A  VPS: chat message -> plan -> validate -> approve(version, spec_hash)
     -> execution -> verification -> report; SSE stream receives every event
     and terminates after execution.finished
  C  delete VPS: destructive plan, typed confirmation, executed through the
     instance service, verified, audited
  D  forced failure after the VPS is created (ORCH_FAULTS, mock only)
     -> partially_completed, recovery options, delete_created makes a new
     destructive plan, retry_failed completes the execution
  R  restart-resume: the API is killed (SIGKILL) while vps.create runs; after
     a restart the task is re-claimed, the stale job is reaped and retried;
     the execution completes with one instance row per idempotency key and
     terminal jobs
  M  MCP: initialize, tools/list from the registry, write tool -> plan only
  S  SIGTERM while a task runs: open SSE streams end within 3 s, the process
     exits cleanly, a restarted API finishes the execution

Usage: python3 scripts/qa/orchestrator_e2e.py
"""
import http.client
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request

QA = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(QA))
BACKEND = os.path.join(ROOT, "backend")
BIN = os.path.join(QA, "nevatal-api-orch-e2e")
PG = "postgres://postgres:smokepw@127.0.0.1:55433/{db}?sslmode=disable"
ADMIN_EMAIL, ADMIN_PASSWORD = "ops@e2e.example.com", "OpsE2EPassword123!"
TERMINAL = ("completed", "failed", "partially_completed", "cancelled")

PASS = FAIL = 0


def check(name, cond, detail=""):
    global PASS, FAIL
    if cond:
        PASS += 1
        print(f"  PASS {name}")
    else:
        FAIL += 1
        print(f"  FAIL {name} {detail}")
    return cond


def psql(db, sql):
    out = subprocess.run(["docker", "exec", "nevatal-smoke-pg", "psql", "-U", "postgres", "-d", db, "-tAc", sql],
                         capture_output=True, text=True, check=True)
    return out.stdout.strip()


def reset_db(name):
    psql("postgres", f"DROP DATABASE IF EXISTS {name} WITH (FORCE)")
    psql("postgres", f"CREATE DATABASE {name}")


class API:
    def __init__(self, port, db, **env):
        self.port, self.db, self.env, self.proc = port, db, env, None
        self.base = f"http://127.0.0.1:{port}/api/v1"
        self.log = os.path.join(QA, f"orch-e2e-{port}.log")

    def start(self):
        env = {
            "PATH": os.environ["PATH"], "HOME": os.environ.get("HOME", "/tmp"),
            "APP_ENV": "development", "DEMO_MODE": "false", "PROXMOX_MOCK": "true",
            "ADMIN_EMAIL": ADMIN_EMAIL, "ADMIN_PASSWORD": ADMIN_PASSWORD,
            "DATABASE_URL": PG.format(db=self.db), "PORT": str(self.port),
            "STORAGE_PATH": os.path.join(QA, "storage-orch-e2e"),
            "ORCH_POLL_MS": "200", "ORCH_LEASE_SECONDS": "6", "ORCH_HEARTBEAT_SECONDS": "2",
            "ORCH_RETRY_BASE_MS": "300",
        }
        env.update({k: str(v) for k, v in self.env.items()})
        logf = open(self.log, "a")
        self.proc = subprocess.Popen([BIN], cwd=QA, env=env, stdout=logf, stderr=subprocess.STDOUT)
        for _ in range(80):
            try:
                urllib.request.urlopen(f"http://127.0.0.1:{self.port}/health", timeout=1)
                return
            except Exception:
                time.sleep(0.25)
        raise RuntimeError(f"API on {self.port} did not start; see {self.log}")

    def kill(self):
        if self.proc:
            self.proc.kill()
            self.proc.wait()
            self.proc = None

    def stop(self):
        if self.proc:
            self.proc.terminate()
            try:
                self.proc.wait(timeout=70)
            except subprocess.TimeoutExpired:
                self.proc.kill()
            self.proc = None

    def call(self, method, path, token=None, body=None, headers=None):
        req = urllib.request.Request(self.base + path, method=method)
        req.add_header("Content-Type", "application/json")
        if token:
            req.add_header("Authorization", "Bearer " + token)
        for k, v in (headers or {}).items():
            req.add_header(k, v)
        data = json.dumps(body).encode() if body is not None else None
        try:
            with urllib.request.urlopen(req, data, timeout=30) as r:
                raw = r.read()
                return r.status, (json.loads(raw) if raw else None)
        except urllib.error.HTTPError as e:
            raw = e.read()
            try:
                return e.code, json.loads(raw)
            except ValueError:
                return e.code, raw.decode(errors="replace")

    def register(self, email):
        s, b = self.call("POST", "/auth/register", body={"email": email, "password": "Password123!", "display_name": email})
        if s != 201:
            raise RuntimeError(f"register {email}: {s} {b}")
        return b["data"]["token"]

    def login(self, email, password):
        s, b = self.call("POST", "/auth/login", body={"email": email, "password": password})
        return b["data"]["token"]

    def wait_execution(self, token, eid, timeout=90):
        deadline = time.time() + timeout
        while time.time() < deadline:
            s, b = self.call("GET", f"/ai/executions/{eid}", token)
            if s == 200 and b["data"]["status"] in TERMINAL:
                return b["data"]
            time.sleep(0.4)
        s, b = self.call("GET", f"/ai/executions/{eid}", token)
        raise RuntimeError(f"execution {eid} not finished: {json.dumps(b)[:1500]}")

    def events(self, token, eid, after=0):
        s, b = self.call("GET", f"/ai/executions/{eid}/events?after={after}&limit=500", token)
        return b["data"]["events"]


def sse_collect(api, token, eid, timeout=90):
    """Reads the SSE stream until the server closes it."""
    conn = http.client.HTTPConnection("127.0.0.1", api.port, timeout=timeout)
    conn.request("GET", f"/api/v1/ai/executions/{eid}/events",
                 headers={"Authorization": "Bearer " + token, "Accept": "text/event-stream"})
    resp = conn.getresponse()
    headers = {k.lower(): v for k, v in resp.getheaders()}
    started = time.time()
    raw = resp.read().decode()  # returns when the server ends the stream
    frames = []
    for block in raw.split("\n\n"):
        f = {}
        for line in block.split("\n"):
            if line.startswith("id: "):
                f["id"] = int(line[4:])
            elif line.startswith("event: "):
                f["event"] = line[7:]
            elif line.startswith("data: "):
                f["data"] = json.loads(line[6:])
        if "event" in f:
            frames.append(f)
    return resp.status, headers, frames, time.time() - started


def chat(api, token, conv, message, **extra):
    body = {"message": message}
    body.update(extra)
    s, b = api.call("POST", f"/ai/conversations/{conv}/messages", token, body)
    if s != 200:
        raise RuntimeError(f"message: {s} {b}")
    return b["data"]


def new_conversation(api, token, title):
    s, b = api.call("POST", "/ai/conversations", token, {"title": title})
    return b["data"]["id"]


def approve_body(rev, **extra):
    body = {"version": rev["version"], "spec_hash": rev["spec_hash"]}
    body.update(extra)
    return body


def scenario_a(api, token):
    print("== A. VPS from a chat message (plan -> validate -> approve -> execute -> verify -> report) + SSE")
    conv = new_conversation(api, token, "scenario A")
    reply = chat(api, token, conv, "Create an ubuntu VPS standard size with public ip named web-a01")
    card = reply.get("plan_proposal") or {}
    check("chat proposes a plan card with plan_id + version", card.get("plan_id") and card.get("version") == 1, json.dumps(card)[:300])
    pid = card["plan_id"]
    s, b = api.call("GET", f"/ai/plans/{pid}", token)
    plan = b["data"]
    rev = plan["revision"]
    check("plan has revision 1 with a sha256 spec hash", rev["version"] == 1 and rev["spec_hash"].startswith("sha256:"))
    check("plan keeps the legacy fields", all(k in plan for k in ("id", "status", "plan_json", "resources", "requires_confirmation")))
    check("task graph quota.check -> vps.create -> vps.wait_ready",
          [t["key"] for t in rev["tasks"]] == ["quota.check", "vps.create:web-a01", "vps.wait_ready:web-a01"], [t["key"] for t in rev["tasks"]])
    check("risk medium, approval required, no confirmation", rev["risk_level"] == "medium" and rev["requires_approval"] and rev["confirmation_required"] is None)
    s, b = api.call("POST", f"/ai/plans/{pid}/validate", token)
    v = b["data"]
    check("validate re-runs the policy (legacy keys kept)", s == 200 and v["valid"] and v["validation"]["status"] in ("passed", "warnings") and "plan_id" in v)
    s, b = api.call("POST", f"/ai/plans/{pid}/approve", token, approve_body(rev, spec_hash="sha256:" + "0" * 64))
    check("approve with a wrong spec_hash -> 409 PLAN_VERSION_MISMATCH", s == 409 and b["error"]["code"] == "PLAN_VERSION_MISMATCH", b)
    s, b = api.call("POST", f"/ai/plans/{pid}/approve", token, approve_body(rev))
    res = b["data"] if s == 200 else {}
    check("approve(version, spec_hash) starts an execution", s == 200 and res.get("status") == "provisioning" and res.get("execution"), b)
    check("legacy approve fields present", isinstance(res.get("progress_steps"), list) and res.get("result_details") == {} and res.get("plan_id") == pid)
    check("approval consumed by the execution", res["approval"]["status"] == "consumed")
    eid = res["execution"]["id"]
    status, headers, frames, took = sse_collect(api, token, eid)
    check("SSE: 200 text/event-stream, X-Accel-Buffering: no", status == 200 and headers.get("content-type") == "text/event-stream" and headers.get("x-accel-buffering") == "no", headers)
    types = [f["event"] for f in frames]
    check("SSE: stream terminates after execution.finished", types and types[-1] == "execution.finished", types[-3:])
    check("SSE: ids strictly increasing from 1", [f["id"] for f in frames] == list(range(1, len(frames) + 1)))
    for t in ("execution.created", "approval.recorded", "execution.started", "task.started", "task.succeeded",
              "verification.started", "verification.result", "execution.completed"):
        check(f"SSE: saw {t}", t in types)
    ex = api.wait_execution(token, eid)
    check("execution completed", ex["status"] == "completed", ex["status"])
    tasks = {t["key"]: t for t in ex["tasks"]}
    create = tasks["vps.create:web-a01"]
    check("vps.create output has instance and job ids (no secrets)", create["output"].get("instance_id") and create["output"].get("job_id"))
    ver = create["verification"]
    checks = {c["name"]: c["status"] for c in ver["checks"]}
    check("verification unverified with the simulated hypervisor", ver["status"] == "unverified" and checks.get("hypervisor_power_state") == "skip" and checks.get("instance_record") == "pass", ver)
    rep = ex["report"]
    check("report lists the VPS with ssh access", rep and rep["resources"] and rep["resources"][0]["name"] == "web-a01" and rep["resources"][0]["access"].get("ssh", "").startswith("ssh "), rep)
    check("report has summary, warnings, next_actions", rep["summary"] and isinstance(rep["warnings"], list) and isinstance(rep["next_actions"], list))
    s, b = api.call("GET", "/instances", token)
    names = [i["name"] for i in b["data"]]
    check("instance web-a01 running", any(i["name"] == "web-a01" and i["status"] == "running" for i in b["data"]), names)
    s, b = api.call("GET", f"/ai/plans/{pid}", token)
    check("plan completed, latest_execution_id set", b["data"]["status"] == "completed" and b["data"]["latest_execution_id"] == eid)
    s, b = api.call("GET", f"/ai/conversations/{conv}", token)
    msgs = b["data"]["messages"]
    check("conversation got the execution result message", msgs[-1]["role"] == "assistant" and "completed" in msgs[-1]["content"].lower())
    s, b = api.call("GET", f"/ai/executions?conversation_id={conv}", token)
    check("execution list with task_counts", s == 200 and b["data"][0]["id"] == eid and b["data"][0]["task_counts"].get("succeeded") == 3)
    blob = json.dumps(ex) + json.dumps(api.events(token, eid))
    check("no secrets in execution or events", not any(x in blob for x in ("gw_sec_", "password", "session_token", "rcon")))
    return eid


def scenario_c(api, token, admin_token):
    print("== C. Delete VPS: destructive plan, typed confirmation, executed, verified, audited")
    conv = new_conversation(api, token, "scenario C")
    reply = chat(api, token, conv, "please delete the vps web-a01")
    pid = reply["plan_proposal"]["plan_id"]
    s, b = api.call("GET", f"/ai/plans/{pid}", token)
    rev = b["data"]["revision"]
    check("destructive plan with typed-name confirmation", rev["risk_level"] == "destructive" and rev["confirmation_required"] == {"type": "typed_name", "expected": ["web-a01"]}, rev)
    s, b = api.call("POST", f"/ai/plans/{pid}/approve", token, approve_body(rev))
    check("no acknowledge_risks -> 422 RISK_ACK_REQUIRED", s == 422 and b["error"]["code"] == "RISK_ACK_REQUIRED", b)
    s, b = api.call("POST", f"/ai/plans/{pid}/approve", token, approve_body(rev, acknowledge_risks=True))
    check("no confirmation -> 422 CONFIRMATION_REQUIRED", s == 422 and b["error"]["code"] == "CONFIRMATION_REQUIRED", b)
    s, b = api.call("POST", f"/ai/plans/{pid}/approve", token, approve_body(rev, acknowledge_risks=True, confirmation="Web-A01"))
    check("wrong case -> 422 CONFIRMATION_MISMATCH", s == 422 and b["error"]["code"] == "CONFIRMATION_MISMATCH", b)
    s, b = api.call("POST", f"/ai/plans/{pid}/approve", token, {"acknowledge_risks": True, "confirmation": "web-a01"})
    check("legacy approve (no spec_hash) refused for destructive", s == 400, b)
    s, b = api.call("POST", f"/ai/plans/{pid}/approve", token, approve_body(rev, acknowledge_risks=True, confirmation="web-a01"))
    check("typed confirmation approves and executes", s == 200 and b["data"]["status"] == "provisioning", b)
    eid = b["data"]["execution"]["id"]
    ex = api.wait_execution(token, eid)
    check("delete execution completed", ex["status"] == "completed", ex["status"])
    task = ex["tasks"][0]
    checks = {c["name"]: c["status"] for c in task["verification"]["checks"]}
    check("verified record absent; hypervisor check unverified (mock)", checks.get("instance_record_absent") == "pass" and task["verification"]["status"] == "unverified", task["verification"])
    s, b = api.call("GET", "/instances", token)
    check("instance gone", not any(i["name"] == "web-a01" for i in b["data"]))
    s, b = api.call("GET", "/admin/audit-logs", admin_token)
    actions = [a.get("action") for a in (b.get("data") or [])] if isinstance(b, dict) else []
    check("audited: ai.plan.approve, ai.execution.start, instance.delete", all(a in actions for a in ("ai.plan.approve", "ai.execution.start", "instance.delete")), actions[:20])


def scenario_mcp(api, token):
    print("== M. MCP: initialize, registry tools/list, write tool -> plan only")
    def rpc(method, params=None, rid=1):
        body = {"jsonrpc": "2.0", "method": method}
        if rid is not None:
            body["id"] = rid
        if params is not None:
            body["params"] = params
        return api.call("POST", "/mcp/rpc", token, body)
    s, b = rpc("initialize", {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "e2e", "version": "1"}})
    check("initialize negotiates protocol + tools capability", s == 200 and b["result"]["protocolVersion"] == "2025-06-18" and "tools" in b["result"]["capabilities"])
    s, b = rpc("notifications/initialized", None, None)
    check("notifications/initialized -> 202 without body", s == 202 and b is None, (s, b))
    s, b = rpc("ping")
    check("ping", s == 200 and b["result"] == {})
    s, b = rpc("tools/list")
    names = sorted(t["name"] for t in b["result"]["tools"])
    s2, reg = api.call("GET", "/ai/tools", token)
    check("tools/list == GET /ai/tools", names == sorted(t["id"] for t in reg["data"]) and len(names) == 23, len(names))
    s, b = api.call("GET", "/templates", token)
    tpl = b["data"][0]["id"]
    s, b = rpc("tools/call", {"name": "vps.create", "arguments": {"name": "mcp-e2e", "template_id": tpl, "vcpus": 1, "memory_mb": 1024, "disk_gb": 20}, "confirmed": True})
    out = b["result"].get("structuredContent", {})
    check("MCP write tool -> plan awaiting approval (not executed)", out.get("status") == "awaiting_approval" and out.get("plan_id") and out.get("version") == 1 and out.get("risk_level") == "medium", b)
    time.sleep(1)
    s, ex = api.call("GET", f"/ai/executions?plan_id={out.get('plan_id')}", token)
    s, inst = api.call("GET", "/instances", token)
    check("no execution and no instance from MCP", ex["data"] == [] and not any(i["name"] == "mcp-e2e" for i in inst["data"]))


def scenario_d(api, token):
    print("== D. Failure after VPS creation -> partial, recovery options, delete_created plan, retry works")
    conv = new_conversation(api, token, "scenario D")
    reply = chat(api, token, conv, "Create an ubuntu VPS nano size with public ip named scn-d-web")
    pid = reply["plan_proposal"]["plan_id"]
    s, b = api.call("GET", f"/ai/plans/{pid}", token)
    rev = b["data"]["revision"]
    s, b = api.call("POST", f"/ai/plans/{pid}/approve", token, approve_body(rev))
    eid = b["data"]["execution"]["id"]
    ex = api.wait_execution(token, eid)
    tasks = {t["key"]: t for t in ex["tasks"]}
    wait = tasks["vps.wait_ready:scn-d-web"]
    check("execution partially_completed", ex["status"] == "partially_completed", ex["status"])
    check("vps.create succeeded, wait_ready failed after 3 attempts", tasks["vps.create:scn-d-web"]["status"] == "succeeded" and wait["status"] == "failed" and wait["attempt"] == 3, (wait["status"], wait["attempt"]))
    check("task error is code + message", wait["error"] and wait["error"]["code"] == "FAULT_INJECTED")
    evs = api.events(token, eid)
    check("two task.retrying events with backoff", sum(1 for e in evs if e["type"] == "task.retrying") == 2)
    actions = [o["action"] for o in ex["recovery_options"]]
    check("recovery options retry_failed, keep, delete_created", actions == ["retry_failed", "keep", "delete_created"], actions)
    check("delete_created requires approval", [o for o in ex["recovery_options"] if o["action"] == "delete_created"][0]["requires_approval"])
    check("report lists the created VPS", any(r["name"] == "scn-d-web" for r in ex["report"]["resources"]))
    s, b = api.call("POST", f"/ai/executions/{eid}/recovery", token, {"action": "delete_created"})
    check("delete_created -> 201 {plan_id}", s == 201 and b["data"].get("plan_id"), b)
    dpid = b["data"]["plan_id"]
    s, b = api.call("GET", f"/ai/plans/{dpid}", token)
    drev = b["data"]["revision"]
    check("new destructive plan awaiting approval (nothing deleted)", drev["risk_level"] == "destructive" and drev["confirmation_required"]["expected"] == ["scn-d-web"] and b["data"]["status"] == "awaiting_approval", drev)
    s, inst = api.call("GET", "/instances", token)
    check("VPS still exists", any(i["name"] == "scn-d-web" for i in inst["data"]))
    s, b = api.call("POST", f"/ai/executions/{eid}/tasks/{tasks['vps.create:scn-d-web']['id']}/retry", token)
    check("retry of a succeeded task -> 409 TASK_NOT_RETRYABLE", s == 409 and b["error"]["code"] == "TASK_NOT_RETRYABLE", b)
    s, b = api.call("POST", f"/ai/executions/{eid}/recovery", token, {"action": "retry_failed"})
    check("retry_failed -> 202 execution running", s == 202 and b["data"]["status"] == "running", b)
    ex = api.wait_execution(token, eid)
    tasks = {t["key"]: t for t in ex["tasks"]}
    check("after retry: execution completed", ex["status"] == "completed", ex["status"])
    check("retried task attempt 4 with a fresh budget", tasks["vps.wait_ready:scn-d-web"]["attempt"] == 4 and tasks["vps.wait_ready:scn-d-web"]["status"] == "succeeded")
    jobs = psql(api.db, f"SELECT count(*) FROM provisioning_jobs WHERE idempotency_key LIKE 'orch:{eid}:vps.create%'")
    check("one provisioning job for the VPS", jobs == "1", jobs)


def scenario_restart(api, token):
    print("== R. Restart-resume: SIGKILL while vps.create runs")
    conv = new_conversation(api, token, "scenario R")
    reply = chat(api, token, conv, "Create an ubuntu VPS nano size with public ip named resume-01")
    pid = reply["plan_proposal"]["plan_id"]
    s, b = api.call("GET", f"/ai/plans/{pid}", token)
    rev = b["data"]["revision"]
    s, b = api.call("POST", f"/ai/plans/{pid}/approve", token, approve_body(rev))
    eid = b["data"]["execution"]["id"]
    key1 = f"orch:{eid}:vps.create:resume-01:1"
    deadline = time.time() + 30
    while time.time() < deadline:
        state = psql(api.db, f"SELECT status FROM provisioning_jobs WHERE idempotency_key = '{key1}'")
        if state == "in_progress":
            break
        time.sleep(0.2)
    check("job of attempt 1 in progress before the crash", state == "in_progress", state)
    api.kill()
    task_state = psql(api.db, f"SELECT status FROM ai_execution_tasks WHERE execution_id = '{eid}' AND task_key = 'vps.create:resume-01'")
    check("task left running by the killed replica", task_state == "running", task_state)
    api.start()
    ex = api.wait_execution(token, eid, timeout=150)
    check("execution completed after restart", ex["status"] == "completed", ex["status"])
    rows = psql(api.db, f"""SELECT j.idempotency_key, j.status, coalesce(j.error_code,''), count(i.id)
        FROM provisioning_jobs j LEFT JOIN instances i ON i.id = j.instance_id
        WHERE j.idempotency_key LIKE 'orch:{eid}:%' GROUP BY 1,2,3 ORDER BY 1""").splitlines()
    print("    jobs:", rows)
    keys = [r.split("|")[0] for r in rows]
    check("one job per idempotency key", len(keys) == len(set(keys)) and len(keys) >= 1, rows)
    check("every job terminal", all(r.split("|")[1] in ("completed", "failed") for r in rows), rows)
    check("one instance row per job/key", all(r.split("|")[3] == "1" for r in rows), rows)
    stale = [r for r in rows if r.split("|")[2] == "JOB_STALE"]
    check("the interrupted job was reaped as JOB_STALE", len(stale) == 1 and stale[0].startswith(key1), rows)
    live = psql(api.db, "SELECT count(*) FROM instances WHERE name = 'resume-01' AND deleted_at IS NULL")
    check("exactly one live instance resume-01", live == "1", live)
    evs = api.events(token, eid)
    check("task.started (resumed) after the restart", any(e["type"] == "task.started" and e["metadata"].get("resumed") for e in evs))
    check("events strictly increasing", [e["seq"] for e in evs] == list(range(1, len(evs) + 1)))


def scenario_sigterm_sse(api, token):
    print("== S. SIGTERM: open SSE streams end promptly, the process exits cleanly")
    import threading
    conv = new_conversation(api, token, "scenario S")
    reply = chat(api, token, conv, "Create an ubuntu VPS nano size with public ip named sse-01")
    pid = reply["plan_proposal"]["plan_id"]
    s, b = api.call("GET", f"/ai/plans/{pid}", token)
    s, b = api.call("POST", f"/ai/plans/{pid}/approve", token, approve_body(b["data"]["revision"]))
    eid = b["data"]["execution"]["id"]
    result = {}

    def reader():
        try:
            result["frames"] = sse_collect(api, token, eid, timeout=120)[2]
        except Exception as exc:  # noqa: BLE001 - reported below
            result["error"] = exc
        result["ended"] = time.time()

    th = threading.Thread(target=reader)
    th.start()
    deadline = time.time() + 20
    while time.time() < deadline:
        state = psql(api.db, f"SELECT status FROM ai_execution_tasks WHERE execution_id = '{eid}' AND task_key = 'vps.create:sse-01'")
        if state == "running":
            break
        time.sleep(0.2)
    time.sleep(0.5)
    sent = time.time()
    api.proc.terminate()
    th.join(timeout=30)
    ended_after = result.get("ended", time.time()) - sent
    frames = result.get("frames") or []
    check("stream was delivering events and was ended by the shutdown (not by execution.finished)",
          frames and frames[-1]["event"] != "execution.finished", result.get("error") or [f["event"] for f in frames][-3:])
    check(f"SSE stream ended {ended_after:.1f}s after SIGTERM (<= 3s)", ended_after <= 3.0, ended_after)
    code = api.proc.wait(timeout=90)
    api.proc = None
    check("process exited cleanly (code 0)", code == 0, code)
    # The engine let the running task finish within the shutdown budget or
    # released its lease; a restarted API finishes the execution either way.
    api.start()
    ex = api.wait_execution(token, eid, timeout=120)
    check("execution finished after the restart", ex["status"] == "completed", ex["status"])
    keys = psql(api.db, f"SELECT idempotency_key FROM provisioning_jobs WHERE idempotency_key LIKE 'orch:{eid}:%'").splitlines()
    check("one job per idempotency key", len(keys) == len(set(keys)) and keys, keys)


def main():
    print("Building the API...")
    subprocess.run(["go", "build", "-o", BIN, "./cmd/api"], cwd=BACKEND, check=True)
    apis = []
    try:
        reset_db("nevatal_orch_e2e_a")
        api = API(18290, "nevatal_orch_e2e_a")
        apis.append(api)
        api.start()
        token = api.register("alice@e2e.example.com")
        admin = api.login(ADMIN_EMAIL, ADMIN_PASSWORD)
        scenario_a(api, token)
        scenario_c(api, token, admin)
        scenario_mcp(api, token)
        api.stop()

        reset_db("nevatal_orch_e2e_d")
        api = API(18292, "nevatal_orch_e2e_d", ORCH_FAULTS="vps.wait_ready@scn-d:3")
        apis.append(api)
        api.start()
        scenario_d(api, api.register("dora@e2e.example.com"))
        api.stop()

        reset_db("nevatal_orch_e2e_r")
        api = API(18294, "nevatal_orch_e2e_r", PROXMOX_MOCK_LATENCY_MS="4000",
                  JOB_HEARTBEAT_SECONDS="2", JOB_STALE_AFTER_SECONDS="10")
        apis.append(api)
        api.start()
        scenario_restart(api, api.register("rex@e2e.example.com"))
        api.stop()

        reset_db("nevatal_orch_e2e_s")
        api = API(18296, "nevatal_orch_e2e_s", PROXMOX_MOCK_LATENCY_MS="3000",
                  JOB_HEARTBEAT_SECONDS="2", JOB_STALE_AFTER_SECONDS="10")
        apis.append(api)
        api.start()
        scenario_sigterm_sse(api, api.register("sid@e2e.example.com"))
        api.stop()
    finally:
        for a in apis:
            a.kill()
    print(f"RESULT: {PASS} passed, {FAIL} failed")
    sys.exit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
