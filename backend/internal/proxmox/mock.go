package proxmox

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type mockVM struct {
	vmid      int
	name      string
	status    string // running, stopped
	cores     int
	memoryMB  int
	diskGB    int
	ip        string
	ciuser    string
	sshkey    string
	startedAt time.Time
	snapshots map[string]*SnapshotInfo
}

type mockPVEClient struct {
	mu         sync.RWMutex
	nodeName   string
	nextVMID   int
	vms        map[int]*mockVM
	tasks      map[string]string // upid -> status (OK, RUNNING)
	deletedVMs map[int]bool
	execs      map[int]*AgentExecStatus // guest-agent pid -> finished result
	execPID    int
	// latency delays clone and start calls (PROXMOX_MOCK_LATENCY_MS), so
	// tests can interrupt a provisioning job while it runs.
	latency time.Duration
}

// simulateLatency waits for the configured latency or until ctx is done.
func (m *mockPVEClient) simulateLatency(ctx context.Context) error {
	if m.latency <= 0 {
		return nil
	}
	select {
	case <-time.After(m.latency):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func NewMockClient(nodeName string) ProxmoxClient {
	if nodeName == "" {
		nodeName = "pve-dell-r640"
	}
	m := &mockPVEClient{
		nodeName:   nodeName,
		nextVMID:   1002,
		vms:        make(map[int]*mockVM),
		tasks:      make(map[string]string),
		deletedVMs: make(map[int]bool),
		execs:      make(map[int]*AgentExecStatus),
		execPID:    4000,
	}

	// Pre-seed demo VM
	m.vms[1001] = &mockVM{
		vmid:      1001,
		name:      "web-app-production",
		status:    "running",
		cores:     2,
		memoryMB:  4096,
		diskGB:    60,
		ip:        "10.10.42.105",
		ciuser:    "ubuntu",
		startedAt: time.Now().Add(-48 * time.Hour),
	}

	return m
}

func (m *mockPVEClient) getOrInitVMLocked(vmid int) *mockVM {
	vm, exists := m.vms[vmid]
	if !exists {
		ipLastOctet := 100 + (vmid % 150)
		vm = &mockVM{
			vmid:      vmid,
			name:      fmt.Sprintf("instance-%d", vmid),
			status:    "running",
			cores:     2,
			memoryMB:  2048,
			diskGB:    40,
			ip:        fmt.Sprintf("10.10.42.%d", ipLastOctet),
			ciuser:    "ubuntu",
			startedAt: time.Now().Add(-2 * time.Hour),
			snapshots: make(map[string]*SnapshotInfo),
		}
		m.vms[vmid] = vm
	}
	if vm.snapshots == nil {
		vm.snapshots = make(map[string]*SnapshotInfo)
	}
	return vm
}

func (m *mockPVEClient) GetNodeStatus(ctx context.Context, node string) (*NodeStatus, error) {
	return &NodeStatus{
		CPU:  0.185,
		Wait: 0.012,
		Memory: MemoryInfo{
			Total: 268435456000, // 256 GB
			Used:  48318382080,  // ~45 GB
			Free:  220117073920,
		},
		RootFS: RootFSInfo{
			Total: 4123168604160, // 3.8 TB
			Used:  687194767360,  // 640 GB
			Free:  3435973836800,
		},
		Uptime:     1542890, // ~17 days
		KVersion:   "Linux 6.8.12-1-pve #1 SMP PREEMPT_DYNAMIC",
		PveVersion: "pve-manager/8.2.4/e8a70c7e0c4a45a3",
	}, nil
}

func (m *mockPVEClient) GetStorage(ctx context.Context, node string) ([]StorageInfo, error) {
	return []StorageInfo{
		{
			Storage: "local-zfs",
			Type:    "zfspool",
			Content: "images,rootdir",
			Total:   3840000000000,
			Used:    687194767360,
			Avail:   3152805232640,
			Active:  1,
		},
		{
			Storage: "local",
			Type:    "dir",
			Content: "iso,vztmpl,backup",
			Total:   100000000000,
			Used:    24500000000,
			Avail:   75500000000,
			Active:  1,
		},
	}, nil
}

func (m *mockPVEClient) GetNextVMID(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	vmid := m.nextVMID
	m.nextVMID++
	return vmid, nil
}

func (m *mockPVEClient) CloneVM(ctx context.Context, node string, templateVMID, newVMID int, name, targetStorage string) (string, error) {
	if err := m.simulateLatency(ctx); err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	upid := fmt.Sprintf("UPID:%s:%08X:00000001:66F81200:qemu-clone:%d:root@pam:", m.nodeName, newVMID, newVMID)
	m.tasks[upid] = "OK"

	delete(m.deletedVMs, newVMID)
	ipLastOctet := 100 + (newVMID % 150)
	m.vms[newVMID] = &mockVM{
		vmid:      newVMID,
		name:      name,
		status:    "stopped",
		cores:     2,
		memoryMB:  2048,
		diskGB:    40,
		ip:        fmt.Sprintf("10.10.42.%d", ipLastOctet),
		ciuser:    "ubuntu",
		startedAt: time.Time{},
	}

	return upid, nil
}

func (m *mockPVEClient) ConfigVM(ctx context.Context, node string, vmid int, cfg VMConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm := m.getOrInitVMLocked(vmid)

	if cfg.Cores > 0 {
		vm.cores = cfg.Cores
	}
	if cfg.MemoryMB > 0 {
		vm.memoryMB = cfg.MemoryMB
	}
	if cfg.CIUser != "" {
		vm.ciuser = cfg.CIUser
	}
	if cfg.SSHPublicKey != "" {
		vm.sshkey = cfg.SSHPublicKey
	}
	return nil
}

func (m *mockPVEClient) ResizeDisk(ctx context.Context, node string, vmid int, disk string, sizeGB int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	vm := m.getOrInitVMLocked(vmid)
	vm.diskGB += sizeGB
	return nil
}

func (m *mockPVEClient) StartVM(ctx context.Context, node string, vmid int) (string, error) {
	if err := m.simulateLatency(ctx); err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	vm := m.getOrInitVMLocked(vmid)
	vm.status = "running"
	vm.startedAt = time.Now()

	upid := fmt.Sprintf("UPID:%s:%08X:00000002:66F81210:qmstart:%d:root@pam:", m.nodeName, vmid, vmid)
	m.tasks[upid] = "OK"
	return upid, nil
}

func (m *mockPVEClient) ShutdownVM(ctx context.Context, node string, vmid int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm := m.getOrInitVMLocked(vmid)
	vm.status = "stopped"

	upid := fmt.Sprintf("UPID:%s:%08X:00000003:66F81220:qmshutdown:%d:root@pam:", m.nodeName, vmid, vmid)
	m.tasks[upid] = "OK"
	return upid, nil
}

func (m *mockPVEClient) StopVM(ctx context.Context, node string, vmid int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm := m.getOrInitVMLocked(vmid)
	vm.status = "stopped"

	upid := fmt.Sprintf("UPID:%s:%08X:00000004:66F81230:qmstop:%d:root@pam:", m.nodeName, vmid, vmid)
	m.tasks[upid] = "OK"
	return upid, nil
}

func (m *mockPVEClient) ResetVM(ctx context.Context, node string, vmid int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm := m.getOrInitVMLocked(vmid)
	vm.status = "running"
	vm.startedAt = time.Now()

	upid := fmt.Sprintf("UPID:%s:%08X:00000005:66F81240:qmreboot:%d:root@pam:", m.nodeName, vmid, vmid)
	m.tasks[upid] = "OK"
	return upid, nil
}

func (m *mockPVEClient) DeleteVM(ctx context.Context, node string, vmid int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deletedVMs[vmid] = true
	delete(m.vms, vmid)
	upid := fmt.Sprintf("UPID:%s:%08X:00000006:66F81250:qmdestroy:%d:root@pam:", m.nodeName, vmid, vmid)
	m.tasks[upid] = "OK"
	return upid, nil
}

func (m *mockPVEClient) GetVMStatus(ctx context.Context, node string, vmid int) (*VMCurrentStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.deletedVMs[vmid] {
		return nil, fmt.Errorf("VM %d does not exist", vmid)
	}

	vm := m.getOrInitVMLocked(vmid)

	var uptime int64
	var cpuUsage float64 = 0.0
	var memUsed int64 = 0

	if vm.status == "running" {
		uptime = int64(time.Since(vm.startedAt).Seconds())
		cpuUsage = 0.045                                      // 4.5%
		memUsed = int64(vm.memoryMB) * 1024 * 1024 * 35 / 100 // 35%
	}

	return &VMCurrentStatus{
		VMID:       vm.vmid,
		Name:       vm.name,
		Status:     vm.status,
		CPUs:       vm.cores,
		CPUUsage:   cpuUsage,
		MemoryMax:  int64(vm.memoryMB) * 1024 * 1024,
		MemoryUsed: memUsed,
		DiskMax:    int64(vm.diskGB) * 1024 * 1024 * 1024,
		DiskUsed:   int64(vm.diskGB) * 1024 * 1024 * 1024 * 15 / 100,
		NetIn:      1428590,
		NetOut:     3928100,
		Uptime:     uptime,
	}, nil
}

func (m *mockPVEClient) GetVMInterfaces(ctx context.Context, node string, vmid int) ([]NetworkInterface, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm := m.getOrInitVMLocked(vmid)

	return []NetworkInterface{
		{
			Name:            "lo",
			HardwareAddress: "00:00:00:00:00:00",
			IPAddresses: []IPInfo{
				{IPAddressType: "ipv4", IPAddress: "127.0.0.1", Prefix: 8},
			},
		},
		{
			Name:            "eth0",
			HardwareAddress: fmt.Sprintf("bc:24:11:a4:%02x:%02x", vmid/256, vmid%256),
			IPAddresses: []IPInfo{
				{IPAddressType: "ipv4", IPAddress: vm.ip, Prefix: 24},
			},
		},
	}, nil
}

func (m *mockPVEClient) CreateSnapshot(ctx context.Context, node string, vmid int, snapname, description string, vmstate bool) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm := m.getOrInitVMLocked(vmid)

	stateVal := 0
	if vmstate {
		stateVal = 1
	}

	vm.snapshots[snapname] = &SnapshotInfo{
		Name:        snapname,
		Description: description,
		SnapTime:    time.Now().Unix(),
		VMState:     stateVal,
	}

	upid := fmt.Sprintf("UPID:%s:%08X:00000007:66F81260:qmsnapshot:%d:root@pam:", m.nodeName, vmid, vmid)
	m.tasks[upid] = "OK"
	return upid, nil
}

func (m *mockPVEClient) ListSnapshots(ctx context.Context, node string, vmid int) ([]SnapshotInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm := m.getOrInitVMLocked(vmid)

	var list []SnapshotInfo
	for _, snap := range vm.snapshots {
		list = append(list, *snap)
	}
	return list, nil
}

func (m *mockPVEClient) RollbackSnapshot(ctx context.Context, node string, vmid int, snapname string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm := m.getOrInitVMLocked(vmid)

	if _, exists := vm.snapshots[snapname]; !exists {
		return "", fmt.Errorf("snapshot %s not found on vm %d", snapname, vmid)
	}

	upid := fmt.Sprintf("UPID:%s:%08X:00000008:66F81270:qmrollback:%d:root@pam:", m.nodeName, vmid, vmid)
	m.tasks[upid] = "OK"
	return upid, nil
}

func (m *mockPVEClient) DeleteSnapshot(ctx context.Context, node string, vmid int, snapname string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm := m.getOrInitVMLocked(vmid)

	delete(vm.snapshots, snapname)
	upid := fmt.Sprintf("UPID:%s:%08X:00000009:66F81280:qmdelsnapshot:%d:root@pam:", m.nodeName, vmid, vmid)
	m.tasks[upid] = "OK"
	return upid, nil
}

func (m *mockPVEClient) GetVNCProxy(ctx context.Context, node string, vmid int) (*VNCProxyTicket, error) {
	return &VNCProxyTicket{
		Ticket: fmt.Sprintf("PVE:termproxy:%s:ticket", node),
		Port:   5900,
		UPID:   fmt.Sprintf("UPID:%s:0000000A:00000001:66F81290:termproxy:%d:root@pam:", node, vmid),
		Cert:   "CERT_EXAMPLE_NEVATAL_CLOUD",
		User:   "root@pam",
	}, nil
}

func (m *mockPVEClient) WaitForTask(ctx context.Context, node string, upid string) error {
	// Realistic micro-delay to simulate hypervisor operations
	select {
	case <-time.After(1200 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
