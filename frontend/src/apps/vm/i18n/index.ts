// VM 插件 i18n 初始化
import { setupPluginI18n } from "$shared/i18n/plugin-init";

export { locale, _ as t } from "svelte-i18n";
export { setLanguage, getCurrentLanguage } from "$shared/i18n/plugin-init";

export async function initI18n(initialLocale?: string) {
  await setupPluginI18n(
    {
      "zh-CN": () => import("./zh-CN.json"),
      "en-US": () => import("./en-US.json"),
    },
    initialLocale
  );
}
