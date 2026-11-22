package usecase

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
)

func TestScrubSensitive(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	input := `User login token: Bearer ` + jwt + ` with "password":"SuperSecret123!"`
	scrubbed := ScrubSensitive(input)

	if strings.Contains(scrubbed, "SuperSecret123!") {
		t.Errorf("Expected password to be scrubbed, got: %s", scrubbed)
	}
	if strings.Contains(scrubbed, jwt) {
		t.Errorf("Expected JWT to be scrubbed, got: %s", scrubbed)
	}
	if !strings.Contains(scrubbed, "[REDACTED]") {
		t.Errorf("Expected redacted marker in scrubbed output, got: %s", scrubbed)
	}
}

func TestCalculateVMHealthScore(t *testing.T) {
	u := &ObservabilityUsecase{}

	// Test 1: Ideal VM
	vmNormal := models.Instance{
		ID:     uuid.New(),
		Status: models.StateRunning,
	}
	hsNormal := u.CalculateVMHealthScore(vmNormal, 15.0, 40.0, 30.0, true)
	if hsNormal.Score != 100 {
		t.Errorf("Expected score 100 for normal VM, got %d", hsNormal.Score)
	}
	if hsNormal.Status != "Healthy" {
		t.Errorf("Expected status Healthy, got %s", hsNormal.Status)
	}

	// Test 2: Degraded VM with memory pressure and no guest agent
	hsDegraded := u.CalculateVMHealthScore(vmNormal, 82.0, 88.0, 50.0, false)
	if hsDegraded.Score >= 100 || hsDegraded.Score <= 0 {
		t.Errorf("Expected degraded score between 0 and 100, got %d", hsDegraded.Score)
	}
	if len(hsDegraded.Reasons) < 2 {
		t.Errorf("Expected multiple reasons for degraded state, got %d", len(hsDegraded.Reasons))
	}

	// Test 3: Critical failed VM
	vmFailed := models.Instance{
		ID:     uuid.New(),
		Status: models.StateFailed,
	}
	hsCritical := u.CalculateVMHealthScore(vmFailed, 95.0, 95.0, 95.0, false)
	if hsCritical.Score > 30 {
		t.Errorf("Expected critical score <= 30, got %d", hsCritical.Score)
	}
	if hsCritical.Status != "Critical" {
		t.Errorf("Expected status Critical, got %s", hsCritical.Status)
	}
}

func TestAPIMetricsRecorder(t *testing.T) {
	rec := &APIMetricsRecorder{
		samples: make([]APIMetricSample, 0, 100),
		maxCap:  100,
	}

	start := time.Now()
	rec.Record(APIMetricSample{
		Timestamp:  start,
		Method:     "GET",
		Path:       "/api/v1/instances",
		StatusCode: 200,
		Duration:   45 * time.Millisecond,
	})
	rec.Record(APIMetricSample{
		Timestamp:  start,
		Method:     "POST",
		Path:       "/api/v1/instances",
		StatusCode: 500,
		Duration:   210 * time.Millisecond,
	})

	samples := rec.Snapshot(start.Add(-1 * time.Minute))
	if len(samples) != 2 {
		t.Fatalf("Expected 2 samples, got %d", len(samples))
	}
}

func TestBroadcaster(t *testing.T) {
	b := newSSEBroadcaster()
	ch, unsub := b.subscribe()
	defer unsub()

	evt := ObservabilitySSEEvent{
		Event: "test_event",
		Data:  map[string]string{"foo": "bar"},
	}
	b.broadcast(evt)

	select {
	case received := <-ch:
		if received.Event != "test_event" {
			t.Errorf("Expected test_event, got %s", received.Event)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for broadcast event")
	}
}
