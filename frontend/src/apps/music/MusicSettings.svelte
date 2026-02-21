<script lang="ts">
  import { t } from "svelte-i18n";
  import { Button, Switch, Select, FolderBrowser } from "$shared/ui";
  import Icon from "@iconify/svelte";
  import { musicSettings, type MusicSettings } from "./settings.svelte";
  import { musicPlayer } from "$shared/stores/music-player.svelte";

  interface Props {
    onClose: () => void;
  }

  let { onClose }: Props = $props();

  // 本地设置副本
  let localSettings = $state<MusicSettings>({ ...musicSettings.settings });
  let showFolderBrowser = $state(false);
  let hasChanges = $state(false);

  // 检测变化
  $effect(() => {
    hasChanges = JSON.stringify(localSettings) !== JSON.stringify(musicSettings.settings);
  });

  function handleFolderSelect(path: string) {
    localSettings.defaultMusicPath = path;
    showFolderBrowser = false;
  }

  function save() {
    musicSettings.update(localSettings);
    onClose();
  }

  function resetToDefaults() {
    musicSettings.reset();
    localSettings = { ...musicSettings.settings };
  }

  const playlistEndOptions = $derived([
    { value: "stop", label: $t('music.settings.playlistEndStop') },
    { value: "repeat", label: $t('music.settings.playlistEndRepeat') },
    { value: "shuffle", label: $t('music.settings.playlistEndShuffle') },
  ]);

  const equalizerOptions = $derived([
    { value: "flat", label: $t('music.settings.eqFlat') },
    { value: "bass", label: $t('music.settings.eqBass') },
    { value: "treble", label: $t('music.settings.eqTreble') },
    { value: "vocal", label: $t('music.settings.eqVocal') },
  ]);
</script>

<div class="settings-panel">
  <div class="settings-header">
    <h2>
      <Icon icon="mdi:cog" width={22} />
      {$t('music.settings.title')}
    </h2>
    <button class="close-btn" onclick={onClose}>
      <Icon icon="mdi:close" width={20} />
    </button>
  </div>

  <div class="settings-content">
    <!-- 音乐文件夹 -->
    <section class="settings-section">
      <h3>
        <Icon icon="mdi:folder-music" width={18} />
        {$t('music.settings.musicFolder')}
      </h3>

      <div class="setting-item">
        <div class="setting-info">
          <span class="setting-label">{$t('music.settings.defaultFolder')}</span>
          <span class="setting-desc">{$t('music.settings.defaultFolderDesc')}</span>
        </div>
        <div class="path-group">
          <span class="path-value">{localSettings.defaultMusicPath || $t('music.settings.notSet')}</span>
          <Button variant="outline" size="sm" onclick={() => showFolderBrowser = true}>
            {$t('music.settings.change')}
          </Button>
        </div>
      </div>

      <div class="setting-item">
        <div class="setting-info">
          <span class="setting-label">{$t('music.settings.autoScan')}</span>
          <span class="setting-desc">{$t('music.settings.autoScanDesc')}</span>
        </div>
        <Switch bind:checked={localSettings.autoScan} />
      </div>

      <div class="setting-item">
        <div class="setting-info">
          <span class="setting-label">{$t('music.settings.includeSubfolders')}</span>
          <span class="setting-desc">{$t('music.settings.includeSubfoldersDesc')}</span>
        </div>
        <Switch bind:checked={localSettings.scanSubfolders} />
      </div>
    </section>

    <!-- 播放设置 -->
    <section class="settings-section">
      <h3>
        <Icon icon="mdi:play-circle" width={18} />
        {$t('music.settings.playback')}
      </h3>

      <div class="setting-item">
        <div class="setting-info">
          <span class="setting-label">{$t('music.settings.onPlaylistEnd')}</span>
          <span class="setting-desc">{$t('music.settings.onPlaylistEndDesc')}</span>
        </div>
        <Select
          options={playlistEndOptions}
          bind:value={localSettings.onPlaylistEnd}
          width="140px"
        />
      </div>

      <div class="setting-item">
        <div class="setting-info">
          <span class="setting-label">{$t('music.settings.rememberPosition')}</span>
          <span class="setting-desc">{$t('music.settings.rememberPositionDesc')}</span>
        </div>
        <Switch bind:checked={localSettings.rememberPosition} />
      </div>

      <div class="setting-item">
        <div class="setting-info">
          <span class="setting-label">{$t('music.settings.equalizerPreset')}</span>
          <span class="setting-desc">{$t('music.settings.equalizerPresetDesc')}</span>
        </div>
        <Select
          options={equalizerOptions}
          bind:value={localSettings.equalizerPreset}
          width="120px"
        />
      </div>
    </section>

    <!-- 显示设置 -->
    <section class="settings-section">
      <h3>
        <Icon icon="mdi:eye" width={18} />
        {$t('music.settings.display')}
      </h3>

      <div class="setting-item">
        <div class="setting-info">
          <span class="setting-label">{$t('music.settings.showExtension')}</span>
          <span class="setting-desc">{$t('music.settings.showExtensionDesc')}</span>
        </div>
        <Switch bind:checked={localSettings.showFileExtension} />
      </div>
    </section>

    <!-- 当前播放状态 -->
    <section class="settings-section">
      <h3>
        <Icon icon="mdi:information" width={18} />
        {$t('music.settings.status')}
      </h3>

      <div class="status-grid">
        <div class="status-item">
          <span class="status-label">{$t('music.settings.playlist')}</span>
          <span class="status-value">{$t('music.settings.tracksCount', { values: { n: musicPlayer.playlist.length } })}</span>
        </div>
        <div class="status-item">
          <span class="status-label">{$t('music.settings.currentTrack')}</span>
          <span class="status-value">{musicPlayer.displayTitle || $t('music.settings.none')}</span>
        </div>
        <div class="status-item">
          <span class="status-label">{$t('music.settings.playMode')}</span>
          <span class="status-value">{musicPlayer.getPlayModeName()}</span>
        </div>
        <div class="status-item">
          <span class="status-label">{$t('music.settings.volume')}</span>
          <span class="status-value">{Math.round(musicPlayer.volume * 100)}%</span>
        </div>
      </div>

      <div class="playlist-actions">
        <Button variant="ghost" size="sm" onclick={() => musicPlayer.clearPlaylist()}>
          <Icon icon="mdi:playlist-remove" width={16} />
          {$t('music.settings.clearPlaylist')}
        </Button>
      </div>
    </section>
  </div>

  <div class="settings-footer">
    <Button variant="ghost" onclick={resetToDefaults}>
      {$t('music.settings.resetDefaults')}
    </Button>
    <div class="footer-right">
      <Button variant="ghost" onclick={onClose}>
        {$t('common.cancel')}
      </Button>
      <Button variant="primary" onclick={save} disabled={!hasChanges}>
        {$t('music.settings.saveSettings')}
      </Button>
    </div>
  </div>

  {#if showFolderBrowser}
    <div class="folder-browser-overlay">
      <div class="folder-browser-modal">
        <div class="modal-header">
          <h3>{$t('music.settings.selectMusicFolder')}</h3>
          <button class="close-modal-btn" onclick={() => showFolderBrowser = false}>
            <Icon icon="mdi:close" width={20} />
          </button>
        </div>
        <FolderBrowser
          onSelect={handleFolderSelect}
          onCancel={() => showFolderBrowser = false}
          initialPath={localSettings.defaultMusicPath || "/"}
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
    padding: 12px 0;
    border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.05));

    &:last-child {
      border-bottom: none;
    }

    :global([data-theme="light"]) & {
      border-bottom-color: rgba(0, 0, 0, 0.05);
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
    gap: 8px;
  }

  .path-value {
    max-width: 180px;
    padding: 6px 10px;
    background: var(--bg-tertiary, rgba(255, 255, 255, 0.05));
    border-radius: 6px;
    font-size: 13px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .status-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 12px;
    margin-bottom: 16px;
  }

  .status-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 12px;
    background: var(--bg-tertiary, rgba(255, 255, 255, 0.05));
    border-radius: 8px;

    :global([data-theme="light"]) & {
      background: rgba(0, 0, 0, 0.03);
    }
  }

  .status-label {
    font-size: 11px;
    color: var(--text-secondary);
    text-transform: uppercase;
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
    position: absolute;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10;
  }

  .folder-browser-modal {
    background: var(--bg-primary, #1a1a2e);
    border: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
    border-radius: 12px;
    width: 90%;
    max-width: 500px;
    max-height: 70vh;
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
    padding: 14px 16px;
    border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));

    h3 {
      margin: 0;
      font-size: 15px;
      font-weight: 500;
      text-transform: none;
      letter-spacing: normal;
      color: var(--text-primary);
    }
  }

  .close-modal-btn {
    width: 28px;
    height: 28px;
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
