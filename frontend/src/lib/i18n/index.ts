// i18n 国际化配置
import {
  init,
  register,
  locale,
  getLocaleFromNavigator,
  waitLocale,
  isLoading,
} from "svelte-i18n";
import { browser } from "$app/environment";

// 注册语言包
register("zh-CN", () => import("./zh-CN.json"));
register("en-US", () => import("./en-US.json"));

// 支持的语言列表
export const supportedLanguages = [
  { code: "zh-CN", name: "简体中文", nativeName: "简体中文" },
  { code: "en-US", name: "English", nativeName: "English" },
] as const;

// 支持的区域列表
export const supportedRegions = [
  {
    code: "cn",
    name: { "zh-CN": "中国大陆", "en-US": "China Mainland" },
    description: {
      "zh-CN": "使用国内镜像源，速度更快",
      "en-US": "Use China mirrors for faster speed",
    },
  },
  {
    code: "intl",
    name: { "zh-CN": "国际", "en-US": "International" },
    description: {
      "zh-CN": "使用官方源",
      "en-US": "Use official sources",
    },
  },
] as const;

export type LanguageCode = (typeof supportedLanguages)[number]["code"];
export type RegionCode = (typeof supportedRegions)[number]["code"];

// localStorage key for language preference
const LANG_STORAGE_KEY = "rde_language";

// 检测用户语言（优先使用本地存储的偏好）
function detectLanguage(): LanguageCode {
  if (!browser) return "en-US";

  // 优先使用本地存储的用户偏好
  const savedLang = localStorage.getItem(LANG_STORAGE_KEY);
  if (savedLang === "zh-CN" || savedLang === "en-US") {
    return savedLang;
  }

  // 否则检测浏览器语言
  const navLocale = getLocaleFromNavigator();
  if (navLocale?.startsWith("zh")) {
    return "zh-CN";
  }
  return "en-US";
}

// 保存语言偏好到本地存储
export function saveLanguagePreference(lang: LanguageCode) {
  if (browser) {
    localStorage.setItem(LANG_STORAGE_KEY, lang);
  }
}

// 根据语言获取推荐区域
export function getDefaultRegionForLanguage(lang: LanguageCode): RegionCode {
  return lang === "zh-CN" ? "cn" : "intl";
}

// 根据区域获取推荐语言
export function getDefaultLanguageForRegion(region: RegionCode): LanguageCode {
  return region === "cn" ? "zh-CN" : "en-US";
}

// 初始化 i18n（返回 Promise，等待语言包加载完成）
export async function initI18n(initialLocale?: string) {
  init({
    fallbackLocale: "en-US",
    initialLocale: initialLocale || detectLanguage(),
  });
  // 等待初始语言包加载完成
  await waitLocale();
}

// 设置语言（并保存到本地存储）
export function setLanguage(lang: LanguageCode) {
  locale.set(lang);
  saveLanguagePreference(lang);
  if (browser) {
    document.documentElement.lang = lang;
  }
}

// 获取当前语言
export function getCurrentLanguage(): LanguageCode {
  let current: LanguageCode = "en-US";
  locale.subscribe((value) => {
    if (value) current = value as LanguageCode;
  })();
  return current;
}

// 导出 svelte-i18n 的核心函数
export { locale, _ as t, format, time, date, number } from "svelte-i18n";
// 导出加载状态，用于在组件中检查是否加载完成
export { isLoading, waitLocale };
