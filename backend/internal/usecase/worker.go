package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/domainroute"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/proxmox"
	"github.com/nevatal/cloud/internal/repository"
)

// WorkerUsecase runs VM provisioning, power and delete jobs in the
// background. Jobs outlive the HTTP request that queued them, so they never
// use a request context: database work uses context.Background() and
// hypervisor calls a Background-derived timeout.
type WorkerUsecase struct {
	tx           repository.Transactor
	jobs         repository.JobRepository
	instances    repository.InstanceRepository
	templates    repository.TemplateRepository
	nodes        repository.NodeRepository
	cfg          *config.Config
	proxmox      proxmox.ProxmoxClient
	quotaService *QuotaUsecase

	// heartbeatInterval is how often a running job refreshes heartbeat_at;
	// staleAfter is how long the reaper waits after the last sign of life
	// before failing a job as JOB_STALE.
	heartbeatInterval time.Duration
	staleAfter        time.Duration

	inflight sync.WaitGroup
}

// Job heartbeat defaults (overridable with JOB_HEARTBEAT_SECONDS and
// JOB_STALE_AFTER_SECONDS).
const (
	defaultJobHeartbeat  = 10 * time.Second
	defaultJobStaleAfter = 5 * time.Minute
)

// errJobNotActive aborts a terminal job write when the job already finished
// (for example the stale-job reaper failed it while its worker was stuck).
var errJobNotActive = errors.New("job is no longer active")

// WorkerRepos are the repositories WorkerUsecase works with.
type WorkerRepos struct {
	Jobs      repository.JobRepository
	Instances repository.InstanceRepository
	Templates repository.TemplateRepository
	Nodes     repository.NodeRepository
}

// Go runs a provisioning job in the background and tracks it so a graceful
// shutdown (SIGTERM during a rolling update) can wait for it.
func (w *WorkerUsecase) Go(job func()) {
	w.inflight.Add(1)
	go func() {
		defer w.inflight.Done()
		job()
	}()
}

// Wait blocks until all tracked jobs finish or ctx is done. It reports
// whether every job finished.
func (w *WorkerUsecase) Wait(ctx context.Context) bool {
	done := make(chan struct{})
	go func() {
		w.inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

func NewWorkerUsecase(tx repository.Transactor, repos WorkerRepos, cfg *config.Config, pve proxmox.ProxmoxClient, quotaService *QuotaUsecase) *WorkerUsecase {
	w := &WorkerUsecase{
		tx:                tx,
		jobs:              repos.Jobs,
		instances:         repos.Instances,
		templates:         repos.Templates,
		nodes:             repos.Nodes,
		cfg:               cfg,
		proxmox:           pve,
		quotaService:      quotaService,
		heartbeatInterval: defaultJobHeartbeat,
		staleAfter:        defaultJobStaleAfter,
	}
	if cfg != nil {
		if cfg.JobHeartbeatInterval > 0 {
			w.heartbeatInterval = cfg.JobHeartbeatInterval
		}
		if cfg.JobStaleAfter > 0 {
			w.staleAfter = cfg.JobStaleAfter
		}
	}
	return w
}

// SetJobTimings overrides the heartbeat interval and the stale threshold
// (tests).
func (w *WorkerUsecase) SetJobTimings(heartbeat, staleAfter time.Duration) {
	if heartbeat > 0 {
		w.heartbeatInterval = heartbeat
	}
	if staleAfter > 0 {
		w.staleAfter = staleAfter
	}
}

// startHeartbeat refreshes the job's heartbeat until the returned stop
// function is called. When the job is no longer active (it finished, or the
// reaper failed it), cancel is called so the worker stops touching it.
func (w *WorkerUsecase) startHeartbeat(jobID uuid.UUID, cancel context.CancelFunc) (stop func()) {
	_, _ = w.jobs.Heartbeat(context.Background(), jobID)
	done := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(w.heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				alive, err := w.jobs.Heartbeat(context.Background(), jobID)
				if err == nil && !alive {
					cancel()
					return
				}
			}
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}

// finishJob runs fn in a transaction after locking the job row, only while
// the job is still queued or in progress. It returns errJobNotActive
// otherwise, so a job failed by the reaper is never resurrected.
func (w *WorkerUsecase) finishJob(jobID uuid.UUID, fn func(txCtx context.Context) error) error {
	return w.tx.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		current, err := w.jobs.FindForUpdate(txCtx, jobID)
		if err != nil {
			return err
		}
		if current.Status != models.JobStatusQueued && current.Status != models.JobStatusInProgress {
			return errJobNotActive
		}
		return fn(txCtx)
	})
}

func (w *WorkerUsecase) ProcessJob(jobID uuid.UUID, sshKey *models.SSHKey) {
	dbCtx := context.Background()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	stopHeartbeat := w.startHeartbeat(jobID, cancel)
	defer stopHeartbeat()

	job, err := w.jobs.FindWithInstance(dbCtx, jobID)
	if err != nil {
		log.Printf("Worker: failed to find job %s: %v", jobID, err)
		return
	}

	if job.Instance == nil {
		w.failJob(job, "INVALID_STATE", "Instance record missing for job")
		return
	}

	now := time.Now()
	job.StartedAt = &now
	job.Status = models.JobStatusInProgress
	w.updateJobStep(job, "cloning_template", 20)

	inst := job.Instance
	_ = w.instances.UpdateStatus(dbCtx, inst, models.StateProvisioning)

	// Fetch template
	template, err := w.templates.FindByID(dbCtx, inst.OSTemplateID)
	if err != nil {
		w.failJob(job, "TEMPLATE_NOT_FOUND", "OS template not found in database")
		return
	}

	// Fetch node
	node, err := w.nodes.FindByID(dbCtx, inst.NodeID)
	if err != nil {
		w.failJob(job, "NODE_NOT_FOUND", "Hypervisor node not found")
		return
	}

	// 1. Allocate next Proxmox VMID
	newVMID, err := w.proxmox.GetNextVMID(ctx)
	if err != nil {
		w.failJob(job, "PROXMOX_API_ERROR", fmt.Sprintf("Failed to allocate next VMID from Proxmox: %v", err))
		return
	}

	inst.ProxmoxVMID = newVMID
	_ = w.instances.UpdateVMID(dbCtx, inst, newVMID)

	// 2. Clone VM from Template
	w.updateJobStep(job, "cloning_template", 30)
	cloneUPID, err := w.proxmox.CloneVM(ctx, node.ProxmoxNodeName, template.ProxmoxTemplateVMID, newVMID, inst.Name, w.cfg.ProxmoxStorage)
	if err != nil {
		w.failJobAndCleanup(job, inst, node.ProxmoxNodeName, newVMID, "CLONE_FAILED", fmt.Sprintf("Proxmox clone failed: %v", err))
		return
	}

	if err := w.proxmox.WaitForTask(ctx, node.ProxmoxNodeName, cloneUPID); err != nil {
		w.failJobAndCleanup(job, inst, node.ProxmoxNodeName, newVMID, "CLONE_TASK_FAILED", fmt.Sprintf("Clone task error: %v", err))
		return
	}

	// 3. Configure Hardware (cores, RAM)
	w.updateJobStep(job, "configuring_hardware", 50)
	var pubKeyStr string
	if sshKey != nil {
		pubKeyStr = sshKey.PublicKey
	}

	vmCfg := proxmox.VMConfig{
		Cores:        inst.VCPUs,
		MemoryMB:     inst.MemoryMB,
		CIUser:       inst.SSHUsername,
		SSHPublicKey: pubKeyStr,
		IPConfig0:    "ip=dhcp",
		Agent:        1,
	}

	if err := w.proxmox.ConfigVM(ctx, node.ProxmoxNodeName, newVMID, vmCfg); err != nil {
		w.failJobAndCleanup(job, inst, node.ProxmoxNodeName, newVMID, "CONFIG_FAILED", fmt.Sprintf("VM hardware config failed: %v", err))
		return
	}

	// 4. Configure Cloud-init and storage
	w.updateJobStep(job, "configuring_cloud_init", 70)
	// If requested disk is greater than base template disk (usually 20GB), resize disk
	if inst.DiskGB > 20 {
		_ = w.proxmox.ResizeDisk(ctx, node.ProxmoxNodeName, newVMID, "scsi0", inst.DiskGB-20)
	}

	// 5. Start VM
	w.updateJobStep(job, "starting_vm", 85)
	startUPID, err := w.proxmox.StartVM(ctx, node.ProxmoxNodeName, newVMID)
	if err != nil {
		w.failJobAndCleanup(job, inst, node.ProxmoxNodeName, newVMID, "START_FAILED", fmt.Sprintf("Starting VM failed: %v", err))
		return
	}
	_ = w.proxmox.WaitForTask(ctx, node.ProxmoxNodeName, startUPID)

	// 6. Acquire Network IP
	w.updateJobStep(job, "waiting_for_network", 95)
	var ipAddress string
	for attempt := 0; attempt < 8; attempt++ {
		interfaces, err := w.proxmox.GetVMInterfaces(ctx, node.ProxmoxNodeName, newVMID)
		if err == nil {
			ipAddress = selectGuestIPv4(interfaces, w.cfg.EdgeTargetCIDRs)
		}
		if ipAddress != "" {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if ipAddress == "" && w.cfg.ProxmoxMock {
		// Simulated hypervisor: keep the historical placeholder address. A
		// real VM never gets an invented address: nevatal-edge would route
		// its domains to whichever host owns it. Its address is retried
		// below and refreshed again on every start/restart.
		ipAddress = fmt.Sprintf("10.10.42.%d", 100+(newVMID%150))
	}

	// 7. Complete job and commit reservation (only while the job is still
	// active: a job the stale-job reaper failed meanwhile stays failed).
	err = w.finishJob(job.ID, func(txCtx context.Context) error {
		_ = w.quotaService.CommitReservation(txCtx, job.ID)
		if ipAddress != "" {
			inst.PrivateIP = &ipAddress
		}
		inst.Status = models.StateRunning
		if err := w.instances.Save(txCtx, inst); err != nil {
			return err
		}

		completedAt := time.Now()
		job.Status = models.JobStatusCompleted
		job.CurrentStep = "completed"
		job.ProgressPercent = 100
		job.CompletedAt = &completedAt
		return w.jobs.Save(txCtx, job)
	})
	if err != nil {
		log.Printf("Worker: job %s for instance %s not completed: %v", job.ID, inst.Name, err)
		return
	}

	log.Printf("Worker: Instance %s (VMID %d, IP %s) provisioned successfully!", inst.Name, inst.ProxmoxVMID, ipAddress)

	if ipAddress == "" {
		// The guest agent did not report an address yet; keep trying a
		// little longer in the background job.
		inst.Node = node
		w.refreshPrivateIP(ctx, inst)
	}
}

func (w *WorkerUsecase) ProcessPowerJob(jobID uuid.UUID, operation string) {
	dbCtx := context.Background()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	stopHeartbeat := w.startHeartbeat(jobID, cancel)
	defer stopHeartbeat()

	job, err := w.jobs.FindWithInstanceNode(dbCtx, jobID)
	if err != nil {
		return
	}

	inst := job.Instance
	if inst == nil || inst.Node == nil {
		w.failJob(job, "INVALID_STATE", "Instance or node missing")
		return
	}

	var upid string
	var finalState string

	switch operation {
	case models.OpStart:
		upid, err = w.proxmox.StartVM(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
		finalState = models.StateRunning
	case models.OpShutdown:
		upid, err = w.proxmox.ShutdownVM(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
		finalState = models.StateStopped
	case models.OpRestart:
		upid, err = w.proxmox.ResetVM(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
		finalState = models.StateRunning
	}

	if err != nil {
		if w.failJob(job, "POWER_OP_FAILED", err.Error()) {
			_ = w.instances.UpdateStatus(dbCtx, inst, models.StateUnknown)
		}
		return
	}

	if upid != "" {
		_ = w.proxmox.WaitForTask(ctx, inst.Node.ProxmoxNodeName, upid)
	}

	if finalState == models.StateRunning {
		// DHCP leases can change across a stop/start or reset. Refresh the
		// address nevatal-edge routes to before the instance is reported
		// running (the edge only proxies to running instances).
		w.refreshPrivateIP(ctx, inst)
	}

	err = w.finishJob(job.ID, func(txCtx context.Context) error {
		completedAt := time.Now()
		_ = w.instances.UpdateStatus(txCtx, inst, finalState)
		job.Status = models.JobStatusCompleted
		job.CurrentStep = "completed"
		job.ProgressPercent = 100
		job.CompletedAt = &completedAt
		return w.jobs.Save(txCtx, job)
	})
	if err != nil {
		log.Printf("Worker: %s job %s not completed: %v", operation, job.ID, err)
	}
}

func (w *WorkerUsecase) ProcessDeleteJob(jobID uuid.UUID) {
	dbCtx := context.Background()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	stopHeartbeat := w.startHeartbeat(jobID, cancel)
	defer stopHeartbeat()

	job, err := w.jobs.FindWithInstanceNode(dbCtx, jobID)
	if err != nil {
		return
	}

	inst := job.Instance
	if inst != nil && inst.ProxmoxVMID > 0 && inst.Node != nil {
		// Stop if running
		_, _ = w.proxmox.StopVM(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
		time.Sleep(500 * time.Millisecond)
		// Delete in Proxmox
		_, _ = w.proxmox.DeleteVM(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
	}

	// Release reservations and soft delete instance
	err = w.finishJob(job.ID, func(txCtx context.Context) error {
		_ = w.quotaService.ReleaseReservation(txCtx, job.ID)
		if inst != nil {
			inst.Status = "deleted"
			_ = w.instances.Save(txCtx, inst)
			_ = w.instances.SoftDelete(txCtx, inst) // Gorm soft delete
		}
		completedAt := time.Now()
		job.Status = models.JobStatusCompleted
		job.CurrentStep = "deleted"
		job.ProgressPercent = 100
		job.CompletedAt = &completedAt
		return w.jobs.Save(txCtx, job)
	})
	if err != nil {
		log.Printf("Worker: delete job %s not completed: %v", job.ID, err)
	}
}

func (w *WorkerUsecase) updateJobStep(job *models.ProvisioningJob, step string, progress int) {
	job.CurrentStep = step
	job.ProgressPercent = progress
	_ = w.jobs.UpdateProgress(context.Background(), job, step, progress)
}

// failJob marks an active job failed, marks its instance failed and releases
// its reservation in one transaction. It reports false when the job had
// already finished (for example it was failed by the stale-job reaper).
func (w *WorkerUsecase) failJob(job *models.ProvisioningJob, code, message string) bool {
	err := w.finishJob(job.ID, func(txCtx context.Context) error {
		completedAt := time.Now()
		job.Status = models.JobStatusFailed
		job.ErrorCode = &code
		job.ErrorMessage = &message
		job.CompletedAt = &completedAt
		if err := w.jobs.Save(txCtx, job); err != nil {
			return err
		}
		if job.InstanceID != nil {
			_ = w.instances.UpdateStatusByID(txCtx, *job.InstanceID, models.StateFailed)
		}
		return w.quotaService.ReleaseReservation(txCtx, job.ID)
	})
	if err != nil {
		log.Printf("Worker: job %s not marked failed (%s): %v", job.ID, code, err)
		return false
	}
	return true
}

// failJobAndCleanup removes the partially created VM first and then fails the
// job, so a failed create job is terminal only once its VM is gone.
func (w *WorkerUsecase) failJobAndCleanup(job *models.ProvisioningJob, inst *models.Instance, node string, vmid int, code, message string) {
	if vmid > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, _ = w.proxmox.StopVM(ctx, node, vmid)
		_, _ = w.proxmox.DeleteVM(ctx, node, vmid)
		cancel()
	}
	w.failJob(job, code, message)
}

// StartReaper fails stale jobs every interval until ctx is done. Every API
// replica runs it; ReapStale's SKIP LOCKED update makes each job reaped once.
func (w *WorkerUsecase) StartReaper(ctx context.Context) {
	interval := w.staleAfter / 4
	if interval > 30*time.Second {
		interval = 30 * time.Second
	}
	if interval < time.Second {
		interval = time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := w.ReapStaleJobs(ctx); err != nil {
					log.Printf("Job reaper: %v", err)
				} else if n > 0 {
					log.Printf("Job reaper: failed %d stale provisioning job(s) as %s", n, models.JobErrorStale)
				}
			}
		}
	}()
}

// ReapStaleJobs runs one reaper pass: it fails jobs without a heartbeat for
// staleAfter as JOB_STALE and cleans up after them (releases the
// reservation; a create job's partially created VM is deleted when its VMID
// is known and its instance is marked failed; other operations leave the
// instance in state unknown). It returns the number of jobs reaped.
func (w *WorkerUsecase) ReapStaleJobs(ctx context.Context) (int, error) {
	reaped, err := w.jobs.ReapStale(context.WithoutCancel(ctx), w.staleAfter, 50)
	if err != nil {
		return 0, err
	}
	for i := range reaped {
		w.cleanupReapedJob(&reaped[i])
	}
	return len(reaped), nil
}

func (w *WorkerUsecase) cleanupReapedJob(job *models.ProvisioningJob) {
	dbCtx := context.Background()
	_ = w.tx.WithinTransaction(dbCtx, func(txCtx context.Context) error {
		return w.quotaService.ReleaseReservation(txCtx, job.ID)
	})
	inst := job.Instance
	if inst == nil || inst.DeletedAt.Valid {
		return
	}
	switch job.OperationType {
	case models.OpCreate:
		if inst.ProxmoxVMID > 0 && inst.Node != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, _ = w.proxmox.StopVM(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
			_, _ = w.proxmox.DeleteVM(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
			cancel()
		}
		_ = w.instances.UpdateStatusByID(dbCtx, inst.ID, models.StateFailed)
	default:
		_ = w.instances.UpdateStatusByID(dbCtx, inst.ID, models.StateUnknown)
	}
	log.Printf("Job reaper: job %s (%s) of instance %s failed as %s", job.ID, job.OperationType, inst.Name, models.JobErrorStale)
}

// Guest address refresh after start/restart: a few attempts while the guest
// agent comes up.
const (
	ipRefreshAttempts = 5
	ipRefreshDelay    = 2 * time.Second
)

// refreshPrivateIP re-reads the VM's address from the QEMU guest agent and
// stores it when it changed. It is best effort: when the agent does not
// answer, the previous address is kept (never cleared).
func (w *WorkerUsecase) refreshPrivateIP(ctx context.Context, inst *models.Instance) {
	if inst.Node == nil || inst.ProxmoxVMID <= 0 {
		return
	}
	for attempt := 0; attempt < ipRefreshAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(ipRefreshDelay):
			}
		}
		interfaces, err := w.proxmox.GetVMInterfaces(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
		if err != nil {
			continue
		}
		ip := selectGuestIPv4(interfaces, w.cfg.EdgeTargetCIDRs)
		if ip == "" {
			continue
		}
		if inst.PrivateIP != nil && *inst.PrivateIP == ip {
			return
		}
		if err := w.instances.UpdatePrivateIP(context.Background(), inst.ID, ip); err != nil {
			log.Printf("Worker: failed to store private IP of instance %s: %v", inst.Name, err)
			return
		}
		log.Printf("Worker: instance %s private IP is now %s", inst.Name, ip)
		inst.PrivateIP = &ip
		return
	}
}

// skippedGuestInterfacePrefixes name guest interfaces whose addresses are
// never the VM's LAN address: container bridges and overlays (Docker gives
// docker0 172.17.0.1), libvirt/LXD bridges and VPN tunnels.
var skippedGuestInterfacePrefixes = []string{
	"docker", "br-", "veth", "virbr", "cni", "flannel", "cali", "tailscale", "wg",
	"lxcbr", "lxdbr", "podman", "kube-", "zt", "tun",
}

func isSkippedGuestInterface(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "lo" {
		return true
	}
	for _, prefix := range skippedGuestInterfacePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// selectGuestIPv4 picks the address of a VM from the guest agent's
// interface list, in this order:
//
//  1. an IPv4 inside one of preferred (EDGE_TARGET_CIDRS) on a regular
//     interface;
//  2. any other IPv4 on a regular interface;
//  3. the original rule: the first IPv4 other than 127.0.0.1 on any
//     interface but lo.
//
// Regular interfaces exclude lo and container/VPN interfaces (docker*,
// br-*, veth*, ...). Steps 1 and 2 skip loopback, link-local, multicast and
// unspecified addresses. It returns "" when no IPv4 address is reported.
func selectGuestIPv4(interfaces []proxmox.NetworkInterface, preferred []netip.Prefix) string {
	var firstRegular string
	for _, iface := range interfaces {
		if isSkippedGuestInterface(iface.Name) {
			continue
		}
		for _, ip := range iface.IPAddresses {
			if ip.IPAddressType != "ipv4" {
				continue
			}
			addr, ok := domainroute.ParseAddr(ip.IPAddress)
			if !ok || !addr.Is4() || addr.Zone() != "" || !domainroute.IsUsableUnicast(addr) {
				continue
			}
			for _, prefix := range preferred {
				if prefix.Contains(addr) {
					return addr.String()
				}
			}
			if firstRegular == "" {
				firstRegular = addr.String()
			}
		}
	}
	if firstRegular != "" {
		return firstRegular
	}

	for _, iface := range interfaces {
		if iface.Name == "lo" {
			continue
		}
		for _, ip := range iface.IPAddresses {
			if ip.IPAddressType == "ipv4" && ip.IPAddress != "127.0.0.1" {
				return ip.IPAddress
			}
		}
	}
	return ""
}
