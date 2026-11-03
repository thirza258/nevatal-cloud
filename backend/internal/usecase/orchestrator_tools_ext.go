package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
)

// Tool implementations for Phases 2-4. They all go through the same policy
// checks, ownership lookups and verification hooks as the Phase 1 tools.

func (s *OrchestratorUsecase) bindToolsExt() {
	bind := func(id string, fn func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error)) {
		s.reg.Bind(id, orchestrator.RunnerFunc(fn))
	}
	bind(orchestrator.ToolServiceList, s.toolServiceList)
	bind(orchestrator.ToolServiceGet, s.toolServiceGet)
	bind(orchestrator.ToolServiceLogs, s.toolServiceLogs)
	bind(orchestrator.ToolServicePower, s.toolServicePower)
	bind(orchestrator.ToolServiceDelete, s.toolServiceDelete)
	bind(orchestrator.ToolServiceBackupCreate, s.toolServiceBackupCreate)
	bind(orchestrator.ToolServiceBackupRestore, s.toolServiceBackupRestore)
	bind(orchestrator.ToolDatabaseCreate, s.toolDatabaseCreate)
	bind(orchestrator.ToolAppDeploy, s.toolAppDeploy)
	bind(orchestrator.ToolBucketUpdate, s.toolBucketUpdate)
	bind(orchestrator.ToolBucketObjectList, s.toolBucketObjectList)
	bind(orchestrator.ToolBucketObjectDelete, s.toolBucketObjectDelete)
	bind(orchestrator.ToolDomainList, s.toolDomainList)
	bind(orchestrator.ToolDomainClaim, s.toolDomainClaim)
	bind(orchestrator.ToolDomainRelease, s.toolDomainRelease)
	bind(orchestrator.ToolTemplateList, s.toolTemplateList)
	bind(orchestrator.ToolTemplateGet, s.toolTemplateGet)
	bind(orchestrator.ToolTemplateSave, s.toolTemplateSave)
	bind(orchestrator.ToolTemplateDeploy, s.toolTemplateDeploy)
	bind(orchestrator.ToolDiagnosticsResource, s.toolDiagnosticsResource)
	bind(orchestrator.ToolAdvisorOptimize, s.toolAdvisorOptimize)
	bind(orchestrator.ToolTopology, s.toolTopology)
}

func serviceStatus(v any) string {
	switch x := v.(type) {
	case *models.Instance:
		return x.Status
	case *models.Application:
		return x.Status
	case *models.AutomationInstance:
		return x.Status
	case *models.OpenClawInstance:
		return x.Status
	case *models.GameServer:
		return x.Status
	case *models.AndroidInstance:
		return x.Status
	case *models.ManagedDatabase:
		return x.Status
	case *models.StorageBucket:
		return "active"
	case *models.Domain:
		if x.AdminDisabled || !x.Enabled {
			return "disabled"
		}
		return "live"
	}
	return "unknown"
}

func serviceName(v any) string {
	switch x := v.(type) {
	case *models.Instance:
		return x.Name
	case *models.Application:
		return x.Name
	case *models.AutomationInstance:
		return x.Name
	case *models.OpenClawInstance:
		return x.Name
	case *models.GameServer:
		return x.Name
	case *models.AndroidInstance:
		return x.Name
	case *models.ManagedDatabase:
		return x.Name
	case *models.StorageBucket:
		return x.Name
	case *models.Domain:
		return x.Hostname
	}
	return ""
}

// getServiceResource loads one resource of any service type.
func (s *OrchestratorUsecase) getServiceResource(ctx context.Context, userID uuid.UUID, resourceType string, id uuid.UUID) (any, error) {
	switch resourceType {
	case orchestrator.TypeVPS:
		return s.svc.Instance.Get(ctx, id, userID, false)
	case orchestrator.TypeApp:
		return s.svc.App.Get(ctx, id, userID, false)
	case orchestrator.TypeN8n:
		return s.svc.Automation.Get(ctx, id, userID, false)
	case orchestrator.TypeOpenClaw:
		return s.svc.OpenClaw.Get(ctx, id, userID, false)
	case orchestrator.TypeGameServer:
		return s.svc.GameServer.Get(ctx, id, userID, false)
	case orchestrator.TypeAndroid:
		return s.svc.Android.Get(ctx, id, userID, false)
	case orchestrator.TypeDatabase:
		if s.svc.Database == nil {
			return nil, errors.New("managed databases are not enabled")
		}
		return s.svc.Database.Get(ctx, id, userID)
	case orchestrator.TypeBucket:
		return s.svc.Storage.GetOwnedBucket(ctx, id, userID)
	case orchestrator.TypeDomain:
		if s.svc.Domain == nil || !s.svc.Domain.Enabled() {
			return nil, errors.New("domains are not enabled")
		}
		domains, err := s.svc.Domain.List(ctx, userID)
		if err != nil {
			return nil, err
		}
		for i := range domains {
			if domains[i].Domain.ID == id {
				return &domains[i].Domain, nil
			}
		}
		return nil, errors.New("domain not found")
	}
	return nil, fmt.Errorf("unsupported resource type %q", resourceType)
}

func (s *OrchestratorUsecase) toolServiceList(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	typ := orchestrator.String(tc.Input, "resource_type")
	status := orchestrator.String(tc.Input, "status")
	var items any
	switch typ {
	case orchestrator.TypeVPS:
		list, err := s.svc.Instance.List(ctx, tc.UserID, false, "")
		if err != nil {
			return nil, classify(err)
		}
		items = list
	case orchestrator.TypeApp:
		list, err := s.svc.App.List(ctx, tc.UserID, false)
		if err != nil {
			return nil, classify(err)
		}
		items = list
	case orchestrator.TypeN8n:
		list, err := s.svc.Automation.List(ctx, tc.UserID, false)
		if err != nil {
			return nil, classify(err)
		}
		items = list
	case orchestrator.TypeOpenClaw:
		list, err := s.svc.OpenClaw.List(ctx, tc.UserID, false)
		if err != nil {
			return nil, classify(err)
		}
		items = list
	case orchestrator.TypeGameServer:
		list, err := s.svc.GameServer.List(ctx, tc.UserID, false, "")
		if err != nil {
			return nil, classify(err)
		}
		items = list
	case orchestrator.TypeAndroid:
		list, err := s.svc.Android.List(ctx, tc.UserID, false, "")
		if err != nil {
			return nil, classify(err)
		}
		items = list
	case orchestrator.TypeDatabase:
		if s.svc.Database == nil {
			return nil, orchestrator.Permanent("FEATURE_DISABLED", "managed databases are not enabled")
		}
		list, err := s.svc.Database.List(ctx, tc.UserID)
		if err != nil {
			return nil, classify(err)
		}
		items = list
	case orchestrator.TypeBucket:
		list, err := s.svc.Storage.ListBuckets(ctx, tc.UserID)
		if err != nil {
			return nil, classify(err)
		}
		items = list
	case orchestrator.TypeDomain:
		if s.svc.Domain == nil || !s.svc.Domain.Enabled() {
			return nil, orchestrator.Permanent("DOMAINS_DISABLED", "domains are not enabled")
		}
		list, err := s.svc.Domain.List(ctx, tc.UserID)
		if err != nil {
			return nil, classify(err)
		}
		items = list
	default:
		return nil, orchestrator.Permanent("INPUT_INVALID", "unsupported resource type")
	}
	return map[string]any{"resource_type": typ, "status_filter": status, "items": filterByStatus(items, status)}, nil
}

// filterByStatus is a best-effort filter over model slices.
func filterByStatus(items any, status string) any {
	if status == "" {
		return items
	}
	switch list := items.(type) {
	case []models.Instance:
		out := make([]models.Instance, 0, len(list))
		for _, x := range list {
			if x.Status == status {
				out = append(out, x)
			}
		}
		return out
	case []models.Application:
		out := make([]models.Application, 0, len(list))
		for _, x := range list {
			if x.Status == status {
				out = append(out, x)
			}
		}
		return out
	case []models.AutomationInstance:
		out := make([]models.AutomationInstance, 0, len(list))
		for _, x := range list {
			if x.Status == status {
				out = append(out, x)
			}
		}
		return out
	case []models.OpenClawInstance:
		out := make([]models.OpenClawInstance, 0, len(list))
		for _, x := range list {
			if x.Status == status {
				out = append(out, x)
			}
		}
		return out
	case []models.GameServer:
		out := make([]models.GameServer, 0, len(list))
		for _, x := range list {
			if x.Status == status {
				out = append(out, x)
			}
		}
		return out
	case []models.AndroidInstance:
		out := make([]models.AndroidInstance, 0, len(list))
		for _, x := range list {
			if x.Status == status {
				out = append(out, x)
			}
		}
		return out
	case []models.ManagedDatabase:
		out := make([]models.ManagedDatabase, 0, len(list))
		for _, x := range list {
			if x.Status == status {
				out = append(out, x)
			}
		}
		return out
	}
	return items
}

func (s *OrchestratorUsecase) toolServiceGet(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	typ := orchestrator.String(tc.Input, "resource_type")
	id, ok := orchestrator.UUID(tc.Input, "resource_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "resource_id must be a UUID")
	}
	item, err := s.getServiceResource(context.Background(), tc.UserID, typ, id)
	if err != nil {
		return nil, classify(err)
	}
	return map[string]any{"resource_type": typ, "resource": item, "status": serviceStatus(item)}, nil
}

func (s *OrchestratorUsecase) toolServiceLogs(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	typ := orchestrator.String(tc.Input, "resource_type")
	id, ok := orchestrator.UUID(tc.Input, "resource_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "resource_id must be a UUID")
	}
	lines := orchestrator.Int(tc.Input, "lines", 200)
	bg := context.Background()
	var logs []string
	var err error
	switch typ {
	case orchestrator.TypeApp:
		var app *models.Application
		app, err = s.svc.App.Get(bg, id, tc.UserID, false)
		if err == nil {
			if len(app.Deployments) > 0 {
				logs = splitLogs(app.Deployments[0].BuildLogs)
			} else {
				logs = []string{"no deployments yet"}
			}
		}
	case orchestrator.TypeN8n:
		logs, err = s.svc.Automation.GetLogs(bg, id, tc.UserID, false)
	case orchestrator.TypeOpenClaw:
		logs, err = s.svc.OpenClaw.GetLogs(bg, id, tc.UserID, false)
	case orchestrator.TypeGameServer:
		logs, err = s.svc.GameServer.GetLogs(bg, id, tc.UserID, false, lines)
	case orchestrator.TypeAndroid:
		logs, err = s.svc.Android.GetLogs(bg, id, tc.UserID, false, lines)
	case orchestrator.TypeDatabase:
		if s.svc.Database == nil {
			return nil, orchestrator.Permanent("FEATURE_DISABLED", "managed databases are not enabled")
		}
		logs, err = s.svc.Database.Logs(bg, id, tc.UserID, lines)
	default:
		return nil, orchestrator.Permanent("INPUT_INVALID", "logs are not available for this resource type")
	}
	if err != nil {
		return nil, classify(err)
	}
	if logs == nil {
		logs = []string{}
	}
	return map[string]any{"resource_type": typ, "resource_id": id.String(), "logs": logs}, nil
}

func splitLogs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	if len(lines) > 500 {
		lines = lines[len(lines)-500:]
	}
	return lines
}

func (s *OrchestratorUsecase) toolServicePower(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	typ := orchestrator.String(tc.Input, "resource_type")
	id, ok := orchestrator.UUID(tc.Input, "resource_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "resource_id must be a UUID")
	}
	action := orchestrator.String(tc.Input, "action")
	bg := context.Background()
	item, err := s.getServiceResource(bg, tc.UserID, typ, id)
	if err != nil {
		return nil, classify(err)
	}
	name := serviceName(item)
	switch typ {
	case orchestrator.TypeApp:
		err = s.svc.App.UpdatePower(bg, id, tc.UserID, false, action, tc.ClientIP)
	case orchestrator.TypeN8n:
		err = s.svc.Automation.UpdatePower(bg, id, tc.UserID, false, action, tc.ClientIP)
	case orchestrator.TypeOpenClaw:
		err = s.svc.OpenClaw.UpdatePower(bg, id, tc.UserID, false, action, tc.ClientIP)
	case orchestrator.TypeGameServer:
		switch action {
		case orchestrator.ActionStart:
			err = s.svc.GameServer.Start(bg, id, tc.UserID, false, tc.ClientIP)
		case orchestrator.ActionStop:
			err = s.svc.GameServer.Stop(bg, id, tc.UserID, false, true, tc.ClientIP)
		case orchestrator.ActionRestart:
			err = s.svc.GameServer.Restart(bg, id, tc.UserID, false, tc.ClientIP)
		default:
			return nil, orchestrator.Permanent("INPUT_INVALID", "unsupported action")
		}
	case orchestrator.TypeAndroid:
		switch action {
		case orchestrator.ActionStart:
			err = s.svc.Android.Start(bg, id, tc.UserID, false, tc.ClientIP)
		case orchestrator.ActionStop:
			err = s.svc.Android.Stop(bg, id, tc.UserID, false, tc.ClientIP)
		case orchestrator.ActionRestart:
			err = s.svc.Android.Restart(bg, id, tc.UserID, false, tc.ClientIP)
		default:
			return nil, orchestrator.Permanent("INPUT_INVALID", "unsupported action")
		}
	case orchestrator.TypeDatabase:
		if s.svc.Database == nil {
			return nil, orchestrator.Permanent("FEATURE_DISABLED", "managed databases are not enabled")
		}
		_, err = s.svc.Database.UpdatePower(bg, id, tc.UserID, action, tc.ClientIP)
	default:
		return nil, orchestrator.Permanent("INPUT_INVALID", "unsupported resource type for power operations")
	}
	if err != nil {
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationModified, typ, id, name)
	fresh, gerr := s.getServiceResource(bg, tc.UserID, typ, id)
	if gerr != nil {
		return map[string]any{"resource_type": typ, "resource_id": id.String(), "action": action, "status": "accepted"}, nil
	}
	return map[string]any{"resource_type": typ, "resource_id": id.String(), "action": action, "resource": fresh, "status": serviceStatus(fresh)}, nil
}

func (s *OrchestratorUsecase) toolServiceDelete(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	typ := orchestrator.String(tc.Input, "resource_type")
	id, ok := orchestrator.UUID(tc.Input, "resource_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "resource_id must be a UUID")
	}
	bg := context.Background()
	item, err := s.getServiceResource(bg, tc.UserID, typ, id)
	if err != nil {
		if isNotFoundError(err) {
			return map[string]any{"resource_type": typ, "resource_id": id.String(), "status": "deleted", "already_absent": true}, nil
		}
		return nil, classify(err)
	}
	name := serviceName(item)
	switch typ {
	case orchestrator.TypeApp:
		err = s.svc.App.Delete(bg, id, tc.UserID, false, tc.ClientIP)
	case orchestrator.TypeN8n:
		err = s.svc.Automation.Delete(bg, id, tc.UserID, false, tc.ClientIP)
	case orchestrator.TypeOpenClaw:
		err = s.svc.OpenClaw.Delete(bg, id, tc.UserID, false, tc.ClientIP)
	case orchestrator.TypeGameServer:
		err = s.svc.GameServer.Delete(bg, id, tc.UserID, false, tc.ClientIP)
	case orchestrator.TypeAndroid:
		err = s.svc.Android.Delete(bg, id, tc.UserID, false, tc.ClientIP)
	case orchestrator.TypeDatabase:
		if s.svc.Database == nil {
			return nil, orchestrator.Permanent("FEATURE_DISABLED", "managed databases are not enabled")
		}
		err = s.svc.Database.Delete(bg, id, tc.UserID, tc.ClientIP)
	default:
		return nil, orchestrator.Permanent("INPUT_INVALID", "unsupported resource type for deletion")
	}
	if err != nil {
		if isNotFoundError(err) {
			return map[string]any{"resource_type": typ, "resource_id": id.String(), "name": name, "status": "deleted", "already_absent": true}, nil
		}
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationDeleted, typ, id, name)
	return map[string]any{"resource_type": typ, "resource_id": id.String(), "name": name, "status": "deleted"}, nil
}

func isNotFoundError(err error) bool {
	var ue *Error
	if errors.As(err, &ue) {
		return ue.Kind == KindNotFound
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "not found") || strings.Contains(lower, "access denied")
}

func (s *OrchestratorUsecase) toolServiceBackupCreate(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	typ := orchestrator.String(tc.Input, "resource_type")
	id, ok := orchestrator.UUID(tc.Input, "resource_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "resource_id must be a UUID")
	}
	name := orchestrator.String(tc.Input, "name")
	bg := context.Background()
	out := map[string]any{"resource_type": typ, "resource_id": id.String()}
	switch typ {
	case orchestrator.TypeN8n:
		b, err := s.svc.Automation.CreateBackup(bg, id, tc.UserID, false, tc.ClientIP)
		if err != nil {
			return nil, classify(err)
		}
		out["backup_id"], out["name"], out["status"] = b.ID.String(), b.Name, b.Status
	case orchestrator.TypeOpenClaw:
		b, err := s.svc.OpenClaw.CreateBackup(bg, id, tc.UserID, false, tc.ClientIP)
		if err != nil {
			return nil, classify(err)
		}
		out["backup_id"], out["name"], out["status"] = b.ID.String(), b.Name, b.Status
	case orchestrator.TypeGameServer:
		b, err := s.svc.GameServer.CreateBackup(bg, id, tc.UserID, false, name, false)
		if err != nil {
			return nil, classify(err)
		}
		out["backup_id"], out["name"], out["status"] = b.ID.String(), b.BackupName, b.Status
	case orchestrator.TypeAndroid:
		b, err := s.svc.Android.CreateBackup(bg, id, tc.UserID, false, name, tc.ClientIP)
		if err != nil {
			return nil, classify(err)
		}
		out["backup_id"], out["name"], out["status"] = b.ID.String(), b.BackupName, b.Status
	case orchestrator.TypeDatabase:
		if s.svc.Database == nil {
			return nil, orchestrator.Permanent("FEATURE_DISABLED", "managed databases are not enabled")
		}
		b, err := s.svc.Database.CreateBackup(bg, id, tc.UserID, name, tc.ClientIP)
		if err != nil {
			return nil, classify(err)
		}
		out["backup_id"], out["name"], out["status"] = b.ID.String(), b.Name, b.Status
	default:
		return nil, orchestrator.Permanent("INPUT_INVALID", "this resource type does not support backups")
	}
	return out, nil
}

func (s *OrchestratorUsecase) toolServiceBackupRestore(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	typ := orchestrator.String(tc.Input, "resource_type")
	id, ok := orchestrator.UUID(tc.Input, "resource_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "resource_id must be a UUID")
	}
	backupID, ok := orchestrator.UUID(tc.Input, "backup_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "backup_id must be a UUID")
	}
	bg := context.Background()
	var err error
	switch typ {
	case orchestrator.TypeGameServer:
		err = s.svc.GameServer.RestoreBackup(bg, id, backupID, tc.UserID, false, tc.ClientIP)
	case orchestrator.TypeAndroid:
		err = s.svc.Android.RestoreBackup(bg, id, backupID, tc.UserID, false, tc.ClientIP)
	case orchestrator.TypeDatabase:
		if s.svc.Database == nil {
			return nil, orchestrator.Permanent("FEATURE_DISABLED", "managed databases are not enabled")
		}
		err = s.svc.Database.RestoreBackup(bg, id, backupID, tc.UserID, tc.ClientIP)
	default:
		return nil, orchestrator.Permanent("INPUT_INVALID", "this resource type does not support backup restore")
	}
	if err != nil {
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationModified, typ, id, orchestrator.String(tc.Input, "name"))
	return map[string]any{"resource_type": typ, "resource_id": id.String(), "backup_id": backupID.String(), "restored": true}, nil
}

func databaseOutput(db *models.ManagedDatabase) map[string]any {
	return map[string]any{
		"database_id": db.ID.String(), "name": db.Name, "engine": db.Engine, "engine_version": db.EngineVersion,
		"status": db.Status, "plan_name": db.PlanName, "vcpus": db.VCPUs, "memory_mb": db.MemoryMB,
		"storage_gb": db.StorageGB, "host": db.Host, "port": db.Port, "database_name": db.DatabaseName,
		"username": db.Username,
	}
}

func (s *OrchestratorUsecase) toolDatabaseCreate(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	bg := context.Background()
	if s.svc.Database == nil {
		return nil, orchestrator.Permanent("FEATURE_DISABLED", "managed databases are not enabled")
	}
	in := tc.Input
	name := strings.ToLower(orchestrator.String(in, "name"))
	if id, ok := s.adopted(tc, orchestrator.TypeDatabase); ok {
		if db, err := s.svc.Database.Get(bg, id, tc.UserID); err == nil {
			return databaseOutput(db), nil
		}
	}
	if list, err := s.svc.Database.List(bg, tc.UserID); err == nil {
		for i := range list {
			if strings.EqualFold(list[i].Name, name) && createdByThisExecution(tc, list[i].CreatedAt) {
				tc.RecordResource(models.RelationCreated, orchestrator.TypeDatabase, list[i].ID, list[i].Name)
				return databaseOutput(&list[i]), nil
			}
		}
	}
	autoBackup := true
	if v, ok := in["auto_backup"].(bool); ok {
		autoBackup = v
	}
	db, _, err := s.svc.Database.Create(bg, tc.UserID, CreateDatabaseInput{
		Name: name, Engine: orchestrator.String(in, "engine"), EngineVersion: orchestrator.String(in, "engine_version"),
		PlanName: orchestrator.String(in, "plan_name"), VCPUs: orchestrator.Int(in, "vcpus", 0),
		MemoryMB: orchestrator.Int(in, "memory_mb", 0), StorageGB: orchestrator.Int(in, "storage_gb", 0),
		DatabaseName: orchestrator.String(in, "database_name"), AutoBackup: &autoBackup,
	}, tc.ClientIP)
	if err != nil {
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationCreated, orchestrator.TypeDatabase, db.ID, db.Name)
	s.linkHosted(tc, orchestrator.TypeDatabase, db.ID, db.Name)
	return databaseOutput(db), nil
}

// linkHosted records that a created service runs on a VPS named by vps_key
// or vps_instance_id.
func (s *OrchestratorUsecase) linkHosted(tc *orchestrator.TaskContext, typ string, id uuid.UUID, name string) {
	var vpsID uuid.UUID
	if key := orchestrator.String(tc.Input, "vps_key"); key != "" {
		for depKey, out := range tc.DependencyOutputs {
			if strings.HasSuffix(depKey, ":"+key) {
				if got, ok := orchestrator.UUID(out, "instance_id"); ok {
					vpsID = got
					break
				}
			}
		}
	}
	if vpsID == uuid.Nil {
		if got, ok := orchestrator.UUID(tc.Input, "vps_instance_id"); ok {
			vpsID = got
		}
	}
	if vpsID != uuid.Nil {
		s.link(context.Background(), tc, models.RelationHosts, typ, id, name, orchestrator.TypeVPS, vpsID, "")
	}
}

func (s *OrchestratorUsecase) toolAppDeploy(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	id, ok := orchestrator.UUID(tc.Input, "app_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "app_id must be a UUID")
	}
	app, err := s.svc.App.Get(context.Background(), id, tc.UserID, false)
	if err != nil {
		return nil, classify(err)
	}
	dep, err := s.svc.App.TriggerDeploy(context.Background(), id, tc.UserID, false, orchestrator.String(tc.Input, "commit_message"), tc.ClientIP)
	if err != nil {
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationModified, orchestrator.TypeApp, id, app.Name)
	out := map[string]any{"application_id": id.String(), "name": app.Name, "status": dep.Status}
	if dep != nil {
		out["deployment_id"] = dep.ID.String()
	}
	return out, nil
}

func (s *OrchestratorUsecase) toolBucketUpdate(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	id, ok := orchestrator.UUID(tc.Input, "bucket_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "bucket_id must be a UUID")
	}
	bucket, err := s.svc.Storage.GetOwnedBucket(context.Background(), id, tc.UserID)
	if err != nil {
		return nil, classify(err)
	}
	in := UpdateBucketInput{Description: bucket.Description, Visibility: bucket.Visibility}
	if v, present := tc.Input["is_public"]; present {
		if b, ok := v.(bool); ok && b {
			in.Visibility = "public"
		} else {
			in.Visibility = "private"
		}
	}
	if v, present := tc.Input["description"]; present {
		if s, ok := v.(string); ok {
			in.Description = s
		}
	}
	updated, err := s.svc.Storage.UpdateBucket(context.Background(), id, tc.UserID, in)
	if err != nil {
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationModified, orchestrator.TypeBucket, id, updated.Name)
	return map[string]any{"bucket_id": updated.ID.String(), "name": updated.Name, "visibility": updated.Visibility}, nil
}

func (s *OrchestratorUsecase) toolBucketObjectList(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	id, ok := orchestrator.UUID(tc.Input, "bucket_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "bucket_id must be a UUID")
	}
	objects, err := s.svc.Storage.ListObjects(context.Background(), id, tc.UserID, orchestrator.String(tc.Input, "prefix"))
	if err != nil {
		return nil, classify(err)
	}
	limit := orchestrator.Int(tc.Input, "limit", 100)
	if limit <= 0 {
		limit = 100
	}
	items := make([]map[string]any, 0, len(objects))
	for i, obj := range objects {
		if i >= limit {
			break
		}
		items = append(items, map[string]any{
			"object_id": obj.ID.String(), "key": obj.Key, "filename": obj.Filename,
			"size_bytes": obj.SizeBytes, "is_folder": obj.IsFolder, "content_type": obj.ContentType,
		})
	}
	return map[string]any{"bucket_id": id.String(), "items": items, "count": len(objects)}, nil
}

func (s *OrchestratorUsecase) toolBucketObjectDelete(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	bucketID, ok := orchestrator.UUID(tc.Input, "bucket_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "bucket_id must be a UUID")
	}
	objectID, ok := orchestrator.UUID(tc.Input, "object_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "object_id must be a UUID")
	}
	obj, err := s.svc.Storage.GetObject(context.Background(), objectID)
	if err != nil || obj.BucketID != bucketID {
		return map[string]any{"bucket_id": bucketID.String(), "object_id": objectID.String(), "status": "deleted", "already_absent": true}, nil
	}
	name := obj.Key
	if err := s.svc.Storage.DeleteObject(context.Background(), objectID, tc.UserID); err != nil {
		if isNotFoundError(err) {
			return map[string]any{"bucket_id": bucketID.String(), "object_id": objectID.String(), "status": "deleted", "already_absent": true}, nil
		}
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationDeleted, "object", objectID, name)
	return map[string]any{"bucket_id": bucketID.String(), "object_id": objectID.String(), "name": name, "status": "deleted"}, nil
}

func (s *OrchestratorUsecase) toolDomainList(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	if s.svc.Domain == nil || !s.svc.Domain.Enabled() {
		return nil, orchestrator.Permanent("DOMAINS_DISABLED", "domains are not enabled")
	}
	list, err := s.svc.Domain.List(context.Background(), tc.UserID)
	if err != nil {
		return nil, classify(err)
	}
	return map[string]any{"domains": list}, nil
}

func (s *OrchestratorUsecase) toolDomainClaim(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	if s.svc.Domain == nil || !s.svc.Domain.Enabled() {
		return nil, orchestrator.Permanent("DOMAINS_DISABLED", "domains are not enabled")
	}
	bg := context.Background()
	sub := strings.ToLower(orchestrator.String(tc.Input, "subdomain"))
	instanceID, ok := orchestrator.UUID(tc.Input, "instance_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "instance_id must be a UUID")
	}
	targetPort := orchestrator.Int(tc.Input, "target_port", 0)
	if existing, err := s.svc.Domain.List(bg, tc.UserID); err == nil {
		for i := range existing {
			d := existing[i].Domain
			if strings.EqualFold(d.Subdomain, sub) && d.InstanceID != nil && *d.InstanceID == instanceID {
				tc.RecordResource(models.RelationCreated, orchestrator.TypeDomain, d.ID, d.Hostname)
				s.link(bg, tc, models.RelationRoutesTo, orchestrator.TypeDomain, d.ID, d.Hostname, orchestrator.TypeVPS, instanceID, "")
				return map[string]any{"domain_id": d.ID.String(), "hostname": d.Hostname, "subdomain": d.Subdomain, "already": true}, nil
			}
		}
	}
	view, err := s.svc.Domain.Create(bg, tc.UserID, false, CreateDomainInput{
		Subdomain: sub, InstanceID: &instanceID, TargetPort: &targetPort,
	}, tc.ClientIP)
	if err != nil {
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationCreated, orchestrator.TypeDomain, view.Domain.ID, view.Domain.Hostname)
	s.link(bg, tc, models.RelationRoutesTo, orchestrator.TypeDomain, view.Domain.ID, view.Domain.Hostname, orchestrator.TypeVPS, instanceID, "")
	return map[string]any{"domain_id": view.Domain.ID.String(), "hostname": view.Domain.Hostname,
		"subdomain": view.Domain.Subdomain, "target_port": view.Domain.TargetPort, "live_status": string(view.LiveStatus)}, nil
}

func (s *OrchestratorUsecase) toolDomainRelease(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	if s.svc.Domain == nil || !s.svc.Domain.Enabled() {
		return nil, orchestrator.Permanent("DOMAINS_DISABLED", "domains are not enabled")
	}
	id, ok := orchestrator.UUID(tc.Input, "domain_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "domain_id must be a UUID")
	}
	bg := context.Background()
	name := orchestrator.String(tc.Input, "name")
	if list, err := s.svc.Domain.List(bg, tc.UserID); err == nil {
		for _, d := range list {
			if d.Domain.ID == id {
				name = d.Domain.Hostname
				break
			}
		}
	}
	if err := s.svc.Domain.Release(bg, id, tc.UserID, false, tc.ClientIP); err != nil {
		if isNotFoundError(err) {
			return map[string]any{"domain_id": id.String(), "status": "released", "already_absent": true}, nil
		}
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationDeleted, orchestrator.TypeDomain, id, name)
	return map[string]any{"domain_id": id.String(), "name": name, "status": "released"}, nil
}

func (s *OrchestratorUsecase) toolTemplateList(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	list, err := s.ListTemplates(context.Background(), tc.UserID)
	if err != nil {
		return nil, classify(err)
	}
	return map[string]any{"templates": list}, nil
}

func (s *OrchestratorUsecase) toolTemplateGet(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	id, ok := orchestrator.UUID(tc.Input, "template_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "template_id must be a UUID")
	}
	tpl, err := s.GetTemplate(context.Background(), id, tc.UserID)
	if err != nil {
		return nil, classify(err)
	}
	return map[string]any{"template": tpl}, nil
}

func (s *OrchestratorUsecase) toolTemplateSave(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	var spec orchestrator.Specification
	raw, err := orchestrator.Canonical(tc.Input["specification"])
	if err != nil {
		return nil, orchestrator.Permanent("INPUT_INVALID", "specification is invalid")
	}
	if err := orchestrator.DecodeJSON(string(raw), &spec); err != nil {
		return nil, orchestrator.Permanent("INPUT_INVALID", "specification is invalid")
	}
	var params []TemplateParameter
	if rawParams, err := orchestrator.Canonical(tc.Input["parameters"]); err == nil {
		_ = orchestrator.DecodeJSON(string(rawParams), &params)
	}
	view, err := s.SaveTemplate(context.Background(), tc.UserID, SaveTemplateInput{
		Name: orchestrator.String(tc.Input, "name"), Description: orchestrator.String(tc.Input, "description"),
		Specification: spec, Parameters: params, IsPublic: orchestrator.Bool(tc.Input, "is_public"),
	}, tc.ClientIP)
	if err != nil {
		return nil, classify(err)
	}
	return map[string]any{"template": view}, nil
}

func (s *OrchestratorUsecase) toolTemplateDeploy(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	id, ok := orchestrator.UUID(tc.Input, "template_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "template_id must be a UUID")
	}
	params, _ := tc.Input["parameters"].(map[string]any)
	plan, err := s.DeployTemplate(context.Background(), tc.UserID, false, id, params)
	if err != nil {
		return nil, classify(err)
	}
	return map[string]any{"plan_id": plan.ID.String(), "version": plan.Version, "status": plan.Status}, nil
}

func (s *OrchestratorUsecase) toolDiagnosticsResource(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	typ := orchestrator.String(tc.Input, "resource_type")
	id, ok := orchestrator.UUID(tc.Input, "resource_id")
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "resource_id must be a UUID")
	}
	res, err := s.Diagnose(context.Background(), tc.UserID, typ, id)
	if err != nil {
		return nil, classify(err)
	}
	return map[string]any{"diagnostic": res}, nil
}

func (s *OrchestratorUsecase) toolAdvisorOptimize(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	report, err := s.Optimize(context.Background(), tc.UserID)
	if err != nil {
		return nil, classify(err)
	}
	return map[string]any{"advisor": report}, nil
}

func (s *OrchestratorUsecase) toolTopology(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	view, err := s.Topology(context.Background(), tc.UserID)
	if err != nil {
		return nil, classify(err)
	}
	return map[string]any{"topology": view}, nil
}

// linkAppDependencies records that a created application uses the database
// and buckets of the same plan (database_key / bucket_keys).
func (s *OrchestratorUsecase) linkAppDependencies(ctx context.Context, tc *orchestrator.TaskContext, app *models.Application) {
	if app == nil {
		return
	}
	dbKey := orchestrator.String(tc.Input, "database_key")
	bucketKeys := orchestrator.Strings(tc.Input, "bucket_keys")
	for depKey, out := range tc.DependencyOutputs {
		if dbKey != "" && strings.HasSuffix(depKey, ":"+dbKey) {
			if id, ok := orchestrator.UUID(out, "database_id"); ok {
				s.link(ctx, tc, models.RelationUses, orchestrator.TypeApp, app.ID, app.Name, orchestrator.TypeDatabase, id, orchestrator.String(out, "name"))
			}
		}
		for _, bKey := range bucketKeys {
			if strings.HasSuffix(depKey, ":"+bKey) {
				if id, ok := orchestrator.UUID(out, "bucket_id"); ok {
					s.link(ctx, tc, models.RelationUses, orchestrator.TypeApp, app.ID, app.Name, orchestrator.TypeBucket, id, orchestrator.String(out, "name"))
				}
			}
		}
	}
}
