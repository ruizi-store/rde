/**
 * Scrcpy Audio Player
 *
 * 使用 MediaSource Extensions (MSE) + ADTS 封装播放 AAC 音频流。
 * - 收到 AudioSpecificConfig 后解析 AAC 参数
 * - 每帧裸 AAC 数据前加 7 字节 ADTS 头后喂入 SourceBuffer
 * - 自动追播（延迟 > 1s 时跳到最新位置）
 * - 缓冲管理（保留最近 5 秒）
 */

export type AudioStatus =
  | "idle"
  | "waiting"
  | "configured"
  | "playing"
  | "no-mse"
  | "unsupported"
  | "error";

export interface AudioPlayerEvents {
  onStatusChange?: (status: AudioStatus) => void;
}

/** AAC 采样率索引表 */
const AAC_SAMPLE_RATES: Record<number, number> = {
  0: 96000,
  1: 88200,
  2: 64000,
  3: 48000,
  4: 44100,
  5: 32000,
  6: 24000,
  7: 22050,
  8: 16000,
  9: 12000,
  10: 11025,
  11: 8000,
  12: 7350,
};

export class AudioPlayer {
  private audioElement: HTMLAudioElement | null = null;
  private mediaSource: MediaSource | null = null;
  private sourceBuffer: SourceBuffer | null = null;
  private queue: Uint8Array[] = [];
  private sourceReady = false;

  // AAC 参数（从 AudioSpecificConfig 解析）
  private aacProfile = 2; // AAC-LC
  private aacFreqIndex = 3; // 48000 Hz
  private aacChannels = 2; // stereo
  private configured = false;

  private _status: AudioStatus = "idle";
  private events: AudioPlayerEvents;

  constructor(events: AudioPlayerEvents = {}) {
    this.events = events;
  }

  private setStatus(s: AudioStatus) {
    this._status = s;
    this.events.onStatusChange?.(s);
  }

  get status(): AudioStatus {
    return this._status;
  }

  /**
   * 初始化 MSE 音频管道。
   * 必须在用户交互后调用以满足 autoplay 策略。
   */
  init(): void {
    this.cleanup();

    if (typeof MediaSource === "undefined") {
      this.setStatus("no-mse");
      return;
    }

    const mimeType = "audio/aac";
    if (!MediaSource.isTypeSupported(mimeType)) {
      this.setStatus("unsupported");
      return;
    }

    this.mediaSource = new MediaSource();
    this.audioElement = new Audio();
    this.audioElement.src = URL.createObjectURL(this.mediaSource);

    this.mediaSource.addEventListener("sourceopen", () => {
      try {
        this.sourceBuffer = this.mediaSource!.addSourceBuffer(mimeType);
        this.sourceBuffer.mode = "sequence";
        this.sourceBuffer.addEventListener("updateend", () => {
          this.flushQueue();
          this.trimBuffer();
        });
        this.sourceReady = true;
        this.audioElement!.play().catch(() => {});
        this.setStatus("waiting");
      } catch {
        this.setStatus("error");
      }
    });
  }

  /**
   * 解析 AAC AudioSpecificConfig（2 字节）。
   * 由 ScrcpyClient 在收到 type=4 消息时调用。
   */
  setConfig(config: Uint8Array): void {
    if (config.length < 2) return;

    const b0 = config[0];
    const b1 = config[1];
    this.aacProfile = (b0 >> 3) & 0x1f;
    this.aacFreqIndex = ((b0 & 0x07) << 1) | ((b1 >> 7) & 0x01);
    this.aacChannels = (b1 >> 3) & 0x0f;
    this.configured = true;

    const sampleRate = AAC_SAMPLE_RATES[this.aacFreqIndex] ?? 48000;
    console.log(
      `[AudioPlayer] AAC config: profile=${this.aacProfile}, freq=${sampleRate}Hz, channels=${this.aacChannels}`,
    );
    this.setStatus("configured");
  }

  /**
   * 喂入裸 AAC 帧数据。
   * 内部自动添加 ADTS 头并通过 MSE 播放。
   */
  feed(aacFrame: Uint8Array): void {
    if (!this.sourceReady || !this.configured) return;

    const header = this.createADTSHeader(aacFrame.length);
    const adtsFrame = new Uint8Array(header.length + aacFrame.length);
    adtsFrame.set(header);
    adtsFrame.set(aacFrame, header.length);

    this.queue.push(adtsFrame);
    this.flushQueue();

    // 确保播放没有卡住
    if (this.audioElement?.paused) {
      this.audioElement.play().catch(() => {});
    }

    // 延迟追赶：落后 > 1 秒时跳到最新
    if (this.audioElement && this.sourceBuffer && this.sourceBuffer.buffered.length > 0) {
      const end = this.sourceBuffer.buffered.end(this.sourceBuffer.buffered.length - 1);
      if (end - this.audioElement.currentTime > 1.0) {
        this.audioElement.currentTime = end - 0.1;
      }
    }

    this.setStatus("playing");
  }

  /** 暂停播放 */
  pause(): void {
    this.audioElement?.pause();
  }

  /** 恢复播放 */
  resume(): void {
    this.audioElement?.play().catch(() => {});
  }

  /** 销毁，释放所有资源 */
  destroy(): void {
    this.cleanup();
    this.setStatus("idle");
  }

  // ============ 私有方法 ============

  private cleanup(): void {
    if (this.audioElement) {
      this.audioElement.pause();
      this.audioElement.src = "";
      this.audioElement = null;
    }
    if (this.mediaSource?.readyState === "open") {
      try {
        this.mediaSource.endOfStream();
      } catch {}
    }
    this.mediaSource = null;
    this.sourceBuffer = null;
    this.queue = [];
    this.sourceReady = false;
    this.configured = false;
  }

  /** 合并队列中所有帧并写入 SourceBuffer */
  private flushQueue(): void {
    if (!this.sourceBuffer || this.sourceBuffer.updating || this.queue.length === 0) return;

    const totalLen = this.queue.reduce((sum, f) => sum + f.length, 0);
    const combined = new Uint8Array(totalLen);
    let offset = 0;
    for (const frame of this.queue) {
      combined.set(frame, offset);
      offset += frame.length;
    }
    this.queue = [];

    try {
      this.sourceBuffer.appendBuffer(combined);
    } catch (e: unknown) {
      if (e instanceof DOMException && e.name === "QuotaExceededError") {
        // 缓冲满，等清理后重试
      }
    }
  }

  /** 保留最近 5 秒的缓冲区 */
  private trimBuffer(): void {
    if (!this.sourceBuffer || this.sourceBuffer.updating) return;
    try {
      if (this.sourceBuffer.buffered.length > 0) {
        const end = this.sourceBuffer.buffered.end(this.sourceBuffer.buffered.length - 1);
        if (end > 10) {
          this.sourceBuffer.remove(0, end - 5);
        }
      }
    } catch {}
  }

  /** 构造 7 字节 ADTS 头（无 CRC） */
  private createADTSHeader(aacFrameLen: number): Uint8Array {
    const totalLen = aacFrameLen + 7;
    const h = new Uint8Array(7);
    h[0] = 0xff;
    h[1] = 0xf1; // sync + MPEG-4 + Layer 0 + no CRC
    h[2] = ((this.aacProfile - 1) << 6) | (this.aacFreqIndex << 2) | ((this.aacChannels >> 2) & 1);
    h[3] = ((this.aacChannels & 3) << 6) | ((totalLen >> 11) & 3);
    h[4] = (totalLen >> 3) & 0xff;
    h[5] = ((totalLen & 7) << 5) | 0x1f;
    h[6] = 0xfc;
    return h;
  }
}
