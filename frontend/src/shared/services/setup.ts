// 类型定义
export interface SetupStatus {
  completed: boolean;
  current_step: number;
  completed_steps: number[];
  can_skip_setup: boolean;
}

export interface DependencyCheck {
  name: string;
  required: boolean;
  installed: boolean;
  version: string;
}

export interface PortCheck {
  port: number;
  in_use: boolean;
  in_use_process: string;
}

export interface DiskSpaceCheck {
  path: string;
  total_bytes: number;
  avail_bytes: number;
  min_required: number;
  sufficient: boolean;
}

export interface SystemCheckResult {
  dependencies: DependencyCheck[];
  ports: PortCheck[];
  disk_space: DiskSpaceCheck;
  all_passed: boolean;
}

export interface LocaleSettings {
  language: string;
  timezone: string;
  time_format: string;
  date_format: string;
}

export interface SetupUserRequest {
  username: string;
  password: string;
  avatar?: string;
  enable_2fa: boolean;
}

export interface TwoFactorSetup {
  secret: string;
  qr_code_url: string;
  backup_codes: string[];
}

export interface Partition {
  device_path: string;
  size: number;
  filesystem: string;
  mount_point: string;
  label: string;
}

export interface DetectedDrive {
  device_path: string;
  size: number;
  model: string;
  serial: string;
  partitions: Partition[];
}

// 前端使用的驱动器信息（从 DetectedDrive 转换）
export interface DriveInfo {
  id: string;
  name: string;
  type: "ssd" | "hdd" | "nvme" | "unknown";
  total_bytes: number;
  used_bytes: number;
  mount_point: string;
}

export interface DriveMount {
  device_path: string;
  mount_point: string;
  filesystem?: string;
  label?: string;
  auto_mount: boolean;
}

export interface NetworkConfig {
  mode?: "dhcp" | "static";
  ip_address?: string;
  netmask?: string;
  gateway?: string;
  dns?: string[];
  http_port?: number;
  https_port?: number;
}

export interface CompleteResponse {
  success: boolean;
  redirect_url: string;
  auto_login_token?: string;
}

// 恢复出厂设置
// 用户身份通过 JWT 验证，但需要再次输入密码确认
export interface FactoryResetRequest {
  password: string; // 当前用户密码（二次确认）
  confirm_text: string; // 必须是 "RESET"
  keep_docker_apps: boolean;
  keep_user_files: boolean;
}

export interface FactoryResetResponse {
  success: boolean;
  message: string;
  redirect_url: string;
}

export interface SecurityConfig {
  use_random_port: boolean;
  custom_port?: number;
}

export interface SecurityConfigResponse {
  port_changed: boolean;
  new_port?: number;
  message: string;
}

export interface DepInstallProgressInfo {
  package: string;
  status: string;
  progress: number;
}

// API 基础路径
const BASE_URL = "/api/v1/setup";

// Setup API 服务
export const setupApi = {
  // 获取初始化状态
  async getStatus(): Promise<SetupStatus> {
    const response = await fetch(`${BASE_URL}/status`);
    if (!response.ok) throw new Error("Failed to get setup status");
    return response.json();
  },

  // Step 1: 系统检查
  async checkSystem(): Promise<SystemCheckResult> {
    const response = await fetch(`${BASE_URL}/check`);
    if (!response.ok) throw new Error("Failed to check system");
    return response.json();
  },

  async completeStep1(): Promise<void> {
    const response = await fetch(`${BASE_URL}/check/complete`, { method: "POST" });
    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || "Failed to complete step 1");
    }
  },

  async installDeps(
    packages: string[],
    onProgress: (data: DepInstallProgressInfo) => void,
  ): Promise<void> {
    const response = await fetch(`${BASE_URL}/install-deps`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ packages }),
    });

    if (!response.ok) throw new Error("Failed to install dependencies");

    const reader = response.body?.getReader();
    if (!reader) return;

    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      buffer = lines.pop() || "";

      for (const line of lines) {
        if (line.startsWith("data: ")) {
          const data = JSON.parse(line.slice(6));
          onProgress(data);
        }
      }
    }
  },

  // Step 2: 语言时区
  async setLocale(settings: LocaleSettings): Promise<void> {
    const response = await fetch(`${BASE_URL}/locale`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(settings),
    });
    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || "Failed to set locale");
    }
  },

  // Step 3: 创建用户
  async createAdmin(user: SetupUserRequest): Promise<{ two_factor?: TwoFactorSetup }> {
    const response = await fetch(`${BASE_URL}/user`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(user),
    });
    if (!response.ok) {
      const data = await response.json();
      // 409 Conflict = admin already exists
      if (response.status === 409) {
        const err = new Error(data.error || "Admin already exists");
        (err as any).status = 409;
        throw err;
      }
      throw new Error(data.error || "Failed to create admin");
    }
    return response.json();
  },

  async verify2FA(userId: string, code: string): Promise<void> {
    const response = await fetch(`${BASE_URL}/user/verify-2fa?user_id=${userId}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code }),
    });
    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || "Failed to verify 2FA");
    }
  },

  // 安全配置（可选随机端口）
  async configureSecurity(config: SecurityConfig): Promise<SecurityConfigResponse> {
    const response = await fetch(`${BASE_URL}/security`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(config),
    });
    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || "Failed to configure security");
    }
    return response.json();
  },

  // Step 4: 存储配置
  async getDrives(): Promise<DriveInfo[]> {
    const response = await fetch(`${BASE_URL}/drives`);
    if (!response.ok) throw new Error("Failed to get drives");
    const data = await response.json();
    const detectedDrives: DetectedDrive[] = data.data || [];

    // 转换 DetectedDrive 为 DriveInfo
    return detectedDrives.map((drive, index) => {
      // 计算总大小和已用空间（从分区信息）
      const totalBytes = drive.size || 0;
      // 假设已用空间为 0（后端未提供此信息）
      const usedBytes = 0;

      // 从分区获取挂载点
      const mountedPartition = drive.partitions?.find((p) => p.mount_point);
      const mountPoint = mountedPartition?.mount_point || `/mnt/disk${index}`;

      // 推测磁盘类型
      let driveType: "ssd" | "hdd" | "nvme" | "unknown" = "unknown";
      const model = (drive.model || "").toLowerCase();
      if (model.includes("nvme")) {
        driveType = "nvme";
      } else if (model.includes("ssd") || model.includes("solid")) {
        driveType = "ssd";
      } else if (model.includes("hdd") || model.includes("hard")) {
        driveType = "hdd";
      }

      return {
        id: drive.device_path || `drive-${index}`,
        name: drive.model || `磁盘 ${index + 1}`,
        type: driveType,
        total_bytes: totalBytes,
        used_bytes: usedBytes,
        mount_point: mountPoint,
      };
    });
  },

  // Step 5: 网络设置
  async configureNetwork(config: NetworkConfig): Promise<void> {
    const response = await fetch(`${BASE_URL}/network`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(config),
    });
    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || "Failed to configure network");
    }
  },

  async skipNetwork(): Promise<void> {
    const response = await fetch(`${BASE_URL}/network/skip`, { method: "POST" });
    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || "Failed to skip network");
    }
  },

  // Step 6: 完成
  async complete(): Promise<CompleteResponse> {
    const response = await fetch(`${BASE_URL}/complete`, { method: "POST" });
    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || "Failed to complete setup");
    }
    return response.json();
  },

  // 恢复出厂设置（需要登录）
  async factoryReset(request: FactoryResetRequest): Promise<FactoryResetResponse> {
    // 从 localStorage 获取 token
    const token = typeof window !== "undefined" ? localStorage.getItem("auth_token") : null;

    const headers: HeadersInit = { "Content-Type": "application/json" };
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    const response = await fetch(`${BASE_URL}/factory-reset`, {
      method: "POST",
      headers,
      body: JSON.stringify(request),
    });
    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || "Failed to factory reset");
    }
    return response.json();
  },
};

// ==================== 模块设置 API ====================

export interface ModuleSetting {
  module_id: string;
  name: string;
  description: string;
  enabled: boolean;
  config: Record<string, unknown>;
}

export interface UpdateModuleRequest {
  enabled: boolean;
  config?: Record<string, unknown>;
}

// 模块设置 API
export const modulesApi = {
  // 获取所有模块设置
  async getAll(): Promise<ModuleSetting[]> {
    const response = await fetch("/api/v1/settings/modules");
    if (!response.ok) throw new Error("Failed to get module settings");
    const data = await response.json();
    return data.data;
  },

  // 更新模块设置
  async update(moduleId: string, request: UpdateModuleRequest): Promise<ModuleSetting> {
    const response = await fetch(`/api/v1/settings/modules/${moduleId}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });
    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || "Failed to update module");
    }
    const data = await response.json();
    return data.data;
  },
};
