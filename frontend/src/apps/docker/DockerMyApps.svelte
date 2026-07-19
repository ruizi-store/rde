<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "svelte-i18n";
  import Icon from "@iconify/svelte";
  import { Button, Spinner, Modal, EmptyState } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import {
    dockerStoreService,
    type InstalledApp,
  } from "./store-service";
  import { refreshExternalApps } from "$apps";

  // ==================== Props ====================
  interface Props {
    onBrowseStore?: () => void;
  }
  let { onBrowseStore }: Props = $props();

  // ==================== 状态 ====================
  let apps = $state<InstalledApp[]>([]);
  let loading = $state(true);
  let error = $state("");
  let actionLoading = $state<Record<string, string>>({}); // name -> action

  // 日志
  let showLogs = $state(false);
  let logsAppName = $state("");
  let logsContent = $state("");
  let logsLoading = $state(false);

  // 卸载确认
  let showUninstall = $state(false);
  let uninstallTarget = $state<InstalledApp | null>(null);
  let uninstalling = $state(false);

  // ==================== 生命周期 ====================
  onMount(async () => {
    await loadApps();
  });

  // ==================== 方法 ====================
  async function loadApps() {
    loading = true;
    error = "";
    try {
      apps = await dockerStoreService.getInstalledApps();
    } catch (e: any) {
      error = e.message || $t("docker.myApps.loadInstalledFailed");
    } finally {
      loading = false;
    }
  }

  async function doAction(name: string, action: string, fn: () => Promise<void>) {
    actionLoading[name] = action;
    try {
      await fn();
      showToast($t("docker.myApps.actionSuccess", { values: { name, action: actionLabel(action) } }), "success");
      await loadApps();
    } catch (e: any) {
      showToast($t("docker.myApps.actionFailed", { values: { name, action: actionLabel(action) } }) + ": " + e.message, "error");
    } finally {
      delete actionLoading[name];
      actionLoading = { ...actionLoading };
    }
  }

  function actionLabel(action: string): string {
    const m: Record<string, string> = { start: $t("docker.myApps.start"), stop: $t("docker.myApps.stop"), restart: $t("docker.myApps.restart") };
    return m[action] || action;
  }

  function startApp(name: string) {
    doAction(name, "start", () => dockerStoreService.startApp(name));
  }
  function stopApp(name: string) {
    doAction(name, "stop", () => dockerStoreService.stopApp(name));
  }
  function restartApp(name: string) {
    doAction(name, "restart", () => dockerStoreService.restartApp(name));
  }

  async function viewLogs(app: InstalledApp) {
    logsAppName = app.name;
    logsContent = "";
    logsLoading = true;
    showLogs = true;
    try {
      logsContent = await dockerStoreService.getAppLogs(app.name, 200);
    } catch (e: any) {
      logsContent = $t("docker.myApps.getLogsFailed") + ": " + e.message;
    } finally {
      logsLoading = false;
    }
  }

  function confirmUninstall(app: InstalledApp) {
    uninstallTarget = app;
    showUninstall = true;
  }

  async function doUninstall() {
    if (!uninstallTarget) return;
    uninstalling = true;
    try {
      await dockerStoreService.uninstallApp(uninstallTarget.name);
      showToast(`${uninstallTarget.name} ${$t("docker.myApps.uninstalled")}`, "success");
      showUninstall = false;
      uninstallTarget = null;
      await loadApps();
      // 刷新外部应用列表（更新开始菜单和最近使用）
      refreshExternalApps();
    } catch (e: any) {
      showToast($t("docker.myApps.uninstallFailed") + ": " + e.message, "error");
    } finally {
      uninstalling = false;
    }
  }

  function statusColor(status: string): string {
    switch (status) {
      case "running": return "#27ae60";
      case "stopped": return "#e74c3c";
      default: return "#999";
    }
  }

  function statusText(status: string): string {
    const m: Record<string, string> = { running: $t("docker.myApps.statusRunning"), stopped: $t("docker.myApps.statusStopped"), unknown: $t("docker.myApps.statusUnknown") };
    return m[status] || status;
  }

  /** 解析展示用端口列表（兼容旧后端只返回 config） */
  function appPorts(app: InstalledApp): string[] {
    if (app.ports?.length) return app.ports;
    if (app.web_port) return [app.web_port];
    const cfg = app.config || {};
    const ports: string[] = [];
    const preferred = cfg["PANEL_APP_PORT_HTTP"];
    if (preferred != null && String(preferred).match(/^\d+$/)) {
      ports.push(String(preferred));
    }
    for (const [k, v] of Object.entries(cfg)) {
      if (!/PORT/i.test(k) || v == null) continue;
      const p = String(v);
      if (!/^\d{1,5}$/.test(p) || ports.includes(p)) continue;
      ports.push(p);
    }
    return ports;
  }

  function webPort(app: InstalledApp): string {
    if (app.web_port) return app.web_port;
    const ports = appPorts(app);
    return ports[0] || "";
  }

  function openApp(app: InstalledApp) {
    const port = webPort(app);
    if (!port) {
      showToast($t("docker.myApps.noPort"), "info");
      return;
    }
    const url = `${location.protocol}//${location.hostname}:${port}`;
    window.open(url, "_blank", "noopener,noreferrer");
  }

  function formatDate(dateStr: string): string {
    if (!dateStr) return "";
    try {
      const d = new Date(dateStr);
      return d.toLocaleDateString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" });
    } catch {
      return dateStr;
    }
  }

  function isActioning(name: string): boolean {
    return !!actionLoading[name];
  }
</script>

<div class="my-apps">
  {#if loading}
    <div class="loading">
      <Spinner center />
    </div>
  {:else if error}
    <EmptyState icon="mdi:alert-circle" title={$t("docker.myApps.loadFailed")} description={error} actionLabel={$t("docker.myApps.retry")} onaction={loadApps} />
  {:else if apps.length === 0}
    <EmptyState
      icon="mdi:package-variant-closed"
      title={$t("docker.myApps.noAppsInstalled")}
      description={$t("docker.myApps.browseAndInstall")}
      actionLabel={$t("docker.myApps.browseStore")}
      onaction={onBrowseStore}
    />
  {:else}
    <div class="panel-header">
      <span class="count">{$t("docker.myApps.installedApps", { values: { n: apps.length } })}</span>
      <Button variant="ghost" size="sm" onclick={loadApps}>
        {$t("docker.myApps.refresh")}
      </Button>
    </div>

    <div class="app-list">
      {#each apps as app (app.name)}
        <div class="app-card">
          <div class="app-top">
            <div class="app-identity">
              <img
                src={dockerStoreService.getIconUrl(app.icon || app.app_id + ".png")}
                alt={app.name}
                class="app-icon"
                onerror={(e) => { (e.target as HTMLImageElement).style.display = "none"; }}
              />
              <div>
                <div class="app-name">{app.name}</div>
                <div class="app-sub">
                  {app.app_id} · v{app.version}
                  {#if app.installed_at}
                    · {$t("docker.myApps.installedAt")} {formatDate(app.installed_at)}
                  {/if}
                </div>
              </div>
            </div>
            <div class="app-status" style="color: {statusColor(app.status)}">
              <span class="status-dot" style="background: {statusColor(app.status)}"></span>
              {statusText(app.status)}
            </div>
          </div>

          {#if appPorts(app).length}
            <div class="app-meta">
              <span class="meta-label">{$t("docker.myApps.ports")}</span>
              <span class="meta-ports">
                {#each appPorts(app) as port (port)}
                  <button
                    type="button"
                    class="port-chip"
                    class:primary={port === webPort(app)}
                    disabled={app.status !== "running"}
                    onclick={() => {
                      const url = `${location.protocol}//${location.hostname}:${port}`;
                      window.open(url, "_blank", "noopener,noreferrer");
                    }}
                    title={$t("docker.myApps.openPort", { values: { port } })}
                  >
                    :{port}
                  </button>
                {/each}
              </span>
            </div>
          {/if}

          <div class="app-actions">
            {#if isActioning(app.name)}
              <span class="action-loading">
                <Spinner />
                <span>{$t("docker.myApps.inProgress", { values: { action: actionLabel(actionLoading[app.name]) } })}</span>
              </span>
            {:else}
              {#if app.status === "running" && webPort(app)}
                <Button variant="primary" size="sm" onclick={() => openApp(app)}>
                  {$t("docker.myApps.open")}
                </Button>
              {/if}
              {#if app.status === "running"}
                <Button variant="ghost" size="sm" onclick={() => stopApp(app.name)}>
                  {$t("docker.myApps.stop")}
                </Button>
                <Button variant="ghost" size="sm" onclick={() => restartApp(app.name)}>
                  {$t("docker.myApps.restart")}
                </Button>
              {:else}
                <Button variant="ghost" size="sm" onclick={() => startApp(app.name)}>
                  {$t("docker.myApps.start")}
                </Button>
              {/if}
              <Button variant="ghost" size="sm" onclick={() => viewLogs(app)}>
                {$t("docker.myApps.logs")}
              </Button>
              <Button variant="ghost" size="sm" onclick={() => confirmUninstall(app)}>
                <Icon icon="mdi:delete-outline" width="14" />
              </Button>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- 日志 -->
<Modal bind:open={showLogs} title="{$t('docker.myApps.logs')} - {logsAppName}" size="lg">
  {#if logsLoading}
    <div class="logs-loading"><Spinner center /></div>
  {:else}
    <pre class="log-output">{logsContent || $t("docker.myApps.noLogs")}</pre>
  {/if}
</Modal>

<!-- 卸载确认 -->
<Modal bind:open={showUninstall} title={$t("docker.myApps.confirmUninstall")} size="sm">
  {#if uninstallTarget}
    <div class="uninstall-confirm">
      <p>{$t("common.confirm")}: <strong>{uninstallTarget.name}</strong>?</p>
      <p class="uninstall-warn">{$t("docker.myApps.uninstallWarn")}</p>
      <div class="uninstall-actions">
        <Button variant="ghost" onclick={() => (showUninstall = false)} disabled={uninstalling}>{$t("common.cancel")}</Button>
        <Button variant="primary" onclick={doUninstall} disabled={uninstalling}>
          {uninstalling ? $t("docker.myApps.uninstalling") : $t("docker.myApps.confirmUninstallBtn")}
        </Button>
      </div>
    </div>
  {/if}
</Modal>

<style>
  .my-apps {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .loading {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }

  .count {
    font-size: 13px;
    color: var(--text-muted, #999);
  }

  .app-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
    overflow-y: auto;
    flex: 1;
  }

  .app-card {
    background: var(--bg-card, white);
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 10px;
    padding: 14px 16px;
    transition: border-color 0.15s;
  }
  .app-card:hover {
    border-color: var(--border-color-hover, #ccc);
  }

  .app-top {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
  }

  .app-identity {
    display: flex;
    gap: 12px;
    align-items: center;
  }

  .app-icon {
    width: 40px;
    height: 40px;
    border-radius: 8px;
    object-fit: contain;
    background: var(--bg-secondary, #f5f5f5);
    flex-shrink: 0;
  }

  .app-name {
    font-weight: 600;
    font-size: 14px;
    color: var(--text-primary, #333);
  }

  .app-sub {
    font-size: 12px;
    color: var(--text-muted, #999);
    margin-top: 2px;
  }

  .app-meta {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 10px;
    flex-wrap: wrap;
  }

  .meta-label {
    font-size: 12px;
    color: var(--text-muted, #999);
  }

  .meta-ports {
    display: inline-flex;
    gap: 6px;
    flex-wrap: wrap;
  }

  .port-chip {
    appearance: none;
    border: 1px solid var(--border-color, #e0e0e0);
    background: var(--bg-secondary, #f5f5f5);
    color: var(--text-primary, #333);
    border-radius: 999px;
    padding: 2px 10px;
    font-size: 12px;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    cursor: pointer;
    line-height: 1.6;
  }
  .port-chip.primary {
    border-color: #3b82f6;
    color: #2563eb;
    background: rgba(59, 130, 246, 0.08);
  }
  .port-chip:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .port-chip:not(:disabled):hover {
    border-color: #3b82f6;
  }

  .app-status {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    font-size: 12px;
    font-weight: 500;
    white-space: nowrap;
  }

  .status-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .app-actions {
    display: flex;
    gap: 6px;
    margin-top: 10px;
    flex-wrap: wrap;
    align-items: center;
  }

  .action-loading {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    color: var(--text-muted, #999);
  }

  /* Logs */
  .logs-loading {
    padding: 40px 0;
  }

  .log-output {
    max-height: 500px;
    overflow: auto;
    padding: 16px;
    background: #1a1a2e;
    color: #eee;
    border-radius: 6px;
    font-size: 12px;
    white-space: pre-wrap;
    word-break: break-all;
    line-height: 1.6;
    font-family: "JetBrains Mono", "Fira Code", monospace;
  }

  /* Uninstall */
  .uninstall-confirm p {
    margin: 0 0 8px;
    font-size: 14px;
    color: var(--text-primary, #333);
  }

  .uninstall-warn {
    font-size: 13px !important;
    color: var(--color-error, #e74c3c) !important;
  }

  .uninstall-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 16px;
  }
</style>
