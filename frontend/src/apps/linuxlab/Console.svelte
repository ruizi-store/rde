<script lang="ts">
  import { onDestroy, tick } from "svelte";
  import Icon from "@iconify/svelte";
  import { Button, Spinner } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import * as linuxLabService from "./service";

  const MAX_LOG_LINES = 2000;

  let { board, running }: { board: string; running: boolean } = $props();

  let booting = $state(false);
  let logs = $state<string[]>([]);
  let bootFailed = $state(false);
  let controller: AbortController | null = null;
  let logContainer: HTMLElement | undefined = $state();

  onDestroy(() => controller?.abort());

  function startBoot() {
    if (!board) {
      showToast("请先选择一个开发板", "error");
      return;
    }
    booting = true;
    bootFailed = false;
    logs = [`>>> 启动 QEMU  (board: ${board})`];

    controller = linuxLabService.startBoot(
      board,
      (ev) => {
        if (ev.line) {
          logs = [...logs, ev.line];
          if (logs.length > MAX_LOG_LINES) logs = logs.slice(-MAX_LOG_LINES);
          tick().then(() => {
            if (logContainer) logContainer.scrollTop = logContainer.scrollHeight;
          });
        }
        if (ev.status === "failed") bootFailed = true;
      },
      () => {
        booting = false;
        if (!bootFailed) showToast("QEMU 已退出", "info");
      },
    );
  }

  async function stopBoot() {
    try {
      await linuxLabService.stopBoot();
      showToast("已发送停止信号", "info");
    } catch (e: any) {
      showToast("停止失败: " + (e.message || e), "error");
    }
  }
</script>

<div class="console">
  {#if !board}
    <div class="empty-state">
      <Icon icon="mdi:alert-circle-outline" width={40} />
      <span>请先在"开发板"标签中选择开发板</span>
    </div>
  {:else}
    <div class="console-bar">
      <div class="status-dot" class:active={running || booting}>
        <span class="dot"></span>
        <span class="status-text">{booting ? "启动中..." : running ? "运行中" : "已停止"}</span>
      </div>
      <span class="board-label mono">{board}</span>
      <div class="console-actions">
        {#if booting}
          <Button variant="danger" size="sm" onclick={stopBoot}>
            <Icon icon="mdi:stop" width={16} />
            停止
          </Button>
        {:else}
          <Button variant="primary" size="sm" onclick={startBoot}>
            <Icon icon="mdi:play" width={16} />
            启动 QEMU
          </Button>
        {/if}
      </div>
    </div>

    <div class="terminal-area" bind:this={logContainer}>
      {#if logs.length > 0}
        {#each logs as line}
          <div class="term-line">{line}</div>
        {/each}
      {:else}
        <div class="term-placeholder">
          <Icon icon="mdi:console" width={28} />
          <span>点击"启动 QEMU"开始</span>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .console {
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

  .console-bar {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    border-radius: 10px;
    background: var(--bg-card);
    border: 1px solid var(--border-color);
  }

  .status-dot {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--text-muted);
    transition: all 0.2s;
  }

  .status-dot.active .dot {
    background: var(--color-success);
    box-shadow: 0 0 6px rgba(40, 167, 69, 0.5);
  }

  .status-text {
    font-size: 13px;
    font-weight: 500;
    color: var(--text-primary);
  }

  .board-label {
    font-size: 12px;
    color: var(--text-muted);
    font-family: "JetBrains Mono", "Fira Code", monospace;
  }

  .console-actions {
    margin-left: auto;
    display: flex;
    gap: 8px;
  }

  .terminal-area {
    flex: 1;
    overflow-y: auto;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    border-radius: 10px;
    padding: 12px 14px;
    font-family: "JetBrains Mono", "Fira Code", monospace;
    font-size: 12px;
  }

  .term-line {
    line-height: 1.6;
    white-space: pre-wrap;
    word-break: break-all;
    color: var(--text-secondary);
  }

  .term-placeholder {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 8px;
    color: var(--text-muted);
    font-family: inherit;
    font-size: 13px;
  }

  .mono {
    font-family: "JetBrains Mono", "Fira Code", monospace;
  }
</style>
