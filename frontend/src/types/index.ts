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
