/**
 * Docker 服务
 * 管理 Docker 容器、镜像和网络
 *
 * 后端路由: /api/v1/docker/...
 */

import { api } from "$shared/services/api";

// ==================== 类型定义 ====================

export interface DockerContainer {
  id: string;
  name: string;
  image: string;
  image_id?: string;
  state: "running" | "stopped" | "paused" | "restarting" | "created" | "exited";
  status: string;
  ports: DockerPort[];
  created: string;
}

export interface DockerPort {
  host_port?: string | number;
  container_port?: string | number;
  private_port?: number;
  public_port?: number;
  protocol?: string;
  type?: "tcp" | "udp";
  ip?: string;
}

export interface DockerImage {
  id: string;
  repository?: string;
  tag?: string;
  tags: string[];
  size: number;
  created: string;
}

export interface DockerStats {
  cpu_percent: number;
  memory_usage: number;
  memory_limit: number;
  memory_percent: number;
  network_rx: number;
  network_tx: number;
  block_read?: number;
  block_write?: number;
}

export interface DockerNetwork {
  id: string;
  name: string;
  driver: string;
  scope: string;
  subnet?: string;
  gateway?: string;
}

export interface DockerInfo {
  version: string;
  api_version?: string;
  os?: string;
  arch?: string;
  containers: number;
  running?: number;
  stopped?: number;
  images: number;
  mem_total?: number;
  ncpu?: number;
}

export interface ContainerStatus {
  id: string;
  name: string;
  image: string;
  state: string;
  running: boolean;
  started_at?: string;
  exit_code?: number;
}

export interface CreateContainerRequest {
  name: string;
  image: string;
  ports?: Record<string, string>;
  volumes?: Record<string, string>;
  environment?: string[];
  env?: Record<string, string>;
  networks?: string[];
  labels?: Record<string, string>;
  restart?: string;
  restart_policy?: string;
  privileged?: boolean;
  command?: string;
}

export interface CreateNetworkRequest {
  name: string;
  driver?: string;
}

// ==================== 服务实现 ====================

class DockerService {
  // ---- 辅助 ----
  private unwrap<T>(res: any): T {
    return (res as any).data ?? res;
  }

  // ---- 系统 ----

  async getInfo(): Promise<DockerInfo> {
    const res = await api.get<{ data: DockerInfo }>("/docker/info");
    return this.unwrap(res);
  }

  async getStatus(): Promise<{ running: boolean }> {
    const res = await api.get<{ data: { running: boolean } }>("/docker/status");
    return this.unwrap(res);
  }

  async isAvailable(): Promise<boolean> {
    try { await this.getInfo(); return true; } catch { return false; }
  }

  // ---- 容器 ----

  async getContainers(all = true): Promise<DockerContainer[]> {
    const res = await api.get<{ data: DockerContainer[] }>(`/docker/containers?all=${all}`);
    return this.unwrap(res);
  }

  async getContainerStatus(id: string): Promise<ContainerStatus> {
    const res = await api.get<{ data: ContainerStatus }>(`/docker/containers/${id}`);
    return this.unwrap(res);
  }

  async createContainer(config: CreateContainerRequest): Promise<DockerContainer> {
    const res = await api.post<{ data: DockerContainer }>("/docker/containers", config);
    return this.unwrap(res);
  }

  async removeContainer(id: string, force = false): Promise<void> {
    await api.delete(`/docker/containers/${id}?force=${force}`);
  }

  async startContainer(id: string): Promise<void> {
    await api.post(`/docker/containers/${id}/start`);
  }

  async stopContainer(id: string): Promise<void> {
    await api.post(`/docker/containers/${id}/stop`);
  }

  async restartContainer(id: string): Promise<void> {
    await api.post(`/docker/containers/${id}/restart`);
  }

  async getContainerStats(id: string): Promise<DockerStats> {
    const res = await api.get<{ data: DockerStats }>(`/docker/containers/${id}/stats`);
    return this.unwrap(res);
  }

  async getContainerLogs(id: string, tail = 200): Promise<string> {
    const res = await api.get<{ data: string }>(`/docker/containers/${id}/logs?tail=${tail}`);
    const data = this.unwrap<any>(res);
    return typeof data === "string" ? data : data?.logs || "";
  }

  async execContainer(id: string, command: string): Promise<{ output: string }> {
    const res = await api.post<{ data: { output: string } }>(`/docker/containers/${id}/exec`, { command });
    return this.unwrap(res);
  }

  // ---- 镜像 ----

  async getImages(): Promise<DockerImage[]> {
    const res = await api.get<{ data: DockerImage[] }>("/docker/images");
    return this.unwrap(res);
  }

  async pullImage(image: string): Promise<void> {
    await api.post("/docker/images/pull", { image });
  }

  async removeImage(id: string, force = false): Promise<void> {
    await api.delete(`/docker/images/${id}?force=${force}`);
  }

  // ---- 网络 ----

  async getNetworks(): Promise<DockerNetwork[]> {
    const res = await api.get<{ data: DockerNetwork[] }>("/docker/networks");
    return this.unwrap(res);
  }

  async createNetwork(req: CreateNetworkRequest): Promise<DockerNetwork> {
    const res = await api.post<{ data: DockerNetwork }>("/docker/networks", req);
    return this.unwrap(res);
  }

  async removeNetwork(id: string): Promise<void> {
    await api.delete(`/docker/networks/${id}`);
  }
}

export const dockerService = new DockerService();
