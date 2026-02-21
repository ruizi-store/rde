<script lang="ts">
  import { t } from "svelte-i18n";
  import { onMount, onDestroy } from "svelte";
  import Icon from "@iconify/svelte";
  import { Button, Modal, Spinner, EmptyState, Tabs, Progress } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import {
    syncService,
    type SyncStatus,
    type SyncFile,
    type UploadProgress,
    UploadQueue,
  } from "./service";

  // ==================== 状态 ====================

  let status = $state<SyncStatus | null>(null);
  let files = $state<SyncFile[]>([]);
  let totalFiles = $state(0);
  let loading = $state(true);
  let activeTab = $state("files");

  // 上传
  let uploadQueue = $state<UploadQueue | null>(null);
  let uploads = $state<Map<string, UploadProgress>>(new Map());
  let dragOver = $state(false);

  // 文件输入
  let fileInput: HTMLInputElement;

  let refreshTimer: ReturnType<typeof setInterval>;

  const tabs = $derived([
    { id: "files", label: $t("sync.synced") },
    { id: "uploads", label: $t("sync.uploading") },
  ]);

  // ==================== 生命周期 ====================

  onMount(() => {
    initUploadQueue();
    refresh();
    refreshTimer = setInterval(refresh, 10000);
  });

  onDestroy(() => {
    clearInterval(refreshTimer);
    uploadQueue?.cancelAll();
  });

  // ==================== 方法 ====================

  function initUploadQueue() {
    uploadQueue = syncService.createUploadQueue();
    uploadQueue.onProgressUpdate = (newUploads) => {
      uploads = newUploads;
    };
    uploadQueue.onComplete = () => {
      showToast($t("sync.uploadComplete"), "success");
      refresh();
    };
    uploadQueue.onError = (id, error) => {
      showToast(`${$t("sync.uploadFailed")}: ${error.message}`, "error");
    };
  }

  async function refresh() {
    try {
      const [s, f] = await Promise.all([
        syncService.getStatus(),
        syncService.listFiles({ limit: 100 }),
      ]);
      status = s;
      files = f.files || [];
      totalFiles = f.total;
    } catch {
      status = null;
    } finally {
      loading = false;
    }
  }

  function openFileDialog() {
    fileInput?.click();
  }

  function handleFileSelect(e: Event) {
    const input = e.target as HTMLInputElement;
    if (input.files && input.files.length > 0) {
      uploadQueue?.add(input.files);
      activeTab = "uploads";
      input.value = "";
    }
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault();
    dragOver = false;
    if (e.dataTransfer?.files && e.dataTransfer.files.length > 0) {
      uploadQueue?.add(e.dataTransfer.files);
      activeTab = "uploads";
    }
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    dragOver = true;
  }

  function handleDragLeave() {
    dragOver = false;
  }

  function pauseUpload(id: string) {
    uploadQueue?.pause(id);
  }

  function resumeUpload(id: string) {
    uploadQueue?.resume(id);
  }

  function cancelUpload(id: string) {
    uploadQueue?.cancel(id);
  }

  async function deleteFile(id: string) {
    if (!confirm($t("sync.confirmDelete"))) return;
    try {
      await syncService.deleteFile(id);
      showToast($t("sync.fileDeleted"), "success");
      await refresh();
    } catch (e: any) {
      showToast($t("sync.deleteFailed") + ": " + e.message, "error");
    }
  }

  function downloadFile(id: string) {
    const url = syncService.getDownloadUrl(id);
    window.open(url, "_blank");
  }

  function formatBytes(bytes: number): string {
    if (!bytes) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  }

  function formatTime(dateStr: string): string {
    const date = new Date(dateStr);
    return date.toLocaleDateString("zh-CN") + " " + date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
  }

  function getFileIcon(mimeType: string): string {
    if (!mimeType) return "mdi:file";
    if (mimeType.startsWith("image/")) return "mdi:file-image";
    if (mimeType.startsWith("video/")) return "mdi:file-video";
    if (mimeType.startsWith("audio/")) return "mdi:file-music";
    if (mimeType.startsWith("text/")) return "mdi:file-document";
    if (mimeType.includes("pdf")) return "mdi:file-pdf-box";
    if (mimeType.includes("zip") || mimeType.includes("rar") || mimeType.includes("tar")) return "mdi:folder-zip";
    return "mdi:file";
  }

  $effect(() => {
    // 当有上传时切换到上传标签
    if (uploads.size > 0 && activeTab !== "uploads") {
      // 可选：自动切换
    }
  });
</script>

<div 
  class="sync-manager"
  ondrop={handleDrop}
  ondragover={handleDragOver}
  ondragleave={handleDragLeave}
>
  <!-- 拖拽遮罩 -->
  {#if dragOver}
    <div class="drag-overlay">
      <Icon icon="mdi:cloud-upload" width="64" />
      <p>{$t("sync.releaseToUpload")}</p>
    </div>
  {/if}

  <!-- 头部 -->
  <header class="header">
    <div class="header-top">
      <div class="header-left">
        <h2>{$t("sync.title")}</h2>
        <span class="service-status running">
          <span class="dot"></span>
          {$t("sync.running")}
        </span>
      </div>
      <div class="header-right">
        {#if status}
          <span class="stats">
            {status.total_files} 文件 · {formatBytes(status.total_size)}
          </span>
        {/if}
        <Button variant="primary" size="sm" onclick={openFileDialog}>
          <Icon icon="mdi:upload" width="16" />
          {$t("sync.uploadFile")}
        </Button>
      </div>
    </div>

    <div class="tabs-row">
      <Tabs {tabs} bind:activeTab={activeTab} variant="underline" size="sm">
        {#snippet children(tab)}
          <!-- 隐藏的文件输入 -->
          <input
            bind:this={fileInput}
            type="file"
            multiple
            style="display: none"
            onchange={handleFileSelect}
          />

          <!-- 内容区 -->
          <main class="content">
            {#if loading}
              <div class="loading">
                <Spinner size="lg" />
                <p>{$t("sync.loadingFiles")}</p>
              </div>
            {:else if tab === "files"}
              <!-- 文件列表 -->
              {#if files.length === 0}
                <EmptyState
                  icon="mdi:folder-open-outline"
                  title={$t("sync.noSyncFiles")}
                  description={$t("sync.dragOrClickUpload")}
                />
              {:else}
                <div class="file-list">
                  {#each files as file}
                    <div class="file-item">
                      <div class="file-icon">
                        <Icon icon={getFileIcon(file.mime_type)} width="32" />
                      </div>
                      <div class="file-info">
                        <span class="file-name">{file.filename}</span>
                        <span class="file-meta">
                          {formatBytes(file.size)} · {formatTime(file.created_at)}
                        </span>
                      </div>
                      <div class="file-actions">
                        <Button variant="ghost" size="sm" onclick={() => downloadFile(file.id)}>
                          <Icon icon="mdi:download" width="18" />
                        </Button>
                        <Button variant="ghost" size="sm" onclick={() => deleteFile(file.id)}>
                          <Icon icon="mdi:delete" width="18" />
                        </Button>
                      </div>
                    </div>
                  {/each}
                </div>
              {/if}
            {:else if tab === "uploads"}
              <!-- 上传队列 -->
              {#if uploads.size === 0}
                <EmptyState
                  icon="mdi:cloud-upload-outline"
                  title={$t("sync.noUploadTasks")}
                  description={$t("sync.dragOrClickUpload")}
                />
              {:else}
                <div class="upload-list">
                  {#each [...uploads.entries()] as [id, progress]}
                    <div class="upload-item" class:failed={progress.status === "failed"}>
                      <div class="upload-icon">
                        <Icon icon={getFileIcon(progress.file.type)} width="32" />
                      </div>
                      <div class="upload-info">
                        <div class="upload-name-row">
                          <span class="upload-name">{progress.file.name}</span>
                          <span class="upload-status">
                            {#if progress.status === "uploading"}
                              {Math.round(progress.progress * 100)}%
                            {:else if progress.status === "paused"}
                              已暂停
                            {:else if progress.status === "failed"}
                              上传失败
                            {:else if progress.status === "pending"}
                              等待中
                            {:else}
                              已完成
                            {/if}
                          </span>
                        </div>
                        <div class="upload-progress-row">
                          <Progress value={progress.progress * 100} max={100} />
                          <span class="upload-size">
                            {formatBytes(progress.bytesUploaded)} / {formatBytes(progress.bytesTotal)}
                          </span>
                        </div>
                        {#if progress.error}
                          <span class="upload-error">{progress.error}</span>
                        {/if}
                      </div>
                      <div class="upload-actions">
                        {#if progress.status === "uploading"}
                          <Button variant="ghost" size="sm" onclick={() => pauseUpload(id)}>
                            <Icon icon="mdi:pause" width="18" />
                          </Button>
                        {:else if progress.status === "paused"}
                          <Button variant="ghost" size="sm" onclick={() => resumeUpload(id)}>
                            <Icon icon="mdi:play" width="18" />
                          </Button>
                        {/if}
                        <Button variant="ghost" size="sm" onclick={() => cancelUpload(id)}>
                          <Icon icon="mdi:close" width="18" />
                        </Button>
                      </div>
                    </div>
                  {/each}
                </div>
              {/if}
            {/if}
          </main>
        {/snippet}
      </Tabs>
    </div>
  </header>
</div>

<style>
  .sync-manager {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--color-bg);
    position: relative;
  }

  .drag-overlay {
    position: absolute;
    inset: 0;
    background: rgba(59, 130, 246, 0.1);
    border: 2px dashed var(--color-primary);
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    z-index: 100;
    color: var(--color-primary);
    font-size: 1.2rem;
    gap: 1rem;
  }

  .header {
    padding: 1rem 1.5rem;
    border-bottom: 1px solid var(--color-border);
  }

  .header-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .header-left h2 {
    margin: 0;
    font-size: 1.25rem;
    font-weight: 600;
  }

  .service-status {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }

  .service-status .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #9ca3af;
  }

  .service-status.running .dot {
    background: #22c55e;
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .stats {
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }

  .tabs-row {
    margin-top: 0.5rem;
  }

  .content {
    flex: 1;
    overflow-y: auto;
    padding: 1rem 1.5rem;
  }

  .loading {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 1rem;
    color: var(--color-text-muted);
  }

  /* 文件列表 */
  .file-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .file-item {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.75rem 1rem;
    background: var(--color-bg-secondary);
    border-radius: 8px;
    transition: background 0.15s;
  }

  .file-item:hover {
    background: var(--color-bg-hover);
  }

  .file-icon {
    color: var(--color-primary);
  }

  .file-info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .file-name {
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .file-meta {
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }

  .file-actions {
    display: flex;
    gap: 0.25rem;
  }

  /* 上传列表 */
  .upload-list {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .upload-item {
    display: flex;
    align-items: flex-start;
    gap: 1rem;
    padding: 1rem;
    background: var(--color-bg-secondary);
    border-radius: 8px;
  }

  .upload-item.failed {
    background: rgba(239, 68, 68, 0.1);
  }

  .upload-icon {
    color: var(--color-primary);
    padding-top: 0.25rem;
  }

  .upload-info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .upload-name-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
  }

  .upload-name {
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .upload-status {
    font-size: 0.875rem;
    color: var(--color-text-muted);
    flex-shrink: 0;
  }

  .upload-progress-row {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .upload-progress-row :global(.progress) {
    flex: 1;
  }

  .upload-size {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    flex-shrink: 0;
    min-width: 120px;
    text-align: right;
  }

  .upload-error {
    font-size: 0.75rem;
    color: var(--color-error);
  }

  .upload-actions {
    display: flex;
    gap: 0.25rem;
    flex-shrink: 0;
  }
</style>
