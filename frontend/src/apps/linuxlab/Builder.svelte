<script lang="ts">
  import { onDestroy, tick } from "svelte";
  import Icon from "@iconify/svelte";
  import { Button, Spinner } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import * as linuxLabService from "./service";
  import type { BuildTarget } from "./types";

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
  let controller: AbortController | null = null;
  let logContainer: HTMLElement | undefined = $state();

  onDestroy(() => controller?.abort());

  function startBuild() {
    if (!board) {
      showToast("请先选择一个开发板", "error");
      return;
    }
    building = true;
    buildFailed = false;
    buildDone = false;
    logs = [`>>> make ${selectedTarget}  (board: ${board})`];

    controller = linuxLabService.startBuild(
      board,
      selectedTarget as BuildTarget,
      (ev) => {
        if (ev.line) {
          logs = [...logs, ev.line];
          if (logs.length > MAX_LOG_LINES) logs = logs.slice(-MAX_LOG_LINES);
          tick().then(() => {
            if (logContainer) logContainer.scrollTop = logContainer.scrollHeight;
          });
        }
        if (ev.status === "failed") buildFailed = true;
      },
      () => {
        building = false;
        buildDone = true;
        if (!buildFailed) showToast("构建完成", "success");
      },
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
      <div class="status-strip" class:ok={buildDone && !buildFailed} class:fail={buildFailed}>
        <Icon icon={buildFailed ? "mdi:close-circle" : buildDone ? "mdi:check-circle" : "mdi:progress-wrench"} width={16} />
        {buildFailed ? "构建失败" : buildDone ? "构建完成" : "正在构建..."}
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
    overflow: hidden;
    padding: 16px;
    gap: 12px;
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
  }

  .status-strip.ok {
    background: rgba(40, 167, 69, 0.1);
    color: var(--color-success);
  }

  .status-strip.fail {
    background: rgba(220, 53, 69, 0.1);
    color: var(--color-danger);
  }

  .log-box {
    flex: 1;
    overflow-y: auto;
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
