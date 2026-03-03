<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import Icon from "@iconify/svelte";
  import { Spinner, Tabs, Button } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import * as linuxLabService from "./service";
  import type { LabStatus, Board } from "./types";
  import BoardSelector from "./BoardSelector.svelte";
  import Builder from "./Builder.svelte";
  import Console from "./Console.svelte";

  // ==================== 状态 ====================

  let loading = $state(true);
  let status = $state<LabStatus | null>(null);
  let boards = $state<Board[]>([]);
  let activeTab = $state("dashboard");
  let selectedBoard = $state<string>("");

  // 安装状态
  let isSettingUp = $state(false);
  let setupLogs = $state<string[]>([]);
  let setupFailed = $state(false);
  let setupController: AbortController | null = null;

  let tabs = [
    { id: "dashboard", label: "仪表盘" },
    { id: "boards", label: "开发板" },
    { id: "build", label: "构建" },
    { id: "console", label: "控制台" },
  ];

  // ==================== 生命周期 ====================

  onMount(async () => {
    await refresh();
    // 如果 Docker 可用但容器未运行，自动开始 setup
    if (status && status.docker_ok && !status.container_running && !isSettingUp) {
      startSetup();
    }
  });

  onDestroy(() => {
    setupController?.abort();
  });

  async function refresh() {
    loading = true;
    try {
      status = await linuxLabService.getStatus();
      if (status.container_running) {
        boards = await linuxLabService.listBoards();
        if (status.current_board) {
          selectedBoard = status.current_board;
        } else if (boards.length > 0) {
          selectedBoard = boards[0].full_path;
        }
      }
    } catch (e) {
      console.error("加载 Linux Lab 状态失败:", e);
    } finally {
      loading = false;
    }
  }

  function startSetup() {
    isSettingUp = true;
    setupFailed = false;
    setupLogs = ["正在初始化 Linux Lab 容器环境..."];

    setupController = linuxLabService.setup(
      (event) => {
        if (event.message) setupLogs = [...setupLogs, event.message];
        if (event.line) setupLogs = [...setupLogs, event.line];
        if (event.status === "failed") setupFailed = true;
      },
      async () => {
        if (!setupFailed) {
          setupLogs = [...setupLogs, "✓ 容器环境就绪，正在刷新..."];
          await refresh();
        }
        isSettingUp = false;
      },
    );
  }

  async function handleSwitchBoard(boardPath: string) {
    try {
      await linuxLabService.switchBoard(boardPath);
      selectedBoard = boardPath;
      showToast(`已切换到 ${boardPath}`, "success");
      status = await linuxLabService.getStatus();
    } catch (e: any) {
      showToast(`切换失败: ${e.message || e}`, "error");
    }
  }

  function goToBoards() { activeTab = "boards"; }
  function goToBuild() { activeTab = "build"; }
  function goToConsole() { activeTab = "console"; }

  let currentBoard = $derived(boards.find((b) => b.full_path === selectedBoard));

  let envReady = $derived(status?.container_running ?? false);
</script>

<div class="linuxlab">
  {#if loading}
    <div class="center-state">
      <Spinner center />
    </div>
  {:else if !status?.docker_ok}
    <!-- Docker 不可用 -->
    <div class="setup-screen">
      <div class="setup-card">
        <div class="setup-header">
          <div class="setup-icon error">
            <Icon icon="mdi:docker" width={40} />
          </div>
          <h2>Docker 不可用</h2>
          <p class="setup-desc">Linux Lab 需要 Docker 来运行。请确保已安装并启动 Docker。</p>
        </div>
        <div class="hint-box">
          <code>sudo apt install docker.io && sudo systemctl start docker</code>
        </div>
        <Button variant="primary" size="sm" onclick={refresh}>
          <Icon icon="mdi:refresh" width={16} />
          重新检测
        </Button>
      </div>
    </div>
  {:else if !envReady}
    <!-- 需要拉取镜像 / 创建容器 -->
    <div class="setup-screen">
      <div class="setup-card">
        <div class="setup-header">
          <div class="setup-icon">
            <Icon icon="mdi:linux" width={40} />
          </div>
          <h2>Linux Lab</h2>
          <p class="setup-desc">嵌入式 Linux 内核开发实验环境 (Docker 容器模式)</p>
        </div>

        {#if isSettingUp}
          <div class="setup-progress">
            <div class="progress-header">
              <Spinner />
              <span>正在初始化容器环境...</span>
            </div>
            <div class="setup-log-box">
              {#each setupLogs as log}
                <div class="slog-line">{log}</div>
              {/each}
            </div>
          </div>
        {:else if setupFailed}
          <div class="setup-progress">
            <div class="progress-header failed">
              <Icon icon="mdi:alert-circle" width={20} />
              <span>初始化失败</span>
            </div>
            <div class="setup-log-box error">
              {#each setupLogs as log}
                <div class="slog-line">{log}</div>
              {/each}
            </div>
            <Button variant="primary" size="sm" onclick={startSetup}>
              <Icon icon="mdi:refresh" width={16} />
              重试
            </Button>
          </div>
        {:else}
          <div class="setup-info">
            <div class="check-item" class:ok={status!.image_ready}>
              <Icon icon={status!.image_ready ? "mdi:check-circle" : "mdi:circle-outline"} width={18} />
              <span>Docker 镜像 ({status!.image})</span>
            </div>
            <div class="check-item" class:ok={status!.container_exists}>
              <Icon icon={status!.container_exists ? "mdi:check-circle" : "mdi:circle-outline"} width={18} />
              <span>容器已创建</span>
            </div>
            <div class="check-item" class:ok={status!.container_running}>
              <Icon icon={status!.container_running ? "mdi:check-circle" : "mdi:circle-outline"} width={18} />
              <span>容器运行中</span>
            </div>
          </div>
          <Button variant="primary" onclick={startSetup}>
            <Icon icon="mdi:docker" width={18} />
            开始初始化
          </Button>
        {/if}
      </div>
    </div>
  {:else}
    <!-- 容器已运行，正常界面 -->
    <div class="content">
      <Tabs {tabs} bind:activeTab variant="underline" size="sm">
        {#snippet children(tab)}
          {#if tab === "dashboard"}
            <div class="dashboard">
              <div class="stat-row">
                <div class="stat-card">
                  <div class="stat-icon si-blue">
                    <Icon icon="mdi:docker" width={22} />
                  </div>
                  <div class="stat-body">
                    <span class="stat-label">容器</span>
                    <span class="stat-value text-ok">运行中</span>
                  </div>
                </div>

                <div class="stat-card">
                  <div class="stat-icon si-green">
                    <Icon icon="mdi:developer-board" width={22} />
                  </div>
                  <div class="stat-body">
                    <span class="stat-label">当前开发板</span>
                    <span class="stat-value mono">{selectedBoard || "未选择"}</span>
                  </div>
                </div>

                <div class="stat-card">
                  <div class="stat-icon" class:si-green={status!.booting} class:si-muted={!status!.booting}>
                    <Icon icon={status!.booting ? "mdi:play-circle" : "mdi:stop-circle"} width={22} />
                  </div>
                  <div class="stat-body">
                    <span class="stat-label">QEMU</span>
                    <span class="stat-value" class:text-ok={status!.booting}>{status!.booting ? "运行中" : "已停止"}</span>
                  </div>
                </div>

                <div class="stat-card">
                  <div class="stat-icon si-purple">
                    <Icon icon="mdi:view-grid-outline" width={22} />
                  </div>
                  <div class="stat-body">
                    <span class="stat-label">开发板</span>
                    <span class="stat-value">{boards.length} 块</span>
                  </div>
                </div>
              </div>

              <div class="panel">
                <div class="panel-header">
                  <h3>
                    <Icon icon="mdi:docker" width={18} />
                    容器信息
                  </h3>
                </div>
                <div class="dep-chips">
                  <div class="dep-chip ok">
                    <Icon icon="mdi:check-circle" width={16} />
                    Docker
                  </div>
                  <div class="dep-chip ok">
                    <Icon icon="mdi:check-circle" width={16} />
                    镜像 {status!.image}
                  </div>
                  <div class="dep-chip ok">
                    <Icon icon="mdi:check-circle" width={16} />
                    容器运行中
                  </div>
                </div>
                <div class="dep-hint">
                  <Icon icon="mdi:information-outline" width={14} />
                  所有工具链、QEMU、编译环境均已包含在容器内，无需手动安装
                </div>
              </div>

              {#if currentBoard}
                <div class="panel">
                  <div class="panel-header">
                    <h3>
                      <Icon icon="mdi:developer-board" width={18} />
                      当前板配置
                      <span class="badge mono">{currentBoard.full_path}</span>
                    </h3>
                  </div>
                  <div class="config-grid">
                    {#each [
                      { label: "架构", value: currentBoard.arch, icon: "mdi:chip" },
                      { label: "CPU", value: currentBoard.cpu, icon: "mdi:cpu-64-bit" },
                      { label: "内存", value: currentBoard.mem, icon: "mdi:memory" },
                      { label: "SMP", value: String(currentBoard.smp || 1), icon: "mdi:view-grid" },
                      { label: "Linux", value: currentBoard.linux, icon: "mdi:linux" },
                      { label: "QEMU", value: currentBoard.qemu, icon: "mdi:monitor" },
                      { label: "U-Boot", value: currentBoard.uboot, icon: "mdi:rocket-launch-outline" },
                      { label: "串口", value: currentBoard.serial, icon: "mdi:serial-port" },
                    ] as item}
                      <div class="config-item">
                        <Icon icon={item.icon} width={14} class="config-icon" />
                        <span class="config-label">{item.label}</span>
                        <span class="config-val mono">{item.value || "-"}</span>
                      </div>
                    {/each}
                  </div>
                </div>
              {/if}

              <div class="panel">
                <div class="panel-header">
                  <h3>
                    <Icon icon="mdi:lightning-bolt" width={18} />
                    快速操作
                  </h3>
                </div>
                <div class="action-row">
                  <button class="action-card" onclick={goToBoards}>
                    <Icon icon="mdi:developer-board" width={24} />
                    <span>选择开发板</span>
                  </button>
                  <button class="action-card primary" onclick={goToBuild} disabled={!selectedBoard}>
                    <Icon icon="mdi:hammer-wrench" width={24} />
                    <span>构建内核</span>
                  </button>
                  <button class="action-card" onclick={goToConsole} disabled={!selectedBoard}>
                    <Icon icon="mdi:play-circle-outline" width={24} />
                    <span>启动开发板</span>
                  </button>
                  <button class="action-card" onclick={refresh}>
                    <Icon icon="mdi:refresh" width={24} />
                    <span>刷新状态</span>
                  </button>
                </div>
              </div>
            </div>

          {:else if tab === "boards"}
            <BoardSelector {boards} {selectedBoard} onSelect={handleSwitchBoard} />

          {:else if tab === "build"}
            <Builder board={selectedBoard} />

          {:else if tab === "console"}
            <Console board={selectedBoard} running={status?.booting ?? false} />
          {/if}
        {/snippet}
      </Tabs>
    </div>
  {/if}
</div>

<style>
  .linuxlab {
    height: 100%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    color: var(--text-primary);
  }

  .center-state {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
  }

  .content {
    height: 100%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  /* ===== Setup 屏幕 ===== */
  .setup-screen {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    padding: 24px;
  }

  .setup-card {
    width: 100%;
    max-width: 560px;
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: 16px;
    padding: 32px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 24px;
  }

  .setup-header {
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
  }

  .setup-icon {
    width: 64px;
    height: 64px;
    border-radius: 16px;
    background: var(--color-primary-light);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--color-primary);
  }

  .setup-icon.error {
    background: rgba(220, 53, 69, 0.1);
    color: var(--color-danger);
  }

  .setup-header h2 {
    margin: 0;
    font-size: 20px;
    font-weight: 700;
    color: var(--text-primary);
  }

  .setup-desc {
    margin: 0;
    font-size: 13px;
    color: var(--text-muted);
  }

  .hint-box {
    width: 100%;
    padding: 12px 16px;
    border-radius: 8px;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    font-family: "JetBrains Mono", "Fira Code", monospace;
    font-size: 12px;
    color: var(--text-secondary);
  }

  .setup-info {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .check-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    border-radius: 8px;
    background: var(--bg-secondary);
    font-size: 13px;
    color: var(--text-muted);
  }

  .check-item.ok {
    color: var(--color-success);
  }

  .setup-progress {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 12px;
    align-items: center;
  }

  .progress-header {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 14px;
    color: var(--text-secondary);
  }

  .progress-header.failed {
    color: var(--color-danger);
  }

  .setup-log-box {
    width: 100%;
    max-height: 300px;
    overflow-y: auto;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    padding: 10px 12px;
    font-family: "JetBrains Mono", "Fira Code", monospace;
    font-size: 11px;
  }

  .setup-log-box.error {
    border-color: var(--color-danger);
  }

  .slog-line {
    color: var(--text-secondary);
    line-height: 1.5;
    white-space: pre-wrap;
    word-break: break-all;
  }

  /* ===== 仪表盘 ===== */
  .dashboard {
    padding: 20px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .stat-row {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 10px;
  }

  .stat-card {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 16px;
    border-radius: 12px;
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    transition: border-color 0.15s;
  }

  .stat-card:hover {
    border-color: var(--color-primary);
  }

  .stat-icon {
    width: 40px;
    height: 40px;
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .si-blue { background: var(--color-primary-light); color: var(--color-primary); }
  .si-green { background: rgba(40, 167, 69, 0.12); color: var(--color-success); }
  .si-purple { background: rgba(111, 66, 193, 0.12); color: #6f42c1; }
  .si-muted { background: var(--bg-tertiary); color: var(--text-muted); }

  .stat-body {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .stat-label {
    font-size: 11px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .stat-value {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .stat-value.mono, .mono {
    font-family: "JetBrains Mono", "Fira Code", monospace;
    font-size: 13px;
  }

  .text-ok {
    color: var(--color-success) !important;
  }

  /* ===== 面板 ===== */
  .panel {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    padding: 16px 18px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .panel-header h3 {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .badge {
    font-size: 11px;
    font-weight: 400;
    padding: 2px 8px;
    border-radius: 6px;
    background: var(--color-primary-light);
    color: var(--color-primary);
  }

  .dep-chips {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
  }

  .dep-chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 14px;
    border-radius: 8px;
    font-size: 13px;
    font-weight: 500;
    background: var(--bg-secondary);
    color: var(--text-muted);
    border: 1px solid var(--border-color);
  }

  .dep-chip.ok {
    color: var(--color-success);
  }

  .dep-hint {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--text-muted);
    padding: 6px 0;
  }

  .config-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(170px, 1fr));
    gap: 6px;
  }

  .config-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    border-radius: 8px;
    background: var(--bg-secondary);
  }

  :global(.config-icon) {
    color: var(--text-muted);
    flex-shrink: 0;
  }

  .config-label {
    font-size: 11px;
    color: var(--text-muted);
    min-width: 36px;
  }

  .config-val {
    font-size: 13px;
    color: var(--text-primary);
    margin-left: auto;
  }

  .action-row {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 8px;
  }

  .action-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 18px 12px;
    border-radius: 10px;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    color: var(--text-secondary);
    font-size: 13px;
    cursor: pointer;
    transition: all 0.15s;
  }

  .action-card:hover:not(:disabled) {
    border-color: var(--color-primary);
    color: var(--text-primary);
    background: var(--bg-hover);
  }

  .action-card:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .action-card.primary {
    background: var(--color-primary-light);
    border-color: var(--color-primary);
    color: var(--color-primary);
  }

  .action-card.primary:hover:not(:disabled) {
    background: var(--bg-active);
  }
</style>
