/**
 * Translate Service - 翻译服务
 *
 * 后端路由: /api/v1/translate/...
 * 使用 LibreTranslate 本地离线翻译
 */

import { api } from "$shared/services/api";

// ==================== 类型定义 ====================

export interface Language {
  code: string;
  name: string;
}

export interface TranslateRequest {
  text: string;
  source?: string; // 源语言，空则自动检测
  target: string;  // 目标语言
}

export interface TranslateResponse {
  translatedText: string;
  detectedLang?: string;
}

export interface DetectRequest {
  text: string;
}

export interface DetectResponse {
  language: string;
  confidence: number;
}

export type ServicePhase =
  | "ready"
  | "starting"
  | "ready_to_start"
  | "missing"
  | "error"
  | string;

export interface ServiceStatus {
  available: boolean;
  url: string;
  message?: string;
  phase?: ServicePhase;
  image?: string;
  container?: string;
  containerRunning?: boolean;
  imageReady?: boolean;
  offlineReady?: boolean;
}

export interface EnsureResult {
  status: "success" | "starting" | "failed" | string;
  available: boolean;
  phase?: ServicePhase;
  message?: string;
}

export interface TranslateConfig {
  defaultSource: string;
  defaultTarget: string;
  serviceUrl: string;
}

// ==================== API 调用 ====================

/**
 * 翻译文本
 */
export async function translateText(req: TranslateRequest): Promise<TranslateResponse> {
  const response = await api.post<TranslateResponse>("/translate/text", req);
  return response;
}

/**
 * 检测语言
 */
export async function detectLanguage(text: string): Promise<DetectResponse> {
  const response = await api.post<DetectResponse>("/translate/detect", { text });
  return response;
}

/**
 * 获取支持的语言列表
 */
export async function getLanguages(): Promise<Language[]> {
  const response = await api.get<Language[]>("/translate/languages");
  return response;
}

/**
 * 获取服务状态
 */
export async function getStatus(): Promise<ServiceStatus> {
  const response = await api.get<ServiceStatus>("/translate/status");
  return response;
}

/**
 * 确保翻译服务已启动（复用容器 / 离线镜像，不走 Docker Store 安装）
 * 旧后端无 /ensure 时降级为轮询 /status，避免 UI 只显示 "not found"
 */
export async function ensureService(waitSec = 20): Promise<EnsureResult> {
  try {
    return await api.post<EnsureResult>(`/translate/ensure?wait=${waitSec}`);
  } catch (e: unknown) {
    const status = (e as { status?: number })?.status;
    const message = (e as { message?: string })?.message || "";
    if (status !== 404 && !/not found/i.test(message)) {
      throw e;
    }
    // 兼容旧包：无 ensure 路由时轮询 status
    const deadline = Date.now() + Math.max(waitSec, 5) * 1000;
    while (Date.now() < deadline) {
      const st = await getStatus().catch(() => null);
      if (st?.available) {
        return {
          status: "success",
          available: true,
          phase: "ready",
          message: st.message || "翻译服务已就绪",
        };
      }
      await new Promise((r) => setTimeout(r, 2000));
    }
    const st = await getStatus().catch(() => null);
    if (st?.available) {
      return { status: "success", available: true, phase: "ready", message: st.message };
    }
    return {
      status: "failed",
      available: false,
      phase: st?.phase || "error",
      message:
        st?.message ||
        "当前系统版本缺少翻译启动接口，请升级 RDE 或等待离线引导完成后再试",
    };
  }
}

/**
 * 获取翻译配置
 * @param lang 系统语言（用于确定默认翻译方向）
 */
export async function getConfig(lang?: string): Promise<TranslateConfig> {
  const params = lang ? `?lang=${lang}` : "";
  const response = await api.get<TranslateConfig>(`/translate/config${params}`);
  return response;
}

// ==================== 辅助函数 ====================

/**
 * 常用语言映射（用于 UI 显示）
 */
export const COMMON_LANGUAGES: Record<string, string> = {
  en: "English",
  zh: "中文",
  ja: "日本語",
  ko: "한국어",
  fr: "Français",
  de: "Deutsch",
  es: "Español",
  pt: "Português",
  ru: "Русский",
  ar: "العربية",
  it: "Italiano",
};

/**
 * 获取语言显示名称
 */
export function getLanguageName(code: string, languages: Language[]): string {
  const lang = languages.find((l) => l.code === code);
  return lang?.name || COMMON_LANGUAGES[code] || code;
}
