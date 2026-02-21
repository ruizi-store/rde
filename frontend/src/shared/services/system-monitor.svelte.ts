/**
 * 系统监控 API 服务
 */
import { api } from "./api";

export interface CPUInfo {
  model_name: string;
  cores: number;
  threads: number;
  mhz: number;
  cache_size: number;
  usage: number;
  temperature: number;
}

export interface MemoryInfo {
  total: number;
  used: number;
  free: number;
  available: number;
  used_percent: number;
  swap_total: number;
  swap_used: number;
  swap_free: number;
}

export interface DiskInfo {
  path: string;
  total: number;
  used: number;
  free: number;
  used_percent: number;
  fs_type: string;
  mount_point: string;
}

export interface NetworkStats {
  interface: string;
  bytes_sent: number;
  bytes_recv: number;
  packets_sent: number;
  packets_recv: number;
  errors_in: number;
  errors_out: number;
}

export interface ResourceUsage {
  timestamp: string;
  cpu_usage: number;
  memory_usage: number;
  disk_usage: number;
  network_rx: number;
  network_tx: number;
}

export interface SystemStats {
  cpu: CPUInfo;
  memory: MemoryInfo;
  disks: DiskInfo[];
  network: NetworkStats[];
}

/**
 * 格式化字节为人类可读格式
 */
export function formatBytes(bytes: number, decimals = 1): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + " " + sizes[i];
}

/**
 * 格式化网络速度
 */
export function formatSpeed(bytesPerSecond: number): string {
  if (bytesPerSecond === 0) return "0 B/s";
  const k = 1024;
  const sizes = ["B/s", "KB/s", "MB/s", "GB/s"];
  const i = Math.floor(Math.log(bytesPerSecond) / Math.log(k));
  return parseFloat((bytesPerSecond / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}

/**
 * 获取 CPU 信息
 */
export async function getCPUInfo(): Promise<CPUInfo> {
  const res = await api.get<{ data: CPUInfo }>("/system/cpu");
  return res.data;
}

/**
 * 获取内存信息
 */
export async function getMemoryInfo(): Promise<MemoryInfo> {
  const res = await api.get<{ data: MemoryInfo }>("/system/memory");
  return res.data;
}

/**
 * 获取所有磁盘信息
 */
export async function getAllDisks(): Promise<DiskInfo[]> {
  const res = await api.get<{ data: DiskInfo[] }>("/system/disks");
  return res.data;
}

/**
 * 获取网络统计
 */
export async function getNetworkStats(): Promise<NetworkStats[]> {
  const res = await api.get<{ data: NetworkStats[] }>("/system/network/stats");
  return res.data;
}

/**
 * 获取资源使用综合信息
 */
export async function getResourceUsage(): Promise<ResourceUsage> {
  const res = await api.get<{ data: ResourceUsage }>("/system/usage");
  return res.data;
}

/**
 * 获取所有系统监控数据
 */
export async function getSystemStats(): Promise<SystemStats> {
  const [cpu, memory, disks, network] = await Promise.all([
    getCPUInfo(),
    getMemoryInfo(),
    getAllDisks(),
    getNetworkStats(),
  ]);

  return { cpu, memory, disks, network };
}

/**
 * 系统监控 Store（响应式）
 */
class SystemMonitor {
  private intervalId: ReturnType<typeof setInterval> | null = null;
  private previousNetwork: NetworkStats[] = [];
  private previousTimestamp = 0;

  // 响应式状态
  cpu = $state<CPUInfo | null>(null);
  memory = $state<MemoryInfo | null>(null);
  disks = $state<DiskInfo[]>([]);
  networkSpeed = $state<{ rx: number; tx: number }>({ rx: 0, tx: 0 });
  loading = $state(false);
  error = $state<string | null>(null);

  /**
   * 获取最新数据
   */
  async refresh() {
    try {
      this.loading = true;
      this.error = null;

      const [cpuData, memoryData, disksData, networkData] = await Promise.all([
        getCPUInfo(),
        getMemoryInfo(),
        getAllDisks(),
        getNetworkStats(),
      ]);

      this.cpu = cpuData;
      this.memory = memoryData;
      this.disks = disksData;

      // 计算网络速度
      const now = Date.now();
      if (this.previousNetwork.length > 0 && this.previousTimestamp > 0) {
        const elapsed = (now - this.previousTimestamp) / 1000; // 秒

        let totalRx = 0;
        let totalTx = 0;

        for (const current of networkData) {
          const prev = this.previousNetwork.find((n) => n.interface === current.interface);
          if (prev) {
            totalRx += (current.bytes_recv - prev.bytes_recv) / elapsed;
            totalTx += (current.bytes_sent - prev.bytes_sent) / elapsed;
          }
        }

        this.networkSpeed = { rx: Math.max(0, totalRx), tx: Math.max(0, totalTx) };
      }

      this.previousNetwork = networkData;
      this.previousTimestamp = now;
    } catch (err) {
      this.error = err instanceof Error ? err.message : "获取系统信息失败";
    } finally {
      this.loading = false;
    }
  }

  /**
   * 开始轮询
   */
  startPolling(intervalMs = 3000) {
    if (this.intervalId) return;

    // 立即获取一次
    this.refresh();

    this.intervalId = setInterval(() => {
      this.refresh();
    }, intervalMs);
  }

  /**
   * 停止轮询
   */
  stopPolling() {
    if (this.intervalId) {
      clearInterval(this.intervalId);
      this.intervalId = null;
    }
  }
}

// 单例导出
export const systemMonitor = new SystemMonitor();
