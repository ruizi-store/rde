<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { t } from "svelte-i18n";
  import Icon from "@iconify/svelte";
  import { Button, Modal, Spinner, EmptyState, Tabs, Progress } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import { fileService, type FileInfo } from "$shared/services/files";
  import {
    downloadService,
    type DownloadTask,
    type DownloadStats,
    type DownloadEventType,
  } from "./service";
  import DownloadSettings from "./DownloadSettings.svelte";
  import { apps } from "$desktop/stores/apps.svelte";

  // ==================== 状态 ====================

  let activeTasks = $state<DownloadTask[]>([]);
  let waitingTasks = $state<DownloadTask[]>([]);
  let stoppedTasks = $state<DownloadTask[]>([]);
  let stats = $state<DownloadStats | null>(null);
  let serviceRunning = $state(false);
  let loading = $state(true);
  let currentFilter = $state("all");
  let showAddModal = $state(false);
  let showSettingsModal = $state(false);
  let addUrls = $state("");
  let addDir = $state("");
  let adding = $state(false);

  // 右键菜单状态
  let contextMenu = $state<{ x: number; y: number; target: HTMLTextAreaElement | null } | null>(null);

  // 目录浏览器状态
  let showDirBrowser = $state(false);
  let browserPath = $state("/");
  let browserFiles = $state<FileInfo[]>([]);
  let loadingBrowser = $state(false);

  let ws: WebSocket | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout>;

  const filterTabs = $derived([
    { id: "all", label: $t('download.filterAll') },
    { id: "active", label: $t('download.filterActive') },
    { id: "waiting", label: $t('download.filterWaiting') },
    { id: "stopped", label: $t('download.filterCompleted') },
  ]);

  // 派生：根据过滤条件显示的任务
  let tasks = $derived.by(() => {
    switch (currentFilter) {
      case "active":
        return activeTasks;
      case "waiting":
        return waitingTasks;
      case "stopped":
        return stoppedTasks;
      default:
        return [...activeTasks, ...waitingTasks, ...stoppedTasks];
    }
  });

  // ==================== 生命周期 ====================

  onMount(() => {
    loadInitialData();
    connectWebSocket();
  });

  onDestroy(() => {
    clearTimeout(reconnectTimer);
    ws?.close();
    ws = null;
  });

  // ==================== WebSocket ====================

  function connectWebSocket() {
    ws = downloadService.connectWebSocket({
      onOpen: () => {
        serviceRunning = true;
      },
      onTask: handleTaskEvent,
      onStats: (newStats) => {
        stats = newStats;
      },
      onServiceStatus: (running) => {
        serviceRunning = running;
        if (running) {
          loadInitialData();
        }
      },
      onError: () => {
        serviceRunning = false;
      },
      onClose: () => {
        // 5 秒后重连
        reconnectTimer = setTimeout(() => {
          if (!ws || ws.readyState === WebSocket.CLOSED) {
            connectWebSocket();
          }
        }, 5000);
      },
    });
  }

  function handleTaskEvent(type: DownloadEventType, task: DownloadTask) {
    switch (type) {
      case "task:added":
        if (task.status === "active") {
          activeTasks = [task, ...activeTasks.filter((t) => t.gid !== task.gid)];
        } else if (task.status === "waiting") {
          waitingTasks = [task, ...waitingTasks.filter((t) => t.gid !== task.gid)];
        }
        break;

      case "task:progress":
        activeTasks = activeTasks.map((t) => (t.gid === task.gid ? task : t));
        break;

      case "task:completed":
        activeTasks = activeTasks.filter((t) => t.gid !== task.gid);
        waitingTasks = waitingTasks.filter((t) => t.gid !== task.gid);
        stoppedTasks = [task, ...stoppedTasks.filter((t) => t.gid !== task.gid)];
        // 发送通知
        if (stats?.num_active === 0) {
          sendNotification(task);
        }
        break;

      case "task:paused":
        activeTasks = activeTasks.map((t) => (t.gid === task.gid ? task : t));
        break;

      case "task:error":
        activeTasks = activeTasks.filter((t) => t.gid !== task.gid);
        waitingTasks = waitingTasks.filter((t) => t.gid !== task.gid);
        stoppedTasks = [task, ...stoppedTasks.filter((t) => t.gid !== task.gid)];
        showToast(`下载失败: ${task.name}`, "error");
        break;

      case "task:removed":
        activeTasks = activeTasks.filter((t) => t.gid !== task.gid);
        waitingTasks = waitingTasks.filter((t) => t.gid !== task.gid);
        stoppedTasks = stoppedTasks.filter((t) => t.gid !== task.gid);
        break;
    }
  }

  function sendNotification(task: DownloadTask) {
    if (!("Notification" in window) || Notification.permission !== "granted") return;
    new Notification($t('download.downloadComplete'), {
      body: task.name,
      icon: "/icons/download.png",
    });
  }

  // ==================== 方法 ====================

  async function loadInitialData() {
    try {
      const response = await downloadService.getTasks();
      activeTasks = response.active || [];
      waitingTasks = response.waiting || [];
      stoppedTasks = response.stopped || [];
      stats = response.stats || null;
      serviceRunning = true;
    } catch (e) {
      console.error("[Download] loadInitialData error:", e);
      serviceRunning = false;
    } finally {
      loading = false;
    }
  }

  async function addDownload() {
    const uris = addUrls
      .split("\n")
      .map((u) => u.trim())
      .filter(Boolean);
    if (uris.length === 0) return;

    adding = true;
    try {
      await downloadService.addUri({ uris, dir: addDir || undefined });
      showAddModal = false;
      addUrls = "";
      addDir = "";
      showToast($t('download.taskAdded'), "success");
    } catch (e: any) {
      showToast($t('download.addFailed') + " " + (e.message || e), "error");
    } finally {
      adding = false;
    }
  }

  // ==================== 右键菜单 ====================

  function showContextMenu(e: MouseEvent) {
    e.preventDefault();
    contextMenu = {
      x: e.clientX,
      y: e.clientY,
      target: e.target as HTMLTextAreaElement,
    };
  }

  function hideContextMenu() {
    contextMenu = null;
  }

  async function contextCut() {
    if (!contextMenu?.target) return;
    const target = contextMenu.target;
    const start = target.selectionStart;
    const end = target.selectionEnd;
    const selected = target.value.substring(start, end);
    if (selected) {
      await navigator.clipboard.writeText(selected);
      addUrls = target.value.substring(0, start) + target.value.substring(end);
    }
    hideContextMenu();
  }

  async function contextCopy() {
    if (!contextMenu?.target) return;
    const target = contextMenu.target;
    const selected = target.value.substring(target.selectionStart, target.selectionEnd);
    if (selected) {
      await navigator.clipboard.writeText(selected);
      showToast($t('download.copied'), "success");
    }
    hideContextMenu();
  }

  async function contextPaste() {
    if (!contextMenu?.target) return;
    const target = contextMenu.target;
    const text = await navigator.clipboard.readText();
    const start = target.selectionStart;
    const end = target.selectionEnd;
    addUrls = target.value.substring(0, start) + text + target.value.substring(end);
    hideContextMenu();
  }

  function contextSelectAll() {
    if (!contextMenu?.target) return;
    contextMenu.target.select();
    hideContextMenu();
  }

  // ==================== 目录浏览器 ====================

  async function openDirBrowser() {
    showDirBrowser = true;
    // 默认打开用户设置的下载目录
    try {
      const settings = await downloadService.getSettings();
      if (settings.download_dir) {
        browserPath = settings.download_dir;
      }
    } catch {
      // 使用默认路径
    }
    await loadBrowserFiles(browserPath);
  }

  async function loadBrowserFiles(path: string) {
    loadingBrowser = true;
    try {
      const response: any = await fileService.list(path, false);
      if (response.data?.content) {
        // 只显示目录
        browserFiles = response.data.content.filter((f: FileInfo) => f.is_dir);
        browserPath = path;
      }
    } catch (e: any) {
      console.error("加载目录列表失败:", e);
    } finally {
      loadingBrowser = false;
    }
  }

  function navigateBrowser(path: string) {
    loadBrowserFiles(path);
  }

  function selectDir(dir: FileInfo) {
    navigateBrowser(dir.path);
  }

  function confirmDir() {
    addDir = browserPath;
    showDirBrowser = false;
  }

  function getBrowserBreadcrumbs() {
    const parts = browserPath.split("/").filter(Boolean);
    const crumbs: { label: string; path: string }[] = [{ label: "/", path: "/" }];
    let accPath = "";
    for (const part of parts) {
      accPath += "/" + part;
      crumbs.push({ label: part, path: accPath });
    }
    return crumbs;
  }

  async function pauseTask(gid: string) {
    try {
      await downloadService.pauseTask(gid);
    } catch (e: any) {
      showToast($t('download.pauseFailed') + " " + e.message, "error");
    }
  }

  async function resumeTask(gid: string) {
    try {
      await downloadService.resumeTask(gid);
    } catch (e: any) {
      showToast($t('download.resumeFailed') + " " + e.message, "error");
    }
  }

  async function removeTask(gid: string) {
    try {
      await downloadService.removeTask(gid);
    } catch (e: any) {
      showToast($t('download.deleteFailed') + " " + e.message, "error");
    }
  }

  function openFileLocation(dir: string | undefined) {
    if (!dir) return;
    apps.launch("file", { initialPath: dir });
  }

  async function purgeCompleted() {
    try {
      await downloadService.purgeResults();
      stoppedTasks = [];
    } catch (e: any) {
      showToast($t('download.clearFailed') + " " + e.message, "error");
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

  function statusText(status: string): string {
    const map: Record<string, string> = {
      active: $t('download.statusDownloading'),
      paused: $t('download.statusPaused'),
      waiting: $t('download.statusWaiting'),
      complete: $t('download.statusComplete'),
      error: $t('download.statusError'),
      removed: $t('download.statusRemoved'),
    };
    return map[status] || status;
  }

  function statusVariant(status: string): "default" | "success" | "warning" | "error" {
    const map: Record<string, "default" | "success" | "warning" | "error"> = {
      active: "default",
      paused: "warning",
      waiting: "default",
      complete: "success",
      error: "error",
    };
    return map[status] || "default";
  }

  function progressVariant(status: string): "default" | "success" | "warning" | "error" {
    const map: Record<string, "default" | "success" | "warning" | "error"> = {
      active: "default",
      paused: "warning",
      complete: "success",
      error: "error",
    };
    return map[status] || "default";
  }
</script>

<div class="download-manager">
  <!-- 头部 -->
  <header class="header">
    <div class="header-left">
      <h2>{$t('download.title')}</h2>
      <span class="service-status" class:running={serviceRunning}>
        <span class="dot"></span>
        {serviceRunning ? $t('download.aria2Running') : $t('download.aria2NotRunning')}
      </span>
    </div>
    {#if stats}
      <div class="speed-info">
        <span class="speed-item">
          <Icon icon="mdi:arrow-down" width="16" />
          {formatSpeed(stats.download_speed)}
        </span>
        <span class="speed-item">
          <Icon icon="mdi:arrow-up" width="16" />
          {formatSpeed(stats.upload_speed)}
        </span>
      </div>
    {/if}
  </header>

  <!-- 工具栏 -->
  <div class="toolbar">
    <div class="toolbar-left">
      <Button variant="primary" size="sm" onclick={() => (showAddModal = true)}>
        <span style="display: inline-flex; align-items: center; gap: 4px;"><Icon icon="mdi:plus" width="16" /> {$t('download.newDownload')}</span>
      </Button>
      <Button variant="ghost" size="sm" onclick={purgeCompleted}>{$t('download.clearCompleted')}</Button>
      <Button variant="ghost" size="sm" onclick={() => (showSettingsModal = true)}>
        <span style="display: inline-flex; align-items: center; gap: 4px;"><Icon icon="mdi:cog" width="16" /> {$t('common.settings')}</span>
      </Button>
    </div>
    <Tabs tabs={filterTabs} bind:activeTab={currentFilter} variant="pills" size="sm">
      {#snippet children(_tab)}{/snippet}
    </Tabs>
  </div>

  <!-- 任务列表 -->
  {#if loading}
    <Spinner center />
  {:else if tasks.length === 0}
    <EmptyState icon="mdi:download-off" title={$t('download.noTasks')} description={$t('download.noTasksHint')} />
  {:else}
    <div class="task-list">
      {#each tasks as task (task.gid)}
        <div class="task-item">
          <div class="task-icon">
            <Icon icon={task.bittorrent ? "mdi:magnet" : "mdi:file-outline"} width="28" />
          </div>
          <div class="task-info">
            <div class="task-name">{task.name || $t('download.unknownFile')}</div>
            <div class="task-meta">
              <span class="status-badge {task.status}">{statusText(task.status)}</span>
              <span>{formatBytes(task.completed_length)} / {formatBytes(task.total_length)}</span>
              {#if task.status === "active"}
                <span>↓ {formatSpeed(task.download_speed)}</span>
                {#if task.bittorrent}
                  <span>↑ {formatSpeed(task.upload_speed)}</span>
                {/if}
              {/if}
            </div>
            <Progress
              value={task.progress}
              max={100}
              size="sm"
              variant={progressVariant(task.status)}
            />
          </div>
          <div class="task-actions">
            {#if task.status === "active"}
              <Button variant="ghost" size="sm" onclick={() => pauseTask(task.gid)}>{$t('download.pause')}</Button>
            {:else if task.status === "paused" || task.status === "waiting"}
              <Button variant="ghost" size="sm" onclick={() => resumeTask(task.gid)}>{$t('download.resume')}</Button>
            {:else if task.status === "complete" && task.dir}
              <Button variant="ghost" size="sm" onclick={() => openFileLocation(task.dir)} title={$t('download.openLocation')}>
                <Icon icon="mdi:folder-open-outline" width="16" />
              </Button>
            {/if}
            <Button variant="ghost" size="sm" onclick={() => removeTask(task.gid)}>
              <Icon icon="mdi:delete-outline" width="16" />
            </Button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- 添加下载模态框 -->
<Modal bind:open={showAddModal} title={$t('download.newDownload')} size="md">
  <form class="add-form" onsubmit={(e) => { e.preventDefault(); addDownload(); }}>
    <div class="form-group">
      <label for="urls">{$t('download.downloadLinks')}</label>
      <textarea
        id="urls"
        bind:value={addUrls}
        required
        placeholder={"http://example.com/file.zip\nmagnet:?xt=urn:btih:..."}
        rows="4"
        oncontextmenu={showContextMenu}
      ></textarea>
    </div>
    <div class="form-group">
      <label for="save-dir">{$t('download.saveDir')}</label>
      <div class="dir-input-row">
        <input id="save-dir" type="text" bind:value={addDir} placeholder={$t('download.saveDirDefault')} />
        <Button variant="outline" onclick={openDirBrowser}>
          <Icon icon="mdi:folder-open" width="16" />
          {$t('download.browse')}
        </Button>
      </div>
    </div>
    <Button variant="primary" fullWidth loading={adding} onclick={addDownload}>{$t('download.startDownload')}</Button>
  </form>
</Modal>

<!-- 右键菜单 -->
{#if contextMenu}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="context-menu-overlay" onclick={hideContextMenu}></div>
  <div class="context-menu" style="left: {contextMenu.x}px; top: {contextMenu.y}px;">
    <button class="context-item" onclick={contextCut}>
      <Icon icon="mdi:content-cut" width="16" />
      {$t('download.cut')}
    </button>
    <button class="context-item" onclick={contextCopy}>
      <Icon icon="mdi:content-copy" width="16" />
      {$t('download.copy')}
    </button>
    <button class="context-item" onclick={contextPaste}>
      <Icon icon="mdi:content-paste" width="16" />
      {$t('download.paste')}
    </button>
    <div class="context-divider"></div>
    <button class="context-item" onclick={contextSelectAll}>
      <Icon icon="mdi:select-all" width="16" />
      {$t('download.selectAll')}
    </button>
  </div>
{/if}

<!-- 目录浏览器 -->
<Modal bind:open={showDirBrowser} title={$t('download.selectSaveDir')} width="550px">
  <div class="dir-browser">
    <!-- 面包屑导航 -->
    <div class="browser-nav">
      <button class="nav-btn" onclick={() => navigateBrowser("/")} title={$t('download.rootDir')}>
        <Icon icon="mdi:home" width="18" />
      </button>
      <button
        class="nav-btn"
        onclick={() => {
          const parent = browserPath.substring(0, browserPath.lastIndexOf("/")) || "/";
          navigateBrowser(parent);
        }}
        disabled={browserPath === "/"}
        title={$t('download.goUp')}
      >
        <Icon icon="mdi:arrow-up" width="18" />
      </button>
      <div class="breadcrumbs">
        {#each getBrowserBreadcrumbs() as crumb, i}
          {#if i > 0}<span class="sep">/</span>{/if}
          <button class="crumb" onclick={() => navigateBrowser(crumb.path)}>
            {crumb.label}
          </button>
        {/each}
      </div>
    </div>

    <!-- 目录列表 -->
    <div class="browser-list">
      {#if loadingBrowser}
        <div class="browser-loading">
          <Icon icon="mdi:loading" width="24" class="spin" />
          <span>{$t('download.loading')}</span>
        </div>
      {:else if browserFiles.length === 0}
        <div class="browser-empty">
          <Icon icon="mdi:folder-open-outline" width="48" />
          <span>{$t('download.noSubDirs')}</span>
        </div>
      {:else}
        {#each browserFiles as file (file.path)}
          <button class="browser-item" ondblclick={() => selectDir(file)}>
            <Icon icon="mdi:folder" width="20" class="folder-icon" />
            <span class="file-name">{file.name}</span>
          </button>
        {/each}
      {/if}
    </div>

    <!-- 底部操作栏 -->
    <div class="browser-footer">
      <div class="selected-path">
        <Icon icon="mdi:folder-marker" width="16" />
        <span>{browserPath}</span>
      </div>
      <div class="browser-actions">
        <Button variant="ghost" onclick={() => (showDirBrowser = false)}>{$t('download.cancel')}</Button>
        <Button variant="primary" onclick={confirmDir}>{$t('download.selectThisDir')}</Button>
      </div>
    </div>
  </div>
</Modal>

<!-- 设置模态框 -->
<Modal bind:open={showSettingsModal} title={$t('download.settings')} size="md">
  <DownloadSettings onClose={() => (showSettingsModal = false)} />
</Modal>

<style>
  .download-manager {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--bg-primary, #f5f5f5);
    color: var(--text-primary, #333);
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    background: var(--bg-card, white);
    border-bottom: 1px solid var(--border-color, #e0e0e0);
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .header h2 {
    margin: 0;
    font-size: 18px;
    font-weight: 600;
  }

  .service-status {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--text-muted, #999);
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #ef4444;
  }

  .service-status.running .dot {
    background: #10b981;
  }

  .speed-info {
    display: flex;
    gap: 16px;
    font-size: 13px;
    color: var(--text-secondary, #666);
  }

  .speed-item {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 20px;
    gap: 12px;
  }

  .toolbar-left {
    display: flex;
    gap: 8px;
  }

  .task-list {
    flex: 1;
    overflow-y: auto;
    padding: 0 20px 20px;
  }

  .task-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 16px;
    background: var(--bg-card, white);
    border-radius: 8px;
    margin-bottom: 8px;
  }

  .task-icon {
    color: var(--text-muted, #999);
    flex-shrink: 0;
  }

  .task-info {
    flex: 1;
    min-width: 0;
  }

  .task-name {
    font-weight: 500;
    font-size: 14px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .task-meta {
    display: flex;
    gap: 12px;
    font-size: 12px;
    color: var(--text-muted, #999);
    margin: 4px 0 6px;
    align-items: center;
  }

  .status-badge {
    padding: 1px 8px;
    border-radius: 4px;
    font-size: 11px;
    font-weight: 500;

    &.active {
      background: #dbeafe;
      color: #1d4ed8;
    }
    &.paused {
      background: #fef3c7;
      color: #b45309;
    }
    &.waiting {
      background: #f3e8ff;
      color: #7c3aed;
    }
    &.complete {
      background: #d1fae5;
      color: #065f46;
    }
    &.error {
      background: #fee2e2;
      color: #991b1b;
    }
  }

  .task-actions {
    display: flex;
    gap: 4px;
    flex-shrink: 0;
  }

  .add-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;

    label {
      font-size: 13px;
      font-weight: 500;
      color: var(--text-secondary, #666);
    }

    input,
    textarea {
      padding: 10px 12px;
      border: 1px solid var(--border-color, #e0e0e0);
      border-radius: 6px;
      font-size: 14px;
      background: var(--bg-input, white);
      color: var(--text-primary, #333);
      resize: vertical;

      &:focus {
        outline: none;
        border-color: var(--color-primary, #4a90d9);
        box-shadow: 0 0 0 3px rgba(74, 144, 217, 0.15);
      }
    }
  }

  /* 目录输入行 */
  .dir-input-row {
    display: flex;
    gap: 8px;
    align-items: center;

    input {
      flex: 1;
    }
  }

  /* 右键菜单 */
  .context-menu-overlay {
    position: fixed;
    inset: 0;
    z-index: 9998;
  }

  .context-menu {
    position: fixed;
    z-index: 9999;
    min-width: 140px;
    background: var(--bg-card, white);
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 8px;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
    padding: 4px 0;
  }

  .context-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 8px 14px;
    border: none;
    background: none;
    font-size: 13px;
    color: var(--text-primary, #333);
    cursor: pointer;
    text-align: left;
  }

  .context-item:hover {
    background: var(--bg-hover, rgba(0, 0, 0, 0.05));
  }

  .context-divider {
    height: 1px;
    margin: 4px 0;
    background: var(--border-color, #e0e0e0);
  }

  /* 目录浏览器 */
  .dir-browser {
    display: flex;
    flex-direction: column;
    height: 400px;
  }

  .browser-nav {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border-color, #e0e0e0);
    background: var(--bg-secondary, #fafafa);
  }

  .nav-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: var(--text-primary, #333);
    cursor: pointer;
  }

  .nav-btn:hover:not(:disabled) {
    background: var(--bg-hover, rgba(0, 0, 0, 0.05));
  }

  .nav-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .breadcrumbs {
    display: flex;
    align-items: center;
    flex: 1;
    overflow-x: auto;
    padding: 0 8px;
    font-size: 13px;
  }

  .breadcrumbs .sep {
    margin: 0 4px;
    color: var(--text-muted, #999);
  }

  .breadcrumbs .crumb {
    border: none;
    background: none;
    color: var(--color-primary, #4a90d9);
    cursor: pointer;
    padding: 2px 4px;
    border-radius: 3px;
  }

  .breadcrumbs .crumb:hover {
    background: var(--bg-hover, rgba(0, 0, 0, 0.05));
  }

  .browser-list {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
  }

  .browser-loading,
  .browser-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 12px;
    color: var(--text-muted, #999);
  }

  .browser-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 10px 12px;
    border: none;
    border-radius: 6px;
    background: transparent;
    text-align: left;
    cursor: pointer;
    transition: background 0.15s;
  }

  .browser-item:hover {
    background: var(--bg-hover, rgba(0, 0, 0, 0.05));
  }

  .browser-item :global(.folder-icon) {
    color: #f5a623;
  }

  .browser-item .file-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .browser-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    border-top: 1px solid var(--border-color, #e0e0e0);
    background: var(--bg-secondary, #fafafa);
  }

  .selected-path {
    display: flex;
    align-items: center;
    gap: 6px;
    flex: 1;
    font-size: 13px;
    overflow: hidden;
    color: var(--text-secondary, #666);
  }

  .selected-path span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .browser-actions {
    display: flex;
    gap: 8px;
    flex-shrink: 0;
  }

  .spin {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
