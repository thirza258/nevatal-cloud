package proxmox

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewAPIErrorUsesReasonPhrase(t *testing.T) {
	err := newAPIError(500, "500 QEMU guest agent is not running", []byte(`{"data":null}`))
	if err.Message != "QEMU guest agent is not running" {
		t.Fatalf("expected reason phrase, got %q", err.Message)
	}
	err = newAPIError(500, "500 Internal Server Error", []byte(`{"data":null,"message":"VM 104 not running\n"}`))
	if err.Message != "VM 104 not running" {
		t.Fatalf("expected body message, got %q", err.Message)
	}
}

func TestClassifyAgentError(t *testing.T) {
	cases := []struct {
		status int
		msg    string
		want   error
	}{
		{500, "VM 104 not running", ErrVMNotRunning},
		{500, "QEMU guest agent is not running", ErrGuestAgentUnavailable},
		{500, "No QEMU guest agent configured", ErrGuestAgentUnavailable},
		{500, "Agent error: The command guest-exec has been disabled for this instance", ErrGuestAgentUnavailable},
		{500, "Agent error: Failed to execute child process “docker” (No such file or directory)", ErrAgentCommandNotFound},
		{403, "Permission check failed (/vms/104, VM.Monitor)", ErrGuestAgentForbidden},
	}
	for _, tc := range cases {
		err := classifyAgentError(&APIError{StatusCode: tc.status, Message: tc.msg}, 104)
		if !errors.Is(err, tc.want) {
			t.Errorf("%q: expected %v, got %v", tc.msg, tc.want, err)
		}
	}
	plain := errors.New("dial tcp: connection refused")
	if got := classifyAgentError(plain, 104); got != plain {
		t.Errorf("non-API errors must pass through, got %v", got)
	}
}

func TestFlexibleJSONScalars(t *testing.T) {
	var v struct {
		A flexBool `json:"a"`
		B flexBool `json:"b"`
		C flexInt  `json:"c"`
		D flexInt  `json:"d"`
	}
	if err := json.Unmarshal([]byte(`{"a":1,"b":true,"c":"42","d":7}`), &v); err != nil {
		t.Fatal(err)
	}
	if !v.A || !v.B || v.C != 42 || v.D != 7 {
		t.Fatalf("unexpected decode: %+v", v)
	}
}

func TestMockGuestAgentDocker(t *testing.T) {
	ctx := context.Background()
	c := NewMockClient("pve-test")

	ps, err := RunAgentCommand(ctx, c, "pve-test", 1001, []string{"docker", "ps", "-a", "--no-trunc", "--format", "{{json .}}"}, time.Second)
	if err != nil || ps.ExitCode != 0 {
		t.Fatalf("docker ps failed: %v %+v", err, ps)
	}
	lines := strings.Split(strings.TrimSpace(ps.OutData), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected several containers, got %d", len(lines))
	}
	var first map[string]string
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil || first["Names"] == "" || len(first["ID"]) != 64 {
		t.Fatalf("unexpected ps row: %v %v", first, err)
	}

	logs, err := RunAgentCommand(ctx, c, "pve-test", 1001, []string{"docker", "logs", "--timestamps", "--tail", "50", first["Names"]}, time.Second)
	if err != nil || logs.ExitCode != 0 {
		t.Fatalf("docker logs failed: %v %+v", err, logs)
	}
	total := strings.Count(logs.OutData, "\n") + strings.Count(logs.ErrData, "\n")
	if total == 0 || total > 50 {
		t.Fatalf("expected 1..50 log lines, got %d", total)
	}

	missing, err := RunAgentCommand(ctx, c, "pve-test", 1001, []string{"docker", "logs", "--tail", "5", "nope"}, time.Second)
	if err != nil || missing.ExitCode == 0 || !strings.Contains(missing.ErrData, "No such container") {
		t.Fatalf("expected 'No such container', got %v %+v", err, missing)
	}

	if _, err := c.StopVM(ctx, "pve-test", 1001); err != nil {
		t.Fatal(err)
	}
	if _, err := RunAgentCommand(ctx, c, "pve-test", 1001, []string{"docker", "ps"}, time.Second); !errors.Is(err, ErrVMNotRunning) {
		t.Fatalf("stopped VM must return ErrVMNotRunning, got %v", err)
	}
}

type slowAgent struct {
	ProxmoxClient
}

func (slowAgent) AgentExec(context.Context, string, int, []string) (int, error) { return 1, nil }
func (slowAgent) AgentExecStatus(context.Context, string, int, int) (*AgentExecStatus, error) {
	return &AgentExecStatus{Exited: false}, nil
}

func TestRunAgentCommandTimeout(t *testing.T) {
	_, err := RunAgentCommand(context.Background(), slowAgent{}, "n", 1, []string{"docker", "ps"}, 300*time.Millisecond)
	if !errors.Is(err, ErrGuestAgentTimeout) {
		t.Fatalf("expected ErrGuestAgentTimeout, got %v", err)
	}
}

func TestDeleteParamsTravelInQueryString(t *testing.T) {
	var gotURL string
	c := &httpPVEClient{
		baseURL:    "https://pve.invalid/api2/json",
		authHeader: "PVEAPIToken=x=y",
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotURL = r.URL.String()
			return &http.Response{StatusCode: 200, Status: "200 OK", Body: http.NoBody, Header: http.Header{}}, nil
		})},
	}
	_, _ = c.DeleteVM(context.Background(), "pve", 104)
	if !strings.Contains(gotURL, "purge=1") || !strings.Contains(gotURL, "destroy-unreferenced-disks=1") {
		t.Fatalf("DELETE params missing from URL: %s", gotURL)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
