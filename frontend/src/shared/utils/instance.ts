/**
 * 安装实例对账：同 origin（同 IP）换机/重装时清空浏览器本地状态。
 */

export const INSTANCE_STORAGE_KEY = "rde_instance_id";

let cachedInstanceId: string | null = null;
let reconcilePromise: Promise<ReconcileResult> | null = null;

export type ReconcileResult = {
  instanceId: string;
  cleared: boolean;
};

/** 当前已知的实例 ID（对账完成后可用） */
export function getInstanceId(): string | null {
  if (typeof window === "undefined") return null;
  if (cachedInstanceId) return cachedInstanceId;
  try {
    return localStorage.getItem(INSTANCE_STORAGE_KEY);
  } catch {
    return null;
  }
}

function hasLocalSessionResidue(): boolean {
  try {
    const keys = [
      "auth_token",
      "refresh_token",
      "rde_device_token",
      "rde_desktop",
      "rde_wallpaper",
      "rde_apps_config",
    ];
    for (const k of keys) {
      if (localStorage.getItem(k)) return true;
    }
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key && (key.startsWith("rde:") || key.startsWith("rde_"))) return true;
    }
  } catch {
    /* ignore */
  }
  return false;
}

/** 清空本站点本地存储（保留将在随后写入的 instance id） */
export function clearSiteLocalState(): void {
  if (typeof window === "undefined") return;
  try {
    localStorage.clear();
  } catch {
    /* ignore */
  }
  try {
    sessionStorage.clear();
  } catch {
    /* ignore */
  }
  cachedInstanceId = null;
}

/** 用户手动清除本地数据并刷新 */
export function clearLocalDataAndReload(): void {
  clearSiteLocalState();
  window.location.href = "/login";
}

/**
 * 与后端 /api/v1/system/instance 对账。
 * instance 不一致或「有会话残留但无 instance」时清空本地存储。
 */
export async function reconcileInstance(): Promise<ReconcileResult> {
  if (typeof window === "undefined") {
    return { instanceId: "", cleared: false };
  }
  if (reconcilePromise) return reconcilePromise;

  reconcilePromise = (async () => {
    const res = await fetch("/api/v1/system/instance", {
      method: "GET",
      cache: "no-store",
      headers: { Accept: "application/json" },
    });
    if (!res.ok) {
      throw new Error(`instance api HTTP ${res.status}`);
    }
    const json = await res.json();
    const serverId: string =
      json?.data?.instance_id || json?.data?.instanceId || "";
    if (!serverId) {
      throw new Error("instance api missing instance_id");
    }

    const headerId = res.headers.get("X-RDE-Instance-Id");
    if (headerId && headerId !== serverId) {
      console.warn("[instance] header/body mismatch", headerId, serverId);
    }

    const localId = localStorage.getItem(INSTANCE_STORAGE_KEY);
    let cleared = false;

    if (localId && localId !== serverId) {
      console.info("[instance] mismatch, clearing site data", { localId, serverId });
      clearSiteLocalState();
      cleared = true;
    } else if (!localId && hasLocalSessionResidue()) {
      console.info("[instance] legacy session without instance id, clearing");
      clearSiteLocalState();
      cleared = true;
    }

    localStorage.setItem(INSTANCE_STORAGE_KEY, serverId);
    cachedInstanceId = serverId;
    return { instanceId: serverId, cleared };
  })();

  try {
    return await reconcilePromise;
  } catch (e) {
    reconcilePromise = null;
    throw e;
  }
}
