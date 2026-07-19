// 系统关机 / 重启：全屏遮罩 + 健康探测，避免前端「假活」

export type SystemPowerMode = "reboot" | "shutdown";

const STORAGE_KEY = "rde_power_pending";
const POLL_MS = 2500;
const PROBE_TIMEOUT_MS = 3500;

async function probeAlive(): Promise<boolean> {
  const ctrl = new AbortController();
  const timer = window.setTimeout(() => ctrl.abort(), PROBE_TIMEOUT_MS);
  try {
    const res = await fetch("/health", {
      method: "GET",
      cache: "no-store",
      signal: ctrl.signal,
    });
    if (res.ok) return true;
  } catch {
    /* try API fallback */
  } finally {
    clearTimeout(timer);
  }

  const ctrl2 = new AbortController();
  const timer2 = window.setTimeout(() => ctrl2.abort(), PROBE_TIMEOUT_MS);
  try {
    const res = await fetch("/api/v1/setup/status", {
      method: "GET",
      cache: "no-store",
      signal: ctrl2.signal,
    });
    // 能拿到 HTTP 响应即认为后端在线（含 401）
    return res.status > 0 && res.status < 500;
  } catch {
    return false;
  } finally {
    clearTimeout(timer2);
  }
}

class SystemPowerStore {
  mode = $state<SystemPowerMode | null>(null);
  /** waiting: 已发起；offline: 已确认离线；coming_back: 重启后服务恢复中 */
  phase = $state<"waiting" | "offline" | "coming_back">("waiting");

  private pollId: ReturnType<typeof setInterval> | null = null;
  private sawOffline = false;

  get active(): boolean {
    return this.mode !== null;
  }

  /** 立刻进入遮罩态（应在调用关机/重启 API 之前或紧随其后） */
  begin(mode: SystemPowerMode): void {
    if (typeof window === "undefined") return;
    this.stopPolling();
    this.mode = mode;
    this.phase = "waiting";
    this.sawOffline = false;
    try {
      sessionStorage.setItem(STORAGE_KEY, mode);
    } catch {
      /* ignore */
    }
    this.startPolling();
  }

  /** 页面加载时恢复（重启过程中用户刷新） */
  async restore(): Promise<void> {
    if (typeof window === "undefined") return;
    let pending: string | null = null;
    try {
      pending = sessionStorage.getItem(STORAGE_KEY);
    } catch {
      return;
    }
    if (pending !== "reboot" && pending !== "shutdown") return;

    const alive = await probeAlive();
    if (alive) {
      try {
        sessionStorage.removeItem(STORAGE_KEY);
      } catch {
        /* ignore */
      }
      return;
    }

    this.mode = pending;
    this.phase = "offline";
    this.sawOffline = true;
    if (pending === "shutdown") {
      return;
    }
    this.startPolling();
  }

  clear(): void {
    this.stopPolling();
    this.mode = null;
    this.phase = "waiting";
    this.sawOffline = false;
    try {
      sessionStorage.removeItem(STORAGE_KEY);
    } catch {
      /* ignore */
    }
  }

  /** 主机是否仍可达（用于区分「权限失败」与「已开始重启断连」） */
  async isHostReachable(): Promise<boolean> {
    return probeAlive();
  }

  private startPolling(): void {
    this.stopPolling();
    void this.tick();
    this.pollId = setInterval(() => void this.tick(), POLL_MS);
  }

  private stopPolling(): void {
    if (this.pollId !== null) {
      clearInterval(this.pollId);
      this.pollId = null;
    }
  }

  private async tick(): Promise<void> {
    if (!this.mode) return;
    const alive = await probeAlive();

    if (this.mode === "shutdown") {
      if (!alive) {
        this.phase = "offline";
        this.sawOffline = true;
        // 关机后无需持续高频探测
        this.stopPolling();
      }
      return;
    }

    // reboot
    if (!alive) {
      this.sawOffline = true;
      this.phase = "offline";
      return;
    }
    if (this.sawOffline) {
      this.phase = "coming_back";
      this.clear();
      window.location.reload();
      return;
    }
    // 仍在线：可能 reboot 尚未生效，继续 waiting
    this.phase = "waiting";
  }
}

export const systemPower = new SystemPowerStore();
