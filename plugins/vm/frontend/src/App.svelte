<!--
  虚拟机管理 — 插件前端 App 壳
-->
<script lang="ts">
  import { onMount } from "svelte";
  import AppShell from "$shared/components/AppShell.svelte";
  import { initI18n } from "./i18n";
  import VM from "./VM.svelte";

  let ready = $state(false);

  onMount(async () => {
    await initI18n();
    ready = true;
  });
</script>

{#if ready}
  <AppShell title="虚拟机管理" icon="🖥️" showBack={true}>
    <VM />
  </AppShell>
{:else}
  <div style="display:flex;align-items:center;justify-content:center;height:100vh;background:#12122a;color:#a0a0c0;">
    加载中...
  </div>
{/if}
