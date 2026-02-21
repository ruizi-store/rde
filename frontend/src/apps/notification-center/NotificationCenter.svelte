<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { t } from "svelte-i18n";
  import {
    notificationService,
    categoryInfo,
    severityInfo,
    channelTypeInfo,
    allCategories,
    allSeverities,
    allChannelTypes,
    type Notification,
    type NotificationSettings,
    type NotificationChannel,
    type NotificationCategory,
    type NotificationSeverity,
    type ChannelType,
  } from "$shared/services/notification";
  import { notificationBubbleStore } from "$shared/stores/notification-bubble.svelte";
  import Icon from "@iconify/svelte";
  import { useConfirm } from "$shared/ui";

  let { windowId }: { windowId: string } = $props();

  const confirm = useConfirm();

  // ===================== 状态 =====================
  let activeTab = $state<"notifications" | "settings">("notifications");
  let notifications = $state<Notification[]>([]);
  let settings = $state<NotificationSettings | null>(null);
  let channels = $state<NotificationChannel[]>([]);
  let loading = $state(false);
  let filter = $state<"all" | "unread">("all");
  let selectedCategories = $state<string[]>([]);
  let selectedSeverities = $state<string[]>([]);
  let selectedNotifications = $state<Set<string>>(new Set());
  let unreadCount = $state(0);
  let totalCount = $state(0);

  // 渠道编辑
  let editingChannel = $state<NotificationChannel | null>(null);
  let showChannelForm = $state(false);
  let channelFormType = $state<ChannelType>("email");

  // WebSocket
  let ws: WebSocket | null = null;

  // ===================== 派生 =====================
  let filteredNotifications = $derived.by(() => {
    let result = notifications;
    if (filter === "unread") {
      result = result.filter((n) => !n.is_read);
    }
    if (selectedCategories.length > 0) {
      result = result.filter((n) => selectedCategories.includes(n.category));
    }
    if (selectedSeverities.length > 0) {
      result = result.filter((n) => selectedSeverities.includes(n.severity));
    }
    return result;
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

  let isAllSelected = $derived(
    filteredNotifications.length > 0 &&
      filteredNotifications.every((n) => selectedNotifications.has(n.id)),
  );

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
      console.error("加载通知失败", err);
    } finally {
      loading = false;
    }
  }

  async function loadSettings() {
    try {
      const response = await notificationService.getSettings();
      if (response.success && response.data) {
        settings = response.data;
      }
    } catch (err) {
      console.error("加载设置失败", err);
    }
  }

  async function loadChannels() {
    try {
      const response = await notificationService.getChannels();
      if (response.success && response.data) {
        channels = response.data;
      }
    } catch (err) {
      console.error("加载渠道失败", err);
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
    selectedNotifications.delete(id);
    selectedNotifications = new Set(selectedNotifications);
    // 移除对应的桌面泡泡
    notificationBubbleStore.removeNotification(id);
  }

  async function deleteSelected() {
    if (selectedNotifications.size === 0) return;
    const confirmed = await confirm.show({
      title: $t("notification.deleteNotification"),
      message: $t("notification.deleteSelectedConfirm", { values: { n: selectedNotifications.size } }),
      type: "warning",
    });
    if (!confirmed) return;

    for (const id of selectedNotifications) {
      await notificationService.deleteNotification(id);
      // 移除对应的桌面泡泡
      notificationBubbleStore.removeNotification(id);
    }
    const deletedUnread = notifications.filter(
      (n) => selectedNotifications.has(n.id) && !n.is_read,
    ).length;
    notifications = notifications.filter((n) => !selectedNotifications.has(n.id));
    unreadCount = Math.max(0, unreadCount - deletedUnread);
    totalCount = Math.max(0, totalCount - selectedNotifications.size);
    selectedNotifications = new Set();
  }

  async function clearReadNotifications() {
    const confirmed = await confirm.show({
      title: $t("notification.clearRead"),
      message: $t("notification.clearReadConfirm"),
      type: "warning",
    });
    if (!confirmed) return;
    await notificationService.deleteReadNotifications();
    const readCount = notifications.filter((n) => n.is_read).length;
    notifications = notifications.filter((n) => !n.is_read);
    totalCount = Math.max(0, totalCount - readCount);
  }

  function toggleSelectAll() {
    if (isAllSelected) {
      selectedNotifications = new Set();
    } else {
      selectedNotifications = new Set(filteredNotifications.map((n) => n.id));
    }
  }

  function toggleSelect(id: string, e: Event) {
    e.stopPropagation();
    if (selectedNotifications.has(id)) {
      selectedNotifications.delete(id);
    } else {
      selectedNotifications.add(id);
    }
    selectedNotifications = new Set(selectedNotifications);
  }

  // ===================== 设置操作 =====================
  async function saveSettings() {
    if (!settings) return;
    await notificationService.updateSettings(settings);
  }

  function toggleCategory(category: string) {
    if (!settings) return;
    const idx = settings.filter_categories.indexOf(category);
    if (idx >= 0) {
      settings.filter_categories = settings.filter_categories.filter((c) => c !== category);
    } else {
      settings.filter_categories = [...settings.filter_categories, category];
    }
    saveSettings();
  }

  function toggleSeverity(severity: string) {
    if (!settings) return;
    const idx = settings.filter_severities.indexOf(severity);
    if (idx >= 0) {
      settings.filter_severities = settings.filter_severities.filter((s) => s !== severity);
    } else {
      settings.filter_severities = [...settings.filter_severities, severity];
    }
    saveSettings();
  }

  // ===================== 渠道操作 =====================
  async function testChannel(channel: NotificationChannel) {
    const result = await notificationService.testChannel(channel.id);
    if (result.success) {
      alert($t("notification.testSent"));
    } else {
      alert($t("notification.testFailed") + ": " + result.message);
    }
  }

  async function toggleChannelEnabled(channel: NotificationChannel) {
    await notificationService.updateChannel(channel.id, { enabled: !channel.enabled });
    await loadChannels();
  }

  async function deleteChannel(channel: NotificationChannel) {
    const confirmed = await confirm.show({
      title: $t("notification.deleteChannel"),
      message: $t("notification.deleteChannelConfirm", { values: { name: channel.name } }),
      type: "warning",
    });
    if (!confirmed) return;
    await notificationService.deleteChannel(channel.id);
    await loadChannels();
  }

  // ===================== 辅助函数 =====================
  function formatTime(dateStr: string): string {
    const date = new Date(dateStr);
    const now = new Date();
    const diff = now.getTime() - date.getTime();

    if (diff < 60000) return $t("notification.justNow");
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

        // 显示桌面通知
        if (settings?.desktop_notify && Notification.permission === "granted") {
          new Notification(newNotification.title, {
            body: newNotification.content,
            icon: "/icons/notification.png",
          });
        }

        // 播放提示音
        if (settings?.sound_enabled) {
          const audio = new Audio("/sounds/notification.mp3");
          audio.play().catch(() => {});
        }
      } else if (data.type === "unread_count") {
        unreadCount = (data.data as { count: number }).count;
      }
    });
  }

  // ===================== 生命周期 =====================
  let refreshInterval: number;

  onMount(() => {
    loadNotifications();
    loadSettings();
    loadChannels();
    connectWebSocket();
    refreshInterval = setInterval(loadNotifications, 60000) as unknown as number;

    // 请求桌面通知权限
    if ("Notification" in window && Notification.permission === "default") {
      Notification.requestPermission();
    }
  });

  onDestroy(() => {
    clearInterval(refreshInterval);
    ws?.close();
  });
</script>

<div class="notification-center">
  <!-- 标签栏 -->
  <div class="header">
    <div class="tabs">
      <button
        class="tab"
        class:active={activeTab === "notifications"}
        onclick={() => (activeTab = "notifications")}
      >
        <Icon icon="mdi:bell" />
        {$t("notification.notifications")}
        {#if unreadCount > 0}
          <span class="badge">{unreadCount}</span>
        {/if}
      </button>
      <button
        class="tab"
        class:active={activeTab === "settings"}
        onclick={() => (activeTab = "settings")}
      >
        <Icon icon="mdi:cog" />
        {$t("notification.settings")}
      </button>
    </div>
  </div>

  <div class="content">
    {#if activeTab === "notifications"}
      <!-- 通知工具栏 -->
      <div class="toolbar">
        <div class="filter-section">
          <div class="filter-tabs">
            <button class:active={filter === "all"} onclick={() => (filter = "all")}>
              {$t("notification.all")} ({totalCount})
            </button>
            <button class:active={filter === "unread"} onclick={() => (filter = "unread")}>
              {$t("notification.unread")} ({unreadCount})
            </button>
          </div>

          <div class="filter-dropdowns">
            <select
              bind:value={selectedCategories}
              multiple
              class="filter-select"
              title={$t("notification.filterByType")}
            >
              {#each allCategories as cat}
                <option value={cat}>{categoryInfo[cat].name}</option>
              {/each}
            </select>
          </div>
        </div>

        <div class="actions">
          <label class="select-all">
            <input
              type="checkbox"
              checked={isAllSelected}
              onchange={toggleSelectAll}
              disabled={filteredNotifications.length === 0}
            />
            {$t("notification.selectAll")}
          </label>
          {#if selectedNotifications.size > 0}
            <button class="action-btn danger" onclick={deleteSelected} title={$t("notification.deleteSelected")}>
              <Icon icon="mdi:delete" />
              {$t("notification.delete")} ({selectedNotifications.size})
            </button>
          {/if}
          {#if unreadCount > 0}
            <button class="action-btn" onclick={markAllAsRead} title={$t("notification.markAllRead")}>
              <Icon icon="mdi:check-all" />
            </button>
          {/if}
          <button class="action-btn" onclick={clearReadNotifications} title={$t("notification.clearRead")}>
            <Icon icon="mdi:delete-sweep" />
          </button>
          <button class="action-btn" onclick={loadNotifications} title={$t("notification.refresh")}>
            <Icon icon="mdi:refresh" />
          </button>
        </div>
      </div>

      <!-- 通知列表 -->
      <div class="notification-list">
        {#if loading}
          <div class="empty">
            <Icon icon="mdi:loading" width="32" class="spin" />
            <span>{$t("notification.loading")}</span>
          </div>
        {:else if filteredNotifications.length === 0}
          <div class="empty">
            <Icon icon="mdi:bell-off-outline" width="64" />
            <p>{filter === "unread" ? $t("notification.noUnread") : $t("notification.empty")}</p>
            <span class="hint">{$t("notification.emptyHint")}</span>
          </div>
        {:else}
          {#each Object.entries(groupedNotifications) as [date, items]}
            <div class="date-group">
              <div class="date-header">
                <Icon icon="mdi:calendar" />
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
                  <div class="checkbox-wrapper">
                    <input
                      type="checkbox"
                      checked={selectedNotifications.has(notification.id)}
                      onclick={(e) => toggleSelect(notification.id, e)}
                    />
                  </div>
                  <div class="icon" style="color: {getNotificationColor(notification)}">
                    <Icon icon={getNotificationIcon(notification)} width="24" />
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
                      <span
                        class="severity-tag"
                        style="color: {getNotificationColor(notification)}"
                      >
                        {severityInfo[notification.severity]?.name || notification.severity}
                      </span>
                    </div>
                    {#if notification.link}
                      <a
                        href={notification.link}
                        class="action-link"
                        onclick={(e) => e.stopPropagation()}
                      >
                        {$t("notification.viewDetails")} →
                      </a>
                    {/if}
                  </div>
                  <button
                    class="delete-btn"
                    onclick={(e) => deleteNotification(notification.id, e)}
                    title={$t("notification.delete")}
                  >
                    <Icon icon="mdi:close" width="16" />
                  </button>
                </div>
              {/each}
            </div>
          {/each}
        {/if}
      </div>
    {:else}
      <!-- 设置面板 -->
      <div class="settings-panel">
        {#if settings}
          <!-- 基本设置 -->
          <div class="setting-section">
            <h3>
              <Icon icon="mdi:tune" />
              {$t("notification.basicSettings")}
            </h3>
            <div class="setting-item">
              <div class="setting-info">
                <span class="setting-label">{$t("notification.enableNotifications")}</span>
                <span class="setting-desc">{$t("notification.receiveSystemNotifications")}</span>
              </div>
              <label class="switch">
                <input type="checkbox" bind:checked={settings.enabled} onchange={saveSettings} />
                <span class="slider"></span>
              </label>
            </div>
            <div class="setting-item">
              <div class="setting-info">
                <span class="setting-label">{$t("notification.desktopNotifications")}</span>
                <span class="setting-desc">{$t("notification.desktopNotificationsDesc")}</span>
              </div>
              <label class="switch">
                <input
                  type="checkbox"
                  bind:checked={settings.desktop_notify}
                  onchange={saveSettings}
                />
                <span class="slider"></span>
              </label>
            </div>
            <div class="setting-item">
              <div class="setting-info">
                <span class="setting-label">{$t("notification.notificationSound")}</span>
                <span class="setting-desc">{$t("notification.notificationSoundDesc")}</span>
              </div>
              <label class="switch">
                <input
                  type="checkbox"
                  bind:checked={settings.sound_enabled}
                  onchange={saveSettings}
                />
                <span class="slider"></span>
              </label>
            </div>
          </div>

          <!-- 免打扰 -->
          <div class="setting-section">
            <h3>
              <Icon icon="mdi:moon-waning-crescent" />
              {$t("notification.dnd")}
            </h3>
            <div class="setting-item">
              <div class="setting-info">
                <span class="setting-label">{$t("notification.enableDnd")}</span>
                <span class="setting-desc">{$t("notification.dndDesc")}</span>
              </div>
              <label class="switch">
                <input
                  type="checkbox"
                  bind:checked={settings.dnd_enabled}
                  onchange={saveSettings}
                />
                <span class="slider"></span>
              </label>
            </div>
            {#if settings.dnd_enabled}
              <div class="setting-item time-range">
                <span>{$t("notification.timeRange")}</span>
                <input type="time" bind:value={settings.dnd_from} onchange={saveSettings} />
                <span>{$t("notification.to")}</span>
                <input type="time" bind:value={settings.dnd_to} onchange={saveSettings} />
              </div>
            {/if}
          </div>

          <!-- 通知类型 -->
          <div class="setting-section">
            <h3>
              <Icon icon="mdi:filter-variant" />
              {$t("notification.notificationTypes")}
            </h3>
            <div class="category-grid">
              {#each allCategories as cat}
                <label
                  class="category-item"
                  class:checked={settings.filter_categories.includes(cat)}
                >
                  <input
                    type="checkbox"
                    checked={settings.filter_categories.includes(cat)}
                    onchange={() => toggleCategory(cat)}
                  />
                  <Icon icon={categoryInfo[cat].icon} style="color: {categoryInfo[cat].color}" />
                  <span>{categoryInfo[cat].name}</span>
                </label>
              {/each}
            </div>
          </div>

          <!-- 严重级别 -->
          <div class="setting-section">
            <h3>
              <Icon icon="mdi:alert-circle-outline" />
              {$t("notification.severityLevels")}
            </h3>
            <div class="severity-grid">
              {#each allSeverities as sev}
                <label
                  class="severity-item"
                  class:checked={settings.filter_severities.includes(sev)}
                >
                  <input
                    type="checkbox"
                    checked={settings.filter_severities.includes(sev)}
                    onchange={() => toggleSeverity(sev)}
                  />
                  <Icon icon={severityInfo[sev].icon} style="color: {severityInfo[sev].color}" />
                  <span>{severityInfo[sev].name}</span>
                </label>
              {/each}
            </div>
          </div>

          <!-- 推送服务 -->
          <div class="setting-section">
            <h3>
              <Icon icon="mdi:send" />
              {$t("notification.pushServices")}
            </h3>
            <div class="channel-list">
              {#if channels.length === 0}
                <div class="empty-channels">
                  <Icon icon="mdi:bell-plus-outline" width="32" />
                  <p>{$t("notification.noPushChannels")}</p>
                </div>
              {:else}
                {#each channels as channel}
                  <div class="channel-item">
                    <div class="channel-icon">
                      <Icon icon={channelTypeInfo[channel.type]?.icon || "mdi:bell"} width="24" />
                    </div>
                    <div class="channel-info">
                      <span class="channel-name">{channel.name}</span>
                      <span class="channel-type"
                        >{channelTypeInfo[channel.type]?.name || channel.type}</span
                      >
                    </div>
                    <div class="channel-actions">
                      <label class="switch small">
                        <input
                          type="checkbox"
                          checked={channel.enabled}
                          onchange={() => toggleChannelEnabled(channel)}
                        />
                        <span class="slider"></span>
                      </label>
                      <button class="icon-btn" onclick={() => testChannel(channel)} title={$t("notification.test")}>
                        <Icon icon="mdi:send" />
                      </button>
                      <button
                        class="icon-btn danger"
                        onclick={() => deleteChannel(channel)}
                        title={$t("notification.delete")}
                      >
                        <Icon icon="mdi:delete" />
                      </button>
                    </div>
                  </div>
                {/each}
              {/if}
            </div>
            <button class="add-channel-btn" onclick={() => (showChannelForm = true)}>
              <Icon icon="mdi:plus" />
              {$t("notification.addPushChannel")}
            </button>
          </div>
        {:else}
          <div class="empty">
            <Icon icon="mdi:loading" width="32" class="spin" />
            <span>{$t("notification.loading")}</span>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .notification-center {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--bg-window, #fafafa);
    color: var(--text-primary, #333);
  }

  .header {
    padding: 12px 16px;
    background: var(--bg-window-header, white);
    border-bottom: 1px solid var(--border-color, #e0e0e0);
  }

  .tabs {
    display: flex;
    gap: 4px;
  }

  .tab {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 16px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--text-secondary, #666);
    font-size: 14px;
    cursor: pointer;
    transition: all 0.15s;

    &:hover {
      background: var(--bg-hover, #f0f0f0);
    }

    &.active {
      background: var(--color-primary, #0066cc);
      color: white;

      .badge {
        background: white;
        color: var(--color-primary, #0066cc);
      }
    }

    .badge {
      min-width: 18px;
      height: 18px;
      padding: 0 6px;
      border-radius: 9px;
      background: #dc3545;
      color: white;
      font-size: 11px;
      font-weight: 600;
      display: flex;
      align-items: center;
      justify-content: center;
    }
  }

  .content {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 16px;
    background: var(--bg-toolbar, #f5f5f5);
    border-bottom: 1px solid var(--border-color, #e0e0e0);
    gap: 12px;
    flex-wrap: wrap;
  }

  .filter-section {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .filter-tabs {
    display: flex;
    gap: 4px;

    button {
      padding: 4px 12px;
      border: 1px solid var(--border-color, #ddd);
      border-radius: 4px;
      background: white;
      font-size: 13px;
      cursor: pointer;
      transition: all 0.15s;

      &:hover {
        background: var(--bg-hover, #f0f0f0);
      }

      &.active {
        background: var(--color-primary, #0066cc);
        color: white;
        border-color: var(--color-primary, #0066cc);
      }
    }
  }

  .actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .select-all {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 13px;
    cursor: pointer;
  }

  .action-btn {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 8px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: var(--text-secondary, #666);
    font-size: 13px;
    cursor: pointer;
    transition: all 0.15s;

    &:hover {
      background: var(--bg-hover, #e0e0e0);
    }

    &.danger {
      color: #dc3545;
      &:hover {
        background: #dc354520;
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
    height: 100%;
    color: var(--text-muted, #999);
    gap: 12px;

    p {
      margin: 0;
      font-size: 16px;
    }

    .hint {
      font-size: 13px;
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
    margin-bottom: 16px;
  }

  .date-header {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 12px;
    font-size: 13px;
    font-weight: 500;
    color: var(--text-secondary, #666);
  }

  .notification-item {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 12px;
    margin: 4px 0;
    background: white;
    border-radius: 8px;
    border: 1px solid var(--border-color, #e8e8e8);
    cursor: pointer;
    transition: all 0.15s;

    &:hover {
      background: var(--bg-hover, #fafafa);
      border-color: var(--border-color-hover, #ddd);

      .delete-btn {
        opacity: 1;
      }
    }

    &.unread {
      background: #f0f7ff;
      border-color: #cce5ff;

      .title {
        font-weight: 600;
      }
    }
  }

  .checkbox-wrapper {
    padding-top: 2px;

    input {
      cursor: pointer;
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
    color: var(--text-primary, #333);
  }

  .time {
    font-size: 12px;
    color: var(--text-muted, #999);
    flex-shrink: 0;
  }

  .message {
    margin: 4px 0;
    font-size: 13px;
    color: var(--text-secondary, #666);
    line-height: 1.5;
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
    color: var(--color-primary, #0066cc);
    text-decoration: none;

    &:hover {
      text-decoration: underline;
    }
  }

  .delete-btn {
    padding: 4px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: var(--text-muted, #999);
    cursor: pointer;
    opacity: 0;
    transition: all 0.15s;

    &:hover {
      background: #dc354520;
      color: #dc3545;
    }
  }

  /* 设置面板 */
  .settings-panel {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
  }

  .setting-section {
    background: white;
    border-radius: 8px;
    border: 1px solid var(--border-color, #e8e8e8);
    margin-bottom: 16px;
    overflow: hidden;

    h3 {
      display: flex;
      align-items: center;
      gap: 8px;
      margin: 0;
      padding: 12px 16px;
      font-size: 14px;
      font-weight: 600;
      background: var(--bg-toolbar, #f8f8f8);
      border-bottom: 1px solid var(--border-color, #e8e8e8);
    }
  }

  .setting-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-color, #f0f0f0);

    &:last-child {
      border-bottom: none;
    }

    &.time-range {
      gap: 8px;

      input[type="time"] {
        padding: 4px 8px;
        border: 1px solid var(--border-color, #ddd);
        border-radius: 4px;
      }
    }
  }

  .setting-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .setting-label {
    font-size: 14px;
    color: var(--text-primary, #333);
  }

  .setting-desc {
    font-size: 12px;
    color: var(--text-muted, #999);
  }

  /* 开关 */
  .switch {
    position: relative;
    display: inline-block;
    width: 44px;
    height: 24px;

    input {
      opacity: 0;
      width: 0;
      height: 0;
    }

    .slider {
      position: absolute;
      cursor: pointer;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background-color: #ccc;
      transition: 0.2s;
      border-radius: 24px;

      &:before {
        position: absolute;
        content: "";
        height: 18px;
        width: 18px;
        left: 3px;
        bottom: 3px;
        background-color: white;
        transition: 0.2s;
        border-radius: 50%;
      }
    }

    input:checked + .slider {
      background-color: var(--color-primary, #0066cc);
    }

    input:checked + .slider:before {
      transform: translateX(20px);
    }

    &.small {
      width: 36px;
      height: 20px;

      .slider:before {
        height: 14px;
        width: 14px;
      }

      input:checked + .slider:before {
        transform: translateX(16px);
      }
    }
  }

  /* 类别和级别网格 */
  .category-grid,
  .severity-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 8px;
    padding: 12px 16px;
  }

  .category-item,
  .severity-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.15s;

    input {
      display: none;
    }

    &:hover {
      background: var(--bg-hover, #f5f5f5);
    }

    &.checked {
      background: var(--color-primary, #0066cc);
      border-color: var(--color-primary, #0066cc);
      color: white;

      :global(svg) {
        color: white !important;
      }
    }
  }

  /* 渠道列表 */
  .channel-list {
    padding: 12px 16px;
  }

  .empty-channels {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 24px;
    color: var(--text-muted, #999);
  }

  .channel-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px;
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 6px;
    margin-bottom: 8px;
  }

  .channel-icon {
    flex-shrink: 0;
    color: var(--text-secondary, #666);
  }

  .channel-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .channel-name {
    font-size: 14px;
    font-weight: 500;
  }

  .channel-type {
    font-size: 12px;
    color: var(--text-muted, #999);
  }

  .channel-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .icon-btn {
    padding: 6px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: var(--text-secondary, #666);
    cursor: pointer;
    transition: all 0.15s;

    &:hover {
      background: var(--bg-hover, #f0f0f0);
    }

    &.danger:hover {
      background: #dc354520;
      color: #dc3545;
    }
  }

  .add-channel-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    width: 100%;
    padding: 10px;
    margin-top: 8px;
    border: 1px dashed var(--border-color, #ccc);
    border-radius: 6px;
    background: transparent;
    color: var(--text-secondary, #666);
    font-size: 14px;
    cursor: pointer;
    transition: all 0.15s;

    &:hover {
      background: var(--bg-hover, #f5f5f5);
      border-color: var(--color-primary, #0066cc);
      color: var(--color-primary, #0066cc);
    }
  }
</style>
