<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { fly, fade, slide } from "svelte/transition";
  import Icon from "@iconify/svelte";
  import { t } from "$lib/i18n";
  import {
    taskService,
    categoryInfo,
    type BackgroundTask,
    type TaskStatus,
  } from "$shared/services/privilege";

  // ===================== 状态 =====================
  let tasks = $state<BackgroundTask[]>([]);
  let isOpen = $state(false);
  let ws: WebSocket | null = null;

  // 派生状态
  let runningTasks = $derived(tasks.filter((t) => t.status === "running"));
  let recentTasks = $derived(
    tasks
      .filter((t) => t.status !== "running")
      .sort((a, b) => {
        const timeA = a.finished_at ? new Date(a.finished_at).getTime() : 0;
        const timeB = b.finished_at ? new Date(b.finished_at).getTime() : 0;
        return timeB - timeA;
      })
      .slice(0, 5)
  );
  let hasRunningTasks = $derived(runningTasks.length > 0);

  // ===================== 辅助函数 =====================
  function getStatusIcon(status: TaskStatus): string {
    switch (status) {
      case "running":
        return "mdi:loading";
      case "completed":
        return "mdi:check-circle";
      case "failed":
        return "mdi:alert-circle";
      case "cancelled":
        return "mdi:cancel";
      default:
        return "mdi:clock-outline";
    }
  }

  function getStatusColor(status: TaskStatus): string {
    switch (status) {
      case "running":
        return "text-blue-500";
      case "completed":
        return "text-green-500";
      case "failed":
        return "text-red-500";
      case "cancelled":
        return "text-gray-500";
      default:
        return "text-gray-400";
    }
  }

  function formatTime(dateStr?: string): string {
    if (!dateStr) return "";
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);

    if (diffMins < 1) return $t('common.justNow');
    if (diffMins < 60) return $t('common.minutesAgo', { values: { n: diffMins } });
    const diffHours = Math.floor(diffMins / 60);
    if (diffHours < 24) return $t('common.hoursAgo', { values: { n: diffHours } });
    return date.toLocaleDateString();
  }

  async function removeTask(taskId: string) {
    await taskService.removeTask(taskId);
    tasks = tasks.filter((t) => t.id !== taskId);
  }

  async function cancelTask(taskId: string) {
    await taskService.cancelTask(taskId);
  }

  function connectWebSocket() {
    ws = taskService.connectTaskUpdates({
      onInit: (initialTasks) => {
        tasks = initialTasks;
      },
      onUpdate: (task) => {
        const index = tasks.findIndex((t) => t.id === task.id);
        if (index >= 0) {
          tasks = [...tasks.slice(0, index), task, ...tasks.slice(index + 1)];
        } else {
          tasks = [task, ...tasks];
        }

        // 任务完成时发送系统通知
        if (task.status === "completed" || task.status === "failed") {
          sendNotification(task);
        }
      },
      onError: (error) => {
        console.error("Task WebSocket error", error);
      },
      onClose: () => {
        // 5 秒后重连
        setTimeout(() => {
          if (!ws) {
            connectWebSocket();
          }
        }, 5000);
      },
    });
  }

  function sendNotification(task: BackgroundTask) {
    if (!("Notification" in window)) return;

    if (Notification.permission === "granted") {
      new Notification(task.status === "completed" ? $t('tasks.completed') : $t('tasks.failed'), {
        body: task.title,
        icon: task.status === "completed" ? "/icons/success.png" : "/icons/error.png",
        tag: task.id,
      });
    } else if (Notification.permission !== "denied") {
      Notification.requestPermission().then((permission) => {
        if (permission === "granted") {
          sendNotification(task);
        }
      });
    }
  }

  // ===================== 生命周期 =====================
  onMount(() => {
    connectWebSocket();
  });

  onDestroy(() => {
    ws?.close();
    ws = null;
  });
</script>

<!-- 托盘图标按钮 -->
<div class="relative">
  <button
    class="relative w-10 h-10 rounded-xl flex items-center justify-center
           transition-colors hover:bg-gray-700/50"
    class:bg-blue-500/20={hasRunningTasks}
    onclick={() => (isOpen = !isOpen)}
    title={$t('tasks.title')}
  >
    <Icon
      icon={hasRunningTasks ? "mdi:progress-clock" : "mdi:tray-full"}
      class="text-xl transition-colors"
      class:text-blue-500={hasRunningTasks}
      class:text-gray-400={!hasRunningTasks}
      class:animate-pulse={hasRunningTasks}
    />
    
    <!-- 运行中任务数量徽章 -->
    {#if runningTasks.length > 0}
      <div
        class="absolute -top-1 -right-1 w-5 h-5 bg-blue-500 rounded-full
               flex items-center justify-center text-xs font-medium text-white"
        transition:fade={{ duration: 150 }}
      >
        {runningTasks.length}
      </div>
    {/if}
  </button>

  <!-- 下拉面板 -->
  {#if isOpen}
    <!-- 点击外部关闭 -->
    <div
      class="fixed inset-0 z-40"
      onclick={() => (isOpen = false)}
      role="presentation"
    ></div>

    <!-- 面板内容 -->
    <div
      class="absolute right-0 top-full mt-2 w-80 bg-gray-800 rounded-xl shadow-2xl
             border border-gray-700 overflow-hidden z-50"
      transition:fly={{ y: -10, duration: 200 }}
    >
      <!-- 头部 -->
      <div class="px-4 py-3 border-b border-gray-700 flex items-center justify-between">
        <h3 class="text-sm font-medium text-white">{$t('tasks.title')}</h3>
        {#if tasks.length > 0}
          <span class="text-xs text-gray-400">{$t('tasks.taskCount', { values: { n: tasks.length } })}</span>
        {/if}
      </div>

      <!-- 任务列表 -->
      <div class="max-h-80 overflow-y-auto">
        {#if tasks.length === 0}
          <div class="px-4 py-8 text-center text-gray-500">
            <Icon icon="mdi:tray-remove" class="text-4xl mb-2 opacity-50" />
            <p class="text-sm">{$t('tasks.noTasks')}</p>
          </div>
        {:else}
          <!-- 运行中的任务 -->
          {#if runningTasks.length > 0}
            <div class="px-3 py-2">
              <div class="text-xs text-gray-500 mb-2">{$t('tasks.running')}</div>
              {#each runningTasks as task (task.id)}
                <div
                  class="p-3 bg-gray-700/50 rounded-lg mb-2"
                  transition:slide={{ duration: 200 }}
                >
                  <div class="flex items-start gap-3">
                    <div
                      class="w-8 h-8 rounded-lg flex items-center justify-center shrink-0"
                      style="background-color: {categoryInfo[task.category]?.color}20"
                    >
                      <Icon
                        icon="mdi:loading"
                        class="text-lg animate-spin"
                        style="color: {categoryInfo[task.category]?.color}"
                      />
                    </div>
                    <div class="flex-1 min-w-0">
                      <div class="text-sm font-medium text-white truncate">
                        {task.title}
                      </div>
                      <div class="text-xs text-gray-400 truncate">
                        {task.package_name}
                      </div>
                    </div>
                    <button
                      class="p-1 text-gray-400 hover:text-red-400 transition-colors"
                      onclick={() => cancelTask(task.id)}
                      title={$t('tasks.cancelTask')}
                    >
                      <Icon icon="mdi:stop" class="text-lg" />
                    </button>
                  </div>
                  
                  <!-- 进度条（如果有） -->
                  {#if task.progress >= 0}
                    <div class="mt-2 h-1 bg-gray-600 rounded-full overflow-hidden">
                      <div
                        class="h-full bg-blue-500 transition-all duration-300"
                        style="width: {task.progress}%"
                      ></div>
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}

          <!-- 最近完成的任务 -->
          {#if recentTasks.length > 0}
            <div class="px-3 py-2 border-t border-gray-700">
              <div class="text-xs text-gray-500 mb-2">{$t('tasks.recentCompleted')}</div>
              {#each recentTasks as task (task.id)}
                <div
                  class="flex items-center gap-3 p-2 rounded-lg hover:bg-gray-700/30 transition-colors group"
                  transition:slide={{ duration: 200 }}
                >
                  <Icon
                    icon={getStatusIcon(task.status)}
                    class="text-lg shrink-0 {getStatusColor(task.status)}"
                  />
                  <div class="flex-1 min-w-0">
                    <div class="text-sm text-white truncate">{task.title}</div>
                    <div class="text-xs text-gray-500">
                      {formatTime(task.finished_at)}
                    </div>
                  </div>
                  <button
                    class="p-1 text-gray-500 hover:text-gray-300 opacity-0 group-hover:opacity-100 transition-opacity"
                    onclick={() => removeTask(task.id)}
                    title={$t('tasks.removeTask')}
                  >
                    <Icon icon="mdi:close" class="text-sm" />
                  </button>
                </div>
              {/each}
            </div>
          {/if}
        {/if}
      </div>

      <!-- 底部操作 -->
      {#if tasks.filter((t) => t.status !== "running").length > 0}
        <div class="px-4 py-3 border-t border-gray-700">
          <button
            class="w-full text-center text-xs text-gray-400 hover:text-gray-200 transition-colors"
            onclick={async () => {
              for (const task of tasks.filter((t) => t.status !== "running")) {
                await taskService.removeTask(task.id);
              }
              tasks = tasks.filter((t) => t.status === "running");
            }}
          >
            {$t('tasks.clearCompleted')}
          </button>
        </div>
      {/if}
    </div>
  {/if}
</div>
