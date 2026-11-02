package proxmox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"
)

// Simulated QEMU guest agent with a fake Docker engine, so the Docker viewer
// works end to end in PROXMOX_MOCK / DEMO_MODE. Output mimics docker 27.

const mockDockerVersion = "27.3.1"

func (m *mockPVEClient) AgentExec(ctx context.Context, node string, vmid int, command []string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(command) == 0 {
		return 0, fmt.Errorf("agent exec: empty command")
	}
	if m.deletedVMs[vmid] {
		return 0, fmt.Errorf("configuration file 'nodes/%s/qemu-server/%d.conf' does not exist", m.nodeName, vmid)
	}
	vm := m.getOrInitVMLocked(vmid)
	if vm.status != "running" {
		return 0, fmt.Errorf("%w: VM %d not running", ErrVMNotRunning, vmid)
	}
	if command[0] != "docker" {
		return 0, fmt.Errorf("%w: Agent error: Failed to execute child process “%s” (No such file or directory)", ErrAgentCommandNotFound, command[0])
	}

	if len(m.execs) > 512 { // results nobody collected
		m.execs = make(map[int]*AgentExecStatus)
	}
	m.execPID++
	m.execs[m.execPID] = fakeDocker(vm, command[1:], time.Now().UTC())
	return m.execPID, nil
}

func (m *mockPVEClient) AgentExecStatus(ctx context.Context, node string, vmid int, pid int) (*AgentExecStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, ok := m.execs[pid]
	if !ok {
		return nil, fmt.Errorf("%w: Agent error: Invalid parameter 'pid'", ErrGuestAgentUnavailable)
	}
	delete(m.execs, pid)
	return status, nil
}

type mockContainer struct {
	name, image, command, ports, kind string
	createdAgo                        time.Duration
	exited                            bool
	exitCode                          int
	exitedAgo                         time.Duration
	cpu, memUsage, memPerc            string
}

func (c mockContainer) id(vmid int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("nevatal-mock/%d/%s", vmid, c.name)))
	return hex.EncodeToString(sum[:])
}

// mockStacks gives each VM one of a few realistic compose stacks.
var mockStacks = [][]mockContainer{
	{
		{name: "web", image: "nginx:1.27-alpine", command: `"/docker-entrypoint.sh nginx -g 'daemon off;'"`, ports: "0.0.0.0:80->80/tcp, [::]:80->80/tcp", kind: "nginx", createdAgo: 9 * 24 * time.Hour, cpu: "0.04%", memUsage: "9.8MiB / 3.82GiB", memPerc: "0.25%"},
		{name: "api", image: "ghcr.io/acme/shop-api:2.4.1", command: `"docker-entrypoint.sh node dist/server.js"`, ports: "3000/tcp", kind: "node", createdAgo: 2 * 24 * time.Hour, cpu: "1.87%", memUsage: "148.2MiB / 3.82GiB", memPerc: "3.79%"},
		{name: "db", image: "postgres:16-alpine", command: `"docker-entrypoint.sh postgres"`, ports: "5432/tcp", kind: "postgres", createdAgo: 9 * 24 * time.Hour, cpu: "0.31%", memUsage: "61.4MiB / 3.82GiB", memPerc: "1.57%"},
		{name: "cache", image: "redis:7-alpine", command: `"docker-entrypoint.sh redis-server"`, ports: "6379/tcp", kind: "redis", createdAgo: 9 * 24 * time.Hour, cpu: "0.12%", memUsage: "4.1MiB / 3.82GiB", memPerc: "0.10%"},
		{name: "migrate", image: "ghcr.io/acme/shop-api:2.4.1", command: `"docker-entrypoint.sh node dist/migrate.js"`, kind: "migrate", createdAgo: 2 * 24 * time.Hour, exited: true, exitCode: 0, exitedAgo: 2*24*time.Hour - 40*time.Second},
	},
	{
		{name: "wordpress", image: "wordpress:6.7-php8.3-apache", command: `"docker-entrypoint.sh apache2-foreground"`, ports: "80/tcp", kind: "wordpress", createdAgo: 21 * 24 * time.Hour, cpu: "0.58%", memUsage: "96.7MiB / 3.82GiB", memPerc: "2.47%"},
		{name: "mariadb", image: "mariadb:11.4", command: `"docker-entrypoint.sh mariadbd"`, ports: "3306/tcp", kind: "mariadb", createdAgo: 21 * 24 * time.Hour, cpu: "0.22%", memUsage: "112.9MiB / 3.82GiB", memPerc: "2.89%"},
		{name: "caddy", image: "caddy:2.8-alpine", command: `"caddy run --config /etc/caddy/Caddyfile --adapter caddyfile"`, ports: "0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp, 443/udp", kind: "caddy", createdAgo: 21 * 24 * time.Hour, cpu: "0.03%", memUsage: "17.2MiB / 3.82GiB", memPerc: "0.44%"},
	},
	{
		{name: "grafana", image: "grafana/grafana:11.3.0", command: `"/run.sh"`, ports: "0.0.0.0:3000->3000/tcp", kind: "grafana", createdAgo: 5 * 24 * time.Hour, cpu: "0.41%", memUsage: "88.3MiB / 3.82GiB", memPerc: "2.26%"},
		{name: "prometheus", image: "prom/prometheus:v2.55.0", command: `"/bin/prometheus --config.file=/etc/prometheus/prometheus.yml"`, ports: "0.0.0.0:9090->9090/tcp", kind: "prometheus", createdAgo: 5 * 24 * time.Hour, cpu: "0.93%", memUsage: "201.5MiB / 3.82GiB", memPerc: "5.15%"},
		{name: "node-exporter", image: "prom/node-exporter:v1.8.2", command: `"/bin/node_exporter --path.rootfs=/host"`, ports: "9100/tcp", kind: "exporter", createdAgo: 5 * 24 * time.Hour, cpu: "0.07%", memUsage: "11.6MiB / 3.82GiB", memPerc: "0.30%"},
		{name: "cadvisor", image: "gcr.io/cadvisor/cadvisor:v0.49.1", command: `"/usr/bin/cadvisor -logtostderr"`, kind: "cadvisor", createdAgo: 5 * 24 * time.Hour, exited: true, exitCode: 255, exitedAgo: 3 * time.Hour},
	},
}

func mockContainersFor(vmid int) []mockContainer {
	return mockStacks[vmid%len(mockStacks)]
}

func fakeDocker(vm *mockVM, args []string, now time.Time) *AgentExecStatus {
	ok := func(out string) *AgentExecStatus { return &AgentExecStatus{Exited: true, OutData: out} }
	fail := func(code int, msg string) *AgentExecStatus {
		return &AgentExecStatus{Exited: true, ExitCode: code, ErrData: msg}
	}
	if len(args) == 0 {
		return fail(1, "Usage:  docker [OPTIONS] COMMAND\n")
	}

	vmUp := now.Sub(vm.startedAt)
	if vm.startedAt.IsZero() || vmUp < 0 {
		vmUp = 0
	}
	containers := mockContainersFor(vm.vmid)

	switch args[0] {
	case "version":
		return ok(mockDockerVersion + "\n")

	case "ps":
		var b strings.Builder
		for _, c := range containers {
			created := now.Add(-c.createdAgo)
			state, status := "running", "Up "+humanDuration(minDuration(c.createdAgo, vmUp))
			if c.exited {
				state = "exited"
				status = fmt.Sprintf("Exited (%d) %s ago", c.exitCode, humanDuration(c.exitedAgo))
			}
			line, _ := json.Marshal(map[string]string{
				"Command":      c.command,
				"CreatedAt":    created.Format("2006-01-02 15:04:05 -0700 MST"),
				"ID":           c.id(vm.vmid),
				"Image":        c.image,
				"Labels":       "com.docker.compose.project=app,com.docker.compose.service=" + c.name,
				"LocalVolumes": "1",
				"Mounts":       "",
				"Names":        c.name,
				"Networks":     "app_default",
				"Ports":        c.ports,
				"RunningFor":   humanDuration(c.createdAgo) + " ago",
				"Size":         "0B",
				"State":        state,
				"Status":       status,
			})
			b.Write(line)
			b.WriteByte('\n')
		}
		return ok(b.String())

	case "stats":
		var b strings.Builder
		for _, c := range containers {
			if c.exited {
				continue
			}
			id := c.id(vm.vmid)
			line, _ := json.Marshal(map[string]string{
				"BlockIO":   "18.4MB / 2.1MB",
				"CPUPerc":   c.cpu,
				"Container": id[:12],
				"ID":        id,
				"MemPerc":   c.memPerc,
				"MemUsage":  c.memUsage,
				"Name":      c.name,
				"NetIO":     "4.82MB / 9.17MB",
				"PIDs":      "7",
			})
			b.Write(line)
			b.WriteByte('\n')
		}
		return ok(b.String())

	case "logs":
		tail, since, target := 100, time.Duration(0), ""
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--timestamps", "-t":
			case "--tail", "-n":
				if i+1 < len(args) {
					i++
					if n, err := strconv.Atoi(args[i]); err == nil {
						tail = n
					}
				}
			case "--since":
				if i+1 < len(args) {
					i++
					if d, err := time.ParseDuration(args[i]); err == nil {
						since = d
					}
				}
			default:
				target = args[i]
			}
		}
		for _, c := range containers {
			id := c.id(vm.vmid)
			if target == c.name || (len(target) >= 3 && strings.HasPrefix(id, target)) {
				stdout, stderr := mockLogs(vm.vmid, c, now, vmUp, tail, since)
				return &AgentExecStatus{Exited: true, OutData: stdout, ErrData: stderr}
			}
		}
		return fail(1, fmt.Sprintf("Error response from daemon: No such container: %s\n", target))

	default:
		return fail(125, fmt.Sprintf("docker: '%s' is not a docker command.\nSee 'docker --help'\n", args[0]))
	}
}

// mockLogs generates deterministic, timestamped log lines for a container.
func mockLogs(vmid int, c mockContainer, now time.Time, vmUp time.Duration, tail int, since time.Duration) (string, string) {
	if tail <= 0 || tail > 5000 {
		tail = 5000
	}
	end := now
	if c.exited {
		end = now.Add(-c.exitedAgo)
	}
	start := now.Add(-c.createdAgo)
	if !c.exited && vmUp > 0 && vmUp < c.createdAgo {
		start = now.Add(-vmUp)
	}
	cutoff := start
	if since > 0 && now.Add(-since).After(cutoff) {
		cutoff = now.Add(-since)
	}

	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%d/%s", vmid, c.name)
	seed := h.Sum64()
	next := func() uint64 {
		seed = seed*6364136223846793005 + 1442695040888963407
		return seed >> 33
	}

	type line struct {
		ts     time.Time
		stderr bool
		text   string
	}
	var lines []line
	ts := end
	for len(lines) < tail && ts.After(cutoff) {
		stderr, text := mockLogLine(c.kind, next(), ts)
		lines = append(lines, line{ts, stderr, text})
		ts = ts.Add(-time.Duration(4+next()%55) * time.Second)
	}
	if c.exited && len(lines) > 0 {
		lines[0] = line{end, true, mockExitLine(c)}
	}

	var out, errOut strings.Builder
	for i := len(lines) - 1; i >= 0; i-- {
		l := lines[i]
		dst := &out
		if l.stderr {
			dst = &errOut
		}
		dst.WriteString(l.ts.Format("2006-01-02T15:04:05.000000000Z"))
		dst.WriteByte(' ')
		dst.WriteString(l.text)
		dst.WriteByte('\n')
	}
	return out.String(), errOut.String()
}

func mockExitLine(c mockContainer) string {
	if c.kind == "migrate" {
		return "[migrate] 3 migrations applied, schema at version 20261031_add_orders_index"
	}
	return "F1102 failed to start container manager: inotify_add_watch /sys/fs/cgroup/cpu: no such file or directory"
}

var (
	mockPaths   = []string{"/", "/api/products", "/api/cart", "/healthz", "/static/app.3f9c1.js", "/api/orders", "/login", "/favicon.ico"}
	mockIPs     = []string{"10.10.42.1", "172.18.0.1", "203.0.113.24", "198.51.100.7", "192.0.2.41"}
	mockAgents  = []string{"Mozilla/5.0 (X11; Linux x86_64)", "curl/8.5.0", "kube-probe/1.31", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6)"}
	mockStatus  = []int{200, 200, 200, 200, 304, 200, 404, 200, 502}
	mockQueries = []string{"SELECT * FROM orders WHERE customer_id = $1", "UPDATE carts SET updated_at = now() WHERE id = $1", "SELECT count(*) FROM products"}
)

func mockLogLine(kind string, r uint64, ts time.Time) (bool, string) {
	pick := func(n int) int { return int(r % uint64(n)) }
	switch kind {
	case "nginx", "wordpress":
		if r%23 == 0 {
			return true, fmt.Sprintf("%s [warn] 29#29: *%d an upstream response is buffered to a temporary file /var/cache/nginx/proxy_temp/1/00/0000000%d", ts.Format("2006/01/02 15:04:05"), 1000+r%9000, r%10)
		}
		status := mockStatus[pick(len(mockStatus))]
		return false, fmt.Sprintf(`%s - - [%s] "GET %s HTTP/1.1" %d %d "-" "%s"`, mockIPs[pick(len(mockIPs))], ts.Format("02/Jan/2006:15:04:05 -0700"), mockPaths[pick(len(mockPaths))], status, 180+r%24000, mockAgents[pick(len(mockAgents))])
	case "node":
		if r%17 == 0 {
			return true, fmt.Sprintf(`{"level":"warn","msg":"slow query","duration_ms":%d,"query":"%s"}`, 500+r%900, mockQueries[pick(len(mockQueries))])
		}
		return false, fmt.Sprintf(`{"level":"info","msg":"request completed","method":"GET","path":"%s","status":%d,"duration_ms":%.1f}`, mockPaths[pick(len(mockPaths))], mockStatus[pick(len(mockStatus))], float64(r%4000)/100)
	case "postgres":
		msgs := []string{"LOG:  checkpoint starting: time", fmt.Sprintf("LOG:  checkpoint complete: wrote %d buffers (0.%d%%); 0 WAL file(s) added", 10+r%400, r%9), "LOG:  automatic vacuum of table \"app.public.orders\": index scans: 1"}
		return true, fmt.Sprintf("%s UTC [%d] %s", ts.Format("2006-01-02 15:04:05.000"), 60+r%40, msgs[pick(len(msgs))])
	case "redis":
		msgs := []string{"* 100 changes in 300 seconds. Saving...", "* Background saving started by pid 412", "* DB saved on disk", "* Background saving terminated with success"}
		return false, fmt.Sprintf("1:M %s %s", ts.Format("02 Jan 2006 15:04:05.000"), msgs[pick(len(msgs))])
	case "mariadb":
		msgs := []string{"[Note] InnoDB: Buffer pool(s) dump completed", "[Note] Aborted connection 412 to db: 'wordpress' user: 'wp' host: '172.18.0.3' (Got timeout reading communication packets)", "[Note] InnoDB: Resuming purge"}
		return true, fmt.Sprintf("%s 0 %s", ts.Format("2006-01-02 15:04:05"), msgs[pick(len(msgs))])
	case "caddy":
		return true, fmt.Sprintf(`{"level":"info","ts":%.3f,"logger":"http.log.access","msg":"handled request","request":{"method":"GET","host":"blog.example.com","uri":"%s"},"status":%d}`, float64(ts.UnixMilli())/1000, mockPaths[pick(len(mockPaths))], mockStatus[pick(len(mockStatus))])
	case "grafana":
		return false, fmt.Sprintf(`logger=context userId=1 orgId=1 uname=admin t=%s level=info msg="Request Completed" method=GET path=/api/dashboards/uid/node status=200 duration=%dms`, ts.Format(time.RFC3339), 2+r%80)
	case "prometheus":
		return true, fmt.Sprintf(`ts=%s caller=compact.go:576 level=info component=tsdb msg="write block" duration=%dms`, ts.Format(time.RFC3339Nano), 40+r%400)
	case "exporter":
		return true, fmt.Sprintf(`ts=%s caller=tls_config.go:313 level=info msg="Listening on" address=[::]:9100`, ts.Format(time.RFC3339Nano))
	case "migrate":
		return false, fmt.Sprintf("[migrate] applying migration %d", 20261001+r%30)
	default:
		return false, "container started"
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if b > 0 && b < a {
		return b
	}
	return a
}

// humanDuration mirrors docker's go-units HumanDuration.
func humanDuration(d time.Duration) string {
	switch seconds := int(d.Seconds()); {
	case seconds < 1:
		return "Less than a second"
	case seconds == 1:
		return "1 second"
	case seconds < 60:
		return fmt.Sprintf("%d seconds", seconds)
	}
	switch minutes := int(d.Minutes()); {
	case minutes == 1:
		return "About a minute"
	case minutes < 60:
		return fmt.Sprintf("%d minutes", minutes)
	}
	switch hours := int(d.Hours() + 0.5); {
	case hours == 1:
		return "About an hour"
	case hours < 48:
		return fmt.Sprintf("%d hours", hours)
	case hours < 24*7*2:
		return fmt.Sprintf("%d days", hours/24)
	case hours < 24*30*2:
		return fmt.Sprintf("%d weeks", hours/24/7)
	default:
		return fmt.Sprintf("%d months", hours/24/30)
	}
}
