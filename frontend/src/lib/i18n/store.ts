// i18n 状态管理
import { writable, derived, get } from "svelte/store";
import { locale } from "svelte-i18n";
import { browser } from "$app/environment";
import {
  getI18nSettings,
  updateI18nSettings,
  type I18nSettingsRequest,
} from "./api";

// 区域代码类型
export type RegionCode = "cn" | "intl";
export type LanguageCode = "zh-CN" | "en-US";

// i18n 状态
interface I18nState {
  language: LanguageCode;
  region: RegionCode;
  detectedRegion: RegionCode;
  mirrors: Record<string, string>;
  lyricSource: string;
  loaded: boolean;
  loading: boolean;
}

// 检测浏览器语言
function detectBrowserLanguage(): LanguageCode {
  if (typeof window === "undefined") return "en-US";
  return navigator.language.startsWith("zh") ? "zh-CN" : "en-US";
}

// 创建状态存储
function createI18nStore() {
  // 初始语言根据浏览器检测
  const initialLang = detectBrowserLanguage();
  const { subscribe, set, update } = writable<I18nState>({
    language: initialLang,
    region: initialLang === "zh-CN" ? "cn" : "intl",
    detectedRegion: initialLang === "zh-CN" ? "cn" : "intl",
    mirrors: {},
    lyricSource: "follow",
    loaded: false,
    loading: false,
  });

  return {
    subscribe,

    // 初始化：从后端加载设置（仅 region/mirrors 等，语言由前端管理）
    async init() {
      update((s) => ({ ...s, loading: true }));
      try {
        const settings = await getI18nSettings();
        update((s) => ({
          ...s,
          region: settings.region as RegionCode,
          detectedRegion: settings.detected_region as RegionCode,
          mirrors: settings.mirrors,
          lyricSource: settings.lyric_source,
          loaded: true,
          loading: false,
        }));
      } catch (error) {
        console.error("Failed to load i18n settings:", error);
        update((s) => ({ 
          ...s, 
          loading: false,
          loaded: true, // 标记为已加载，使用默认值
        }));
      }
    },

    // 设置语言（纯前端，不同步后端）
    async setLanguage(lang: LanguageCode) {
      const currentState = get({ subscribe });
      if (currentState.language === lang) return;

      // 保存到 localStorage
      if (browser) {
        localStorage.setItem("rde_language", lang);
      }
      
      update((s) => ({ ...s, language: lang }));
      locale.set(lang);
      if (browser) {
        document.documentElement.lang = lang;
      }
    },

    // 设置区域
    async setRegion(region: RegionCode) {
      const currentState = get({ subscribe });
      if (currentState.region === region) return;

      try {
        await updateI18nSettings({ region });
        update((s) => ({ ...s, region }));
      } catch (error) {
        console.error("Failed to update region:", error);
        throw error;
      }
    },

    // 更新设置（仅同步 region/mirrors 等到后端，语言不同步）
    async updateSettings(settings: I18nSettingsRequest) {
      // 先更新本地状态（即使 API 失败也能生效）
      if (settings.region) {
        update((s) => ({ ...s, region: settings.region as RegionCode }));
      }

      // 尝试同步到后端
      try {
        const updated = await updateI18nSettings(settings);
        update((s) => ({
          ...s,
          region: updated.region as RegionCode,
          mirrors: updated.mirrors,
          lyricSource: updated.lyric_source,
        }));
      } catch (error) {
        console.error("Failed to update i18n settings to backend:", error);
        // API 失败不阻塞，本地状态已更新
      }
    },

    // 获取当前语言
    getLanguage(): LanguageCode {
      return get({ subscribe }).language;
    },

    // 获取当前区域
    getRegion(): RegionCode {
      return get({ subscribe }).region;
    },
  };
}

// 导出 store 实例
export const i18nStore = createI18nStore();

// 派生 store：当前语言
export const currentLanguage = derived(
  i18nStore,
  ($state) => $state.language
);

// 派生 store：当前区域
export const currentRegion = derived(i18nStore, ($state) => $state.region);

// 派生 store：是否中国区
export const isChinaRegion = derived(
  i18nStore,
  ($state) => $state.region === "cn"
);

// 派生 store：是否已加载
export const i18nLoaded = derived(i18nStore, ($state) => $state.loaded);
