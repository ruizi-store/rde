<!--
  AI 助手 — 插件前端 App 壳
  使用 AppShell 包裹从 rde 主前端迁移的完整 AI.svelte
-->
<script lang="ts">
  import { onMount } from "svelte";
  import AppShell from "$shared/components/AppShell.svelte";
  import { initI18n } from "./i18n";
  import AI from "./AI.svelte";

  let ready = $state(false);

  onMount(async () => {
    await initI18n();
    ready = true;
  });
</script>

{#if ready}
  <AppShell title="AI 助手" icon="🤖" showBack={true}>
    <AI />
  </AppShell>
{:else}
  <div style="display:flex;align-items:center;justify-content:center;height:100vh;background:#12122a;color:#a0a0c0;">
    加载中...
  </div>
{/if}
