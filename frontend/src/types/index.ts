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
  type: 'vps' | 'app' | 'automation' | 'ai_agent' | 'bucket' | 'game_server';
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


