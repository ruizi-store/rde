// Bot（数字员工）服务
import { api } from "./api";
import { getAuthItem } from "$shared/utils/auth-storage";

// Bot 定义（与后端 modules/bot/model.go 对齐）
export interface Bot {
  id: string;
  name: string;
  description: string;
  model: string;
  provider_id: string;
  image: string;
  channels: string;
  status: "created" | "starting" | "running" | "stopped" | "error";
  container_id: string;
  error_message: string;
  enabled: boolean;
  taken_over_by: string;
  taken_over_at?: string;
  taken_over_by_name?: string;
  created_at: string;
  updated_at: string;
  members?: BotMember[];
  my_role?: "admin" | "member" | "viewer" | "";
}

export interface BotMember {
  id?: number;
  bot_id?: string;
  user_id: string;
  role: "admin" | "member" | "viewer";
}

export interface ExecResult {
  exit_code: number;
  output: string;
  truncated: boolean;
}

export interface ExecRequest {
  script: string;
  language: "bash" | "python";
  timeout?: number;
}

// 列出可见的 bot
export async function listBots(): Promise<Bot[]> {
  const res = await api.get<Bot[]>("/bots");
  return res;
}

// 创建 bot
export async function createBot(req: {
  name: string;
  description?: string;
  model?: string;
  provider_id?: string;
  image?: string;
}): Promise<Bot> {
  return api.post<Bot>("/bots", req);
}

// 详情
export async function getBot(id: string): Promise<Bot> {
  return api.get<Bot>(`/bots/${id}`);
}

// 更新
export async function updateBot(
  id: string,
  req: Partial<{ name: string; description: string; model: string; provider_id: string; enabled: boolean }>,
): Promise<Bot> {
  return api.put<Bot>(`/bots/${id}`, req);
}

// 删除（data=true 连带删除工作目录）
export async function deleteBot(id: string, data = false): Promise<void> {
  await api.delete(`/bots/${id}${data ? "?data=1" : ""}`);
}

// 沙箱生命周期
export async function startBot(id: string): Promise<Bot> {
  return api.post<Bot>(`/bots/${id}/start`, {});
}
export async function stopBot(id: string): Promise<Bot> {
  return api.post<Bot>(`/bots/${id}/stop`, {});
}
export async function restartBot(id: string): Promise<Bot> {
  return api.post<Bot>(`/bots/${id}/restart`, {});
}

// 沙箱日志
export async function getBotLogs(id: string): Promise<string> {
  return api.get<string>(`/bots/${id}/logs`);
}

// 沙箱执行脚本（工具调用）
export async function execInBot(id: string, req: ExecRequest): Promise<ExecResult> {
  return api.post<ExecResult>(`/bots/${id}/exec`, req);
}

// 成员管理
export async function getBotMembers(id: string): Promise<BotMember[]> {
  return api.get<BotMember[]>(`/bots/${id}/members`);
}
export async function setBotMembers(id: string, members: BotMember[]): Promise<void> {
  await api.put(`/bots/${id}/members`, { members });
}

// 接管 / 交还
export async function takeoverBot(id: string): Promise<void> {
  await api.post(`/bots/${id}/takeover`, {});
}
export async function releaseBot(id: string): Promise<void> {
  await api.post(`/bots/${id}/release`, {});
}

// 沙箱串流 URL（iframe 直接加载；token 走查询参数）
export function botStreamUrl(id: string): string {
  const token = getAuthItem("auth_token") || "";
  return `/api/v1/bots/${id}/vnc/?token=${encodeURIComponent(token)}`;
}
