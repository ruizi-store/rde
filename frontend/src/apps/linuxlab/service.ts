/**
 * Linux Lab API 服务（容器模式）
 */

import { api } from "$shared/services/api";
import type { Board, LabStatus, BuildStatus, BuildTarget, ProgressEvent } from "./types";

const BASE = "/linuxlab";

/** 获取环境状态 */
export async function getStatus(): Promise<LabStatus> {
  return api.get<LabStatus>(`${BASE}/status`);
}

/** SSE 流式请求辅助函数 */
function sseRequest(
  url: string,
  method: "POST" | "GET",
  body: unknown | null,
  onProgress: (event: ProgressEvent) => void,
  onDone: () => void,
): AbortController {
  const controller = new AbortController();
  const token = localStorage.getItem("auth_token");

  const init: RequestInit = {
    method,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    signal: controller.signal,
  };
  if (body !== null) init.body = JSON.stringify(body);

  fetch(`/api/v1${url}`, init)
    .then((response) => {
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const reader = response.body?.getReader();
      if (!reader) return;

      const decoder = new TextDecoder();
      let buffer = "";

      function read() {
        reader!.read().then(({ done, value }) => {
          if (done) { onDone(); return; }
          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split("\n");
          buffer = lines.pop() || "";
          for (const line of lines) {
            if (line.startsWith("data: ")) {
              try {
                const data = JSON.parse(line.slice(6));
                if (data.status === "done") { onDone(); return; }
                onProgress(data as ProgressEvent);
              } catch { /* ignore */ }
            }
          }
          read();
        });
      }
      read();
    })
    .catch((err) => {
      if (err.name !== "AbortError") {
        onProgress({ status: "failed", message: `连接失败: ${err.message}` });
        onDone();
      }
    });

  return controller;
}

/**
 * 初始化环境（SSE 流式拉取镜像+创建容器）
 */
export function setup(
  onProgress: (event: ProgressEvent) => void,
  onDone: () => void,
): AbortController {
  return sseRequest(`${BASE}/setup`, "POST", null, onProgress, onDone);
}

/** 列出所有开发板 */
export async function listBoards(): Promise<Board[]> {
  return api.get<Board[]>(`${BASE}/boards`);
}

/** 获取开发板详情 */
export async function getBoardDetail(boardPath: string): Promise<Board> {
  return api.get<Board>(`${BASE}/boards/${boardPath}`);
}

/** 切换当前开发板 */
export async function switchBoard(board: string): Promise<void> {
  await api.post(`${BASE}/boards/switch`, { board });
}

/** 获取构建状态 */
export async function getBuildStatus(): Promise<BuildStatus> {
  return api.get<BuildStatus>(`${BASE}/build/status`);
}

/** 停止虚拟开发板 */
export async function stopBoot(): Promise<void> {
  await api.delete(`${BASE}/boot`);
}

/** 触发构建（SSE 流式输出） */
export function startBuild(
  board: string,
  target: BuildTarget,
  onProgress: (event: ProgressEvent) => void,
  onDone: () => void,
): AbortController {
  return sseRequest(`${BASE}/build`, "POST", { board, target }, onProgress, onDone);
}

/** 启动虚拟开发板（SSE 流式输出） */
export function startBoot(
  board: string,
  onProgress: (event: ProgressEvent) => void,
  onDone: () => void,
): AbortController {
  return sseRequest(`${BASE}/boot`, "POST", { board }, onProgress, onDone);
}

/** 执行任意 make 目标（SSE 流式输出） */
export function execMake(
  board: string,
  target: string,
  onProgress: (event: ProgressEvent) => void,
  onDone: () => void,
): AbortController {
  return sseRequest(`${BASE}/make`, "POST", { board, target }, onProgress, onDone);
}
