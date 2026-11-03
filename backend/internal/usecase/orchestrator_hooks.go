package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
	"github.com/nevatal/cloud/internal/repository"
)

// Engine hooks: execute-time policy, verification, reports and the plan /
// conversation side effects of execution status changes.

var _ orchestrator.Hooks = (*OrchestratorUsecase)(nil)

// CheckTask re-runs the policy engine for one task right before it runs.
func (s *OrchestratorUsecase) CheckTask(ctx context.Context, exec *models.AIExecution, task *models.AIExecutionTask, input map[string]any) *orchestrator.TaskError {
	return s.policy.CheckTask(ctx, exec.UserID, task.Tool, input)
}

func check(name string, ok bool, detail string) orchestrator.Check {
	status := "pass"
	if !ok {
		status = "fail"
	}
	return orchestrator.Check{Name: name, Status: status, Detail: detail}
}

func skip(name, detail string) orchestrator.Check {
	return orchestrator.Check{Name: name, Status: "skip", Detail: detail}
}

const simulatedDetail = "simulated hypervisor (PROXMOX_MOCK=true): not observable"

// verifyVPS compares an instance with the desired power state: the instance
// record, the hypervisor's VM status and (for running) a private address
// reported by the guest agent. Checks that cannot be observed (simulated
// hypervisor, unreachable host) are skipped, which leaves the result
// unverified, never verified.
func (s *OrchestratorUsecase) verifyVPS(ctx context.Context, userID, instanceID uuid.UUID, want string, wantIP bool) *orchestrator.Verification {
	v := &orchestrator.Verification{}
	inst, err := s.repos.Instances.FindByID(ctx, instanceID, &userID)
	if err != nil {
		v.Checks = append(v.Checks, check("instance_record", false, "instance not found"))
		return v.Summarize()
	}
	v.Checks = append(v.Checks, check("instance_record", inst.Status == want, fmt.Sprintf("recorded status %s (want %s)", inst.Status, want)))

	switch {
	case s.cfg.ProxmoxMock:
		v.Checks = append(v.Checks, skip("hypervisor_power_state", simulatedDetail))
	case inst.Node == nil || inst.ProxmoxVMID <= 0:
		v.Checks = append(v.Checks, check("hypervisor_power_state", false, "the instance has no hypervisor placement"))
	default:
		pveCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		st, err := s.pve.GetVMStatus(pveCtx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
		cancel()
		if err != nil {
			v.Checks = append(v.Checks, skip("hypervisor_power_state", "hypervisor unreachable"))
		} else {
			v.Checks = append(v.Checks, check("hypervisor_power_state", st.Status == want, "hypervisor reports "+st.Status))
		}
	}

	if wantIP {
		switch {
		case s.cfg.ProxmoxMock:
			if inst.PrivateIP != nil && *inst.PrivateIP != "" {
				v.Checks = append(v.Checks, skip("private_ip", "simulated address "+*inst.PrivateIP+"; guest agent not observable"))
			} else {
				v.Checks = append(v.Checks, check("private_ip", false, "no address recorded"))
			}
		case inst.Node == nil || inst.ProxmoxVMID <= 0:
			v.Checks = append(v.Checks, check("private_ip", false, "the instance has no hypervisor placement"))
		default:
			pveCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			ifaces, err := s.pve.GetVMInterfaces(pveCtx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
			cancel()
			if err != nil {
				v.Checks = append(v.Checks, skip("private_ip", "guest agent unreachable"))
			} else if ip := selectGuestIPv4(ifaces, s.cfg.EdgeTargetCIDRs); ip != "" {
				v.Checks = append(v.Checks, check("private_ip", true, "guest agent reports "+ip))
			} else {
				v.Checks = append(v.Checks, check("private_ip", false, "guest agent reports no IPv4 address"))
			}
		}
	}
	return v.Summarize()
}

// verifyVPSAbsent checks that a deleted instance is gone from the records
// and from the hypervisor.
func (s *OrchestratorUsecase) verifyVPSAbsent(ctx context.Context, userID, instanceID uuid.UUID, output map[string]any) *orchestrator.Verification {
	v := &orchestrator.Verification{}
	_, err := s.repos.Instances.FindByID(ctx, instanceID, &userID)
	v.Checks = append(v.Checks, check("instance_record_absent", errors.Is(err, repository.ErrNotFound), "instance record deleted"))
	vmid := orchestrator.Int(output, "vmid", 0)
	node := orchestrator.String(output, "node")
	switch {
	case s.cfg.ProxmoxMock:
		v.Checks = append(v.Checks, skip("hypervisor_vm_absent", simulatedDetail))
	case vmid <= 0 || node == "":
		v.Checks = append(v.Checks, check("hypervisor_vm_absent", true, "the instance never had a VM"))
	default:
		pveCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := s.pve.GetVMStatus(pveCtx, node, vmid)
		cancel()
		switch {
		case err == nil:
			v.Checks = append(v.Checks, check("hypervisor_vm_absent", false, fmt.Sprintf("VM %d still exists", vmid)))
		case strings.Contains(strings.ToLower(err.Error()), "does not exist") || strings.Contains(err.Error(), "500") || strings.Contains(err.Error(), "404"):
			v.Checks = append(v.Checks, check("hypervisor_vm_absent", true, fmt.Sprintf("VM %d is gone", vmid)))
		default:
			v.Checks = append(v.Checks, skip("hypervisor_vm_absent", "hypervisor unreachable"))
		}
	}
	return v.Summarize()
}

// Verify compares the desired state of a finished write task with reality.
func (s *OrchestratorUsecase) Verify(ctx context.Context, exec *models.AIExecution, task *models.AIExecutionTask, output map[string]any) *orchestrator.Verification {
	user := exec.UserID
	idFrom := func(key string) uuid.UUID {
		id, _ := orchestrator.UUID(output, key)
		return id
	}
	runtime := func(kind string) orchestrator.Check {
		if s.cfg.ProxmoxMock {
			return skip("runtime_probe", simulatedDetail)
		}
		return skip("runtime_probe", "no runtime probe for "+kind+" in Phase 1")
	}
	row := func(found bool, status string, want ...string) orchestrator.Check {
		if !found {
			return check("resource_record", false, "resource not found")
		}
		for _, w := range want {
			if status == w {
				return check("resource_record", true, "status "+status)
			}
		}
		return check("resource_record", false, fmt.Sprintf("status %s (want %s)", status, strings.Join(want, " or ")))
	}
	v := &orchestrator.Verification{}
	switch task.Tool {
	case orchestrator.ToolVPSCreate, orchestrator.ToolVPSStart, orchestrator.ToolVPSRestart:
		return s.verifyVPS(ctx, user, idFrom("instance_id"), models.StateRunning, true)
	case orchestrator.ToolVPSStop:
		return s.verifyVPS(ctx, user, idFrom("instance_id"), models.StateStopped, false)
	case orchestrator.ToolVPSDelete:
		return s.verifyVPSAbsent(ctx, user, idFrom("instance_id"), output)
	case orchestrator.ToolVPSResize:
		inst, err := s.repos.Instances.FindByID(ctx, idFrom("instance_id"), &user)
		if err != nil {
			v.Checks = append(v.Checks, check("instance_record", false, "instance not found"))
			break
		}
		var in map[string]any
		_ = orchestrator.DecodeJSON(task.Input, &in)
		ok := inst.VCPUs == orchestrator.Int(in, "vcpus", -1) && inst.MemoryMB == orchestrator.Int(in, "memory_mb", -1) && inst.DiskGB == orchestrator.Int(in, "disk_gb", -1)
		v.Checks = append(v.Checks, check("instance_size", ok, fmt.Sprintf("%d vCPU, %d MB, %d GB", inst.VCPUs, inst.MemoryMB, inst.DiskGB)))
		if s.cfg.ProxmoxMock {
			v.Checks = append(v.Checks, skip("hypervisor_config", simulatedDetail))
		} else {
			v.Checks = append(v.Checks, skip("hypervisor_config", "hypervisor configuration is not compared in Phase 1"))
		}
	case orchestrator.ToolVPSSnapshotCreate:
		id := idFrom("instance_id")
		n, _ := s.repos.Snapshots.CountByName(ctx, id, orchestrator.String(output, "snapshot_name"))
		v.Checks = append(v.Checks, check("snapshot_record", n > 0, "snapshot recorded"))
		if s.cfg.ProxmoxMock {
			v.Checks = append(v.Checks, skip("hypervisor_snapshot", simulatedDetail))
		} else if inst, err := s.repos.Instances.FindByID(ctx, id, &user); err == nil && inst.Node != nil && inst.ProxmoxVMID > 0 {
			pveCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			snaps, err := s.pve.ListSnapshots(pveCtx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
			cancel()
			if err != nil {
				v.Checks = append(v.Checks, skip("hypervisor_snapshot", "hypervisor unreachable"))
			} else {
				found := false
				for _, sn := range snaps {
					if sn.Name == orchestrator.String(output, "snapshot_name") {
						found = true
					}
				}
				v.Checks = append(v.Checks, check("hypervisor_snapshot", found, "snapshot listed by the hypervisor"))
			}
		} else {
			v.Checks = append(v.Checks, check("hypervisor_snapshot", false, "the instance has no hypervisor placement"))
		}
	case orchestrator.ToolVPSSnapshotRestore:
		_, err := s.repos.Instances.FindByID(ctx, idFrom("instance_id"), &user)
		v.Checks = append(v.Checks, check("instance_record", err == nil, "instance exists"))
		v.Checks = append(v.Checks, skip("hypervisor_rollback", "a rollback cannot be observed after the fact"))
	case orchestrator.ToolBucketCreate:
		b, err := s.repos.Storage.FindBucket(ctx, idFrom("bucket_id"), user)
		v.Checks = append(v.Checks, check("bucket_record", err == nil, "bucket recorded"))
		if err == nil {
			_, statErr := os.Stat(filepath.Join(s.svc.Storage.basePath, b.Name))
			v.Checks = append(v.Checks, check("bucket_storage", statErr == nil, "bucket directory present on the storage volume"))
		}
	case orchestrator.ToolBucketDelete:
		_, err := s.repos.Storage.FindBucket(ctx, idFrom("bucket_id"), user)
		v.Checks = append(v.Checks, check("bucket_record_absent", errors.Is(err, repository.ErrNotFound), "bucket record deleted"))
		if name := orchestrator.String(output, "name"); name != "" {
			_, statErr := os.Stat(filepath.Join(s.svc.Storage.basePath, name))
			v.Checks = append(v.Checks, check("bucket_storage_absent", os.IsNotExist(statErr), "bucket directory removed"))
		}
	case orchestrator.ToolAppCreate:
		a, err := s.svc.App.Get(ctx, idFrom("application_id"), user, false)
		status := ""
		if err == nil {
			status = a.Status
		}
		v.Checks = append(v.Checks, row(err == nil, status, "building", "deploying", "running", "active"), runtime("applications"))
	case orchestrator.ToolN8nCreate:
		a, err := s.svc.Automation.Get(ctx, idFrom("automation_id"), user, false)
		status := ""
		if err == nil {
			status = a.Status
		}
		v.Checks = append(v.Checks, row(err == nil, status, "running", "provisioning"), runtime("n8n instances"))
	case orchestrator.ToolOpenClawCreate:
		a, err := s.svc.OpenClaw.Get(ctx, idFrom("agent_id"), user, false)
		status := ""
		if err == nil {
			status = a.Status
		}
		v.Checks = append(v.Checks, row(err == nil, status, "running", "provisioning"), runtime("OpenClaw agents"))
	case orchestrator.ToolGameServerCreate:
		g, err := s.svc.GameServer.Get(ctx, idFrom("game_server_id"), user, false)
		status := ""
		if err == nil {
			status = g.Status
		}
		v.Checks = append(v.Checks, row(err == nil, status, models.GameServerStatusRunning, "provisioning", "installing", "starting"), runtime("game servers"))
	case orchestrator.ToolAndroidCreate:
		a, err := s.svc.Android.Get(ctx, idFrom("android_instance_id"), user, false)
		status := ""
		if err == nil {
			status = a.Status
		}
		v.Checks = append(v.Checks, row(err == nil, status, models.AndroidStatusRunning, "provisioning"), runtime("Android instances"))
	case orchestrator.ToolServicePower:
		in := taskInput(task)
		typ := orchestrator.String(in, "resource_type")
		id := idFrom("resource_id")
		want := models.StateRunning
		if orchestrator.String(in, "action") == orchestrator.ActionStop {
			want = models.StateStopped
		}
		return s.verifyServiceState(ctx, user, typ, id, want)
	case orchestrator.ToolServiceDelete:
		in := taskInput(task)
		typ := orchestrator.String(in, "resource_type")
		return s.verifyServiceAbsent(ctx, user, typ, idFrom("resource_id"))
	case orchestrator.ToolDatabaseCreate:
		db, err := s.svc.Database.Get(ctx, idFrom("database_id"), user)
		status := ""
		if err == nil {
			status = db.Status
		}
		v.Checks = append(v.Checks, row(err == nil, status, models.DatabaseStatusRunning, "provisioning"))
		v.Checks = append(v.Checks, skip("database_probe", "a live connection probe is not performed for managed databases"))
	case orchestrator.ToolAppDeploy:
		app, err := s.svc.App.Get(ctx, idFrom("application_id"), user, false)
		if err != nil {
			v.Checks = append(v.Checks, check("application_record", false, "application not found"))
			break
		}
		ok := app.Status != "failed"
		detail := "application status " + app.Status
		if len(app.Deployments) > 0 {
			dep := app.Deployments[0]
			detail = "latest deployment " + dep.Status
			ok = dep.Status != "failed" && (orchestrator.String(output, "deployment_id") == "" || dep.ID.String() == orchestrator.String(output, "deployment_id"))
		}
		v.Checks = append(v.Checks, check("deployment_record", ok, detail))
		v.Checks = append(v.Checks, runtime("application health"))
	case orchestrator.ToolServiceBackupCreate:
		v.Checks = append(v.Checks, check("backup_record", orchestrator.String(output, "backup_id") != "", "backup was recorded"))
		v.Checks = append(v.Checks, skip("backup_integrity", "backup contents are verified by the storage backend, not by the control plane"))
	case orchestrator.ToolServiceBackupRestore:
		v.Checks = append(v.Checks, skip("restore_result", "a restore cannot be observed after the fact"))
	case orchestrator.ToolBucketUpdate:
		in := taskInput(task)
		b, err := s.repos.Storage.FindBucket(ctx, idFrom("bucket_id"), user)
		if err != nil {
			v.Checks = append(v.Checks, check("bucket_record", false, "bucket not found"))
		} else {
			ok := true
			if raw, present := in["is_public"]; present {
				if wantPublic, isBool := raw.(bool); isBool {
					ok = (b.Visibility == "public") == wantPublic
				}
			}
			v.Checks = append(v.Checks, check("bucket_visibility", ok, "visibility is "+b.Visibility))
		}
	case orchestrator.ToolBucketObjectDelete:
		objID := idFrom("object_id")
		_, err := s.svc.Storage.GetObject(ctx, objID)
		v.Checks = append(v.Checks, check("object_absent", err != nil, "object record removed"))
	case orchestrator.ToolDomainClaim:
		v.Checks = append(v.Checks, check("domain_record", orchestrator.String(output, "domain_id") != "", "domain was recorded"))
		v.Checks = append(v.Checks, skip("domain_routing", "edge routing is verified by the edge health endpoint"))
	case orchestrator.ToolDomainRelease:
		v.Checks = append(v.Checks, check("domain_record", true, "domain release recorded"))
		v.Checks = append(v.Checks, skip("domain_routing", "edge routing is verified by the edge health endpoint"))
	default:
		v.Checks = append(v.Checks, skip("verifier", "no verifier for "+task.Tool))
	}
	if s.cfg.ProxmoxMock && !hasSkip(v) {
		// The whole platform is simulated: never report verified.
		v.Checks = append(v.Checks, skip("simulated_platform", "PROXMOX_MOCK=true: the platform is simulated"))
	}
	return v.Summarize()
}

func hasSkip(v *orchestrator.Verification) bool {
	for _, c := range v.Checks {
		if c.Status == "skip" {
			return true
		}
	}
	return false
}

func taskInput(task *models.AIExecutionTask) map[string]any {
	var in map[string]any
	_ = orchestrator.DecodeJSON(task.Input, &in)
	if in == nil {
		in = map[string]any{}
	}
	return in
}

func stateMatches(status, want string) bool {
	if status == want {
		return true
	}
	if want == models.StateRunning && (status == "active" || status == models.DatabaseStatusRunning) {
		return true
	}
	return false
}

// verifyServiceState compares a generic service resource with a desired
// power state.
func (s *OrchestratorUsecase) verifyServiceState(ctx context.Context, userID uuid.UUID, typ string, id uuid.UUID, want string) *orchestrator.Verification {
	v := &orchestrator.Verification{}
	if typ == orchestrator.TypeVPS {
		return s.verifyVPS(ctx, userID, id, want, want == models.StateRunning)
	}
	item, err := s.getServiceResource(ctx, userID, typ, id)
	if err != nil {
		v.Checks = append(v.Checks, check("resource_record", false, "resource not found"))
		return v.Summarize()
	}
	status := serviceStatus(item)
	v.Checks = append(v.Checks, check("resource_record", stateMatches(status, want), fmt.Sprintf("recorded status %s (want %s)", status, want)))
	v.Checks = append(v.Checks, skip("runtime_probe", "no runtime probe for "+typ+" in this phase"))
	return v.Summarize()
}

// verifyServiceAbsent checks that a deleted service resource is gone.
func (s *OrchestratorUsecase) verifyServiceAbsent(ctx context.Context, userID uuid.UUID, typ string, id uuid.UUID) *orchestrator.Verification {
	v := &orchestrator.Verification{}
	_, err := s.getServiceResource(ctx, userID, typ, id)
	v.Checks = append(v.Checks, check("resource_record_absent", err != nil, "resource record deleted"))
	return v.Summarize()
}

type reportEntry struct {
	typ, name, relation string
	id                  uuid.UUID
}

// Report builds the final report from the resources the execution touched.
func (s *OrchestratorUsecase) Report(ctx context.Context, exec *models.AIExecution, tasks []models.AIExecutionTask, status string) *orchestrator.Report {
	r := &orchestrator.Report{Resources: []orchestrator.ReportResource{}, Warnings: []string{}, NextActions: []string{}}
	rels, _ := s.repos.Orch.ListRelationships(ctx, exec.ID)
	order := []uuid.UUID{}
	entries := map[uuid.UUID]*reportEntry{}
	for _, rel := range rels {
		e, ok := entries[rel.ResourceID]
		if !ok {
			e = &reportEntry{typ: rel.ResourceType, id: rel.ResourceID}
			entries[rel.ResourceID] = e
			order = append(order, rel.ResourceID)
		}
		if rel.ResourceName != "" {
			e.name = rel.ResourceName
		}
		switch {
		case rel.Relation == models.RelationDeleted:
			e.relation = models.RelationDeleted
		case e.relation == "":
			e.relation = rel.Relation
		}
	}
	created, deleted := 0, 0
	for _, id := range order {
		e := entries[id]
		res := s.reportResource(ctx, exec.UserID, e)
		if res == nil {
			continue
		}
		if e.relation == models.RelationCreated && res.Status != "deleted" {
			created++
		}
		if res.Status == "deleted" {
			deleted++
		}
		r.Resources = append(r.Resources, *res)
		if ssh := res.Access["ssh"]; ssh != "" && status == models.ExecCompleted {
			r.NextActions = append(r.NextActions, "Connect to "+res.Name+": "+ssh)
		}
	}

	counts := map[string]int{}
	for _, t := range tasks {
		counts[t.Status]++
		switch t.Status {
		case models.TaskFailed:
			msg := ""
			if t.Error != nil {
				var te orchestrator.TaskError
				if json.Unmarshal([]byte(*t.Error), &te) == nil {
					msg = te.Message
				}
			}
			r.Warnings = append(r.Warnings, fmt.Sprintf("%s failed: %s", t.Title, msg))
		case models.TaskSkipped:
			r.Warnings = append(r.Warnings, t.Title+" was skipped because a task it depends on did not succeed")
		}
		if t.Verification != nil {
			var v orchestrator.Verification
			if json.Unmarshal([]byte(*t.Verification), &v) == nil {
				switch v.Status {
				case orchestrator.Unverified:
					r.Warnings = append(r.Warnings, t.Title+": result could not be verified ("+firstSkip(v)+")")
				case orchestrator.VerifyFail:
					r.Warnings = append(r.Warnings, t.Title+": verification failed ("+firstFail(v)+")")
				}
			}
		}
	}
	total := len(tasks)
	switch status {
	case models.ExecCompleted:
		r.Summary = fmt.Sprintf("Completed: all %d task(s) succeeded.", total)
	case models.ExecPartiallyCompleted:
		r.Summary = fmt.Sprintf("Partially completed: %d of %d task(s) succeeded, %d failed, %d skipped.", counts[models.TaskSucceeded], total, counts[models.TaskFailed], counts[models.TaskSkipped])
		r.NextActions = append(r.NextActions, "Retry the failed tasks, keep what was created, or create a plan that deletes it.")
	case models.ExecFailed:
		r.Summary = fmt.Sprintf("Failed: %d of %d task(s) failed; nothing was changed.", counts[models.TaskFailed], total)
		r.NextActions = append(r.NextActions, "Retry the failed tasks or revise the plan.")
	case models.ExecCancelled:
		r.Summary = fmt.Sprintf("Cancelled: %d of %d task(s) had finished.", counts[models.TaskSucceeded], total)
	}
	if created > 0 {
		r.Summary += fmt.Sprintf(" Created %d resource(s).", created)
	}
	if deleted > 0 {
		r.Summary += fmt.Sprintf(" Deleted %d resource(s).", deleted)
	}
	return r
}

func firstSkip(v orchestrator.Verification) string {
	for _, c := range v.Checks {
		if c.Status == "skip" {
			return c.Detail
		}
	}
	return "not observable"
}

func firstFail(v orchestrator.Verification) string {
	for _, c := range v.Checks {
		if c.Status == "fail" {
			return c.Name + ": " + c.Detail
		}
	}
	return "check failed"
}

// reportResource describes one resource for the report (no secrets).
func (s *OrchestratorUsecase) reportResource(ctx context.Context, userID uuid.UUID, e *reportEntry) *orchestrator.ReportResource {
	res := &orchestrator.ReportResource{Type: e.typ, ID: e.id.String(), Name: e.name, Status: "deleted", Access: map[string]string{}}
	if e.relation == models.RelationDeleted {
		return res
	}
	switch e.typ {
	case orchestrator.TypeVPS:
		inst, err := s.repos.Instances.FindByID(ctx, e.id, &userID)
		if err != nil {
			return res
		}
		res.Name, res.Status = inst.Name, inst.Status
		if inst.PrivateIP != nil && *inst.PrivateIP != "" {
			res.Access["ssh"] = fmt.Sprintf("ssh %s@%s", inst.SSHUsername, *inst.PrivateIP)
		}
	case orchestrator.TypeBucket:
		b, err := s.repos.Storage.FindBucket(ctx, e.id, userID)
		if err != nil {
			return res
		}
		res.Name, res.Status = b.Name, "active"
		res.Access["s3"] = "https://s3.nevatal.id/" + b.Name
	case orchestrator.TypeApp:
		a, err := s.svc.App.Get(ctx, e.id, userID, false)
		if err != nil {
			return res
		}
		res.Name, res.Status = a.Name, a.Status
		res.Access["url"] = "https://" + a.AssignedSubdomain
	case orchestrator.TypeN8n:
		a, err := s.svc.Automation.Get(ctx, e.id, userID, false)
		if err != nil {
			return res
		}
		res.Name, res.Status = a.Name, a.Status
		res.Access["url"] = a.DashboardURL
	case orchestrator.TypeOpenClaw:
		a, err := s.svc.OpenClaw.Get(ctx, e.id, userID, false)
		if err != nil {
			return res
		}
		res.Name, res.Status = a.Name, a.Status
		res.Access["gateway"] = a.GatewayEndpoint
	case orchestrator.TypeGameServer:
		g, err := s.svc.GameServer.Get(ctx, e.id, userID, false)
		if err != nil {
			return res
		}
		res.Name, res.Status = g.Name, g.Status
		res.Access["address"] = g.PublicAddress
	case orchestrator.TypeAndroid:
		a, err := s.svc.Android.Get(ctx, e.id, userID, false)
		if err != nil {
			return res
		}
		res.Name, res.Status = a.Name, a.Status
	case orchestrator.TypeDatabase:
		db, err := s.svc.Database.Get(ctx, e.id, userID)
		if err != nil {
			return res
		}
		res.Name, res.Status = db.Name, db.Status
		res.Access["endpoint"] = fmt.Sprintf("%s:%d", db.Host, db.Port)
		res.Access["database"] = db.DatabaseName
	case orchestrator.TypeDomain:
		domains, err := s.svc.Domain.List(ctx, userID)
		if err != nil {
			return res
		}
		for _, d := range domains {
			if d.Domain.ID == e.id {
				res.Name, res.Status = d.Domain.Hostname, string(d.LiveStatus)
				res.Access["url"] = "https://" + d.Domain.Hostname
				break
			}
		}
	}
	return res
}

var planStatusFor = map[string]string{
	models.ExecQueued:             models.PlanStatusApproved,
	models.ExecRunning:            models.PlanStatusExecuting,
	models.ExecVerifying:          models.PlanStatusVerifying,
	models.ExecCompleted:          models.PlanStatusCompleted,
	models.ExecFailed:             models.PlanStatusFailed,
	models.ExecPartiallyCompleted: models.PlanStatusPartiallyCompleted,
	models.ExecCancelled:          models.PlanStatusCancelled,
}

var conversationStateFor = map[string]models.WorkflowState{
	models.ExecRunning:            models.StateAIProvisioning,
	models.ExecVerifying:          models.StateValidating,
	models.ExecCompleted:          models.StateCompleted,
	models.ExecFailed:             models.StateAIFailed,
	models.ExecPartiallyCompleted: models.StateRecovery,
	models.ExecCancelled:          models.StateCancelled,
}

// StatusChanged keeps the plan, the conversation and the legacy resource
// rows in step with the execution, and posts the result to the chat when the
// execution finishes. It runs inside the engine's transaction.
func (s *OrchestratorUsecase) StatusChanged(ctx context.Context, exec *models.AIExecution, tasks []models.AIExecutionTask, status string) {
	if ps, ok := planStatusFor[status]; ok {
		_ = s.repos.Orch.UpdatePlan(ctx, exec.PlanID, map[string]any{"status": ps})
	}
	if cs, ok := conversationStateFor[status]; ok {
		_ = s.repos.Agent.SetConversationState(ctx, exec.ConversationID, cs)
		if state, err := s.repos.Agent.FindState(ctx, exec.ConversationID); err == nil {
			state.State = cs
			_ = s.repos.Agent.SaveState(ctx, state)
		}
	}
	if !isTerminalExecStatus(status) {
		return
	}
	// Legacy resource rows.
	for _, t := range tasks {
		if !orchestrator.IsWrite(t.Risk) {
			continue
		}
		var in, out map[string]any
		_ = orchestrator.DecodeJSON(t.Input, &in)
		if t.Output != nil {
			_ = orchestrator.DecodeJSON(*t.Output, &out)
		}
		name := orchestrator.String(in, "name")
		if name == "" {
			continue
		}
		legacy, provisioned, errMsg := "failed", "", ""
		switch t.Status {
		case models.TaskSucceeded:
			legacy = "completed"
			for _, k := range []string{"instance_id", "bucket_id", "application_id", "automation_id", "agent_id", "game_server_id", "android_instance_id"} {
				if v := orchestrator.String(out, k); v != "" {
					provisioned = v
					break
				}
			}
		case models.TaskCancelled, models.TaskSkipped:
			legacy = "cancelled"
		}
		if t.Error != nil {
			var te orchestrator.TaskError
			if json.Unmarshal([]byte(*t.Error), &te) == nil && t.Status == models.TaskFailed {
				errMsg = te.Message
			}
		}
		_ = s.repos.Orch.SetPlanResourceResult(ctx, exec.PlanID, name, legacy, provisioned, errMsg)
	}
	s.postResultMessage(ctx, exec, tasks, status)
	if isTerminalExecStatus(status) {
		s.afterExecutionTerminal(exec, status)
	}
}

// afterExecutionTerminal writes the in-app notification and evaluates the
// user's automation policies after the transaction that finished the
// execution committed. It never runs inside the engine transaction, and a
// failure here can never change the execution outcome.
func (s *OrchestratorUsecase) afterExecutionTerminal(exec *models.AIExecution, status string) {
	userID, execID, planID, convID := exec.UserID, exec.ID, exec.PlanID, exec.ConversationID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		summary := "Execution " + status
		if fresh, err := s.repos.Orch.FindExecution(ctx, execID, nil); err == nil && fresh.Report != nil {
			var report orchestrator.Report
			if json.Unmarshal([]byte(*fresh.Report), &report) == nil && report.Summary != "" {
				summary = report.Summary
			}
		}
		severity := models.NotificationSeverityInfo
		title := "Execution completed"
		switch status {
		case models.ExecFailed:
			severity, title = models.NotificationSeverityCritical, "Execution failed"
		case models.ExecPartiallyCompleted:
			severity, title = models.NotificationSeverityWarning, "Execution partially completed"
		case models.ExecCancelled:
			severity, title = models.NotificationSeverityWarning, "Execution cancelled"
		}
		if s.svc.Notifications != nil {
			s.svc.Notifications.Notify(ctx, NotificationInput{
				UserID: userID, Type: models.NotificationTypeExecution, Severity: severity,
				Title: title, Message: summary, ResourceType: "plan", ResourceID: &planID,
				ActionURL: PlanURL(convID, planID), DedupeKey: "execution:" + execID.String() + ":" + status,
			})
		}
		if s.svc.Policies != nil {
			trigger := models.PolicyTriggerExecutionCompleted
			if status == models.ExecFailed || status == models.ExecPartiallyCompleted {
				trigger = models.PolicyTriggerExecutionFailed
			}
			s.svc.Policies.Evaluate(ctx, userID, PolicyEvent{
				Trigger: trigger, Severity: severity, Action: status, Message: summary,
				ExecutionID: execID.String(), PlanID: planID.String(),
			})
		}
	}()
}

// postResultMessage writes the assistant message that reports a finished
// execution in the conversation (as the synchronous flow used to).
func (s *OrchestratorUsecase) postResultMessage(ctx context.Context, exec *models.AIExecution, tasks []models.AIExecutionTask, status string) {
	var report orchestrator.Report
	if fresh, err := s.repos.Orch.FindExecution(ctx, exec.ID, nil); err == nil && fresh.Report != nil {
		_ = json.Unmarshal([]byte(*fresh.Report), &report)
	}
	steps := make([]models.UIProgressStep, 0, len(tasks))
	for _, t := range tasks {
		st := "pending"
		switch t.Status {
		case models.TaskSucceeded:
			st = "completed"
		case models.TaskFailed:
			st = "failed"
		case models.TaskSkipped, models.TaskCancelled:
			st = t.Status
		}
		step := models.UIProgressStep{StepID: t.Key, Title: t.Title, Status: st}
		if t.FinishedAt != nil {
			step.CompletedAt = t.FinishedAt.UTC().Format("15:04:05")
		}
		steps = append(steps, step)
	}
	data := map[string]any{"execution_id": exec.ID.String(), "status": status, "resources": report.Resources}
	payload := models.UIComponentsPayload{ProgressSteps: steps}
	var b strings.Builder
	switch status {
	case models.ExecCompleted:
		b.WriteString("🎉 **Execution completed**\n\n")
		payload.SuccessData = data
	case models.ExecPartiallyCompleted:
		b.WriteString("⚠️ **Execution partially completed**\n\n")
		payload.ErrorData = data
	case models.ExecFailed:
		b.WriteString("❌ **Execution failed**\n\n")
		payload.ErrorData = data
	default:
		b.WriteString("🛑 **Execution cancelled**\n\n")
		payload.ErrorData = data
	}
	b.WriteString(report.Summary)
	for _, r := range report.Resources {
		fmt.Fprintf(&b, "\n- **%s** `%s`: %s", r.Type, r.Name, r.Status)
		if ssh := r.Access["ssh"]; ssh != "" {
			fmt.Fprintf(&b, " — `%s`", ssh)
		}
	}
	for _, w := range report.Warnings {
		b.WriteString("\n- ⚠️ " + w)
	}
	ui, _ := json.Marshal(payload)
	_ = s.repos.Agent.CreateMessage(ctx, &models.AIMessage{
		ConversationID: exec.ConversationID, UserID: exec.UserID, Role: "assistant",
		Content: b.String(), UIComponents: string(ui), CreatedAt: time.Now(),
	})
}
