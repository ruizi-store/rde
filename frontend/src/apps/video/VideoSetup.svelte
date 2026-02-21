<script lang="ts">
  import { t } from "svelte-i18n";
  import { Button, Spinner, FolderBrowser } from "$shared/ui";
  import Icon from "@iconify/svelte";
  import { videoSettings } from "./settings.svelte";

  interface Props {
    onComplete: () => void;
    onSkip: () => void;
  }

  let { onComplete, onSkip }: Props = $props();

  let step = $state<"welcome" | "select" | "complete">("welcome");
  let videoPath = $state("");
  let defaultVideoPath = $state("");
  let loading = $state(false);
  let error = $state("");
  let showFolderBrowser = $state(false);

  // 检测默认路径
  async function detectDefaults() {
    loading = true;
    try {
      defaultVideoPath = await videoSettings.detectDefaultVideoPath();
      videoPath = defaultVideoPath;
    } catch (e) {
      console.error($t("video.detectFailed"), e);
    } finally {
      loading = false;
    }
  }

  // 完成设置
  function completeSetup() {
    if (!videoPath) {
      error = $t("video.pleaseSelectFolder");
      return;
    }
    videoSettings.completeSetup(videoPath);
    onComplete();
  }

  // 跳过设置
  async function skipSetup() {
    loading = true;
    try {
      const path = defaultVideoPath || await videoSettings.detectDefaultVideoPath();
      videoSettings.completeSetup(path);
      onSkip();
    } catch (e) {
      error = $t("video.setupFailed");
      step = "select";
    } finally {
      loading = false;
    }
  }

  function handleFolderSelect(path: string) {
    videoPath = path;
    showFolderBrowser = false;
  }

  function useDefaultPath() {
    videoPath = defaultVideoPath;
  }

  $effect(() => {
    detectDefaults();
  });
</script>

<div class="setup-container">
  {#if step === "welcome"}
    <div class="setup-card">
      <div class="setup-icon">
        <Icon icon="mdi:movie-open" width={72} />
      </div>
      <h1>{$t("video.welcome")}</h1>
      <p class="setup-desc">
        {$t("video.setupDesc")}
      </p>

      {#if loading}
        <div class="loading-hint">
          <Spinner size="sm" />
          <span>{$t("video.detecting")}</span>
        </div>
      {:else if defaultVideoPath}
        <div class="detected-path">
          <Icon icon="mdi:folder-play" width={20} />
          <span>{$t("video.detected", { values: { path: defaultVideoPath } })}</span>
        </div>
      {/if}

      <div class="setup-actions">
        <Button variant="primary" size="lg" onclick={() => step = "select"}>
          <Icon icon="mdi:folder-cog" width={20} />
          {$t("video.selectFolder")}
        </Button>
        <Button variant="ghost" size="lg" onclick={skipSetup} disabled={loading}>
          {$t("video.useDefault")}
        </Button>
      </div>
    </div>
  {:else if step === "select"}
    <div class="setup-card wide">
      <div class="setup-header">
        <button class="back-btn" onclick={() => step = "welcome"}>
          <Icon icon="mdi:arrow-left" width={20} />
        </button>
        <h2>{$t("video.selectVideoFolder")}</h2>
      </div>

      <p class="setup-hint">{$t("video.selectHint")}</p>

      {#if error}
        <div class="error-message">
          <Icon icon="mdi:alert-circle" width={18} />
          {error}
        </div>
      {/if}

      <div class="path-input-group">
        <div class="path-display">
          <Icon icon="mdi:folder-play" width={20} />
          <span class="path-text">{videoPath || $t("video.notSelected")}</span>
        </div>
        <Button variant="outline" onclick={() => showFolderBrowser = true}>
          <Icon icon="mdi:folder-open" width={18} />
          {$t("video.browse")}
        </Button>
      </div>

      {#if defaultVideoPath && videoPath !== defaultVideoPath}
        <button class="default-hint" onclick={useDefaultPath}>
          <Icon icon="mdi:lightbulb-outline" width={16} />
          {$t("video.useDetectedPath", { values: { path: defaultVideoPath } })}
        </button>
      {/if}

      <div class="setup-actions">
        <Button variant="primary" size="lg" onclick={completeSetup} disabled={!videoPath}>
          <Icon icon="mdi:check" width={20} />
          {$t("video.completeSetup")}
        </Button>
      </div>
    </div>
  {/if}

  {#if showFolderBrowser}
    <div class="folder-browser-overlay">
      <div class="folder-browser-modal">
        <div class="modal-header">
          <h3>{$t("common.selectFolder")}</h3>
          <button class="close-btn" onclick={() => showFolderBrowser = false}>
            <Icon icon="mdi:close" width={20} />
          </button>
        </div>
        <FolderBrowser
          onSelect={handleFolderSelect}
          onCancel={() => showFolderBrowser = false}
          initialPath={defaultVideoPath || "/"}
        />
      </div>
    </div>
  {/if}
</div>

<style>
  .setup-container {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100%;
    padding: 40px;
    background: linear-gradient(135deg, 
      var(--bg-primary, #1a1a2e) 0%, 
      color-mix(in srgb, var(--accent-color, #7E57C2) 15%, var(--bg-primary, #1a1a2e)) 100%
    );
  }

  .setup-card {
    background: var(--bg-secondary, rgba(255, 255, 255, 0.05));
    border: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
    border-radius: 16px;
    padding: 48px;
    max-width: 480px;
    width: 100%;
    text-align: center;
    backdrop-filter: blur(10px);

    &.wide {
      max-width: 560px;
      text-align: left;
    }

    :global([data-theme="light"]) & {
      background: rgba(255, 255, 255, 0.9);
      border-color: rgba(0, 0, 0, 0.1);
    }
  }

  .setup-icon {
    margin-bottom: 24px;
    color: var(--accent-color, #7E57C2);
  }

  h1 {
    margin: 0 0 12px;
    font-size: 24px;
    font-weight: 600;
    color: var(--text-primary, #fff);

    :global([data-theme="light"]) & {
      color: #333;
    }
  }

  h2 {
    margin: 0;
    font-size: 18px;
    font-weight: 500;
  }

  .setup-desc {
    margin: 0 0 32px;
    color: var(--text-secondary, rgba(255, 255, 255, 0.7));
    line-height: 1.6;

    :global([data-theme="light"]) & {
      color: #666;
    }
  }

  .setup-hint {
    margin: 0 0 24px;
    color: var(--text-secondary, rgba(255, 255, 255, 0.6));
    font-size: 14px;
  }

  .setup-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 16px;
  }

  .back-btn {
    width: 36px;
    height: 36px;
    border: none;
    border-radius: 8px;
    background: var(--bg-tertiary, rgba(255, 255, 255, 0.1));
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;

    &:hover {
      background: var(--bg-hover, rgba(255, 255, 255, 0.15));
    }
  }

  .loading-hint {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    margin-bottom: 24px;
    color: var(--text-secondary);
    font-size: 14px;
  }

  .detected-path {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 12px 16px;
    background: var(--bg-tertiary, rgba(255, 255, 255, 0.08));
    border-radius: 8px;
    margin-bottom: 24px;
    font-size: 13px;
    color: var(--text-secondary);
  }

  .setup-actions {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .path-input-group {
    display: flex;
    gap: 8px;
    margin-bottom: 16px;
  }

  .path-display {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    background: var(--bg-tertiary, rgba(255, 255, 255, 0.08));
    border: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
    border-radius: 8px;
    color: var(--text-secondary);

    .path-text {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .default-hint {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 12px;
    margin-bottom: 24px;
    background: none;
    border: none;
    color: var(--accent-color, #7E57C2);
    font-size: 13px;
    cursor: pointer;
    border-radius: 6px;

    &:hover {
      background: var(--bg-tertiary, rgba(255, 255, 255, 0.05));
    }
  }

  .error-message {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    margin-bottom: 16px;
    background: rgba(244, 67, 54, 0.1);
    border: 1px solid rgba(244, 67, 54, 0.3);
    border-radius: 8px;
    color: #f44336;
    font-size: 14px;
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

  .close-btn {
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
