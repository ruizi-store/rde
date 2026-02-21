<script lang="ts">
  import { onMount } from "svelte";
  import Icon from "@iconify/svelte";
  import { t } from "svelte-i18n";
  import { Button, Spinner, Switch } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import { downloadService, type DownloadSettings } from "./service";

  // Props
  interface Props {
    onClose?: () => void;
  }
  let { onClose }: Props = $props();

  // 状态
  let settings = $state<DownloadSettings | null>(null);
  let loading = $state(true);
  let saving = $state(false);

  // 表单字段
  let downloadDir = $state("");
  let maxConcurrent = $state(5);
  let maxConnPerServer = $state(16);
  let split = $state(16);
  let globalDownloadLimit = $state(0);
  let globalUploadLimit = $state(0);
  let seedRatio = $state(1.0);
  let seedTime = $state(0);
  let enableDHT = $state(true);
  let notifyOnComplete = $state(true);
  let autoStart = $state(true);

  onMount(async () => {
    try {
      settings = await downloadService.getSettings();
      // 填充表单
      downloadDir = settings.download_dir;
      maxConcurrent = settings.max_concurrent;
      maxConnPerServer = settings.max_conn_per_server;
      split = settings.split;
      globalDownloadLimit = settings.global_download_limit;
      globalUploadLimit = settings.global_upload_limit;
      seedRatio = settings.seed_ratio;
      seedTime = settings.seed_time;
      enableDHT = settings.enable_dht;
      notifyOnComplete = settings.notify_on_complete;
      autoStart = settings.auto_start;
    } catch (e: any) {
      showToast($t("download.loadSettingsFailed") + ": " + e.message, "error");
    } finally {
      loading = false;
    }
  });

  async function saveSettings() {
    saving = true;
    try {
      await downloadService.updateSettings({
        download_dir: downloadDir,
        max_concurrent: maxConcurrent,
        max_conn_per_server: maxConnPerServer,
        split,
        global_download_limit: globalDownloadLimit,
        global_upload_limit: globalUploadLimit,
        seed_ratio: seedRatio,
        seed_time: seedTime,
        enable_dht: enableDHT,
        notify_on_complete: notifyOnComplete,
        auto_start: autoStart,
      });
      showToast($t("download.settingsSaved"), "success");
      onClose?.();
    } catch (e: any) {
      showToast($t("download.saveFailed") + ": " + e.message, "error");
    } finally {
      saving = false;
    }
  }

  function formatSpeedLimit(bytes: number): string {
    if (bytes === 0) return $t("download.noLimit");
    const kb = bytes / 1024;
    if (kb < 1024) return `${kb.toFixed(0)} KB/s`;
    return `${(kb / 1024).toFixed(1)} MB/s`;
  }
</script>

<div class="settings-panel">
  {#if loading}
    <div class="loading-container">
      <Spinner />
    </div>
  {:else}
    <div class="settings-content">
      <!-- 基本设置 -->
      <section class="settings-section">
        <h3><Icon icon="mdi:folder-download" width="18" /> {$t("download.basicSettings")}</h3>

        <div class="form-group">
          <label for="download-dir">{$t("download.downloadDir")}</label>
          <input
            id="download-dir"
            type="text"
            bind:value={downloadDir}
            placeholder="~/Downloads"
          />
        </div>

        <div class="form-row">
          <div class="form-group">
            <label for="max-concurrent">{$t("download.maxConcurrent")}</label>
            <input
              id="max-concurrent"
              type="number"
              min="1"
              max="20"
              bind:value={maxConcurrent}
            />
          </div>

          <div class="form-group">
            <label for="max-conn">{$t("download.maxConnPerTask")}</label>
            <input
              id="max-conn"
              type="number"
              min="1"
              max="64"
              bind:value={maxConnPerServer}
            />
          </div>

          <div class="form-group">
            <label for="split">{$t("download.splitCount")}</label>
            <input
              id="split"
              type="number"
              min="1"
              max="64"
              bind:value={split}
            />
          </div>
        </div>
      </section>

      <!-- 速度限制 -->
      <section class="settings-section">
        <h3><Icon icon="mdi:speedometer" width="18" /> {$t("download.speedLimit")}</h3>

        <div class="form-row">
          <div class="form-group">
            <label for="download-limit">{$t("download.downloadLimit")}</label>
            <input
              id="download-limit"
              type="number"
              min="0"
              bind:value={globalDownloadLimit}
              onchange={(e) => {
                const v = parseInt((e.target as HTMLInputElement).value);
                globalDownloadLimit = v > 0 ? v * 1024 : 0;
              }}
            />
            <span class="hint">{formatSpeedLimit(globalDownloadLimit)}</span>
          </div>

          <div class="form-group">
            <label for="upload-limit">{$t("download.uploadLimit")}</label>
            <input
              id="upload-limit"
              type="number"
              min="0"
              bind:value={globalUploadLimit}
              onchange={(e) => {
                const v = parseInt((e.target as HTMLInputElement).value);
                globalUploadLimit = v > 0 ? v * 1024 : 0;
              }}
            />
            <span class="hint">{formatSpeedLimit(globalUploadLimit)}</span>
          </div>
        </div>
      </section>

      <!-- BT 设置 -->
      <section class="settings-section">
        <h3><Icon icon="mdi:magnet" width="18" /> {$t("download.btSettings")}</h3>

        <div class="form-row">
          <div class="form-group">
            <label for="seed-ratio">{$t("download.seedRatio")}</label>
            <input
              id="seed-ratio"
              type="number"
              min="0"
              max="10"
              step="0.1"
              bind:value={seedRatio}
            />
          </div>

          <div class="form-group">
            <label for="seed-time">{$t("download.seedTime")}</label>
            <input
              id="seed-time"
              type="number"
              min="0"
              bind:value={seedTime}
            />
            <span class="hint">{$t("download.noTimeLimit")}</span>
          </div>
        </div>

        <div class="toggle-row">
          <div class="toggle-item">
            <span>{$t("download.enableDHT")}</span>
            <Switch bind:checked={enableDHT} />
          </div>
        </div>
      </section>

      <!-- 通知设置 -->
      <section class="settings-section">
        <h3><Icon icon="mdi:bell" width="18" /> {$t("download.notificationSettings")}</h3>

        <div class="toggle-row">
          <div class="toggle-item">
            <span>{$t("download.notifyOnComplete")}</span>
            <Switch bind:checked={notifyOnComplete} />
          </div>
          <div class="toggle-item">
            <span>{$t("download.autoStartAria2")}</span>
            <Switch bind:checked={autoStart} />
          </div>
        </div>
      </section>
    </div>

    <div class="settings-footer">
      <Button variant="ghost" onclick={onClose}>{$t("download.cancel")}</Button>
      <Button variant="primary" loading={saving} onclick={saveSettings}>{$t("download.saveSettings")}</Button>
    </div>
  {/if}
</div>

<style>
  .settings-panel {
    display: flex;
    flex-direction: column;
    height: 100%;
    max-height: 70vh;
  }

  .loading-container {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 40px;
  }

  .settings-content {
    flex: 1;
    overflow-y: auto;
    padding: 16px 0;
  }

  .settings-section {
    margin-bottom: 24px;

    h3 {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 14px;
      font-weight: 600;
      color: var(--text-primary, #333);
      margin-bottom: 12px;
      padding-bottom: 8px;
      border-bottom: 1px solid var(--border-color, #e0e0e0);
    }
  }

  .form-group {
    margin-bottom: 12px;

    label {
      display: block;
      font-size: 13px;
      font-weight: 500;
      color: var(--text-secondary, #666);
      margin-bottom: 4px;
    }

    input {
      width: 100%;
      padding: 8px 12px;
      border: 1px solid var(--border-color, #e0e0e0);
      border-radius: 6px;
      font-size: 14px;
      background: var(--bg-input, white);
      color: var(--text-primary, #333);

      &:focus {
        outline: none;
        border-color: var(--color-primary, #4a90d9);
      }
    }

    .hint {
      font-size: 11px;
      color: var(--text-muted, #999);
      margin-top: 2px;
    }
  }

  .form-row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
    gap: 12px;
  }

  .toggle-row {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .toggle-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 0;

    span {
      font-size: 13px;
      color: var(--text-primary, #333);
    }
  }

  .settings-footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding-top: 16px;
    border-top: 1px solid var(--border-color, #e0e0e0);
  }
</style>
