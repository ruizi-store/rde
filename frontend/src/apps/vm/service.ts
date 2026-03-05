/**
 * VM Service - 虚拟机管理
 *
 * 后端路由: /api/v1/vm/...
 */

import { api } from "$shared/services/api";

// ==================== 类型定义 ====================

export interface VM {
  id: string;
  name: string;
  description?: string;
  status: VMStatus;
  memory: number; // MB
  cpu: number;
  disk_size: number; // GB
  disk_path: string;
  iso_path?: string;
  vnc_port: number;
  spice_port?: number;
  ssh_port?: number;
  port_forwards?: PortForward[];
  usb_devices?: USBDevice[];      // USB 设备直通
  network_mode?: string;          // user, bridge, none
  bridge_iface?: string;          // 桥接网卡名
  // 性能优化
  cpu_model?: string;             // host, host-passthrough, qemu64
  enable_huge?: boolean;          // 大页内存
  io_thread?: boolean;            // I/O 线程
  cpu_pinning?: number[];         // CPU 绑定
  // P5: 自动启动
  auto_start?: boolean;           // 系统启动时自动运行
  start_order?: number;           // 启动顺序
  start_delay?: number;           // 启动延迟秒数
  os_type?: string;
  arch?: string;
  accelerator?: string;
  pid?: number;
  use_kvm?: boolean;
  created_at: string;
  updated_at?: string;
}

export interface PortForward {
  name?: string;
  protocol?: "tcp" | "udp";
  host_port: number;
  guest_port: number;
}

export type VMStatus = "stopped" | "starting" | "running" | "stopping" | "paused" | "error";

export interface CreateVMRequest {
  name: string;
  description?: string;
  cpu?: number;
  memory?: number;
  disk_size?: number;
  iso_path?: string;
  os_type?: string;
  arch?: string;
  template?: string;
  auto_start?: boolean;
  port_forwards?: PortForward[];
  usb_devices?: USBDevice[];
  network_mode?: string;
  bridge_iface?: string;
  // 性能优化
  cpu_model?: string;
  enable_huge?: boolean;
  io_thread?: boolean;
  cpu_pinning?: number[];
}

export interface UpdateVMRequest {
  name?: string;
  description?: string;
  cpu?: number;
  memory?: number;
  port_forwards?: PortForward[];
  usb_devices?: USBDevice[];
  network_mode?: string;
  bridge_iface?: string;
  // 性能优化
  cpu_model?: string;
  enable_huge?: boolean;
  io_thread?: boolean;
  cpu_pinning?: number[];
}

export interface ISOFile {
  name: string;
  path: string;
  size: number;
  mod_time: string;
}

export interface Snapshot {
  id: string;
  vm_id: string;
  name: string;
  tag?: string;
  size?: number;
  parent?: string;
  created_at: string;
}

export interface VMTemplate {
  id: string;
  name: string;
  description?: string;
  os?: string;
  os_type: string;
  arch?: string;
  memory: number;
  cpu: number;
  disk_size: number;
  base_disk?: string;
  is_custom?: boolean;
  created_at?: string;
}

export interface CreateTemplateRequest {
  vm_id: string;
  name: string;
  description?: string;
}

export interface BackupInfo {
  id: string;
  vm_id: string;
  vm_name: string;
  name?: string;
  path: string;
  size: number;
  compressed: boolean;
  created_at: string;
  description?: string;
}

export interface CreateBackupRequest {
  vm_id: string;
  name?: string;
  description?: string;
  compress?: boolean;
}

export interface VNCInfo {
  host: string;
  port: number;
  websocket?: string;
}

export interface VNCToken {
  token: string;
}

// ==================== 服务实现 ====================

class VMService {
  // ---- VM CRUD ----

  async listVMs(): Promise<VM[]> {
    return api.get<VM[]>("/vm/vms");
  }

  async getVM(id: string): Promise<VM> {
    return api.get<VM>(`/vm/vms/${id}`);
  }

  async createVM(req: CreateVMRequest): Promise<VM> {
    return api.post<VM>("/vm/vms", req);
  }

  async updateVM(id: string, req: UpdateVMRequest): Promise<VM> {
    return api.put<VM>(`/vm/vms/${id}`, req);
  }

  async deleteVM(id: string): Promise<void> {
    await api.delete(`/vm/vms/${id}`);
  }

  // ---- VM 控制 ----

  async startVM(id: string): Promise<void> {
    await api.post(`/vm/vms/${id}/start`);
  }

  async stopVM(id: string, force = false): Promise<void> {
    await api.post(`/vm/vms/${id}/stop${force ? "?force=true" : ""}`);
  }

  async pauseVM(id: string): Promise<void> {
    await api.post(`/vm/vms/${id}/pause`);
  }

  async resumeVM(id: string): Promise<void> {
    await api.post(`/vm/vms/${id}/resume`);
  }

  // ---- VNC ----

  async getVNCInfo(id: string): Promise<VNCInfo> {
    return api.get<VNCInfo>(`/vm/vms/${id}/vnc`);
  }

  async getVNCToken(id: string): Promise<VNCToken> {
    return api.post<VNCToken>(`/vm/vms/${id}/vnc/token`);
  }

  /**
   * 获取 VNC WebSocket URL
   */
  getVNCWebSocketURL(token: string): string {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = window.location.host;
    return `${protocol}//${host}/api/v1/vm/vnc/websocket?token=${token}`;
  }

  // ---- QMP 操作 ----

  async sendKey(id: string, keys: string[]): Promise<void> {
    await api.post(`/vm/vms/${id}/sendkey`, { keys });
  }

  async sendCtrlAltDel(id: string): Promise<void> {
    await api.post(`/vm/vms/${id}/ctrlaltdel`);
  }

  async screendump(id: string): Promise<{ path: string }> {
    return api.post(`/vm/vms/${id}/screendump`);
  }

  // ---- 磁盘 ----

  async resizeDisk(id: string, newSizeGB: number): Promise<void> {
    await api.post(`/vm/vms/${id}/resize`, { size: newSizeGB });
  }

  // ---- 快照 ----

  async listSnapshots(vmId: string): Promise<Snapshot[]> {
    return api.get<Snapshot[]>(`/vm/vms/${vmId}/snapshots`);
  }

  async createSnapshot(vmId: string, name: string): Promise<Snapshot> {
    return api.post<Snapshot>(`/vm/vms/${vmId}/snapshots`, { name });
  }

  async deleteSnapshot(vmId: string, tag: string): Promise<void> {
    await api.delete(`/vm/vms/${vmId}/snapshots/${tag}`);
  }

  async revertSnapshot(vmId: string, tag: string): Promise<void> {
    await api.post(`/vm/vms/${vmId}/snapshots/${tag}/revert`);
  }

  // ---- ISO & 模板 ----

  async listISOs(): Promise<ISOFile[]> {
    return api.get<ISOFile[]>("/vm/isos");
  }

  async uploadISO(file: File): Promise<{ path: string }> {
    const formData = new FormData();
    formData.append("file", file);
    return api.upload<{ path: string }>("/vm/isos", formData);
  }

  async deleteISO(name: string): Promise<void> {
    await api.delete(`/vm/isos/${encodeURIComponent(name)}`);
  }

  async getTemplates(): Promise<VMTemplate[]> {
    return api.get<VMTemplate[]>("/vm/templates");
  }

  async createTemplate(req: CreateTemplateRequest): Promise<VMTemplate> {
    return api.post<VMTemplate>("/vm/templates", req);
  }

  async deleteTemplate(id: string): Promise<void> {
    await api.delete(`/vm/templates/${id}`);
  }

  // ---- 备份管理 ----

  async listBackups(vmId?: string): Promise<BackupInfo[]> {
    const query = vmId ? `?vm_id=${vmId}` : "";
    return api.get<BackupInfo[]>(`/vm/backups${query}`);
  }

  async createBackup(req: CreateBackupRequest): Promise<BackupInfo> {
    return api.post<BackupInfo>("/vm/backups", req);
  }

  async restoreBackup(backupId: string, newName?: string): Promise<VM> {
    return api.post<VM>(`/vm/backups/${backupId}/restore`, { new_name: newName });
  }

  async deleteBackup(id: string): Promise<void> {
    await api.delete(`/vm/backups/${id}`);
  }

  // ---- 资源统计 ----

  async getVMStats(id: string): Promise<VMStats> {
    return api.get<VMStats>(`/vm/vms/${id}/stats`);
  }

  async getAllVMStats(): Promise<VMStats[]> {
    return api.get<VMStats[]>("/vm/stats");
  }

  // ---- P3: 批量操作 ----

  async batchStart(ids: string[]): Promise<BatchResult[]> {
    return api.post<BatchResult[]>("/vm/batch/start", { ids });
  }

  async batchStop(ids: string[], force = false): Promise<BatchResult[]> {
    return api.post<BatchResult[]>("/vm/batch/stop", { ids, force });
  }

  async batchDelete(ids: string[]): Promise<BatchResult[]> {
    return api.post<BatchResult[]>("/vm/batch/delete", { ids });
  }

  // ---- P3: 克隆 ----

  async cloneVM(id: string, name: string): Promise<VM> {
    return api.post<VM>(`/vm/vms/${id}/clone`, { name });
  }

  // ---- P3: 存储信息 ----

  async getStorageInfo(): Promise<StorageInfo> {
    return api.get<StorageInfo>("/vm/storage");
  }

  // ---- P4: USB设备和网络接口 ----

  async listUSBDevices(): Promise<USBDevice[]> {
    return api.get<USBDevice[]>("/vm/usb-devices");
  }

  async listNetworkInterfaces(): Promise<NetworkInterface[]> {
    return api.get<NetworkInterface[]>("/vm/network-interfaces");
  }

  // ---- P5: 自动启动 ----

  async setAutoStart(id: string, enabled: boolean, order = 0, delay = 0): Promise<void> {
    await api.put(`/vm/vms/${id}/autostart`, { enabled, order, delay });
  }

  async getAutoStartVMs(): Promise<VM[]> {
    return api.get<VM[]>("/vm/autostart");
  }

  async runAutoStart(): Promise<BatchResult[]> {
    return api.post<BatchResult[]>("/vm/autostart/run", {});
  }

  // ---- P5: 导入导出 ----

  async exportVM(req: ExportVMRequest): Promise<ExportResult> {
    return api.post<ExportResult>("/vm/export", req);
  }

  async importVM(req: ImportVMRequest): Promise<VM> {
    return api.post<VM>("/vm/import", req);
  }

  // ---- P5: 实时快照 ----

  async createLiveSnapshot(id: string, req: LiveSnapshotRequest): Promise<Snapshot> {
    return api.post<Snapshot>(`/vm/vms/${id}/live-snapshot`, req);
  }

  // ---- P6: SSH 终端集成 ----

  async getVMSSHInfo(id: string): Promise<VMSSHSession> {
    return api.get<VMSSHSession>(`/vm/vms/${id}/ssh-info`);
  }

  // ---- P6: 资源监控 ----

  async getVMResources(id: string): Promise<VMResourceStats> {
    return api.get<VMResourceStats>(`/vm/vms/${id}/resources`);
  }

  async getAllVMResources(): Promise<VMResourceStats[]> {
    return api.get<VMResourceStats[]>("/vm/resources");
  }
}

export interface VMStats {
  vm_id: string;
  cpu_percent: number;
  memory_used: number;
  memory_total: number;
  disk_read: number;
  disk_write: number;
  net_rx: number;
  net_tx: number;
  uptime: number;
  timestamp: string;
}

// ==================== P3: 批量操作和存储 ====================

export interface BatchResult {
  id: string;
  success: boolean;
  error?: string;
}

export interface StorageInfo {
  total_space: number;
  used_space: number;
  free_space: number;
  vm_disk_usage: number;
  iso_usage: number;
  backup_usage: number;
  template_usage: number;
  snapshot_usage: number;
  vm_count: number;
  running_vm_count: number;
}

// ==================== P4: USB设备和网络接口 ====================

export interface USBDevice {
  vendor_id: string;
  product_id: string;
  name?: string;
  bus?: number;
  device?: number;
}

export interface NetworkInterface {
  name: string;
  type: string;  // bridge, ethernet, wireless, virtual, other
  status: string;
  address?: string;
}

// ==================== P5: 导入导出和自动化 ====================

export type ExportFormat = "ova" | "qcow2" | "raw";

export interface ExportVMRequest {
  vm_id: string;
  format?: ExportFormat;
  include_iso?: boolean;
  compress?: boolean;
}

export interface ExportResult {
  path: string;
  size: number;
  format: string;
  checksum?: string;
}

export interface ImportVMRequest {
  path: string;
  name?: string;
  description?: string;
}

export interface AutoStartConfig {
  vm_id: string;
  enabled: boolean;
  order: number;
  delay: number;
}

export interface LiveSnapshotRequest {
  name: string;
  description?: string;
  include_ram?: boolean;
}

// ==================== P6: SSH 终端集成 ====================

export interface VMSSHSession {
  id: string;
  vm_id: string;
  vm_name: string;
  host: string;
  port: number;
  username?: string;
  created_at: number;
}

// ==================== P6: 资源监控仪表板 ====================

export interface VMResourceStats {
  vm_id: string;
  timestamp: number;
  cpu: VMCPUStats;
  memory: VMMemoryStats;
  disks: VMDiskStats[];
  networks: VMNetworkStats[];
}

export interface VMCPUStats {
  usage_percent: number;
  cpu_time: number;
  vcpus: number;
}

export interface VMMemoryStats {
  total: number;      // MB
  used: number;       // MB
  available: number;  // MB
  used_percent: number;
  balloon: number;
}

export interface VMDiskStats {
  device: string;
  path?: string;
  bytes_read: number;
  bytes_written: number;
  ops_read: number;
  ops_written: number;
  read_speed: number;   // B/s
  write_speed: number;  // B/s
}

export interface VMNetworkStats {
  device: string;
  bytes_rx: number;
  bytes_tx: number;
  packets_rx: number;
  packets_tx: number;
  rx_speed: number;   // B/s
  tx_speed: number;   // B/s
}

export const vmService = new VMService();
