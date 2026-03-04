/**
 * Scrcpy Service - 业务层封装
 *
 * 提供投屏会话的完整生命周期管理：
 * - 通过 REST API 启停后端 scrcpy 会话
 * - 通过 ScrcpyClient 管理 WebSocket 流（视频/音频/输入）
 */

import {
  ScrcpyClient,
  type ScrcpySession,
  type ScrcpyConfig,
  type ScrcpyClientEvents,
  type AudioStatus,
} from "./index";

// ==================== 类型定义 ====================

export type { ScrcpySession, ScrcpyConfig, AudioStatus };

export interface ScrcpyServiceEvents {
  onConnected?: () => void;
  onDisconnected?: () => void;
  onSession?: (session: ScrcpySession) => void;
  onError?: (error: string) => void;
  onAudioStatus?: (status: AudioStatus) => void;
}

// ==================== 服务实现 ====================

class ScrcpyService {
  private client: ScrcpyClient | null = null;
  private apiBase = "/api";

  /**
   * 设置 API 基础路径（用于通过 Core 代理访问 pkg-android 后端）。
   */
  setApiBase(base: string): void {
    this.apiBase = base;
  }

  /**
   * 开始投屏会话。
   * 1. POST 启动后端 scrcpy 会话
   * 2. 建立 WebSocket 连接
   * 3. 开始接收视频/音频流
   */
  async startSession(
    deviceSerial: string,
    config: ScrcpyConfig,
    videoElement: HTMLVideoElement,
    inputElement: HTMLElement,
    events: ScrcpyServiceEvents = {},
  ): Promise<ScrcpySession | null> {
    // 先关闭已有会话
    await this.stopSession();

    // 启动后端会话
    const res = await fetch(`${this.apiBase}/session/start`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        serial: deviceSerial,
        config: {
          maxSize: config.maxSize ?? 1280,
          bitrate: config.bitrate ?? 8000000,
          maxFps: config.maxFps ?? 60,
          audioEnabled: config.audioEnabled ?? true,
          showTouches: config.showTouches ?? false,
        },
      }),
    });

    const data = await res.json();
    if (!data.success) {
      throw new Error(data.error || "启动投屏失败");
    }

    const session = data.data as ScrcpySession;

    // 构建 WebSocket URL
    const wsUrl = this.buildWsUrl();

    // 创建 ScrcpyClient 并连接
    const clientEvents: ScrcpyClientEvents = {
      onConnected: events.onConnected,
      onDisconnected: events.onDisconnected,
      onSession: events.onSession,
      onError: events.onError,
      onAudioStatus: events.onAudioStatus,
    };

    this.client = new ScrcpyClient(clientEvents);
    await this.client.connect(wsUrl, videoElement, inputElement, config.audioEnabled ?? true);

    return session;
  }

  /**
   * 停止投屏会话。
   */
  async stopSession(): Promise<void> {
    if (this.client) {
      this.client.disconnect();
      this.client = null;
    }

    try {
      await fetch(`${this.apiBase}/session/stop`, { method: "POST" });
    } catch {
      // 忽略停止失败
    }
  }

  /**
   * 获取底层 ScrcpyClient 实例（用于直接访问 input/video/audio 控制）。
   */
  getClient(): ScrcpyClient | null {
    return this.client;
  }

  /**
   * 销毁服务，释放所有资源。
   */
  destroy(): void {
    if (this.client) {
      this.client.destroy();
      this.client = null;
    }
  }

  // ============ 私有方法 ============

  private buildWsUrl(): string {
    if (typeof window === "undefined") return "ws://localhost/ws/screen";

    const protocol = location.protocol === "https:" ? "wss:" : "ws:";
    return `${protocol}//${location.host}/api/v1/android/ws/screen`;
  }
}

// 导出单例
export const scrcpyService = new ScrcpyService();
