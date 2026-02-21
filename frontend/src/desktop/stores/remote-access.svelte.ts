// 远程访问设置状态管理
// 管理 SSH 和终端的启用状态

import { systemService } from "$shared/services/system";

export interface RemoteAccessState {
  sshEnabled: boolean;
  sshRunning: boolean;
  sshPort: number;
  terminalEnabled: boolean;
  loading: boolean;
  loaded: boolean;
  error: string | null;
}

class RemoteAccessStore {
  private _state = $state<RemoteAccessState>({
    sshEnabled: false,
    sshRunning: false,
    sshPort: 22,
    terminalEnabled: false,
    loading: false,
    loaded: false,
    error: null,
  });

  // 获取终端是否启用
  get terminalEnabled(): boolean {
    return this._state.terminalEnabled;
  }

  // 获取 SSH 是否启用
  get sshEnabled(): boolean {
    return this._state.sshEnabled;
  }

  // 获取 SSH 是否运行中
  get sshRunning(): boolean {
    return this._state.sshRunning;
  }

  // 获取 SSH 端口
  get sshPort(): number {
    return this._state.sshPort;
  }

  // 是否正在加载
  get loading(): boolean {
    return this._state.loading;
  }

  // 是否已加载
  get loaded(): boolean {
    return this._state.loaded;
  }

  // 错误信息
  get error(): string | null {
    return this._state.error;
  }

  // 获取完整状态
  get state(): RemoteAccessState {
    return this._state;
  }

  // 从服务器加载设置
  async load(): Promise<void> {
    if (this._state.loading) return;

    this._state.loading = true;
    this._state.error = null;

    try {
      const response = await systemService.getRemoteAccessSettings();
      if (response.success === 200 && response.data) {
        this._state.sshEnabled = response.data.ssh_enabled;
        this._state.sshRunning = response.data.ssh_running;
        this._state.sshPort = response.data.ssh_port;
        this._state.terminalEnabled = response.data.terminal_enabled;
        this._state.loaded = true;
      } else {
        this._state.error = response.message || "加载远程访问设置失败";
      }
    } catch (e: any) {
      // 401 表示未登录或 token 过期，不需要在控制台报错（API 层会自动跳转登录页）
      if (e?.status === 401) {
        console.debug("远程访问设置：未认证，跳过加载");
      } else {
        this._state.error = e.message || "加载远程访问设置失败";
        console.error("加载远程访问设置失败:", e);
      }
    } finally {
      this._state.loading = false;
    }
  }

  // 更新设置（内部使用，由 Settings.svelte 调用后刷新）
  update(data: {
    ssh_enabled?: boolean;
    ssh_running?: boolean;
    ssh_port?: number;
    terminal_enabled?: boolean;
  }): void {
    if (data.ssh_enabled !== undefined) {
      this._state.sshEnabled = data.ssh_enabled;
    }
    if (data.ssh_running !== undefined) {
      this._state.sshRunning = data.ssh_running;
    }
    if (data.ssh_port !== undefined) {
      this._state.sshPort = data.ssh_port;
    }
    if (data.terminal_enabled !== undefined) {
      this._state.terminalEnabled = data.terminal_enabled;
    }
  }

  // 重置状态
  reset(): void {
    this._state = {
      sshEnabled: false,
      sshRunning: false,
      sshPort: 22,
      terminalEnabled: false,
      loading: false,
      loaded: false,
      error: null,
    };
  }
}

export const remoteAccessStore = new RemoteAccessStore();
