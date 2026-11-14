export type UserRole = 'customer' | 'admin';
export type UserStatus = 'active' | 'suspended';

export interface User {
  id: string;
  email: string;
  display_name: string;
  role: UserRole;
  status: UserStatus;
  email_verified_at?: string;
  last_login_at?: string;
  created_at?: string;
}

export interface ResourceQuota {
  id: string;
  user_id: string;
  max_vcpus: number;
  max_memory_mb: number;
  max_storage_gb: number;
  max_instances: number;
}

export interface QuotaUsage {
  max_vcpus: number;
  used_vcpus: number;
  max_memory_mb: number;
  used_memory_mb: number;
  max_storage_gb: number;
  used_storage_gb: number;
  max_instances: number;
  used_instances: number;
}

export type InstanceStatus =
  | 'queued'
  | 'provisioning'
  | 'starting'
  | 'running'
  | 'stopping'
  | 'stopped'
  | 'restarting'
  | 'deleting'
  | 'failed'
  | 'unknown';

export interface OSTemplate {
  id: string;
  name: string;
  distribution: string;
  version: string;
  description: string;
  proxmox_template_vmid: number;
  architecture: string;
  cloud_init_supported: boolean;
  default_username: string;
  enabled: boolean;
}

export interface ResourcePlan {
  id: string;
  name: string;
  description: string;
  vcpus: number;
  memory_mb: number;
  disk_gb: number;
  enabled: boolean;
}

export interface SSHKey {
  id: string;
  owner_id: string;
  name: string;
  public_key: string;
  fingerprint: string;
  created_at: string;
}

export interface Node {
  id: string;
  name: string;
  proxmox_node_name: string;
  status: string;
  total_cpu_threads: number;
  total_memory_mb: number;
  total_disk_gb: number;
  last_health_check_at?: string;
}

export interface Instance {
  id: string;
  owner_id: string;
  node_id: string;
  os_template_id: string;
  proxmox_vmid: number;
  name: string;
  hostname: string;
  status: InstanceStatus;
  vcpus: number;
  memory_mb: number;
  disk_gb: number;
  private_ip?: string;
  public_ip?: string;
  ssh_username: string;
  created_at: string;
  updated_at: string;
  deleted_at?: string;
  os_template?: OSTemplate;
  node?: Node;
  owner?: User;
}

export interface ProvisioningJob {
  id: string;
  instance_id?: string;
  operation_type: 'create' | 'delete' | 'start' | 'shutdown' | 'restart';
  status: 'queued' | 'in_progress' | 'completed' | 'failed';
  current_step: string;
  progress_percent: number;
  error_code?: string;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
}

export interface AuditLog {
  id: string;
  actor_user_id?: string;
  action: string;
  resource_type: string;
  resource_id?: string;
  result: 'success' | 'failure';
  ip_address?: string;
  metadata?: string;
  created_at: string;
  actor?: User;
}

export interface DashboardSummary {
  instances: {
    total: number;
    running: number;
    stopped: number;
    provisioning: number;
  };
  usage: QuotaUsage;
  recent_instances: Instance[];
  recent_activity: AuditLog[];
}

export interface InstanceMetrics {
  available: boolean;
  reason?: string;
  cpu_usage?: number;
  memory_used?: number;
  memory_max?: number;
  disk_used?: number;
  disk_max?: number;
  net_in?: number;
  net_out?: number;
  uptime?: number;
}

export interface InstanceSnapshot {
  id: string;
  instance_id: string;
  name: string;
  description: string;
  includes_ram: boolean;
  created_at: string;
}

export interface ConsoleTicket {
  ticket: string;
  port: number;
  upid: string;
  cert: string;
  user: string;
}

// --- Application Hosting ---
export interface ApplicationEnvVar {
  id: string;
  application_id: string;
  key: string;
  value: string;
  is_secret: boolean;
  created_at: string;
  updated_at: string;
}

export interface Deployment {
  id: string;
  application_id: string;
  source_commit: string;
  commit_message: string;
  source_branch: string;
  status: 'queued' | 'building' | 'deploying' | 'active' | 'failed' | 'superseded';
  build_logs: string;
  artifact_reference: string;
  runtime_image_digest: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
}

export interface Application {
  id: string;
  owner_id: string;
  name: string;
  slug: string;
  application_type: 'static' | 'react' | 'nextjs' | 'nodejs' | 'go' | 'python' | 'docker';
  deployment_source_type: 'git' | 'template' | 'docker';
  repository_url?: string;
  production_branch: string;
  root_directory: string;
  build_command: string;
  output_directory: string;
  start_command: string;
  application_port: number;
  health_check_path: string;
  assigned_subdomain: string;
  custom_domain?: string;
  custom_domain_verified: boolean;
  status: 'draft' | 'queued' | 'building' | 'deploying' | 'starting' | 'running' | 'stopped' | 'failed' | 'degraded';
  vcpus: number;
  memory_mb: number;
  disk_gb: number;
  created_at: string;
  updated_at: string;
  deployments?: Deployment[];
  env_vars?: ApplicationEnvVar[];
}

// --- Workflow Automation (n8n) ---
export interface AutomationBackup {
  id: string;
  automation_instance_id: string;
  name: string;
  backup_reference: string;
  size_bytes: number;
  status: string;
  created_at: string;
}

export interface AutomationInstance {
  id: string;
  owner_id: string;
  name: string;
  n8n_version: string;
  status: 'running' | 'stopped' | 'starting' | 'error';
  hostname: string;
  dashboard_url: string;
  encryption_key?: string;
  database_name: string;
  cpu_limit: number;
  memory_limit_mb: number;
  storage_limit_gb: number;
  storage_used_gb: number;
  active_workflows_count: number;
  total_executions: number;
  last_health_check_at?: string;
  created_at: string;
  updated_at: string;
  backups?: AutomationBackup[];
}

// --- AI Agents (OpenClaw) ---
export interface OpenClawIntegration {
  id: string;
  openclaw_instance_id: string;
  channel: 'telegram' | 'discord' | 'slack' | 'whatsapp';
  identifier: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface OpenClawBackup {
  id: string;
  openclaw_instance_id: string;
  name: string;
  backup_reference: string;
  size_bytes: number;
  status: string;
  created_at: string;
}

export interface OpenClawInstance {
  id: string;
  owner_id: string;
  name: string;
  version: string;
  status: 'running' | 'stopped' | 'starting' | 'degraded';
  hostname: string;
  gateway_endpoint: string;
  gateway_auth_token?: string;
  workspace_path: string;
  ai_provider: string;
  default_model: string;
  masked_api_key?: string;
  cpu_limit: number;
  memory_limit_mb: number;
  storage_limit_gb: number;
  storage_used_gb: number;
  active_skills_count: number;
  total_interactions: number;
  last_health_check_at?: string;
  created_at: string;
  updated_at: string;
  integrations?: OpenClawIntegration[];
  backups?: OpenClawBackup[];
}

// --- ISO Image Catalog ---
export interface OSImage {
  id: string;
  name: string;
  version: string;
  architecture: string;
  category: string;
  description: string;
  filename: string;
  file_size_bytes: number;
  sha256: string;
  proxmox_storage: string;
  proxmox_volume: string;
  visibility: 'published' | 'unpublished';
  status: 'available' | 'uploading' | 'verifying' | 'failed';
  created_at: string;
  updated_at: string;
}

// --- Object Storage (MinIO S3) ---
export interface StorageOverview {
  total_buckets: number;
  total_objects: number;
  total_size_bytes: number;
  total_access_keys: number;
}

export interface StorageBucket {
  id: string;
  owner_id: string;
  name: string;
  description: string;
  visibility: 'private' | 'public';
  versioning_enabled: boolean;
  region: string;
  total_objects: number;
  total_size_bytes: number;
  created_at: string;
  updated_at: string;
}

export interface StorageObject {
  id: string;
  bucket_id: string;
  key: string;
  filename: string;
  folder_prefix: string;
  content_type: string;
  size_bytes: number;
  etag: string;
  is_folder: boolean;
  public_url?: string;
  created_at: string;
  updated_at: string;
}

export interface StorageAccessKey {
  id: string;
  owner_id: string;
  access_key_id: string;
  secret_access_key?: string;
  description: string;
  permissions: 'read_only' | 'read_write' | 'admin';
  bucket_scope: string;
  status: 'active' | 'revoked';
  last_used_at?: string;
  created_at: string;
}

export interface StorageSharedLink {
  id: string;
  owner_id: string;
  object_id: string;
  share_token: string;
  share_url: string;
  access_type: 'view' | 'download';
  has_password: boolean;
  expires_at?: string;
  downloads: number;
  max_downloads?: number;
  created_at: string;
  object?: StorageObject;
}

// AI Assistant & OpenRouter Types
export interface OpenRouterModelDefinition {
  id: string;
  name: string;
  provider: string;
  description: string;
  context_len: number;
}

export interface CreateInstanceActionParams {
  name: string;
  hostname: string;
  template_id: string;
  template_name: string;
  vcpus: number;
  memory_mb: number;
  disk_gb: number;
  ssh_key_id?: string;
  ssh_key_name?: string;
  reason?: string;
}

export interface AIAssistantActionProposal {
  action: 'create_instance' | 'start_instance' | 'shutdown_instance' | 'restart_instance';
  title: string;
  description: string;
  params: CreateInstanceActionParams | any;
  executed: boolean;
}

export interface AIAssistantMessage {
  id: string;
  user_id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  model: string;
  action_type?: string;
  action_payload?: string;
  action_executed: boolean;
  created_at: string;
  updated_at?: string;
}

/**
 * POST /ai-assistant/execute: the action card became an orchestrator plan. Nothing is created until
 * the user approves the plan in the AI workspace (`url`).
 */
export interface AIAssistantExecuteResult {
  conversation_id: string;
  message: string;
  name: string;
  plan_id: string;
  risk_level: string;
  /** Plan status: `awaiting_approval`, or `draft` when the policy engine rejected the proposal. */
  status: 'awaiting_approval' | 'draft' | string;
  /** Workspace deep link: /dashboard/ai/conversations/<conversationId>?plan=<planId>. */
  url: string;
  version: number;
}

export interface AIAssistantChatResponse {
  message: AIAssistantMessage;
  action_proposal?: AIAssistantActionProposal;
  used_fallback: boolean;
  error_notice?: string;
}

export interface AIPlatformContext {
  user: User;
  quota_usage: QuotaUsage;
  instances: Instance[];
  templates: OSTemplate[];
  plans: ResourcePlan[];
  ssh_keys: SSHKey[];
  has_server_api_key: boolean;
  default_model: string;
  available_models: OpenRouterModelDefinition[];
}

// PRD Section 33-48: Nevatal AI Cloud Agent & MCP Integration Types

export type WorkflowState =
  | 'UnderstandingRequest'
  | 'ClarifyingRequirements'
  | 'Planning'
  | 'AwaitingConfirmation'
  | 'Provisioning'
  | 'Validating'
  | 'Recovery'
  | 'Completed'
  | 'Failed'
  | 'Cancelled';

export interface AIConversation {
  id: string;
  user_id: string;
  title: string;
  state: WorkflowState;
  current_intent?: string;
  summary?: string;
  created_at: string;
  updated_at: string;
  messages?: AIMessage[];
  plans?: AIResourcePlan[];
  /** Preloaded by GET /ai/conversations/:id. */
  state_obj?: { active_plan_id?: string | null } | null;
}

export interface AIMessage {
  id: string;
  conversation_id: string;
  user_id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  ui_components?: string;
  raw_input?: string;
  created_at: string;
}

export interface StructuredQuestionOption {
  id: string;
  label: string;
  description?: string;
  value: string;
  is_default?: boolean;
}

export interface StructuredQuestion {
  id: string;
  key: string;
  title: string;
  description?: string;
  type: 'single_choice' | 'multi_choice' | 'text' | 'slider';
  options?: StructuredQuestionOption[];
  required: boolean;
  default_value?: string;
  answered?: boolean;
  answer?: string;
}

export interface UIProgressStep {
  step_id: string;
  title: string;
  status: 'pending' | 'in_progress' | 'completed' | 'failed';
  message?: string;
  completed_at?: string;
}

export interface StructuredResourceView {
  type: string;
  name: string;
  summary: string;
  details: Record<string, any>;
  order: number;
  status: string;
}

export interface StructuredPlanView {
  plan_id: string;
  version: number;
  title: string;
  intent: string;
  summary: string;
  resources: StructuredResourceView[];
  estimated_allocations: Record<string, any>;
  estimated_pricing?: string;
  reversible: boolean;
  requires_confirmation: boolean;
  status: string;
}

export interface UIComponentsPayload {
  questions?: StructuredQuestion[];
  plan_proposal?: StructuredPlanView;
  progress_steps?: UIProgressStep[];
  success_data?: Record<string, any>;
  error_data?: Record<string, any>;
}

export interface AIResourcePlan {
  id: string;
  conversation_id: string;
  user_id: string;
  version: number;
  intent: string;
  status: string;
  title: string;
  description: string;
  plan_json: string;
  estimated_costs?: string;
  validation_errors?: string;
  requires_confirmation: boolean;
  created_at: string;
  updated_at: string;
  resources?: any[];
}

export interface MCPServer {
  id: string;
  name: string;
  display_name: string;
  description: string;
  transport: string;
  endpoint_url?: string;
  is_first_party: boolean;
  enabled: boolean;
  auth_type: string;
  icon?: string;
  created_at: string;
  tools?: MCPTool[];
}

export interface MCPTool {
  id: string;
  server_id: string;
  name: string;
  description: string;
  risk_level: 'read' | 'write' | 'destructive';
  input_schema: string;
  enabled: boolean;
}

export interface MCPConnection {
  id: string;
  user_id: string;
  server_id: string;
  status: 'connected' | 'disconnected' | 'error';
  config_json?: string;
  last_health_check?: string;
  created_at: string;
  updated_at: string;
  server?: MCPServer;
  permissions?: MCPPermission[];
}

export interface MCPPermission {
  id: string;
  connection_id: string;
  tool_id: string;
  granted: boolean;
  tool?: MCPTool;
}

export interface MCPAuditLog {
  id: string;
  user_id: string;
  server_id: string;
  tool_name: string;
  risk_level: string;
  confirmed: boolean;
  input_summary: string;
  result_status: string;
  ip_address?: string;
  created_at: string;
  server?: MCPServer;
}

export interface UnifiedResourceItem {
  id: string;
  type: 'vps' | 'app' | 'automation' | 'ai_agent' | 'bucket' | 'game_server' | 'android_instance' | 'database';
  name: string;
  status: string;
  endpoint?: string;
  specs?: string;
  created_at: string;
  details?: Record<string, any>;
}

// Game Server Hosting Types (PRD Addendum 49-65)

export type GameType = 'minecraft_java' | 'minecraft_bedrock' | 'terraria';
export type GameServerStatus = 'provisioning' | 'running' | 'stopped' | 'restarting' | 'error' | 'reinstalling';

export interface GameServerPlan {
  id: string;
  name: string;
  vcpus: number;
  memory_mb: number;
  storage_gb: number;
  max_players: number;
  suggested_use: string;
}

export interface GameServerVersion {
  id: string;
  template_id: string;
  version_name: string;
  is_latest: boolean;
  release_type: string;
  download_url?: string;
}

export interface GameServerTemplate {
  id: string;
  name: string;
  game_type: GameType;
  server_software: string;
  default_version: string;
  default_port: number;
  protocol: string;
  icon: string;
  description: string;
  versions?: GameServerVersion[];
}

export interface GameServerPlayer {
  id: string;
  server_id: string;
  player_uuid?: string;
  player_name: string;
  is_online: boolean;
  is_whitelisted: boolean;
  is_operator: boolean;
  is_banned: boolean;
  ban_reason?: string;
  first_joined_at?: string;
  last_seen_at?: string;
}

export interface GameServerPlugin {
  id: string;
  server_id: string;
  name: string;
  filename: string;
  file_size_bytes: number;
  version: string;
  enabled: boolean;
  is_official: boolean;
  author?: string;
  description?: string;
  created_at: string;
}

export interface GameServerMod {
  id: string;
  server_id: string;
  name: string;
  filename: string;
  file_size_bytes: number;
  version: string;
  enabled: boolean;
  mod_loader: string;
  author?: string;
  description?: string;
  created_at: string;
}

export interface GameServerWorld {
  id: string;
  server_id: string;
  world_name: string;
  size_bytes: number;
  world_seed?: string;
  difficulty?: string;
  gamemode?: string;
  is_active: boolean;
  created_at: string;
}

export interface GameServerBackup {
  id: string;
  server_id: string;
  user_id: string;
  backup_name: string;
  file_size_bytes: number;
  storage_path: string;
  world_name: string;
  is_automated: boolean;
  checksum?: string;
  status: string;
  created_at: string;
}

export interface GameServerMetrics {
  cpu_percent: number;
  memory_usage_mb: number;
  memory_limit_mb: number;
  disk_usage_gb: number;
  disk_limit_gb: number;
  online_players: number;
  max_players: number;
  tps: number;
  mspt: number;
  uptime_seconds: number;
}

export interface GameServer {
  id: string;
  user_id: string;
  name: string;
  slug: string;
  game_type: GameType;
  server_software: string;
  game_version: string;
  plan_name: string;
  vcpus: number;
  memory_mb: number;
  storage_gb: number;
  assigned_port: number;
  protocol: string;
  connection_address: string;
  custom_domain?: string;
  srv_record_hint?: string;
  max_players: number;
  online_player_count: number;
  motd: string;
  status: GameServerStatus;
  uptime_seconds: number;
  proxmox_node: string;
  auto_backup_enabled: boolean;
  backup_schedule: string;
  world_name: string;
  created_at: string;
  updated_at: string;
  template?: GameServerTemplate;
  players?: GameServerPlayer[];
  plugins?: GameServerPlugin[];
  mods?: GameServerMod[];
  worlds?: GameServerWorld[];
  backups?: GameServerBackup[];
}

// Managed Waydroid Android Cloud Types (PRD Addendum 66-83)

export type AndroidStatus = 'provisioning' | 'running' | 'stopped' | 'suspended' | 'error' | 'reinstalling';
export type AndroidImageType = 'standard' | 'developer' | 'testing' | 'custom';

export interface AndroidImageVersion {
  id: string;
  image_id: string;
  version_tag: string;
  channel: string;
  release_notes?: string;
  system_checksum?: string;
  vendor_checksum?: string;
  is_active: boolean;
  created_at: string;
}

export interface AndroidImage {
  id: string;
  name: string;
  slug: string;
  android_version: string;
  architecture: string;
  image_type: AndroidImageType;
  system_image_url: string;
  vendor_image_url: string;
  image_source: string;
  checksum_sha256: string;
  min_cpu: number;
  min_ram_mb: number;
  min_storage_gb: number;
  is_published: boolean;
  is_verified: boolean;
  description: string;
  created_at: string;
  updated_at: string;
  versions?: AndroidImageVersion[];
}

export interface AndroidWorkerNode {
  id: string;
  name: string;
  proxmox_node_name: string;
  ip_address: string;
  status: string;
  kernel_version: string;
  binder_status: string;
  total_cpu_cores: number;
  total_memory_mb: number;
  total_storage_gb: number;
  active_instances_count: number;
  last_health_check_at: string;
}

export interface AndroidApplication {
  id: string;
  instance_id: string;
  package_name: string;
  app_name: string;
  version_name: string;
  version_code: number;
  file_size_bytes: number;
  icon_url?: string;
  is_system_app: boolean;
  is_running: boolean;
  installed_at: string;
  last_launched_at?: string;
}

export interface AndroidSession {
  id: string;
  instance_id: string;
  user_id: string;
  session_token: string;
  access_type: string;
  client_ip: string;
  user_agent?: string;
  is_active: boolean;
  expires_at: string;
  started_at: string;
  ended_at?: string;
}

export interface AndroidBackup {
  id: string;
  instance_id: string;
  user_id: string;
  backup_name: string;
  backup_type: string;
  file_size_bytes: number;
  storage_path: string;
  checksum_sha256: string;
  is_automated: boolean;
  status: string;
  retention_days: number;
  created_at: string;
}

export interface AndroidMetrics {
  cpu_percent: number;
  memory_usage_mb: number;
  memory_limit_mb: number;
  storage_usage_gb: number;
  storage_limit_gb: number;
  fps: number;
  display_resolution: string;
  active_sessions_count: number;
  adb_connected: boolean;
  uptime_seconds: number;
}

export interface AndroidPlan {
  id: string;
  name: string;
  vcpus: number;
  memory_mb: number;
  storage_gb: number;
  suggested_use: string;
  monthly_price: number;
}

export interface AndroidInstance {
  id: string;
  owner_id: string;
  image_id: string;
  worker_node_id?: string;
  name: string;
  slug: string;
  status: AndroidStatus;
  plan_name: string;
  vcpus: number;
  memory_mb: number;
  storage_gb: number;
  display_resolution: string;
  display_density: number;
  orientation: 'portrait' | 'landscape';
  remote_access_type: string;
  access_url: string;
  session_token?: string;
  ip_address: string;
  gateway_port: number;
  adb_enabled: boolean;
  adb_port: number;
  adb_auth_key?: string;
  persistence_enabled: boolean;
  auto_backup_enabled: boolean;
  backup_schedule: string;
  uptime_seconds: number;
  created_at: string;
  updated_at: string;
  image?: AndroidImage;
  worker_node?: AndroidWorkerNode;
  applications?: AndroidApplication[];
  backups?: AndroidBackup[];
  sessions?: AndroidSession[];
}




// --- Runtime System Configuration (GET /system/config, public) ---
export interface SystemTelemetry {
  node_name?: string;
  proxmox_node_name?: string;
  cpu_threads?: number;
  cpu_cores?: number;
  cpu_model?: string;
  memory_total_mb?: number;
  memory_total_gb?: number;
  memory_used_gb?: number;
  disk_total_gb?: number;
  disk_total_tb?: string;
  disk_used_gb?: number;
  pve_version?: string;
  kernel_version?: string;
  status?: string;
}

export interface SystemConfig {
  environment: 'production' | 'development' | (string & {});
  demo_mode: boolean;
  proxmox_mock: boolean;
  version: string;
  features: {
    docker_viewer: boolean;
    /** Public domains. Always equal to `domains.enabled` after normalization (fail closed). */
    domains: boolean;
  };
  domains: DomainsConfig;
  auth: {
    password_login: boolean;
    social_login: boolean;
    registration: boolean;
  };
  telemetry?: SystemTelemetry;
}

// --- Public Domains (https://<subdomain>.<root> → a port of the customer's VM) ---
export interface DomainsConfig {
  /** True only when the backend enables the feature and reports a usable root domain. */
  enabled: boolean;
  /** Root domain the subdomains live under, e.g. "nevatal.id" (lowercase, no leading dot). */
  root: string;
  /** Per-user limit; 0 when unknown (the server still enforces DOMAIN_LIMIT_REACHED). */
  max_per_user: number;
}

export type DomainLiveStatus =
  | 'live'
  | 'disabled'
  | 'instance_missing'
  | 'instance_stopped'
  | 'no_ip'
  | 'ip_not_routable'
  | 'ip_conflict';

export type DomainCheckReasonCode = '' | 'INVALID' | 'RESERVED' | 'TAKEN' | 'QUARANTINED';

export interface DomainOwner {
  id: string;
  email: string;
  display_name: string;
}

export interface Domain {
  id: string;
  subdomain: string;
  hostname: string;
  url: string;
  instance_id: string | null;
  instance_name: string;
  instance_status: InstanceStatus | (string & {});
  target_port: number;
  target_ip: string | null;
  enabled: boolean;
  admin_disabled: boolean;
  disabled_reason: string;
  live_status: DomainLiveStatus | (string & {});
  created_at: string;
  updated_at: string;
  /** Present on the admin listing only. */
  owner?: DomainOwner;
}

export interface DomainAvailability {
  subdomain: string;
  hostname: string;
  available: boolean;
  reason_code: DomainCheckReasonCode | (string & {});
  reason: string;
}

export interface DomainPatch {
  instance_id?: string;
  target_port?: number;
  enabled?: boolean;
}

// --- Docker Viewer (read-only, via QEMU guest agent) ---
export type DockerReasonCode =
  | ''
  | 'VM_NOT_RUNNING'
  | 'GUEST_AGENT_UNAVAILABLE'
  | 'GUEST_AGENT_TIMEOUT'
  | 'DOCKER_NOT_INSTALLED'
  | 'DOCKER_DAEMON_UNAVAILABLE'
  | 'DOCKER_PERMISSION_DENIED';

export type DockerContainerState =
  | 'running'
  | 'exited'
  | 'paused'
  | 'restarting'
  | 'created'
  | 'dead'
  | 'removing';

export interface DockerContainer {
  id: string;
  name: string;
  image: string;
  state: DockerContainerState | (string & {});
  status: string;
  ports: string;
  created_at: string;
  command: string;
  cpu_percent?: string;
  mem_usage?: string;
  mem_percent?: string;
}

export interface DockerOverview {
  instance_id: string;
  hostname: string;
  vmid: number;
  node: string;
  available: boolean;
  reason_code: DockerReasonCode | (string & {});
  reason: string;
  docker_version: string;
  containers: DockerContainer[];
  fetched_at: string;
}

export interface DockerLogLine {
  ts: string;
  stream: 'stdout' | 'stderr';
  message: string;
}

export interface DockerLogs {
  instance_id: string;
  container: string;
  tail: number;
  since: string;
  lines: DockerLogLine[];
  truncated: boolean;
  fetched_at: string;
}

// --- AI Infrastructure Orchestrator, Phase 1 (contract: docs/ORCHESTRATOR_API.md) ---

/** Task risk = tool risk; plan risk = the highest risk of its tasks. */
export type RiskLevel = 'read' | 'low' | 'medium' | 'high' | 'destructive';

/** `awaiting_confirmation` is the pre-orchestrator spelling of `awaiting_approval`. */
export type OrchestratorPlanStatus =
  | 'draft'
  | 'validating'
  | 'awaiting_approval'
  | 'awaiting_confirmation'
  | 'approved'
  | 'executing'
  | 'verifying'
  | 'completed'
  | 'rejected'
  | 'cancelled'
  | 'failed'
  | 'partially_completed'
  | (string & {});

export interface PlanSpecResource {
  key: string;
  type: string;
  action: string;
  config: Record<string, unknown>;
}

export interface PlanSpecification {
  intent?: string;
  resources?: PlanSpecResource[];
  [field: string]: unknown;
}

export interface PlanTask {
  key: string;
  tool: string;
  title: string;
  depends_on: string[];
  risk: RiskLevel;
  input?: Record<string, unknown>;
}

export interface PlanValidationIssue {
  code: string;
  message: string;
  task_key?: string;
}

export interface PlanValidation {
  status: 'passed' | 'warnings' | 'failed' | (string & {});
  checked_at?: string;
  errors: PlanValidationIssue[];
  warnings: PlanValidationIssue[];
}

export interface ConfirmationRequirement {
  type: 'typed_name' | (string & {});
  expected: string[];
}

export interface PlanRevision {
  version: number;
  spec_hash: string;
  created_at: string;
  created_by: 'planner' | 'user' | (string & {});
  summary: string;
  specification: PlanSpecification;
  tasks: PlanTask[];
  estimated_resources: Record<string, unknown>;
  risk_level: RiskLevel;
  requires_approval: boolean;
  confirmation_required: ConfirmationRequirement | null;
  validation: PlanValidation | null;
}

export interface PlanRevisionSummary {
  version: number;
  spec_hash: string;
  created_at: string;
  created_by: string;
  summary: string;
  risk_level: RiskLevel;
}

export type ApprovalStatus = 'approved' | 'superseded' | 'expired' | 'rejected' | 'consumed' | (string & {});

export interface PlanApproval {
  id: string;
  version: number;
  spec_hash: string;
  status: ApprovalStatus;
  risk_level: RiskLevel;
  expires_at: string;
  created_at: string;
}

/** GET /ai/plans/:id: the legacy plan fields plus the orchestrator fields. */
export interface OrchestratorPlan {
  id: string;
  conversation_id: string;
  user_id?: string;
  version?: number;
  intent?: string;
  status: OrchestratorPlanStatus;
  title: string;
  description?: string;
  created_at?: string;
  updated_at?: string;
  current_version: number;
  revision: PlanRevision | null;
  approval: PlanApproval | null;
  latest_execution_id: string | null;
}

export type ExecutionStatus =
  | 'queued'
  | 'running'
  | 'verifying'
  | 'completed'
  | 'failed'
  | 'partially_completed'
  | 'cancelling'
  | 'cancelled'
  | (string & {});

export type TaskStatus =
  | 'queued'
  | 'blocked'
  | 'running'
  | 'retrying'
  | 'succeeded'
  | 'failed'
  | 'cancelled'
  | 'skipped'
  | 'compensating'
  | (string & {});

export interface VerificationCheck {
  name: string;
  status: 'pass' | 'fail' | 'skip' | (string & {});
  detail?: string;
}

export interface TaskVerification {
  status: 'verified' | 'unverified' | 'failed' | (string & {});
  checks: VerificationCheck[];
}

export interface ExecutionTask {
  id: string;
  key: string;
  tool: string;
  title: string;
  risk: RiskLevel;
  depends_on: string[];
  status: TaskStatus;
  attempt: number;
  max_attempts: number;
  next_attempt_at: string | null;
  started_at: string | null;
  finished_at: string | null;
  output: Record<string, unknown> | null;
  error: { code: string; message: string } | null;
  verification: TaskVerification | null;
}

export interface ExecutionReportResource {
  type: string;
  id: string;
  name: string;
  status: string;
  access?: Record<string, string>;
}

export interface ExecutionReport {
  summary: string;
  resources: ExecutionReportResource[];
  warnings: string[];
  next_actions: string[];
}

export type RecoveryAction = 'retry_failed' | 'keep' | 'delete_created';

export interface RecoveryOption {
  action: RecoveryAction | (string & {});
  label: string;
  requires_approval: boolean;
}

export interface Execution {
  id: string;
  plan_id: string;
  plan_version: number;
  conversation_id: string | null;
  status: ExecutionStatus;
  cancel_requested: boolean;
  created_at: string;
  started_at: string | null;
  finished_at: string | null;
  tasks: ExecutionTask[];
  report: ExecutionReport | null;
  recovery_options: RecoveryOption[];
  /** Only on ExecutionSummary (GET /ai/executions). */
  task_counts?: Partial<Record<string, number>>;
}

export type ExecutionSummary = Omit<Execution, 'tasks'> & { tasks?: ExecutionTask[] };

export type ExecutionEventSeverity = 'info' | 'warning' | 'error' | (string & {});

export interface ExecutionEvent {
  seq: number;
  id: string;
  type: string;
  severity: ExecutionEventSeverity;
  message: string;
  task_key: string | null;
  metadata: Record<string, unknown>;
  created_at: string;
}

export interface OrchestratorTool {
  id: string;
  title: string;
  description: string;
  risk: RiskLevel;
  resource_types: string[];
  idempotent: boolean;
  timeout_seconds: number;
  requires_admin: boolean;
  input_schema: unknown;
}

/** POST /ai/plans/:id/approve response. */
export interface ApprovePlanResult {
  approval: PlanApproval | null;
  execution: Execution | null;
  plan_id: string;
  status: string;
  message: string;
}

// --- AI Infrastructure Orchestrator, Phases 2-4 (contract: docs/ORCHESTRATOR_API.md) ---

export type ManagedDatabaseEngine = 'postgresql' | 'mysql' | 'redis' | 'mongodb' | (string & {});
export type ManagedDatabaseStatus = 'provisioning' | 'running' | 'stopped' | 'failed' | 'deleting' | (string & {});

export interface ManagedDatabase {
  id: string;
  owner_id: string;
  name: string;
  engine: ManagedDatabaseEngine;
  engine_version: string;
  status: ManagedDatabaseStatus;
  plan_name: string;
  vcpus: number;
  memory_mb: number;
  storage_gb: number;
  host: string;
  port: number;
  database_name: string;
  username: string;
  auto_backup: boolean;
  last_backup_at?: string | null;
  last_health_check_at?: string | null;
  cpu_percent: number;
  memory_usage_percent: number;
  storage_usage_percent: number;
  connection_count: number;
  created_at: string;
  updated_at: string;
  deleted_at?: string | null;
}

export interface DatabasePlan {
  id: string;
  name: string;
  vcpus: number;
  memory_mb: number;
  storage_gb: number;
}

export interface DatabaseBackup {
  id: string;
  database_id: string;
  name: string;
  backup_reference: string;
  size_bytes: number;
  status: string;
  created_at: string;
}

export interface DatabaseCredentials {
  host: string;
  port: number;
  database_name: string;
  username: string;
  password: string;
  uri: string;
}

export interface DatabaseMetrics {
  cpu_percent: number;
  memory_usage_percent: number;
  storage_usage_percent: number;
  connection_count: number;
  status: string;
  timestamp: string;
}

export interface CreateDatabaseInput {
  name: string;
  engine: ManagedDatabaseEngine;
  engine_version?: string;
  plan_name: string;
  vcpus?: number;
  memory_mb?: number;
  storage_gb?: number;
  database_name?: string;
  auto_backup?: boolean;
}

export interface CreateDatabaseResult {
  database: ManagedDatabase;
  password: string;
}

export type TemplateParameterType = 'string' | 'integer' | 'boolean' | 'enum' | (string & {});

export interface TemplateParameter {
  key: string;
  label: string;
  description?: string;
  type: TemplateParameterType;
  required: boolean;
  default?: unknown;
  enum?: string[];
  min?: number;
  max?: number;
}

export interface InfraTemplate {
  id: string;
  owner_id: string;
  name: string;
  description: string;
  intent: string;
  version: number;
  is_public: boolean;
  usage_count: number;
  validation_status: string;
  specification: PlanSpecification;
  parameters: TemplateParameter[];
  created_at: string;
  updated_at: string;
}

export interface SaveTemplateInput {
  name: string;
  description: string;
  specification: PlanSpecification;
  parameters: TemplateParameter[];
  is_public: boolean;
}

/** POST /ai/templates/:id/deploy: plans are created, never executed. */
export interface DeployTemplateResult {
  plan_id: string;
  conversation_id: string;
  status: OrchestratorPlanStatus;
}

export type NotificationType = 'execution' | 'check' | 'advisor' | 'system' | (string & {});
export type NotificationSeverity = 'info' | 'warning' | 'critical' | (string & {});

export interface NotificationItem {
  id: string;
  user_id: string;
  type: NotificationType;
  severity: NotificationSeverity;
  title: string;
  message: string;
  resource_type: string;
  resource_id?: string | null;
  action_url: string;
  read_at?: string | null;
  created_at: string;
}

export interface NotificationList {
  unread_count: number;
  notifications: NotificationItem[];
}

export type InfraCheckType =
  | 'instance.health'
  | 'app.health'
  | 'game_server.health'
  | 'android.health'
  | 'database.health'
  | 'backup.freshness'
  | 'quota.usage'
  | (string & {});

export type InfraCheckStatus = 'ok' | 'warning' | 'critical' | 'unknown' | (string & {});

export interface InfraCheck {
  id: string;
  owner_id: string;
  name: string;
  resource_type: string;
  resource_id: string;
  check_type: InfraCheckType;
  interval_seconds: number;
  enabled: boolean;
  status: InfraCheckStatus;
  consecutive_failures: number;
  last_run_at?: string | null;
  next_run_at: string;
  created_at: string;
  updated_at: string;
}

export interface CheckView extends InfraCheck {
  last_result: {
    status?: string;
    message?: string;
    facts?: Record<string, unknown>;
    observed_at?: string;
    [key: string]: unknown;
  };
}

export interface CreateCheckInput {
  name: string;
  resource_type: string;
  resource_id: string;
  check_type: InfraCheckType;
  interval_seconds: number;
  enabled?: boolean;
}

export type PolicyTrigger =
  | 'execution.failed'
  | 'execution.completed'
  | 'check.failed'
  | 'advisor.recommendation'
  | (string & {});

export type PolicyAction = 'notify' | 'create_plan' | (string & {});

export interface AutomationPolicy {
  id: string;
  name: string;
  trigger: PolicyTrigger;
  conditions: Record<string, string>;
  action: PolicyAction;
  action_config: Record<string, unknown>;
  enabled: boolean;
  last_triggered_at?: string | null;
  trigger_count: number;
  created_at: string;
  updated_at: string;
}

export interface SavePolicyInput {
  name: string;
  trigger: PolicyTrigger;
  conditions: Record<string, string>;
  action: PolicyAction;
  action_config: Record<string, unknown>;
  enabled?: boolean;
}

export type DiagnosticHealth = 'healthy' | 'degraded' | 'critical' | 'unknown' | (string & {});
export type DiagnosticFactStatus = 'ok' | 'warning' | 'critical' | 'info' | (string & {});

export interface DiagnosticFact {
  name: string;
  value: string;
  status: DiagnosticFactStatus;
}

export interface DiagnosticHypothesis {
  cause: string;
  confidence: 'high' | 'medium' | 'low' | (string & {});
  evidence: string[];
}

export interface DiagnosticRecommendation {
  action: string;
  label: string;
  risk: RiskLevel;
  requires_approval: boolean;
  tool: string;
  input: Record<string, unknown>;
}

export interface DiagnosticResult {
  resource_type: string;
  resource_id: string;
  name: string;
  health: DiagnosticHealth;
  observed_at: string;
  facts: DiagnosticFact[];
  hypotheses: DiagnosticHypothesis[];
  recommendations: DiagnosticRecommendation[];
}

export interface RemediationInput {
  action: string;
  vcpus?: number;
  memory_mb?: number;
  disk_gb?: number;
}

export interface AdvisorCostItem {
  resource_type: string;
  id: string;
  name: string;
  vcpus: number;
  memory_mb: number;
  disk_gb: number;
  size_bytes: number;
  status: string;
  monthly: number;
}

export interface CostReport {
  currency: string;
  monthly_total: number;
  resources: AdvisorCostItem[];
  notes: string[];
}

export interface Optimization {
  id: string;
  category: 'waste' | 'capacity' | 'rightsize' | 'reliability' | (string & {});
  title: string;
  resource_type: string;
  resource_id: string;
  resource_name: string;
  current: string;
  observed: string;
  proposed: string;
  expected_benefit: string;
  risk: RiskLevel;
  requires_approval: boolean;
  tool: string;
  input: Record<string, unknown>;
}

export interface AdvisorReport {
  generated_at: string;
  currency: string;
  monthly_total: number;
  cost_resources: AdvisorCostItem[];
  optimizations: Optimization[];
  notes: string[];
  quota?: QuotaUsage | null;
}

export interface TopologyNode {
  id: string;
  type: string;
  name: string;
  status: string;
  metadata?: Record<string, unknown>;
}

export interface TopologyEdge {
  from: string;
  from_type: string;
  to: string;
  to_type: string;
  relation: string;
  to_name?: string;
}

export interface TopologyView {
  generated_at: string;
  nodes: TopologyNode[];
  edges: TopologyEdge[];
}

export interface AgentRole {
  id: string;
  name: string;
  description: string;
  tools: string[];
}

/** POST /ai/diagnostics/:type/:id/remediate returns the same shape as GET /ai/plans/:id. */
export type RemediationPlan = OrchestratorPlan;
