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

// 渠道绑定（与后端 modules/bot/channels.go 对齐）
export interface BotChannel {
  platform: "telegram" | "wecom" | "webhook";
  chat_id?: string;
  user_id?: string;
}

// 技能元数据（SKILL.md frontmatter）
export interface SkillMeta {
  name: string;
  description: string;
}

// 记忆条目（与后端 modules/bot/memory.go 对齐）
export interface BotMemory {
  id: number;
  bot_id: string;
  key: string;
  content: string;
  created_at: string;
  updated_at: string;
}

// 定时日程（与后端 modules/bot/schedule.go 对齐）
export interface BotSchedule {
  id?: number;
  bot_id?: string;
  name: string;
  cron: string;
  prompt: string;
  enabled: boolean;
  last_run_at?: string;
  last_result?: string;
  last_error?: string;
}

// 审批记录（与后端 modules/bot/approvals.go 对齐）
export interface BotApproval {
  id: number;
  bot_id: string;
  action: string;
  reason: string;
  status: "pending" | "approved" | "denied" | "expired";
  requested_by: string;
  decided_by: string;
  decided_at?: string;
  created_at: string;
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

// 渠道绑定（P3：网关消息按渠道路由到 Bot）
export async function getBotChannels(id: string): Promise<BotChannel[]> {
  return api.get<BotChannel[]>(`/bots/${id}/channels`);
}
export async function setBotChannels(id: string, channels: BotChannel[]): Promise<void> {
  await api.put(`/bots/${id}/channels`, { channels });
}

// 技能文件化（P3：SKILL.md 技能清单）
export async function listSkills(): Promise<SkillMeta[]> {
  return api.get<SkillMeta[]>("/bots/skills");
}
export async function reloadSkills(): Promise<SkillMeta[]> {
  return api.post<SkillMeta[]>("/bots/skills/reload", {});
}

// 记忆（P4：长期记忆审计）
export async function listMemories(id: string, q = ""): Promise<BotMemory[]> {
  return api.get<BotMemory[]>(`/bots/${id}/memories${q ? `?q=${encodeURIComponent(q)}` : ""}`);
}
export async function deleteMemory(id: string, memoryId: number): Promise<void> {
  await api.delete(`/bots/${id}/memories/${memoryId}`);
}

// 定时日程（P4：主动汇报）
export async function listSchedules(id: string): Promise<BotSchedule[]> {
  return api.get<BotSchedule[]>(`/bots/${id}/schedules`);
}
export async function createSchedule(id: string, req: Omit<BotSchedule, "id" | "bot_id">): Promise<BotSchedule> {
  return api.post<BotSchedule>(`/bots/${id}/schedules`, req);
}
export async function updateSchedule(
  id: string,
  sid: number,
  updates: Partial<{ name: string; cron: string; prompt: string; enabled: boolean }>,
): Promise<BotSchedule> {
  return api.put<BotSchedule>(`/bots/${id}/schedules/${sid}`, updates);
}
export async function deleteSchedule(id: string, sid: number): Promise<void> {
  await api.delete(`/bots/${id}/schedules/${sid}`);
}
export async function runScheduleNow(id: string, sid: number): Promise<string> {
  return api.post<string>(`/bots/${id}/schedules/${sid}/run`, {});
}

// 审批（P4：高风险操作人工确认）
export async function listApprovals(id: string, status = ""): Promise<BotApproval[]> {
  return api.get<BotApproval[]>(`/bots/${id}/approvals${status ? `?status=${status}` : ""}`);
}
export async function decideApproval(id: string, aid: number, approve: boolean): Promise<string> {
  return api.post<string>(`/bots/${id}/approvals/${aid}/decide`, { approve });
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
