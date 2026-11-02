package proxmox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Guest agent errors. Callers classify failures with errors.Is.
var (
	// ErrVMNotRunning means the VM is stopped or paused.
	ErrVMNotRunning = errors.New("vm is not running")
	// ErrGuestAgentUnavailable means qemu-guest-agent is not enabled on the
	// VM, not running inside the guest, or guest-exec is disabled by policy.
	ErrGuestAgentUnavailable = errors.New("qemu guest agent is not available")
	// ErrGuestAgentForbidden means the API token lacks the privilege needed
	// for guest-agent exec (PVE 8: VM.Monitor, PVE 9: VM.GuestAgent.Unrestricted).
	ErrGuestAgentForbidden = errors.New("proxmox api token is not allowed to run guest agent commands")
	// ErrGuestAgentTimeout means the command did not finish in time.
	ErrGuestAgentTimeout = errors.New("timed out waiting for guest agent command")
	// ErrAgentCommandNotFound means the executable does not exist in the guest.
	ErrAgentCommandNotFound = errors.New("command not found in guest")
)

// APIError is returned for non-2xx Proxmox API responses.
type APIError struct {
	StatusCode int
	Message    string // Proxmox "message" field when present
	Body       string
}

func (e *APIError) Error() string {
	detail := e.Message
	if detail == "" {
		detail = e.Body
	}
	return fmt.Sprintf("proxmox api returned HTTP %d: %s", e.StatusCode, detail)
}

// newAPIError extracts the human-readable reason. pveproxy usually sends it in
// the HTTP reason phrase ("500 VM 104 not running") with a body of
// {"data":null}; some endpoints also put it in a "message" field.
func newAPIError(statusCode int, statusLine string, body []byte) *APIError {
	apiErr := &APIError{StatusCode: statusCode, Body: strings.TrimSpace(string(body))}

	var parsed struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &parsed) == nil && strings.TrimSpace(parsed.Message) != "" {
		apiErr.Message = strings.TrimSpace(parsed.Message)
		return apiErr
	}

	reason := strings.TrimSpace(strings.TrimPrefix(statusLine, strconv.Itoa(statusCode)))
	if reason != "" && reason != http.StatusText(statusCode) {
		apiErr.Message = reason
	} else {
		apiErr.Message = apiErr.Body
	}
	return apiErr
}

// AgentExecStatus is the result of a guest-agent exec-status call.
type AgentExecStatus struct {
	Exited       bool
	ExitCode     int
	Signal       int
	OutData      string
	ErrData      string
	OutTruncated bool
	ErrTruncated bool
}

// AgentExec starts argv inside the guest via the QEMU guest agent and returns
// the guest PID. No shell is involved: arguments are passed verbatim.
func (c *httpPVEClient) AgentExec(ctx context.Context, node string, vmid int, command []string) (int, error) {
	if len(command) == 0 {
		return 0, errors.New("agent exec: empty command")
	}
	payload, err := json.Marshal(map[string]any{"command": command})
	if err != nil {
		return 0, fmt.Errorf("agent exec: encode payload: %w", err)
	}

	endpoint := fmt.Sprintf("/nodes/%s/qemu/%d/agent/exec", url.PathEscape(node), vmid)
	data, err := c.send(ctx, http.MethodPost, endpoint, bytes.NewReader(payload), "application/json")
	if err != nil {
		return 0, classifyAgentError(err, vmid)
	}

	var res struct {
		Data struct {
			PID flexInt `json:"pid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return 0, fmt.Errorf("agent exec: decode response: %w", err)
	}
	if res.Data.PID <= 0 {
		return 0, fmt.Errorf("agent exec: guest agent returned no pid")
	}
	return int(res.Data.PID), nil
}

// AgentExecStatus polls the state of a command started with AgentExec.
func (c *httpPVEClient) AgentExecStatus(ctx context.Context, node string, vmid int, pid int) (*AgentExecStatus, error) {
	endpoint := fmt.Sprintf("/nodes/%s/qemu/%d/agent/exec-status?pid=%d", url.PathEscape(node), vmid, pid)
	data, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, classifyAgentError(err, vmid)
	}

	var res struct {
		Data struct {
			Exited       flexBool `json:"exited"`
			ExitCode     flexInt  `json:"exitcode"`
			Signal       flexInt  `json:"signal"`
			OutData      string   `json:"out-data"`
			ErrData      string   `json:"err-data"`
			OutTruncated flexBool `json:"out-truncated"`
			ErrTruncated flexBool `json:"err-truncated"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("agent exec-status: decode response: %w", err)
	}
	return &AgentExecStatus{
		Exited:       bool(res.Data.Exited),
		ExitCode:     int(res.Data.ExitCode),
		Signal:       int(res.Data.Signal),
		OutData:      res.Data.OutData,
		ErrData:      res.Data.ErrData,
		OutTruncated: bool(res.Data.OutTruncated),
		ErrTruncated: bool(res.Data.ErrTruncated),
	}, nil
}

// RunAgentCommand runs argv in the guest and waits until it exits or the
// timeout elapses. A deadline maps to ErrGuestAgentTimeout.
func RunAgentCommand(ctx context.Context, client ProxmoxClient, node string, vmid int, command []string, timeout time.Duration) (*AgentExecStatus, error) {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pid, err := client.AgentExec(ctx, node, vmid, command)
	if err != nil {
		return nil, timeoutOr(ctx, err)
	}

	delay := 100 * time.Millisecond
	for {
		status, err := client.AgentExecStatus(ctx, node, vmid, pid)
		if err != nil {
			return nil, timeoutOr(ctx, err)
		}
		if status.Exited {
			return status, nil
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, timeoutOr(ctx, ctx.Err())
		case <-timer.C:
		}
		if delay < time.Second {
			delay *= 2
		}
	}
}

func timeoutOr(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrGuestAgentTimeout
	}
	return err
}

// classifyAgentError maps Proxmox API failures to the sentinel errors above.
// Messages observed from PVE 8/9:
//
//	"VM 104 not running"
//	"QEMU guest agent is not running"
//	"No QEMU guest agent configured"
//	"Agent error: Failed to execute child process “docker” (No such file or directory)"
//	"Agent error: The command guest-exec has been disabled for this instance"
//	"Permission check failed (/vms/104, VM.Monitor)"
func classifyAgentError(err error, vmid int) error {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	msg := apiErr.Message
	lower := strings.ToLower(msg)

	switch {
	case apiErr.StatusCode == http.StatusForbidden || strings.Contains(lower, "permission check failed"):
		return fmt.Errorf("%w: %s", ErrGuestAgentForbidden, msg)
	case strings.Contains(lower, "vm "+strconv.Itoa(vmid)+" not running") || (strings.Contains(lower, "not running") && !strings.Contains(lower, "agent")):
		return fmt.Errorf("%w: %s", ErrVMNotRunning, msg)
	case strings.Contains(lower, "no such file or directory") || strings.Contains(lower, "failed to execute child process"):
		return fmt.Errorf("%w: %s", ErrAgentCommandNotFound, msg)
	case strings.Contains(lower, "guest agent") || strings.Contains(lower, "has been disabled") || strings.Contains(lower, "agent error"):
		return fmt.Errorf("%w: %s", ErrGuestAgentUnavailable, msg)
	default:
		return err
	}
}

// flexBool accepts true/false, 1/0 and "1"/"0" (Perl JSON encodes booleans inconsistently).
type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	switch strings.Trim(strings.TrimSpace(string(data)), `"`) {
	case "1", "true":
		*b = true
	default:
		*b = false
	}
	return nil
}

// flexInt accepts numbers and numeric strings.
type flexInt int

func (i *flexInt) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if raw == "" || raw == "null" {
		*i = 0
		return nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("invalid integer %q", raw)
	}
	*i = flexInt(n)
	return nil
}
