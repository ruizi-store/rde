<script lang="ts">
  import "../app.css";
  import { onMount } from "svelte";
  import { browser } from "$app/environment";
  import { goto } from "$app/navigation";
  import { theme } from "$shared/stores/theme.svelte";
  import { initOfflineIcons } from "$shared/icons";
  import { ConfirmModal } from "$shared/ui";
  import { setUnauthorizedHandler } from "$shared/services/api";
  import { initI18n, isLoading } from "$lib/i18n";
  import { i18nStore } from "$lib/i18n/store";
  import { clearAuthItems, migrateLegacyAuthKeys } from "$shared/utils/auth-storage";
  import { reconcileInstance } from "$shared/utils/instance";

  let { children } = $props();
  let i18nReady = $state(false);
  let instanceReady = $state(!browser);

  // 初始化 i18n（异步等待语言包加载）
  initI18n().then(() => {
    i18nReady = true;
  });

  // 设置全局 401 处理器：清认证并跳转登录
  if (browser) {
    setUnauthorizedHandler(() => {
      clearAuthItems();
      const path = window.location.pathname;
      if (!path.startsWith("/login") && !path.startsWith("/setup")) {
        goto("/login");
      }
    });
  }

  onMount(async () => {
    // 最先对账实例 ID，避免同 IP 换机沿用旧 localStorage
    try {
      await reconcileInstance();
      migrateLegacyAuthKeys();
    } catch (e) {
      console.error("[instance] reconcile failed", e);
    } finally {
      instanceReady = true;
    }

    try {
      await initOfflineIcons();
    } catch (e) {
      console.error("[Icons] init failed", e);
    }
    theme.set(theme.mode);

    const path = window.location.pathname;
    if (!path.startsWith("/login") && !path.startsWith("/setup")) {
      await i18nStore.init();
    }
  });
</script>

<svelte:head>
  <link rel="icon" href="/favicon.svg" />
  <!-- 字体已本地化，无需在线加载 -->
</svelte:head>

{#if i18nReady && instanceReady}
  {@render children()}
  <ConfirmModal />
{/if}
