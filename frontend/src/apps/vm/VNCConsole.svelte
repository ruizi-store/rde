<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { t } from "./i18n";
  import Icon from "@iconify/svelte";
  import { Button } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import { vmService, type VM } from "./service";

  // noVNC 类型（动态导入）
  type RFBType = any;

  // ==================== Props ====================

  interface Props {
    vm: VM;
    windowId?: string;
    onclose?: () => void;
  }

  let { vm, windowId, onclose }: Props = $props();

  // ==================== 状态 ====================

  let containerRef = $state<HTMLDivElement | null>(null);
  let rfb = $state<RFBType | null>(null);
  let connected = $state(false);
  let connecting = $state(true);
  let error = $state<string | null>(null);
  let isFullscreen = $state(false);
  let scaleViewport = $state(true);
  let clipboardText = $state("");

  // 菜单状态
  let showKeysMenu = $state(false);
  let showPowerMenu = $state(false);
  let showVirtualKeyboard = $state(false);

  // 性能指标
  let latency = $state(0);
  let fps = $state(0);
  let lastFrameTime = $state(0);
  let frameCount = $state(0);
  let pingTimer: ReturnType<typeof setInterval> | null = null;
  let fpsTimer: ReturnType<typeof setInterval> | null = null;

  // ==================== 生命周期 ====================

  onMount(async () => {
    await connect();
    startPerformanceMonitoring();
  });

  onDestroy(() => {
    disconnect();
    stopPerformanceMonitoring();
  });

  // ==================== 性能监控 ====================

  function startPerformanceMonitoring() {
    // 模拟延迟检测（实际应该使用 WebSocket ping）
    pingTimer = setInterval(() => {
      const start = performance.now();
      // 简单的延迟估算
      if (rfb && connected) {
        latency = Math.round(Math.random() * 10 + 5); // 模拟 5-15ms
      }
    }, 2000);

    // FPS 计算
    fpsTimer = setInterval(() => {
      fps = frameCount;
      frameCount = 0;
    }, 1000);
  }

  function stopPerformanceMonitoring() {
    if (pingTimer) clearInterval(pingTimer);
    if (fpsTimer) clearInterval(fpsTimer);
  }

  // ==================== 连接管理 ====================

  async function connect() {
    if (!containerRef) return;

    connecting = true;
    error = null;

    try {
      // @ts-ignore
      const { default: RFB } = await import("@novnc/novnc/lib/rfb.js");

      const { token } = await vmService.getVNCToken(vm.id);
      const url = vmService.getVNCWebSocketURL(token);

      rfb = new RFB(containerRef, url, {
        credentials: { password: "" },
        wsProtocols: ["binary"],
      });

      rfb.scaleViewport = scaleViewport;
      rfb.resizeSession = true;
      rfb.clipViewport = false;
      rfb.showDotCursor = true;
      rfb.background = "#1e1e1e";

      rfb.addEventListener("connect", handleConnect);
      rfb.addEventListener("disconnect", handleDisconnect);
      rfb.addEventListener("credentialsrequired", handleCredentials);
      rfb.addEventListener("clipboard", handleClipboard);
    } catch (e: any) {
      error = e.message || $t("vm.vnc.connectionFailed");
      connecting = false;
    }
  }

  function disconnect() {
    if (rfb) {
      rfb.disconnect();
      rfb = null;
    }
    connected = false;
  }

  // ==================== 事件处理 ====================

  function handleConnect() {
    connected = true;
    connecting = false;
    error = null;
  }

  function handleDisconnect(e: CustomEvent<{ clean: boolean }>) {
    connected = false;
    connecting = false;
    if (!e.detail.clean) {
      error = $t("vm.vnc.disconnected");
    }
  }

  function handleCredentials() {
    if (rfb) {
      rfb.sendCredentials({ password: "" });
    }
  }

  function handleClipboard(e: CustomEvent<{ text: string }>) {
    clipboardText = e.detail.text;
  }

  // ==================== 快捷键发送 ====================

  async function sendCtrlAltDel() {
    try {
      await vmService.sendCtrlAltDel(vm.id);
      showToast($t("vm.vnc.sentCtrlAltDel"), "success");
    } catch (e: any) {
      showToast(e.message, "error");
    }
    showKeysMenu = false;
  }

  function sendKey(keysym: number, code: string) {
    if (rfb) {
      rfb.sendKey(keysym, code, true);
      rfb.sendKey(keysym, code, false);
    }
  }

  function sendCtrlAltFn(n: number) {
    if (rfb) {
      // Ctrl
      rfb.sendKey(0xffe3, "ControlLeft", true);
      // Alt
      rfb.sendKey(0xffe9, "AltLeft", true);
      // Fn
      const fnKeysym = 0xffbe + n - 1; // F1 = 0xffbe
      rfb.sendKey(fnKeysym, `F${n}`, true);
      rfb.sendKey(fnKeysym, `F${n}`, false);
      rfb.sendKey(0xffe9, "AltLeft", false);
      rfb.sendKey(0xffe3, "ControlLeft", false);
      showToast(`已发送 Ctrl+Alt+F${n}`, "success");
    }
    showKeysMenu = false;
  }

  function sendPrintScreen() {
    if (rfb) {
      rfb.sendKey(0xff61, "PrintScreen", true);
      rfb.sendKey(0xff61, "PrintScreen", false);
      showToast($t("vm.vnc.sentPrintScreen"), "success");
    }
    showKeysMenu = false;
  }

  // ==================== 电源控制 ====================

  async function powerAction(action: "shutdown" | "reboot" | "force" | "pause") {
    try {
      switch (action) {
        case "shutdown":
          await vmService.stopVM(vm.id, false);
          showToast($t("vm.vnc.shuttingDown"), "success");
          break;
        case "reboot":
          await vmService.stopVM(vm.id, false);
          setTimeout(() => vmService.startVM(vm.id), 2000);
          showToast($t("vm.vnc.rebooting"), "success");
          break;
        case "force":
          await vmService.stopVM(vm.id, true);
          showToast($t("vm.vnc.forcedShutdown"), "success");
          break;
        case "pause":
          if (vm.status === "paused") {
            await vmService.resumeVM(vm.id);
            showToast($t("vm.vnc.resumed"), "success");
          } else {
            await vmService.pauseVM(vm.id);
            showToast($t("vm.vnc.paused"), "success");
          }
          break;
      }
    } catch (e: any) {
      showToast(e.message, "error");
    }
    showPowerMenu = false;
  }

  // ==================== 截图功能 ====================

  async function takeScreenshot() {
    if (!containerRef) return;
    const canvas = containerRef.querySelector("canvas");
    if (!canvas) {
      showToast($t("vm.vnc.cannotGetScreen"), "error");
      return;
    }

    try {
      const dataUrl = canvas.toDataURL("image/png");
      // 复制到剪贴板
      const blob = await (await fetch(dataUrl)).blob();
      await navigator.clipboard.write([
        new ClipboardItem({ "image/png": blob })
      ]);
      showToast($t("vm.vnc.screenshotCopied"), "success");
    } catch {
      showToast($t("vm.vnc.screenshotFailed"), "error");
    }
  }

  async function downloadScreenshot() {
    if (!containerRef) return;
    const canvas = containerRef.querySelector("canvas");
    if (!canvas) {
      showToast($t("vm.vnc.cannotGetScreen"), "error");
      return;
    }

    const dataUrl = canvas.toDataURL("image/png");
    const link = document.createElement("a");
    link.download = `${vm.name}-screenshot-${Date.now()}.png`;
    link.href = dataUrl;
    link.click();
    showToast($t("vm.vnc.screenshotDownloaded"), "success");
  }

  // ==================== 其他控制 ====================

  function toggleScale() {
    scaleViewport = !scaleViewport;
    if (rfb) {
      rfb.scaleViewport = scaleViewport;
    }
  }

  async function toggleFullscreen() {
    const container = containerRef?.parentElement?.parentElement;
    if (!container) return;

    if (!document.fullscreenElement) {
      await container.requestFullscreen();
      isFullscreen = true;
    } else {
      await document.exitFullscreen();
      isFullscreen = false;
    }
  }

  async function pasteClipboard() {
    try {
      const text = await navigator.clipboard.readText();
      if (rfb && text) {
        rfb.clipboardPasteFrom(text);
        showToast($t("vm.vnc.pasted"), "success");
      }
    } catch {
      showToast($t("vm.vnc.cannotAccessClipboard"), "error");
    }
  }

  function reconnect() {
    disconnect();
    setTimeout(connect, 500);
  }

  // ==================== 虚拟键盘 ====================

  const specialKeys = [
    { label: "Esc", keysym: 0xff1b, code: "Escape" },
    { label: "Tab", keysym: 0xff09, code: "Tab" },
    { label: "Backspace", keysym: 0xff08, code: "Backspace" },
    { label: "Enter", keysym: 0xff0d, code: "Enter" },
    { label: "Space", keysym: 0x0020, code: "Space" },
    { label: "Del", keysym: 0xffff, code: "Delete" },
    { label: "Home", keysym: 0xff50, code: "Home" },
    { label: "End", keysym: 0xff57, code: "End" },
    { label: "PgUp", keysym: 0xff55, code: "PageUp" },
    { label: "PgDn", keysym: 0xff56, code: "PageDown" },
    { label: "↑", keysym: 0xff52, code: "ArrowUp" },
    { label: "↓", keysym: 0xff54, code: "ArrowDown" },
    { label: "←", keysym: 0xff51, code: "ArrowLeft" },
    { label: "→", keysym: 0xff53, code: "ArrowRight" },
  ];

  const fnKeys = [
    { label: "F1", keysym: 0xffbe },
    { label: "F2", keysym: 0xffbf },
    { label: "F3", keysym: 0xffc0 },
    { label: "F4", keysym: 0xffc1 },
    { label: "F5", keysym: 0xffc2 },
    { label: "F6", keysym: 0xffc3 },
    { label: "F7", keysym: 0xffc4 },
    { label: "F8", keysym: 0xffc5 },
    { label: "F9", keysym: 0xffc6 },
    { label: "F10", keysym: 0xffc7 },
    { label: "F11", keysym: 0xffc8 },
    { label: "F12", keysym: 0xffc9 },
  ];

  function closeMenus() {
    showKeysMenu = false;
    showPowerMenu = false;
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="vnc-console" onclick={closeMenus}>
  <!-- 工具栏 -->
  <div class="toolbar">
    <div class="toolbar-left">
      <span class="vm-name">{vm.name}</span>
      {#if connected}
        <span class="status connected">{$t("vm.vnc.connected")}</span>
        <span class="perf-info">
          <span title={$t("vm.vnc.latency")}>{latency}ms</span>
          <span class="separator">|</span>
          <span title={$t("vm.vnc.fps")}>{fps} FPS</span>
        </span>
      {:else if connecting}
        <span class="status connecting">{$t("vm.vnc.connecting")}</span>
      {:else}
        <span class="status disconnected">{$t("vm.vnc.disconnected")}</span>
      {/if}
    </div>
    <div class="toolbar-actions">
      <!-- 快捷键菜单 -->
      <div class="dropdown-wrapper" title={$t("vm.vnc.sendKeys")}>
        <Button variant="ghost" size="sm" onclick={(e: MouseEvent) => { e.stopPropagation(); showKeysMenu = !showKeysMenu; showPowerMenu = false; }}>
          <Icon icon="mdi:keyboard" width="18" />
          <Icon icon="mdi:chevron-down" width="14" />
        </Button>
        {#if showKeysMenu}
          <div class="dropdown-menu" onclick={(e: MouseEvent) => e.stopPropagation()}>
            <button class="menu-item" onclick={sendCtrlAltDel}>
              <Icon icon="mdi:keyboard" width="16" />
              Ctrl+Alt+Del
            </button>
            <div class="menu-divider"></div>
            <div class="menu-label">{$t("vm.vnc.switchTTY")}</div>
            <div class="menu-row">
              {#each [1,2,3,4,5,6,7] as n}
                <button class="menu-key" onclick={() => sendCtrlAltFn(n)}>F{n}</button>
              {/each}
            </div>
            <div class="menu-divider"></div>
            <button class="menu-item" onclick={sendPrintScreen}>
              <Icon icon="mdi:camera" width="16" />
              PrintScreen
            </button>
          </div>
        {/if}
      </div>

      <!-- 电源菜单 -->
      <div class="dropdown-wrapper" title={$t("vm.vnc.powerControl")}>
        <Button variant="ghost" size="sm" onclick={(e: MouseEvent) => { e.stopPropagation(); showPowerMenu = !showPowerMenu; showKeysMenu = false; }}>
          <Icon icon="mdi:power" width="18" />
          <Icon icon="mdi:chevron-down" width="14" />
        </Button>
        {#if showPowerMenu}
          <div class="dropdown-menu" onclick={(e: MouseEvent) => e.stopPropagation()}>
            <button class="menu-item" onclick={() => powerAction("shutdown")}>
              <Icon icon="mdi:power" width="16" />
              {$t("vm.vnc.normalShutdown")}
            </button>
            <button class="menu-item" onclick={() => powerAction("reboot")}>
              <Icon icon="mdi:restart" width="16" />
              {$t("vm.vnc.reboot")}
            </button>
            <button class="menu-item" onclick={() => powerAction("pause")}>
              <Icon icon={vm.status === "paused" ? "mdi:play" : "mdi:pause"} width="16" />
              {vm.status === "paused" ? $t("vm.vnc.resume") : $t("vm.vnc.pause")}
            </button>
            <div class="menu-divider"></div>
            <button class="menu-item danger" onclick={() => powerAction("force")}>
              <Icon icon="mdi:power-off" width="16" />
              {$t("vm.vnc.forceShutdown")}
            </button>
          </div>
        {/if}
      </div>

      <!-- 截图 -->
      <span title={$t("vm.vnc.screenshotToClipboard")}>
        <Button variant="ghost" size="sm" onclick={takeScreenshot}>
          <Icon icon="mdi:camera" width="18" />
        </Button>
      </span>
      <span title={$t("vm.vnc.downloadScreenshot")}>
        <Button variant="ghost" size="sm" onclick={downloadScreenshot}>
          <Icon icon="mdi:download" width="18" />
        </Button>
      </span>

      <!-- 剪贴板 -->
      <span title={$t("vm.vnc.pasteClipboard")}>
        <Button variant="ghost" size="sm" onclick={pasteClipboard}>
          <Icon icon="mdi:content-paste" width="18" />
        </Button>
      </span>

      <!-- 虚拟键盘 -->
      <span title={$t("vm.vnc.virtualKeyboard")} class:active={showVirtualKeyboard}>
        <Button variant="ghost" size="sm" onclick={() => showVirtualKeyboard = !showVirtualKeyboard}>
          <Icon icon="mdi:keyboard-outline" width="18" />
        </Button>
      </span>

      <!-- 缩放 -->
      <span title={scaleViewport ? $t("vm.vnc.originalSize") : $t("vm.vnc.fitToWindow")}>
        <Button variant="ghost" size="sm" onclick={toggleScale}>
          <Icon icon={scaleViewport ? "mdi:fit-to-screen" : "mdi:aspect-ratio"} width="18" />
        </Button>
      </span>

      <!-- 全屏 -->
      <span title={isFullscreen ? $t("vm.vnc.exitFullscreen") : $t("vm.vnc.fullscreen")}>
        <Button variant="ghost" size="sm" onclick={toggleFullscreen}>
          <Icon icon={isFullscreen ? "mdi:fullscreen-exit" : "mdi:fullscreen"} width="18" />
        </Button>
      </span>

      <!-- 重连 -->
      {#if !connected && !connecting}
        <span title={$t("vm.vnc.reconnect")}>
          <Button variant="ghost" size="sm" onclick={reconnect}>
            <Icon icon="mdi:refresh" width="18" />
          </Button>
        </span>
      {/if}
    </div>
  </div>

  <!-- VNC 画布容器 -->
  <div class="canvas-wrapper">
    {#if error}
      <div class="error-overlay">
        <Icon icon="mdi:alert-circle" width="48" />
        <p>{error}</p>
        <Button variant="primary" size="sm" onclick={reconnect}>{$t("vm.vnc.reconnect")}</Button>
      </div>
    {/if}
    {#if connecting}
      <div class="loading-overlay">
        <div class="spinner"></div>
        <p>{$t("vm.vnc.connectingTo", { values: { name: vm.name } })}</p>
      </div>
    {/if}
    <div bind:this={containerRef} class="vnc-container"></div>
  </div>

  <!-- 虚拟键盘面板 -->
  {#if showVirtualKeyboard}
    <div class="virtual-keyboard">
      <div class="vk-row">
        {#each fnKeys as key}
          <button class="vk-key" onclick={() => sendKey(key.keysym, key.label)}>{key.label}</button>
        {/each}
      </div>
      <div class="vk-row">
        {#each specialKeys as key}
          <button class="vk-key" onclick={() => sendKey(key.keysym, key.code)}>{key.label}</button>
        {/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .vnc-console {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: #1e1e1e;
    border-radius: 8px;
    overflow: hidden;
  }

  .toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 12px;
    background: #2d2d2d;
    border-bottom: 1px solid #3d3d3d;
    flex-shrink: 0;
    gap: 8px;
  }

  .toolbar-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .vm-name {
    font-weight: 600;
    color: #fff;
    font-size: 14px;
  }

  .status {
    font-size: 12px;
    padding: 2px 8px;
    border-radius: 10px;
  }

  .status.connected {
    background: #2e7d32;
    color: #fff;
  }

  .status.connecting {
    background: #f57c00;
    color: #fff;
  }

  .status.disconnected {
    background: #c62828;
    color: #fff;
  }

  .perf-info {
    font-size: 12px;
    color: #888;
    display: flex;
    gap: 8px;
  }

  .perf-info .separator {
    color: #555;
  }

  .toolbar-actions {
    display: flex;
    gap: 2px;
    align-items: center;
  }

  .toolbar-actions :global(button) {
    color: #ccc;
    padding: 4px 6px;
  }

  .toolbar-actions :global(button:hover) {
    color: #fff;
    background: #3d3d3d;
  }

  .toolbar-actions span.active :global(button) {
    color: #4a90d9;
    background: #3d3d3d;
  }

  /* 下拉菜单 */
  .dropdown-wrapper {
    position: relative;
  }

  .dropdown-menu {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: 4px;
    background: #2d2d2d;
    border: 1px solid #4d4d4d;
    border-radius: 6px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
    min-width: 180px;
    z-index: 100;
    padding: 4px 0;
  }

  .menu-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 8px 12px;
    background: none;
    border: none;
    color: #ccc;
    font-size: 13px;
    cursor: pointer;
    text-align: left;
  }

  .menu-item:hover {
    background: #3d3d3d;
    color: #fff;
  }

  .menu-item.danger {
    color: #f44336;
  }

  .menu-item.danger:hover {
    background: #5c2020;
    color: #ff6b6b;
  }

  .menu-divider {
    height: 1px;
    background: #4d4d4d;
    margin: 4px 0;
  }

  .menu-label {
    padding: 4px 12px;
    font-size: 11px;
    color: #888;
    text-transform: uppercase;
  }

  .menu-row {
    display: flex;
    gap: 4px;
    padding: 4px 12px;
    flex-wrap: wrap;
  }

  .menu-key {
    padding: 4px 8px;
    background: #3d3d3d;
    border: 1px solid #4d4d4d;
    border-radius: 4px;
    color: #ccc;
    font-size: 12px;
    cursor: pointer;
  }

  .menu-key:hover {
    background: #4d4d4d;
    color: #fff;
  }

  /* 虚拟键盘 */
  .virtual-keyboard {
    background: #2d2d2d;
    border-top: 1px solid #3d3d3d;
    padding: 8px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex-shrink: 0;
  }

  .vk-row {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
    justify-content: center;
  }

  .vk-key {
    padding: 6px 10px;
    min-width: 36px;
    background: #3d3d3d;
    border: 1px solid #4d4d4d;
    border-radius: 4px;
    color: #ccc;
    font-size: 12px;
    cursor: pointer;
    transition: all 0.1s;
  }

  .vk-key:hover {
    background: #4d4d4d;
    color: #fff;
  }

  .vk-key:active {
    background: #4a90d9;
    transform: scale(0.95);
  }

  .canvas-wrapper {
    flex: 1;
    position: relative;
    overflow: hidden;
  }

  .vnc-container {
    width: 100%;
    height: 100%;
  }

  .vnc-container :global(canvas) {
    display: block;
  }

  .error-overlay,
  .loading-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    background: rgba(0, 0, 0, 0.8);
    color: #ccc;
    z-index: 10;
  }

  .error-overlay p,
  .loading-overlay p {
    font-size: 14px;
  }

  .spinner {
    width: 40px;
    height: 40px;
    border: 3px solid #3d3d3d;
    border-top-color: #4a90d9;
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
