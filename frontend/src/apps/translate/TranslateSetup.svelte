<script lang="ts">
  import { get } from "svelte/store";
  import { t } from "./i18n";
  import { Button, Spinner } from "$shared/ui";
  import { dockerStoreService } from "$apps/docker/store-service";
  import type { ServiceStatus } from "./service";

  interface Props {
    status: ServiceStatus | null;
    onRetry: () => Promise<void>;
  }

  let { status, onRetry }: Props = $props();

  let installing = $state(false);
  let installProgress = $state("");
  let installError = $state("");

  async function handleInstall() {
    installing = true;
    installError = "";
    
    const getText = (key: string) => get(t)(key);
    const downloadingText = getText("downloading");
    const configuringText = getText("configuring");
    const almostDoneText = getText("almostDone");
    const installFailedText = getText("installFailed");
    
    installProgress = downloadingText;

    try {
      const task = await dockerStoreService.installAppAsync("libretranslate", {
        CONTAINER_NAME: "libretranslate",
        PANEL_APP_PORT_HTTP: 5000,
        LT_LANGUAGES: "en,zh",
      });

      const result = await dockerStoreService.pollInstallTask(task.id, (taskStatus) => {
        if (taskStatus.output) {
          const output = taskStatus.output.toLowerCase();
          if (output.includes("pulling") || output.includes("download")) {
            installProgress = downloadingText;
          } else if (output.includes("creating") || output.includes("starting")) {
            installProgress = configuringText;
          } else {
            installProgress = almostDoneText;
          }
        }
      });

      if (result.status === "success") {
        installProgress = almostDoneText;
        setTimeout(async () => {
          await onRetry();
        }, 2000);
      } else {
        installError = installFailedText;
      }
    } catch (e: any) {
      installError = installFailedText;
    } finally {
      if (installError) {
        installing = false;
      }
    }
  }
</script>

<div class="setup-container">
  <div class="setup-card">
    <!-- 大图标 -->
    <div class="setup-icon">
      <svg width="80" height="80" viewBox="0 0 24 24" fill="none">
        <defs>
          <linearGradient id="iconGradient" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" style="stop-color:#3b82f6" />
            <stop offset="100%" style="stop-color:#06b6d4" />
          </linearGradient>
        </defs>
        <path 
          d="M12.87 15.07l-2.54-2.51.03-.03A17.52 17.52 0 0014.07 6H17V4h-7V2H8v2H1v2h11.17C11.5 7.92 10.44 9.75 9 11.35 8.07 10.32 7.3 9.19 6.69 8H4.69c.73 1.63 1.73 3.17 2.98 4.56l-5.09 5.02L4 19l5-5 3.11 3.11.76-2.04zM18.5 10h-2L12 22h2l1.12-3h4.75L21 22h2l-4.5-12zm-2.62 7l1.62-4.33L19.12 17h-3.24z" 
          fill="url(#iconGradient)"
        />
      </svg>
    </div>

    <!-- 标题 -->
    <h2 class="setup-title">{$t("title")}</h2>
    <p class="setup-subtitle">{$t("subtitle")}</p>

    <!-- 特性列表 -->
    <div class="features">
      <div class="feature">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
        </svg>
        <span>{$t("featurePrivate")}</span>
      </div>
      <div class="feature">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="2" y="3" width="20" height="14" rx="2" ry="2" />
          <line x1="8" y1="21" x2="16" y2="21" />
          <line x1="12" y1="17" x2="12" y2="21" />
        </svg>
        <span>{$t("featureOffline")}</span>
      </div>
      <div class="feature">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10" />
          <line x1="2" y1="12" x2="22" y2="12" />
          <path d="M12 2a15.3 15.3 0 014 10 15.3 15.3 0 01-4 10 15.3 15.3 0 01-4-10 15.3 15.3 0 014-10z" />
        </svg>
        <span>{$t("featureLanguages")}</span>
      </div>
    </div>

    <!-- 安装按钮 -->
    <div class="install-section">
      {#if installing}
        <div class="progress-area">
          <Spinner size="lg" />
          <p class="progress-text">{installProgress}</p>
        </div>
      {:else if installError}
        <div class="error-area">
          <p class="error-text">{installError}</p>
          <Button variant="primary" size="lg" onclick={handleInstall}>
            {$t("retry")}
          </Button>
        </div>
      {:else}
        <Button variant="primary" size="lg" onclick={handleInstall} class="install-btn">
          {$t("installButton")}
        </Button>
        <p class="install-hint">{$t("installHint")}</p>
      {/if}
    </div>
  </div>
</div>

<style>
  .setup-container {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    padding: 2rem;
    background: var(--color-bg-primary);
  }

  .setup-card {
    max-width: 400px;
    width: 100%;
    padding: 2.5rem 2rem;
    background: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    border-radius: 20px;
    text-align: center;
  }

  .setup-icon {
    margin-bottom: 1.5rem;
  }

  .setup-title {
    font-size: 1.5rem;
    font-weight: 600;
    margin: 0 0 0.5rem;
    color: var(--color-text-primary);
  }

  .setup-subtitle {
    color: var(--color-text-secondary);
    margin: 0 0 2rem;
    line-height: 1.5;
  }

  .features {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    margin-bottom: 2rem;
    padding: 1rem;
    background: var(--color-bg-tertiary);
    border-radius: 12px;
  }

  .feature {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    color: var(--color-text-secondary);
    font-size: 0.9rem;
  }

  .feature svg {
    color: var(--color-primary);
    flex-shrink: 0;
  }

  .install-section {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.75rem;
  }

  :global(.install-btn) {
    min-width: 200px;
    font-size: 1rem;
    padding: 0.875rem 2rem;
  }

  .install-hint {
    color: var(--color-text-tertiary);
    font-size: 0.8rem;
    margin: 0;
  }

  .progress-area {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1rem;
    padding: 1rem 0;
  }

  .progress-text {
    color: var(--color-text-secondary);
    font-size: 0.9rem;
    margin: 0;
  }

  .error-area {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1rem;
  }

  .error-text {
    color: var(--color-error);
    font-size: 0.9rem;
    margin: 0;
  }
</style>
