<script lang="ts">
  import { t } from "svelte-i18n";
  import { windowManager, type WindowState } from "$desktop/stores/windows.svelte";
  import { draggable } from "$desktop/utils/drag";
  import { resizable } from "$desktop/utils/resize";
  import Icon from "@iconify/svelte";
  import type { Component } from "svelte";
  import { onMount, onDestroy } from "svelte";
  import { iframeBridge } from "$shared/services/iframe-bridge";
  import { toast } from "$shared/stores/toast.svelte";

  let {
    window,
    snapPreview = $bindable(null),
  }: {
    window: WindowState;
    snapPreview?:
      | "left"
      | "right"
      | "top-left"
      | "top-right"
      | "bottom-left"
      | "bottom-right"
      | "maximize"
      | null;
  } = $props();

  // xpra 控件显示状态
  let xpraToolbarVisible = $state(false);
  let iframeRef: HTMLIFrameElement | undefined = $state();

  // 判断是否是 xpra 会话窗口
  const isXpraWindow = $derived(
    window.type === "iframe" && window.appId?.startsWith("xpra-"),
  );

  // 获取组件 (仅组件类型窗口)
  const AppComponent = $derived(
    window.type === "component" ? (window.component as Component<any>) : null,
  );

  // 处理来自 iframe 的消息
  function handleIframeMessage(event: MessageEvent) {
    // 验证消息来源
    if (!window.iframeConfig || window.type !== "iframe") return;

    // 处理相对路径 URL，使用当前页面 origin 作为基准
    const baseUrl = globalThis.window?.location.origin || "";
    let expectedOrigin: string;
    try {
      expectedOrigin = new URL(window.iframeConfig.url, baseUrl).origin;
    } catch {
      // URL 解析失败时，使用当前页面 origin
      expectedOrigin = baseUrl;
    }

    if (event.origin !== expectedOrigin && event.origin !== baseUrl) {
      return;
    }

    const data = event.data;
    if (!data || typeof data !== "object" || !data.type) return;

    switch (data.type) {
      case "API_REQUEST":
        handleApiRequest(data);
        break;
      case "WINDOW_CONTROL":
        handleWindowControl(data);
        break;
      case "READY":
        // iframe 已就绪，发送初始化数据
        sendToIframe({
          type: "INIT",
          windowId: window.id,
          permissions: window.iframeConfig.permissions,
        });
        break;
    }
  }

  // 处理 API 请求
  async function handleApiRequest(data: {
    requestId: string;
    endpoint: string;
    method: string;
    body?: unknown;
  }) {
    if (!window.iframeConfig) return;

    const permissions = window.iframeConfig.permissions || [];

    // 权限检查 (支持 /api/v1/ 和 /api/ 两种路径)
    const endpointPermissions: Record<string, string> = {
      "/api/v1/files": "files:read",
      "/api/v1/system": "system:read",
      "/api/v1/network": "network:read",
      "/api/files": "files:read",
      "/api/system": "system:read",
      "/api/network": "network:read",
    };

    const requiredPermission = Object.entries(endpointPermissions).find(([path]) =>
      data.endpoint.startsWith(path),
    )?.[1];

    if (requiredPermission && !permissions.includes(requiredPermission)) {
      sendToIframe({
        type: "API_RESPONSE",
        requestId: data.requestId,
        success: false,
        error: `Permission denied: ${requiredPermission}`,
      });
      return;
    }

    try {
      // 代理请求到后端
      const response = await fetch(data.endpoint, {
        method: data.method,
        headers: {
          "Content-Type": "application/json",
        },
        body: data.body ? JSON.stringify(data.body) : undefined,
        credentials: "include",
      });

      const result = await response.json();

      sendToIframe({
        type: "API_RESPONSE",
        requestId: data.requestId,
        success: response.ok,
        data: result,
      });
    } catch (error) {
      sendToIframe({
        type: "API_RESPONSE",
        requestId: data.requestId,
        success: false,
        error: error instanceof Error ? error.message : "Unknown error",
      });
    }
  }

  // 处理窗口控制命令
  function handleWindowControl(data: { action: string; payload?: unknown }) {
    switch (data.action) {
      case "close":
        windowManager.close(window.id);
        break;
      case "minimize":
        windowManager.minimize(window.id);
        break;
      case "maximize":
        windowManager.toggleMaximize(window.id);
        break;
      case "setTitle":
        if (typeof data.payload === "string") {
          windowManager.setTitle(window.id, data.payload);
        }
        break;
      case "resize":
        if (data.payload && typeof data.payload === "object") {
          const { width, height } = data.payload as { width?: number; height?: number };
          if (width && height) {
            // 取消最大化状态以便调整大小
            if (window.isMaximized) {
              windowManager.toggleMaximize(window.id);
            }
            windowManager.resize(window.id, width, height);
          }
        }
        break;
      case "showNotification":
        if (data.payload && typeof data.payload === "object") {
          const {
            title,
            message,
            type = "info",
          } = data.payload as {
            title?: string;
            message?: string;
            type?: "info" | "success" | "warning" | "error";
          };
          if (title) {
            toast.add({ type, title, message });
          }
        }
        break;
    }
  }

  // 发送消息到 iframe
  function sendToIframe(message: unknown) {
    if (iframeRef?.contentWindow) {
      iframeRef.contentWindow.postMessage(message, "*");
    }
  }

  // 当窗口大小变化时通知 iframe
  $effect(() => {
    if (window.type === "iframe" && iframeRef) {
      sendToIframe({
        type: "WINDOW_RESIZE",
        width: window.width,
        height: window.height,
        isMaximized: window.isMaximized,
      });
    }
  });

  // 设置消息监听
  onMount(() => {
    if (window.type === "iframe") {
      globalThis.window?.addEventListener("message", handleIframeMessage);
    }
  });

  onDestroy(() => {
    if (window.type === "iframe") {
      globalThis.window?.removeEventListener("message", handleIframeMessage);
      // 从 iframe bridge 注销
      if (window.iframeConfig?.packageId) {
        iframeBridge.unregister(window.iframeConfig.packageId);
      }
    }
  });

  // 计算样式
  let style = $derived(
    window.isMaximized
      ? "top: 0; left: 0; width: 100%; height: calc(100% - 48px);"
      : `top: ${window.y}px; left: ${window.x}px; width: ${window.width}px; height: ${window.height}px;`,
  );

  function handleClose() {
    windowManager.close(window.id);
  }

  function toggleXpraToolbar() {
    xpraToolbarVisible = !xpraToolbarVisible;
    try {
      const doc = iframeRef?.contentDocument;
      const win = iframeRef?.contentWindow as any;
      if (!doc) return;
      const fm = doc.getElementById("float_menu");
      if (!fm) return;
      if (xpraToolbarVisible) {
        fm.style.setProperty("opacity", "1", "important");
        fm.style.setProperty("pointer-events", "auto", "important");
        fm.style.setProperty("display", "inline-block", "important");
        fm.style.setProperty("z-index", "99999", "important");
        if (typeof win?.expand_float_menu === "function") {
          win.expand_float_menu();
        }
      } else {
        fm.style.setProperty("opacity", "0", "important");
        fm.style.setProperty("pointer-events", "none", "important");
      }
    } catch {
      // ignore cross-origin errors
    }
  }

  function handleMinimize() {
    windowManager.minimize(window.id);
  }

  function handleMaximize() {
    windowManager.toggleMaximize(window.id);
  }

  function handleFocus() {
    if (!window.isFocused) {
      windowManager.focus(window.id);
    }
  }

  function handleDragMove(e: { x: number; y: number; mouseX: number; mouseY: number }) {
    windowManager.move(window.id, e.x, e.y);

    // 基于鼠标位置检测吸附区域
    const screenWidth = globalThis.window?.innerWidth || 1920;
    const screenHeight = globalThis.window?.innerHeight || 1080;
    const taskbarHeight = 48;
    const edgeThreshold = 10; // 边缘检测阈值
    const cornerSize = 50; // 角落检测区域

    const mx = e.mouseX;
    const my = e.mouseY;

    // 顶部边缘
    if (my <= edgeThreshold) {
      if (mx <= cornerSize) {
        snapPreview = "top-left";
      } else if (mx >= screenWidth - cornerSize) {
        snapPreview = "top-right";
      } else {
        snapPreview = "maximize";
      }
    }
    // 左边缘
    else if (mx <= edgeThreshold) {
      if (my >= screenHeight - cornerSize - taskbarHeight) {
        snapPreview = "bottom-left";
      } else {
        snapPreview = "left";
      }
    }
    // 右边缘
    else if (mx >= screenWidth - edgeThreshold) {
      if (my >= screenHeight - cornerSize - taskbarHeight) {
        snapPreview = "bottom-right";
      } else {
        snapPreview = "right";
      }
    }
    // 不在边缘
    else {
      snapPreview = null;
    }
  }

  function handleDragEnd(e: { x: number; y: number; mouseX: number; mouseY: number }) {
    // 执行吸附
    if (snapPreview) {
      switch (snapPreview) {
        case "left":
          windowManager.snapLeft(window.id);
          break;
        case "right":
          windowManager.snapRight(window.id);
          break;
        case "top-left":
          windowManager.snapTopLeft(window.id);
          break;
        case "top-right":
          windowManager.snapTopRight(window.id);
          break;
        case "bottom-left":
          windowManager.snapBottomLeft(window.id);
          break;
        case "bottom-right":
          windowManager.snapBottomRight(window.id);
          break;
        case "maximize":
          windowManager.toggleMaximize(window.id);
          break;
      }
      snapPreview = null;
    }
  }

  function handleResize(e: { width: number; height: number; x?: number; y?: number }) {
    windowManager.resize(window.id, e.width, e.height, e.x, e.y);
  }

  // 键盘快捷键
  function handleKeydown(e: KeyboardEvent) {
    if (!window.isFocused) return;

    // Win + 方向键 吸附
    if (e.metaKey || e.ctrlKey) {
      switch (e.key) {
        case "ArrowLeft":
          e.preventDefault();
          windowManager.snapLeft(window.id);
          break;
        case "ArrowRight":
          e.preventDefault();
          windowManager.snapRight(window.id);
          break;
        case "ArrowUp":
          e.preventDefault();
          windowManager.toggleMaximize(window.id);
          break;
        case "ArrowDown":
          e.preventDefault();
          if (window.isMaximized) {
            windowManager.toggleMaximize(window.id);
          } else {
            windowManager.minimize(window.id);
          }
          break;
      }
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- 吸附预览 -->
{#if snapPreview}
  <div class="snap-preview {snapPreview}"></div>
{/if}

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="window"
  class:focused={window.isFocused}
  class:maximized={window.isMaximized}
  class:minimized={window.isMinimized}
  style="{style} z-index: {window.zIndex};"
  onmousedown={handleFocus}
  oncontextmenu={(e) => {
    e.preventDefault();
    e.stopPropagation();
  }}
>
  <!-- 标题栏 -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <header
    class="window-header"
    use:draggable={{
      disabled: window.isMaximized,
      onMove: handleDragMove,
      onEnd: handleDragEnd,
    }}
    ondblclick={handleMaximize}
  >
    <div class="window-title">
      <img
        src={window.icon}
        alt=""
        class="window-icon"
        onerror={(e) => ((e.currentTarget as HTMLImageElement).style.display = "none")}
      />
      <span>{window.title}</span>
    </div>

    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="window-controls" onmousedown={(e) => e.stopPropagation()}>
      {#if isXpraWindow}
        <button
          class="control xpra-toolbar"
          class:active={xpraToolbarVisible}
          onclick={toggleXpraToolbar}
          title={xpraToolbarVisible ? $t("window.hideXpra") : $t("window.showXpra")}
        >
          <Icon icon="mdi:toolbox-outline" />
        </button>
      {/if}
      <button class="control minimize" onclick={handleMinimize} title={$t("window.minimize")}>
        <Icon icon="mdi:minus" />
      </button>
      <button
        class="control maximize"
        onclick={handleMaximize}
        title={window.isMaximized ? $t("window.restore") : $t("window.maximize")}
      >
        <Icon icon={window.isMaximized ? "mdi:window-restore" : "mdi:window-maximize"} />
      </button>
      <button class="control close" onclick={handleClose} title={$t("common.close")}>
        <Icon icon="mdi:close" />
      </button>
    </div>
  </header>

  <!-- 内容区 -->
  <main class="window-content">
    {#if window.type === "iframe" && window.iframeConfig}
      {#if isXpraWindow}
        <!-- xpra 窗口不使用 sandbox，保证父页面可操作 iframe 文档 -->
        <iframe
          bind:this={iframeRef}
          src={window.iframeConfig.url}
          allowfullscreen
          title={window.title}
          allow="fullscreen; autoplay; microphone"
          class="window-iframe"
          onload={() => {
            if (iframeRef && window.iframeConfig?.packageId) {
              iframeBridge.register(window.iframeConfig.packageId, iframeRef, window.id);
            }
          }}
        ></iframe>
      {:else}
        <iframe
          bind:this={iframeRef}
          src={window.iframeConfig.url}
          allowfullscreen
          title={window.title}
          sandbox={window.iframeConfig.sandbox}
          allow={window.iframeConfig.allow}
          class="window-iframe"
          onload={() => {
            if (iframeRef && window.iframeConfig?.packageId) {
              iframeBridge.register(window.iframeConfig.packageId, iframeRef, window.id);
            }
          }}
        ></iframe>
      {/if}
    {:else if AppComponent}
      <AppComponent {...window.props} windowId={window.id} />
    {:else}
      <div class="window-error">
        <Icon icon="mdi:alert-circle-outline" width="48" />
        <p>{$t("window.loadContentFailed")}</p>
      </div>
    {/if}
  </main>

  <!-- 调整大小手柄 -->
  {#if !window.isMaximized}
    <div
      class="resize-handle n"
      use:resizable={{
        direction: "n",
        minWidth: window.minWidth,
        minHeight: window.minHeight,
        onResize: handleResize,
      }}
    ></div>
    <div
      class="resize-handle s"
      use:resizable={{
        direction: "s",
        minWidth: window.minWidth,
        minHeight: window.minHeight,
        onResize: handleResize,
      }}
    ></div>
    <div
      class="resize-handle e"
      use:resizable={{
        direction: "e",
        minWidth: window.minWidth,
        minHeight: window.minHeight,
        onResize: handleResize,
      }}
    ></div>
    <div
      class="resize-handle w"
      use:resizable={{
        direction: "w",
        minWidth: window.minWidth,
        minHeight: window.minHeight,
        onResize: handleResize,
      }}
    ></div>
    <div
      class="resize-handle ne"
      use:resizable={{
        direction: "ne",
        minWidth: window.minWidth,
        minHeight: window.minHeight,
        onResize: handleResize,
      }}
    ></div>
    <div
      class="resize-handle nw"
      use:resizable={{
        direction: "nw",
        minWidth: window.minWidth,
        minHeight: window.minHeight,
        onResize: handleResize,
      }}
    ></div>
    <div
      class="resize-handle se"
      use:resizable={{
        direction: "se",
        minWidth: window.minWidth,
        minHeight: window.minHeight,
        onResize: handleResize,
      }}
    ></div>
    <div
      class="resize-handle sw"
      use:resizable={{
        direction: "sw",
        minWidth: window.minWidth,
        minHeight: window.minHeight,
        onResize: handleResize,
      }}
    ></div>
  {/if}
</div>

<style>
  .window {
    position: absolute;
    display: flex;
    flex-direction: column;
    background: var(--bg-window, #ffffff);
    border-radius: 8px;
    box-shadow: var(--shadow-window, 0 8px 32px rgba(0, 0, 0, 0.15));
    overflow: hidden;
    transition: box-shadow 0.2s ease;

    /* 打开窗口动画 */
    animation: windowOpen 0.2s cubic-bezier(0.22, 1, 0.36, 1);

    @keyframes windowOpen {
      from {
        opacity: 0;
        transform: scale(0.95);
      }
      to {
        opacity: 1;
        transform: scale(1);
      }
    }

    &.focused {
      box-shadow: 0 12px 48px rgba(0, 0, 0, 0.25);
    }

    &.minimized {
      display: none;
    }

    &.maximized {
      border-radius: 0;
      /* 最大化动画 */
      animation: none;
      transition: all 0.2s cubic-bezier(0.22, 1, 0.36, 1);
    }

    &:global(.resizing) {
      user-select: none;
    }

    /* 吸附时的过渡动画 */
    &:global(.snapping) {
      transition: all 0.15s cubic-bezier(0.22, 1, 0.36, 1);
    }
  }

  .window-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 36px;
    padding: 0 8px;
    background: var(--bg-window-header, #f8f9fa);
    border-bottom: 1px solid var(--border-color, #d0d0d0);
    color: var(--text-primary, #333);
    cursor: default;
    user-select: none;
    flex-shrink: 0;

    .focused & {
      background: linear-gradient(
        180deg,
        var(--color-primary, #4a90d9) 0%,
        var(--color-primary-hover, #357abd) 100%
      );
      border-bottom-color: var(--color-primary-hover, #2a5f9e);
      color: white;
    }
  }

  .window-title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    font-weight: 500;
    overflow: hidden;

    span {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .window-icon {
    width: 16px;
    height: 16px;
    flex-shrink: 0;
  }

  .window-controls {
    display: flex;
    gap: 4px;
    flex-shrink: 0;

    .control {
      width: 28px;
      height: 28px;
      border: none;
      border-radius: 4px;
      background: transparent;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      color: inherit;
      font-size: 16px;

      &:hover {
        background: rgba(0, 0, 0, 0.1);

        .focused & {
          background: rgba(255, 255, 255, 0.2);
        }
      }

      &.close:hover {
        background: #e53935;
        color: white;
      }

      &.xpra-toolbar {
        opacity: 0.7;
        &:hover {
          opacity: 1;
          background: rgba(255, 255, 255, 0.2);
        }
        &.active {
          opacity: 1;
          background: rgba(255, 255, 255, 0.25);
        }
      }
    }
  }

  .window-content {
    flex: 1;
    overflow: auto;
    background: var(--bg-window, #ffffff);
    color: var(--text-primary, #333);
  }

  /* iframe 窗口样式 */
  .window-iframe {
    width: 100%;
    height: 100%;
    border: none;
    background: white;
  }

  /* 窗口错误状态 */
  .window-error {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--text-secondary, #666);
    gap: 12px;

    p {
      margin: 0;
      font-size: 14px;
    }
  }

  /* 调整大小手柄 */
  .resize-handle {
    position: absolute;

    &.n,
    &.s {
      left: 8px;
      right: 8px;
      height: 4px;
      cursor: ns-resize;
    }

    &.e,
    &.w {
      top: 8px;
      bottom: 8px;
      width: 4px;
      cursor: ew-resize;
    }

    &.n {
      top: 0;
    }
    &.s {
      bottom: 0;
    }
    &.e {
      right: 0;
    }
    &.w {
      left: 0;
    }

    &.ne,
    &.nw,
    &.se,
    &.sw {
      width: 12px;
      height: 12px;
    }

    &.ne {
      top: 0;
      right: 0;
      cursor: nesw-resize;
    }
    &.nw {
      top: 0;
      left: 0;
      cursor: nwse-resize;
    }
    &.se {
      bottom: 0;
      right: 0;
      cursor: nwse-resize;
    }
    &.sw {
      bottom: 0;
      left: 0;
      cursor: nesw-resize;
    }
  }

  /* 吸附预览 */
  .snap-preview {
    position: fixed;
    background: rgba(74, 144, 217, 0.3);
    border: 2px solid rgba(74, 144, 217, 0.8);
    border-radius: 8px;
    z-index: 99998;
    pointer-events: none;
    animation: snapFadeIn 0.15s ease-out;

    &.left {
      top: 0;
      left: 0;
      width: 50%;
      height: calc(100% - 48px);
    }

    &.right {
      top: 0;
      right: 0;
      width: 50%;
      height: calc(100% - 48px);
    }

    &.top-left {
      top: 0;
      left: 0;
      width: 50%;
      height: calc(50% - 24px);
    }

    &.top-right {
      top: 0;
      right: 0;
      width: 50%;
      height: calc(50% - 24px);
    }

    &.bottom-left {
      bottom: 48px;
      left: 0;
      width: 50%;
      height: calc(50% - 24px);
    }

    &.bottom-right {
      bottom: 48px;
      right: 0;
      width: 50%;
      height: calc(50% - 24px);
    }
  }

  @keyframes snapFadeIn {
    from {
      opacity: 0;
      transform: scale(0.95);
    }
    to {
      opacity: 1;
      transform: scale(1);
    }
  }
</style>
