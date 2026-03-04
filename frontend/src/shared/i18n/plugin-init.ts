// i18n 国际化配置 — 插件 SPA 版（不依赖 SvelteKit）
// 各插件在自己的 i18n/index.ts 中调用 setupPluginI18n() 注册语言包
import { init, register, locale, getLocaleFromNavigator, waitLocale } from "svelte-i18n";

export type LanguageCode = "zh-CN" | "en-US";

const LANG_STORAGE_KEY = "rde_language";

function detectLanguage(): LanguageCode {
  const saved = localStorage.getItem(LANG_STORAGE_KEY);
  if (saved === "zh-CN" || saved === "en-US") return saved;
  const nav = getLocaleFromNavigator();
  return nav?.startsWith("zh") ? "zh-CN" : "en-US";
}

/**
 * 插件 i18n 初始化
 * @param loaders 各语言的动态加载函数，如 { "zh-CN": () => import("./zh-CN.json"), "en-US": () => import("./en-US.json") }
 */
export async function setupPluginI18n(
  loaders: Record<string, () => Promise<any>>,
  initialLocale?: string
) {
  for (const [lang, loader] of Object.entries(loaders)) {
    register(lang, loader);
  }
  init({
    fallbackLocale: "en-US",
    initialLocale: initialLocale || detectLanguage(),
  });
  await waitLocale();
}

export function setLanguage(lang: LanguageCode) {
  locale.set(lang);
  localStorage.setItem(LANG_STORAGE_KEY, lang);
  document.documentElement.lang = lang;
}

export function getCurrentLanguage(): LanguageCode {
  let current: LanguageCode = "en-US";
  locale.subscribe((v) => {
    if (v) current = v as LanguageCode;
  })();
  return current;
}

// 导出 svelte-i18n 核心
export { locale, _ as t } from "svelte-i18n";
