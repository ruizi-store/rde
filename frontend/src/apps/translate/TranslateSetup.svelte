<script lang="ts">
  import { get } from "svelte/store";
  import { t } from "./i18n";
  import { Button, Spinner } from "$shared/ui";
  import { ensureService, getStatus, type ServiceStatus } from "./service";

  interface Props {
    status: ServiceStatus | null;
    /** 重新检查状态；返回 true 表示服务已可用 */
    onReady: () => Promise<boolean>;
  }

  let { status, onReady }: Props = $props();

  let working = $state(false);
  let progress = $state("");
  let errorText = $state("");
  let autoStarted = $state(false);

  const phase = $derived(status?.phase || "missing");
  const hasLocalImage = $derived(!!(status?.offlineReady || status?.imageReady));

  async function pollUntilReady(timeoutMs: number): Promise<boolean> {
    const getText = (key: string) => get(t)(key);
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      progress = getText("loadingModels");
      const ready = await onReady();
      if (ready) return true;
      const st = await getStatus().catch(() => null);
      if (st?.available) {
        return onReady();
      }
      await new Promise((r) => setTimeout(r, 2000));
    }
    return false;
  }

  async function handleEnsure() {
    if (working) return;
    working = true;
    errorText = "";
    const getText = (key: string) => get(t)(key);
    progress = getText("starting");

    try {
      const result = await ensureService(25);

      if (result.available || result.status === "success") {
        progress = getText("almostDone");
        await onReady();
        return;
      }

      if (result.status === "starting" || result.phase === "starting") {
        progress = result.message || getText("loadingModels");
        const ok = await pollUntilReady(3 * 60 * 1000);
        if (ok) {
          progress = getText("almostDone");
          return;
        }
        errorText = getText("startTimeout");
      } else {
        errorText = result.message || getText("startFailed");
      }
    } catch (e: any) {
      const msg = e?.response?.data?.message || e?.data?.message || e?.message;
      errorText = msg || getText("startFailed");
    } finally {
      working = false;
    }
  }

  // bootstrap 已拉起容器时自动跟进等待，避免用户再点「安装」撞端口
  $effect(() => {
    if ((phase === "starting" || status?.containerRunning) && !autoStarted && !working) {
      autoStarted = true;
      handleEnsure();
    }
  });
</script>

<div class="setup-container">
  <div class="setup-card">
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

    <h2 class="setup-title">{$t("title")}</h2>
    <p class="setup-subtitle">{$t("subtitle")}</p>

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

    <div class="install-section">
      {#if working}
        <div class="progress-area">
          <Spinner size="lg" />
          <p class="progress-text">{progress}</p>
        </div>
      {:else if errorText}
        <div class="error-area">
          <p class="error-text">{errorText}</p>
          <Button variant="primary" size="lg" onclick={handleEnsure}>
            {$t("retry")}
          </Button>
        </div>
      {:else}
        <Button variant="primary" size="lg" onclick={handleEnsure} class="install-btn">
          {#if phase === "ready_to_start" || phase === "starting" || hasLocalImage}
            {$t("startButton")}
          {:else}
            {$t("installButton")}
          {/if}
        </Button>
        <p class="install-hint">
          {#if hasLocalImage}
            {$t("startHintOffline")}
          {:else}
            {$t("installHint")}
          {/if}
        </p>
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
