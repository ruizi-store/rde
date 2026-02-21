<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { fly, fade } from "svelte/transition";
  import Icon from "@iconify/svelte";
  import { t } from "$lib/i18n";
  import {
    privilegeService,
    taskService,
    categoryInfo,
    riskLevelInfo,
    type AuthorizationRequest,
    type BackgroundTask,
  } from "$shared/services/privilege";

  // ===================== 类型 =====================
  type DialogMode = "authorize" | "executing" | "completed";

  // ===================== 状态 =====================
  let requests = $state<AuthorizationRequest[]>([]);
  let currentRequest = $derived(requests[0]);
  let processing = $state(false);
  let rememberChoice = $state(false);
  let countdown = $state(0);

  // 执行模式状态
  let mode = $state<DialogMode>("authorize");
  let currentTask = $state<BackgroundTask | null>(null);
  let terminalOutput = $state<string>("");
  let taskWs: WebSocket | null = null;

  // WebSocket 连接
  let ws: WebSocket | null = null;
  let countdownInterval: number | null = null;

  // ===================== 方法 =====================
  function updateCountdown() {
    if (!currentRequest || mode !== "authorize") {
      countdown = 0;
      return;
    }
    const expiresAt = new Date(currentRequest.expires_at).getTime();
    const now = Date.now();
    countdown = Math.max(0, Math.floor((expiresAt - now) / 1000));

    if (countdown === 0) {
      // 超时，移除请求
      requests = requests.slice(1);
    }
  }

  async function handleAuthorize(approved: boolean, useStreamMode: boolean = true) {
    if (!currentRequest || processing) return;
    processing = true;

    try {
      const response = await privilegeService.authorize({
        request_id: currentRequest.id,
        approved,
        remember: rememberChoice,
        stream_mode: useStreamMode && approved, // 只有同意时才使用流式模式
      });

      if (approved && response.stream_mode && response.task_id) {
        // 切换到执行模式
        mode = "executing";
        terminalOutput = "";
        connectTaskWebSocket(response.task_id);
      } else {
        // 传统模式或拒绝，直接移除请求
        requests = requests.slice(1);
        rememberChoice = false;
        mode = "authorize";
      }
    } catch (e) {
      console.error($t('auth.authFailed'), e);
    } finally {
      processing = false;
    }
  }

  function connectTaskWebSocket(taskId: string) {
    taskWs = taskService.connectTaskOutput(taskId, {
      onInit: (task) => {
        currentTask = task;
      },
      onOutput: (data) => {
        terminalOutput += data;
        // 自动滚动到底部
        requestAnimationFrame(() => {
          const terminal = document.getElementById("terminal-output");
          if (terminal) {
            terminal.scrollTop = terminal.scrollHeight;
          }
        });
      },
      onStatus: (task) => {
        currentTask = task;
        if (task.status === "completed" || task.status === "failed" || task.status === "cancelled") {
          mode = "completed";
        }
      },
      onError: (error) => {
        console.error($t('auth.taskWebSocketError'), error);
      },
      onClose: () => {
        // 连接关闭
      },
    });
  }

  function handleBackgroundRun() {
    // 放到后台运行
    taskWs?.close();
    taskWs = null;
    currentTask = null;
    terminalOutput = "";
    mode = "authorize";
    requests = requests.slice(1);
    rememberChoice = false;
  }

  function handleComplete() {
    // 完成后关闭
    taskWs?.close();
    taskWs = null;
    currentTask = null;
    terminalOutput = "";
    mode = "authorize";
    requests = requests.slice(1);
    rememberChoice = false;
  }

  function connectWebSocket() {
    ws = privilegeService.connectWebSocket((request) => {
      // 添加新请求到队列，标记支持流式模式
      request.stream_mode = true;
      requests = [...requests, request];
    });
  }

  // ===================== 生命周期 =====================
  onMount(async () => {
    // 连接 WebSocket
    connectWebSocket();

    // 等待授权请求通过 WebSocket 推送，不主动轮询

    // 启动倒计时
    countdownInterval = setInterval(updateCountdown, 1000) as unknown as number;
  });

  onDestroy(() => {
    ws?.close();
    taskWs?.close();
    if (countdownInterval) clearInterval(countdownInterval);
  });

  // 当请求变化时更新倒计时
  $effect(() => {
    if (currentRequest && mode === "authorize") {
      updateCountdown();
    }
  });
</script>

{#if currentRequest || mode !== "authorize"}
  <!-- 遮罩层 -->
  <div
    class="fixed inset-0 bg-black/60 backdrop-blur-md z-[9999] flex items-center justify-center"
    transition:fade={{ duration: 150 }}
    role="presentation"
  >
    {#if mode === "authorize" && currentRequest}
      <!-- 授权弹窗 -->
      <div
        class="bg-gray-900 rounded-2xl shadow-2xl w-[480px] max-w-[90vw] overflow-hidden border border-gray-700"
        transition:fly={{ y: 20, duration: 200 }}
        role="dialog"
        aria-modal="true"
        aria-labelledby="privilege-dialog-title"
      >
        <!-- 头部 -->
        <div class="px-6 py-4 border-b border-gray-700 flex items-center gap-3">
          <div class="w-10 h-10 rounded-xl bg-amber-500/20 flex items-center justify-center">
            <Icon icon="mdi:shield-lock" class="text-2xl text-amber-500" />
          </div>
          <div class="flex-1">
            <h2 id="privilege-dialog-title" class="text-lg font-semibold text-white">
              特权操作请求
            </h2>
            <p class="text-sm text-gray-400">
              需要您的授权才能继续
            </p>
          </div>
          <!-- 倒计时 -->
          <div class="text-sm text-gray-400">
            {countdown}s
          </div>
        </div>

        <!-- 内容 -->
        <div class="px-6 py-5 space-y-4">
          <!-- 套件信息 -->
          <div class="flex items-center gap-3 p-3 bg-gray-800 rounded-xl">
            <div
              class="w-10 h-10 rounded-lg flex items-center justify-center"
              style="background-color: {categoryInfo[currentRequest.category]?.color}20"
            >
              <Icon
                icon={categoryInfo[currentRequest.category]?.icon || "mdi:application"}
                class="text-xl"
                style="color: {categoryInfo[currentRequest.category]?.color}"
              />
            </div>
            <div class="flex-1 min-w-0">
              <div class="font-medium text-white truncate">
                {currentRequest.package_name}
              </div>
              <div class="text-sm text-gray-400 truncate">
                {currentRequest.package_id}
              </div>
            </div>
            <!-- 风险等级标签 -->
            <div
              class="px-2 py-1 rounded-md text-xs font-medium"
              style="
                background-color: {riskLevelInfo[currentRequest.risk_level]?.bgColor};
                color: {riskLevelInfo[currentRequest.risk_level]?.color};
              "
            >
              {riskLevelInfo[currentRequest.risk_level]?.name}
            </div>
          </div>

          <!-- 操作说明 -->
          <div>
            <div class="text-sm font-medium text-white mb-1">
              {currentRequest.title}
            </div>
            <div class="text-sm text-gray-400">
              {currentRequest.description}
            </div>
          </div>

          <!-- 命令预览 -->
          <div class="bg-gray-800 rounded-xl p-3">
            <div class="text-xs text-gray-500 mb-2">{$t('auth.commandToExecute')}</div>
            <code
              class="block text-sm font-mono text-gray-200 bg-black/30 p-2 rounded-lg overflow-x-auto whitespace-nowrap"
            >
              {currentRequest.command}
            </code>
          </div>

          <!-- 记住选择 -->
          <label class="flex items-center gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              bind:checked={rememberChoice}
              class="w-4 h-4 rounded border-2 border-gray-600 bg-transparent checked:bg-blue-500 checked:border-blue-500"
            />
            <span class="text-sm text-gray-400">
              记住此选择（此套件的此类操作将自动授权）
            </span>
          </label>
        </div>

        <!-- 操作按钮 -->
        <div class="px-6 py-4 border-t border-gray-700 flex gap-3">
          <button
            class="flex-1 px-4 py-2.5 rounded-xl border border-gray-600
                   text-gray-200 hover:bg-gray-700
                   transition-colors disabled:opacity-50"
            disabled={processing}
            onclick={() => handleAuthorize(false)}
          >
            取消
          </button>
          <button
            class="flex-1 px-4 py-2.5 rounded-xl bg-blue-500 text-white
                   hover:bg-blue-600 transition-colors disabled:opacity-50
                   flex items-center justify-center gap-2"
            disabled={processing}
            onclick={() => handleAuthorize(true, true)}
          >
            {#if processing}
              <Icon icon="mdi:loading" class="animate-spin" />
            {:else}
              <Icon icon="mdi:shield-check" />
            {/if}
            允许执行
          </button>
        </div>

        <!-- 队列指示器 -->
        {#if requests.length > 1}
          <div class="px-6 pb-4 text-center">
            <span class="text-xs text-gray-500">
              还有 {requests.length - 1} 个请求等待处理
            </span>
          </div>
        {/if}
      </div>
    {:else if mode === "executing" || mode === "completed"}
      <!-- 执行中/完成弹窗 -->
      <div
        class="bg-gray-900 rounded-2xl shadow-2xl w-[640px] max-w-[95vw] max-h-[80vh] overflow-hidden border border-gray-700 flex flex-col"
        transition:fly={{ y: 20, duration: 200 }}
        role="dialog"
        aria-modal="true"
      >
        <!-- 头部 -->
        <div class="px-6 py-4 border-b border-gray-700 flex items-center gap-3 shrink-0">
          <div
            class="w-10 h-10 rounded-xl flex items-center justify-center"
            class:bg-blue-500/20={mode === "executing"}
            class:bg-green-500/20={mode === "completed" && currentTask?.status === "completed"}
            class:bg-red-500/20={mode === "completed" && currentTask?.status === "failed"}
          >
            {#if mode === "executing"}
              <Icon icon="mdi:loading" class="text-2xl text-blue-500 animate-spin" />
            {:else if currentTask?.status === "completed"}
              <Icon icon="mdi:check-circle" class="text-2xl text-green-500" />
            {:else}
              <Icon icon="mdi:alert-circle" class="text-2xl text-red-500" />
            {/if}
          </div>
          <div class="flex-1">
            <h2 class="text-lg font-semibold text-white">
              {#if mode === "executing"}
                正在执行特权操作
              {:else if currentTask?.status === "completed"}
                操作完成
              {:else}
                操作失败
              {/if}
            </h2>
            <p class="text-sm text-gray-400">
              {currentTask?.title || currentRequest?.title}
            </p>
          </div>
        </div>

        <!-- 终端输出 -->
        <div class="flex-1 overflow-hidden p-4">
          <div
            id="terminal-output"
            class="w-full h-full bg-black rounded-xl p-4 font-mono text-sm text-gray-200 overflow-auto whitespace-pre-wrap"
            style="min-height: 300px; max-height: 400px;"
          >
            {terminalOutput || $t('auth.waitingOutput')}
          </div>
        </div>

        <!-- 错误信息 -->
        {#if currentTask?.error}
          <div class="px-4 pb-2">
            <div class="bg-red-500/10 border border-red-500/30 rounded-lg p-3 text-sm text-red-400">
              {currentTask.error}
            </div>
          </div>
        {/if}

        <!-- 操作按钮 -->
        <div class="px-6 py-4 border-t border-gray-700 flex gap-3 shrink-0">
          {#if mode === "executing"}
            <button
              class="flex-1 px-4 py-2.5 rounded-xl border border-gray-600
                     text-gray-200 hover:bg-gray-700 transition-colors
                     flex items-center justify-center gap-2"
              onclick={handleBackgroundRun}
            >
              <Icon icon="mdi:tray-arrow-down" />
              后台运行
            </button>
            <button
              class="px-4 py-2.5 rounded-xl border border-red-500/50
                     text-red-400 hover:bg-red-500/10 transition-colors
                     flex items-center justify-center gap-2"
              onclick={() => currentTask && taskService.cancelTask(currentTask.id)}
            >
              <Icon icon="mdi:stop" />
              取消
            </button>
          {:else}
            <button
              class="flex-1 px-4 py-2.5 rounded-xl bg-blue-500 text-white
                     hover:bg-blue-600 transition-colors
                     flex items-center justify-center gap-2"
              onclick={handleComplete}
            >
              <Icon icon="mdi:check" />
              确定
            </button>
          {/if}
        </div>
      </div>
    {/if}
  </div>
{/if}
