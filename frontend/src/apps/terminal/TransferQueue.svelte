<script lang="ts">
  import { t } from "svelte-i18n";
  import { sshService, type TransferTask, type TransferProgress } from "$shared/services/ssh";
  import Icon from "@iconify/svelte";

  interface Props {
    sessionId?: string;
    onClose: () => void;
  }

  let { sessionId, onClose }: Props = $props();

  // 状态
  let tasks = $state<TransferTask[]>([]);
  let loading = $state(false);
  let ws = $state<WebSocket | null>(null);

  // 派生状态
  let activeTasks = $derived(tasks.filter(t => t.status === "running" || t.status === "pending"));
  let completedTasks = $derived(tasks.filter(t => t.status === "completed" || t.status === "failed" || t.status === "cancelled"));
  let hasCompletedTasks = $derived(completedTasks.length > 0);

  // 初始化
  $effect(() => {
    loadTasks();
    connectWebSocket();
    return () => {
      if (ws) {
        ws.close();
      }
    };
  });

  // 加载任务列表
  async function loadTasks() {
    loading = true;
    try {
      const response = await sshService.listTransfers(sessionId);
      if (response.success && response.data) {
        tasks = response.data;
      }
    } catch (e) {
      console.error("加载传输任务失败", e);
    } finally {
      loading = false;
    }
  }

  // 连接 WebSocket 接收进度更新
  function connectWebSocket() {
    const url = sshService.getTransferProgressUrl();
    ws = new WebSocket(url);

    ws.onmessage = (event) => {
      try {
        const progress: TransferProgress = JSON.parse(event.data);
        updateTaskProgress(progress);
      } catch (e) {
        console.error("解析进度消息失败", e);
      }
    };

    ws.onclose = () => {
      // 5秒后重连
      setTimeout(() => {
        if (ws?.readyState === WebSocket.CLOSED) {
          connectWebSocket();
        }
      }, 5000);
    };

    ws.onerror = (e) => {
      console.error("WebSocket 错误", e);
    };
  }

  // 更新任务进度
  function updateTaskProgress(progress: TransferProgress) {
    tasks = tasks.map(task => {
      if (task.id === progress.task_id) {
        return {
          ...task,
          status: progress.status as TransferTask["status"],
          progress: progress.progress,
          transferred: progress.transferred,
          total_size: progress.total_size,
          error: progress.error,
        };
      }
      return task;
    });
  }

  // 取消任务
  async function cancelTask(taskId: string) {
    try {
      await sshService.cancelTransfer(taskId);
      tasks = tasks.map(t => t.id === taskId ? { ...t, status: "cancelled" as const } : t);
    } catch (e) {
      console.error("取消任务失败", e);
    }
  }

  // 清理已完成任务
  async function clearCompleted() {
    try {
      await sshService.clearCompletedTransfers();
      tasks = tasks.filter(t => t.status === "running" || t.status === "pending");
    } catch (e) {
      console.error("清理任务失败", e);
    }
  }

  // 格式化文件大小
  function formatSize(bytes: number): string {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  }

  // 获取任务图标
  function getTaskIcon(task: TransferTask): string {
    if (task.type === "upload") {
      return "mdi:upload";
    }
    return "mdi:download";
  }

  // 获取状态图标
  function getStatusIcon(status: string): string {
    switch (status) {
      case "pending": return "mdi:clock-outline";
      case "running": return "mdi:loading";
      case "completed": return "mdi:check-circle";
      case "failed": return "mdi:alert-circle";
      case "cancelled": return "mdi:cancel";
      default: return "mdi:help-circle";
    }
  }

  // 获取状态颜色
  function getStatusColor(status: string): string {
    switch (status) {
      case "pending": return "text-gray-400";
      case "running": return "text-blue-400";
      case "completed": return "text-green-400";
      case "failed": return "text-red-400";
      case "cancelled": return "text-yellow-400";
      default: return "text-gray-400";
    }
  }

  // 获取文件名
  function getFileName(path: string): string {
    return path.split("/").pop() || path;
  }
</script>

<div class="transfer-queue">
  <div class="header">
    <div class="title">
      <Icon icon="mdi:swap-vertical" class="icon" />
      <span>{$t("terminal.transfer.queue")}</span>
      {#if activeTasks.length > 0}
        <span class="badge">{activeTasks.length}</span>
      {/if}
    </div>
    <div class="actions">
      {#if hasCompletedTasks}
        <button class="clear-btn" onclick={clearCompleted} title={$t("terminal.transfer.clearCompleted")}>
          <Icon icon="mdi:broom" />
        </button>
      {/if}
      <button class="close-btn" onclick={onClose} title={$t("terminal.transfer.close")}>
        <Icon icon="mdi:close" />
      </button>
    </div>
  </div>

  <div class="content">
    {#if loading}
      <div class="loading">
        <Icon icon="mdi:loading" class="animate-spin" />
        <span>{$t("terminal.transfer.loading")}</span>
      </div>
    {:else if tasks.length === 0}
      <div class="empty">
        <Icon icon="mdi:tray-arrow-down" class="empty-icon" />
        <span>{$t("terminal.transfer.noTasks")}</span>
      </div>
    {:else}
      <div class="task-list">
        {#each tasks as task (task.id)}
          <div class="task-item" class:completed={task.status === "completed"} class:failed={task.status === "failed"}>
            <div class="task-icon {getStatusColor(task.status)}">
              <Icon icon={getTaskIcon(task)} />
            </div>
            <div class="task-info">
              <div class="task-name" title={task.remote_path}>
                {getFileName(task.remote_path)}
              </div>
              <div class="task-path">
                {task.type === "upload" ? task.local_path : task.remote_path}
                <Icon icon="mdi:arrow-right" class="arrow" />
                {task.type === "upload" ? task.remote_path : task.local_path}
              </div>
              {#if task.status === "running" || task.status === "pending"}
                <div class="progress-bar">
                  <div class="progress-fill" style="width: {task.progress}%"></div>
                </div>
                <div class="progress-text">
                  {formatSize(task.transferred)} / {formatSize(task.total_size)} ({task.progress.toFixed(1)}%)
                </div>
              {:else if task.status === "completed"}
                <div class="status-text completed">
                  <Icon icon="mdi:check" /> {$t("terminal.transfer.completed")} - {formatSize(task.total_size)}
                </div>
              {:else if task.status === "failed"}
                <div class="status-text failed">
                  <Icon icon="mdi:alert" /> {task.error || $t("terminal.transfer.failed")}
                </div>
              {:else if task.status === "cancelled"}
                <div class="status-text cancelled">
                  <Icon icon="mdi:cancel" /> {$t("terminal.transfer.cancelled")}
                </div>
              {/if}
            </div>
            <div class="task-actions">
              {#if task.status === "running" || task.status === "pending"}
                <button class="cancel-btn" onclick={() => cancelTask(task.id)} title={$t("terminal.transfer.cancel")}>
                  <Icon icon="mdi:close" />
                </button>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .transfer-queue {
    display: flex;
    flex-direction: column;
    width: 400px;
    max-height: 500px;
    background: #1e1e2e;
    border: 1px solid #313244;
    border-radius: 8px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
    overflow: hidden;
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    background: #181825;
    border-bottom: 1px solid #313244;
  }

  .title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 14px;
    font-weight: 500;
    color: #cdd6f4;
  }

  .title :global(.icon) {
    font-size: 18px;
    color: #89b4fa;
  }

  .badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 20px;
    height: 20px;
    padding: 0 6px;
    font-size: 11px;
    font-weight: 600;
    color: #1e1e2e;
    background: #89b4fa;
    border-radius: 10px;
  }

  .actions {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .clear-btn,
  .close-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: #a6adc8;
    cursor: pointer;
    transition: all 0.15s;
  }

  .clear-btn:hover {
    background: #313244;
    color: #f9e2af;
  }

  .close-btn:hover {
    background: #313244;
    color: #f38ba8;
  }

  .content {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
  }

  .loading,
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 40px 20px;
    color: #6c7086;
  }

  .empty-icon {
    font-size: 48px;
    opacity: 0.5;
  }

  .task-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .task-item {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 12px;
    background: #181825;
    border: 1px solid #313244;
    border-radius: 6px;
    transition: all 0.15s;
  }

  .task-item:hover {
    border-color: #45475a;
  }

  .task-item.completed {
    opacity: 0.7;
  }

  .task-item.failed {
    border-color: #f38ba8;
    background: rgba(243, 139, 168, 0.05);
  }

  .task-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    font-size: 18px;
    flex-shrink: 0;
  }

  .task-info {
    flex: 1;
    min-width: 0;
  }

  .task-name {
    font-size: 13px;
    font-weight: 500;
    color: #cdd6f4;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .task-path {
    display: flex;
    align-items: center;
    gap: 4px;
    margin-top: 2px;
    font-size: 11px;
    color: #6c7086;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .task-path :global(.arrow) {
    font-size: 10px;
    flex-shrink: 0;
  }

  .progress-bar {
    margin-top: 8px;
    height: 4px;
    background: #313244;
    border-radius: 2px;
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: linear-gradient(90deg, #89b4fa, #74c7ec);
    border-radius: 2px;
    transition: width 0.3s ease;
  }

  .progress-text {
    margin-top: 4px;
    font-size: 11px;
    color: #a6adc8;
  }

  .status-text {
    display: flex;
    align-items: center;
    gap: 4px;
    margin-top: 6px;
    font-size: 11px;
  }

  .status-text.completed {
    color: #a6e3a1;
  }

  .status-text.failed {
    color: #f38ba8;
  }

  .status-text.cancelled {
    color: #f9e2af;
  }

  .task-actions {
    display: flex;
    align-items: center;
    flex-shrink: 0;
  }

  .cancel-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: #a6adc8;
    cursor: pointer;
    transition: all 0.15s;
  }

  .cancel-btn:hover {
    background: #f38ba8;
    color: #1e1e2e;
  }

  /* 滚动条样式 */
  .content::-webkit-scrollbar {
    width: 6px;
  }

  .content::-webkit-scrollbar-track {
    background: transparent;
  }

  .content::-webkit-scrollbar-thumb {
    background: #45475a;
    border-radius: 3px;
  }

  .content::-webkit-scrollbar-thumb:hover {
    background: #585b70;
  }

  /* 动画 */
  :global(.animate-spin) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
