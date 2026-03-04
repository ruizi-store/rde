<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import Icon from "@iconify/svelte";
  import { t } from "./i18n";
  import { vmService, type VM, type VMStats } from "./service";

  // ==================== Props ====================

  interface Props {
    vm: VM;
  }

  let { vm }: Props = $props();

  // ==================== 状态 ====================

  let stats = $state<VMStats | null>(null);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let refreshTimer: ReturnType<typeof setInterval> | null = null;

  // ==================== 生命周期 ====================

  onMount(() => {
    fetchStats();
    refreshTimer = setInterval(fetchStats, 2000);
  });

  onDestroy(() => {
    if (refreshTimer) clearInterval(refreshTimer);
  });

  // ==================== 方法 ====================

  async function fetchStats() {
    if (vm.status !== "running") {
      stats = null;
      loading = false;
      return;
    }

    try {
      stats = await vmService.getVMStats(vm.id);
      error = null;
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function formatBytes(bytes: number): string {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  }

  function formatUptime(seconds: number): string {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const mins = Math.floor((seconds % 3600) / 60);

    if (days > 0) return `${days}d ${hours}h`;
    if (hours > 0) return `${hours}h ${mins}m`;
    return `${mins}m`;
  }

  function getProgressColor(percent: number): string {
    if (percent > 90) return "var(--color-error, #c62828)";
    if (percent > 70) return "var(--color-warning, #f57c00)";
    return "var(--color-success, #2e7d32)";
  }
</script>

<div class="vm-stats">
  {#if vm.status !== "running"}
    <div class="not-running">
      <Icon icon="mdi:power-off" width="24" />
      <span>{$t("vm.stats.notRunning")}</span>
    </div>
  {:else if loading}
    <div class="loading">{$t("common.loading")}</div>
  {:else if error}
    <div class="error">{error}</div>
  {:else if stats}
    <div class="stats-grid">
      <!-- CPU -->
      <div class="stat-item">
        <div class="stat-header">
          <Icon icon="mdi:cpu-64-bit" width="18" />
          <span>CPU</span>
        </div>
        <div class="stat-value">{stats.cpu_percent.toFixed(1)}%</div>
        <div class="progress-bar">
          <div
            class="progress-fill"
            style="width: {Math.min(stats.cpu_percent, 100)}%; background: {getProgressColor(stats.cpu_percent)}"
          ></div>
        </div>
      </div>

      <!-- 内存 -->
      <div class="stat-item">
        <div class="stat-header">
          <Icon icon="mdi:memory" width="18" />
          <span>{$t("vm.resources.memory")}</span>
        </div>
        {@const memPercent = stats.memory_total > 0 ? (stats.memory_used / stats.memory_total) * 100 : 0}
        <div class="stat-value">
          {formatBytes(stats.memory_used)} / {formatBytes(stats.memory_total)}
        </div>
        <div class="progress-bar">
          <div
            class="progress-fill"
            style="width: {Math.min(memPercent, 100)}%; background: {getProgressColor(memPercent)}"
          ></div>
        </div>
      </div>

      <!-- 磁盘 IO -->
      <div class="stat-item">
        <div class="stat-header">
          <Icon icon="mdi:harddisk" width="18" />
          <span>{$t("vm.resources.diskIO")}</span>
        </div>
        <div class="stat-value small">
          <span class="io-label">{$t("vm.resources.read")}</span> {formatBytes(stats.disk_read)}
          <span class="io-sep">|</span>
          <span class="io-label">{$t("vm.resources.write")}</span> {formatBytes(stats.disk_write)}
        </div>
      </div>

      <!-- 运行时间 -->
      <div class="stat-item">
        <div class="stat-header">
          <Icon icon="mdi:clock-outline" width="18" />
          <span>{$t("vm.stats.uptime")}</span>
        </div>
        <div class="stat-value">{formatUptime(stats.uptime)}</div>
      </div>
    </div>
  {/if}
</div>

<style>
  .vm-stats {
    padding: 12px;
    background: var(--bg-tertiary, #f5f5f5);
    border-radius: 8px;
    min-height: 80px;
  }

  .not-running,
  .loading,
  .error {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    color: var(--text-muted, #999);
    font-size: 13px;
    padding: 20px;
  }

  .error {
    color: var(--color-error, #c62828);
  }

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
  }

  .stat-item {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .stat-header {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--text-secondary, #666);
  }

  .stat-value {
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary, #333);
  }

  .stat-value.small {
    font-size: 13px;
  }

  .io-label {
    color: var(--text-muted, #999);
    font-weight: normal;
  }

  .io-sep {
    margin: 0 6px;
    color: var(--border-color, #ddd);
  }

  .progress-bar {
    height: 4px;
    background: var(--border-color, #e0e0e0);
    border-radius: 2px;
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    border-radius: 2px;
    transition: width 0.3s ease;
  }
</style>
