<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { t, initI18n } from "./i18n";
  import Icon from "@iconify/svelte";
  import { Button, Spinner } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import { androidService, type EnvStatus } from "./service";
  import { api } from "$shared/services/api";
  import { fileService, type FileInfo } from "$shared/services/files";
  import { ScrcpyClient, type ScrcpySession, type AudioStatus } from "./lib";

  type APIResp = { success: boolean; data?: any; error?: string };

  // ==================== 常量 ====================
  const EMULATOR_ADDRESS = "localhost:5555";
  const ANDROID_WS_BASE = "/api/v1/android";

  // ==================== 环境状态 ====================
  let envReady = $state<boolean | null>(null);
  let envStatus = $state<EnvStatus | null>(null);
  let envPollTimer: ReturnType<typeof setInterval> | null = null;

  // ==================== 安装向导 ====================
  let installRunning = $state(false);
  let installSteps = $state<any[]>([]);
  let installError = $state<string | null>(null);
  let installWs: WebSocket | null = null;

  // ==================== 模拟器状态 ====================
  type EmulatorStatus = "stopped" | "starting" | "running" | "stopping" | "error";
  let emulatorStatus = $state<EmulatorStatus>("stopped");
  let emulatorInfo = $state<{ name: string; androidVersion: string } | null>(null);

  // ==================== 投屏状态 ====================
  let scrcpyClient: ScrcpyClient | null = null;
  let mirrorConnected = $state(false);
  let mirrorSession = $state<ScrcpySession | null>(null);
  let mirrorAudioStatus = $state<AudioStatus>("idle");
  let videoElement: HTMLVideoElement;
  let videoContainer: HTMLDivElement;

  // ==================== 投屏配置 ====================
  let mirrorConfig = $state({
    maxSize: 1280,
    bitrate: 8000000,
    maxFps: 60,
    audioEnabled: true,
    showTouches: false,
  });
  let showSettings = $state(false);

  // ==================== APK 安装 ====================
  let showApkDialog = $state(false);
  let apkInstalling = $state(false);
  let apkInstallProgress = $state("");
  let apkDragOver = $state(false);
  let apkFileInput: HTMLInputElement;
  
  // 服务器文件浏览
  let showServerBrowser = $state(false);
  let serverPath = $state("/home");
  let serverFiles = $state<FileInfo[]>([]);
  let serverLoading = $state(false);
  
  // 文件浏览器增强功能
  type ViewMode = "list" | "grid";
  type SortBy = "name" | "size" | "modified";
  type SortOrder = "asc" | "desc";
  
  let viewMode = $state<ViewMode>("list");
  let searchQuery = $state("");
  let sortBy = $state<SortBy>("name");
  let sortOrder = $state<SortOrder>("asc");
  let selectedFiles = $state<Set<string>>(new Set());
  let recentPaths = $state<string[]>([]); // 最近安装的APK路径
  
  // 筛选和排序后的文件列表
  let filteredFiles = $derived(() => {
    let files = [...serverFiles];
    
    // 搜索过滤
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      files = files.filter(f => f.name.toLowerCase().includes(query));
    }
    
    // 排序
    files.sort((a, b) => {
      // 目录始终在前
      if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
      
      let cmp = 0;
      switch (sortBy) {
        case "name":
          cmp = a.name.toLowerCase().localeCompare(b.name.toLowerCase());
          break;
        case "size":
          cmp = a.size - b.size;
          break;
        case "modified":
          cmp = new Date(a.modified || 0).getTime() - new Date(b.modified || 0).getTime();
          break;
      }
      return sortOrder === "asc" ? cmp : -cmp;
    });
    
    return files;
  });

  // ==================== 缩放 ====================
  let zoomLevel = $state(1);
  const ZOOM_STEP = 0.1;
  const ZOOM_MIN = 0.5;
  const ZOOM_MAX = 2;

  // ==================== 屏幕方向 ====================
  let screenAspectRatio = $derived(
    mirrorSession && mirrorSession.width > 0 && mirrorSession.height > 0
      ? `${mirrorSession.width} / ${mirrorSession.height}`
      : "9 / 16"
  );

  // ==================== 生命周期 ====================
  let envCheckReady: Promise<void>;
  let resolveEnvCheck: () => void;

  function onApkInstallEvent(e: Event) {
    handleExternalApkInstall(e as CustomEvent<{ path: string; name: string }>);
  }

  onMount(async () => {
    envCheckReady = new Promise(resolve => { resolveEnvCheck = resolve; });
    window.addEventListener("rde:install-apk", onApkInstallEvent);
    await initI18n();
    await checkEnv();
    resolveEnvCheck();
  });

  onDestroy(() => {
    window.removeEventListener("rde:install-apk", onApkInstallEvent);
    if (envPollTimer) clearInterval(envPollTimer);
    if (mirrorDelayTimer) clearInterval(mirrorDelayTimer);
    stopInstallWs();
    if (scrcpyClient) {
      scrcpyClient.destroy();
      scrcpyClient = null;
    }
  });

  // ==================== 外部 APK 安装请求 ====================
  async function handleExternalApkInstall(e: CustomEvent<{ path: string; name: string }>) {
    const { path, name } = e.detail;

    // 等待环境检测完成
    if (envCheckReady) {
      await envCheckReady;
    }

    // 环境未就绪
    if (!envReady) {
      showToast($t("android.apkNeedSetup"), "error");
      return;
    }

    // 模拟器未运行，尝试启动
    if (emulatorStatus !== "running") {
      showToast($t("android.apkStartingEmulator"), "info");
      try {
        await androidService.startContainer();
        await new Promise(r => setTimeout(r, 2000));
        await androidService.connect(EMULATOR_ADDRESS);
        // 等待模拟器就绪（最多 30 秒）
        const ready = await waitForEmulatorRunning(30000);
        if (!ready) {
          showToast($t("android.apkEmulatorStartFailed"), "error");
          return;
        }
      } catch (e: any) {
        showToast($t("android.apkEmulatorStartFailed"), "error");
        emulatorStatus = "error";
        return;
      }
    }

    // 安装 APK
    showToast($t("android.apkInstallingFile", { values: { name } }), "info");
    try {
      const app = await androidService.installAPK(path);
      const appName = app.app_name || app.package_name || name;
      showToast($t("android.installed", { values: { name: appName } }), "success");
      addToRecentPaths(path);

      // 启动刚安装的应用
      if (app.package_name) {
        try {
          await androidService.launchApp(EMULATOR_ADDRESS, app.package_name);
        } catch {
          // 启动失败不影响安装结果
        }
      }
    } catch (e: any) {
      showToast($t("android.apkInstallFailed", { values: { error: e.message } }), "error");
    }
  }

  /** 等待模拟器进入 running 状态 */
  async function waitForEmulatorRunning(timeoutMs: number): Promise<boolean> {
    const start = Date.now();
    while (Date.now() - start < timeoutMs) {
      try {
        const devices = await androidService.getDevices();
        const emulator = devices.find(d => d.id === EMULATOR_ADDRESS || d.address === EMULATOR_ADDRESS);
        if (emulator && emulator.status === "device") {
          emulatorStatus = "running";
          emulatorInfo = {
            name: emulator.name || emulator.model || $t("android.emulatorTitle"),
            androidVersion: emulator.android_version || $t("android.unknown"),
          };
          return true;
        }
      } catch {}
      await new Promise(r => setTimeout(r, 2000));
    }
    return false;
  }

  // ==================== 环境检测 ====================
  async function checkEnv() {
    try {
      const data = await androidService.getEnvStatus();
      envStatus = data;
      // 容器已存在（无论运行与否）视为环境就绪
      envReady = data?.is_ready ?? false;
    } catch {
      envReady = true;
    }
    if (envReady) {
      stopEnvPoll();
      stopInstallWs();
      await checkEmulatorStatus();
    }
  }

  function startEnvPoll() {
    if (envPollTimer) return;
    envPollTimer = setInterval(checkEnv, 2000);
  }

  function stopEnvPoll() {
    if (envPollTimer) { clearInterval(envPollTimer); envPollTimer = null; }
  }

  // ==================== 安装流程 ====================
  async function handleStartInstall() {
    try {
      installError = null;
      await androidService.startInstall();
      installRunning = true;
      showToast($t("android.installStarted"), "success");
      connectInstallWs();
      startEnvPoll();
    } catch (e: any) {
      showToast(e.message, "error");
      installError = e.message;
    }
  }

  async function handleCancelInstall() {
    try {
      await androidService.cancelInstall();
      installRunning = false;
      stopInstallWs();
      showToast($t("android.installCancelled"), "success");
      await checkEnv();
    } catch (e: any) { showToast(e.message, "error"); }
  }

  function connectInstallWs() {
    stopInstallWs();
    installWs = androidService.connectInstallWS((msg: any) => {
      if (msg.type === "init") {
        installSteps = msg.data?.steps ?? [];
        installRunning = msg.data?.is_running ?? false;
      } else if (msg.type === "step_update") {
        const step = msg.data;
        installSteps = installSteps.map((s: any) => s.step === step.step ? step : s);
        if (step.step === "completed" && step.status === "completed") {
          installRunning = false;
          checkEnv();
        }
        if (step.status === "failed") {
          installError = step.error || $t("android.installFailed");
          installRunning = false;
        }
      }
    });
  }

  function stopInstallWs() {
    if (installWs) { installWs.close(); installWs = null; }
  }

  // ==================== 模拟器控制 ====================
  let mirrorDelayTimer: ReturnType<typeof setTimeout> | null = null;
  let mirrorDelaySeconds = $state(0);
  
  async function checkEmulatorStatus(autoMirror = true) {
    try {
      const devices = await androidService.getDevices();
      const emulator = devices.find(d => d.id === EMULATOR_ADDRESS || d.address === EMULATOR_ADDRESS);
      if (emulator && emulator.status === "device") {
        const wasStarting = emulatorStatus === "starting";
        emulatorStatus = "running";
        emulatorInfo = {
          name: emulator.name || emulator.model || $t("android.emulatorTitle"),
          androidVersion: emulator.android_version || $t("android.unknown"),
        };
        // 自动开始投屏（如果是刚启动则延迟15秒）
        if (autoMirror && !mirrorConnected && !scrcpyClient && !mirrorDelayTimer) {
          if (wasStarting) {
            // 刚启动完成，等待15秒让scrcpy server准备好
            mirrorDelaySeconds = 15;
            showToast($t("android.emulatorStartedWaiting"), "success");
            mirrorDelayTimer = setInterval(() => {
              mirrorDelaySeconds--;
              if (mirrorDelaySeconds <= 0) {
                if (mirrorDelayTimer) clearInterval(mirrorDelayTimer);
                mirrorDelayTimer = null;
                startMirror();
              }
            }, 1000);
          } else {
            await startMirror();
          }
        }
      } else {
        emulatorStatus = "stopped";
        emulatorInfo = null;
      }
    } catch {
      emulatorStatus = "stopped";
      emulatorInfo = null;
    }
  }

  async function startEmulator() {
    emulatorStatus = "starting";
    try {
      // 启动容器
      await androidService.startContainer();
      showToast($t("android.startingEmulator"), "success");
      // 等待容器启动后连接 ADB
      await new Promise(r => setTimeout(r, 2000));
      await androidService.connect(EMULATOR_ADDRESS);
      // 轮询检查状态
      const pollInterval = setInterval(async () => {
        await checkEmulatorStatus();
        if (emulatorStatus === "running") {
          clearInterval(pollInterval);
        }
      }, 2000);
      // 30秒超时
      setTimeout(() => {
        clearInterval(pollInterval);
        if (emulatorStatus === "starting") {
          emulatorStatus = "error";
          showToast($t("android.startTimeout"), "error");
        }
      }, 30000);
    } catch (e: any) {
      emulatorStatus = "error";
      showToast(e.message, "error");
    }
  }

  async function stopEmulator() {
    emulatorStatus = "stopping";
    try {
      await stopMirror();
      await androidService.stopContainer();
      emulatorStatus = "stopped";
      emulatorInfo = null;
      showToast($t("android.emulatorStopped"), "success");
    } catch (e: any) {
      showToast(e.message, "error");
      emulatorStatus = "running";
    }
  }

  async function rebootEmulator() {
    try {
      await androidService.reboot(EMULATOR_ADDRESS);
      showToast($t("android.restarting"), "success");
      emulatorStatus = "starting";
      await stopMirror();
      setTimeout(() => checkEmulatorStatus(), 5000);
    } catch (e: any) {
      showToast(e.message, "error");
    }
  }

  // ==================== 投屏控制 ====================
  async function startMirror() {
    try {
      const data = await api.post<APIResp>("/android/session/start", {
        serial: EMULATOR_ADDRESS,
        config: {
          maxSize: mirrorConfig.maxSize,
          bitrate: mirrorConfig.bitrate,
          maxFps: mirrorConfig.maxFps,
          audioEnabled: mirrorConfig.audioEnabled,
          showTouches: mirrorConfig.showTouches,
        },
      });
      if (!data.success) throw new Error(data.error || $t("android.mirrorFailed"));

      await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));

      const protocol = location.protocol === "https:" ? "wss:" : "ws:";
      const wsUrl = `${protocol}//${location.host}${ANDROID_WS_BASE}/ws/screen`;

      scrcpyClient = new ScrcpyClient({
        onConnected: () => { mirrorConnected = true; },
        onDisconnected: () => { mirrorConnected = false; },
        onSession: (s) => { mirrorSession = s; },
        onError: (err) => { showToast(err, "error"); },
        onAudioStatus: (s) => { mirrorAudioStatus = s; },
      });

      await scrcpyClient.connect(wsUrl, videoElement, videoContainer, mirrorConfig.audioEnabled);
    } catch (e: any) {
      showToast(e.message, "error");
    }
  }

  async function stopMirror() {
    // 清理延迟定时器
    if (mirrorDelayTimer) {
      clearInterval(mirrorDelayTimer);
      mirrorDelayTimer = null;
      mirrorDelaySeconds = 0;
    }
    if (scrcpyClient) {
      scrcpyClient.disconnect();
      scrcpyClient = null;
    }
    mirrorConnected = false;
    mirrorSession = null;
    try {
      await api.post("/android/session/stop").catch(() => {});
    } catch {}
  }

  // ==================== 工具栏操作 ====================
  function handlePower() {
    if (emulatorStatus === "running") {
      stopEmulator();
    } else if (emulatorStatus === "stopped" || emulatorStatus === "error") {
      startEmulator();
    }
  }

  function handleVolumeUp() {
    scrcpyClient?.sendControl({ type: "key", data: { keycode: 24, action: "down" } }); // KEYCODE_VOLUME_UP
  }

  function handleVolumeDown() {
    scrcpyClient?.sendControl({ type: "key", data: { keycode: 25, action: "down" } }); // KEYCODE_VOLUME_DOWN
  }

  function handleRotate() {
    scrcpyClient?.sendControl({ type: "rotate" });
  }

  async function handleScreenshot() {
    try {
      let image = await androidService.screenshot(EMULATOR_ADDRESS);
      // 去掉 data:image/png;base64, 前缀
      if (image.startsWith("data:")) {
        image = image.split(",")[1] || image;
      }
      // Base64 转 Blob
      const byteCharacters = atob(image);
      const byteNumbers = new Array(byteCharacters.length);
      for (let i = 0; i < byteCharacters.length; i++) {
        byteNumbers[i] = byteCharacters.charCodeAt(i);
      }
      const byteArray = new Uint8Array(byteNumbers);
      const blob = new Blob([byteArray], { type: "image/png" });
      // 创建 Blob URL 下载
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `screenshot_${Date.now()}.png`;
      link.click();
      URL.revokeObjectURL(url);
      showToast($t("android.screenshotSaved"), "success");
    } catch (e: any) {
      showToast(e.message, "error");
    }
  }

  function handleZoomIn() {
    zoomLevel = Math.min(ZOOM_MAX, zoomLevel + ZOOM_STEP);
  }

  function handleZoomOut() {
    zoomLevel = Math.max(ZOOM_MIN, zoomLevel - ZOOM_STEP);
  }

  function handleBack() {
    scrcpyClient?.sendControl({ type: "back" });
  }

  function handleHome() {
    scrcpyClient?.sendControl({ type: "home" });
  }

  function handleRecent() {
    scrcpyClient?.sendControl({ type: "recent" });
  }

  function toggleSettings() {
    showSettings = !showSettings;
  }

  // ==================== APK 安装操作 ====================
  function openApkDialog() {
    showApkDialog = true;
    apkInstallProgress = "";
    selectedFiles = new Set();
    searchQuery = "";
    // 从 localStorage 加载最近安装记录
    try {
      const saved = localStorage.getItem("rde_recent_apk_paths");
      if (saved) recentPaths = JSON.parse(saved);
    } catch {}
  }

  function closeApkDialog() {
    if (apkInstalling) return;
    showApkDialog = false;
    showServerBrowser = false;
    apkDragOver = false;
    selectedFiles = new Set();
    searchQuery = "";
  }

  async function handleApkFromServer() {
    showServerBrowser = true;
    serverPath = "/home";
    selectedFiles = new Set();
    searchQuery = "";
    await loadServerFiles();
  }

  async function loadServerFiles() {
    serverLoading = true;
    try {
      const res = await fileService.list(serverPath, false);
      if (res.success && res.data?.content) {
        // 只显示目录和 .apk 文件
        serverFiles = res.data.content.filter(f => 
          f.is_dir || f.name.toLowerCase().endsWith(".apk")
        );
      } else {
        serverFiles = [];
      }
    } catch (e: any) {
      showToast($t("android.loadFileListFailed") + " " + e.message, "error");
      serverFiles = [];
    } finally {
      serverLoading = false;
    }
  }

  async function navigateServerPath(path: string) {
    serverPath = path;
    selectedFiles = new Set(); // 切换目录时清空选择
    await loadServerFiles();
  }

  function toggleFileSelection(path: string) {
    const newSet = new Set(selectedFiles);
    if (newSet.has(path)) {
      newSet.delete(path);
    } else {
      newSet.add(path);
    }
    selectedFiles = newSet;
  }

  function selectAllApks() {
    const apkFiles = filteredFiles().filter(f => !f.is_dir);
    const allSelected = apkFiles.every(f => selectedFiles.has(serverPath + "/" + f.name));
    if (allSelected) {
      selectedFiles = new Set();
    } else {
      selectedFiles = new Set(apkFiles.map(f => serverPath + "/" + f.name));
    }
  }

  function clearSelection() {
    selectedFiles = new Set();
  }

  function toggleSortOrder() {
    sortOrder = sortOrder === "asc" ? "desc" : "asc";
  }

  async function installSelectedFiles() {
    if (selectedFiles.size === 0) return;
    
    const paths = Array.from(selectedFiles);
    apkInstalling = true;
    
    let successCount = 0;
    let failCount = 0;
    
    for (let i = 0; i < paths.length; i++) {
      const path = paths[i];
      const fileName = path.split('/').pop() || path;
      apkInstallProgress = `${$t("android.installing")} (${i + 1}/${paths.length}): ${fileName}`;
      
      try {
        await androidService.installAPK(path);
        successCount++;
        // 添加到最近安装
        addToRecentPaths(path);
      } catch (e: any) {
        failCount++;
        console.error(`Install failed: ${path}`, e);
      }
    }
    
    apkInstalling = false;
    selectedFiles = new Set();
    
    if (failCount === 0) {
      showToast($t("android.successInstalled", { n: successCount }), "success");
      closeApkDialog();
    } else {
      showToast($t("android.installComplete", { success: successCount, fail: failCount }), failCount > 0 ? "error" : "success");
    }
  }

  function addToRecentPaths(path: string) {
    // 去重并限制数量
    const newPaths = [path, ...recentPaths.filter(p => p !== path)].slice(0, 10);
    recentPaths = newPaths;
    try {
      localStorage.setItem("rde_recent_apk_paths", JSON.stringify(newPaths));
    } catch {}
  }

  async function installFromRecent(path: string) {
    apkInstalling = true;
    apkInstallProgress = `${$t("android.installing")} ${path.split('/').pop()}...`;
    
    try {
      const app = await androidService.installAPK(path);
      showToast($t("android.installed", { name: app.app_name || app.package_name }), "success");
      addToRecentPaths(path);
      closeApkDialog();
    } catch (e: any) {
      showToast(e.message, "error");
      apkInstallProgress = $t("android.installFailed2", { error: e.message });
    } finally {
      apkInstalling = false;
    }
  }

  function removeFromRecent(path: string) {
    recentPaths = recentPaths.filter(p => p !== path);
    try {
      localStorage.setItem("rde_recent_apk_paths", JSON.stringify(recentPaths));
    } catch {}
  }

  function selectServerFile(file: { name: string; is_dir: boolean; size: number }) {
    if (file.is_dir) {
      navigateServerPath(serverPath === "/" ? "/" + file.name : serverPath + "/" + file.name);
    } else {
      toggleFileSelection(serverPath + "/" + file.name);
    }
  }

  function handleApkFromLocal() {
    apkFileInput?.click();
  }

  async function handleApkFileSelected(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    await installApkFile(file);
    input.value = "";
  }

  function handleApkDragOver(e: DragEvent) {
    e.preventDefault();
    apkDragOver = true;
  }

  function handleApkDragLeave(e: DragEvent) {
    e.preventDefault();
    apkDragOver = false;
  }

  async function handleApkDrop(e: DragEvent) {
    e.preventDefault();
    apkDragOver = false;
    const file = e.dataTransfer?.files[0];
    if (!file) return;
    if (!file.name.toLowerCase().endsWith(".apk")) {
      showToast($t("android.onlyApkSupported"), "error");
      return;
    }
    await installApkFile(file);
  }

  async function installApkFile(file: File) {
    if (apkInstalling) return;
    apkInstalling = true;
    apkInstallProgress = `${$t("android.installing")} ${file.name}...`;
    
    try {
      apkInstallProgress = $t("android.installing");
      const app = await androidService.uploadAPK(file);
      showToast($t("android.installed", { name: app.app_name || app.package_name }), "success");
      closeApkDialog();
    } catch (e: any) {
      showToast(e.message, "error");
      apkInstallProgress = $t("android.installFailed2", { error: e.message });
    } finally {
      apkInstalling = false;
    }
  }

  function formatFileSize(bytes: number): string {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  }
</script>

{#if envReady === null}
  <!-- 检测中 -->
  <div class="emulator-container">
    <div class="loading-state">
      <Spinner />
      <span>{$t("android.detectingEnv")}</span>
    </div>
  </div>
{:else if !envReady && envStatus}
  <!-- 环境设置面板 -->
  <div class="emulator-container">
    <div class="setup-panel">
      <div class="setup-header">
        <Icon icon="mdi:android" width="32" />
        <div>
          <h2>{$t("android.androidEmulator")}</h2>
          <p class="setup-subtitle">{$t("android.completeEnvSetup")}</p>
        </div>
      </div>

      <div class="setup-steps">
        <div class="setup-card">
          <div class="setup-card-header">
            <Icon icon="mdi:clipboard-check-outline" width="20" />
            <span class="setup-card-title">{$t("android.envCheck")}</span>
          </div>
          <div class="setup-card-meta">
            <span>DKMS: {envStatus.dkms_installed ? "✅" : "❌"}</span>
            <span>{$t("android.kernelHeaders")} {envStatus.headers_installed ? "✅" : "❌"}</span>
            <span>Binder: {envStatus.binder_loaded ? "✅" : "❌"}</span>
            <span>Docker: {envStatus.docker_installed ? "✅" : "❌"}</span>
            <span>{$t("android.image")} {envStatus.image_exists ? "✅" : "❌"}</span>
            <span>{$t("android.container")} {envStatus.container_running ? "✅" : envStatus.container_exists ? "⚠️ " + $t("android.stopped") : "❌"}</span>
          </div>
        </div>

        {#if installSteps.length > 0}
          <div class="setup-card">
            <div class="setup-card-header">
              <Icon icon="mdi:progress-wrench" width="20" />
              <span class="setup-card-title">{$t("android.installProgress")}</span>
            </div>
            <div class="install-steps">
              {#each installSteps as step}
                <div class="install-step" class:completed={step.status === "completed"} class:active={step.status === "in_progress"} class:failed={step.status === "failed"}>
                  <span class="step-icon">
                    {#if step.status === "completed"}✅
                    {:else if step.status === "in_progress"}<Spinner size="sm" />
                    {:else if step.status === "failed"}❌
                    {:else if step.status === "skipped"}⏭
                    {:else}⬜{/if}
                  </span>
                  <span class="step-title">{step.title || step.step}</span>
                </div>
              {/each}
            </div>
          </div>
        {/if}

        {#if installError}
          <p class="setup-error">{installError}</p>
        {/if}
      </div>

      <div class="setup-footer">
        {#if !installRunning}
          <Button variant="primary" size="sm" onclick={handleStartInstall}>
            <Icon icon="mdi:download" width="16" /> {$t("android.oneClickInstall")}
          </Button>
        {:else}
          <Button variant="ghost" size="sm" onclick={handleCancelInstall}>
            <Icon icon="mdi:close" width="16" /> {$t("android.cancelInstall")}
          </Button>
        {/if}
        <Button variant="ghost" size="sm" onclick={checkEnv}>
          <Icon icon="mdi:refresh" width="16" /> {$t("android.redetect")}
        </Button>
      </div>
    </div>
  </div>
{:else}
  <!-- 主视图：投屏 + 工具栏 -->
  <div class="emulator-container">
    <div class="emulator-main">
      <!-- 投屏区域 -->
      <div class="screen-wrapper">
        <div
          class="screen-container"
          bind:this={videoContainer}
          style="transform: scale({zoomLevel}); aspect-ratio: {screenAspectRatio}"
        >
          <video bind:this={videoElement} autoplay playsinline class:hidden={!mirrorConnected}></video>
          {#if emulatorStatus === "running" && mirrorConnected}
            <!-- 投屏正常，不显示 overlay -->
          {:else if emulatorStatus === "running" && !mirrorConnected}
            <div class="screen-overlay">
              <Spinner />
              {#if mirrorDelaySeconds > 0}
                <span>{$t("android.waitingMirrorService")} ({mirrorDelaySeconds}s)...</span>
              {:else}
                <span>{$t("android.connectingMirror")}</span>
              {/if}
            </div>
          {:else if emulatorStatus === "starting"}
            <div class="screen-overlay">
              <Spinner />
              <span>{$t("android.startingEmulator")}</span>
            </div>
          {:else if emulatorStatus === "stopping"}
            <div class="screen-overlay">
              <Spinner />
              <span>{$t("android.stopping")}</span>
            </div>
          {:else if emulatorStatus === "error"}
            <div class="screen-overlay error">
              <Icon icon="mdi:alert-circle" width="48" />
              <span>{$t("android.startFailed")}</span>
              <Button variant="primary" size="sm" onclick={startEmulator}>
                <Icon icon="mdi:refresh" width="16" /> {$t("android.retry")}
              </Button>
            </div>
          {:else}
            <div class="screen-overlay">
              <Icon icon="mdi:android" width="64" class="android-icon" />
              <span class="title">{$t("android.androidEmulator")}</span>
              <Button variant="primary" onclick={startEmulator}>
                <Icon icon="mdi:play" width="18" /> {$t("android.emulatorTitle")}
              </Button>
            </div>
          {/if}
        </div>
      </div>

      <!-- 右侧工具栏 -->
      <div class="toolbar">
        <button
          class="tool-btn"
          class:active={emulatorStatus === "running"}
          class:tool-btn-danger={emulatorStatus === "running"}
          onclick={handlePower}
          title={emulatorStatus === "running" ? $t("android.stopEmulator") : $t("android.startEmulator")}
        >
          <Icon icon="mdi:power" width="20" />
        </button>

        <div class="tool-divider"></div>

        <button class="tool-btn" onclick={handleVolumeUp} disabled={emulatorStatus !== "running"} title={$t("android.volumeUp")}>
          <Icon icon="mdi:volume-plus" width="20" />
        </button>
        <button class="tool-btn" onclick={handleVolumeDown} disabled={emulatorStatus !== "running"} title={$t("android.volumeDown")}>
          <Icon icon="mdi:volume-minus" width="20" />
        </button>

        <div class="tool-divider"></div>

        <button class="tool-btn" onclick={handleRotate} disabled={emulatorStatus !== "running"} title={$t("android.rotateScreen")}>
          <Icon icon="mdi:screen-rotation" width="20" />
        </button>

        <div class="tool-divider"></div>

        <button class="tool-btn" onclick={handleScreenshot} disabled={emulatorStatus !== "running"} title={$t("android.screenshot")}>
          <Icon icon="mdi:camera" width="20" />
        </button>

        <div class="tool-divider"></div>

        <button class="tool-btn" onclick={handleZoomIn} disabled={zoomLevel >= ZOOM_MAX} title={$t("android.zoomIn")}>
          <Icon icon="mdi:magnify-plus" width="20" />
        </button>
        <button class="tool-btn" onclick={handleZoomOut} disabled={zoomLevel <= ZOOM_MIN} title={$t("android.zoomOut")}>
          <Icon icon="mdi:magnify-minus" width="20" />
        </button>

        <div class="tool-divider"></div>

        <button class="tool-btn" onclick={handleBack} disabled={emulatorStatus !== "running"} title={$t("android.back")}>
          <Icon icon="mdi:arrow-left" width="20" />
        </button>
        <button class="tool-btn" onclick={handleHome} disabled={emulatorStatus !== "running"} title={$t("android.home")}>
          <Icon icon="mdi:circle-outline" width="20" />
        </button>
        <button class="tool-btn" onclick={handleRecent} disabled={emulatorStatus !== "running"} title={$t("android.recentTasks")}>
          <Icon icon="mdi:square-outline" width="20" />
        </button>

        <div class="tool-divider"></div>

        <button class="tool-btn" onclick={openApkDialog} disabled={emulatorStatus !== "running"} title={$t("android.installApk")}>
          <Icon icon="mdi:package-down" width="20" />
        </button>

        <div class="tool-spacer"></div>

        <button class="tool-btn" onclick={toggleSettings} title={$t("android.settings")}>
          <Icon icon="mdi:cog" width="20" />
        </button>
      </div>
    </div>

    <!-- 设置面板 -->
    {#if showSettings}
      <div class="settings-panel">
        <div class="settings-header">
          <span>{$t("android.settings")}</span>
          <button class="close-btn" onclick={() => showSettings = false}>
            <Icon icon="mdi:close" width="18" />
          </button>
        </div>
        <div class="settings-content">
          <label class="setting-item">
            <span>{$t("android.resolution")}</span>
            <select bind:value={mirrorConfig.maxSize}>
              <option value={720}>720p</option>
              <option value={1280}>1280p</option>
              <option value={1920}>1920p</option>
            </select>
          </label>
          <label class="setting-item">
            <span>{$t("android.bitrate")}</span>
            <select bind:value={mirrorConfig.bitrate}>
              <option value={4000000}>4 Mbps</option>
              <option value={8000000}>8 Mbps</option>
              <option value={16000000}>16 Mbps</option>
            </select>
          </label>
          <label class="setting-item">
            <span>{$t("android.frameRate")}</span>
            <select bind:value={mirrorConfig.maxFps}>
              <option value={30}>30 fps</option>
              <option value={60}>60 fps</option>
              <option value={120}>120 fps</option>
            </select>
          </label>
          <label class="setting-checkbox">
            <input type="checkbox" bind:checked={mirrorConfig.audioEnabled} />
            <span>{$t("android.enableAudio")}</span>
          </label>
          <label class="setting-checkbox">
            <input type="checkbox" bind:checked={mirrorConfig.showTouches} />
            <span>{$t("android.showTouchFeedback")}</span>
          </label>

          {#if emulatorStatus === "running"}
            <div class="settings-actions">
              <Button variant="ghost" size="sm" onclick={rebootEmulator}>
                <Icon icon="mdi:restart" width="16" /> {$t("android.restartEmulator")}
              </Button>
            </div>
          {/if}
        </div>
      </div>
    {/if}

    <!-- APK 安装对话框 -->
    {#if showApkDialog}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="apk-dialog-overlay" onclick={closeApkDialog}>
        <div class="apk-dialog" class:browser-mode={showServerBrowser} onclick={(e) => e.stopPropagation()}>
          <div class="apk-dialog-header">
            {#if showServerBrowser}
              <button class="back-btn" onclick={() => showServerBrowser = false} disabled={apkInstalling}>
                <Icon icon="mdi:arrow-left" width="18" />
              </button>
              <span>{$t("android.selectApkFile")}</span>
            {:else}
              <span>{$t("android.installApk")}</span>
            {/if}
            <button class="close-btn" onclick={closeApkDialog} disabled={apkInstalling}>
              <Icon icon="mdi:close" width="18" />
            </button>
          </div>
          
          <div class="apk-dialog-content">
            {#if showServerBrowser}
              <!-- 服务器文件浏览器（增强版） -->
              <div class="server-browser">
                <!-- 工具栏：搜索、视图切换、排序 -->
                <div class="browser-toolbar">
                  <div class="search-box">
                    <Icon icon="mdi:magnify" width="16" />
                    <input 
                      type="text" 
                      placeholder={$t("android.searchFiles")} 
                      bind:value={searchQuery}
                      class="search-input"
                    />
                    {#if searchQuery}
                      <button class="search-clear" onclick={() => searchQuery = ""}>
                        <Icon icon="mdi:close" width="14" />
                      </button>
                    {/if}
                  </div>
                  
                  <div class="toolbar-actions">
                    <!-- 视图切换 -->
                    <div class="view-toggle">
                      <button 
                        class="view-btn" 
                        class:active={viewMode === "list"}
                        onclick={() => viewMode = "list"}
                        title={$t("android.listView")}
                      >
                        <Icon icon="mdi:view-list" width="18" />
                      </button>
                      <button 
                        class="view-btn"
                        class:active={viewMode === "grid"}
                        onclick={() => viewMode = "grid"}
                        title={$t("android.gridView")}
                      >
                        <Icon icon="mdi:view-grid" width="18" />
                      </button>
                    </div>
                    
                    <!-- 排序 -->
                    <div class="sort-dropdown">
                      <select bind:value={sortBy} class="sort-select">
                        <option value="name">{$t("android.sortByName")}</option>
                        <option value="size">{$t("android.sortBySize")}</option>
                        <option value="modified">{$t("android.sortByModified")}</option>
                      </select>
                      <button class="sort-order-btn" onclick={toggleSortOrder} title={sortOrder === "asc" ? $t("android.sortAsc") : $t("android.sortDesc")}>
                        <Icon icon={sortOrder === "asc" ? "mdi:sort-ascending" : "mdi:sort-descending"} width="18" />
                      </button>
                    </div>
                  </div>
                </div>
                
                <!-- 路径导航 -->
                <div class="path-nav">
                  <button class="path-btn" onclick={() => navigateServerPath("/")} disabled={serverLoading}>
                    <Icon icon="mdi:home" width="16" />
                  </button>
                  {#each serverPath.split("/").filter(Boolean) as part, i}
                    <span class="path-sep">/</span>
                    <button 
                      class="path-btn"
                      onclick={() => navigateServerPath("/" + serverPath.split("/").filter(Boolean).slice(0, i + 1).join("/"))}
                      disabled={serverLoading}
                    >
                      {part}
                    </button>
                  {/each}
                </div>
                
                <!-- 最近安装的APK -->
                {#if recentPaths.length > 0 && serverPath === "/"}
                  <div class="recent-section">
                    <div class="recent-header">
                      <Icon icon="mdi:history" width="16" />
                      <span>{$t("android.recentInstall")}</span>
                    </div>
                    <div class="recent-list">
                      {#each recentPaths.slice(0, 5) as path}
                        <div class="recent-item">
                          <button class="recent-btn" onclick={() => installFromRecent(path)} disabled={apkInstalling}>
                            <Icon icon="mdi:package-variant" width="16" />
                            <span class="recent-name">{path.split("/").pop()}</span>
                          </button>
                          <button class="recent-remove" onclick={() => removeFromRecent(path)} title={$t("android.removeFromHistory")}>
                            <Icon icon="mdi:close" width="14" />
                          </button>
                        </div>
                      {/each}
                    </div>
                  </div>
                {/if}
                
                <!-- 文件列表 -->
                <div class="file-list" class:grid-view={viewMode === "grid"}>
                  {#if serverLoading}
                    <div class="file-list-loading">
                      <Spinner />
                      <span>{$t("common.loading")}</span>
                    </div>
                  {:else if filteredFiles().length === 0 && serverFiles.length > 0}
                    <div class="file-list-empty">
                      <Icon icon="mdi:magnify" width="32" />
                      <span>{$t("android.noMatchingFiles")}</span>
                    </div>
                  {:else if serverFiles.length === 0}
                    <div class="file-list-empty">
                      <Icon icon="mdi:folder-open-outline" width="32" />
                      <span>{$t("android.noApkFiles")}</span>
                    </div>
                  {:else}
                    <!-- 上级目录 -->
                    {#if serverPath !== "/"}
                      <button 
                        class="file-item parent-dir" 
                        onclick={() => navigateServerPath(serverPath.split("/").slice(0, -1).join("/") || "/")}
                      >
                        <Icon icon="mdi:folder-upload" width="20" class="file-icon folder" />
                        <span class="file-name">..</span>
                      </button>
                    {/if}
                    
                    <!-- 全选按钮（仅当有APK文件时） -->
                    {#if filteredFiles().some(f => !f.is_dir)}
                      <div class="select-all-row">
                        <label class="select-all-label">
                          <input 
                            type="checkbox"
                            checked={filteredFiles().filter(f => !f.is_dir).every(f => selectedFiles.has(serverPath + "/" + f.name))}
                            onchange={selectAllApks}
                          />
                          <span>{$t("android.selectAllApks")}</span>
                        </label>
                        {#if selectedFiles.size > 0}
                          <span class="selection-hint">{$t("android.selectedCount", { n: selectedFiles.size })}</span>
                        {/if}
                      </div>
                    {/if}
                    
                    {#each filteredFiles() as file}
                      <div class="file-item" class:selected={selectedFiles.has(serverPath + "/" + file.name)}>
                        {#if !file.is_dir}
                          <input 
                            type="checkbox"
                            class="file-checkbox"
                            checked={selectedFiles.has(serverPath + "/" + file.name)}
                            onchange={() => toggleFileSelection(serverPath + "/" + file.name)}
                          />
                        {/if}
                        <button class="file-btn" onclick={() => selectServerFile(file)}>
                          <Icon 
                            icon={file.is_dir ? "mdi:folder" : "mdi:package-variant"} 
                            width={viewMode === "grid" ? 32 : 20}
                            class="file-icon {file.is_dir ? 'folder' : 'apk'}"
                          />
                          <span class="file-name">{file.name}</span>
                          {#if !file.is_dir && viewMode === "list"}
                            <span class="file-size">{formatFileSize(file.size)}</span>
                          {/if}
                        </button>
                      </div>
                    {/each}
                  {/if}
                </div>
                
                <!-- 选择操作栏 -->
                {#if selectedFiles.size > 0}
                  <div class="selection-footer">
                    <span class="selection-count">{$t("android.selectedFileCount", { n: selectedFiles.size })}</span>
                    <div class="selection-actions">
                      <Button variant="ghost" size="sm" onclick={clearSelection}>
                        {$t("common.cancel")}
                      </Button>
                      <Button variant="default" size="sm" onclick={installSelectedFiles} disabled={apkInstalling}>
                        {#if apkInstalling}
                          <Spinner size="sm" />
                          <span>{$t("android.installing")}</span>
                        {:else}
                          <Icon icon="mdi:download" width="16" />
                          <span>{$t("android.installCount", { n: selectedFiles.size })}</span>
                        {/if}
                      </Button>
                    </div>
                  </div>
                {/if}
              </div>
            {:else}
              <!-- 选择按钮行 -->
              <div class="apk-source-buttons">
                <button class="apk-source-btn" onclick={handleApkFromServer} disabled={apkInstalling}>
                  <Icon icon="mdi:folder-open" width="24" />
                  <span>{$t("android.fromServer")}</span>
                </button>
                <button class="apk-source-btn" onclick={handleApkFromLocal} disabled={apkInstalling}>
                  <Icon icon="mdi:laptop" width="24" />
                  <span>{$t("android.fromLocal")}</span>
                </button>
              </div>

              <!-- 拖拽区域 -->
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div 
                class="apk-drop-zone"
                class:drag-over={apkDragOver}
                ondragover={handleApkDragOver}
                ondragleave={handleApkDragLeave}
                ondrop={handleApkDrop}
                onclick={handleApkFromLocal}
              >
                {#if apkInstalling}
                  <Spinner />
                  <span>{apkInstallProgress}</span>
                {:else}
                  <Icon icon="mdi:package-variant" width="48" class="drop-icon" />
                  <span>{$t("android.dragApkHere")}</span>
                  <span class="drop-hint">{$t("android.orClickToSelect")}</span>
                {/if}
              </div>
            {/if}
          </div>

          <input 
            type="file" 
            accept=".apk"
            class="hidden"
            bind:this={apkFileInput}
            onchange={handleApkFileSelected}
          />
        </div>
      </div>
    {/if}


  </div>
{/if}

<style>
  .emulator-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: #1a1a1a;
    position: relative;
  }

  .loading-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    color: #888;
  }

  /* ==================== 主视图 ==================== */
  .emulator-main {
    flex: 1;
    display: flex;
    overflow: hidden;
  }

  .screen-wrapper {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    padding: 16px;
  }

  .screen-container {
    position: relative;
    background: #000;
    border-radius: 8px;
    overflow: hidden;
    /* aspect-ratio 由 style 动态绑定 */
    height: 100%;
    max-height: calc(100vh - 100px);
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
    transition: transform 0.2s ease, aspect-ratio 0.3s ease;
  }

  .screen-container video {
    width: 100%;
    height: 100%;
    object-fit: contain;
  }

  .screen-container video.hidden {
    visibility: hidden;
  }

  .screen-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    background: linear-gradient(135deg, #1e1e1e 0%, #2d2d2d 100%);
    color: #aaa;
  }

  .screen-overlay.error {
    color: #ff6b6b;
  }

  .screen-overlay .title {
    font-size: 18px;
    font-weight: 500;
    color: #ccc;
  }

  .screen-overlay :global(.android-icon) {
    color: #3ddc84;
    opacity: 0.8;
  }

  /* ==================== 工具栏 ==================== */
  .toolbar {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 8px;
    background: #252525;
    border-left: 1px solid #333;
    gap: 2px;
  }

  .tool-btn {
    width: 36px;
    height: 36px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: #aaa;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.15s;
  }

  .tool-btn:hover:not(:disabled) {
    background: #333;
    color: #fff;
  }

  .tool-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  .tool-btn.active {
    color: #3ddc84;
  }

  .tool-btn-danger {
    color: #ff6b6b;
  }

  .tool-btn-danger:hover:not(:disabled) {
    background: rgba(255, 107, 107, 0.15);
    color: #ff6b6b;
  }

  .tool-divider {
    width: 24px;
    height: 1px;
    background: #333;
    margin: 4px 0;
  }

  .tool-spacer {
    flex: 1;
  }

  /* ==================== 设置面板 ==================== */
  .settings-panel {
    position: absolute;
    top: 8px;
    right: 60px;
    width: 240px;
    background: #2a2a2a;
    border-radius: 8px;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
    border: 1px solid #333;
    z-index: 100;
  }

  .settings-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 14px;
    border-bottom: 1px solid #333;
    font-size: 14px;
    font-weight: 500;
    color: #ccc;
  }

  .close-btn {
    background: none;
    border: none;
    color: #888;
    cursor: pointer;
    padding: 2px;
    display: flex;
  }

  .close-btn:hover {
    color: #fff;
  }

  .settings-content {
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .setting-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 12px;
    color: #888;
  }

  .setting-item select {
    padding: 6px 8px;
    border-radius: 4px;
    border: 1px solid #444;
    background: #1a1a1a;
    color: #ccc;
    font-size: 12px;
  }

  .setting-checkbox {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: #aaa;
    cursor: pointer;
  }

  .setting-checkbox input {
    width: 14px;
    height: 14px;
  }

  .settings-actions {
    margin-top: 8px;
    padding-top: 12px;
    border-top: 1px solid #333;
  }



  /* ==================== 环境设置面板 ==================== */
  .setup-panel {
    max-width: 480px;
    margin: 40px auto;
    padding: 0 20px;
  }

  .setup-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 24px;
    color: #ccc;
  }

  .setup-header h2 {
    margin: 0;
    font-size: 20px;
  }

  .setup-subtitle {
    margin: 4px 0 0;
    font-size: 13px;
    color: #888;
  }

  .setup-steps {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .setup-card {
    background: #252525;
    border-radius: 10px;
    padding: 16px;
    border: 1px solid #333;
  }

  .setup-card-header {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 10px;
    color: #aaa;
  }

  .setup-card-title {
    flex: 1;
    font-weight: 600;
    font-size: 14px;
    color: #ccc;
  }

  .setup-card-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    font-size: 12px;
    color: #888;
  }

  .setup-error {
    font-size: 12px;
    color: #ff6b6b;
    background: rgba(255, 107, 107, 0.1);
    padding: 8px 10px;
    border-radius: 6px;
    margin: 0;
  }

  .setup-footer {
    margin-top: 20px;
    display: flex;
    justify-content: center;
    gap: 8px;
  }

  .install-steps {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .install-step {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 6px;
    border-radius: 4px;
    font-size: 12px;
    color: #888;
  }

  .install-step.active {
    background: rgba(61, 220, 132, 0.1);
    color: #3ddc84;
  }

  .install-step.completed {
    color: #3ddc84;
  }

  .install-step.failed {
    color: #ff6b6b;
  }

  .step-icon {
    width: 18px;
    text-align: center;
    flex-shrink: 0;
  }

  .step-title {
    flex: 1;
  }

  /* ==================== APK 安装对话框 ==================== */
  .apk-dialog-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .apk-dialog {
    width: 400px;
    max-width: 90vw;
    background: #2a2a2a;
    border-radius: 12px;
    border: 1px solid #444;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
  }

  .apk-dialog-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 16px;
    border-bottom: 1px solid #333;
    font-weight: 500;
  }

  .apk-dialog-content {
    padding: 16px;
  }

  .apk-source-buttons {
    display: flex;
    gap: 12px;
    margin-bottom: 16px;
  }

  .apk-source-btn {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 16px 12px;
    background: #333;
    border: 1px solid #444;
    border-radius: 8px;
    color: #ccc;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .apk-source-btn:hover:not(:disabled) {
    background: #3a3a3a;
    border-color: #3ddc84;
    color: #fff;
  }

  .apk-source-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .apk-source-btn span {
    font-size: 13px;
  }

  .apk-drop-zone {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 32px;
    border: 2px dashed #444;
    border-radius: 8px;
    color: #888;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .apk-drop-zone:hover {
    border-color: #555;
    background: rgba(255, 255, 255, 0.02);
  }

  .apk-drop-zone.drag-over {
    border-color: #3ddc84;
    background: rgba(61, 220, 132, 0.05);
    color: #3ddc84;
  }

  .apk-drop-zone :global(.drop-icon) {
    color: #555;
  }

  .apk-drop-zone.drag-over :global(.drop-icon) {
    color: #3ddc84;
  }

  .drop-hint {
    font-size: 12px;
    color: #666;
  }

  /* ==================== 服务器文件浏览器 ==================== */
  .apk-dialog.browser-mode {
    width: 600px;
    max-height: 80vh;
  }

  .apk-dialog-header .back-btn {
    background: transparent;
    border: none;
    color: #888;
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
    display: flex;
    align-items: center;
  }

  .apk-dialog-header .back-btn:hover {
    background: #333;
    color: #fff;
  }

  .server-browser {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  /* 工具栏 */
  .browser-toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 0;
  }

  .search-box {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 12px;
    background: #252525;
    border: 1px solid #333;
    border-radius: 6px;
    color: #888;
  }

  .search-box:focus-within {
    border-color: #3ddc84;
  }

  .search-input {
    flex: 1;
    background: transparent;
    border: none;
    color: #fff;
    font-size: 13px;
    outline: none;
  }

  .search-input::placeholder {
    color: #666;
  }

  .search-clear {
    background: transparent;
    border: none;
    color: #666;
    cursor: pointer;
    padding: 2px;
    display: flex;
    border-radius: 4px;
  }

  .search-clear:hover {
    color: #fff;
    background: #333;
  }

  .toolbar-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .view-toggle {
    display: flex;
    background: #252525;
    border-radius: 6px;
    overflow: hidden;
  }

  .view-btn {
    background: transparent;
    border: none;
    color: #666;
    cursor: pointer;
    padding: 6px 8px;
    display: flex;
    transition: all 0.15s ease;
  }

  .view-btn:hover {
    color: #888;
  }

  .view-btn.active {
    background: #333;
    color: #3ddc84;
  }

  .sort-dropdown {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .sort-select {
    background: #252525;
    border: 1px solid #333;
    border-radius: 6px;
    color: #ccc;
    padding: 6px 10px;
    font-size: 12px;
    cursor: pointer;
  }

  .sort-select:focus {
    outline: none;
    border-color: #3ddc84;
  }

  .sort-order-btn {
    background: #252525;
    border: 1px solid #333;
    border-radius: 6px;
    color: #888;
    cursor: pointer;
    padding: 6px;
    display: flex;
  }

  .sort-order-btn:hover {
    color: #fff;
    background: #333;
  }

  /* 最近安装 */
  .recent-section {
    background: #252525;
    border-radius: 6px;
    padding: 10px 12px;
  }

  .recent-header {
    display: flex;
    align-items: center;
    gap: 6px;
    color: #888;
    font-size: 12px;
    margin-bottom: 8px;
  }

  .recent-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .recent-item {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .recent-btn {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: #ccc;
    font-size: 13px;
    cursor: pointer;
    text-align: left;
  }

  .recent-btn:hover:not(:disabled) {
    background: #333;
  }

  .recent-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .recent-btn :global(.iconify) {
    color: #3ddc84;
  }

  .recent-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .recent-remove {
    background: transparent;
    border: none;
    color: #555;
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
    display: flex;
  }

  .recent-remove:hover {
    color: #ff5252;
    background: #332222;
  }

  .path-nav {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 8px 12px;
    background: #252525;
    border-radius: 6px;
    overflow-x: auto;
    flex-wrap: nowrap;
  }

  .path-btn {
    background: transparent;
    border: none;
    color: #3ddc84;
    cursor: pointer;
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 13px;
    white-space: nowrap;
  }

  .path-btn:hover:not(:disabled) {
    background: #333;
  }

  .path-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .path-sep {
    color: #555;
    font-size: 12px;
  }

  .file-list {
    flex: 1;
    min-height: 200px;
    max-height: 350px;
    overflow-y: auto;
    border: 1px solid #333;
    border-radius: 6px;
  }

  .file-list-loading,
  .file-list-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 40px;
    color: #666;
  }

  /* 全选行 */
  .select-all-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    background: #252525;
    border-bottom: 1px solid #333;
  }

  .select-all-label {
    display: flex;
    align-items: center;
    gap: 8px;
    color: #888;
    font-size: 12px;
    cursor: pointer;
  }

  .select-all-label:hover {
    color: #ccc;
  }

  .selection-hint {
    font-size: 12px;
    color: #3ddc84;
  }

  .file-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 10px 12px;
    background: transparent;
    border: none;
    border-bottom: 1px solid #2a2a2a;
    color: #ccc;
    text-align: left;
    transition: all 0.15s ease;
    position: relative;
  }

  .file-item.selected {
    background: #2a3a2f;
  }

  .file-item:last-child {
    border-bottom: none;
  }

  .file-item:hover {
    background: #333;
  }

  .file-item.selected:hover {
    background: #354038;
  }

  .file-checkbox {
    width: 16px;
    height: 16px;
    cursor: pointer;
    accent-color: #3ddc84;
  }

  .file-btn {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 10px;
    background: transparent;
    border: none;
    color: inherit;
    cursor: pointer;
    text-align: left;
    padding: 0;
  }

  .file-item :global(.file-icon.folder) {
    color: #ffd54f;
  }

  .file-item :global(.file-icon.apk) {
    color: #3ddc84;
  }

  .file-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .file-size {
    font-size: 12px;
    color: #666;
    flex-shrink: 0;
  }

  /* ==================== 网格视图覆盖样式 ==================== */
  .file-list.grid-view {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
    gap: 8px;
    padding: 12px;
  }

  .file-list.grid-view .file-item {
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: 12px 8px;
    padding-top: 28px;
    border: 1px solid #333;
    border-radius: 8px;
    border-bottom: 1px solid #333;
  }

  .file-list.grid-view .file-item.selected {
    border-color: #3ddc84;
    background: #1a2a1f;
  }

  .file-list.grid-view .file-item:hover {
    border-color: #3ddc84;
    background: #252525;
  }

  .file-list.grid-view .file-item.selected:hover {
    background: #223328;
  }

  .file-list.grid-view .file-btn {
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: 6px;
  }

  .file-list.grid-view .file-name {
    font-size: 11px;
    word-break: break-all;
    white-space: normal;
    line-height: 1.3;
    max-height: 2.6em;
    overflow: hidden;
  }

  .file-list.grid-view .file-checkbox {
    position: absolute;
    top: 6px;
    left: 6px;
    width: 18px;
    height: 18px;
  }

  .file-list.grid-view .parent-dir {
    grid-column: 1 / -1;
    flex-direction: row;
    justify-content: flex-start;
    text-align: left;
    padding: 8px 12px;
    padding-top: 8px;
    border: none;
    border-bottom: 1px solid #333;
    border-radius: 0;
    background: #252525;
  }

  .file-list.grid-view .parent-dir .file-name {
    font-size: 13px;
    white-space: nowrap;
  }

  .file-list.grid-view .select-all-row {
    grid-column: 1 / -1;
    border: none;
    border-bottom: 1px solid #333;
    border-radius: 0;
    padding: 8px 12px;
    background: #252525;
    margin: -12px -12px 4px -12px;
    width: calc(100% + 24px);
  }

  /* 选择操作栏 */
  .selection-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    background: #1a1a1a;
    border-radius: 6px;
    margin-top: 8px;
    border: 1px solid #333;
    box-shadow: 0 -2px 8px rgba(0, 0, 0, 0.3);
  }

  .selection-count {
    font-size: 13px;
    color: #3ddc84;
    font-weight: 500;
  }

  .selection-actions {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .selection-actions :global(button) {
    font-weight: 500;
  }

  .selection-actions :global(button:first-child) {
    color: #999;
  }

  .selection-actions :global(button:first-child:hover) {
    color: #fff;
    background: #333;
  }

  .selection-actions :global(button:last-child) {
    background: #3ddc84;
    color: #000;
  }

  .selection-actions :global(button:last-child:hover) {
    background: #4ae88f;
  }

  .selection-actions :global(button:last-child:disabled) {
    background: #2a5a3a;
    color: #666;
  }

  .hidden {
    display: none;
  }
</style>
