<script lang="ts">
  import { t } from "svelte-i18n";
  import { Button, Switch, Select, FolderBrowser } from "$shared/ui";
  import Icon from "@iconify/svelte";
  import { videoSettings, type VideoSettings } from "./settings.svelte";

  interface Props {
    onClose: () => void;
  }

  let { onClose }: Props = $props();

  // 本地设置副本
  let localSettings = $state<VideoSettings>({ ...videoSettings.settings });
  let showFolderBrowser = $state(false);
  let hasChanges = $state(false);

  // 检测变化
  $effect(() => {
    hasChanges = JSON.stringify(localSettings) !== JSON.stringify(videoSettings.settings);
  });

  function handleFolderSelect(path: string) {
    localSettings.defaultVideoPath = path;
    showFolderBrowser = false;
  }

  function save() {
    videoSettings.update(localSettings);
    onClose();
  }

  function resetToDefaults() {
    videoSettings.reset();
    localSettings = { ...videoSettings.settings };
  }

  function clearHistory() {
    videoSettings.clearHistory();
  }

  const playbackRateOptions = [
    { value: 0.5, label: "0.5x" },
    { value: 0.75, label: "0.75x" },
    { value: 1, label: $t("video.settings.normalSpeed") },
    { value: 1.25, label: "1.25x" },
    { value: 1.5, label: "1.5x" },
    { value: 2, label: "2x" },
  ];

  const subtitleSizeOptions = [
    { value: 16, label: $t("video.settings.sizeSmall") + " (16px)" },
    { value: 20, label: $t("video.settings.sizeMediumSmall") + " (20px)" },
    { value: 24, label: $t("video.settings.sizeMedium") + " (24px)" },
    { value: 28, label: $t("video.settings.sizeMediumLarge") + " (28px)" },
    { value: 32, label: $t("video.settings.sizeLarge") + " (32px)" },
    { value: 40, label: $t("video.settings.sizeExtraLarge") + " (40px)" },
  ];
</script>

<div class="settings-panel">
  <div class="settings-header">
    <h2>
      <Icon icon="mdi:cog" width={22} />
      {$t("video.settings.title")}
    </h2>
    <button class="close-btn" onclick={onClose}>
      <Icon icon="mdi:close" width={20} />
    </button>
  </div>

  <div class="settings-content">
    <!-- 视频文件夹 -->
    <section class="settings-section">
      <h3>
        <Icon icon="mdi:folder-play" width={18} />
        {$t("video.settings.videoFolder")}
      </h3>

      <div class="setting-item">
        <div class="setting-info">
          <span class="setting-label">{$t("video.settings.defaultVideoFolder")}</span>
          <span class="setting-desc">{$t("video.settings.defaultVideoFolderDesc")}</span>
        </div>
        <div class="path-group">
          <span class="path-value">{localSettings.defaultVideoPath || $t("video.settings.notSet")}</span>
          <Button variant="outline" size="sm" onclick={() => showFolderBrowser = true}>
            {$t("video.settings.change")}
          </Button>
        </div>
      </div>
    </section>

    <!-- 播放设置 -->
    <section class="settings-section">
      <h3>
        <Icon icon="mdi:play-circle" width={18} />
        {$t("video.settings.playbackSettings")}
      </h3>

      <div class="setting-item">
        <div class="setting-info">
          <span class="setting-label">{$t("video.settings.defaultPlaybackSpeed")}</span>
          <span class="setting-desc">{$t("video.settings.defaultPlaybackSpeedDesc")}</span>
        </div>
        <Select
          options={playbackRateOptions}
          bind:value={localSettings.defaultPlaybackRate}
          width="120px"
        />
      </div>

      <div class="setting-item">
        <div class="setting-info">
          <span class="setting-label">{$t("video.settings.rememberPosition")}</span>
          <span class="setting-desc">{$t("video.settings.rememberPositionDesc")}</span>
        </div>
        <Switch bind:checked={localSettings.rememberPosition} />
      </div>

      <div class="setting-item">
        <div class="setting-info">
          <span class="setting-label">{$t("video.settings.autoPlayNext")}</span>
          <span class="setting-desc">{$t("video.settings.autoPlayNextDesc")}</span>
        </div>
        <Switch bind:checked={localSettings.autoPlayNext} />
      </div>
    </section>

    <!-- 字幕设置 -->
    <section class="settings-section">
      <h3>
        <Icon icon="mdi:subtitles" width={18} />
        {$t("video.settings.subtitleSettings")}
      </h3>

      <div class="setting-item">
        <div class="setting-info">
          <span class="setting-label">{$t("video.settings.autoDetectSubtitle")}</span>
          <span class="setting-desc">{$t("video.settings.autoDetectSubtitleDesc")}</span>
        </div>
        <Switch bind:checked={localSettings.autoDetectSubtitle} />
      </div>

      <div class="setting-item">
        <div class="setting-info">
          <span class="setting-label">{$t("video.settings.subtitleFontSize")}</span>
          <span class="setting-desc">{$t("video.settings.subtitleFontSizeDesc")}</span>
        </div>
        <Select
          options={subtitleSizeOptions}
          bind:value={localSettings.subtitleFontSize}
          width="130px"
        />
      </div>
    </section>

    <!-- 显示设置 -->
    <section class="settings-section">
      <h3>
        <Icon icon="mdi:eye" width={18} />
        {$t("video.settings.displaySettings")}
      </h3>

      <div class="setting-item">
        <div class="setting-info">
          <span class="setting-label">{$t("video.settings.showFileExtension")}</span>
          <span class="setting-desc">{$t("video.settings.showFileExtensionDesc")}</span>
        </div>
        <Switch bind:checked={localSettings.showFileExtension} />
      </div>
    </section>

    <!-- 播放历史 -->
    <section class="settings-section">
      <h3>
        <Icon icon="mdi:history" width={18} />
        {$t("video.settings.playbackHistory")}
      </h3>

      <div class="status-grid">
        <div class="status-item">
          <span class="status-label">{$t("video.settings.historyCount")}</span>
          <span class="status-value">{$t("video.settings.videosCount", { values: { n: videoSettings.history.length } })}</span>
        </div>
      </div>

      <div class="playlist-actions">
        <Button variant="ghost" size="sm" onclick={clearHistory}>
          <Icon icon="mdi:delete-sweep" width={16} />
          {$t("video.settings.clearHistory")}
        </Button>
      </div>
    </section>
  </div>

  <div class="settings-footer">
    <Button variant="ghost" onclick={resetToDefaults}>
      {$t("video.settings.restoreDefaults")}
    </Button>
    <div class="footer-right">
      <Button variant="ghost" onclick={onClose}>
        {$t("common.cancel")}
      </Button>
      <Button variant="primary" onclick={save} disabled={!hasChanges}>
        {$t("video.settings.saveSettings")}
      </Button>
    </div>
  </div>

  {#if showFolderBrowser}
    <div class="folder-browser-overlay">
      <div class="folder-browser-modal">
        <div class="modal-header">
          <h3>{$t("video.settings.selectVideoFolder")}</h3>
          <button class="close-modal-btn" onclick={() => showFolderBrowser = false}>
            <Icon icon="mdi:close" width={20} />
          </button>
        </div>
        <FolderBrowser
          onSelect={handleFolderSelect}
          onCancel={() => showFolderBrowser = false}
          initialPath={localSettings.defaultVideoPath || "/"}
        />
      </div>
    </div>
  {/if}
</div>

<style>
  .settings-panel {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--bg-primary, #1a1a2e);
    color: var(--text-primary, #fff);

    :global([data-theme="light"]) & {
      background: #f8f9fa;
      color: #333;
    }
  }

  .settings-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));

    h2 {
      display: flex;
      align-items: center;
      gap: 8px;
      margin: 0;
      font-size: 18px;
      font-weight: 500;
    }

    :global([data-theme="light"]) & {
      border-bottom-color: rgba(0, 0, 0, 0.1);
    }
  }

  .close-btn {
    width: 36px;
    height: 36px;
    border: none;
    border-radius: 8px;
    background: transparent;
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;

    &:hover {
      background: var(--bg-hover, rgba(255, 255, 255, 0.1));
    }
  }

  .settings-content {
    flex: 1;
    overflow-y: auto;
    padding: 16px 20px;
  }

  .settings-section {
    margin-bottom: 24px;

    h3 {
      display: flex;
      align-items: center;
      gap: 8px;
      margin: 0 0 16px;
      font-size: 14px;
      font-weight: 500;
      color: var(--text-secondary, rgba(255, 255, 255, 0.7));
      text-transform: uppercase;
      letter-spacing: 0.5px;

      :global([data-theme="light"]) & {
        color: #666;
      }
    }
  }

  .setting-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    background: var(--bg-secondary, rgba(255, 255, 255, 0.05));
    border-radius: 8px;
    margin-bottom: 8px;

    :global([data-theme="light"]) & {
      background: rgba(0, 0, 0, 0.03);
    }
  }

  .setting-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .setting-label {
    font-size: 14px;
    font-weight: 500;
  }

  .setting-desc {
    font-size: 12px;
    color: var(--text-secondary, rgba(255, 255, 255, 0.5));

    :global([data-theme="light"]) & {
      color: #888;
    }
  }

  .path-group {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .path-value {
    max-width: 200px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 13px;
    color: var(--text-secondary);
  }

  .status-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
    margin-bottom: 12px;
  }

  .status-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 12px 16px;
    background: var(--bg-secondary, rgba(255, 255, 255, 0.05));
    border-radius: 8px;
  }

  .status-label {
    font-size: 12px;
    color: var(--text-secondary, rgba(255, 255, 255, 0.5));
  }

  .status-value {
    font-size: 14px;
    font-weight: 500;
  }

  .playlist-actions {
    display: flex;
    gap: 8px;
  }

  .settings-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-top: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));

    :global([data-theme="light"]) & {
      border-top-color: rgba(0, 0, 0, 0.1);
    }
  }

  .footer-right {
    display: flex;
    gap: 8px;
  }

  .folder-browser-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .folder-browser-modal {
    background: var(--bg-primary, #1a1a2e);
    border: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
    border-radius: 12px;
    width: 90%;
    max-width: 600px;
    max-height: 80vh;
    overflow: hidden;
    display: flex;
    flex-direction: column;

    :global([data-theme="light"]) & {
      background: #fff;
      border-color: rgba(0, 0, 0, 0.1);
    }
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));

    h3 {
      margin: 0;
      font-size: 16px;
      font-weight: 500;
    }
  }

  .close-modal-btn {
    width: 32px;
    height: 32px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;

    &:hover {
      background: var(--bg-hover, rgba(255, 255, 255, 0.1));
    }
  }
</style>
