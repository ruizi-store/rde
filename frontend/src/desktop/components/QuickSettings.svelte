<script lang="ts">
  import { t } from "svelte-i18n";
  import Icon from "@iconify/svelte";
  import { systemMonitor, formatBytes, formatSpeed } from "$shared/services/system-monitor.svelte";

  let { visible = $bindable(false) }: { visible: boolean } = $props();

  // 监控开关
  $effect(() => {
    if (visible) {
      systemMonitor.startPolling(3000);
    } else {
      systemMonitor.stopPolling();
    }
  });

  function close() {
    visible = false;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      close();
    }
  }
</script>

{#if visible}
  <div
    class="quick-settings-overlay"
    onclick={close}
    onkeydown={handleKeydown}
    role="presentation"
    tabindex="-1"
  >
    <div class="quick-settings" onclick={(e) => e.stopPropagation()}>
      <!-- 系统监控 -->
      <div class="section monitor-section">
        <div class="section-title">
          <Icon icon="mdi:chart-line" width="16" />
          <span>{$t("desktop.systemMonitor")}</span>
        </div>

        <div class="monitor-grid">
          <!-- CPU -->
          <div class="monitor-card">
            <div class="monitor-icon cpu">
              <Icon icon="mdi:chip" width="20" />
            </div>
            <div class="monitor-info">
              <span class="monitor-label">CPU</span>
              <span class="monitor-value">{systemMonitor.cpu?.usage ?? 0}%</span>
            </div>
            <div class="monitor-bar">
              <div
                class="monitor-bar-fill cpu"
                style="width: {systemMonitor.cpu?.usage ?? 0}%"
              ></div>
            </div>
          </div>

          <!-- 内存 -->
          <div class="monitor-card">
            <div class="monitor-icon memory">
              <Icon icon="mdi:memory" width="20" />
            </div>
            <div class="monitor-info">
              <span class="monitor-label">{$t("desktop.memory")}</span>
              <span class="monitor-value">
                {formatBytes(systemMonitor.memory?.used ?? 0, 1)}
              </span>
            </div>
            <div class="monitor-bar">
              <div
                class="monitor-bar-fill memory"
                style="width: {systemMonitor.memory?.used_percent ?? 0}%"
              ></div>
            </div>
            <span class="monitor-total">/ {formatBytes(systemMonitor.memory?.total ?? 0, 0)}</span>
          </div>
        </div>

        <!-- 磁盘列表 -->
        {#if systemMonitor.disks.length > 0}
          <div class="disk-list">
            {#each systemMonitor.disks as disk}
              <div class="disk-item">
                <Icon icon="mdi:harddisk" width="16" />
                <span class="disk-path" title={disk.mount_point}>{disk.mount_point}</span>
                <div class="disk-bar">
                  <div
                    class="disk-bar-fill"
                    class:warning={disk.used_percent > 80}
                    class:danger={disk.used_percent > 90}
                    style="width: {disk.used_percent}%"
                  ></div>
                </div>
                <span class="disk-usage">
                  {formatBytes(disk.used, 0)} / {formatBytes(disk.total, 0)}
                </span>
              </div>
            {/each}
          </div>
        {/if}

        <!-- 网络速度 -->
        <div class="network-stats">
          <div class="network-item">
            <Icon icon="mdi:arrow-up" width="14" />
            <span class="network-label">{$t("desktop.upload")}</span>
            <span class="network-value">{formatSpeed(systemMonitor.networkSpeed.tx)}</span>
          </div>
          <div class="network-item">
            <Icon icon="mdi:arrow-down" width="14" />
            <span class="network-label">{$t("desktop.download")}</span>
            <span class="network-value">{formatSpeed(systemMonitor.networkSpeed.rx)}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .quick-settings-overlay {
    position: fixed;
    inset: 0;
    z-index: 10000;
  }

  .quick-settings {
    position: absolute;
    bottom: 56px;
    right: 8px;
    width: 340px;
    background: rgba(30, 30, 34, 0.95);
    backdrop-filter: blur(24px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
    animation: slideUp 0.2s ease-out;
    max-height: calc(100vh - 80px);
    overflow-y: auto;

    :global([data-theme="light"]) & {
      background: rgba(255, 255, 255, 0.95);
      border-color: rgba(0, 0, 0, 0.1);
      box-shadow: 0 12px 40px rgba(0, 0, 0, 0.15);
    }

    &::-webkit-scrollbar {
      width: 6px;
    }
    &::-webkit-scrollbar-thumb {
      background: rgba(255, 255, 255, 0.2);
      border-radius: 3px;

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.2);
      }
    }
  }

  .section {
    padding: 12px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.06);

    :global([data-theme="light"]) & {
      border-bottom-color: rgba(0, 0, 0, 0.06);
    }

    &:last-child {
      border-bottom: none;
    }
  }

  .section-title {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: rgba(255, 255, 255, 0.5);
    margin-bottom: 10px;

    :global([data-theme="light"]) & {
      color: rgba(0, 0, 0, 0.5);
    }
  }

  /* 系统监控 */
  .monitor-section {
    .monitor-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 8px;
      margin-bottom: 10px;
    }

    .monitor-card {
      background: rgba(255, 255, 255, 0.04);
      border-radius: 8px;
      padding: 10px;
      position: relative;

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.04);
      }
    }

    .monitor-icon {
      width: 32px;
      height: 32px;
      border-radius: 8px;
      display: flex;
      align-items: center;
      justify-content: center;
      margin-bottom: 8px;

      &.cpu {
        background: rgba(74, 144, 217, 0.2);
        color: #4a90d9;
      }
      &.memory {
        background: rgba(156, 89, 182, 0.2);
        color: #9c59b6;
      }
    }

    .monitor-info {
      display: flex;
      justify-content: space-between;
      align-items: baseline;
      margin-bottom: 6px;
    }

    .monitor-label {
      font-size: 12px;
      color: rgba(255, 255, 255, 0.6);

      :global([data-theme="light"]) & {
        color: rgba(0, 0, 0, 0.6);
      }
    }

    .monitor-value {
      font-size: 16px;
      font-weight: 600;
      color: rgba(255, 255, 255, 0.95);

      :global([data-theme="light"]) & {
        color: rgba(0, 0, 0, 0.85);
      }
    }

    .monitor-bar {
      height: 4px;
      background: rgba(255, 255, 255, 0.1);
      border-radius: 2px;
      overflow: hidden;

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.1);
      }
    }

    .monitor-bar-fill {
      height: 100%;
      border-radius: 2px;
      transition: width 0.3s ease;

      &.cpu {
        background: linear-gradient(90deg, #4a90d9, #7bb8f5);
      }
      &.memory {
        background: linear-gradient(90deg, #9c59b6, #c39bd3);
      }
    }

    .monitor-total {
      font-size: 10px;
      color: rgba(255, 255, 255, 0.4);
      margin-top: 4px;
      display: block;

      :global([data-theme="light"]) & {
        color: rgba(0, 0, 0, 0.4);
      }
    }
  }

  /* 磁盘列表 */
  .disk-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: 10px;
  }

  .disk-item {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: rgba(255, 255, 255, 0.7);

    :global([data-theme="light"]) & {
      color: rgba(0, 0, 0, 0.7);
    }
  }

  .disk-path {
    min-width: 40px;
    max-width: 60px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .disk-bar {
    flex: 1;
    height: 4px;
    background: rgba(255, 255, 255, 0.1);
    border-radius: 2px;
    overflow: hidden;

    :global([data-theme="light"]) & {
      background: rgba(0, 0, 0, 0.1);
    }
  }

  .disk-bar-fill {
    height: 100%;
    background: #52c41a;
    border-radius: 2px;
    transition: width 0.3s;

    &.warning {
      background: #faad14;
    }
    &.danger {
      background: #ff4d4f;
    }
  }

  .disk-usage {
    font-size: 10px;
    color: rgba(255, 255, 255, 0.5);
    min-width: 90px;
    text-align: right;

    :global([data-theme="light"]) & {
      color: rgba(0, 0, 0, 0.5);
    }
  }

  /* 网络速度 */
  .network-stats {
    display: flex;
    gap: 16px;
    background: rgba(255, 255, 255, 0.04);
    border-radius: 8px;
    padding: 8px 12px;

    :global([data-theme="light"]) & {
      background: rgba(0, 0, 0, 0.04);
    }
  }

  .network-item {
    display: flex;
    align-items: center;
    gap: 6px;
    flex: 1;
    color: rgba(255, 255, 255, 0.7);
    font-size: 12px;

    :global([data-theme="light"]) & {
      color: rgba(0, 0, 0, 0.7);
    }
  }

  .network-label {
    color: rgba(255, 255, 255, 0.5);

    :global([data-theme="light"]) & {
      color: rgba(0, 0, 0, 0.5);
    }
  }

  .network-value {
    margin-left: auto;
    color: rgba(255, 255, 255, 0.9);
    font-weight: 500;

    :global([data-theme="light"]) & {
      color: rgba(0, 0, 0, 0.85);
    }
  }

  @keyframes slideUp {
    from {
      opacity: 0;
      transform: translateY(10px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
