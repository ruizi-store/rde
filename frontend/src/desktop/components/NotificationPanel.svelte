<script lang="ts">
  import { t } from "svelte-i18n";
  import { onMount, onDestroy } from "svelte";
  import { fly } from "svelte/transition";
  import {
    notificationService,
    categoryInfo,
    severityInfo,
    type Notification,
    type NotificationCategory,
  } from "$shared/services/notification";
  import { notificationBubbleStore } from "$shared/stores/notification-bubble.svelte";
  import Icon from "@iconify/svelte";

  let { visible = $bindable(false) }: { visible: boolean } = $props();

  // ===================== 状态 =====================
  let notifications = $state<Notification[]>([]);
  let loading = $state(false);
  let filter = $state<"all" | "unread">("all");
  let unreadCount = $state(0);
  let totalCount = $state(0);

  // WebSocket
  let ws: WebSocket | null = null;

  // ===================== 派生 =====================
  let filteredNotifications = $derived.by(() => {
    if (filter === "unread") {
      return notifications.filter((n) => !n.is_read);
    }
    return notifications;
  });

  let groupedNotifications = $derived.by(() => {
    const groups: { [key: string]: Notification[] } = {};
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const yesterday = new Date(today);
    yesterday.setDate(yesterday.getDate() - 1);

    for (const n of filteredNotifications) {
      const date = new Date(n.created_at);
      date.setHours(0, 0, 0, 0);

      let key: string;
      if (date.getTime() === today.getTime()) {
        key = $t("notification.today");
      } else if (date.getTime() === yesterday.getTime()) {
        key = $t("notification.yesterday");
      } else {
        key = date.toLocaleDateString("zh-CN", { month: "long", day: "numeric" });
      }

      if (!groups[key]) {
        groups[key] = [];
      }
      groups[key].push(n);
    }
    return groups;
  });

  // ===================== 加载数据 =====================
  async function loadNotifications() {
    loading = true;
    try {
      const response = await notificationService.getNotifications();
      if (response.success && response.data) {
        notifications = response.data.items || [];
        unreadCount = response.data.unread_count;
        totalCount = response.data.total;
      }
    } catch (err) {
      console.error($t("notification.loadFailed"), err);
    } finally {
      loading = false;
    }
  }

  // ===================== 通知操作 =====================
  async function markAsRead(notification: Notification) {
    if (notification.is_read) return;
    await notificationService.markAsRead(notification.id);
    notifications = notifications.map((n) =>
      n.id === notification.id ? { ...n, is_read: true } : n,
    );
    unreadCount = Math.max(0, unreadCount - 1);
    // 移除对应的桌面泡泡
    notificationBubbleStore.removeNotification(notification.id);
  }

  async function markAllAsRead() {
    await notificationService.markAllAsRead();
    notifications = notifications.map((n) => ({ ...n, is_read: true }));
    unreadCount = 0;
    // 清空所有桌面泡泡
    notificationBubbleStore.triggerClear();
  }

  async function deleteNotification(id: string, e?: Event) {
    e?.stopPropagation();
    await notificationService.deleteNotification(id);
    const wasUnread = notifications.find((n) => n.id === id && !n.is_read);
    notifications = notifications.filter((n) => n.id !== id);
    if (wasUnread) unreadCount = Math.max(0, unreadCount - 1);
    totalCount = Math.max(0, totalCount - 1);
    // 移除对应的桌面泡泡
    notificationBubbleStore.removeNotification(id);
  }

  async function clearAllNotifications() {
    await notificationService.clearAll();
    notifications = [];
    unreadCount = 0;
    totalCount = 0;
    // 清空所有桌面泡泡
    notificationBubbleStore.triggerClear();
  }

  // ===================== 辅助函数 =====================
  function formatTime(dateStr: string): string {
    const date = new Date(dateStr);
    const now = new Date();
    const diff = now.getTime() - date.getTime();

    if (diff < 60000) return $t("common.justNow");
    if (diff < 3600000) return $t("common.minutesAgo", { values: { n: Math.floor(diff / 60000) } });
    if (diff < 86400000) return $t("common.hoursAgo", { values: { n: Math.floor(diff / 3600000) } });
    return date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
  }

  function getNotificationIcon(notification: Notification): string {
    return severityInfo[notification.severity]?.icon || "mdi:bell";
  }

  function getNotificationColor(notification: Notification): string {
    return severityInfo[notification.severity]?.color || "#666";
  }

  function getCategoryColor(category: NotificationCategory): string {
    return categoryInfo[category]?.color || "#666";
  }

  // ===================== WebSocket =====================
  function connectWebSocket() {
    ws = notificationService.connectWebSocket((data) => {
      if (data.type === "notification") {
        const newNotification = data.data as Notification;
        notifications = [newNotification, ...notifications];
        unreadCount++;
        totalCount++;
      } else if (data.type === "unread_count") {
        unreadCount = (data.data as { count: number }).count;
      }
    });
  }

  function close() {
    visible = false;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      close();
    }
  }

  // ===================== 生命周期 =====================
  let refreshInterval: number;

  $effect(() => {
    if (visible) {
      loadNotifications();
      connectWebSocket();
      refreshInterval = setInterval(loadNotifications, 60000) as unknown as number;
    } else {
      ws?.close();
      ws = null;
      clearInterval(refreshInterval);
    }
  });

  onDestroy(() => {
    ws?.close();
    clearInterval(refreshInterval);
  });
</script>

{#if visible}
  <!-- 遮罩层 -->
  <div
    class="notification-overlay"
    onclick={close}
    onkeydown={handleKeydown}
    role="presentation"
    tabindex="-1"
  ></div>

  <!-- 侧边面板 -->
  <aside class="notification-panel" transition:fly={{ x: 380, duration: 200 }}>
    <!-- 头部 -->
    <header class="panel-header">
      <h2>{$t("notification.title")}</h2>
      <button class="close-btn" onclick={close} title={$t("common.close")}>
        <Icon icon="mdi:close" width="20" />
      </button>
    </header>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <div class="filter-tabs">
        <button class:active={filter === "all"} onclick={() => (filter = "all")}>
          全部 ({totalCount})
        </button>
        <button class:active={filter === "unread"} onclick={() => (filter = "unread")}>
          未读 ({unreadCount})
        </button>
      </div>
    </div>

    <!-- 通知列表 -->
    <div class="notification-list">
      {#if loading && notifications.length === 0}
        <div class="empty">
          <Icon icon="mdi:loading" width="32" class="spin" />
          <p>{$t("common.loading")}</p>
        </div>
      {:else if filteredNotifications.length === 0}
        <div class="empty">
          <Icon icon="mdi:bell-off-outline" width="48" />
          <p>{filter === "unread" ? $t("notification.noUnread") : $t("notification.empty")}</p>
          <span class="hint">{$t("notification.emptyHint")}</span>
        </div>
      {:else}
        {#each Object.entries(groupedNotifications) as [date, items]}
          <div class="date-group">
            <div class="date-header">
              <Icon icon="mdi:calendar" width="14" />
              {date}
            </div>
            {#each items as notification (notification.id)}
              <div
                class="notification-item"
                class:unread={!notification.is_read}
                onclick={() => markAsRead(notification)}
                role="button"
                tabindex="0"
                onkeydown={(e) => e.key === "Enter" && markAsRead(notification)}
              >
                <div class="icon" style="color: {getNotificationColor(notification)}">
                  <Icon icon={getNotificationIcon(notification)} width="20" />
                </div>
                <div class="body">
                  <div class="title-row">
                    <span class="title">{notification.title}</span>
                    <span class="time">{formatTime(notification.created_at)}</span>
                  </div>
                  <p class="message">{notification.content}</p>
                  <div class="meta">
                    <span
                      class="category-tag"
                      style="background: {getCategoryColor(
                        notification.category,
                      )}20; color: {getCategoryColor(notification.category)}"
                    >
                      {categoryInfo[notification.category]?.name || notification.category}
                    </span>
                    <span class="severity-tag" style="color: {getNotificationColor(notification)}">
                      {severityInfo[notification.severity]?.name || notification.severity}
                    </span>
                  </div>
                  {#if notification.link}
                    <a
                      href={notification.link}
                      class="action-link"
                      onclick={(e) => e.stopPropagation()}
                    >
                      查看详情 →
                    </a>
                  {/if}
                </div>
                <button
                  class="delete-btn"
                  onclick={(e) => deleteNotification(notification.id, e)}
                  title={$t("common.delete")}
                >
                  <Icon icon="mdi:close" width="14" />
                </button>
              </div>
            {/each}
          </div>
        {/each}
      {/if}
    </div>

    <!-- 底部操作栏 -->
    {#if notifications.length > 0}
      <footer class="panel-footer">
        {#if unreadCount > 0}
          <button class="footer-btn" onclick={markAllAsRead}>
            <Icon icon="mdi:check-all" width="16" />
            全部已读
          </button>
        {/if}
        <button class="footer-btn" onclick={clearAllNotifications}>
          <Icon icon="mdi:delete-sweep" width="16" />
          清空
        </button>
      </footer>
    {/if}
  </aside>
{/if}

<style>
  .notification-overlay {
    position: fixed;
    inset: 0;
    bottom: 48px; /* 任务栏高度 */
    z-index: 9998;
    background: transparent;
  }

  .notification-panel {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 48px; /* 任务栏高度 */
    width: 380px;
    background: var(--panel-bg, rgba(32, 32, 36, 0.98));
    backdrop-filter: blur(20px);
    border-left: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
    display: flex;
    flex-direction: column;
    z-index: 9999;
    box-shadow: -4px 0 24px rgba(0, 0, 0, 0.3);

    :global([data-theme="light"]) & {
      background: var(--panel-bg, rgba(255, 255, 255, 0.98));
      border-left-color: var(--border-color, rgba(0, 0, 0, 0.1));
      box-shadow: -4px 0 24px rgba(0, 0, 0, 0.1);
    }
  }

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.08));

    :global([data-theme="light"]) & {
      border-bottom-color: var(--border-color, rgba(0, 0, 0, 0.08));
    }

    h2 {
      margin: 0;
      font-size: 18px;
      font-weight: 600;
      color: var(--text-primary, #fff);

      :global([data-theme="light"]) & {
        color: var(--text-primary, #1a1a1a);
      }
    }

    .close-btn {
      width: 32px;
      height: 32px;
      border: none;
      border-radius: 6px;
      background: transparent;
      color: var(--text-secondary, rgba(255, 255, 255, 0.7));
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: all 0.15s;

      &:hover {
        background: var(--hover-bg, rgba(255, 255, 255, 0.1));
      }

      :global([data-theme="light"]) & {
        color: var(--text-secondary, rgba(0, 0, 0, 0.6));

        &:hover {
          background: var(--hover-bg, rgba(0, 0, 0, 0.06));
        }
      }
    }
  }

  .filter-bar {
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.05));

    :global([data-theme="light"]) & {
      border-bottom-color: var(--border-color, rgba(0, 0, 0, 0.05));
    }
  }

  .filter-tabs {
    display: flex;
    gap: 4px;
    background: var(--tab-bg, rgba(255, 255, 255, 0.05));
    border-radius: 8px;
    padding: 4px;

    :global([data-theme="light"]) & {
      background: var(--tab-bg, rgba(0, 0, 0, 0.05));
    }

    button {
      flex: 1;
      padding: 8px 12px;
      border: none;
      border-radius: 6px;
      background: transparent;
      color: var(--text-secondary, rgba(255, 255, 255, 0.7));
      font-size: 13px;
      cursor: pointer;
      transition: all 0.15s;

      :global([data-theme="light"]) & {
        color: var(--text-secondary, rgba(0, 0, 0, 0.6));
      }

      &:hover {
        background: var(--hover-bg, rgba(255, 255, 255, 0.08));

        :global([data-theme="light"]) & {
          background: var(--hover-bg, rgba(0, 0, 0, 0.06));
        }
      }

      &.active {
        background: var(--active-bg, rgba(255, 255, 255, 0.12));
        color: var(--text-primary, #fff);

        :global([data-theme="light"]) & {
          background: var(--active-bg, #fff);
          color: var(--text-primary, #1a1a1a);
          box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
        }
      }
    }
  }

  .notification-list {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
  }

  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 60px 20px;
    color: var(--text-muted, rgba(255, 255, 255, 0.4));
    text-align: center;

    :global([data-theme="light"]) & {
      color: var(--text-muted, rgba(0, 0, 0, 0.4));
    }

    p {
      margin: 12px 0 4px;
      font-size: 15px;
    }

    .hint {
      font-size: 13px;
      opacity: 0.7;
    }

    :global(.spin) {
      animation: spin 1s linear infinite;
    }
  }

  @keyframes spin {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }

  .date-group {
    margin-bottom: 12px;
  }

  .date-header {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 12px;
    font-size: 12px;
    font-weight: 500;
    color: var(--text-muted, rgba(255, 255, 255, 0.5));

    :global([data-theme="light"]) & {
      color: var(--text-muted, rgba(0, 0, 0, 0.5));
    }
  }

  .notification-item {
    display: flex;
    gap: 12px;
    padding: 12px;
    margin: 4px 0;
    background: var(--item-bg, rgba(255, 255, 255, 0.03));
    border-radius: 10px;
    cursor: pointer;
    transition: all 0.15s;
    position: relative;

    :global([data-theme="light"]) & {
      background: var(--item-bg, #fff);
      border: 1px solid rgba(0, 0, 0, 0.06);
    }

    &:hover {
      background: var(--item-hover-bg, rgba(255, 255, 255, 0.06));

      :global([data-theme="light"]) & {
        background: var(--item-hover-bg, #f8f8f8);
      }

      .delete-btn {
        opacity: 1;
      }
    }

    &.unread {
      background: var(--unread-bg, rgba(59, 130, 246, 0.1));
      border-left: 3px solid #3b82f6;

      :global([data-theme="light"]) & {
        background: var(--unread-bg, rgba(59, 130, 246, 0.08));
      }

      .title {
        font-weight: 600;
      }
    }
  }

  .icon {
    flex-shrink: 0;
    padding-top: 2px;
  }

  .body {
    flex: 1;
    min-width: 0;
  }

  .title-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .title {
    font-size: 14px;
    color: var(--text-primary, #fff);

    :global([data-theme="light"]) & {
      color: var(--text-primary, #1a1a1a);
    }
  }

  .time {
    font-size: 11px;
    color: var(--text-muted, rgba(255, 255, 255, 0.4));
    flex-shrink: 0;

    :global([data-theme="light"]) & {
      color: var(--text-muted, rgba(0, 0, 0, 0.4));
    }
  }

  .message {
    margin: 4px 0;
    font-size: 13px;
    color: var(--text-secondary, rgba(255, 255, 255, 0.7));
    line-height: 1.4;

    :global([data-theme="light"]) & {
      color: var(--text-secondary, rgba(0, 0, 0, 0.6));
    }
  }

  .meta {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 6px;
  }

  .category-tag {
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
  }

  .severity-tag {
    font-size: 11px;
  }

  .action-link {
    display: inline-block;
    margin-top: 6px;
    font-size: 12px;
    color: #3b82f6;
    text-decoration: none;

    &:hover {
      text-decoration: underline;
    }
  }

  .delete-btn {
    position: absolute;
    top: 8px;
    right: 8px;
    width: 24px;
    height: 24px;
    padding: 0;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: var(--text-muted, rgba(255, 255, 255, 0.4));
    cursor: pointer;
    opacity: 0;
    transition: all 0.15s;
    display: flex;
    align-items: center;
    justify-content: center;

    :global([data-theme="light"]) & {
      color: var(--text-muted, rgba(0, 0, 0, 0.4));
    }

    &:hover {
      background: rgba(239, 68, 68, 0.2);
      color: #ef4444;
    }
  }

  .panel-footer {
    display: flex;
    gap: 8px;
    padding: 12px 16px;
    border-top: 1px solid var(--border-color, rgba(255, 255, 255, 0.08));

    :global([data-theme="light"]) & {
      border-top-color: var(--border-color, rgba(0, 0, 0, 0.08));
    }
  }

  .footer-btn {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 10px 12px;
    border: none;
    border-radius: 8px;
    background: var(--btn-bg, rgba(255, 255, 255, 0.08));
    color: var(--text-secondary, rgba(255, 255, 255, 0.8));
    font-size: 13px;
    cursor: pointer;
    transition: all 0.15s;

    :global([data-theme="light"]) & {
      background: var(--btn-bg, rgba(0, 0, 0, 0.05));
      color: var(--text-secondary, rgba(0, 0, 0, 0.7));
    }

    &:hover {
      background: var(--btn-hover-bg, rgba(255, 255, 255, 0.12));

      :global([data-theme="light"]) & {
        background: var(--btn-hover-bg, rgba(0, 0, 0, 0.08));
      }
    }
  }
</style>
