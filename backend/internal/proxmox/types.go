package proxmox

import "context"

type ProxmoxClient interface {
	GetNodeStatus(ctx context.Context, node string) (*NodeStatus, error)
	GetStorage(ctx context.Context, node string) ([]StorageInfo, error)
	CloneVM(ctx context.Context, node string, templateVMID, newVMID int, name, targetStorage string) (string, error)
	ConfigVM(ctx context.Context, node string, vmid int, cfg VMConfig) error
	ResizeDisk(ctx context.Context, node string, vmid int, disk string, sizeGB int) error
	StartVM(ctx context.Context, node string, vmid int) (string, error)
	ShutdownVM(ctx context.Context, node string, vmid int) (string, error)
	StopVM(ctx context.Context, node string, vmid int) (string, error)
	ResetVM(ctx context.Context, node string, vmid int) (string, error)
	DeleteVM(ctx context.Context, node string, vmid int) (string, error)
	GetVMStatus(ctx context.Context, node string, vmid int) (*VMCurrentStatus, error)
	GetVMInterfaces(ctx context.Context, node string, vmid int) ([]NetworkInterface, error)
	WaitForTask(ctx context.Context, node string, upid string) error
	GetNextVMID(ctx context.Context) (int, error)
}

type NodeStatus struct {
	CPU      float64 `json:"cpu"`
	Wait     float64 `json:"wait"`
	Memory   MemoryInfo `json:"memory"`
	RootFS   RootFSInfo `json:"rootfs"`
	Uptime   int64   `json:"uptime"`
	KVersion string  `json:"kversion"`
	PveVersion string `json:"pveversion"`
}

type MemoryInfo struct {
	Total int64 `json:"total"`
	Used  int64 `json:"used"`
	Free  int64 `json:"free"`
}

type RootFSInfo struct {
	Total int64 `json:"total"`
	Used  int64 `json:"used"`
	Free  int64 `json:"free"`
}

type StorageInfo struct {
	Storage string `json:"storage"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Total   int64  `json:"total"`
	Used    int64  `json:"used"`
	Avail   int64  `json:"avail"`
	Active  int    `json:"active"`
}

type VMConfig struct {
	Cores    int    `json:"cores"`
	MemoryMB int    `json:"memory"`
	CIUser   string `json:"ciuser"`
	SSHPublicKey string `json:"sshkeys"`
	IPConfig0 string `json:"ipconfig0"` // e.g. "ip=dhcp"
	Agent    int    `json:"agent"`     // 1 for enabled
}

type VMCurrentStatus struct {
	VMID      int     `json:"vmid"`
	Name      string  `json:"name"`
	Status    string  `json:"status"` // running, stopped, paused
	CPUs      int     `json:"cpus"`
	CPUUsage  float64 `json:"cpu"`
	MemoryMax int64   `json:"maxmem"`
	MemoryUsed int64  `json:"mem"`
	DiskMax   int64   `json:"maxdisk"`
	DiskUsed  int64   `json:"disk"`
	NetIn     int64   `json:"netin"`
	NetOut    int64   `json:"netout"`
	Uptime    int64   `json:"uptime"`
}

type NetworkInterface struct {
	Name            string   `json:"name"`
	HardwareAddress string   `json:"hardware-address"`
	IPAddresses     []IPInfo `json:"ip-addresses"`
}

type IPInfo struct {
	IPAddressType string `json:"ip-address-type"` // ipv4, ipv6
	IPAddress     string `json:"ip-address"`
	Prefix        int    `json:"prefix"`
}

type PVEApiResponse[T any] struct {
	Data   T       `json:"data"`
	Errors *string `json:"errors,omitempty"`
}
