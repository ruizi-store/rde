/**
 * 文件管理器跨窗口共享剪贴板与目录变更通知。
 * 多个「文件」窗口实例共用同一份 cut/copy 状态，并在移动/粘贴后互相刷新。
 */

export type FileClipboardOperation = "copy" | "cut";

export interface FileClipboardPayload {
  paths: string[];
  operation: FileClipboardOperation;
}

function parentDir(path: string): string {
  if (!path || path === "/") return "/";
  const normalized = path.endsWith("/") && path.length > 1 ? path.slice(0, -1) : path;
  const i = normalized.lastIndexOf("/");
  if (i <= 0) return "/";
  return normalized.slice(0, i) || "/";
}

class FileClipboardStore {
  clipboard = $state<FileClipboardPayload | null>(null);

  /** 变更序号：任意文件管理器实例可订阅后按需 refresh */
  revision = $state(0);

  /** 最近一次操作影响到的目录（需刷新的 currentPath 候选） */
  affectedDirs = $state<string[]>([]);

  set(paths: string[], operation: FileClipboardOperation) {
    this.clipboard = { paths: [...paths], operation };
  }

  clear() {
    this.clipboard = null;
  }

  /**
   * 通知各文件窗口：sources 已离开其父目录，destDir 有新内容。
   * cut/move 传 sources；纯 copy 可只传 destDir（sources 为空）。
   */
  notifyChanged(sources: string[], destDir: string) {
    const dirs = new Set<string>();
    if (destDir) dirs.add(destDir === "" ? "/" : destDir);
    for (const p of sources) {
      dirs.add(parentDir(p));
    }
    this.affectedDirs = [...dirs];
    this.revision += 1;
  }

  /** 当前目录是否应刷新 */
  shouldRefresh(currentPath: string): boolean {
    const cur = currentPath || "/";
    return this.affectedDirs.some((d) => d === cur);
  }
}

export const fileClipboard = new FileClipboardStore();
