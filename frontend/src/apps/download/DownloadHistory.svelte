<script lang="ts">
  import { onMount } from "svelte";
  import Icon from "@iconify/svelte";
  import { t } from "svelte-i18n";
  import { Button, Spinner, EmptyState } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import { downloadService, type DownloadHistory } from "./service";

  // Props
  interface Props {
    onClose?: () => void;
  }
  let { onClose }: Props = $props();

  // 状态
  let history = $state<DownloadHistory[]>([]);
  let total = $state(0);
  let loading = $state(true);
  let loadingMore = $state(false);
  let searchKeyword = $state("");
  let currentOffset = $state(0);
  const pageSize = 30;

  onMount(() => {
    loadHistory();
  });

  async function loadHistory(append = false) {
    if (append) {
      loadingMore = true;
    } else {
      loading = true;
      currentOffset = 0;
    }

    try {
      const result = await downloadService.getHistory(pageSize, currentOffset);
      if (append) {
        history = [...history, ...result.items];
      } else {
        history = result.items;
      }
      total = result.total;
      currentOffset += result.items.length;
    } catch (e: any) {
      showToast($t("download.loadHistoryFailed") + ": " + e.message, "error");
    } finally {
      loading = false;
      loadingMore = false;
    }
  }

  async function searchHistory() {
    if (!searchKeyword.trim()) {
      loadHistory();
      return;
    }

    loading = true;
    try {
      const result = await downloadService.searchHistory(searchKeyword.trim());
      history = result.items;
      total = result.items.length;
    } catch (e: any) {
      showToast($t("download.searchFailed") + ": " + e.message, "error");
    } finally {
      loading = false;
    }
  }

  async function deleteItem(id: number) {
    try {
      await downloadService.deleteHistoryItem(id);
      history = history.filter((h) => h.id !== id);
      total--;
      showToast($t("download.deleted"), "success");
    } catch (e: any) {
      showToast($t("download.deleteFailed") + ": " + e.message, "error");
    }
  }

  async function clearAll() {
    if (!confirm($t("download.clearHistoryConfirm"))) return;

    try {
      await downloadService.clearHistory();
      history = [];
      total = 0;
      showToast($t("download.historyCleared"), "success");
    } catch (e: any) {
      showToast($t("download.clearFailed") + ": " + e.message, "error");
    }
  }

  function formatBytes(bytes: number): string {
    if (!bytes || bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
  }

  function formatSpeed(bps: number): string {
    return formatBytes(bps) + "/s";
  }

  function formatDate(dateStr: string): string {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffDays === 0) {
      return date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
    } else if (diffDays === 1) {
      return $t("download.yesterday");
    } else if (diffDays < 7) {
      return `${diffDays} 天前`;
    } else {
      return date.toLocaleDateString("zh-CN");
    }
  }

  function formatDuration(seconds: number): string {
    if (seconds < 60) return `${seconds}秒`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}分钟`;
    const hours = Math.floor(seconds / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    return `${hours}小时${mins}分钟`;
  }

  function statusIcon(status: string): string {
    switch (status) {
      case "complete":
        return "mdi:check-circle";
      case "error":
        return "mdi:alert-circle";
      default:
        return "mdi:file-outline";
    }
  }

  function statusColor(status: string): string {
    switch (status) {
      case "complete":
        return "text-green-500";
      case "error":
        return "text-red-500";
      default:
        return "text-gray-500";
    }
  }

  let hasMore = $derived(history.length < total);
</script>

<div class="history-panel">
  <!-- 头部 -->
  <div class="history-header">
    <div class="search-box">
      <Icon icon="mdi:magnify" width="18" />
      <input
        type="text"
        placeholder={$t("download.searchHistory")}
        bind:value={searchKeyword}
        onkeydown={(e) => e.key === "Enter" && searchHistory()}
      />
      {#if searchKeyword}
        <button class="clear-search" onclick={() => { searchKeyword = ""; loadHistory(); }}>
          <Icon icon="mdi:close" width="16" />
        </button>
      {/if}
    </div>
    <Button variant="ghost" size="sm" onclick={clearAll} disabled={history.length === 0}>
      <Icon icon="mdi:delete-sweep" width="16" /> 清空
    </Button>
  </div>

  <!-- 列表 -->
  <div class="history-content">
    {#if loading && history.length === 0}
      <div class="loading-container">
        <Spinner />
      </div>
    {:else if history.length === 0}
      <EmptyState
        icon="mdi:history"
        title={$t("download.noHistory")}
        description={$t("download.historyDescription")}
      />
    {:else}
      <div class="history-list">
        {#each history as item (item.id)}
          <div class="history-item">
            <div class="item-icon {statusColor(item.status)}">
              <Icon icon={statusIcon(item.status)} width="24" />
            </div>
            <div class="item-info">
              <div class="item-name" title={item.name}>{item.name}</div>
              <div class="item-meta">
                <span>{formatBytes(item.size)}</span>
                {#if item.avg_speed > 0}
                  <span>{$t("download.average")} {formatSpeed(item.avg_speed)}</span>
                {/if}
                {#if item.duration > 0}
                  <span>{$t("download.duration")} {formatDuration(item.duration)}</span>
                {/if}
                <span>{formatDate(item.created_at)}</span>
              </div>
              {#if item.error_message}
                <div class="item-error">{item.error_message}</div>
              {/if}
            </div>
            <button class="delete-btn" onclick={() => deleteItem(item.id)} title={$t("download.delete")}>
              <Icon icon="mdi:close" width="16" />
            </button>
          </div>
        {/each}
      </div>

      {#if hasMore}
        <div class="load-more">
          <Button variant="ghost" size="sm" loading={loadingMore} onclick={() => loadHistory(true)}>
            加载更多
          </Button>
        </div>
      {/if}
    {/if}
  </div>
</div>

<style>
  .history-panel {
    display: flex;
    flex-direction: column;
    height: 100%;
    max-height: 70vh;
  }

  .history-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--border-color, #e0e0e0);
  }

  .search-box {
    display: flex;
    align-items: center;
    flex: 1;
    gap: 8px;
    padding: 8px 12px;
    background: var(--bg-secondary, #f5f5f5);
    border-radius: 8px;
    color: var(--text-muted, #999);

    input {
      flex: 1;
      border: none;
      background: transparent;
      font-size: 14px;
      color: var(--text-primary, #333);
      outline: none;

      &::placeholder {
        color: var(--text-muted, #999);
      }
    }

    .clear-search {
      display: flex;
      padding: 2px;
      border: none;
      background: transparent;
      color: var(--text-muted, #999);
      cursor: pointer;

      &:hover {
        color: var(--text-primary, #333);
      }
    }
  }

  .history-content {
    flex: 1;
    overflow-y: auto;
    padding: 12px 0;
  }

  .loading-container {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 40px;
  }

  .history-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .history-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px;
    background: var(--bg-card, white);
    border-radius: 8px;
    border: 1px solid var(--border-color, #e0e0e0);

    &:hover {
      border-color: var(--color-primary, #4a90d9);

      .delete-btn {
        opacity: 1;
      }
    }
  }

  .item-icon {
    flex-shrink: 0;

    &.text-green-500 {
      color: #10b981;
    }
    &.text-red-500 {
      color: #ef4444;
    }
    &.text-gray-500 {
      color: #6b7280;
    }
  }

  .item-info {
    flex: 1;
    min-width: 0;
  }

  .item-name {
    font-size: 14px;
    font-weight: 500;
    color: var(--text-primary, #333);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .item-meta {
    display: flex;
    gap: 12px;
    margin-top: 4px;
    font-size: 12px;
    color: var(--text-muted, #999);
  }

  .item-error {
    margin-top: 4px;
    font-size: 12px;
    color: #ef4444;
  }

  .delete-btn {
    display: flex;
    padding: 6px;
    border: none;
    background: transparent;
    color: var(--text-muted, #999);
    cursor: pointer;
    opacity: 0;
    transition: opacity 0.2s;
    border-radius: 4px;

    &:hover {
      background: var(--bg-secondary, #f5f5f5);
      color: #ef4444;
    }
  }

  .load-more {
    display: flex;
    justify-content: center;
    padding: 12px;
  }
</style>
