<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { fly, fade } from "svelte/transition";
  import Icon from "@iconify/svelte";
  import { t } from "$lib/i18n";
  import {
    privilegeService,
    categoryInfo,
    riskLevelInfo,
    type AuthorizationRequest,
  } from "$shared/services/privilege";

  // ===================== 状态 =====================
  let requests = $state<AuthorizationRequest[]>([]);
  let currentRequest = $derived(requests[0]);
  let processing = $state(false);
  let rememberChoice = $state(false);
  let countdown = $state(0);

  // WebSocket 连接
  let ws: WebSocket | null = null;
  let countdownInterval: number | null = null;

  // ===================== 方法 =====================
  function updateCountdown() {
    if (!currentRequest) {
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

  async function handleAuthorize(approved: boolean) {
    if (!currentRequest || processing) return;
    processing = true;

    try {
      await privilegeService.authorize({
        request_id: currentRequest.id,
        approved,
        remember: rememberChoice,
      });
      // 移除已处理的请求
      requests = requests.slice(1);
      rememberChoice = false;
    } catch (e) {
      console.error($t('auth.authFailed'), e);
    } finally {
      processing = false;
    }
  }

  function connectWebSocket() {
    ws = privilegeService.connectWebSocket((request) => {
      // 添加新请求到队列
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
    if (countdownInterval) clearInterval(countdownInterval);
  });

  // 当请求变化时更新倒计时
  $effect(() => {
    if (currentRequest) {
      updateCountdown();
    }
  });
</script>

{#if currentRequest}
  <!-- 遮罩层 -->
  <div
    class="fixed inset-0 bg-black/60 backdrop-blur-md z-[9999] flex items-center justify-center"
    transition:fade={{ duration: 150 }}
    role="presentation"
  >
    <!-- 弹窗 -->
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
            {$t('auth.privilegeRequest')}
          </h2>
          <p class="text-sm text-gray-400">
            {$t('auth.requireAuth')}
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
            {$t('auth.rememberChoice')}
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
          {$t('common.cancel')}
        </button>
        <button
          class="flex-1 px-4 py-2.5 rounded-xl bg-blue-500 text-white
                 hover:bg-blue-600 transition-colors disabled:opacity-50
                 flex items-center justify-center gap-2"
          disabled={processing}
          onclick={() => handleAuthorize(true)}
        >
          {#if processing}
            <Icon icon="mdi:loading" class="animate-spin" />
          {:else}
            <Icon icon="mdi:shield-check" />
          {/if}
          {$t('auth.allowExecute')}
        </button>
      </div>

      <!-- 队列指示器 -->
      {#if requests.length > 1}
        <div class="px-6 pb-4 text-center">
          <span class="text-xs text-gray-500">
            {$t('auth.pendingRequests', { values: { n: requests.length - 1 } })}
          </span>
        </div>
      {/if}
    </div>
  </div>
{/if}
