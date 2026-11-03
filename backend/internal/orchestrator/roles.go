package orchestrator

// Logical agent roles. Phase 4 exposes these as specialized roles of one
// deterministic orchestrator: a role classifies a tool and the task using it.
// Roles never grant permissions; the policy engine, approval manager and
// execution engine remain the only authority.
const (
	RolePlanner        = "planner"
	RoleInfrastructure = "infrastructure"
	RoleApplication    = "application"
	RoleDatabase       = "database"
	RoleNetwork        = "network"
	RoleSecurity       = "security"
	RoleDiagnostic     = "diagnostic"
	RoleMonitoring     = "monitoring"
	RoleAdvisor        = "advisor"
	RoleVerification   = "verification"
)

// AgentRole describes one logical role.
type AgentRole struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tools       []string `json:"tools"`
}

// RoleForTool returns the logical role of a registry tool.
func RoleForTool(tool string) string {
	switch tool {
	case ToolDiagnosticsResource, ToolServiceList, ToolServiceGet, ToolServiceLogs:
		return RoleDiagnostic
	case ToolAdvisorOptimize, ToolTopology:
		return RoleAdvisor
	case ToolServicePower, ToolServiceDelete:
		return RoleInfrastructure
	case ToolServiceBackupCreate, ToolServiceBackupRestore:
		return RoleMonitoring
	case ToolDatabaseCreate:
		return RoleDatabase
	case ToolDomainClaim, ToolDomainRelease, ToolDomainList:
		return RoleNetwork
	case ToolTemplateList, ToolTemplateGet, ToolTemplateSave, ToolTemplateDeploy:
		return RolePlanner
	case ToolVPSVerify:
		return RoleVerification
	case ToolQuotaCheck:
		return RoleSecurity
	}
	switch {
	case hasPrefix(tool, "vps."), hasPrefix(tool, "bucket."):
		return RoleInfrastructure
	case hasPrefix(tool, "app."):
		return RoleApplication
	case hasPrefix(tool, "database."):
		return RoleDatabase
	case hasPrefix(tool, "domain."):
		return RoleNetwork
	case hasPrefix(tool, "diagnostics."), hasPrefix(tool, "advisor."), hasPrefix(tool, "infra."):
		return RoleDiagnostic
	case hasPrefix(tool, "template."):
		return RolePlanner
	}
	return RoleInfrastructure
}

// RoleForResourceType returns the role of a resource type (service tools use
// the resource type of their target rather than the generic tool name).
func RoleForResourceType(resourceType string) string {
	switch resourceType {
	case TypeApp:
		return RoleApplication
	case TypeDatabase:
		return RoleDatabase
	case TypeVPS, TypeBucket, TypeN8n, TypeOpenClaw, TypeGameServer, TypeAndroid:
		return RoleInfrastructure
	case TypeDomain:
		return RoleNetwork
	}
	return RoleInfrastructure
}

// RoleOf returns the role of a task: the target resource's role for generic
// service tools, the tool's role otherwise.
func RoleOf(tool, resourceType string) string {
	if tool == ToolServicePower || tool == ToolServiceDelete || tool == ToolServiceBackupCreate ||
		tool == ToolServiceBackupRestore || tool == ToolServiceList || tool == ToolServiceGet || tool == ToolServiceLogs {
		return RoleForResourceType(resourceType)
	}
	return RoleForTool(tool)
}

// AgentRoles returns the role catalog with the registry tools of each role.
func AgentRoles(reg *Registry) []AgentRole {
	defs := []AgentRole{
		{ID: RolePlanner, Name: "Planner", Description: "Turns requirements into versioned infrastructure plans and reusable templates."},
		{ID: RoleInfrastructure, Name: "Infrastructure", Description: "Provisions and operates VPS, storage, n8n, OpenClaw, game servers and Android instances."},
		{ID: RoleApplication, Name: "Application", Description: "Deploys and redeploys Git-backed applications."},
		{ID: RoleDatabase, Name: "Database", Description: "Provisions and operates managed databases."},
		{ID: RoleNetwork, Name: "Network", Description: "Claims and releases tenant domains."},
		{ID: RoleSecurity, Name: "Security", Description: "Enforces quota, ownership and policy checks."},
		{ID: RoleDiagnostic, Name: "Diagnostic", Description: "Collects metrics, logs and evidence and proposes remediation."},
		{ID: RoleMonitoring, Name: "Monitoring", Description: "Runs scheduled checks, backups and freshness verification."},
		{ID: RoleAdvisor, Name: "Advisor", Description: "Recommends optimizations and estimates resource consumption."},
		{ID: RoleVerification, Name: "Verification", Description: "Compares desired and actual infrastructure state."},
	}
	if reg == nil {
		return defs
	}
	byRole := map[string][]string{}
	for _, d := range reg.List() {
		role := d.Role
		if role == "" {
			role = RoleForTool(d.ID)
		}
		byRole[role] = append(byRole[role], d.ID)
	}
	for i := range defs {
		defs[i].Tools = byRole[defs[i].ID]
		if defs[i].Tools == nil {
			defs[i].Tools = []string{}
		}
	}
	return defs
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
