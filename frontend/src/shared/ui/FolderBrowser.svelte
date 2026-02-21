<script lang="ts">
  import Icon from "@iconify/svelte";
  import { Button, Modal, Spinner } from "$shared/ui";
  import { fileService, type FileInfo } from "$shared/services/files";
  import { t } from "$lib/i18n";

  interface Props {
    open: boolean;
    title?: string;
    initialPath?: string;
    onConfirm: (path: string) => void;
    onClose: () => void;
  }

  let { 
    open = $bindable(), 
    title = $t('files.selectFolder'),
    initialPath = "/",
    onConfirm,
    onClose 
  }: Props = $props();

  let currentPath = $state(initialPath);
  let files = $state<FileInfo[]>([]);
  let loading = $state(false);
  let error = $state("");

  async function loadFiles(path: string) {
    loading = true;
    error = "";
    try {
      const response = await fileService.list(path, false);
      if (response.data?.content) {
        // 只显示目录
        files = response.data.content.filter((f: FileInfo) => f.is_dir);
        currentPath = path;
      }
    } catch (e) {
      error = e instanceof Error ? e.message : $t('common.loadFailed');
      console.error("Failed to load directory:", e);
    } finally {
      loading = false;
    }
  }

  function navigate(path: string) {
    loadFiles(path);
  }

  function goUp() {
    const parts = currentPath.split("/").filter(Boolean);
    parts.pop();
    navigate("/" + parts.join("/"));
  }

  function enterDir(file: FileInfo) {
    navigate(file.path);
  }

  function getBreadcrumbs() {
    const parts = currentPath.split("/").filter(Boolean);
    const crumbs: { label: string; path: string }[] = [{ label: $t('files.rootDir'), path: "/" }];
    let accPath = "";
    for (const part of parts) {
      accPath += "/" + part;
      crumbs.push({ label: part, path: accPath });
    }
    return crumbs;
  }

  function confirm() {
    onConfirm(currentPath);
    open = false;
  }

  function close() {
    onClose();
    open = false;
  }

  $effect(() => {
    if (open) {
      loadFiles(initialPath || "/");
    }
  });
</script>

<Modal {open} {title} onclose={close} size="md">
  <div class="folder-browser">
    <!-- 导航栏 -->
    <div class="browser-nav">
      <button class="nav-btn" onclick={goUp} disabled={currentPath === "/"}>
        <Icon icon="mdi:arrow-up" width={20} />
      </button>
      <div class="breadcrumbs">
        {#each getBreadcrumbs() as crumb, i}
          {#if i > 0}<span class="sep">/</span>{/if}
          <button class="crumb" onclick={() => navigate(crumb.path)}>
            {crumb.label}
          </button>
        {/each}
      </div>
    </div>

    <!-- 文件列表 -->
    <div class="browser-list">
      {#if loading}
        <div class="browser-loading">
          <Spinner size="md" />
          <span>{$t('common.loading')}</span>
        </div>
      {:else if error}
        <div class="browser-error">
          <Icon icon="mdi:alert-circle" width={32} />
          <span>{error}</span>
          <Button size="sm" onclick={() => loadFiles(currentPath)}>{$t('files.retry')}</Button>
        </div>
      {:else if files.length === 0}
        <div class="browser-empty">
          <Icon icon="mdi:folder-open-outline" width={48} />
          <span>{$t('files.noSubfolders')}</span>
        </div>
      {:else}
        {#each files as file (file.path)}
          <button class="browser-item" ondblclick={() => enterDir(file)}>
            <Icon icon="mdi:folder" width={20} class="folder-icon" />
            <span class="file-name">{file.name}</span>
          </button>
        {/each}
      {/if}
    </div>

    <!-- 底部 -->
    <div class="browser-footer">
      <div class="selected-path">
        <Icon icon="mdi:folder-marker" width={16} />
        <span>{currentPath}</span>
      </div>
      <div class="browser-actions">
        <Button variant="ghost" onclick={close}>{$t('common.cancel')}</Button>
        <Button variant="primary" onclick={confirm}>{$t('files.selectThisFolder')}</Button>
      </div>
    </div>
  </div>
</Modal>

<style>
  .folder-browser {
    display: flex;
    flex-direction: column;
    min-height: 400px;
    max-height: 500px;
  }

  .browser-nav {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 0;
    border-bottom: 1px solid var(--border-color);
  }

  .nav-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    border: none;
    background: none;
    color: var(--text-secondary);
    cursor: pointer;
    border-radius: 6px;
  }

  .nav-btn:hover:not(:disabled) {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .nav-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .breadcrumbs {
    display: flex;
    align-items: center;
    gap: 4px;
    flex: 1;
    overflow-x: auto;
    font-size: 13px;
  }

  .crumb {
    background: none;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    padding: 4px 8px;
    border-radius: 4px;
    white-space: nowrap;
  }

  .crumb:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .crumb:last-child {
    color: var(--text-primary);
    font-weight: 500;
  }

  .sep {
    color: var(--text-muted);
  }

  .browser-list {
    flex: 1;
    overflow-y: auto;
    padding: 8px 0;
  }

  .browser-loading,
  .browser-error,
  .browser-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    height: 200px;
    color: var(--text-muted);
  }

  .browser-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 10px 12px;
    background: none;
    border: none;
    border-radius: 6px;
    color: var(--text-primary);
    cursor: pointer;
    text-align: left;
  }

  .browser-item:hover {
    background: var(--bg-hover);
  }

  .browser-item :global(.folder-icon) {
    color: var(--color-primary);
  }

  .file-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .browser-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 0 0;
    border-top: 1px solid var(--border-color);
    margin-top: auto;
  }

  .selected-path {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    color: var(--text-secondary);
    background: var(--bg-hover);
    padding: 6px 12px;
    border-radius: 6px;
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .browser-actions {
    display: flex;
    gap: 8px;
  }
</style>
