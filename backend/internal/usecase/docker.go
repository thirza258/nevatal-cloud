package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/proxmox"
)

// DockerUsecase inspects Docker inside customer VMs through the Proxmox QEMU
// guest agent. Every command is a fixed argv array built here; the only user
// input that reaches the guest is a container reference matching
// containerRefPattern, a bounded --tail count and a validated --since value.
// No shell is ever involved.
type DockerUsecase struct {
	instances *InstanceUsecase
	pve       proxmox.ProxmoxClient
	audit     *AuditUsecase
	timeout   time.Duration

	auditMu   sync.Mutex
	lastAudit map[string]time.Time
}

func NewDockerUsecase(instances *InstanceUsecase, pve proxmox.ProxmoxClient, audit *AuditUsecase) *DockerUsecase {
	return &DockerUsecase{
		instances: instances,
		pve:       pve,
		audit:     audit,
		timeout:   20 * time.Second,
		lastAudit: make(map[string]time.Time),
	}
}

// Reason codes for an unavailable Docker overview.
const (
	DockerReasonVMNotRunning        = "VM_NOT_RUNNING"
	DockerReasonAgentUnavailable    = "GUEST_AGENT_UNAVAILABLE"
	DockerReasonAgentTimeout        = "GUEST_AGENT_TIMEOUT"
	DockerReasonNotInstalled        = "DOCKER_NOT_INSTALLED"
	DockerReasonDaemonUnavailable   = "DOCKER_DAEMON_UNAVAILABLE"
	DockerReasonPermissionDenied    = "DOCKER_PERMISSION_DENIED"
	DockerDefaultLogTail            = 200
	DockerMaxLogTail                = 5000
	dockerAuditInterval             = 10 * time.Minute
	dockerMaxErrorDetail            = 300
	dockerVersionFormat             = "{{.Server.Version}}"
	dockerJSONFormat                = "{{json .}}"
	dockerReasonForbiddenGuestAgent = "The platform's Proxmox API token is not allowed to run guest-agent commands (requires VM.Monitor on Proxmox VE 8 or VM.GuestAgent.Unrestricted on Proxmox VE 9). Contact the platform administrator."
)

var (
	// Docker names/IDs: must start alphanumeric, so the value can never be
	// parsed as a CLI flag.
	containerRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	logSincePattern     = regexp.MustCompile(`^[0-9]{1,4}[smh]$`)
)

type DockerContainer struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Image      string `json:"image"`
	State      string `json:"state"`
	Status     string `json:"status"`
	Ports      string `json:"ports"`
	CreatedAt  string `json:"created_at"`
	Command    string `json:"command"`
	CPUPercent string `json:"cpu_percent,omitempty"`
	MemUsage   string `json:"mem_usage,omitempty"`
	MemPercent string `json:"mem_percent,omitempty"`
}

type DockerOverview struct {
	InstanceID    string            `json:"instance_id"`
	Hostname      string            `json:"hostname"`
	VMID          int               `json:"vmid"`
	Node          string            `json:"node"`
	Available     bool              `json:"available"`
	ReasonCode    string            `json:"reason_code"`
	Reason        string            `json:"reason"`
	DockerVersion string            `json:"docker_version"`
	Containers    []DockerContainer `json:"containers"`
	FetchedAt     time.Time         `json:"fetched_at"`
}

type DockerLogLine struct {
	TS      string `json:"ts"`
	Stream  string `json:"stream"` // stdout | stderr
	Message string `json:"message"`
}

type DockerLogs struct {
	InstanceID string          `json:"instance_id"`
	Container  string          `json:"container"`
	Tail       int             `json:"tail"`
	Since      string          `json:"since"`
	Lines      []DockerLogLine `json:"lines"`
	Truncated  bool            `json:"truncated"`
	FetchedAt  time.Time       `json:"fetched_at"`
}

// DockerError is a failure with an HTTP status and stable error code.
type DockerError struct {
	Status  int
	Code    string
	Message string
}

func (e *DockerError) Error() string { return e.Message }

func dockerErr(status int, code, message string) *DockerError {
	return &DockerError{Status: status, Code: code, Message: message}
}

// Overview lists the containers in the VM. Expected conditions (VM stopped,
// no guest agent, no Docker) return Available=false with a reason instead of
// an error so the UI can show remediation steps.
func (s *DockerUsecase) Overview(ctx context.Context, instanceID, userID uuid.UUID, isAdmin, withStats bool, clientIP string) (*DockerOverview, error) {
	inst, err := s.instances.Get(ctx, instanceID, userID, isAdmin)
	if err != nil {
		return nil, dockerErr(http.StatusNotFound, "INSTANCE_NOT_FOUND", "Instance not found")
	}

	ov := &DockerOverview{
		InstanceID: inst.ID.String(),
		Hostname:   inst.Hostname,
		VMID:       inst.ProxmoxVMID,
		Node:       instanceNode(inst),
		Containers: []DockerContainer{},
		FetchedAt:  time.Now().UTC(),
	}
	s.auditAccess(ctx, userID, inst, "instance.docker.view", "", clientIP)

	if inst.ProxmoxVMID <= 0 || ov.Node == "" {
		return ov.unavailable(DockerReasonVMNotRunning, "The instance has not been provisioned on a hypervisor yet."), nil
	}
	if inst.Status != models.StateRunning {
		return ov.unavailable(DockerReasonVMNotRunning, "The instance is "+inst.Status+". Start it to inspect Docker."), nil
	}

	ps, err := s.exec(ctx, inst, "docker", "ps", "-a", "--no-trunc", "--format", dockerJSONFormat)
	if err != nil {
		if code, reason, ok := agentUnavailableReason(err); ok {
			return ov.unavailable(code, reason), nil
		}
		return nil, dockerErr(http.StatusBadGateway, "GUEST_AGENT_ERROR", "Guest agent request failed: "+truncate(err.Error(), dockerMaxErrorDetail))
	}
	if ps.ExitCode != 0 {
		code, reason := dockerFailureReason(ps)
		return ov.unavailable(code, reason), nil
	}
	ov.Containers = parseDockerPS(ps.OutData)
	ov.Available = true

	// Version and stats are best effort and run concurrently.
	var wg sync.WaitGroup
	var stats map[string]dockerStatsRow
	wg.Add(1)
	go func() {
		defer wg.Done()
		if res, err := s.exec(ctx, inst, "docker", "version", "--format", dockerVersionFormat); err == nil && res.ExitCode == 0 {
			ov.DockerVersion = strings.TrimSpace(res.OutData)
		}
	}()
	if withStats {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if res, err := s.exec(ctx, inst, "docker", "stats", "--no-stream", "--no-trunc", "--format", dockerJSONFormat); err == nil && res.ExitCode == 0 {
				stats = parseDockerStats(res.OutData)
			}
		}()
	}
	wg.Wait()

	for i := range ov.Containers {
		c := &ov.Containers[i]
		row, ok := stats[c.Name]
		if !ok {
			row, ok = stats[c.ID]
		}
		if ok {
			c.CPUPercent, c.MemUsage, c.MemPercent = row.CPUPerc, row.MemUsage, row.MemPerc
		}
	}
	return ov, nil
}

// Logs returns the last tail lines (stdout and stderr merged by timestamp).
func (s *DockerUsecase) Logs(ctx context.Context, instanceID, userID uuid.UUID, isAdmin bool, container string, tail int, since, clientIP string) (*DockerLogs, error) {
	if !containerRefPattern.MatchString(container) {
		return nil, dockerErr(http.StatusBadRequest, "VALIDATION_ERROR", "Invalid container name or ID")
	}
	if tail == 0 {
		tail = DockerDefaultLogTail
	}
	if tail < 1 || tail > DockerMaxLogTail {
		return nil, dockerErr(http.StatusBadRequest, "VALIDATION_ERROR", "tail must be between 1 and "+strconv.Itoa(DockerMaxLogTail))
	}
	if since != "" && !logSincePattern.MatchString(since) {
		return nil, dockerErr(http.StatusBadRequest, "VALIDATION_ERROR", "since must look like 15m, 1h or 30s")
	}

	inst, err := s.instances.Get(ctx, instanceID, userID, isAdmin)
	if err != nil {
		return nil, dockerErr(http.StatusNotFound, "INSTANCE_NOT_FOUND", "Instance not found")
	}
	if inst.ProxmoxVMID <= 0 || instanceNode(inst) == "" || inst.Status != models.StateRunning {
		return nil, dockerErr(http.StatusConflict, "INSTANCE_NOT_RUNNING", "The instance must be running to read container logs")
	}
	s.auditAccess(ctx, userID, inst, "instance.docker.logs", container, clientIP)

	argv := []string{"docker", "logs", "--timestamps", "--tail", strconv.Itoa(tail)}
	if since != "" {
		argv = append(argv, "--since", since)
	}
	argv = append(argv, container)

	res, err := s.exec(ctx, inst, argv...)
	if err != nil {
		switch {
		case errors.Is(err, proxmox.ErrVMNotRunning):
			return nil, dockerErr(http.StatusConflict, "INSTANCE_NOT_RUNNING", "The VM is not running")
		case errors.Is(err, proxmox.ErrGuestAgentTimeout):
			return nil, dockerErr(http.StatusGatewayTimeout, "GUEST_AGENT_TIMEOUT", "The guest agent did not answer in time")
		case errors.Is(err, proxmox.ErrAgentCommandNotFound):
			return nil, dockerErr(http.StatusConflict, "DOCKER_UNAVAILABLE", "Docker is not installed in this VM")
		case errors.Is(err, proxmox.ErrGuestAgentForbidden):
			return nil, dockerErr(http.StatusConflict, "GUEST_AGENT_UNAVAILABLE", dockerReasonForbiddenGuestAgent)
		case errors.Is(err, proxmox.ErrGuestAgentUnavailable):
			return nil, dockerErr(http.StatusConflict, "GUEST_AGENT_UNAVAILABLE", "The QEMU guest agent is not running in this VM")
		default:
			return nil, dockerErr(http.StatusBadGateway, "GUEST_AGENT_ERROR", "Guest agent request failed: "+truncate(err.Error(), dockerMaxErrorDetail))
		}
	}
	if res.ExitCode != 0 {
		stderr := strings.TrimSpace(res.ErrData)
		lower := strings.ToLower(stderr)
		switch {
		case strings.Contains(lower, "no such container"):
			return nil, dockerErr(http.StatusNotFound, "CONTAINER_NOT_FOUND", "No such container: "+container)
		case res.ExitCode == 127:
			return nil, dockerErr(http.StatusConflict, "DOCKER_UNAVAILABLE", "Docker is not installed in this VM")
		case isDockerDaemonDown(lower):
			return nil, dockerErr(http.StatusConflict, "DOCKER_UNAVAILABLE", "The Docker daemon is not running in this VM")
		default:
			return nil, dockerErr(http.StatusBadGateway, "GUEST_AGENT_ERROR", "docker logs failed: "+truncate(firstLine(stderr), dockerMaxErrorDetail))
		}
	}

	lines := mergeDockerLogStreams(res.OutData, res.ErrData)
	if len(lines) > tail {
		lines = lines[len(lines)-tail:]
	}
	return &DockerLogs{
		InstanceID: inst.ID.String(),
		Container:  container,
		Tail:       tail,
		Since:      since,
		Lines:      lines,
		Truncated:  res.OutTruncated || res.ErrTruncated,
		FetchedAt:  time.Now().UTC(),
	}, nil
}

func (s *DockerUsecase) exec(ctx context.Context, inst *models.Instance, argv ...string) (*proxmox.AgentExecStatus, error) {
	return proxmox.RunAgentCommand(ctx, s.pve, instanceNode(inst), inst.ProxmoxVMID, argv, s.timeout)
}

// auditAccess records Docker access at most once per actor/instance/container
// every dockerAuditInterval, so log polling does not flood the audit trail.
func (s *DockerUsecase) auditAccess(ctx context.Context, actorID uuid.UUID, inst *models.Instance, action, container, clientIP string) {
	if s.audit == nil {
		return
	}
	key := actorID.String() + "|" + inst.ID.String() + "|" + action + "|" + container
	now := time.Now()
	s.auditMu.Lock()
	if last, ok := s.lastAudit[key]; ok && now.Sub(last) < dockerAuditInterval {
		s.auditMu.Unlock()
		return
	}
	s.lastAudit[key] = now
	if len(s.lastAudit) > 10000 {
		for k, t := range s.lastAudit {
			if now.Sub(t) >= dockerAuditInterval {
				delete(s.lastAudit, k)
			}
		}
	}
	s.auditMu.Unlock()

	meta := map[string]string{"vmid": strconv.Itoa(inst.ProxmoxVMID)}
	if container != "" {
		meta["container"] = container
	}
	if inst.OwnerID != actorID {
		meta["scope"] = "admin_cross_tenant"
	}
	instID := inst.ID
	s.audit.Log(ctx, &actorID, action, "instance", &instID, "success", clientIP, meta)
}

func (ov *DockerOverview) unavailable(code, reason string) *DockerOverview {
	ov.Available = false
	ov.ReasonCode = code
	ov.Reason = reason
	return ov
}

func instanceNode(inst *models.Instance) string {
	if inst.Node == nil {
		return ""
	}
	return inst.Node.ProxmoxNodeName
}

func agentUnavailableReason(err error) (code, reason string, ok bool) {
	switch {
	case errors.Is(err, proxmox.ErrVMNotRunning):
		return DockerReasonVMNotRunning, "The VM is not running.", true
	case errors.Is(err, proxmox.ErrGuestAgentTimeout):
		return DockerReasonAgentTimeout, "The QEMU guest agent did not answer in time.", true
	case errors.Is(err, proxmox.ErrAgentCommandNotFound):
		return DockerReasonNotInstalled, "Docker is not installed in this VM.", true
	case errors.Is(err, proxmox.ErrGuestAgentForbidden):
		return DockerReasonAgentUnavailable, dockerReasonForbiddenGuestAgent, true
	case errors.Is(err, proxmox.ErrGuestAgentUnavailable):
		return DockerReasonAgentUnavailable, "The QEMU guest agent is not running in this VM, or the VM option \"QEMU Guest Agent\" is disabled.", true
	}
	return "", "", false
}

func dockerFailureReason(res *proxmox.AgentExecStatus) (code, reason string) {
	stderr := strings.TrimSpace(res.ErrData)
	lower := strings.ToLower(stderr)
	switch {
	case res.ExitCode == 127 || strings.Contains(lower, "command not found"):
		return DockerReasonNotInstalled, "Docker is not installed in this VM."
	case strings.Contains(lower, "permission denied"):
		return DockerReasonPermissionDenied, "Permission denied while connecting to the Docker daemon socket."
	case isDockerDaemonDown(lower):
		return DockerReasonDaemonUnavailable, "The Docker daemon is not running in this VM."
	default:
		detail := truncate(firstLine(stderr), dockerMaxErrorDetail)
		if detail == "" {
			detail = "exit code " + strconv.Itoa(res.ExitCode)
		}
		return DockerReasonDaemonUnavailable, "Docker returned an error: " + detail
	}
}

func isDockerDaemonDown(lowerStderr string) bool {
	return strings.Contains(lowerStderr, "cannot connect to the docker daemon") ||
		strings.Contains(lowerStderr, "is the docker daemon running") ||
		strings.Contains(lowerStderr, "docker.sock: connect")
}

// parseDockerPS parses `docker ps -a --no-trunc --format '{{json .}}'`.
func parseDockerPS(out string) []DockerContainer {
	containers := []DockerContainer{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row struct {
			ID        string `json:"ID"`
			Names     string `json:"Names"`
			Image     string `json:"Image"`
			State     string `json:"State"`
			Status    string `json:"Status"`
			Ports     string `json:"Ports"`
			CreatedAt string `json:"CreatedAt"`
			Command   string `json:"Command"`
		}
		if json.Unmarshal([]byte(line), &row) != nil {
			continue
		}
		id := row.ID
		if len(id) > 12 {
			id = id[:12]
		}
		state := strings.ToLower(row.State)
		if state == "" {
			state = stateFromStatus(row.Status)
		}
		containers = append(containers, DockerContainer{
			ID:        id,
			Name:      strings.TrimPrefix(strings.Split(row.Names, ",")[0], "/"),
			Image:     row.Image,
			State:     state,
			Status:    row.Status,
			Ports:     row.Ports,
			CreatedAt: row.CreatedAt,
			Command:   row.Command,
		})
	}
	sort.SliceStable(containers, func(i, j int) bool {
		ri, rj := containers[i].State == "running", containers[j].State == "running"
		if ri != rj {
			return ri
		}
		return containers[i].Name < containers[j].Name
	})
	return containers
}

// stateFromStatus covers Docker versions whose ps output has no State field.
func stateFromStatus(status string) string {
	lower := strings.ToLower(status)
	switch {
	case strings.HasPrefix(lower, "up") && strings.Contains(lower, "paused"):
		return "paused"
	case strings.HasPrefix(lower, "up"):
		return "running"
	case strings.HasPrefix(lower, "restarting"):
		return "restarting"
	case strings.HasPrefix(lower, "exited"):
		return "exited"
	case strings.HasPrefix(lower, "created"):
		return "created"
	case strings.HasPrefix(lower, "dead"):
		return "dead"
	case strings.HasPrefix(lower, "removal"):
		return "removing"
	}
	return lower
}

type dockerStatsRow struct {
	CPUPerc, MemUsage, MemPerc string
}

// parseDockerStats parses `docker stats --no-stream --no-trunc --format '{{json .}}'`,
// keyed by container name and by short ID.
func parseDockerStats(out string) map[string]dockerStatsRow {
	rows := map[string]dockerStatsRow{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row struct {
			ID       string `json:"ID"`
			Name     string `json:"Name"`
			CPUPerc  string `json:"CPUPerc"`
			MemUsage string `json:"MemUsage"`
			MemPerc  string `json:"MemPerc"`
		}
		if json.Unmarshal([]byte(line), &row) != nil {
			continue
		}
		stat := dockerStatsRow{CPUPerc: row.CPUPerc, MemUsage: row.MemUsage, MemPerc: row.MemPerc}
		if row.Name != "" {
			rows[strings.TrimPrefix(row.Name, "/")] = stat
		}
		if len(row.ID) >= 12 {
			rows[row.ID[:12]] = stat
		}
	}
	return rows
}

// mergeDockerLogStreams interleaves `docker logs --timestamps` stdout and
// stderr by timestamp. Lines without a parsable timestamp keep their position
// relative to the previous line of the same stream.
func mergeDockerLogStreams(stdout, stderr string) []DockerLogLine {
	type parsed struct {
		at   time.Time
		line DockerLogLine
	}
	parse := func(data, stream string) []parsed {
		var out []parsed
		var last time.Time
		for _, raw := range strings.Split(strings.TrimRight(data, "\n"), "\n") {
			if raw == "" {
				continue
			}
			raw = strings.TrimRight(raw, "\r")
			ts, msg := "", raw
			if idx := strings.IndexByte(raw, ' '); idx > 0 {
				if at, err := time.Parse(time.RFC3339Nano, raw[:idx]); err == nil {
					ts, msg, last = raw[:idx], raw[idx+1:], at
				}
			}
			out = append(out, parsed{at: last, line: DockerLogLine{TS: ts, Stream: stream, Message: msg}})
		}
		return out
	}

	a, b := parse(stdout, "stdout"), parse(stderr, "stderr")
	merged := make([]DockerLogLine, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if !b[j].at.Before(a[i].at) {
			merged = append(merged, a[i].line)
			i++
		} else {
			merged = append(merged, b[j].line)
			j++
		}
	}
	for ; i < len(a); i++ {
		merged = append(merged, a[i].line)
	}
	for ; j < len(b); j++ {
		merged = append(merged, b[j].line)
	}
	return merged
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
