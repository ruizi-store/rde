/**
 * Scrcpy 模块 - 核心协调器 + 导出
 *
 * ScrcpyClient 管理单个 WebSocket 连接，负责：
 * - 二进制消息解析与路由（前缀字节 2=视频, 3=音频, 4=音频配置）
 * - JSON 消息解析（device_info / error）
 * - 控制消息发送（touch / key / scroll / 导航按钮）
 * - 协调 VideoDecoder、AudioPlayer、InputHandler 的生命周期
 */

export { VideoDecoder, type VideoDecoderOptions } from "./VideoDecoder";
export { AudioPlayer, type AudioStatus, type AudioPlayerEvents } from "./AudioPlayer";
export {
  InputHandler,
  type ControlMessage,
  type TouchData,
  type KeyData,
  type ScrollData,
  type InputHandlerCallbacks,
} from "./InputHandler";

import { VideoDecoder, type VideoDecoderOptions } from "./VideoDecoder";
import { AudioPlayer, type AudioStatus } from "./AudioPlayer";
import { InputHandler, type ControlMessage } from "./InputHandler";

// ==================== 类型定义 ====================

/** WebSocket 二进制消息前缀字节 */
const MSG_VIDEO = 2;
const MSG_AUDIO = 3;
const MSG_AUDIO_CONFIG = 4;

/** 设备会话信息（从后端 device_info 消息获取） */
export interface ScrcpySession {
  id: string;
  deviceSerial: string;
  width: number;
  height: number;
  bitrate: number;
  maxFps: number;
  videoCodec: string;
  audioEnabled: boolean;
  status: "starting" | "running" | "stopped" | "error";
  startedAt?: string;
}

/** 投屏配置 */
export interface ScrcpyConfig {
  maxSize?: number;
  bitrate?: number;
  maxFps?: number;
  audioEnabled?: boolean;
  showTouches?: boolean;
  videoCodec?: string;
  audioCodec?: string;
}

/** ScrcpyClient 事件回调 */
export interface ScrcpyClientEvents {
  onConnected?: () => void;
  onDisconnected?: () => void;
  onSession?: (session: ScrcpySession) => void;
  onError?: (error: string) => void;
  onAudioStatus?: (status: AudioStatus) => void;
}

// ==================== ScrcpyClient ====================

export class ScrcpyClient {
  private ws: WebSocket | null = null;
  private videoDecoder: VideoDecoder;
  private audioPlayer: AudioPlayer;
  private inputHandler: InputHandler;
  private events: ScrcpyClientEvents;
  private _connected = false;
  private _session: ScrcpySession | null = null;

  constructor(events: ScrcpyClientEvents = {}) {
    this.events = events;

    this.videoDecoder = new VideoDecoder({
      fps: 60,
    });

    this.audioPlayer = new AudioPlayer({
      onStatusChange: (status) => events.onAudioStatus?.(status),
    });

    this.inputHandler = new InputHandler();
  }

  // ==================== 公开 API ====================

  get connected(): boolean {
    return this._connected;
  }

  get session(): ScrcpySession | null {
    return this._session;
  }

  get video(): VideoDecoder {
    return this.videoDecoder;
  }

  get audio(): AudioPlayer {
    return this.audioPlayer;
  }

  get input(): InputHandler {
    return this.inputHandler;
  }

  /**
   * 连接 WebSocket 并初始化视频/音频管道。
   * @param wsUrl WebSocket 地址，如 ws://host/ws/screen
   * @param videoElement 渲染视频的 <video> 元素
   * @param inputElement 捕获输入事件的元素（通常是视频容器）
   * @param audioEnabled 是否启用音频
   */
  async connect(
    wsUrl: string,
    videoElement: HTMLVideoElement,
    inputElement: HTMLElement,
    audioEnabled = true,
  ): Promise<void> {
    this.disconnect();

    // 初始化视频解码器
    await this.videoDecoder.init(videoElement);

    // 初始化音频
    if (audioEnabled) {
      this.audioPlayer.init();
    }

    // 绑定输入
    this.inputHandler.bind(inputElement, {
      onControl: (msg) => this.sendControl(msg),
    });

    // 建立 WebSocket
    this.ws = new WebSocket(wsUrl);
    this.ws.binaryType = "arraybuffer";

    this.ws.onopen = () => {
      this._connected = true;
      this.events.onConnected?.();
    };

    this.ws.onmessage = (event) => {
      if (event.data instanceof ArrayBuffer) {
        this.handleBinary(new Uint8Array(event.data));
      } else {
        try {
          this.handleJson(JSON.parse(event.data));
        } catch {
          console.warn("[ScrcpyClient] Invalid JSON message");
        }
      }
    };

    this.ws.onerror = () => {
      this.events.onError?.("WebSocket 连接错误");
    };

    this.ws.onclose = () => {
      this._connected = false;
      this.events.onDisconnected?.();
    };
  }

  /**
   * 发送控制消息到后端。
   */
  sendControl(msg: ControlMessage): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
    this.ws.send(JSON.stringify(msg));
  }

  /**
   * 断开连接，释放所有资源。
   */
  disconnect(): void {
    if (this.ws) {
      this.ws.onopen = null;
      this.ws.onmessage = null;
      this.ws.onerror = null;
      this.ws.onclose = null;
      if (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING) {
        this.ws.close();
      }
      this.ws = null;
    }

    this.inputHandler.unbind();
    this.audioPlayer.destroy();
    this.videoDecoder.destroy();

    this._connected = false;
    this._session = null;
  }

  /**
   * 完全销毁，释放所有引用。
   */
  destroy(): void {
    this.disconnect();
    this.inputHandler.destroy();
  }

  // ==================== 内部消息处理 ====================

  private handleBinary(data: Uint8Array): void {
    if (data.length < 2) return;

    const type = data[0];
    const payload = data.subarray(1);

    switch (type) {
      case MSG_VIDEO:
        this.videoDecoder.feed(payload);
        break;
      case MSG_AUDIO:
        this.audioPlayer.feed(payload);
        break;
      case MSG_AUDIO_CONFIG:
        this.audioPlayer.setConfig(payload);
        break;
    }
  }

  private handleJson(msg: { type: string; payload?: unknown; error?: string }): void {
    switch (msg.type) {
      case "device_info":
        this._session = msg.payload as ScrcpySession;
        this.events.onSession?.(this._session);
        break;
      case "error":
        this.events.onError?.(msg.error ?? "未知错误");
        break;
    }
  }
}
