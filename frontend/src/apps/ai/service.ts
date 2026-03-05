/**
 * AI Service - AI 助手
 *
 * 后端路由: /api/v1/ai/...
 * 支持多 Provider (Ollama/OpenAI/Claude/Deepseek 等)
 * 流式聊天使用 SSE (Server-Sent Events)
 */

import { api } from "$shared/services/api";

// ==================== 类型定义 ====================

export type ProviderType =
  | "ollama"
  | "openai"
  | "claude"
  | "gemini"
  | "deepseek"
  | "zhipu"
  | "qwen"
  | "moonshot"
  | "groq"
  | "openrouter";

export interface ProviderConfig {
  id: string;
  provider: ProviderType;
  name: string;
  base_url: string;
  api_key?: string;
  models?: string[];
  enabled: boolean;
}

export interface AIModel {
  id: string;
  name: string;
  provider: string;
  provider_id?: string;
  description?: string;
  context_length?: number;
  capabilities?: string[];
}

export interface Conversation {
  id: string;
  title: string;
  model_id: string;
  provider_id?: string;
  messages: ChatMessage[];
  created_at: string;
  updated_at: string;
}

export interface ChatMessage {
  id: string;
  role: "system" | "user" | "assistant";
  content: string;
  timestamp: string;
}

export interface ChatRequest {
  provider_id: string;
  model: string;
  messages: { role: string; content: string }[];
  conversation_id?: string;
  stream?: boolean;
  max_tokens?: number;
  temperature?: number;
  top_p?: number;
  system_prompt?: string;
}

export interface ChatResponse {
  id: string;
  conversation_id?: string;
  model: string;
  content: string;
  finish_reason?: string;
  usage?: { prompt_tokens: number; completion_tokens: number; total_tokens: number };
}

export interface StreamChunk {
  id: string;
  delta: string;
  finish_reason?: string;
  error?: string;
}

// ---- Config / Status ----

export interface AIConfig {
  enabled: boolean;
  default_provider: string;
  default_model: string;
  system_prompt: string;
  max_tokens: number;
  temperature: number;
  enable_tools: boolean;
}

export interface AIServiceStatus {
  enabled: boolean;
  providers: { id: string; provider: string; name: string; enabled: boolean; status: string; error?: string }[];
  conversation_count: number;
  default_provider: string;
  default_model: string;
  tools_enabled: boolean;
}

// ---- Setup ----

export type SetupStepType = "environment" | "model" | "skills" | "complete";

export interface SetupState {
  status: "pending" | "in_progress" | "complete";
  current_step: SetupStepType;
  selected_model: string;
  skills_enabled: string[];
  started_at: string;
  completed_at?: string;
}

export interface EnvironmentCheck {
  docker: ComponentStatus;
  disk_space: DiskSpaceStatus;
  network: NetworkStatus;
  gpu: ComponentStatus;
  os: string;
  arch: string;
}

export interface ComponentStatus {
  name: string;
  status: "ready" | "not_installed" | "not_available";
  version?: string;
  message?: string;
}

export interface DiskSpaceStatus {
  available: string;
  total?: string;
  required: string;
  sufficient: boolean;
}

export interface NetworkStatus {
  internet: boolean;
  docker_hub: boolean;
  ollama_reachable: boolean;
}

export interface ModelInfo {
  id: string;
  name: string;
  size: string;
  description: string;
  category?: string;
  min_ram?: string;
}

export interface DownloadProgress {
  model: string;
  status: string;
  percentage: number;
  log?: string;
  error?: string;
}

// ---- Skills ----

export interface Skill {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
  category?: string;
}

export interface SkillResponse {
  success: boolean;
  data?: unknown;
  error?: string;
  summary?: string;
}

// ---- Gateway ----

export interface GatewayConfig {
  enabled: boolean;
  wecom: {
    enabled: boolean;
    corp_id: string;
    agent_id: number;
    secret: string;
    token: string;
    encoding_key: string;
    callback_url: string;
  };
  telegram: {
    enabled: boolean;
    bot_token: string;
    webhook_url: string;
    use_webhook: boolean;
    proxy_mode: string;
    proxy_url: string;
  };
  webhook: {
    enabled: boolean;
    api_key: string;
  };
  security: {
    allowed_users: string[];
    require_confirmation: string[];
    daily_limit: number;
    require_pin: boolean;
    pin: string;
  };
}

export interface GatewayStatus {
  enabled: boolean;
  platforms: Record<string, { enabled: boolean }>;
  active_sessions: number;
}

// ---- Alerts ----

export interface AlertConfig {
  enabled: boolean;
  check_interval: number;
  disk_warning_pct: number;
  disk_critical_pct: number;
  cpu_warning_pct: number;
  memory_warning_pct: number;
  temp_warning_c: number;
  enabled_alerts: string[];
  notify_platforms: string[];
  notify_users: string[];
  quiet_hours_start: number;
  quiet_hours_end: number;
  cooldown_minutes: number;
}

export interface Alert {
  id: string;
  type: string;
  level: "info" | "warning" | "critical";
  title: string;
  message: string;
  source: string;
  value: number;
  threshold: number;
  timestamp: string;
  notified: boolean;
  resolved: boolean;
  resolved_at?: string;
}

export interface AlertStatus {
  running: boolean;
  last_check?: string;
  alert_count: number;
}

// ---- Voice ----

export interface VoiceConfig {
  stt_provider: string;
  tts_provider: string;
  stt_model: string;
  tts_model: string;
  tts_voice: string;
  openai_key?: string;
  azure_key?: string;
  azure_region?: string;
}

// ==================== 服务实现 ====================

class AIService {
  // ---- Provider ----

  async getProviders(): Promise<ProviderConfig[]> {
    return api.get<ProviderConfig[]>("/ai/providers");
  }

  async getProvider(id: string): Promise<ProviderConfig> {
    return api.get<ProviderConfig>(`/ai/providers/${id}`);
  }

  async createProvider(data: Partial<ProviderConfig>): Promise<ProviderConfig> {
    return api.post<ProviderConfig>("/ai/providers", data);
  }

  async updateProvider(id: string, data: Partial<ProviderConfig>): Promise<ProviderConfig> {
    return api.put<ProviderConfig>(`/ai/providers/${id}`, data);
  }

  async deleteProvider(id: string): Promise<void> {
    await api.delete(`/ai/providers/${id}`);
  }

  // ---- Models ----

  async getModels(providerId: string): Promise<AIModel[]> {
    return api.get<AIModel[]>(`/ai/providers/${providerId}/models`);
  }

  async pullOllamaModel(providerId: string, model: string): Promise<void> {
    await api.post(`/ai/providers/${providerId}/models/pull`, { model });
  }

  async deleteOllamaModel(providerId: string, model: string): Promise<void> {
    await api.delete(`/ai/providers/${providerId}/models`, { data: { model } } as any);
  }

  // ---- Conversations ----

  async getConversations(): Promise<Conversation[]> {
    return api.get<Conversation[]>("/ai/conversations");
  }

  async searchConversations(query: string): Promise<Conversation[]> {
    return api.get<Conversation[]>(`/ai/conversations/search?q=${encodeURIComponent(query)}`);
  }

  async getConversation(id: string): Promise<Conversation> {
    return api.get<Conversation>(`/ai/conversations/${id}`);
  }

  async createConversation(title: string, modelId: string): Promise<Conversation> {
    return api.post<Conversation>("/ai/conversations", { title, model_id: modelId });
  }

  async updateConversation(id: string, data: { title?: string }): Promise<Conversation> {
    return api.put<Conversation>(`/ai/conversations/${id}`, data);
  }

  async deleteConversation(id: string): Promise<void> {
    await api.delete(`/ai/conversations/${id}`);
  }

  async clearMessages(id: string): Promise<void> {
    await api.delete(`/ai/conversations/${id}/messages`);
  }

  // ---- Chat (Non-Streaming) ----

  async chat(req: ChatRequest): Promise<ChatResponse> {
    return api.post<ChatResponse>("/ai/chat", { ...req, stream: false });
  }

  // ---- Chat (SSE Streaming) ----

  async *streamChat(
    req: ChatRequest,
    signal?: AbortSignal,
  ): AsyncGenerator<StreamChunk, void, unknown> {
    const token = typeof window !== "undefined" ? localStorage.getItem("auth_token") : null;
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (token) headers["Authorization"] = `Bearer ${token}`;

    const response = await fetch("/api/v1/ai/chat/stream", {
      method: "POST",
      headers,
      body: JSON.stringify({ ...req, stream: true }),
      signal,
    });

    if (!response.ok) {
      const err = await response.text();
      throw new Error(err || `HTTP ${response.status}`);
    }

    const reader = response.body?.getReader();
    if (!reader) throw new Error("No readable stream");

    const decoder = new TextDecoder();
    let buffer = "";

    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";

        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed || trimmed === ":" || trimmed === "data: [DONE]") continue;
          if (trimmed.startsWith("data: ")) {
            try {
              const chunk = JSON.parse(trimmed.slice(6)) as StreamChunk;
              yield chunk;
            } catch {
              // Ignore malformed JSON
            }
          }
        }
      }
    } finally {
      reader.releaseLock();
    }
  }

  // ---- Save Messages (after streaming) ----

  async saveMessages(conversationId: string, data: { userContent: string; assistantContent: string; provider: string; model: string }): Promise<void> {
    await api.post(`/ai/conversations/${conversationId}/messages/save`, data);
  }

  // ---- Config / Status ----

  async getConfig(): Promise<AIConfig> {
    return api.get<AIConfig>("/ai/config");
  }

  async updateConfig(data: Partial<AIConfig>): Promise<AIConfig> {
    return api.put<AIConfig>("/ai/config", data);
  }

  async getStatus(): Promise<AIServiceStatus> {
    return api.get<AIServiceStatus>("/ai/status");
  }

  // ---- Setup ----

  async getSetupStatus(): Promise<SetupState> {
    return api.get<SetupState>("/ai/setup/status");
  }

  async checkEnvironment(): Promise<EnvironmentCheck> {
    return api.post<EnvironmentCheck>("/ai/setup/check-env");
  }

  async getAvailableModels(): Promise<ModelInfo[]> {
    return api.get<ModelInfo[]>("/ai/setup/models");
  }

  async *downloadModel(model: string, signal?: AbortSignal): AsyncGenerator<DownloadProgress> {
    const token = typeof window !== "undefined" ? localStorage.getItem("auth_token") : null;
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (token) headers["Authorization"] = `Bearer ${token}`;

    const resp = await fetch("/api/v1/ai/setup/models/download", {
      method: "POST",
      headers,
      body: JSON.stringify({ model }),
      signal,
    });
    if (!resp.ok) throw new Error(await resp.text());
    if (!resp.body) throw new Error("No response body");

    const reader = resp.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });

      let idx: number;
      while ((idx = buffer.indexOf("\n\n")) >= 0) {
        const chunk = buffer.slice(0, idx).trim();
        buffer = buffer.slice(idx + 2);
        if (!chunk.startsWith("data: ")) continue;
        try {
          const progress: DownloadProgress = JSON.parse(chunk.slice(6));
          yield progress;
          if (progress.error) return;
          if (progress.status === "completed" || progress.status === "error") return;
        } catch { /* skip malformed */ }
      }
    }
  }

  async getDefaultSkills(): Promise<Skill[]> {
    return api.get<Skill[]>("/ai/setup/skills");
  }

  async setSetupStep(step: SetupStepType): Promise<void> {
    await api.put("/ai/setup/step", { step });
  }

  async completeSetup(data: { selected_model: string; skills_enabled: string[] }): Promise<void> {
    await api.post("/ai/setup/complete", data);
  }

  async resetSetup(): Promise<void> {
    await api.post("/ai/setup/reset");
  }

  // ---- Skills ----

  async executeSkill(skillId: string, action: string, args?: Record<string, unknown>): Promise<SkillResponse> {
    return api.post<SkillResponse>("/ai/skills/execute", { skill_id: skillId, action, arguments: args });
  }

  async getStorageAnalysis(path = "/"): Promise<SkillResponse> {
    return api.get<SkillResponse>(`/ai/storage/analysis?path=${encodeURIComponent(path)}`);
  }

  async getSystemInfo(): Promise<SkillResponse> {
    return api.get<SkillResponse>("/ai/system/info");
  }

  async searchFiles(query: string, dir = "/"): Promise<SkillResponse> {
    return api.get<SkillResponse>(`/ai/files/search?q=${encodeURIComponent(query)}&dir=${encodeURIComponent(dir)}`);
  }

  // ---- Gateway ----

  async getGatewayConfig(): Promise<GatewayConfig> {
    return api.get<GatewayConfig>("/ai/gateway/config");
  }

  async updateGatewayConfig(config: GatewayConfig): Promise<void> {
    await api.put("/ai/gateway/config", config);
  }

  async getGatewayStatus(): Promise<GatewayStatus> {
    return api.get<GatewayStatus>("/ai/gateway/status");
  }

  async startGateway(): Promise<void> {
    await api.post("/ai/gateway/start");
  }

  async stopGateway(): Promise<void> {
    await api.post("/ai/gateway/stop");
  }

  async testTelegram(): Promise<{ message?: string; error?: string }> {
    return api.post("/ai/gateway/test-telegram");
  }

  // ---- Alerts ----

  async getAlertsConfig(): Promise<AlertConfig> {
    return api.get<AlertConfig>("/ai/alerts/config");
  }

  async updateAlertsConfig(config: AlertConfig): Promise<void> {
    await api.put("/ai/alerts/config", config);
  }

  async getAlertsStatus(): Promise<AlertStatus> {
    return api.get<AlertStatus>("/ai/alerts/status");
  }

  async getAlertsHistory(): Promise<Alert[]> {
    return api.get<Alert[]>("/ai/alerts/history");
  }

  async clearAlertsHistory(): Promise<void> {
    await api.delete("/ai/alerts/history");
  }

  async startAlerts(): Promise<void> {
    await api.post("/ai/alerts/start");
  }

  async stopAlerts(): Promise<void> {
    await api.post("/ai/alerts/stop");
  }

  // ---- Sessions ----

  async getSessionStats(): Promise<unknown> {
    return api.get("/ai/sessions/stats");
  }

  async getSessions(): Promise<unknown[]> {
    return api.get<unknown[]>("/ai/sessions");
  }

  async getUserMessages(userId: string): Promise<unknown[]> {
    return api.get<unknown[]>(`/ai/sessions/${userId}/messages`);
  }

  async exportUserData(userId: string): Promise<unknown> {
    return api.get(`/ai/sessions/${userId}/export`);
  }

  async deleteUserData(userId: string): Promise<void> {
    await api.delete(`/ai/sessions/${userId}`);
  }

  // ---- Voice ----

  async getVoiceConfig(): Promise<VoiceConfig> {
    return api.get<VoiceConfig>("/ai/voice/config");
  }

  async updateVoiceConfig(config: VoiceConfig): Promise<void> {
    await api.put("/ai/voice/config", config);
  }

  async transcribeAudio(file: File): Promise<{ text: string; filename: string }> {
    const formData = new FormData();
    formData.append("audio", file);
    const token = typeof window !== "undefined" ? localStorage.getItem("auth_token") : null;
    const headers: Record<string, string> = {};
    if (token) headers["Authorization"] = `Bearer ${token}`;
    const resp = await fetch("/api/v1/ai/voice/transcribe", { method: "POST", headers, body: formData });
    if (!resp.ok) throw new Error(await resp.text());
    return resp.json();
  }

  async textToSpeech(text: string): Promise<Blob> {
    const token = typeof window !== "undefined" ? localStorage.getItem("auth_token") : null;
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (token) headers["Authorization"] = `Bearer ${token}`;
    const resp = await fetch("/api/v1/ai/voice/tts", { method: "POST", headers, body: JSON.stringify({ text }) });
    if (!resp.ok) throw new Error(await resp.text());
    return resp.blob();
  }
}

export const aiService = new AIService();
