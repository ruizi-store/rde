<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import Icon from "@iconify/svelte";
  import { Button } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import { retroGameService } from "./service";
  import { PLATFORMS, EMULATORJS_CDN } from "./constants";
  import type { RomFile, RetroGameSettings } from "./types";
  import { userStore } from "$shared/stores/user.svelte";
  import { t } from "svelte-i18n";

  // ==================== Props ====================

  interface Props {
    rom: RomFile;
    settings: RetroGameSettings;
    onClose: () => void;
  }

  let { rom, settings, onClose }: Props = $props();

  // ==================== 状态 ====================

  let containerRef = $state<HTMLDivElement | null>(null);
  let loading = $state(true);
  let error = $state<string | null>(null);

  // ROM Blob URL（需要在清理时释放）
  let romBlobUrl: string | null = null;
  // 防止重复清理
  let isCleaningUp = false;

  // 手柄调试面板
  let showGamepadDebug = $state(false);
  let gamepadDebugInfo = $state("");
  let debugPollingId: number | null = null;

  // ==================== 非标准手柄 D-pad 修复 ====================
  // 小米等蓝牙手柄在 Windows/Linux 下 mapping !== "standard"，
  // D-pad 通过单轴 HAT switch (axis 9) 上报，而非 buttons[12-15]。
  // EmulatorJS 只从 buttons 读 D-pad，所以我们在 API 层拦截，
  // 将 axes D-pad 转为标准 buttons，并标记 mapping = "standard"。

  let gamepadPatched = false;
  const AXIS_THRESHOLD = 0.5;

  // 记住每个手柄的 HAT 轴索引（通过检测特征值自动识别）
  const hatAxisMap = new Map<number, number>();

  // 单轴 HAT switch 的 8 方向值（步进 = 2/7）
  // 小米手柄实测: Up=-1.00, Right≈-0.43, Down≈0.14, Left≈0.71, 中位≈3.29
  const HAT_DIRECTIONS: { v: number; x: number; y: number }[] = [
    { v: -1.000, x:  0, y: -1 },  // Up
    { v: -0.714, x:  1, y: -1 },  // Up-Right
    { v: -0.429, x:  1, y:  0 },  // Right
    { v: -0.143, x:  1, y:  1 },  // Down-Right
    { v:  0.143, x:  0, y:  1 },  // Down
    { v:  0.429, x: -1, y:  1 },  // Down-Left
    { v:  0.714, x: -1, y:  0 },  // Left
    { v:  1.000, x: -1, y: -1 },  // Up-Left
  ];

  /** 将单轴 HAT 值解码为方向，中位(>1.5)返回 null */
  function decodeHatValue(v: number): { x: number; y: number } | null {
    // 中位值 (小米手柄 ≈3.29) → 无方向
    if (v > 1.5) return null;
    for (const h of HAT_DIRECTIONS) {
      if (Math.abs(v - h.v) < 0.09) return { x: h.x, y: h.y };
    }
    return null;
  }

  /** 判断轴值是否为 HAT 轴特征：中位值 >1.5 或有效方向值 */
  function isHatAxis(v: number): boolean {
    // 中位值（小米 ≈3.29）— 正常摇杆/扳机不会有 >1.5 的值
    if (v > 1.5) return true;
    // 有效方向值（排除可能与扳机冲突的 ±1.0）
    const distinctiveVals = [-0.714, -0.429, -0.143, 0.143, 0.429, 0.714];
    return distinctiveVals.some(dv => Math.abs(v - dv) < 0.09);
  }

  function patchGamepadAPI() {
    if (gamepadPatched) return;
    gamepadPatched = true;

    // 补丁 navigator.getGamepads — 让 EmulatorJS 的 GamepadHandler 轮询读取到标准化手柄
    // GamepadHandler 不使用浏览器 gamepadconnected 事件，仅通过 navigator.getGamepads() 轮询
    const originalGetGamepads = navigator.getGamepads.bind(navigator);

    (navigator as any).getGamepads = function (): any {
      const raw = originalGetGamepads();
      const patched: (Gamepad | null)[] = [];

      for (let i = 0; i < raw.length; i++) {
        const gp = raw[i];
        if (!gp) { patched.push(null); continue; }
        if (gp.mapping === "standard") { patched.push(gp); continue; }
        patched.push(createDpadProxy(gp));
      }

      (patched as any).item = (idx: number) => patched[idx] ?? null;
      return patched;
    };
  }

  /**
   * 用 Proxy 包装非标准手柄：只拦截 buttons 属性注入 D-pad，
   * 其他全部直接读取原生 Gamepad（保留 identity/mapping/axes 不变）。
   * 这样 EmulatorJS 能正常检测手柄，同时 D-pad 方向键生效。
   */
  function createDpadProxy(gp: Gamepad): any {
    let dpadX = 0, dpadY = 0;

    // ---- 策略1: 单轴 HAT switch（小米/8BitDo 等蓝牙手柄）----
    const knownHat = hatAxisMap.get(gp.index);
    if (knownHat !== undefined && knownHat < gp.axes.length) {
      const d = decodeHatValue(gp.axes[knownHat]);
      if (d) { dpadX = d.x; dpadY = d.y; }
    }

    // 自动识别 HAT 轴
    if (dpadX === 0 && dpadY === 0 && !hatAxisMap.has(gp.index)) {
      for (let a = gp.axes.length - 1; a >= 2; a--) {
        if (isHatAxis(gp.axes[a])) {
          hatAxisMap.set(gp.index, a);
          const d = decodeHatValue(gp.axes[a]);
          if (d) { dpadX = d.x; dpadY = d.y; }
          break;
        }
      }
    }

    // ---- 策略2: 左摇杆回退 (axes 0/1) ----
    if (dpadX === 0 && dpadY === 0 && gp.axes.length >= 2) {
      const lx = gp.axes[0], ly = gp.axes[1];
      if (Math.abs(lx) > AXIS_THRESHOLD) dpadX = lx > 0 ? 1 : -1;
      if (Math.abs(ly) > AXIS_THRESHOLD) dpadY = ly > 0 ? 1 : -1;
    }

    // ---- 构建 D-pad 按钮，注入到 buttons[12-15] ----
    const origBtns = gp.buttons;
    const buttons: any[] = [];
    for (let i = 0; i < origBtns.length; i++) {
      const b = origBtns[i];
      buttons.push({ pressed: b.pressed, touched: b.touched, value: b.value });
    }
    while (buttons.length <= 15) {
      buttons.push({ pressed: false, touched: false, value: 0 });
    }

    const up = dpadY < 0;
    const down = dpadY > 0;
    const left = dpadX < 0;
    const right = dpadX > 0;

    buttons[12] = { pressed: up, touched: up, value: up ? 1 : 0 };
    buttons[13] = { pressed: down, touched: down, value: down ? 1 : 0 };
    buttons[14] = { pressed: left, touched: left, value: left ? 1 : 0 };
    buttons[15] = { pressed: right, touched: right, value: right ? 1 : 0 };

    // Proxy：拦截 buttons/mapping/axes，其余属性直接读原生 Gamepad
    // - mapping="standard" → EmulatorJS 自动识别为默认控制器
    // - buttons → 注入 D-pad 到 [12-15]
    // - axes → 只保留左摇杆，清零扳机静息值（避免角色自动移动）
    const cleanAxes = [
      Math.abs(gp.axes[0] ?? 0) > 0.15 ? gp.axes[0] : 0,
      Math.abs(gp.axes[1] ?? 0) > 0.15 ? gp.axes[1] : 0,
      0,
      0,
    ];

    return new Proxy(gp, {
      get(target: any, prop: string | symbol) {
        if (prop === "buttons") return buttons;
        if (prop === "mapping") return "standard";
        if (prop === "axes") return cleanAxes;
        const val = (target as any)[prop];
        if (typeof val === "function") return val.bind(target);
        return val;
      },
    });
  }

  /**
   * 修复 GamepadHandler 的初始化竞态：
   *
   * GamepadHandler 构造函数中 loop() → updateGamepadState() 同步检测到手柄，
   * 但 'connected' 事件监听器是在构造函数 *之后* 才注册的，
   * 同时 gamepadLabels 也是之后才初始化的。导致首次 'connected' 事件丢失。
   *
   * 修复方法：游戏启动后清空 GamepadHandler 内部缓存的手柄列表，
   * 强制下一次轮询周期重新检测手柄，此时监听器和 gamepadLabels 均已就绪。
   */
  function forceGamepadRedetection() {
    setTimeout(() => {
      const ejs = (window as any).EJS_emulator;
      if (ejs?.gamepad?.gamepads) {
        // 清空已检测的手柄缓存 → 下次 updateGamepadState() 会重新检测，
        // 正常触发 'connected' 事件 → 自动分配到 Player 1
        ejs.gamepad.gamepads = [];
        console.log("[GamepadFix] 已强制 GamepadHandler 重新检测手柄");
      }
    }, 500);
  }

  // ==================== 屏幕内手柄调试 ====================

  function startDebugOverlay() {
    if (debugPollingId !== null) return;

    function poll() {
      // 直接用底层 API（绕过我们的补丁，看原始数据）
      const raw = (navigator as any).__originalGetGamepads
        ? (navigator as any).__originalGetGamepads()
        : navigator.getGamepads();
      const lines: string[] = [];

      let found = false;
      for (let i = 0; i < raw.length; i++) {
        const gp = raw[i];
        if (!gp) continue;
        found = true;

        lines.push(`🎮 #${i}: ${gp.id}`);
        lines.push(`   mapping: "${gp.mapping}" | buttons: ${gp.buttons.length} | axes: ${gp.axes.length}`);

        // 显示所有非零 axes
        const activeAxes: string[] = [];
        for (let a = 0; a < gp.axes.length; a++) {
          if (Math.abs(gp.axes[a]) > 0.05) {
            activeAxes.push(`ax${a}=${gp.axes[a].toFixed(2)}`);
          }
        }
        if (activeAxes.length) lines.push(`   axes: ${activeAxes.join(", ")}`);
        else lines.push("   axes: (all zero)");

        // 显示按下的 buttons
        const pressed: string[] = [];
        for (let b = 0; b < gp.buttons.length; b++) {
          if (gp.buttons[b]?.pressed) pressed.push(`btn${b}`);
        }
        if (pressed.length) lines.push(`   buttons: ${pressed.join(", ")}`);
        else lines.push("   buttons: (none pressed)");

        lines.push("");
      }

      if (!found) {
        lines.push($t("retrogame.noGamepadDetected"));
        lines.push($t("retrogame.pressAnyButton"));
        lines.push($t("retrogame.gamepadApiFocus"));
      }

      gamepadDebugInfo = lines.join("\n");
      debugPollingId = requestAnimationFrame(poll);
    }

    debugPollingId = requestAnimationFrame(poll);
  }

  function stopDebugOverlay() {
    if (debugPollingId !== null) {
      cancelAnimationFrame(debugPollingId);
      debugPollingId = null;
    }
  }

  function toggleDebug() {
    showGamepadDebug = !showGamepadDebug;
    if (showGamepadDebug) {
      startDebugOverlay();
    } else {
      stopDebugOverlay();
    }
  }

  // ==================== 生命周期 ====================

  onMount(async () => {
    // 猴补丁 getGamepads — 必须在 EmulatorJS 脚本加载前执行
    // 保存原始引用供调试面板使用
    if (!(navigator as any).__originalGetGamepads) {
      (navigator as any).__originalGetGamepads = navigator.getGamepads.bind(navigator);
    }
    patchGamepadAPI();

    await initEmulator();
  });

  onDestroy(() => {
    stopDebugOverlay();
    autoSaveAndClose();
  });

  // ==================== 方法 ====================

  /**
   * 确保 EmulatorJS 脚本只加载一次（不使用 loader.js）
   * loader.js 每次都会重新加载 emulator.min.js，导致 class 重复声明错误
   */
  async function ensureEmulatorScripts(): Promise<void> {
    // 已加载（EmulatorJS class 存在）
    if (typeof (window as any).EmulatorJS === "function") return;

    // 加载 emulator.min.js
    await new Promise<void>((resolve, reject) => {
      const script = document.createElement("script");
      script.src = `${EMULATORJS_CDN}emulator.min.js`;
      script.onload = () => resolve();
      script.onerror = () => reject(new Error($t("retrogame.loadEmulatorFailed")));
      document.head.appendChild(script);
    });

    // 加载 emulator.min.css（仅首次）
    if (!document.querySelector(`link[href*="emulator.min.css"]`)) {
      const css = document.createElement("link");
      css.rel = "stylesheet";
      css.href = `${EMULATORJS_CDN}emulator.min.css`;
      document.head.appendChild(css);
    }
  }

  /** 加载语言文件 */
  async function loadLanguageJson(): Promise<Record<string, string> | null> {
    try {
      const resp = await fetch(`${EMULATORJS_CDN}localization/zh.json`);
      if (resp.ok) return JSON.parse(await resp.text());
    } catch {}
    return null;
  }

  async function initEmulator() {
    if (!containerRef) return;

    try {
      loading = true;
      error = null;

      const platform = PLATFORMS.find((p) => p.id === rom.platform);
      if (!platform) {
        throw new Error(`不支持的平台: ${rom.platform}`);
      }

      // 加载 ROM 文件：本地文件直接用 Blob URL，远程文件从服务器下载
      if (rom.localFile) {
        romBlobUrl = URL.createObjectURL(rom.localFile);
      } else {
        const romArrayBuffer = await downloadRomAsArrayBuffer(rom.path);
        const romBlob = new Blob([romArrayBuffer], { type: "application/octet-stream" });
        romBlobUrl = URL.createObjectURL(romBlob);
      }

      const romExt = rom.path.toLowerCase().slice(rom.path.lastIndexOf("."));

      // 确保脚本已加载（仅首次，后续复用）
      await ensureEmulatorScripts();

      // 加载语言
      const langJson = await loadLanguageJson();

      // 清空容器
      const gameContainer = document.getElementById("game-container");
      if (gameContainer) {
        gameContainer.innerHTML = "";
        gameContainer.classList.remove("ejs_parent");
      }

      // 构建配置对象 — 直接传给 EmulatorJS 构造函数，不用 window.EJS_* 全局变量
      const config: Record<string, any> = {
        gameUrl: romBlobUrl,
        dataPath: EMULATORJS_CDN,
        system: platform.core,
        gameName: rom.name + romExt,
        startOnLoad: true,
        fullscreenOnLoad: false,
        disableDatabases: true,
        color: "#6366f1",
        backgroundBlur: true,
        backgroundColor: "#1a1a2e",
        volume: settings.audioVolume / 100,
      };

      if (langJson) {
        config.language = "zh";
        config.langJson = langJson;
      }

      // BIOS 配置
      if (platform.needsBios && platform.biosFile) {
        config.biosUrl = retroGameService.getFileUrl(
          `${settings.romDirectory}/../BIOS/${platform.biosFile}`
        );
      }

      // 直接创建 EmulatorJS 实例（不经过 loader.js）
      const EJS = (window as any).EmulatorJS;
      const ejs = new EJS("#game-container", config);
      (window as any).EJS_emulator = ejs;

      // 绑定事件
      ejs.on("start", () => {
        loading = false;
        loadAutoSaveIfExists();
        // 修复 GamepadHandler 初始化竞态：强制重新检测已连接的手柄
        forceGamepadRedetection();
      });

      // 用户在 EmulatorJS UI 中点击退出 → 返回列表
      ejs.on("exit", () => {
        autoSaveAndClose();
      });
    } catch (e: any) {
      console.error("初始化模拟器错误:", e);
      error = e.message;
      loading = false;
    }
  }

  async function downloadRomAsArrayBuffer(path: string): Promise<ArrayBuffer> {
    const url = retroGameService.getFileUrl(path);
    const token = userStore.token || localStorage.getItem("auth_token");

    const headers: HeadersInit = {};
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    const response = await fetch(url, { headers, credentials: "include" });

    if (!response.ok) {
      throw new Error(`下载 ROM 失败: ${response.status} ${response.statusText}`);
    }

    const arrayBuffer = await response.arrayBuffer();
    if (arrayBuffer.byteLength < 1024) {
      throw new Error(`ROM 文件无效（大小: ${arrayBuffer.byteLength} 字节）`);
    }

    return arrayBuffer;
  }

  /** 尝试自动存档，然后关闭并返回列表 */
  function autoSaveAndClose() {
    if (isCleaningUp) return;
    isCleaningUp = true;

    try {
      const ejs = (window as any).EJS_emulator;
      if (ejs?.gameManager && settings.autoSave) {
        const stateData = ejs.gameManager.getState();
        if (stateData && stateData.byteLength > 0) {
          retroGameService.saveAutoState(rom.path, stateData);
          showToast($t("retrogame.autoSaved"), "success");
        }
      }
    } catch (e) {
      console.warn("自动存档失败:", e);
    }
    cleanupEmulator();
    onClose();
  }

  function cleanupEmulator() {
    try {
      const ejs = (window as any).EJS_emulator;
      if (ejs) {
        // 暂停模拟器（停止音频输出）
        if (typeof ejs.pause === "function" && !ejs.paused) {
          ejs.pause();
        }
        // 停止 WASM 主循环（不调用 callEvent("exit")，因为那会触发 Module.abort 导致后续重启失败）
        try {
          ejs.gameManager?.toggleMainLoop?.(0);
        } catch {}
        // 关闭 AudioContext
        try {
          const audioCtx = ejs?.Module?.AL?.currentCtx?.audioCtx;
          if (audioCtx && audioCtx.state !== "closed") {
            audioCtx.close();
          }
        } catch {}
        // 标记已停止，防止 beforeunload 事件再触发 exit
        ejs.started = false;
      }
    } catch (e) {
      console.warn("EmulatorJS cleanup error:", e);
    }

    // 清理实例引用（但不删除脚本定义的全局 class，如 EJS_GameManager、EJS_DUMMYSTORAGE）
    delete (window as any).EJS_emulator;

    // 释放 ROM Blob URL
    if (romBlobUrl) {
      URL.revokeObjectURL(romBlobUrl);
      romBlobUrl = null;
    }

    // 清空容器 DOM + 移除 ejs_parent class（下次 EmulatorJS 会重新添加）
    const gameContainer = document.getElementById("game-container");
    if (gameContainer) {
      gameContainer.innerHTML = "";
      gameContainer.classList.remove("ejs_parent");
    }
  }

  function handleBack() {
    autoSaveAndClose();
  }

  /** 如果有自动存档，延迟加载恢复 */
  function loadAutoSaveIfExists() {
    const stateData = retroGameService.loadAutoState(rom.path);
    if (!stateData) return;

    setTimeout(() => {
      try {
        const ejs = (window as any).EJS_emulator;
        if (ejs?.gameManager) {
          ejs.gameManager.loadState(stateData);
          showToast($t("retrogame.progressRestored"), "success");
        }
      } catch (e) {
        console.warn("恢复存档失败:", e);
      }
    }, 500);
  }
</script>

<div class="game-player" bind:this={containerRef}>
  <!-- 最小化顶部栏：只有返回和游戏名 -->
  <header class="toolbar">
    <Button variant="ghost" size="sm" onclick={handleBack}>
      <Icon icon="mdi:arrow-left" width={18} />
      {$t("retrogame.back")}
    </Button>
    <span class="game-title">{rom.name}</span>
    <Button variant="ghost" size="sm" onclick={toggleDebug} title={$t("retrogame.gamepadDebug")}>
      <Icon icon="mdi:controller" width={18} />
    </Button>
  </header>

  <!-- 游戏画面容器 -->
  <div class="game-area">
    {#if loading}
      <div class="loading-overlay">
        <div class="loader"></div>
        <p>{$t("retrogame.loadingGame")}</p>
      </div>
    {/if}

    {#if error}
      <div class="error-overlay">
        <Icon icon="mdi:alert-circle" width={48} />
        <p>{error}</p>
        <Button onclick={onClose}>{$t("retrogame.back")}</Button>
      </div>
    {/if}

    <!-- EmulatorJS 挂载点 — 所有控制由 EmulatorJS 内置 UI 提供 -->
    <div id="game-container"></div>

    <!-- 手柄调试面板（显示原始手柄数据，不依赖 DevTools） -->
    {#if showGamepadDebug}
      <div class="gamepad-debug">
        <div class="debug-header">
          <span>🎮 {$t("retrogame.gamepadDebugTitle")}</span>
          <button class="debug-close" onclick={toggleDebug}>✕</button>
        </div>
        <pre class="debug-content">{gamepadDebugInfo}</pre>
        <div class="debug-hint">{$t("retrogame.dpadHint")}</div>
      </div>
    {/if}
  </div>
</div>

<style>
  .game-player {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: #0a0a0f;
    color: #fff;
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 4px 12px;
    background: rgba(255, 255, 255, 0.05);
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    flex-shrink: 0;
  }

  .game-title {
    font-size: 14px;
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .game-area {
    flex: 1;
    position: relative;
    overflow: hidden;
  }

  #game-container {
    position: absolute;
    inset: 0;
  }

  .loading-overlay,
  .error-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    background: rgba(0, 0, 0, 0.8);
    z-index: 10;
  }

  .loader {
    width: 48px;
    height: 48px;
    border: 4px solid rgba(255, 255, 255, 0.2);
    border-top-color: #6366f1;
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .error-overlay {
    color: #ef4444;
  }

  /* 手柄调试面板 */
  .gamepad-debug {
    position: absolute;
    top: 8px;
    right: 8px;
    width: 360px;
    max-height: 280px;
    background: rgba(0, 0, 0, 0.85);
    border: 1px solid rgba(99, 102, 241, 0.5);
    border-radius: 8px;
    z-index: 100;
    font-family: monospace;
    font-size: 12px;
    overflow: hidden;
    pointer-events: auto;
  }

  .debug-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 10px;
    background: rgba(99, 102, 241, 0.2);
    font-weight: 600;
    color: #a5b4fc;
  }

  .debug-close {
    background: none;
    border: none;
    color: #a5b4fc;
    cursor: pointer;
    font-size: 14px;
    padding: 0 4px;
  }

  .debug-content {
    padding: 8px 10px;
    margin: 0;
    white-space: pre-wrap;
    word-break: break-all;
    overflow-y: auto;
    max-height: 200px;
    color: #e0e7ff;
    line-height: 1.5;
  }

  .debug-hint {
    padding: 4px 10px 6px;
    color: #6b7280;
    font-size: 11px;
    border-top: 1px solid rgba(255, 255, 255, 0.1);
  }
</style>
