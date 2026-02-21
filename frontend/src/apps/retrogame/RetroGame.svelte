<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import Icon from "@iconify/svelte";
  import { Button, Spinner, EmptyState, Modal, Input } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import { retroGameService } from "./service";
  import { PLATFORMS, DEFAULT_ROM_DIRECTORY, getPlatformByExtension } from "./constants";
  import type { RomFile, Platform, GamepadState, RetroGameSettings, SetupProgress } from "./types";
  import GamePlayer from "./GamePlayer.svelte";
  import GameSettings from "./GameSettings.svelte";
  import { fileService } from "$shared/services/files";
  import { windowManager } from "$desktop/stores/windows.svelte";
  import { t } from "svelte-i18n";

  // ==================== 状态 ====================

  // EmulatorJS 安装状态
  let emulatorReady = $state(false);
  let checkingEmulator = $state(true);
  let setupProgress = $state<SetupProgress | null>(null);
  let isSettingUp = $state(false);

  let roms = $state<RomFile[]>([]);
  let loading = $state(true);
  let selectedPlatform = $state<Platform | "all">("all");
  let searchQuery = $state("");
  let settings = $state<RetroGameSettings>(retroGameService.loadSettings());
  let gamepad = $state<GamepadState | null>(null);

  // 当前游戏
  let currentRom = $state<RomFile | null>(null);
  let showPlayer = $state(false);

  // 设置面板
  let showSettings = $state(false);

  // 本地文件选择
  let localFileInput: HTMLInputElement | null = null;

  // ROM 目录选择 - 文件浏览器
  let showDirSelect = $state(false);
  let browserPath = $state("/");
  let browserFiles = $state<any[]>([]);
  let loadingBrowser = $state(false);
  let selectedDir = $state("");
  let homePath = $state("/");

  // ==================== 计算属性 ====================

  let filteredRoms = $derived.by(() => {
    let result = roms;

    if (selectedPlatform !== "all") {
      result = result.filter((r) => r.platform === selectedPlatform);
    }

    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      result = result.filter((r) => r.name.toLowerCase().includes(query));
    }

    return result;
  });

  let platformCounts = $derived.by(() => {
    const counts = new Map<Platform | "all", number>();
    counts.set("all", roms.length);
    for (const rom of roms) {
      counts.set(rom.platform, (counts.get(rom.platform) || 0) + 1);
    }
    return counts;
  });

  // ==================== 生命周期 ====================

  onMount(async () => {
    // 1. 检查 EmulatorJS 是否已安装
    try {
      checkingEmulator = true;
      const status = await retroGameService.checkEmulatorStatus();
      emulatorReady = status.installed;
    } catch (e) {
      console.error("检查 EmulatorJS 状态失败:", e);
      emulatorReady = false;
    } finally {
      checkingEmulator = false;
    }

    // 如果 EmulatorJS 未安装，自动开始安装（无需用户手动点击）
    if (!emulatorReady) {
      startEmulatorSetup();
      return;
    }

    // 2. 获取用户主目录
    try {
      const bookmarks = await fileService.getBookmarks();
      if (bookmarks.home_path) {
        homePath = bookmarks.home_path;
      }
    } catch (e) {
      console.error("获取主目录失败:", e);
    }

    // 如果是默认的 ~/Games/ROMs 目录，尝试自动创建
    if (settings.romDirectory === DEFAULT_ROM_DIRECTORY) {
      await ensureDefaultRomDirectory();
    }

    await scanRoms();

    // 监听手柄
    const unsubscribe = retroGameService.onGamepadChange((state) => {
      gamepad = state;
      if (state) {
        showToast($t("retrogame.gamepadConnected") + ": " + state.id, "success");
      }
    });

    return () => unsubscribe();
  });

  // ==================== EmulatorJS 安装 ====================

  async function startEmulatorSetup() {
    isSettingUp = true;
    setupProgress = { status: "downloading", message: $t("retrogame.preparing"), progress: 0 };

    try {
      await retroGameService.setupEmulator((event) => {
        setupProgress = event;
      });

      if (setupProgress?.status === "completed") {
        emulatorReady = true;
        showToast($t("retrogame.emulatorInstalled"), "success");

        // 安装完成后继续加载 ROM
        try {
          const bookmarks = await fileService.getBookmarks();
          if (bookmarks.home_path) {
            homePath = bookmarks.home_path;
          }
        } catch {}
        if (settings.romDirectory === DEFAULT_ROM_DIRECTORY) {
          await ensureDefaultRomDirectory();
        }
        await scanRoms();
      }
    } catch (e: any) {
      showToast($t("retrogame.installFailed") + ": " + e.message, "error");
      setupProgress = { status: "failed", message: e.message, progress: 0 };
    } finally {
      isSettingUp = false;
    }
  }

  // 确保默认 ROM 目录存在
  async function ensureDefaultRomDirectory() {
    // 将 ~ 替换为实际主目录路径
    const actualPath = settings.romDirectory.replace(/^~/, homePath);
    settings.romDirectory = actualPath;
    retroGameService.saveSettings(settings);

    // 尝试创建 Games 和 ROMs 目录
    try {
      const gamesResult = await fileService.createDir(homePath, "Games");
      if (!gamesResult.success) {
        console.warn("创建 Games 目录:", gamesResult.message);
      }
    } catch (e) {
      console.warn("创建 Games 目录失败:", e);
    }
    
    try {
      const romsResult = await fileService.createDir(`${homePath}/Games`, "ROMs");
      if (!romsResult.success) {
        console.warn("创建 ROMs 目录:", romsResult.message);
      }
    } catch (e) {
      console.warn("创建 ROMs 目录失败:", e);
    }
  }

  // ==================== 方法 ====================

  async function scanRoms() {
    loading = true;
    try {
      roms = await retroGameService.scanRoms(settings.romDirectory);
      
      // 为每个 ROM 生成封面 URL
      for (const rom of roms) {
        const urls = retroGameService.getCoverUrls(rom);
        rom.coverUrl = urls[0] || "";
        rom.coverUrls = urls;
      }
      
      if (roms.length === 0 && settings.romDirectory === DEFAULT_ROM_DIRECTORY) {
        showDirSelect = true;
      }
    } catch (error: any) {
      showToast($t("retrogame.scanRomFailed") + ": " + error.message, "error");
    } finally {
      loading = false;
    }
  }

  function playGame(rom: RomFile) {
    currentRom = rom;
    showPlayer = true;
    retroGameService.recordPlay(rom.path);
  }

  function closePlayer() {
    showPlayer = false;
    currentRom = null;
    // 刷新列表以更新"继续"标记
    roms = [...roms];
  }

  function formatSize(bytes: number): string {
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
    if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + " MB";
    return (bytes / 1024 / 1024 / 1024).toFixed(1) + " GB";
  }

  function getPlatformIcon(platform: Platform): string {
    const info = PLATFORMS.find((p) => p.id === platform);
    return info?.icon || "mdi:gamepad";
  }

  function getPlatformName(platform: Platform): string {
    const info = PLATFORMS.find((p) => p.id === platform);
    return info?.name || platform;
  }

  // ==================== 文件浏览器功能 ====================

  async function openDirBrowser() {
    // 打开文件管理器并跳转到 ROM 目录，方便用户上传 ROM
    const romPath = settings.romDirectory || homePath;
    await windowManager.open("file", { initialPath: romPath });
  }

  async function loadBrowserFiles(path: string) {
    loadingBrowser = true;
    try {
      const response: any = await fileService.list(path, false);
      // API 返回格式: { success: 200, data: { content: [...] } }
      const content = response.data?.content || [];
      // 只显示目录
      browserFiles = content.filter((f: any) => f.is_dir);
      browserPath = path;
    } catch (e: any) {
      console.error("加载文件列表失败:", e);
      // 如果加载失败且不是根目录，尝试回到根目录
      if (path !== "/") {
        await loadBrowserFiles("/");
      } else {
        // 根目录也失败，设置为空
        browserFiles = [];
        browserPath = "/";
      }
    } finally {
      loadingBrowser = false;
    }
  }

  function navigateBrowser(path: string) {
    loadBrowserFiles(path);
  }

  function selectDirectory(file: any) {
    if (file.is_dir) {
      navigateBrowser(file.path);
    }
  }

  function getBrowserBreadcrumbs() {
    const parts = browserPath.split("/").filter(Boolean);
    const crumbs: { label: string; path: string }[] = [{ label: "/", path: "/" }];
    let accPath = "";
    for (const part of parts) {
      accPath += "/" + part;
      crumbs.push({ label: part, path: accPath });
    }
    return crumbs;
  }

  async function confirmDirectory() {
    const dir = selectedDir || browserPath;
    if (!dir) return;
    settings.romDirectory = dir;
    retroGameService.saveSettings(settings);
    showDirSelect = false;
    await scanRoms();
  }

  function handleSettingsSave(newSettings: RetroGameSettings) {
    settings = newSettings;
    retroGameService.saveSettings(settings);
    showSettings = false;
    showToast($t("retrogame.settingsSaved"), "success");
  }

  // ==================== 运行本地游戏 ====================

  function openLocalFile() {
    localFileInput?.click();
  }

  async function handleLocalFileSelected(event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    // 重置 input 值，以便可以重复选择同一文件
    input.value = "";

    const filename = file.name;
    const ext = filename.toLowerCase().slice(filename.lastIndexOf("."));

    // 对 zip 文件，优先通过内部文件扩展名精确识别平台（避免全部当作 arcade）
    let platform;
    if (ext === ".zip") {
      platform = await detectZipPlatformLocal(file);
    } else {
      platform = getPlatformByExtension(filename);
    }

    if (!platform) {
      showToast($t("retrogame.unsupportedFormat") + ": " + ext, "error");
      return;
    }

    const localRom: RomFile = {
      name: filename.replace(/\.[^/.]+$/, ""),
      path: filename, // 用文件名作为路径标识
      size: file.size,
      platform: platform.id,
      localFile: file,
    };

    currentRom = localRom;
    showPlayer = true;
  }

  /**
   * 读取 ZIP 文件内部，通过内部 ROM 文件扩展名判断真实平台
   * 使用浏览器原生 API 解析 ZIP 目录（不依赖第三方库）
   */
  async function detectZipPlatformLocal(file: File): Promise<import("./types").PlatformInfo | undefined> {
    try {
      const buffer = await file.arrayBuffer();
      const view = new DataView(buffer);

      // 从文件末尾查找 End of Central Directory (EOCD) 签名: 0x06054b50
      let eocdOffset = -1;
      for (let i = buffer.byteLength - 22; i >= Math.max(0, buffer.byteLength - 65557); i--) {
        if (view.getUint32(i, true) === 0x06054b50) {
          eocdOffset = i;
          break;
        }
      }
      if (eocdOffset < 0) return undefined;

      const cdOffset = view.getUint32(eocdOffset + 16, true);
      const cdEntries = view.getUint16(eocdOffset + 10, true);

      let offset = cdOffset;
      const decoder = new TextDecoder();

      for (let i = 0; i < cdEntries && offset < buffer.byteLength; i++) {
        if (view.getUint32(offset, true) !== 0x02014b50) break;
        const nameLen = view.getUint16(offset + 28, true);
        const extraLen = view.getUint16(offset + 30, true);
        const commentLen = view.getUint16(offset + 32, true);
        const name = decoder.decode(new Uint8Array(buffer, offset + 46, nameLen));

        // 跳过目录
        if (!name.endsWith("/")) {
          const p = getPlatformByExtension(name);
          if (p && p.id !== "arcade") return p;
        }

        offset += 46 + nameLen + extraLen + commentLen;
      }
    } catch (e) {
      console.warn("解析 ZIP 失败，回退为 arcade:", e);
    }

    // 回退：当成 arcade ROM
    return PLATFORMS.find(p => p.id === "arcade");
  }
</script>

{#if checkingEmulator}
  <!-- 检查 EmulatorJS 状态 -->
  <div class="setup-screen">
    <Spinner size="lg" />
    <p>{$t("retrogame.checkingEmulator")}</p>
  </div>
{:else if !emulatorReady}
  <!-- EmulatorJS 未安装，显示安装界面 -->
  <div class="setup-screen">
    <div class="setup-card">
      <div class="setup-icon">
        <Icon icon="mdi:gamepad-variant" width={64} />
      </div>
      <h2>{$t("retrogame.retroGame")}</h2>
      <p class="setup-desc">{$t("retrogame.firstTimeDownload")}</p>

      {#if setupProgress}
        <div class="setup-progress">
          <div class="progress-bar">
            <div class="progress-fill" class:indeterminate={setupProgress.progress <= 5 && setupProgress.status === 'downloading'} style="width: {Math.max(setupProgress.progress, 5)}%"></div>
          </div>
          <p class="progress-text">{setupProgress.message}</p>
          {#if setupProgress.status === "failed"}
            <p class="progress-error">{$t("retrogame.downloadFailed")}</p>
            <Button variant="primary" onclick={startEmulatorSetup}>
              <Icon icon="mdi:refresh" width={18} />
              {$t("retrogame.retry")}
            </Button>
          {/if}
        </div>
      {:else}
        <div class="setup-progress">
          <div class="progress-bar">
            <div class="progress-fill indeterminate" style="width: 30%"></div>
          </div>
          <p class="progress-text">{$t("retrogame.preparing")}</p>
        </div>
      {/if}
    </div>
  </div>
{:else if showPlayer && currentRom}
  <!-- 游戏播放界面 -->
  <GamePlayer rom={currentRom} {settings} onClose={closePlayer} />
{:else}
  <!-- 主界面 -->
  <div class="retrogame">
    <!-- 头部 -->
    <header class="header">
      <div class="header-left">
        <Icon icon="mdi:gamepad-variant" width={24} />
        <h1>{$t("retrogame.retroGame")}</h1>
        {#if gamepad}
          <span class="gamepad-badge">
            <Icon icon="mdi:controller" width={16} />
            {gamepad.id.split("(")[0].trim()}
          </span>
        {/if}
      </div>
      <div class="header-right">
        <div class="search-box">
          <Icon icon="mdi:magnify" width={18} />
          <input type="text" placeholder={$t("retrogame.searchPlaceholder")} bind:value={searchQuery} />
        </div>
        <Button variant="ghost" onclick={() => scanRoms()}>
          <Icon icon="mdi:refresh" width={18} />
        </Button>
        <Button variant="ghost" onclick={openLocalFile} title={$t("retrogame.openLocalGame")}>
          <Icon icon="mdi:folder-open" width={18} />
        </Button>
        <Button variant="ghost" onclick={() => (showSettings = true)}>
          <Icon icon="mdi:cog" width={18} />
        </Button>
      </div>
    </header>

    <div class="content">
      <!-- 侧边栏 - 平台列表 -->
      <aside class="sidebar">
        <nav class="platform-list">
          <button
            class="platform-item"
            class:active={selectedPlatform === "all"}
            onclick={() => (selectedPlatform = "all")}
          >
            <Icon icon="mdi:gamepad-variant" width={20} />
            <span>{$t("retrogame.all")}</span>
            <span class="count">{platformCounts.get("all") || 0}</span>
          </button>

          {#each PLATFORMS as platform}
            {@const count = platformCounts.get(platform.id) || 0}
            {#if count > 0}
              <button
                class="platform-item"
                class:active={selectedPlatform === platform.id}
                onclick={() => (selectedPlatform = platform.id)}
              >
                <Icon icon={platform.icon} width={20} />
                <span>{platform.name}</span>
                <span class="count">{count}</span>
              </button>
            {/if}
          {/each}
        </nav>

        <div class="sidebar-footer">
          <Button variant="outline" size="sm" onclick={openLocalFile}>
            <Icon icon="mdi:play-circle" width={16} />
            {$t("retrogame.runLocalGame")}
          </Button>
          <Button variant="outline" size="sm" onclick={openDirBrowser}>
            <Icon icon="mdi:folder-open" width={16} />
            {$t("retrogame.openRomDir")}
          </Button>
        </div>
      </aside>

      <!-- 主内容 - ROM 网格 -->
      <main class="main">
        {#if loading}
          <div class="loading">
            <Spinner size="lg" />
            <p>{$t("retrogame.scanning")}</p>
          </div>
        {:else if filteredRoms.length === 0}
          <EmptyState
            icon="mdi:gamepad-variant-outline"
            title={$t("retrogame.noGamesFound")}
            description={searchQuery ? $t("retrogame.tryOtherSearch") : $t("retrogame.putRomsIn") + " " + settings.romDirectory}
          >
            <div style="display: flex; gap: 8px;">
              <Button variant="primary" onclick={openLocalFile}>
                <Icon icon="mdi:play-circle" width={18} />
                {$t("retrogame.runLocalGame")}
              </Button>
              <Button onclick={openDirBrowser}>
                <Icon icon="mdi:folder-open" width={18} />
                {$t("retrogame.openRomDir")}
              </Button>
            </div>
          </EmptyState>
        {:else}
          <div class="rom-grid">
            {#each filteredRoms as rom}
              <button class="rom-card" onclick={() => playGame(rom)}>
                <div class="rom-cover">
                  {#if rom.coverUrl}
                    {@const coverUrls = rom.coverUrls || [rom.coverUrl]}
                    <img
                      src={rom.coverUrl}
                      alt={rom.name}
                      class="cover-img"
                      onerror={(e) => {
                        const target = e.currentTarget as HTMLImageElement;
                        // 尝试下一个候选封面 URL（带不同区域后缀）
                        const idx = ((target as any).__coverIdx as number | undefined) ?? 0;
                        if (idx + 1 < coverUrls.length) {
                          (target as any).__coverIdx = idx + 1;
                          target.src = coverUrls[idx + 1];
                        } else {
                          // 所有候选都失败，显示 fallback 图标
                          target.style.display = 'none';
                          target.nextElementSibling?.classList.remove('hidden');
                        }
                      }}
                    />
                    <div class="cover-fallback hidden">
                      <Icon icon={getPlatformIcon(rom.platform)} width={48} />
                    </div>
                  {:else}
                    <Icon icon={getPlatformIcon(rom.platform)} width={48} />
                  {/if}
                  {#if retroGameService.hasAutoState(rom.path)}
                    <span class="continue-badge">{$t("retrogame.continue")}</span>
                  {/if}
                </div>
                <div class="rom-info">
                  <h3 class="rom-name">{rom.name}</h3>
                  <div class="rom-meta">
                    <span class="platform-tag">{getPlatformName(rom.platform)}</span>
                    <span class="size">{formatSize(rom.size)}</span>
                  </div>
                </div>
              </button>
            {/each}
          </div>
        {/if}
      </main>
    </div>
  </div>
{/if}

<!-- 隐藏的本地文件选择器 -->
<input
  type="file"
  bind:this={localFileInput}
  onchange={handleLocalFileSelected}
  accept=".nes,.smc,.sfc,.gb,.gbc,.gba,.n64,.z64,.v64,.nds,.bin,.cue,.iso,.pbp,.cso,.md,.gen,.zip"
  style="display: none;"
/>

<!-- 设置面板 -->
<Modal bind:open={showSettings} title={$t("retrogame.gameSettings")} size="md">
  <GameSettings {settings} onSave={handleSettingsSave} onCancel={() => (showSettings = false)} />
</Modal>

<!-- ROM 目录选择 - 文件浏览器 -->
<Modal bind:open={showDirSelect} title={$t("retrogame.selectRomDir")} size="md">
  <div class="dir-browser">
    <!-- 面包屑导航 -->
    <div class="browser-nav">
      <button class="nav-btn" onclick={() => navigateBrowser("/")} title={$t("retrogame.rootDir")}>
        <Icon icon="mdi:home" width="18" />
      </button>
      <button
        class="nav-btn"
        onclick={() => {
          const parent = browserPath.substring(0, browserPath.lastIndexOf("/")) || "/";
          navigateBrowser(parent);
        }}
        disabled={browserPath === "/"}
        title={$t("retrogame.goUp")}
      >
        <Icon icon="mdi:arrow-up" width="18" />
      </button>
      <div class="breadcrumbs">
        {#each getBrowserBreadcrumbs() as crumb, i}
          {#if i > 0}<span class="sep">/</span>{/if}
          <button class="crumb" onclick={() => navigateBrowser(crumb.path)}>
            {crumb.label}
          </button>
        {/each}
      </div>
    </div>

    <!-- 文件列表 -->
    <div class="browser-list">
      {#if loadingBrowser}
        <div class="browser-loading">
          <Spinner size="md" />
          <span>{$t("retrogame.loading")}</span>
        </div>
      {:else if browserFiles.length === 0}
        <div class="browser-empty">
          <Icon icon="mdi:folder-open-outline" width="48" />
          <span>{$t("retrogame.noSubfolders")}</span>
        </div>
      {:else}
        {#each browserFiles as file (file.path)}
          <button
            class="browser-item"
            ondblclick={() => selectDirectory(file)}
            onclick={() => (selectedDir = file.path)}
            class:selected={selectedDir === file.path}
          >
            <Icon icon="mdi:folder" width="20" class="folder-icon" />
            <span class="file-name">{file.name}</span>
          </button>
        {/each}
      {/if}
    </div>

    <!-- 底部操作栏 -->
    <div class="browser-footer">
      <div class="selected-path">
        {#if selectedDir || browserPath !== "/"}
          <Icon icon="mdi:check-circle" width="16" class="selected-icon" />
          <span>{$t("retrogame.selected")}: {selectedDir || browserPath}</span>
        {:else}
          <span class="hint">{$t("retrogame.doubleClickHint")}</span>
        {/if}
      </div>
      <div class="browser-actions">
        <Button variant="ghost" onclick={() => (showDirSelect = false)}>{$t("common.cancel")}</Button>
        <Button variant="primary" onclick={() => { selectedDir = browserPath; confirmDirectory(); }}>
          {$t("retrogame.selectCurrentDir")}
        </Button>
        <Button onclick={confirmDirectory} disabled={!selectedDir && browserPath === "/"}>
          {$t("common.ok")}
        </Button>
      </div>
    </div>
  </div>
</Modal>

<style>
  /* ==================== Setup 界面 ==================== */

  .setup-screen {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    background: var(--bg-primary);
    color: var(--text-primary);
    gap: 16px;
  }

  .setup-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    padding: 48px;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    border-radius: 16px;
    max-width: 420px;
    width: 100%;
    text-align: center;
  }

  .setup-icon {
    color: var(--color-primary);
    opacity: 0.8;
  }

  .setup-card h2 {
    font-size: 20px;
    font-weight: 600;
    margin: 0;
  }

  .setup-desc {
    color: var(--text-secondary);
    font-size: 14px;
    margin: 0;
  }

  .setup-progress {
    width: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
  }

  .progress-bar {
    width: 100%;
    height: 8px;
    background: var(--bg-tertiary);
    border-radius: 4px;
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: var(--color-primary);
    border-radius: 4px;
    transition: width 0.3s ease;
  }

  .progress-fill.indeterminate {
    animation: indeterminate 1.5s ease-in-out infinite;
  }

  @keyframes indeterminate {
    0% { transform: translateX(-100%); }
    50% { transform: translateX(100%); }
    100% { transform: translateX(-100%); }
  }

  .progress-text {
    font-size: 13px;
    color: var(--text-secondary);
    margin: 0;
  }

  .progress-error {
    font-size: 13px;
    color: var(--color-error);
    margin: 0;
  }

  /* ==================== 主界面 ==================== */

  .retrogame {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--bg-primary);
    color: var(--text-primary);
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-color);
    background: var(--bg-secondary);
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .header-left h1 {
    font-size: 16px;
    font-weight: 600;
    margin: 0;
  }

  .gamepad-badge {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 2px 8px;
    background: var(--color-success-bg);
    color: var(--color-success);
    border-radius: 12px;
    font-size: 12px;
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .search-box {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 12px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border-color);
    border-radius: 6px;
  }

  .search-box input {
    border: none;
    background: none;
    outline: none;
    color: var(--text-primary);
    font-size: 13px;
    width: 160px;
  }

  .content {
    display: flex;
    flex: 1;
    overflow: hidden;
  }

  .sidebar {
    width: 200px;
    border-right: 1px solid var(--border-color);
    background: var(--bg-secondary);
    display: flex;
    flex-direction: column;
  }

  .platform-list {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
  }

  .platform-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 8px 12px;
    border: none;
    background: none;
    color: var(--text-secondary);
    border-radius: 6px;
    cursor: pointer;
    font-size: 13px;
    text-align: left;
  }

  .platform-item:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .platform-item.active {
    background: var(--color-primary-bg);
    color: var(--color-primary);
  }

  .platform-item .count {
    margin-left: auto;
    font-size: 12px;
    opacity: 0.7;
  }

  .sidebar-footer {
    padding: 12px;
    border-top: 1px solid var(--border-color);
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .main {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
  }

  .loading {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 16px;
    color: var(--text-secondary);
  }

  .rom-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
    gap: 16px;
  }

  .rom-card {
    display: flex;
    flex-direction: column;
    padding: 0;
    border: 1px solid var(--border-color);
    border-radius: 8px;
    background: var(--bg-secondary);
    cursor: pointer;
    overflow: hidden;
    transition: all 0.2s;
  }

  .rom-card:hover {
    border-color: var(--color-primary);
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  }

  .rom-cover {
    aspect-ratio: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-tertiary);
    color: var(--text-tertiary);
    position: relative;
    overflow: hidden;
  }

  .cover-img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .cover-fallback {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 100%;
  }

  .cover-fallback.hidden,
  :global(.hidden) {
    display: none !important;
  }

  .continue-badge {
    position: absolute;
    top: 8px;
    right: 8px;
    padding: 2px 8px;
    background: #6366f1;
    color: #fff;
    border-radius: 4px;
    font-size: 11px;
    font-weight: 600;
  }

  .rom-info {
    padding: 10px;
  }

  .rom-name {
    font-size: 13px;
    font-weight: 500;
    margin: 0 0 6px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .rom-meta {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--text-tertiary);
  }

  .platform-tag {
    padding: 2px 6px;
    background: var(--bg-tertiary);
    border-radius: 4px;
  }

  /* 文件浏览器样式 */
  .dir-browser {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .browser-nav {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px;
    background: var(--bg-tertiary);
    border-radius: 6px;
  }

  .nav-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    border: none;
    background: var(--bg-secondary);
    border-radius: 4px;
    cursor: pointer;
    color: var(--text-secondary);
  }

  .nav-btn:hover:not(:disabled) {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .nav-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .breadcrumbs {
    display: flex;
    align-items: center;
    gap: 4px;
    flex: 1;
    overflow-x: auto;
  }

  .breadcrumbs .sep {
    color: var(--text-tertiary);
  }

  .breadcrumbs .crumb {
    padding: 4px 8px;
    border: none;
    background: none;
    color: var(--text-secondary);
    cursor: pointer;
    border-radius: 4px;
    font-size: 13px;
  }

  .breadcrumbs .crumb:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .browser-list {
    min-height: 300px;
    max-height: 400px;
    overflow-y: auto;
    border: 1px solid var(--border-color);
    border-radius: 6px;
    background: var(--bg-secondary);
  }

  .browser-loading,
  .browser-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 200px;
    gap: 12px;
    color: var(--text-tertiary);
  }

  .browser-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 10px 12px;
    border: none;
    background: none;
    cursor: pointer;
    text-align: left;
    border-bottom: 1px solid var(--border-color);
  }

  .browser-item:last-child {
    border-bottom: none;
  }

  .browser-item:hover {
    background: var(--bg-hover);
  }

  .browser-item.selected {
    background: var(--color-primary-bg);
  }

  .browser-item :global(.folder-icon) {
    color: #f59e0b;
  }

  .browser-item .file-name {
    flex: 1;
    font-size: 13px;
    color: var(--text-primary);
  }

  .browser-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-top: 12px;
    border-top: 1px solid var(--border-color);
  }

  .selected-path {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    color: var(--text-secondary);
    flex: 1;
    overflow: hidden;
  }

  .selected-path :global(.selected-icon) {
    color: var(--color-success);
    flex-shrink: 0;
  }

  .selected-path span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .selected-path .hint {
    color: var(--text-tertiary);
  }

  .browser-actions {
    display: flex;
    gap: 8px;
    flex-shrink: 0;
  }
</style>
