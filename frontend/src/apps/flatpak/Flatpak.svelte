<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import Icon from "@iconify/svelte";
  import { Button, Spinner } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import {
    flatpakService,
    type FlatpakApp,
    type DesktopStatus,
    type DesktopConfig,
    type SetupStatus,
  } from "./service";
  import AppStore from "./AppStore.svelte";

  // ==================== 状态 ====================

  let setupStatus = $state<SetupStatus | null>(null);
  let desktopStatus = $state<DesktopStatus | null>(null);
  let desktopConfig = $state<DesktopConfig | null>(null);
  let installedApps = $state<FlatpakApp[]>([]);
  let loading = $state(true);
  let setupRunning = $state(false);
  let setupFailed = $state(false);
  let setupError = $state("");
  let setupLogs = $state<string[]>([]);
  let currentTab = $state<"desktop" | "apps" | "store" | "settings">("apps");
  let searchQuery = $state("");
  let selectedApp = $state<FlatpakApp | null>(null);
  let launching = $state(false);
  let launchingAppId = $state<string | null>(null);
  let cancelSetup: (() => void) | null = null;

  let refreshTimer: ReturnType<typeof setInterval> | null = null;

  // GitHub 连通性检测
  let ghCheck = $state<{ accessible: boolean; latency_ms: number; error?: string } | null>(null);
  let ghChecking = $state(false);

  // 全屏
  let isFullscreen = $state(false);
  let desktopViewEl = $state<HTMLDivElement | null>(null);

  // VNC iframe 与重连
  let vncIframe = $state<HTMLIFrameElement | null>(null);
  let vncKey = $state(0);

  // 剪贴板
  let clipboardExpanded = $state(false);
  let clipboardText = $state("");
  let clipboardSending = $state(false);

  // 应用崩溃检测
  let prevRunningApps = $state<string[]>([]);

  // 过滤后的已安装应用
  let filteredApps = $derived.by(() => {
    if (!searchQuery) return installedApps;
    const q = searchQuery.toLowerCase();
    return installedApps.filter(
      (a) =>
        a.name.toLowerCase().includes(q) ||
        a.app_id.toLowerCase().includes(q) ||
        a.description?.toLowerCase().includes(q)
    );
  });

  // 是否需要初始化（环境未就绪）
  let needsSetup = $derived(!setupStatus?.ready);

  // ==================== 生命周期 ====================

  onMount(async () => {
    await refresh();
    refreshTimer = setInterval(refreshDesktopStatus, 5000);
    // 如果需要 setup，提前检测 GitHub 连通性
    if (!setupStatus?.ready) {
      checkGitHub();
    }
    // 监听全屏变化
    document.addEventListener("fullscreenchange", onFullscreenChange);
  });

  onDestroy(() => {
    if (refreshTimer) clearInterval(refreshTimer);
    if (cancelSetup) cancelSetup();
    document.removeEventListener("fullscreenchange", onFullscreenChange);
  });

  // 自动聚焦 iframe
  $effect(() => {
    if (currentTab === "desktop" && vncIframe && desktopStatus?.running) {
      // 延迟一帧确保 DOM 已更新
      requestAnimationFrame(() => {
        vncIframe?.focus();
      });
    }
  });

  // ==================== 方法 ====================

  async function refresh() {
    try {
      setupStatus = await flatpakService.getSetupStatus();
      if (setupStatus.ready) {
        const [status, apps, config] = await Promise.all([
          flatpakService.getDesktopStatus(),
          flatpakService.getInstalledApps(),
          flatpakService.getDesktopConfig(),
        ]);
        desktopStatus = status;
        installedApps = apps || [];
        desktopConfig = config;
        prevRunningApps = status.running_apps || [];
      }
    } catch (e: any) {
      console.error("Failed to load flatpak status:", e);
    } finally {
      loading = false;
    }
  }

  async function refreshDesktopStatus() {
    if (!setupStatus?.ready) return;
    try {
      const newStatus = await flatpakService.getDesktopStatus();

      // 应用崩溃检测：对比之前运行的应用
      if (desktopStatus?.running && newStatus.running) {
        const prevSet = new Set(prevRunningApps);
        const currentApps = newStatus.running_apps || [];
        for (const appId of prevSet) {
          if (!currentApps.includes(appId)) {
            // 应用从运行列表中消失 = 崩溃或退出
            const appName = installedApps.find(a => a.app_id === appId)?.name || appId;
            showToast(`${appName} 已退出`, "info");
          }
        }
        prevRunningApps = currentApps;
      }

      desktopStatus = newStatus;
    } catch {}
  }

  // ==================== 全屏 ====================

  function toggleFullscreen() {
    if (!desktopViewEl) return;
    if (!document.fullscreenElement) {
      desktopViewEl.requestFullscreen().catch(() => {});
    } else {
      document.exitFullscreen().catch(() => {});
    }
  }

  function onFullscreenChange() {
    isFullscreen = !!document.fullscreenElement;
  }

  // ==================== VNC 重连 ====================

  function reconnectVNC() {
    vncKey++;
  }

  // ==================== 剪贴板 ====================

  async function sendClipboard() {
    if (!clipboardText.trim()) return;
    clipboardSending = true;
    try {
      await flatpakService.setClipboard(clipboardText);
      showToast("已同步到桌面剪贴板", "success");
    } catch (e: any) {
      showToast(`同步失败: ${e.message}`, "error");
    } finally {
      clipboardSending = false;
    }
  }

  async function receiveClipboard() {
    clipboardSending = true;
    try {
      clipboardText = await flatpakService.getClipboard();
      showToast("已获取桌面剪贴板", "success");
    } catch (e: any) {
      showToast(`获取失败: ${e.message}`, "error");
    } finally {
      clipboardSending = false;
    }
  }

  function runSetup() {
    setupRunning = true;
    setupFailed = false;
    setupError = "";
    setupLogs = [];
    cancelSetup = flatpakService.runSetupStream(
      (line) => {
        setupLogs = [...setupLogs, line];
      },
      async (success, error) => {
        setupRunning = false;
        if (success) {
          showToast("环境配置完成！", "success");
          await refresh();
        } else {
          setupFailed = true;
          setupError = error || "未知错误";
          showToast(`配置失败: ${error}`, "error");
        }
      }
    );
  }

  async function checkGitHub() {
    ghChecking = true;
    try {
      const start = Date.now();
      const response = await fetch("https://github.com", { mode: "no-cors", signal: AbortSignal.timeout(5000) });
      ghCheck = { accessible: true, latency_ms: Date.now() - start };
    } catch (e) {
      ghCheck = { accessible: false, latency_ms: 0, error: "GitHub 不可达" };
    } finally {
      ghChecking = false;
    }
  }

  async function startDesktop() {
    try {
      await flatpakService.startDesktop();
      showToast("桌面已启动", "success");
      await refreshDesktopStatus();
    } catch (e: any) {
      showToast(`启动失败: ${e.message}`, "error");
    }
  }

  async function stopDesktop() {
    try {
      await flatpakService.stopDesktop();
      showToast("桌面已停止", "success");
      desktopStatus = await flatpakService.getDesktopStatus();
    } catch (e: any) {
      showToast(`停止失败: ${e.message}`, "error");
    }
  }

  async function restartDesktop() {
    try {
      await flatpakService.restartDesktop();
      showToast("桌面已重启", "success");
      await refreshDesktopStatus();
    } catch (e: any) {
      showToast(`重启失败: ${e.message}`, "error");
    }
  }

  async function runApp(app: FlatpakApp) {
    launching = true;
    launchingAppId = app.app_id;
    try {
      showToast(`正在启动 ${app.name}...`, "info");
      await flatpakService.runApp(app.app_id);
      showToast(`${app.name} 已启动`, "success");
      // 切换到桌面标签
      currentTab = "desktop";
      await refreshDesktopStatus();
    } catch (e: any) {
      showToast(`启动失败: ${e.message}`, "error");
    } finally {
      launching = false;
      launchingAppId = null;
    }
  }

  async function uninstallApp(app: FlatpakApp) {
    try {
      await flatpakService.uninstallApp(app.app_id);
      showToast(`${app.name} 已卸载`, "success");
      installedApps = await flatpakService.getInstalledApps();
    } catch (e: any) {
      showToast(`卸载失败: ${e.message}`, "error");
    }
  }

  async function onAppInstalled() {
    installedApps = (await flatpakService.getInstalledApps()) || [];
  }

  // ==================== 设置 ====================

  async function saveConfigField(field: string, value: any) {
    if (!desktopConfig) return;
    const updated = { ...desktopConfig, [field]: value };
    desktopConfig = updated;
    try {
      await flatpakService.updateDesktopConfig(updated);
      showToast("设置已保存", "success");
    } catch (e: any) {
      showToast(`保存失败: ${e.message}`, "error");
    }
  }
</script>

<div class="flatpak-container">
  {#if loading}
    <!-- 加载中 -->
    <div class="loading-state">
      <Spinner size="lg" />
      <p>加载中...</p>
    </div>
  {:else if needsSetup && !setupRunning && !setupFailed}
    <!-- 环境初始化向导 -->
    <div class="setup-wizard">
      <div class="setup-header">
        <Icon icon="mdi:package-variant" width={48} />
        <h2>Flatpak 应用</h2>
        <p>需要配置运行环境才能使用 Flatpak 应用。将自动安装 KasmVNC、Flatpak 和相关依赖。</p>
      </div>

      <div class="setup-checklist">
        <div class="check-item" class:ok={setupStatus?.kasmvnc_installed}>
          <Icon icon={setupStatus?.kasmvnc_installed ? "mdi:check-circle" : "mdi:circle-outline"} width={20} />
          <span>KasmVNC {setupStatus?.kasmvnc_expected || ""}</span>
        </div>
        <div class="check-item" class:ok={setupStatus?.flatpak_installed}>
          <Icon icon={setupStatus?.flatpak_installed ? "mdi:check-circle" : "mdi:circle-outline"} width={20} />
          <span>Flatpak</span>
        </div>
        <div class="check-item" class:ok={setupStatus?.openbox_installed}>
          <Icon icon={setupStatus?.openbox_installed ? "mdi:check-circle" : "mdi:circle-outline"} width={20} />
          <span>Openbox 窗口管理器</span>
        </div>
        <div class="check-item" class:ok={setupStatus?.pulseaudio_installed}>
          <Icon icon={setupStatus?.pulseaudio_installed ? "mdi:check-circle" : "mdi:circle-outline"} width={20} />
          <span>PulseAudio 音频</span>
        </div>
        <div class="check-item" class:ok={setupStatus?.virtual_sink_ready}>
          <Icon icon={setupStatus?.virtual_sink_ready ? "mdi:check-circle" : "mdi:circle-outline"} width={20} />
          <span>虚拟声卡</span>
        </div>
      </div>

      <!-- GitHub 连通性检测提示 -->
      {#if ghChecking}
        <div class="gh-check-banner info">
          <Spinner size="sm" />
          <span>正在检测 GitHub 连通性...</span>
        </div>
      {:else if ghCheck && !ghCheck.accessible}
        <div class="gh-check-banner warning">
          <Icon icon="mdi:alert" width={20} />
          <div class="gh-check-text">
            <strong>GitHub 不可达</strong>
            <p>KasmVNC 需要从 GitHub 下载，当前网络无法访问 GitHub。请检查网络连接或配置代理。</p>
          </div>
        </div>
      {:else if ghCheck && ghCheck.accessible}
        <div class="gh-check-banner success">
          <Icon icon="mdi:check-circle" width={20} />
          <span>GitHub 可达 ({ghCheck.latency_ms}ms)，可以正常安装</span>
        </div>
      {/if}

      <Button variant="primary" onclick={runSetup}>
        <Icon icon="mdi:download" width={18} />
        开始配置
      </Button>
    </div>
  {:else if setupRunning || setupFailed}
    <!-- 安装进度 -->
    <div class="setup-progress">
      <div class="progress-header">
        {#if setupRunning}
          <Spinner size="sm" />
          <h3>正在配置环境...</h3>
        {:else}
          <Icon icon="mdi:alert-circle" width={24} class="error-icon" />
          <h3 class="error-title">配置失败</h3>
        {/if}
      </div>
      {#if setupFailed}
        <div class="setup-error-banner">
          <Icon icon="mdi:alert" width={18} />
          <span>{setupError}</span>
        </div>
      {/if}
      <div class="log-output">
        {#each setupLogs as line}
          <div class="log-line" class:error-line={line.startsWith('✗') || line.startsWith('⚠')}>{line}</div>
        {/each}
      </div>
      {#if setupFailed}
        <div class="setup-retry-actions">
          <Button variant="primary" onclick={runSetup}>
            <Icon icon="mdi:refresh" width={18} />
            重试
          </Button>
          <Button variant="ghost" onclick={() => { setupFailed = false; }}>
            返回
          </Button>
        </div>
      {/if}
    </div>
  {:else}
    <!-- 主界面 -->
    <div class="main-layout">
      <!-- 顶部标签栏 -->
      <div class="tab-bar">
        <button
          class="tab-btn"
          class:active={currentTab === "apps"}
          onclick={() => (currentTab = "apps")}
        >
          <Icon icon="mdi:apps" width={18} />
          已安装
          {#if installedApps.length > 0}
            <span class="badge">{installedApps.length}</span>
          {/if}
        </button>
        <button
          class="tab-btn"
          class:active={currentTab === "desktop"}
          onclick={() => (currentTab = "desktop")}
        >
          <Icon icon="mdi:monitor" width={18} />
          桌面
        </button>
        <button
          class="tab-btn"
          class:active={currentTab === "store"}
          onclick={() => (currentTab = "store")}
        >
          <Icon icon="mdi:store" width={18} />
          应用商店
        </button>
        <button
          class="tab-btn"
          class:active={currentTab === "settings"}
          onclick={() => (currentTab = "settings")}
        >
          <Icon icon="mdi:cog" width={18} />
          设置
        </button>

        <!-- 右侧状态 -->
        <div class="tab-bar-right">
          {#if desktopStatus?.running}
            <span class="status-dot running"></span>
            <span class="status-text">桌面运行中</span>
          {:else}
            <span class="status-dot stopped"></span>
            <span class="status-text">桌面已停止</span>
          {/if}
        </div>
      </div>

      <!-- 内容区 -->
      <div class="tab-content">
        {#if currentTab === "desktop"}
          <!-- 桌面视图 -->
          <div class="desktop-view" bind:this={desktopViewEl}>
            {#if desktopStatus?.running}
              <div class="vnc-toolbar">
                <div class="vnc-info">
                  <Icon icon="mdi:monitor" width={16} />
                  <span>{desktopStatus.resolution}</span>
                  <span class="separator">|</span>
                  <span>KasmVNC {desktopStatus.kasmvnc_version}</span>
                  {#if desktopStatus.running_apps?.length > 0}
                    <span class="separator">|</span>
                    <span>{desktopStatus.running_apps.length} 个应用运行中</span>
                  {/if}
                </div>
                <div class="vnc-actions">
                  <span title="剪贴板">
                    <Button size="sm" variant="ghost" onclick={() => clipboardExpanded = !clipboardExpanded}>
                      <Icon icon="mdi:clipboard-text" width={16} />
                    </Button>
                  </span>
                  <span title="重新连接">
                    <Button size="sm" variant="ghost" onclick={reconnectVNC}>
                      <Icon icon="mdi:refresh" width={16} />
                    </Button>
                  </span>
                  <span title={isFullscreen ? "退出全屏" : "全屏"}>
                    <Button size="sm" variant="ghost" onclick={toggleFullscreen}>
                      <Icon icon={isFullscreen ? "mdi:fullscreen-exit" : "mdi:fullscreen"} width={16} />
                    </Button>
                  </span>
                  <span class="separator">|</span>
                  <Button size="sm" variant="ghost" onclick={restartDesktop}>
                    <Icon icon="mdi:restart" width={16} />
                    重启
                  </Button>
                  <Button size="sm" variant="ghost" onclick={stopDesktop}>
                    <Icon icon="mdi:stop" width={16} />
                    停止
                  </Button>
                </div>
              </div>
              {#if clipboardExpanded}
                <div class="clipboard-panel">
                  <textarea
                    class="clipboard-textarea"
                    placeholder="在此粘贴文本，然后点击「发送到桌面」..."
                    bind:value={clipboardText}
                    rows="3"
                  ></textarea>
                  <div class="clipboard-actions">
                    <Button size="sm" variant="primary" onclick={sendClipboard} disabled={clipboardSending || !clipboardText.trim()}>
                      <Icon icon="mdi:send" width={14} />
                      发送到桌面
                    </Button>
                    <Button size="sm" variant="ghost" onclick={receiveClipboard} disabled={clipboardSending}>
                      <Icon icon="mdi:download" width={14} />
                      从桌面获取
                    </Button>
                    <span class="clipboard-hint">浏览器无法直接粘贴到 VNC 时可使用此功能</span>
                  </div>
                </div>
              {/if}
              <div class="vnc-frame-wrapper">
                {#if !desktopStatus?.running_apps?.length}
                  <div class="desktop-empty-hint">
                    <Icon icon="mdi:application-outline" width={48} />
                    <p>桌面已启动，暂无运行中的应用</p>
                    <p class="hint-sub">前往「已安装」标签页启动应用，窗口将在此处显示</p>
                    <Button size="sm" variant="primary" onclick={() => currentTab = 'apps'}>
                      <Icon icon="mdi:apps" width={16} />
                      打开已安装应用
                    </Button>
                  </div>
                {/if}
                {#key vncKey}
                  <iframe
                    bind:this={vncIframe}
                    src={flatpakService.getVNCUrl()}
                    title="KasmVNC Desktop"
                    class="vnc-frame"
                    allow="clipboard-read; clipboard-write; fullscreen"
                  ></iframe>
                {/key}
              </div>
            {:else}
              <div class="desktop-stopped">
                <Icon icon="mdi:monitor-off" width={64} />
                <h3>桌面未运行</h3>
                <p>启动桌面以使用 Flatpak GUI 应用</p>
                <Button variant="primary" onclick={startDesktop}>
                  <Icon icon="mdi:play" width={18} />
                  启动桌面
                </Button>
              </div>
            {/if}
          </div>
        {:else if currentTab === "apps"}
          <!-- 已安装应用 -->
          <div class="apps-view">
            <div class="apps-toolbar">
              <div class="search-box">
                <Icon icon="mdi:magnify" width={18} />
                <input
                  type="text"
                  placeholder="搜索已安装应用..."
                  bind:value={searchQuery}
                />
              </div>
            </div>

            {#if filteredApps.length === 0}
              <div class="empty-state">
                <Icon icon="mdi:package-variant" width={48} />
                <p>{searchQuery ? "没有匹配的应用" : "还没有安装 Flatpak 应用"}</p>
                {#if !searchQuery}
                  <Button variant="ghost" onclick={() => (currentTab = "store")}>
                    前往应用商店
                  </Button>
                {/if}
              </div>
            {:else}
              <div class="app-grid">
                {#each filteredApps as app}
                  <div class="app-card" class:running={app.running}>
                    <div class="app-icon">
                      {#if app.icon}
                        <img src={flatpakService.getIconUrl(app.app_id)} alt={app.name} />
                      {:else}
                        <Icon icon="mdi:package-variant" width={40} />
                      {/if}
                    </div>
                    <div class="app-info">
                      <div class="app-name">
                        {app.name}
                        {#if app.running}
                          <span class="running-badge">运行中</span>
                        {/if}
                      </div>
                      <div class="app-desc">{app.description || app.app_id}</div>
                      {#if app.version}
                        <div class="app-version">v{app.version}</div>
                      {/if}
                    </div>
                    <div class="app-actions">
                      <Button
                        size="sm"
                        variant="primary"
                        onclick={() => runApp(app)}
                        disabled={launching || !desktopStatus?.running}
                      >
                        {#if launchingAppId === app.app_id}
                          <Spinner size="sm" />
                          启动中...
                        {:else}
                          <Icon icon="mdi:play" width={14} />
                          {app.running ? "再次打开" : "运行"}
                        {/if}
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        onclick={() => uninstallApp(app)}
                      >
                        <Icon icon="mdi:delete" width={14} />
                      </Button>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        {:else if currentTab === "store"}
          <!-- 应用商店 -->
          <AppStore onInstalled={onAppInstalled} />
        {:else if currentTab === "settings"}
          <!-- 设置 -->
          <div class="settings-view">
            <h3>桌面设置</h3>
            {#if desktopConfig}
              <div class="setting-group">
                <div class="setting-item">
                  <div class="setting-label">
                    <span>自动启动桌面</span>
                    <span class="setting-desc">模块启动时自动启动 KasmVNC 桌面</span>
                  </div>
                  <input
                    type="checkbox"
                    checked={desktopConfig.auto_start}
                    onchange={(e) => saveConfigField("auto_start", (e.target as HTMLInputElement).checked)}
                  />
                </div>
                <div class="setting-item">
                  <div class="setting-label">
                    <span>分辨率</span>
                    <span class="setting-desc">桌面默认分辨率（修改后需重启桌面生效）</span>
                  </div>
                  <select
                    value={desktopConfig.default_resolution}
                    onchange={(e) => saveConfigField("default_resolution", (e.target as HTMLSelectElement).value)}
                  >
                    <option value="1920x1080">1920x1080</option>
                    <option value="1280x720">1280x720</option>
                    <option value="1600x900">1600x900</option>
                    <option value="2560x1440">2560x1440</option>
                  </select>
                </div>
                <div class="setting-item">
                  <div class="setting-label">
                    <span>音频</span>
                    <span class="setting-desc">启用应用音频转发</span>
                  </div>
                  <input
                    type="checkbox"
                    checked={desktopConfig.audio_enabled}
                    onchange={(e) => saveConfigField("audio_enabled", (e.target as HTMLInputElement).checked)}
                  />
                </div>
                <div class="setting-item">
                  <div class="setting-label">
                    <span>剪贴板同步</span>
                    <span class="setting-desc">同步浏览器和桌面剪贴板</span>
                  </div>
                  <input
                    type="checkbox"
                    checked={desktopConfig.clipboard_sync}
                    onchange={(e) => saveConfigField("clipboard_sync", (e.target as HTMLInputElement).checked)}
                  />
                </div>
              </div>
            {:else}
              <Spinner size="sm" />
            {/if}
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .flatpak-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--bg-window);
    color: var(--text-primary);
  }

  /* Loading */
  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 12px;
    color: var(--text-secondary);
  }

  /* Setup Wizard */
  .setup-wizard {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 24px;
    padding: 40px;
  }

  .setup-header {
    text-align: center;
  }

  .setup-header h2 {
    margin: 12px 0 8px;
    font-size: 24px;
  }

  .setup-header p {
    color: var(--text-secondary);
    max-width: 400px;
    line-height: 1.5;
  }

  .setup-checklist {
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-width: 250px;
  }

  .check-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    border-radius: 6px;
    background: var(--bg-tertiary);
    color: var(--text-secondary);
  }

  .check-item.ok {
    color: var(--color-success, #4caf50);
  }

  /* GitHub check banner */
  .gh-check-banner {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 12px 16px;
    border-radius: 8px;
    font-size: 14px;
    max-width: 480px;
    width: 100%;
  }

  .gh-check-banner.info {
    background: rgba(59, 130, 246, 0.1);
    color: var(--text-secondary);
  }

  .gh-check-banner.warning {
    background: rgba(245, 158, 11, 0.1);
    color: #f59e0b;
  }

  .gh-check-banner.success {
    background: rgba(34, 197, 94, 0.1);
    color: #22c55e;
  }

  .gh-check-text {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .gh-check-text p {
    margin: 0;
    font-size: 13px;
    color: var(--text-secondary);
  }

  .gh-check-text .link-btn {
    background: none;
    border: none;
    color: var(--accent-color, #8b5cf6);
    cursor: pointer;
    text-decoration: underline;
    padding: 0;
    font-size: 13px;
  }

  .gh-proxy-ok {
    display: flex;
    align-items: center;
    gap: 4px;
    color: #22c55e !important;
  }

  /* Setup Progress */
  .setup-progress {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 24px;
  }

  .progress-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 16px;
  }

  .log-output {
    flex: 1;
    background: var(--bg-secondary);
    border-radius: 8px;
    padding: 12px;
    overflow-y: auto;
    font-family: "JetBrains Mono", "Fira Code", monospace;
    font-size: 12px;
    line-height: 1.6;
  }

  .log-line {
    white-space: pre-wrap;
    word-break: break-all;
    color: var(--text-secondary);
  }

  .log-line.error-line {
    color: #f59e0b;
  }

  .error-title {
    color: #ef4444;
  }

  :global(.error-icon) {
    color: #ef4444;
  }

  .setup-error-banner {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 14px;
    margin-bottom: 12px;
    border-radius: 8px;
    background: rgba(239, 68, 68, 0.15);
    color: #ef4444;
    font-size: 13px;
  }

  .setup-retry-actions {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 16px;
    justify-content: center;
  }

  /* Main Layout */
  .main-layout {
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  /* Tab Bar */
  .tab-bar {
    display: flex;
    align-items: center;
    padding: 0 12px;
    height: 42px;
    border-bottom: 1px solid var(--border-color);
    background: var(--bg-secondary);
    flex-shrink: 0;
  }

  .tab-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 14px;
    border: none;
    background: none;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 13px;
    border-bottom: 2px solid transparent;
    transition: all 0.15s;
  }

  .tab-btn:hover {
    color: var(--text-primary);
  }

  .tab-btn.active {
    color: var(--color-primary);
    border-bottom-color: var(--color-primary);
  }

  .badge {
    background: var(--color-primary);
    color: #fff;
    font-size: 11px;
    padding: 1px 6px;
    border-radius: 10px;
    min-width: 18px;
    text-align: center;
  }

  .tab-bar-right {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }

  .status-dot.running {
    background: var(--color-success, #4caf50);
    box-shadow: 0 0 6px var(--color-success, #4caf50);
  }

  .status-dot.stopped {
    background: var(--text-muted);
  }

  .status-text {
    color: var(--text-secondary);
  }

  /* Tab Content */
  .tab-content {
    flex: 1;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  /* Desktop View */
  .desktop-view {
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .vnc-toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 12px;
    background: var(--bg-tertiary);
    border-bottom: 1px solid var(--border-color);
    font-size: 12px;
    flex-shrink: 0;
  }

  .vnc-info {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--text-secondary);
  }

  .separator {
    opacity: 0.3;
  }

  .vnc-actions {
    display: flex;
    gap: 4px;
  }

  .vnc-frame-wrapper {
    flex: 1;
    position: relative;
    overflow: hidden;
  }

  .vnc-frame {
    width: 100%;
    height: 100%;
    border: none;
    background: #000;
  }

  /* Clipboard Panel */
  .clipboard-panel {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 8px 12px;
    background: var(--bg-tertiary);
    border-bottom: 1px solid var(--border-color);
    flex-shrink: 0;
  }

  .clipboard-textarea {
    width: 100%;
    background: var(--bg-input);
    border: 1px solid var(--border-color);
    border-radius: 6px;
    padding: 8px;
    color: var(--text-primary);
    font-size: 13px;
    resize: vertical;
    font-family: inherit;
  }

  .clipboard-textarea::placeholder {
    color: var(--text-muted);
  }

  .clipboard-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .clipboard-hint {
    font-size: 11px;
    color: var(--text-muted);
    margin-left: auto;
  }

  /* Fullscreen */
  .desktop-view:fullscreen {
    background: #000;
  }

  .desktop-view:fullscreen .vnc-toolbar {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    z-index: 10;
    opacity: 0;
    transition: opacity 0.2s;
  }

  .desktop-view:fullscreen .vnc-toolbar:hover {
    opacity: 1;
  }

  .desktop-view:fullscreen .clipboard-panel {
    position: absolute;
    top: 42px;
    left: 0;
    right: 0;
    z-index: 10;
  }

  .desktop-view:fullscreen .vnc-frame-wrapper {
    height: 100%;
  }

  .desktop-stopped {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 12px;
    color: var(--text-secondary);
  }

  .desktop-stopped h3 {
    color: var(--text-primary);
    margin: 0;
  }

  .desktop-stopped p {
    margin: 0 0 8px;
  }

  .desktop-empty-hint {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    color: var(--text-secondary);
    background: var(--bg-primary);
    z-index: 2;
  }

  .desktop-empty-hint p {
    margin: 0;
    font-size: 14px;
  }

  .desktop-empty-hint .hint-sub {
    font-size: 12px;
    opacity: 0.7;
    margin-bottom: 8px;
  }

  /* Apps View */
  .apps-view {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .apps-toolbar {
    display: flex;
    padding: 12px;
    flex-shrink: 0;
  }

  .search-box {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--bg-input);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    padding: 6px 12px;
    flex: 1;
    max-width: 400px;
  }

  .search-box input {
    border: none;
    background: none;
    outline: none;
    color: var(--text-primary);
    width: 100%;
    font-size: 13px;
  }

  .search-box input::placeholder {
    color: var(--text-muted);
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    flex: 1;
    gap: 12px;
    color: var(--text-muted);
  }

  .app-grid {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 0 12px 12px;
    overflow-y: auto;
  }

  .app-card {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    border-radius: 8px;
    background: var(--bg-card);
    transition: all 0.15s;
  }

  .app-card:hover {
    background: var(--bg-hover);
  }

  .app-card.running {
    border-left: 3px solid var(--color-success, #4caf50);
  }

  .app-icon {
    width: 40px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .app-icon img {
    width: 100%;
    height: 100%;
    object-fit: contain;
    border-radius: 8px;
  }

  .app-info {
    flex: 1;
    min-width: 0;
  }

  .app-name {
    font-size: 13px;
    font-weight: 500;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .running-badge {
    font-size: 10px;
    padding: 1px 6px;
    background: var(--color-success, #4caf50);
    color: #fff;
    border-radius: 4px;
  }

  .app-desc {
    font-size: 12px;
    color: var(--text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .app-version {
    font-size: 11px;
    color: var(--text-muted);
  }

  .app-actions {
    display: flex;
    gap: 4px;
    flex-shrink: 0;
  }

  /* Settings */
  .settings-view {
    padding: 24px;
    overflow-y: auto;
  }

  .settings-view h3 {
    margin: 0 0 16px;
    font-size: 16px;
  }

  .setting-group {
    display: flex;
    flex-direction: column;
    gap: 2px;
    background: var(--bg-tertiary);
    border-radius: 8px;
    overflow: hidden;
  }

  .setting-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 16px;
  }

  .setting-label span:first-child {
    font-size: 13px;
    display: block;
  }

  .setting-desc {
    font-size: 11px;
    color: var(--text-muted);
    display: block;
    margin-top: 2px;
  }

  .setting-item select {
    background: var(--bg-input);
    border: 1px solid var(--border-color);
    color: var(--text-primary);
    padding: 4px 8px;
    border-radius: 4px;
    font-size: 13px;
  }
</style>
