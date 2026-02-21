<script lang="ts">
  import { onMount } from "svelte";
  import Icon from "@iconify/svelte";
  import { Button, Spinner } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import {
    flatpakService,
    type FlatpakApp,
  } from "./service";

  // Props
  interface Props {
    onInstalled?: () => void;
  }
  let { onInstalled }: Props = $props();

  // 分类
  const categories = [
    { id: "", name: "全部", icon: "mdi:apps" },
    { id: "browser", name: "浏览器", icon: "mdi:web" },
    { id: "office", name: "办公", icon: "mdi:file-document" },
    { id: "development", name: "开发", icon: "mdi:code-braces" },
    { id: "graphics", name: "图形", icon: "mdi:palette" },
    { id: "multimedia", name: "多媒体", icon: "mdi:music-box" },
    { id: "communication", name: "通讯", icon: "mdi:chat" },
    { id: "utility", name: "工具", icon: "mdi:wrench" },
  ];

  // 状态
  let searchQuery = $state("");
  let selectedCategory = $state("");
  let recommendedApps = $state<FlatpakApp[]>([]);
  let searchResults = $state<FlatpakApp[]>([]);
  let loading = $state(true);
  let searching = $state(false);
  let installing = $state<Set<string>>(new Set());

  // 安装进度
  let showProgress = $state(false);
  let progressApp = $state("");
  let progressLogs = $state<string[]>([]);
  let progressStatus = $state<"installing" | "success" | "error">("installing");
  let progressError = $state("");
  let abortInstall = $state<(() => void) | null>(null);
  let logContainer: HTMLDivElement | null = null;

  // 搜索防抖
  let searchTimer: ReturnType<typeof setTimeout> | null = null;

  // 当前显示的应用列表
  let displayApps = $derived.by(() => {
    if (searchQuery.trim().length >= 2) {
      return searchResults;
    }
    if (selectedCategory) {
      return recommendedApps.filter((a) => a.category === selectedCategory);
    }
    return recommendedApps;
  });

  // ==================== 生命周期 ====================

  onMount(async () => {
    try {
      recommendedApps = await flatpakService.getRecommendedApps();
    } catch (e: any) {
      console.error("Failed to load recommended apps:", e);
    } finally {
      loading = false;
    }
  });

  // 搜索变化自动触发
  $effect(() => {
    const q = searchQuery.trim();
    if (q.length < 2) {
      searchResults = [];
      return;
    }
    if (searchTimer) clearTimeout(searchTimer);
    searchTimer = setTimeout(() => doSearch(q), 300);
  });

  // ==================== 方法 ====================

  async function doSearch(query: string) {
    searching = true;
    try {
      searchResults = await flatpakService.searchApps(query);
    } catch (e: any) {
      console.error("Search failed:", e);
    } finally {
      searching = false;
    }
  }

  function installApp(app: FlatpakApp) {
    const id = app.app_id;
    installing = new Set([...installing, id]);
    progressApp = app.name;
    progressLogs = [];
    progressStatus = "installing";
    progressError = "";
    showProgress = true;

    abortInstall = flatpakService.installAppStream(
      id,
      (line) => {
        progressLogs = [...progressLogs, line];
        // 自动滚动到底部
        requestAnimationFrame(() => {
          if (logContainer) {
            logContainer.scrollTop = logContainer.scrollHeight;
          }
        });
      },
      (success, error) => {
        const newSet = new Set(installing);
        newSet.delete(id);
        installing = newSet;

        if (success) {
          progressStatus = "success";
          showToast(`${app.name} 安装完成`, "success");
          // 刷新应用列表
          onInstalled?.();
          // 更新推荐列表中的 installed 状态
          recommendedApps = recommendedApps.map((a) =>
            a.app_id === id ? { ...a, installed: true } : a
          );
          searchResults = searchResults.map((a) =>
            a.app_id === id ? { ...a, installed: true } : a
          );
        } else {
          progressStatus = "error";
          progressError = error || "未知错误";
          showToast(`安装失败: ${error}`, "error");
        }
      }
    );
  }
</script>

<div class="store-container">
  <!-- 搜索栏 -->
  <div class="store-toolbar">
    <div class="search-box">
      <Icon icon="mdi:magnify" width={18} />
      <input
        type="text"
        placeholder="搜索 Flathub 应用..."
        bind:value={searchQuery}
      />
      {#if searching}
        <Spinner size="sm" />
      {/if}
    </div>
  </div>

  <!-- 分类标签 -->
  {#if !searchQuery}
    <div class="category-bar">
      {#each categories as cat}
        <button
          class="cat-btn"
          class:active={selectedCategory === cat.id}
          onclick={() => (selectedCategory = cat.id)}
        >
          <Icon icon={cat.icon} width={16} />
          {cat.name}
        </button>
      {/each}
    </div>
  {/if}

  <!-- 应用列表 -->
  <div class="store-content">
    {#if loading}
      <div class="center-state">
        <Spinner size="lg" />
      </div>
    {:else if displayApps.length === 0}
      <div class="center-state">
        <Icon icon="mdi:package-variant-closed" width={48} />
        <p>{searchQuery ? "没有找到匹配的应用" : "暂无推荐应用"}</p>
      </div>
    {:else}
      <div class="app-list">
        {#each displayApps as app}
          <div class="store-app-card">
            <div class="store-app-icon">
              {#if app.installed && app.icon}
                <img src={flatpakService.getIconUrl(app.app_id)} alt={app.name} />
              {:else}
                <Icon icon="mdi:package-variant" width={36} />
              {/if}
            </div>
            <div class="store-app-info">
              <div class="store-app-name">{app.name}</div>
              <div class="store-app-desc">
                {app.description || app.app_id}
              </div>
              {#if app.version}
                <div class="store-app-meta">v{app.version}</div>
              {/if}
            </div>
            <div class="store-app-action">
              {#if app.installed}
                <Button size="sm" variant="ghost" disabled>
                  <Icon icon="mdi:check" width={14} />
                  已安装
                </Button>
              {:else if installing.has(app.app_id)}
                <Button size="sm" variant="ghost" disabled>
                  <Spinner size="sm" />
                  安装中
                </Button>
              {:else}
                <Button size="sm" variant="primary" onclick={() => installApp(app)}>
                  <Icon icon="mdi:download" width={14} />
                  安装
                </Button>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- 安装进度弹窗 -->
  {#if showProgress}
    <div class="progress-overlay" onclick={() => { if (progressStatus !== "installing") showProgress = false; }}>
      <div class="progress-modal" onclick={(e) => e.stopPropagation()}>
        <div class="progress-header">
          {#if progressStatus === "installing"}
            <Spinner size="sm" />
            <span>正在安装 {progressApp}...</span>
          {:else if progressStatus === "success"}
            <Icon icon="mdi:check-circle" width={20} color="var(--color-success, #4caf50)" />
            <span>{progressApp} 安装完成</span>
          {:else}
            <Icon icon="mdi:alert-circle" width={20} color="var(--color-error, #f44336)" />
            <span>安装失败</span>
          {/if}
        </div>
        <div class="progress-logs" bind:this={logContainer}>
          {#each progressLogs as line}
            <div class="log-line">{line}</div>
          {/each}
          {#if progressError}
            <div class="log-line error">{progressError}</div>
          {/if}
        </div>
        <div class="progress-footer">
          {#if progressStatus === "installing"}
            <Button size="sm" variant="ghost" onclick={() => {
              abortInstall?.();
              showProgress = false;
            }}>
              取消
            </Button>
          {:else}
            <Button size="sm" variant="primary" onclick={() => (showProgress = false)}>
              关闭
            </Button>
          {/if}
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .store-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .store-toolbar {
    display: flex;
    padding: 12px;
    flex-shrink: 0;
  }

  .search-box {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--bg-tertiary, rgba(255, 255, 255, 0.05));
    border: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
    border-radius: 8px;
    padding: 8px 14px;
    flex: 1;
    max-width: 500px;
  }

  .search-box input {
    border: none;
    background: none;
    outline: none;
    color: var(--text-primary, #e0e0e0);
    width: 100%;
    font-size: 13px;
  }

  /* Category Bar */
  .category-bar {
    display: flex;
    gap: 4px;
    padding: 0 12px 8px;
    flex-shrink: 0;
    overflow-x: auto;
  }

  .cat-btn {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 5px 12px;
    border: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
    border-radius: 16px;
    background: none;
    color: var(--text-secondary, #999);
    cursor: pointer;
    font-size: 12px;
    white-space: nowrap;
    transition: all 0.15s;
  }

  .cat-btn:hover {
    background: var(--bg-hover, rgba(255, 255, 255, 0.05));
  }

  .cat-btn.active {
    background: var(--color-primary, #6c63ff);
    border-color: var(--color-primary, #6c63ff);
    color: #fff;
  }

  /* Content */
  .store-content {
    flex: 1;
    overflow-y: auto;
    padding: 0 12px 12px;
  }

  .center-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 12px;
    color: var(--text-secondary, #999);
  }

  .app-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .store-app-card {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    border-radius: 8px;
    background: var(--bg-tertiary, rgba(255, 255, 255, 0.03));
    transition: background 0.15s;
  }

  .store-app-card:hover {
    background: var(--bg-hover, rgba(255, 255, 255, 0.06));
  }

  .store-app-icon {
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .store-app-icon img {
    width: 100%;
    height: 100%;
    object-fit: contain;
    border-radius: 6px;
  }

  .store-app-info {
    flex: 1;
    min-width: 0;
  }

  .store-app-name {
    font-size: 13px;
    font-weight: 500;
  }

  .store-app-desc {
    font-size: 12px;
    color: var(--text-secondary, #999);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .store-app-meta {
    font-size: 11px;
    color: var(--text-tertiary, #666);
  }

  .store-app-action {
    flex-shrink: 0;
  }

  /* Progress Modal */
  .progress-overlay {
    position: absolute;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .progress-modal {
    background: var(--bg-secondary, #16213e);
    border: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
    border-radius: 12px;
    width: 540px;
    max-height: 400px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .progress-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 14px 16px;
    border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
    font-size: 14px;
    font-weight: 500;
  }

  .progress-logs {
    flex: 1;
    overflow-y: auto;
    padding: 10px 14px;
    font-family: "JetBrains Mono", "Fira Code", monospace;
    font-size: 11px;
    line-height: 1.6;
    max-height: 250px;
  }

  .log-line {
    white-space: pre-wrap;
    word-break: break-all;
    color: var(--text-secondary, #aaa);
  }

  .log-line.error {
    color: var(--color-error, #f44336);
  }

  .progress-footer {
    display: flex;
    justify-content: flex-end;
    padding: 10px 14px;
    border-top: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
  }
</style>
