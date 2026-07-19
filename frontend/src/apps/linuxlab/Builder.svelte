<script lang="ts">
  import { onDestroy, onMount, tick } from "svelte";
  import Icon from "@iconify/svelte";
  import { Button, Spinner } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import * as linuxLabService from "./service";
  import type { BuildTarget, ProgressEvent } from "./types";

  const MAX_LOG_LINES = 2000;

  let { board }: { board: string } = $props();

  type Target = { label: string; value: string; icon: string; desc: string };
  const targets: Target[] = [
    { label: "kernel", value: "kernel", icon: "mdi:linux", desc: "编译 Linux 内核" },
    { label: "modules", value: "modules", icon: "mdi:puzzle-outline", desc: "编译内核模块" },
    { label: "uboot", value: "uboot", icon: "mdi:rocket-launch-outline", desc: "编译 U-Boot" },
    { label: "root", value: "root", icon: "mdi:folder-open-outline", desc: "构建根文件系统" },
  ];

  let selectedTarget = $state("kernel");
  let building = $state(false);
  let logs = $state<string[]>([]);
  let buildFailed = $state(false);
  let buildDone = $state(false);
  let detachedHint = $state(false); // 刷新后重连
  let logsAvailable = $state(false); // 是否已挂上 job 日志流
  let lastSeq = $state(0);
  let controller: AbortController | null = null;
  let logContainer: HTMLElement | undefined = $state();
  let statusPoll: ReturnType<typeof setInterval> | null = null;

  onMount(() => {
    void resumeIfBuilding();
  });

  onDestroy(() => {
    controller?.abort();
    if (statusPoll) clearInterval(statusPoll);
  });

  function appendLog(line: string) {
    logs = [...logs, line];
    if (logs.length > MAX_LOG_LINES) logs = logs.slice(-MAX_LOG_LINES);
    tick().then(() => {
      if (logContainer) logContainer.scrollTop = logContainer.scrollHeight;
    });
  }

  function handleProgress(ev: ProgressEvent) {
    if (ev.seq && ev.seq > lastSeq) lastSeq = ev.seq;
    if (ev.message) appendLog(ev.message);
    if (ev.line) appendLog(ev.line);
    if (ev.status === "failed") buildFailed = true;
  }

  function handleBuildDone() {
    building = false;
    buildDone = true;
    detachedHint = false;
    logsAvailable = false;
    if (statusPoll) {
      clearInterval(statusPoll);
      statusPoll = null;
    }
    if (!buildFailed) showToast("构建完成", "success");
  }

  function pollUntilIdle() {
    if (statusPoll) clearInterval(statusPoll);
    statusPoll = setInterval(async () => {
      try {
        const cur = await linuxLabService.getBuildStatus();
        if (!cur.building) {
          handleBuildDone();
          appendLog("✓ 后台构建已结束（服务重启后无法恢复详细日志）");
        }
      } catch {
        /* ignore */
      }
    }, 5000);
  }

  async function resumeIfBuilding() {
    try {
      const st = await linuxLabService.getBuildStatus();
      if (!st.building && !st.job_id) return;

      if (st.target) selectedTarget = st.target;
      building = !!st.building;
      buildDone = !st.building && st.status === "completed";
      buildFailed = st.status === "failed";
      detachedHint = !!st.building;
      lastSeq = 0;

      // 有 job：可恢复环形缓冲日志
      if (st.job_id) {
        logsAvailable = true;
        logs = [
          st.building
            ? "检测到后台构建，正在恢复日志…"
            : "正在加载最近一次构建日志…",
          st.board || st.target
            ? `目标: ${st.board || board || "?"} / ${st.target || selectedTarget}`
            : "",
        ].filter(Boolean);

        controller?.abort();
        controller = linuxLabService.streamBuildLogs(
          0,
          handleProgress,
          () => {
            if (st.building || building) {
              handleBuildDone();
            } else {
              detachedHint = false;
              logsAvailable = false;
              building = false;
              if (buildFailed || st.status === "completed") buildDone = true;
            }
          },
        );
        return;
      }

      // 仅检测到容器内 make（例如后端刚重启，内存任务已丢）
      logsAvailable = false;
      logs = [
        "检测到容器内仍有构建进程在运行。",
        "服务重启后历史日志无法恢复；构建会继续，结束后状态会自动更新。",
        "请勿重复点击「开始构建」。",
      ];
      pollUntilIdle();
    } catch {
      /* ignore */
    }
  }

  function startBuild() {
    if (!board) {
      showToast("请先选择一个开发板", "error");
      return;
    }
    if (building) {
      showToast("已有构建任务在进行中", "info");
      return;
    }
    building = true;
    buildFailed = false;
    buildDone = false;
    detachedHint = false;
    logsAvailable = true;
    lastSeq = 0;
    logs = [];
    if (statusPoll) {
      clearInterval(statusPoll);
      statusPoll = null;
    }

    controller?.abort();
    controller = linuxLabService.startBuild(
      board,
      selectedTarget as BuildTarget,
      handleProgress,
      handleBuildDone,
    );
  }
</script>

<div class="builder">
  {#if !board}
    <div class="empty-state">
      <Icon icon="mdi:alert-circle-outline" width={40} />
      <span>请先在"开发板"标签中选择开发板</span>
    </div>
  {:else}
    <div class="target-section">
      <h4>选择构建目标</h4>
      <div class="target-grid">
        {#each targets as t}
          <button
            class="target-card"
            class:active={selectedTarget === t.value}
            onclick={() => (selectedTarget = t.value)}
            disabled={building}
          >
            <Icon icon={t.icon} width={20} />
            <span class="target-label">{t.label}</span>
            <span class="target-desc">{t.desc}</span>
          </button>
        {/each}
      </div>
      <div class="build-bar">
        <div class="build-info mono">
          Board: {board} / Target: {selectedTarget}
        </div>
        <Button variant="primary" size="sm" onclick={startBuild} disabled={building}>
          {#if building}
            <Spinner />
            构建中...
          {:else}
            <Icon icon="mdi:hammer-wrench" width={16} />
            开始构建
          {/if}
        </Button>
      </div>
    </div>

    {#if logs.length > 0}
      <div
        class="status-strip"
        class:ok={buildDone && !buildFailed}
        class:fail={buildFailed}
        class:detached={detachedHint && building}
      >
        <Icon icon={buildFailed ? "mdi:close-circle" : buildDone ? "mdi:check-circle" : detachedHint ? "mdi:cloud-sync" : "mdi:progress-wrench"} width={16} />
        {#if buildFailed}
          构建失败
        {:else if buildDone}
          构建完成
        {:else if detachedHint}
          {logsAvailable ? "已重连后台构建日志" : "后台构建中（日志缓冲不可用）"}
        {:else}
          正在构建...
        {/if}
      </div>
      <div class="log-box" bind:this={logContainer}>
        {#each logs as line}
          <div class="log-line">{line}</div>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  .builder {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    flex: 1;
    overflow: hidden;
    padding: 16px;
    gap: 12px;
    box-sizing: border-box;
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    height: 100%;
    color: var(--text-muted);
    font-size: 14px;
  }

  .target-section {
    display: flex;
    flex-direction: column;
    gap: 10px;
    flex-shrink: 0;
  }

  .target-section h4 {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .target-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 8px;
  }

  .target-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    padding: 14px 10px;
    border-radius: 10px;
    border: 1px solid var(--border-color);
    background: var(--bg-card);
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s;
  }

  .target-card:hover:not(:disabled) {
    border-color: var(--color-primary);
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .target-card.active {
    border-color: var(--color-primary);
    background: var(--bg-active);
    color: var(--color-primary);
  }

  .target-card:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .target-label {
    font-size: 13px;
    font-weight: 600;
    font-family: "JetBrains Mono", "Fira Code", monospace;
  }

  .target-desc {
    font-size: 11px;
    color: var(--text-muted);
  }

  .build-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    border-radius: 8px;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
  }

  .build-info {
    font-size: 12px;
    color: var(--text-muted);
    font-family: "JetBrains Mono", "Fira Code", monospace;
  }

  .status-strip {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 14px;
    border-radius: 8px;
    font-size: 13px;
    font-weight: 500;
    background: var(--bg-secondary);
    color: var(--text-secondary);
    flex-shrink: 0;
  }

  .status-strip.ok {
    background: rgba(40, 167, 69, 0.1);
    color: var(--color-success);
  }

  .status-strip.fail {
    background: rgba(220, 53, 69, 0.1);
    color: var(--color-danger);
  }

  .status-strip.detached {
    background: rgba(59, 130, 246, 0.1);
    color: #2563eb;
  }

  .log-box {
    flex: 1 1 auto;
    min-height: 120px;
    overflow-x: hidden;
    overflow-y: auto;
    overscroll-behavior: contain;
    scrollbar-gutter: stable;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    padding: 10px 12px;
    font-family: "JetBrains Mono", "Fira Code", monospace;
    font-size: 11px;
  }

  .log-line {
    line-height: 1.5;
    white-space: pre-wrap;
    word-break: break-all;
    color: var(--text-secondary);
  }
</style>
