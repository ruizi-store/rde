<script lang="ts">
  import { t } from "svelte-i18n";
  import { Button, Input, Switch, Modal, Spinner, FolderBrowser } from "$shared/ui";
  import Icon from "@iconify/svelte";
  import { photoService, type Library } from "$shared/services/photos";

  interface Props {
    onClose: () => void;
  }

  let { onClose }: Props = $props();

  // 设置状态
  let loading = $state(true);
  let saving = $state(false);
  let libraries = $state<Library[]>([]);
  let defaultLibraryId = $state("");
  let showDeleteConfirm = $state(false);
  let libraryToDelete = $state<Library | null>(null);

  // 设置选项
  let settings = $state({
    autoScan: true,
    scanInterval: 60,
    thumbnailSize: "medium" as "small" | "medium" | "large",
    showHiddenFiles: false,
    sortBy: "date_desc" as "date_desc" | "date_asc" | "name_asc" | "name_desc",
    groupBy: "day" as "day" | "month" | "year",
  });

  // 添加图库弹窗
  let showAddLibrary = $state(false);
  let showFolderBrowser = $state(false);
  let newLibraryName = $state("");
  let newLibraryPath = $state("");
  let addingLibrary = $state(false);
  let addError = $state("");

  async function loadSettings() {
    loading = true;
    try {
      libraries = await photoService.listLibraries();
      // 加载保存的设置
      const savedSettings = localStorage.getItem("photos_settings");
      if (savedSettings) {
        const parsed = JSON.parse(savedSettings);
        settings = { ...settings, ...parsed };
        defaultLibraryId = parsed.defaultLibraryId || "";
      }
      // 如果没有默认图库，使用第一个
      if (!defaultLibraryId && libraries.length > 0) {
        defaultLibraryId = libraries[0].id;
      }
    } catch (e) {
      console.error("Failed to load settings", e);
    } finally {
      loading = false;
    }
  }

  function saveSettings() {
    const toSave = {
      ...settings,
      defaultLibraryId,
    };
    localStorage.setItem("photos_settings", JSON.stringify(toSave));
  }

  async function addLibrary() {
    if (!newLibraryPath) {
      addError = $t("photos.enterLibraryPath");
      return;
    }

    addingLibrary = true;
    addError = "";

    try {
      await photoService.createLibrary(
        newLibraryName || $t("photos.newLibrary"),
        newLibraryPath,
        settings.autoScan
      );
      libraries = await photoService.listLibraries();
      showAddLibrary = false;
      newLibraryName = "";
      newLibraryPath = "";
    } catch (e) {
      addError = e instanceof Error ? e.message : $t("photos.createFailed");
    } finally {
      addingLibrary = false;
    }
  }

  function handleFolderSelect(path: string) {
    newLibraryPath = path;
    showFolderBrowser = false;
  }

  async function deleteLibrary() {
    if (!libraryToDelete) return;

    try {
      await photoService.deleteLibrary(libraryToDelete.id);
      libraries = libraries.filter(l => l.id !== libraryToDelete!.id);
      if (defaultLibraryId === libraryToDelete.id) {
        defaultLibraryId = libraries[0]?.id || "";
      }
    } catch (e) {
      console.error("Failed to delete library", e);
    } finally {
      showDeleteConfirm = false;
      libraryToDelete = null;
    }
  }

  async function scanLibrary(id: string) {
    try {
      await photoService.scanLibrary(id);
    } catch (e) {
      console.error("Failed to trigger scan", e);
    }
  }

  $effect(() => {
    loadSettings();
  });

  $effect(() => {
    // 自动保存设置
    if (!loading) {
      saveSettings();
    }
  });
</script>

<div class="settings-container">
  <header class="settings-header">
    <h2>
      <Icon icon="mdi:cog" width={24} />
      {$t("photos.settings.title")}
    </h2>
    <button class="close-btn" onclick={onClose}>
      <Icon icon="mdi:close" width={20} />
    </button>
  </header>

  {#if loading}
    <div class="loading-state">
      <Spinner size="lg" />
    </div>
  {:else}
    <div class="settings-content">
      <!-- 图库管理 -->
      <section class="settings-section">
        <h3>
          <Icon icon="mdi:folder-multiple-image" width={20} />
          {$t("photos.settings.libraryManagement")}
        </h3>

        <div class="library-list">
          {#each libraries as lib}
            <div class="library-item">
              <div class="library-info">
                <span class="library-name">{lib.name}</span>
                <span class="library-path">{lib.path}</span>
                <span class="library-stats">
                  {lib.photo_count} {$t("photos.settings.photos")} · {lib.video_count} {$t("photos.settings.videos")}
                </span>
              </div>
              <div class="library-actions">
                {#if lib.id === defaultLibraryId}
                  <span class="default-badge">{$t("photos.settings.default")}</span>
                {:else}
                  <Button size="sm" variant="ghost" onclick={() => { defaultLibraryId = lib.id; saveSettings(); }}>
                    {$t("photos.settings.setDefault")}
                  </Button>
                {/if}
                <Button size="sm" variant="ghost" onclick={() => scanLibrary(lib.id)}>
                  <Icon icon="mdi:refresh" width={16} />
                </Button>
                <Button size="sm" variant="ghost" onclick={() => { libraryToDelete = lib; showDeleteConfirm = true; }}>
                  <Icon icon="mdi:delete" width={16} />
                </Button>
              </div>
            </div>
          {/each}

          {#if libraries.length === 0}
            <div class="empty-libraries">
              <Icon icon="mdi:folder-off" width={32} />
              <span>{$t("photos.settings.noLibraries")}</span>
            </div>
          {/if}
        </div>

        <Button variant="outline" onclick={() => showAddLibrary = true}>
          <Icon icon="mdi:plus" width={18} />
          {$t("photos.addLibrary")}
        </Button>
      </section>

      <!-- 扫描设置 -->
      <section class="settings-section">
        <h3>
          <Icon icon="mdi:magnify-scan" width={20} />
          {$t("photos.settings.scanSettings")}
        </h3>

        <div class="setting-item">
          <div class="setting-info">
            <span class="setting-label">{$t("photos.settings.autoScan")}</span>
            <span class="setting-desc">{$t("photos.settings.autoScanDesc")}</span>
          </div>
          <Switch bind:checked={settings.autoScan} />
        </div>

        {#if settings.autoScan}
          <div class="setting-item">
            <div class="setting-info">
              <span class="setting-label">{$t("photos.settings.scanInterval")}</span>
              <span class="setting-desc">{$t("photos.settings.scanIntervalDesc")}</span>
            </div>
            <select bind:value={settings.scanInterval} class="setting-select">
              <option value={30}>30 分钟</option>
              <option value={60}>1 小时</option>
              <option value={180}>3 小时</option>
              <option value={360}>6 小时</option>
              <option value={720}>12 小时</option>
              <option value={1440}>24 小时</option>
            </select>
          </div>
        {/if}

        <div class="setting-item">
          <div class="setting-info">
            <span class="setting-label">{$t("photos.settings.showHiddenFiles")}</span>
            <span class="setting-desc">{$t("photos.settings.showHiddenFilesDesc")}</span>
          </div>
          <Switch bind:checked={settings.showHiddenFiles} />
        </div>
      </section>

      <!-- 显示设置 -->
      <section class="settings-section">
        <h3>
          <Icon icon="mdi:view-grid" width={20} />
          {$t("photos.settings.displaySettings")}
        </h3>

        <div class="setting-item">
          <div class="setting-info">
            <span class="setting-label">{$t("photos.settings.thumbnailSize")}</span>
          </div>
          <select bind:value={settings.thumbnailSize} class="setting-select">
            <option value="small">{$t("photos.settings.small")}</option>
            <option value="medium">{$t("photos.settings.medium")}</option>
            <option value="large">{$t("photos.settings.large")}</option>
          </select>
        </div>

        <div class="setting-item">
          <div class="setting-info">
            <span class="setting-label">{$t("photos.settings.defaultSort")}</span>
          </div>
          <select bind:value={settings.sortBy} class="setting-select">
            <option value="date_desc">{$t("photos.settings.dateNewest")}</option>
            <option value="date_asc">{$t("photos.settings.dateOldest")}</option>
            <option value="name_asc">{$t("photos.settings.nameAZ")}</option>
            <option value="name_desc">{$t("photos.settings.nameZA")}</option>
          </select>
        </div>

        <div class="setting-item">
          <div class="setting-info">
            <span class="setting-label">{$t("photos.settings.timelineGroup")}</span>
          </div>
          <select bind:value={settings.groupBy} class="setting-select">
            <option value="day">{$t("photos.settings.groupByDay")}</option>
            <option value="month">{$t("photos.settings.groupByMonth")}</option>
            <option value="year">{$t("photos.settings.groupByYear")}</option>
          </select>
        </div>
      </section>
    </div>
  {/if}
</div>

<!-- 添加图库弹窗 -->
<Modal 
  open={showAddLibrary} 
  title={$t("photos.addLibrary")} 
  onclose={() => showAddLibrary = false}
  showFooter={true}
>
  <div class="modal-form">
    <div class="form-group">
      <label>{$t("photos.name")}</label>
      <Input bind:value={newLibraryName} placeholder={$t("photos.myPhotos")} />
    </div>
    <div class="form-group">
      <label>{$t("photos.path")}</label>
      <div class="path-input-row">
        <Input bind:value={newLibraryPath} placeholder="/home/user/Pictures" />
        <Button size="sm" variant="outline" onclick={() => showFolderBrowser = true}>
          <Icon icon="mdi:folder-open" width={16} />
          {$t("photos.browse")}
        </Button>
      </div>
    </div>
    {#if addError}
      <div class="error-msg">{addError}</div>
    {/if}
  </div>
  {#snippet footer()}
    <Button variant="ghost" onclick={() => showAddLibrary = false}>{$t("common.cancel")}</Button>
    <Button onclick={addLibrary} disabled={addingLibrary}>
      {#if addingLibrary}
        <Spinner size="sm" />
      {/if}
      {$t("photos.add")}
    </Button>
  {/snippet}
</Modal>

<!-- 删除确认弹窗 -->
<Modal 
  open={showDeleteConfirm} 
  title={$t("photos.deleteLibrary")} 
  onclose={() => showDeleteConfirm = false}
  showFooter={true}
>
  <p>{$t("photos.deleteConfirm", { values: { name: libraryToDelete?.name || "" } })}</p>
  <p class="warning-text">{$t("photos.deleteWarning")}</p>
  {#snippet footer()}
    <Button variant="ghost" onclick={() => showDeleteConfirm = false}>{$t("common.cancel")}</Button>
    <Button variant="destructive" onclick={deleteLibrary}>{$t("common.delete")}</Button>
  {/snippet}
</Modal>

<!-- 文件夹选择器 -->
<FolderBrowser
  bind:open={showFolderBrowser}
  title={$t("photos.selectFolder")}
  initialPath="/home"
  onConfirm={handleFolderSelect}
  onClose={() => showFolderBrowser = false}
/>

<style>
  .settings-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--bg-window);
  }

  .settings-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-color);
  }

  .settings-header h2 {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0;
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .close-btn {
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

  .close-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .loading-state {
    display: flex;
    align-items: center;
    justify-content: center;
    flex: 1;
  }

  .settings-content {
    flex: 1;
    overflow-y: auto;
    padding: 20px;
  }

  .settings-section {
    margin-bottom: 32px;
  }

  .settings-section h3 {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 16px 0;
    padding-bottom: 8px;
    border-bottom: 1px solid var(--border-color-light);
  }

  .library-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 16px;
  }

  .library-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    background: var(--bg-secondary);
    border-radius: 8px;
    border: 1px solid var(--border-color-light);
  }

  .library-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .library-name {
    font-weight: 500;
    color: var(--text-primary);
  }

  .library-path {
    font-size: 12px;
    color: var(--text-muted);
    font-family: monospace;
  }

  .library-stats {
    font-size: 12px;
    color: var(--text-secondary);
  }

  .library-actions {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .default-badge {
    padding: 4px 8px;
    background: var(--color-primary-light);
    color: var(--color-primary);
    border-radius: 4px;
    font-size: 12px;
    font-weight: 500;
  }

  .empty-libraries {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 32px;
    color: var(--text-muted);
  }

  .setting-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 0;
    border-bottom: 1px solid var(--border-color-light);
  }

  .setting-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .setting-label {
    font-weight: 500;
    color: var(--text-primary);
  }

  .setting-desc {
    font-size: 12px;
    color: var(--text-muted);
  }

  .setting-select {
    padding: 8px 12px;
    border: 1px solid var(--border-color);
    border-radius: 6px;
    background: var(--bg-window);
    color: var(--text-primary);
    font-size: 14px;
    min-width: 120px;
  }

  .setting-select:focus {
    outline: none;
    border-color: var(--color-primary);
  }

  .modal-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 8px 0;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .form-group label {
    font-size: 14px;
    font-weight: 500;
    color: var(--text-primary);
  }

  .path-input-row {
    display: flex;
    gap: 8px;
  }

  .path-input-row :global(.input-wrapper) {
    flex: 1;
  }

  .error-msg {
    padding: 8px 12px;
    background: rgba(220, 53, 69, 0.1);
    border-radius: 6px;
    color: var(--color-danger);
    font-size: 14px;
  }

  .warning-text {
    font-size: 14px;
    color: var(--text-muted);
    margin-top: 8px;
  }
</style>
