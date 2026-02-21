<script lang="ts">
  import Icon from "@iconify/svelte";
  import { fileService } from "$shared/services/files";
  import { t } from "$lib/i18n";

  let {
    targetPath = "/",
    onupload,
  }: {
    targetPath?: string;
    onupload?: (files: File[]) => void;
  } = $props();

  type UploadStatus = "pending" | "uploading" | "done" | "error";

  let isDragging = $state(false);
  let uploadQueue = $state<{ file: File; progress: number; status: UploadStatus }[]>([]);
  let showUploadPanel = $state(false);

  function handleDragEnter(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    isDragging = true;
  }

  function handleDragLeave(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    /* 检查是否真的离开了区域 */
    const relatedTarget = e.relatedTarget as Node | null;
    if (!relatedTarget || !e.currentTarget || !(e.currentTarget as Node).contains(relatedTarget)) {
      isDragging = false;
    }
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
  }

  async function handleDrop(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    isDragging = false;

    const files = Array.from(e.dataTransfer?.files || []);
    if (files.length === 0) return;

    onupload?.(files);
    await uploadFiles(files);
  }

  async function uploadFiles(files: File[]) {
    showUploadPanel = true;

    /* 添加到上传队列 */
    const newItems: { file: File; progress: number; status: UploadStatus }[] = files.map(
      (file) => ({
        file,
        progress: 0,
        status: "pending" as UploadStatus,
      }),
    );
    uploadQueue = [...uploadQueue, ...newItems];

    /* 逐个上传 */
    for (const item of newItems) {
      item.status = "uploading";
      uploadQueue = [...uploadQueue];

      try {
        await simulateUpload(item);
        item.status = "done";
        item.progress = 100;
      } catch {
        item.status = "error";
      }
      uploadQueue = [...uploadQueue];
    }
  }

  /* 模拟上传进度 (实际应调用 API) */
  async function simulateUpload(item: (typeof uploadQueue)[0]): Promise<void> {
    return new Promise((resolve) => {
      const interval = setInterval(() => {
        item.progress = Math.min(item.progress + Math.random() * 20, 99);
        uploadQueue = [...uploadQueue];
        if (item.progress >= 99) {
          clearInterval(interval);
          item.progress = 100;
          resolve();
        }
      }, 200);
    });
  }

  function clearCompleted() {
    uploadQueue = uploadQueue.filter((item) => item.status !== "done");
    if (uploadQueue.length === 0) {
      showUploadPanel = false;
    }
  }

  function formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
    return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
  }

  let completedCount = $derived(uploadQueue.filter((item) => item.status === "done").length);
  let totalCount = $derived(uploadQueue.length);
</script>

<!-- 拖拽区域覆盖层 -->
{#if isDragging}
  <div
    class="drop-overlay"
    ondragenter={handleDragEnter}
    ondragleave={handleDragLeave}
    ondragover={handleDragOver}
    ondrop={handleDrop}
    role="region"
    aria-label={$t('upload.dragArea')}
  >
    <div class="drop-indicator">
      <Icon icon="mdi:cloud-upload" width="64" />
      <h3>{$t('upload.releaseToUpload')}</h3>
      <p>{$t('upload.uploadTo', { values: { path: targetPath } })}</p>
    </div>
  </div>
{/if}

<!-- 上传进度面板 -->
{#if showUploadPanel && uploadQueue.length > 0}
  <div class="upload-panel">
    <div class="panel-header">
      <span class="title">
        {$t('upload.uploadTasks', { values: { completed: completedCount, total: totalCount } })}
      </span>
      <div class="header-actions">
        {#if completedCount > 0}
          <button class="clear-btn" onclick={clearCompleted}>{$t('common.clearCompleted')}</button>
        {/if}
        <button class="close-btn" onclick={() => (showUploadPanel = false)}>
          <Icon icon="mdi:chevron-down" width="20" />
        </button>
      </div>
    </div>

    <div class="upload-list">
      {#each uploadQueue as item (item.file.name + item.file.lastModified)}
        <div
          class="upload-item"
          class:done={item.status === "done"}
          class:error={item.status === "error"}
        >
          <div class="file-icon">
            <Icon icon="mdi:file-document-outline" width="24" />
          </div>
          <div class="file-info">
            <span class="file-name">{item.file.name}</span>
            <span class="file-size">{formatFileSize(item.file.size)}</span>
          </div>
          <div class="progress-wrapper">
            {#if item.status === "uploading"}
              <div class="progress-bar">
                <div class="progress-fill" style="width: {item.progress}%"></div>
              </div>
              <span class="progress-text">{Math.round(item.progress)}%</span>
            {:else if item.status === "done"}
              <Icon icon="mdi:check-circle" width="20" class="status-icon done" />
            {:else if item.status === "error"}
              <Icon icon="mdi:alert-circle" width="20" class="status-icon error" />
            {:else}
              <span class="status-text">{$t('common.waiting')}</span>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  </div>
{/if}

<style>
  .drop-overlay {
    position: fixed;
    inset: 0;
    background: rgba(74, 144, 217, 0.15);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10002;
    animation: fadeIn 0.2s ease-out;
  }

  .drop-indicator {
    text-align: center;
    padding: 48px 64px;
    background: rgba(255, 255, 255, 0.95);
    border-radius: 24px;
    border: 3px dashed var(--color-primary, #4a90d9);
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.2);
    color: var(--color-primary, #4a90d9);

    h3 {
      margin: 16px 0 8px;
      font-size: 24px;
      color: var(--text-primary, #333);
    }

    p {
      margin: 0;
      font-size: 14px;
      color: var(--text-muted, #888);
    }
  }

  .upload-panel {
    position: fixed;
    bottom: 60px;
    right: 16px;
    width: 360px;
    background: var(--bg-card, white);
    border-radius: 12px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
    z-index: 9998;
    animation: slideUp 0.2s ease-out;
    overflow: hidden;
  }

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    background: var(--bg-tertiary, #f5f5f5);
    border-bottom: 1px solid var(--border-color, #e0e0e0);
  }

  .title {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary, #333);
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .clear-btn {
    font-size: 12px;
    color: var(--color-primary, #4a90d9);
    background: none;
    border: none;
    cursor: pointer;

    &:hover {
      text-decoration: underline;
    }
  }

  .close-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    background: none;
    border: none;
    border-radius: 6px;
    color: var(--text-muted, #888);
    cursor: pointer;

    &:hover {
      background: var(--bg-hover, #e0e0e0);
    }
  }

  .upload-list {
    max-height: 300px;
    overflow-y: auto;
  }

  .upload-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-color, #f0f0f0);

    &:last-child {
      border-bottom: none;
    }

    &.done {
      opacity: 0.6;
    }
  }

  .file-icon {
    color: var(--text-muted, #888);
  }

  .file-info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .file-name {
    font-size: 13px;
    color: var(--text-primary, #333);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .file-size {
    font-size: 11px;
    color: var(--text-muted, #888);
  }

  .progress-wrapper {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 80px;
    justify-content: flex-end;
  }

  .progress-bar {
    width: 60px;
    height: 4px;
    background: var(--bg-tertiary, #e0e0e0);
    border-radius: 2px;
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: var(--color-primary, #4a90d9);
    border-radius: 2px;
    transition: width 0.2s;
  }

  .progress-text {
    font-size: 11px;
    color: var(--text-muted, #888);
    min-width: 32px;
    text-align: right;
  }

  .status-text {
    font-size: 12px;
    color: var(--text-muted, #888);
  }

  :global(.status-icon.done) {
    color: #52c41a;
  }

  :global(.status-icon.error) {
    color: #ff4d4f;
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  @keyframes slideUp {
    from {
      opacity: 0;
      transform: translateY(20px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
