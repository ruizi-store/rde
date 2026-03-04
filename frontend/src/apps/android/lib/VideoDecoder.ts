/**
 * Scrcpy Video Decoder
 *
 * 使用 jmuxer 将 H.264 NAL 单元解码并渲染到 <video> 元素。
 * jmuxer 内部将 H.264 封装为 fMP4 并通过 MSE 喂给 <video>。
 */

import type JMuxer from "jmuxer";

export interface VideoDecoderOptions {
  /** 目标帧率，默认 60 */
  fps?: number;
  /** 是否开启 jmuxer 调试日志 */
  debug?: boolean;
}

export class VideoDecoder {
  private jmuxer: JMuxer | null = null;
  private videoElement: HTMLVideoElement | null = null;
  private options: Required<VideoDecoderOptions>;
  private initialized = false;

  constructor(options: VideoDecoderOptions = {}) {
    this.options = {
      fps: options.fps ?? 60,
      debug: options.debug ?? false,
    };
  }

  /**
   * 绑定 <video> 元素并初始化 jmuxer。
   * 使用动态 import 以支持 SSR 环境。
   */
  async init(videoElement: HTMLVideoElement): Promise<void> {
    this.videoElement = videoElement;

    const JMuxer = (await import("jmuxer")).default;

    this.jmuxer = new JMuxer({
      node: videoElement,
      mode: "video",
      flushingTime: 0,
      fps: this.options.fps,
      debug: this.options.debug,
    });

    this.initialized = true;
  }

  /**
   * 喂入 H.264 NAL 数据（从 WebSocket 二进制消息中提取的裸载荷）。
   */
  feed(data: Uint8Array): void {
    if (!this.initialized || !this.jmuxer) return;
    this.jmuxer.feed({ video: data });
  }

  /** 是否已初始化 */
  get ready(): boolean {
    return this.initialized;
  }

  /** 销毁，释放资源 */
  destroy(): void {
    if (this.jmuxer) {
      this.jmuxer.destroy();
      this.jmuxer = null;
    }
    this.videoElement = null;
    this.initialized = false;
  }
}
