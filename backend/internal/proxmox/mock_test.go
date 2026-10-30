package proxmox

import (
	"context"
	"testing"
)

func TestMockProxmoxClient(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient("pve-dell-r640")

	// 1. Check Node Status
	status, err := client.GetNodeStatus(ctx, "pve-dell-r640")
	if err != nil {
		t.Fatalf("GetNodeStatus failed: %v", err)
	}
	if status.Memory.Total == 0 || status.CPU == 0 {
		t.Errorf("Expected node telemetry data, got zeros: %+v", status)
	}

	// 2. Clone a new VM
	newVMID, err := client.GetNextVMID(ctx)
	if err != nil {
		t.Fatalf("GetNextVMID failed: %v", err)
	}

	upid, err := client.CloneVM(ctx, "pve-dell-r640", 9000, newVMID, "test-vm-01", "local-zfs")
	if err != nil {
		t.Fatalf("CloneVM failed: %v", err)
	}
	if upid == "" {
		t.Errorf("Expected non-empty UPID task string")
	}

	// 3. Configure VM
	cfg := VMConfig{
		Cores:    2,
		MemoryMB: 4096,
		CIUser:   "ubuntu",
	}
	if err := client.ConfigVM(ctx, "pve-dell-r640", newVMID, cfg); err != nil {
		t.Fatalf("ConfigVM failed: %v", err)
	}

	// 4. Start VM
	_, err = client.StartVM(ctx, "pve-dell-r640", newVMID)
	if err != nil {
		t.Fatalf("StartVM failed: %v", err)
	}

	// 5. Check VM status
	vmStatus, err := client.GetVMStatus(ctx, "pve-dell-r640", newVMID)
	if err != nil {
		t.Fatalf("GetVMStatus failed: %v", err)
	}
	if vmStatus.Status != "running" {
		t.Errorf("Expected VM status 'running', got '%s'", vmStatus.Status)
	}
	if vmStatus.CPUs != 2 {
		t.Errorf("Expected 2 CPUs, got %d", vmStatus.CPUs)
	}

	// 6. Network interfaces
	ifaces, err := client.GetVMInterfaces(ctx, "pve-dell-r640", newVMID)
	if err != nil || len(ifaces) == 0 {
		t.Fatalf("GetVMInterfaces failed: %v", err)
	}

	// 7. Delete VM
	_, err = client.DeleteVM(ctx, "pve-dell-r640", newVMID)
	if err != nil {
		t.Fatalf("DeleteVM failed: %v", err)
	}

	_, err = client.GetVMStatus(ctx, "pve-dell-r640", newVMID)
	if err == nil {
		t.Errorf("Expected error getting deleted VM status, got nil")
	}
}
