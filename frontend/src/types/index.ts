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
