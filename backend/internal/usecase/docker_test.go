package usecase

import (
	"testing"

	"github.com/nevatal/cloud/internal/proxmox"
)

func TestContainerRefPattern(t *testing.T) {
	valid := []string{"web", "app_db-1", "3f2a9c1b7d4e", "my.service"}
	invalid := []string{"", "-v", "--help", "a b", "a;rm -rf /", "../etc", "$(id)", "x/y", string(make([]byte, 200))}
	for _, v := range valid {
		if !containerRefPattern.MatchString(v) {
			t.Errorf("expected %q to be valid", v)
		}
	}
	for _, v := range invalid {
		if containerRefPattern.MatchString(v) {
			t.Errorf("expected %q to be rejected", v)
		}
	}
	for _, s := range []string{"5m", "1h", "30s", "9999h"} {
		if !logSincePattern.MatchString(s) {
			t.Errorf("since %q should be valid", s)
		}
	}
	for _, s := range []string{"1d", "-1h", "1h30m", "now", "10000s"} {
		if logSincePattern.MatchString(s) {
			t.Errorf("since %q should be rejected", s)
		}
	}
}

func TestParseDockerPS(t *testing.T) {
	out := `{"Command":"\"nginx\"","CreatedAt":"2026-11-02 08:00:00 +0000 UTC","ID":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","Image":"nginx:1.27","Names":"web","Ports":"0.0.0.0:80->80/tcp","State":"running","Status":"Up 3 hours"}
{"ID":"fedcba9876543210","Image":"busybox","Names":"/job","Status":"Exited (0) 2 days ago"}
not-json
`
	got := parseDockerPS(out)
	if len(got) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(got))
	}
	if got[0].Name != "web" || got[0].ID != "0123456789ab" || got[0].State != "running" {
		t.Fatalf("unexpected first container: %+v", got[0])
	}
	if got[1].Name != "job" || got[1].State != "exited" {
		t.Fatalf("state must be derived from Status when missing: %+v", got[1])
	}
}

func TestParseDockerStats(t *testing.T) {
	stats := parseDockerStats(`{"ID":"0123456789abcdef","Name":"web","CPUPerc":"1.5%","MemUsage":"10MiB / 1GiB","MemPerc":"1%"}`)
	if stats["web"].CPUPerc != "1.5%" || stats["0123456789ab"].MemPerc != "1%" {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestMergeDockerLogStreams(t *testing.T) {
	stdout := "2026-11-02T10:00:00.000000001Z first\n2026-11-02T10:00:02.000000000Z third\n"
	stderr := "2026-11-02T10:00:01.000000000Z second (err)\n2026-11-02T10:00:03.000000000Z fourth (err)\n"
	got := mergeDockerLogStreams(stdout, stderr)
	want := []struct{ msg, stream string }{{"first", "stdout"}, {"second (err)", "stderr"}, {"third", "stdout"}, {"fourth (err)", "stderr"}}
	if len(got) != len(want) {
		t.Fatalf("expected %d lines, got %d: %+v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i].Message != w.msg || got[i].Stream != w.stream || got[i].TS == "" {
			t.Fatalf("line %d: got %+v, want %+v", i, got[i], w)
		}
	}
	if lines := mergeDockerLogStreams("no timestamp here\n", ""); len(lines) != 1 || lines[0].TS != "" || lines[0].Message != "no timestamp here" {
		t.Fatalf("lines without timestamps must be kept verbatim: %+v", lines)
	}
}

func TestDockerFailureReason(t *testing.T) {
	cases := []struct {
		res  proxmox.AgentExecStatus
		want string
	}{
		{proxmox.AgentExecStatus{ExitCode: 127}, DockerReasonNotInstalled},
		{proxmox.AgentExecStatus{ExitCode: 1, ErrData: "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?"}, DockerReasonDaemonUnavailable},
		{proxmox.AgentExecStatus{ExitCode: 1, ErrData: "permission denied while trying to connect to the Docker daemon socket"}, DockerReasonPermissionDenied},
	}
	for _, tc := range cases {
		res := tc.res
		if code, _ := dockerFailureReason(&res); code != tc.want {
			t.Errorf("%+v: expected %s, got %s", tc.res, tc.want, code)
		}
	}
}
