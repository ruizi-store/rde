<script lang="ts">
  import { t } from "svelte-i18n";
  import { sshService, type SFTPFile } from "$shared/services/ssh";
  import Icon from "@iconify/svelte";
  import TransferQueue from "./TransferQueue.svelte";

  interface Props {
    sessionId: string;
    connectionName: string;
    onClose: () => void;
  }

  let { sessionId, connectionName, onClose }: Props = $props();

  // 状态
  let currentPath = $state("/");
  let files = $state<SFTPFile[]>([]);
  let loading = $state(false);
  let error = $state("");
  let selectedPaths = $state<Set<string>>(new Set());
  let viewMode = $state<"grid" | "list">("list");
  let showHidden = $state(false);
  let sortBy = $state<"name" | "size" | "modified">("name");
  let sortAsc = $state(true);

  // 弹窗状态
  let showNewFolderModal = $state(false);
  let newFolderName = $state("");
  let showRenameModal = $state(false);
  let renameTarget = $state<SFTPFile | null>(null);
  let newName = $state("");
  let showDeleteConfirm = $state(false);

  // 上传状态
  let isDragOver = $state(false);
  let uploading = $state(false);
  let uploadProgress = $state(0);

  // 下载状态
  let showDownloadModal = $state(false);
  let downloadLocalDir = $state("/tmp");
  let downloading = $state(false);

  // 传输队列
  let showTransferQueue = $state(false);

  // 通知
  let notification = $state<{ type: "success" | "error" | "info"; message: string } | null>(null);

  // 历史导航
  let history = $state<string[]>([]);
  let historyIndex = $state(-1);

  // 派生状态
  let sortedFiles = $derived.by(() => {
    let result = [...files];

    // 过滤隐藏文件
    if (!showHidden) {
      result = result.filter(f => !f.name.startsWith("."));
    }

    // 排序：目录优先
    result.sort((a, b) => {
      if (a.is_dir !== b.is_dir) {
        return a.is_dir ? -1 : 1;
      }

      let cmp = 0;
      switch (sortBy) {
        case "name":
          cmp = a.name.localeCompare(b.name);
          break;
        case "size":
          cmp = a.size - b.size;
          break;
        case "modified":
          cmp = new Date(a.mod_time).getTime() - new Date(b.mod_time).getTime();
          break;
      }
      return sortAsc ? cmp : -cmp;
    });

    return result;
  });

  let canGoBack = $derived(historyIndex > 0);
  let canGoForward = $derived(historyIndex < history.length - 1);
  let canGoUp = $derived(currentPath !== "/");

  // 初始化
  $effect(() => {
    loadDirectory("/");
  });

  // 加载目录
  async function loadDirectory(path: string, addToHistory = true) {
    loading = true;
    error = "";

    try {
      const response = await sshService.listDir(sessionId, path);
      if (response.success && response.data) {
        files = response.data;
        currentPath = path;

        if (addToHistory) {
          // 清除forward历史
          if (historyIndex < history.length - 1) {
            history = history.slice(0, historyIndex + 1);
          }
          history = [...history, path];
          historyIndex = history.length - 1;
        }

        selectedPaths = new Set();
      } else {
        error = response.message || $t("sftp.loadDirFailed");
      }
    } catch (e: any) {
      error = e.message || $t("sftp.loadDirFailed");
    } finally {
      loading = false;
    }
  }

  // 导航到目录
  function navigateTo(path: string) {
    loadDirectory(path);
  }

  // 返回上一级
  function goUp() {
    if (!canGoUp) return;
    const parts = currentPath.split("/").filter(Boolean);
    parts.pop();
    const parentPath = "/" + parts.join("/");
    loadDirectory(parentPath);
  }

  // 历史导航
  function goBack() {
    if (!canGoBack) return;
    historyIndex--;
    loadDirectory(history[historyIndex], false);
  }

  function goForward() {
    if (!canGoForward) return;
    historyIndex++;
    loadDirectory(history[historyIndex], false);
  }

  // 刷新
  function refresh() {
    loadDirectory(currentPath, false);
  }

  // 点击文件/目录
  function handleItemClick(file: SFTPFile, event: MouseEvent) {
    if (event.ctrlKey || event.metaKey) {
      // 多选
      const newSet = new Set(selectedPaths);
      if (newSet.has(file.path)) {
        newSet.delete(file.path);
      } else {
        newSet.add(file.path);
      }
      selectedPaths = newSet;
    } else {
      selectedPaths = new Set([file.path]);
    }
  }

  // 双击
  function handleItemDoubleClick(file: SFTPFile) {
    if (file.is_dir) {
      navigateTo(file.path);
    }
  }

  // 全选
  function selectAll() {
    selectedPaths = new Set(files.map(f => f.path));
  }

  // 清除选择
  function clearSelection() {
    selectedPaths = new Set();
  }

  // 创建目录
  async function createFolder() {
    if (!newFolderName.trim()) return;

    try {
      const path = currentPath === "/" ? `/${newFolderName}` : `${currentPath}/${newFolderName}`;
      const response = await sshService.mkdir(sessionId, path);
      if (response.success) {
        showNotification("success", $t("sftp.directoryCreated"));
        refresh();
      } else {
        showNotification("error", response.message || $t("sftp.createDirFailed"));
      }
    } catch (e: any) {
      showNotification("error", e.message || $t("sftp.createDirFailed"));
    } finally {
      showNewFolderModal = false;
      newFolderName = "";
    }
  }

  // 重命名
  async function renameFile() {
    if (!renameTarget || !newName.trim()) return;

    try {
      const oldPath = renameTarget.path;
      const parts = oldPath.split("/");
      parts.pop();
      const newPath = [...parts, newName].join("/") || `/${newName}`;

      const response = await sshService.rename(sessionId, oldPath, newPath);
      if (response.success) {
        showNotification("success", $t("sftp.renameSuccess"));
        refresh();
      } else {
        showNotification("error", response.message || $t("sftp.renameFailed"));
      }
    } catch (e: any) {
      showNotification("error", e.message || $t("sftp.renameFailed"));
    } finally {
      showRenameModal = false;
      renameTarget = null;
      newName = "";
    }
  }

  // 删除
  async function deleteSelected() {
    if (selectedPaths.size === 0) return;

    try {
      const response = await sshService.delete(sessionId, Array.from(selectedPaths));
      if (response.success) {
        showNotification("success", $t("sftp.deleted", { values: { count: selectedPaths.size } }));
        refresh();
      } else {
        showNotification("error", response.message || $t("sftp.deleteFailed"));
      }
    } catch (e: any) {
      showNotification("error", e.message || $t("sftp.deleteFailed"));
    } finally {
      showDeleteConfirm = false;
      selectedPaths = new Set();
    }
  }

  // 上传文件
  async function handleUpload(event: Event) {
    const input = event.target as HTMLInputElement;
    if (!input.files || input.files.length === 0) return;

    uploading = true;
    uploadProgress = 0;

    try {
      const response = await sshService.uploadFiles(
        sessionId,
        currentPath,
        Array.from(input.files)
      );
      if (response.success) {
        showNotification("success", $t("sftp.uploaded", { values: { count: input.files.length } }));
        refresh();
      } else {
        showNotification("error", response.message || $t("sftp.uploadFailed"));
      }
    } catch (e: any) {
      showNotification("error", e.message || $t("sftp.uploadFailed"));
    } finally {
      uploading = false;
      input.value = "";
    }
  }

  // 拖拽上传
  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    isDragOver = true;
  }

  function handleDragLeave(e: DragEvent) {
    e.preventDefault();
    isDragOver = false;
  }

  async function handleDrop(e: DragEvent) {
    e.preventDefault();
    isDragOver = false;

    const files = e.dataTransfer?.files;
    if (!files || files.length === 0) return;

    uploading = true;
    try {
      const response = await sshService.uploadFiles(sessionId, currentPath, Array.from(files));
      if (response.success) {
        showNotification("success", $t("sftp.uploaded", { values: { count: files.length } }));
        refresh();
      } else {
        showNotification("error", response.message || $t("sftp.uploadFailed"));
      }
    } catch (e: any) {
      showNotification("error", e.message || $t("sftp.uploadFailed"));
    } finally {
      uploading = false;
    }
  }

  // 下载文件
  async function downloadSelected() {
    if (selectedPaths.size === 0) return;

    downloading = true;
    try {
      const response = await sshService.downloadFiles(
        sessionId,
        Array.from(selectedPaths),
        downloadLocalDir
      );
      if (response.success) {
        showNotification("success", $t("sftp.downloadedTo", { values: { path: downloadLocalDir } }));
      } else {
        showNotification("error", response.message || $t("sftp.downloadFailed"));
      }
    } catch (e: any) {
      showNotification("error", e.message || $t("sftp.downloadFailed"));
    } finally {
      downloading = false;
      showDownloadModal = false;
    }
  }

  // 显示通知
  function showNotification(type: "success" | "error" | "info", message: string) {
    notification = { type, message };
    setTimeout(() => {
      notification = null;
    }, 3000);
  }

  // 格式化文件大小
  function formatSize(bytes: number): string {
    if (bytes === 0) return "-";
    const units = ["B", "KB", "MB", "GB", "TB"];
    let i = 0;
    while (bytes >= 1024 && i < units.length - 1) {
      bytes /= 1024;
      i++;
    }
    return `${bytes.toFixed(1)} ${units[i]}`;
  }

  // 格式化时间
  function formatTime(dateStr: string): string {
    const date = new Date(dateStr);
    return date.toLocaleString();
  }

  // 获取文件图标
  function getFileIcon(file: SFTPFile): string {
    if (file.is_dir) return "mdi:folder";
    
    const ext = file.name.split(".").pop()?.toLowerCase() || "";
    const iconMap: Record<string, string> = {
      // 文档
      txt: "mdi:file-document",
      md: "mdi:language-markdown",
      pdf: "mdi:file-pdf-box",
      doc: "mdi:file-word",
      docx: "mdi:file-word",
      xls: "mdi:file-excel",
      xlsx: "mdi:file-excel",
      ppt: "mdi:file-powerpoint",
      pptx: "mdi:file-powerpoint",
      // 图片
      jpg: "mdi:file-image",
      jpeg: "mdi:file-image",
      png: "mdi:file-image",
      gif: "mdi:file-image",
      svg: "mdi:file-image",
      webp: "mdi:file-image",
      // 视频
      mp4: "mdi:file-video",
      mkv: "mdi:file-video",
      avi: "mdi:file-video",
      mov: "mdi:file-video",
      // 音频
      mp3: "mdi:file-music",
      wav: "mdi:file-music",
      flac: "mdi:file-music",
      // 压缩包
      zip: "mdi:folder-zip",
      tar: "mdi:folder-zip",
      gz: "mdi:folder-zip",
      rar: "mdi:folder-zip",
      "7z": "mdi:folder-zip",
      // 代码
      js: "mdi:language-javascript",
      ts: "mdi:language-typescript",
      py: "mdi:language-python",
      go: "mdi:language-go",
      rs: "mdi:language-rust",
      java: "mdi:language-java",
      c: "mdi:language-c",
      cpp: "mdi:language-cpp",
      h: "mdi:language-c",
      html: "mdi:language-html5",
      css: "mdi:language-css3",
      json: "mdi:code-json",
      xml: "mdi:file-xml-box",
      yaml: "mdi:file-code",
      yml: "mdi:file-code",
      sh: "mdi:bash",
      // 其他
      exe: "mdi:application",
      dmg: "mdi:apple",
      deb: "mdi:debian",
      rpm: "mdi:redhat",
    };
    return iconMap[ext] || "mdi:file";
  }

  // 打开重命名对话框
  function openRenameModal(file: SFTPFile) {
    renameTarget = file;
    newName = file.name;
    showRenameModal = true;
  }

  // 打开下载对话框
  function openDownloadModal() {
    if (selectedPaths.size === 0) return;
    showDownloadModal = true;
  }

  // 键盘快捷键
  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Delete" && selectedPaths.size > 0) {
      showDeleteConfirm = true;
    }
    if (e.ctrlKey && e.key === "a") {
      e.preventDefault();
      selectAll();
    }
    if (e.key === "Escape") {
      clearSelection();
    }
    if (e.key === "F5") {
      e.preventDefault();
      refresh();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="sftp-browser">
  <!-- 标题栏 -->
  <div class="title-bar">
    <div class="title-left">
      <Icon icon="mdi:folder-network" />
      <span>SFTP - {connectionName}</span>
    </div>
    <button class="close-btn" onclick={onClose} title={$t("sftp.close")}>
      <Icon icon="mdi:close" />
    </button>
  </div>

  <!-- 工具栏 -->
  <div class="toolbar">
    <div class="nav-buttons">
      <button onclick={goBack} disabled={!canGoBack} title={$t("sftp.back")}>
        <Icon icon="mdi:arrow-left" />
      </button>
      <button onclick={goForward} disabled={!canGoForward} title={$t("sftp.forward")}>
        <Icon icon="mdi:arrow-right" />
      </button>
      <button onclick={goUp} disabled={!canGoUp} title={$t("sftp.parentDir")}>
        <Icon icon="mdi:arrow-up" />
      </button>
      <button onclick={refresh} title={$t("sftp.refreshF5")}>
        <Icon icon="mdi:refresh" />
      </button>
    </div>

    <div class="path-bar">
      <Icon icon="mdi:folder" />
      <input
        type="text"
        value={currentPath}
        onkeydown={(e) => e.key === "Enter" && navigateTo((e.target as HTMLInputElement).value)}
        onblur={(e) => navigateTo((e.target as HTMLInputElement).value)}
      />
    </div>

    <div class="actions">
      <button onclick={() => showNewFolderModal = true} title={$t("sftp.newFolder")}>
        <Icon icon="mdi:folder-plus" />
      </button>
      <label class="upload-btn" title={$t("sftp.uploadFile")}>
        <Icon icon="mdi:upload" />
        <input type="file" multiple onchange={handleUpload} hidden />
      </label>
      <button onclick={openDownloadModal} disabled={selectedPaths.size === 0} title={$t("sftp.download")}>
        <Icon icon="mdi:download" />
      </button>
      <button onclick={() => showDeleteConfirm = true} disabled={selectedPaths.size === 0} title={$t("sftp.delete")} class="danger">
        <Icon icon="mdi:delete" />
      </button>
      <div class="divider"></div>
      <button onclick={() => showHidden = !showHidden} class:active={showHidden} title={$t("sftp.showHidden")}>
        <Icon icon={showHidden ? "mdi:eye" : "mdi:eye-off"} />
      </button>
      <button onclick={() => viewMode = viewMode === "list" ? "grid" : "list"} title={$t("sftp.toggleView")}>
        <Icon icon={viewMode === "list" ? "mdi:view-grid" : "mdi:view-list"} />
      </button>
      <button onclick={() => showTransferQueue = !showTransferQueue} class:active={showTransferQueue} title={$t("sftp.transferQueue")}>
        <Icon icon="mdi:swap-vertical" />
      </button>
    </div>
  </div>

  <!-- 文件列表 -->
  <div
    class="file-list"
    class:drag-over={isDragOver}
    ondragover={handleDragOver}
    ondragleave={handleDragLeave}
    ondrop={handleDrop}
    role="listbox"
    tabindex="-1"
  >
    {#if loading}
      <div class="loading">
        <Icon icon="mdi:loading" class="spin" />
        <span>{$t("sftp.loading")}</span>
      </div>
    {:else if error}
      <div class="error">
        <Icon icon="mdi:alert-circle" />
        <span>{error}</span>
        <button onclick={refresh}>{$t("sftp.retry")}</button>
      </div>
    {:else if sortedFiles.length === 0}
      <div class="empty">
        <Icon icon="mdi:folder-open" />
        <span>{$t("sftp.directoryEmpty")}</span>
      </div>
    {:else}
      {#if viewMode === "list"}
        <div class="list-header">
          <button class="col-name" onclick={() => { sortBy = "name"; sortAsc = sortBy === "name" ? !sortAsc : true; }}>
            {$t("sftp.name")}
            {#if sortBy === "name"}
              <Icon icon={sortAsc ? "mdi:arrow-up" : "mdi:arrow-down"} />
            {/if}
          </button>
          <button class="col-size" onclick={() => { sortBy = "size"; sortAsc = sortBy === "size" ? !sortAsc : true; }}>
            {$t("sftp.size")}
            {#if sortBy === "size"}
              <Icon icon={sortAsc ? "mdi:arrow-up" : "mdi:arrow-down"} />
            {/if}
          </button>
          <button class="col-modified" onclick={() => { sortBy = "modified"; sortAsc = sortBy === "modified" ? !sortAsc : true; }}>
            {$t("sftp.modifiedTime")}
            {#if sortBy === "modified"}
              <Icon icon={sortAsc ? "mdi:arrow-up" : "mdi:arrow-down"} />
            {/if}
          </button>
          <span class="col-mode">{$t("sftp.permissions")}</span>
        </div>
        <div class="list-body">
          {#each sortedFiles as file (file.path)}
            <!-- svelte-ignore a11y_no_static_element_interactions -->
            <div
              class="list-item"
              class:selected={selectedPaths.has(file.path)}
              onclick={(e) => handleItemClick(file, e)}
              ondblclick={() => handleItemDoubleClick(file)}
              oncontextmenu={(e) => { e.preventDefault(); handleItemClick(file, e); }}
              role="option"
              aria-selected={selectedPaths.has(file.path)}
            >
              <div class="col-name">
                <Icon icon={getFileIcon(file)} class={file.is_dir ? "folder" : "file"} />
                <span class="name">{file.name}</span>
              </div>
              <span class="col-size">{file.is_dir ? "-" : formatSize(file.size)}</span>
              <span class="col-modified">{formatTime(file.mod_time)}</span>
              <span class="col-mode">{file.mode}</span>
              <div class="item-actions">
                <button onclick={(e) => { e.stopPropagation(); openRenameModal(file); }} title={$t("sftp.rename")}>
                  <Icon icon="mdi:pencil" />
                </button>
              </div>
            </div>
          {/each}
        </div>
      {:else}
        <div class="grid-body">
          {#each sortedFiles as file (file.path)}
            <!-- svelte-ignore a11y_no_static_element_interactions -->
            <div
              class="grid-item"
              class:selected={selectedPaths.has(file.path)}
              onclick={(e) => handleItemClick(file, e)}
              ondblclick={() => handleItemDoubleClick(file)}
              role="option"
              aria-selected={selectedPaths.has(file.path)}
            >
              <Icon icon={getFileIcon(file)} class={file.is_dir ? "folder" : "file"} />
              <span class="name" title={file.name}>{file.name}</span>
            </div>
          {/each}
        </div>
      {/if}
    {/if}

    {#if isDragOver}
      <div class="drop-overlay">
        <Icon icon="mdi:upload" />
        <span>{$t("sftp.dropToUpload")}</span>
      </div>
    {/if}
  </div>

  <!-- 状态栏 -->
  <div class="status-bar">
    <span>{sortedFiles.length} {$t("sftp.items")}</span>
    {#if selectedPaths.size > 0}
      <span>{$t("sftp.selectedCount", { values: { count: selectedPaths.size } })}</span>
    {/if}
    {#if uploading}
      <span class="uploading"><Icon icon="mdi:upload" class="spin" /> {$t("sftp.uploading")}</span>
    {/if}
  </div>

  <!-- 通知 -->
  {#if notification}
    <div class="notification {notification.type}">
      <Icon icon={notification.type === "success" ? "mdi:check-circle" : notification.type === "error" ? "mdi:alert-circle" : "mdi:information"} />
      <span>{notification.message}</span>
    </div>
  {/if}

  <!-- 传输队列浮窗 -->
  {#if showTransferQueue}
    <div class="transfer-queue-overlay">
      <TransferQueue sessionId={sessionId} onClose={() => showTransferQueue = false} />
    </div>
  {/if}

  <!-- 新建目录对话框 -->
  {#if showNewFolderModal}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="modal-overlay" onclick={() => showNewFolderModal = false}>
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="modal" onclick={(e) => e.stopPropagation()}>
        <h3>{$t("sftp.newFolder")}</h3>
        <input
          type="text"
          bind:value={newFolderName}
          placeholder={$t("sftp.directoryName")}
          onkeydown={(e) => e.key === "Enter" && createFolder()}
        />
        <div class="modal-actions">
          <button onclick={() => showNewFolderModal = false}>{$t("sftp.cancel")}</button>
          <button class="primary" onclick={createFolder} disabled={!newFolderName.trim()}>{$t("sftp.create")}</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- 重命名对话框 -->
  {#if showRenameModal}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="modal-overlay" onclick={() => showRenameModal = false}>
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="modal" onclick={(e) => e.stopPropagation()}>
        <h3>{$t("sftp.rename")}</h3>
        <input
          type="text"
          bind:value={newName}
          placeholder={$t("sftp.newName")}
          onkeydown={(e) => e.key === "Enter" && renameFile()}
        />
        <div class="modal-actions">
          <button onclick={() => showRenameModal = false}>{$t("sftp.cancel")}</button>
          <button class="primary" onclick={renameFile} disabled={!newName.trim()}>{$t("common.ok")}</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- 删除确认对话框 -->
  {#if showDeleteConfirm}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="modal-overlay" onclick={() => showDeleteConfirm = false}>
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="modal" onclick={(e) => e.stopPropagation()}>
        <h3>{$t("sftp.confirmDelete")}</h3>
        <p>{$t("sftp.deleteConfirmMsg", { values: { count: selectedPaths.size } })}</p>
        <div class="modal-actions">
          <button onclick={() => showDeleteConfirm = false}>{$t("sftp.cancel")}</button>
          <button class="danger" onclick={deleteSelected}>{$t("sftp.delete")}</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- 下载对话框 -->
  {#if showDownloadModal}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="modal-overlay" onclick={() => showDownloadModal = false}>
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="modal" onclick={(e) => e.stopPropagation()}>
        <h3>{$t("sftp.downloadToLocal")}</h3>
        <p>{$t("sftp.selectedFiles", { values: { count: selectedPaths.size } })}</p>
        <label>
          {$t("sftp.localDirectory")}
          <input type="text" bind:value={downloadLocalDir} placeholder="/tmp" />
        </label>
        <div class="modal-actions">
          <button onclick={() => showDownloadModal = false}>{$t("sftp.cancel")}</button>
          <button class="primary" onclick={downloadSelected} disabled={downloading}>
            {downloading ? $t("sftp.downloading") : $t("sftp.download")}
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .sftp-browser {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: #1e1e2e;
    color: #cdd6f4;
    font-size: 13px;
    position: relative;
  }

  .title-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    background: #181825;
    border-bottom: 1px solid #313244;

    .title-left {
      display: flex;
      align-items: center;
      gap: 8px;
      font-weight: 500;
    }

    .close-btn {
      width: 28px;
      height: 28px;
      padding: 0;
      background: transparent;
      border: none;
      color: #a6adc8;
      cursor: pointer;
      border-radius: 4px;

      &:hover {
        color: #f38ba8;
        background: rgba(243, 139, 168, 0.1);
      }
    }
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: #1e1e2e;
    border-bottom: 1px solid #313244;

    button, .upload-btn {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 32px;
      height: 32px;
      padding: 0;
      background: transparent;
      border: none;
      color: #a6adc8;
      cursor: pointer;
      border-radius: 6px;
      transition: all 0.15s;

      &:hover:not(:disabled) {
        color: #cdd6f4;
        background: #313244;
      }

      &:disabled {
        opacity: 0.3;
        cursor: not-allowed;
      }

      &.active {
        color: #89b4fa;
        background: rgba(137, 180, 250, 0.1);
      }

      &.danger:hover:not(:disabled) {
        color: #f38ba8;
        background: rgba(243, 139, 168, 0.1);
      }
    }
  }

  .nav-buttons {
    display: flex;
    gap: 2px;
  }

  .path-bar {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 12px;
    height: 32px;
    background: #313244;
    border-radius: 6px;
    color: #6c7086;

    input {
      flex: 1;
      background: transparent;
      border: none;
      color: #cdd6f4;
      font-size: 13px;
      font-family: monospace;
      outline: none;
    }
  }

  .actions {
    display: flex;
    gap: 2px;

    .divider {
      width: 1px;
      height: 20px;
      background: #45475a;
      margin: 0 6px;
    }
  }

  .file-list {
    flex: 1;
    overflow: auto;
    position: relative;

    &.drag-over {
      background: rgba(137, 180, 250, 0.05);
    }
  }

  .loading, .error, .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    height: 200px;
    color: #6c7086;

    button {
      padding: 8px 16px;
      background: #89b4fa;
      color: #1e1e2e;
      border: none;
      border-radius: 6px;
      cursor: pointer;

      &:hover {
        background: #74a8fc;
      }
    }
  }

  .error {
    color: #f38ba8;
  }

  .list-header {
    display: flex;
    align-items: center;
    padding: 8px 12px;
    background: #181825;
    border-bottom: 1px solid #313244;
    font-size: 12px;
    color: #6c7086;
    position: sticky;
    top: 0;
    z-index: 1;

    button {
      display: flex;
      align-items: center;
      gap: 4px;
      background: transparent;
      border: none;
      color: inherit;
      cursor: pointer;
      padding: 4px 8px;
      border-radius: 4px;

      &:hover {
        color: #cdd6f4;
        background: #313244;
      }
    }
  }

  .col-name {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  .col-size {
    width: 80px;
    text-align: right;
  }

  .col-modified {
    width: 160px;
  }

  .col-mode {
    width: 80px;
    font-family: monospace;
  }

  .list-item {
    display: flex;
    align-items: center;
    padding: 8px 12px;
    cursor: pointer;
    transition: background 0.1s;

    &:hover {
      background: rgba(255, 255, 255, 0.03);
    }

    &.selected {
      background: rgba(137, 180, 250, 0.15);
    }

    .name {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    :global(.folder) {
      color: #f9e2af;
    }

    :global(.file) {
      color: #89b4fa;
    }

    .item-actions {
      display: flex;
      gap: 4px;
      opacity: 0;
      transition: opacity 0.15s;

      button {
        width: 24px;
        height: 24px;
        padding: 0;
        background: transparent;
        border: none;
        color: #a6adc8;
        cursor: pointer;
        border-radius: 4px;

        &:hover {
          color: #cdd6f4;
          background: #45475a;
        }
      }
    }

    &:hover .item-actions {
      opacity: 1;
    }
  }

  .grid-body {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
    gap: 8px;
    padding: 12px;
  }

  .grid-item {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 12px 8px;
    border-radius: 8px;
    cursor: pointer;
    transition: background 0.1s;

    &:hover {
      background: rgba(255, 255, 255, 0.03);
    }

    &.selected {
      background: rgba(137, 180, 250, 0.15);
    }

    :global(svg) {
      font-size: 40px;
    }

    :global(.folder) {
      color: #f9e2af;
    }

    :global(.file) {
      color: #89b4fa;
    }

    .name {
      max-width: 100%;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      text-align: center;
      font-size: 12px;
    }
  }

  .drop-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    background: rgba(137, 180, 250, 0.1);
    border: 2px dashed #89b4fa;
    border-radius: 8px;
    margin: 12px;
    color: #89b4fa;
    font-size: 16px;
    pointer-events: none;

    :global(svg) {
      font-size: 48px;
    }
  }

  .status-bar {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 6px 12px;
    background: #181825;
    border-top: 1px solid #313244;
    font-size: 12px;
    color: #6c7086;

    .uploading {
      display: flex;
      align-items: center;
      gap: 4px;
      color: #89b4fa;
    }
  }

  .notification {
    position: absolute;
    bottom: 48px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 16px;
    border-radius: 8px;
    font-size: 13px;
    animation: slideUp 0.2s ease-out;

    &.success {
      background: rgba(166, 227, 161, 0.15);
      color: #a6e3a1;
    }

    &.error {
      background: rgba(243, 139, 168, 0.15);
      color: #f38ba8;
    }

    &.info {
      background: rgba(137, 180, 250, 0.15);
      color: #89b4fa;
    }
  }

  @keyframes slideUp {
    from {
      opacity: 0;
      transform: translateX(-50%) translateY(10px);
    }
    to {
      opacity: 1;
      transform: translateX(-50%) translateY(0);
    }
  }

  .modal-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(0, 0, 0, 0.5);
    z-index: 100;
  }

  .modal {
    width: 320px;
    background: #1e1e2e;
    border: 1px solid #45475a;
    border-radius: 12px;
    padding: 20px;

    h3 {
      margin: 0 0 16px;
      font-size: 16px;
      font-weight: 600;
    }

    p {
      margin: 0 0 16px;
      color: #a6adc8;
    }

    label {
      display: block;
      margin-bottom: 12px;
      font-size: 12px;
      color: #a6adc8;
    }

    input {
      width: 100%;
      padding: 10px 12px;
      margin-top: 6px;
      background: #313244;
      border: 1px solid #45475a;
      border-radius: 6px;
      color: #cdd6f4;
      font-size: 13px;
      box-sizing: border-box;
      outline: none;

      &:focus {
        border-color: #89b4fa;
      }
    }
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 20px;

    button {
      padding: 8px 16px;
      border-radius: 6px;
      font-size: 13px;
      cursor: pointer;
      transition: all 0.15s;
      background: transparent;
      border: 1px solid #45475a;
      color: #a6adc8;

      &:hover:not(:disabled) {
        color: #cdd6f4;
        border-color: #6c7086;
      }

      &:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }

      &.primary {
        background: #89b4fa;
        color: #1e1e2e;
        border: none;

        &:hover:not(:disabled) {
          background: #74a8fc;
        }
      }

      &.danger {
        background: #f38ba8;
        color: #1e1e2e;
        border: none;

        &:hover:not(:disabled) {
          background: #f17497;
        }
      }
    }
  }

  :global(.spin) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  /* 传输队列浮窗 */
  .transfer-queue-overlay {
    position: absolute;
    top: 60px;
    right: 12px;
    z-index: 100;
  }
</style>
