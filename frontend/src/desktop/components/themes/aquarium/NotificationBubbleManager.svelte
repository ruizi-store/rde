<!-- NotificationBubbleManager.svelte - 通知泡泡管理器 -->
<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import NotificationBubble from "./NotificationBubble.svelte";
  import FallingText from "./FallingText.svelte";
  import ParticleSystem from "../effects/ParticleSystem.svelte";
  import { notificationBubbleStore } from "$shared/stores/notification-bubble.svelte";
  import { uiState } from "$shared/stores/ui-state.svelte";
  import { notificationService, type Notification } from "$shared/services/notification";

  interface BubbleData {
    id: string;
    notification: Notification;
    x: number;
    y: number;
    size: number;
  }

  interface FallingTextData {
    id: string;
    text: string;
    x: number;
    y: number;
    notificationId: string;
  }

  interface ParticleEffect {
    id: string;
    x: number;
    y: number;
    type: "pop" | "shatter";
    chars?: string[];
  }

  interface Props {
    taskbarHeight?: number;
  }

  let { taskbarHeight = 48 }: Props = $props();

  let bubbles = $state<BubbleData[]>([]);
  let fallingTexts = $state<FallingTextData[]>([]);
  let particleEffects = $state<ParticleEffect[]>([]);
  let pendingNotifications = $state<Notification[]>([]);

  // WebSocket 连接
  let ws: WebSocket | null = null;
  let bubbleIdCounter = 0;

  // 获取设置
  let maxBubbles = $derived(notificationBubbleStore.maxBubbles);
  let autoHideSeconds = $derived(notificationBubbleStore.autoHideSeconds);

  // 创建泡泡位置（避开桌面图标区域）
  function createBubblePosition(): { x: number; y: number } {
    const margin = 100;
    const iconAreaWidth = 120; // 桌面图标区域宽度

    // 在屏幕右侧 2/3 区域生成泡泡
    const minX = window.innerWidth * 0.35;
    const maxX = window.innerWidth - margin;
    const minY = margin;
    const maxY = window.innerHeight - taskbarHeight - margin;

    return {
      x: minX + Math.random() * (maxX - minX),
      y: minY + Math.random() * (maxY - minY),
    };
  }

  // 添加新通知泡泡
  function addNotificationBubble(notification: Notification) {
    // 检查是否已存在该通知的泡泡
    if (bubbles.some((b) => b.notification.id === notification.id)) {
      return;
    }

    // 如果已达到最大数量，加入队列
    if (bubbles.length >= maxBubbles) {
      if (!pendingNotifications.some((n) => n.id === notification.id)) {
        pendingNotifications = [...pendingNotifications, notification];
      }
      return;
    }

    const pos = createBubblePosition();
    const size = 85 + Math.random() * 20; // 85-105px

    bubbles = [
      ...bubbles,
      {
        id: `notif-bubble-${bubbleIdCounter++}`,
        notification,
        x: pos.x,
        y: pos.y,
        size,
      },
    ];
  }

  // 处理队列中的待处理通知
  function processQueue() {
    if (pendingNotifications.length > 0 && bubbles.length < maxBubbles) {
      const next = pendingNotifications[0];
      pendingNotifications = pendingNotifications.slice(1);
      addNotificationBubble(next);
    }
  }

  // 泡泡被戳破
  async function handleBubblePop(notification: Notification, x: number, y: number) {
    // 添加破裂粒子效果
    particleEffects = [
      ...particleEffects,
      {
        id: `pop-${Date.now()}`,
        x,
        y,
        type: "pop",
      },
    ];

    // 移除泡泡
    bubbles = bubbles.filter((b) => b.notification.id !== notification.id);

    // 添加掉落的文字
    fallingTexts = [
      ...fallingTexts,
      {
        id: `falling-${Date.now()}`,
        text: notification.title,
        x,
        y,
        notificationId: notification.id,
      },
    ];

    // 标记通知为已读
    try {
      await notificationService.markAsRead(notification.id);
    } catch (e) {
      console.error("标记通知已读失败:", e);
    }

    // 请求打开通知面板
    uiState.requestOpenNotificationPanel();

    // 处理队列
    setTimeout(processQueue, 500);
  }

  // 泡泡自动过期（飘走或超时）
  function handleBubbleExpire(notification: Notification) {
    bubbles = bubbles.filter((b) => b.notification.id !== notification.id);
    // 处理队列
    setTimeout(processQueue, 500);
  }

  // 文字碰到任务栏
  function handleTextHitTaskbar(x: number, y: number, chars: string[]) {
    // 添加碎裂效果
    particleEffects = [
      ...particleEffects,
      {
        id: `shatter-${Date.now()}`,
        x,
        y,
        type: "shatter",
        chars,
      },
    ];

    // 移除掉落的文字
    fallingTexts = fallingTexts.filter(
      (ft) => !(Math.abs(ft.x - x) < 50 && Math.abs(ft.y - y) < 50),
    );
  }

  // 粒子效果完成
  function handleParticleComplete(id: string) {
    particleEffects = particleEffects.filter((p) => p.id !== id);
  }

  // 连接 WebSocket 接收新通知
  function connectWebSocket() {
    ws = notificationService.connectWebSocket((data) => {
      if (data.type === "notification" && notificationBubbleStore.isEnabled) {
        const newNotification = data.data as Notification;
        // 新通知到达，添加泡泡
        addNotificationBubble(newNotification);
      }
    });
  }

  // 加载已有的未读通知并创建泡泡
  async function loadUnreadNotifications() {
    try {
      const result = await notificationService.getNotifications({ is_read: false });
      if (!result.success || !result.data) {
        console.error("加载未读通知失败:", result.message);
        return;
      }

      const unreadNotifications = result.data.items || [];

      // 延迟添加泡泡，让它们依次出现
      unreadNotifications.slice(0, maxBubbles).forEach((notification, index) => {
        setTimeout(() => {
          if (notificationBubbleStore.isEnabled) {
            addNotificationBubble(notification);
          }
        }, index * 300); // 每个泡泡间隔 300ms 出现
      });

      // 剩余的加入队列
      if (unreadNotifications.length > maxBubbles) {
        pendingNotifications = unreadNotifications.slice(maxBubbles);
      }
    } catch (e) {
      console.error("加载未读通知失败:", e);
    }
  }

  onMount(() => {
    if (notificationBubbleStore.isEnabled) {
      connectWebSocket();
      // 加载已有的未读通知
      loadUnreadNotifications();
    }
  });

  onDestroy(() => {
    ws?.close();
  });

  // 监听启用状态变化
  $effect(() => {
    if (notificationBubbleStore.isEnabled) {
      if (!ws) {
        connectWebSocket();
        // 加载已有的未读通知
        loadUnreadNotifications();
      }
    } else {
      ws?.close();
      ws = null;
      // 清空所有泡泡
      bubbles = [];
      pendingNotifications = [];
    }
  });

  // 监听清空信号
  $effect(() => {
    const signal = notificationBubbleStore.clearSignal;
    if (signal > 0) {
      // 清空所有泡泡和待处理队列
      bubbles = [];
      pendingNotifications = [];
      fallingTexts = [];
    }
  });

  // 监听单个通知移除
  $effect(() => {
    const removedIds = notificationBubbleStore.removedNotificationIds;
    if (removedIds.length > 0) {
      bubbles = bubbles.filter(b => !removedIds.includes(b.notification.id));
      pendingNotifications = pendingNotifications.filter(n => !removedIds.includes(n.id));
      notificationBubbleStore.clearRemovedIds();
    }
  });
</script>

{#if notificationBubbleStore.isEnabled}
  <div class="notification-bubble-manager">
    <!-- 通知泡泡 -->
    {#each bubbles as bubble (bubble.id)}
      <NotificationBubble
        notification={bubble.notification}
        initialX={bubble.x}
        initialY={bubble.y}
        size={bubble.size}
        {autoHideSeconds}
        onPop={handleBubblePop}
        onExpire={handleBubbleExpire}
      />
    {/each}

    <!-- 掉落的文字 -->
    {#each fallingTexts as ft (ft.id)}
      <FallingText
        text={ft.text}
        startX={ft.x}
        startY={ft.y}
        {taskbarHeight}
        onHitTaskbar={handleTextHitTaskbar}
      />
    {/each}

    <!-- 粒子效果 -->
    {#each particleEffects as effect (effect.id)}
      {#if effect.type === "pop"}
        <ParticleSystem
          x={effect.x}
          y={effect.y}
          particleCount={18}
          colors={["#fff", "#87CEEB", "#4FC3F7", "#B3E5FC", "#E1F5FE"]}
          spread={360}
          gravity={250}
          initialVelocity={180}
          lifetime={0.9}
          onComplete={() => handleParticleComplete(effect.id)}
        />
      {:else if effect.type === "shatter"}
        <ParticleSystem
          x={effect.x}
          y={effect.y}
          particleCount={effect.chars?.length || 10}
          chars={effect.chars}
          colors={["#fff", "#FFD700", "#FFA500", "#FF6B6B"]}
          spread={180}
          gravity={600}
          initialVelocity={250}
          lifetime={1.2}
          onComplete={() => handleParticleComplete(effect.id)}
        />
      {/if}
    {/each}
  </div>
{/if}

<style>
  .notification-bubble-manager {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
    z-index: 6; /* 比谚语泡泡高一层 */
    overflow: hidden;
  }
</style>
