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

  let { children } = $props();
  let i18nReady = $state(false);

  // 初始化 i18n（异步等待语言包加载）
  initI18n().then(() => {
    i18nReady = true;
  });

  // 设置全局 401 处理器
  if (browser) {
    setUnauthorizedHandler(() => {
      const path = window.location.pathname;
      if (!path.startsWith("/login") && !path.startsWith("/setup")) {
        goto("/login");
      }
    });
  }

  // 确保主题在客户端初始化
  onMount(async () => {
    // 异步加载离线图标集（避免阻塞首屏）
    await initOfflineIcons();
    // 强制初始化主题（确保 data-theme 和强调色被应用）
    theme.set(theme.mode);
    
    // 只在非登录/setup 页面加载 i18n 设置（需要认证）
    const path = window.location.pathname;
    if (!path.startsWith("/login") && !path.startsWith("/setup")) {
      // 从后端加载 i18n 设置
      await i18nStore.init();
    }
  });
</script>

<svelte:head>
  <link rel="icon" href="/favicon.svg" />
  <!-- 字体已本地化，无需在线加载 -->
</svelte:head>

{#if i18nReady}
  {@render children()}
  <ConfirmModal />
{/if}
