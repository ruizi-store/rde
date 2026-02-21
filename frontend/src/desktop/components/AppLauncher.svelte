<script lang="ts">
  import { apps, type ExtendedAppDefinition } from "$desktop/stores/apps.svelte";
  import { windows } from "$desktop/stores/windows.svelte";
  import { user } from "$shared/stores/user.svelte";
  import { authService } from "$shared/services/auth";
  import { systemService } from "$shared/services/system";
  import { getAvatarUrl } from "$shared/utils/avatar";
  import Icon from "@iconify/svelte";
  import { t } from "svelte-i18n";
  import AppContextMenu from "./AppContextMenu.svelte";

  let { visible = $bindable(false) }: { visible: boolean } = $props();

  let searchQuery = $state("");
  let searchInput = $state<HTMLInputElement | null>(null);
  let showAllApps = $state(false);

  // 确认弹窗状态
  let showConfirmModal = $state(false);
  let confirmAction = $state<"shutdown" | "reboot" | "logout" | null>(null);
  let actionLoading = $state(false);

  // 应用右键菜单
  let contextMenu = $state<{ x: number; y: number; app: ExtendedAppDefinition } | null>(null);

  // 获取应用的本地化名称
  function getAppDisplayName(app: ExtendedAppDefinition): string {
    const key = `apps.names.${app.id}`;
    const translated = $t(key);
    // 如果翻译返回 key 本身，说明没有翻译，使用 fallback
    return translated === key ? app.name : translated;
  }

  // 搜索过滤的应用
  let filteredApps = $derived(
    searchQuery.trim()
      ? apps.list.filter(
          (app) => {
            const displayName = getAppDisplayName(app);
            return displayName.toLowerCase().includes(searchQuery.toLowerCase()) ||
            app.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
            app.id.toLowerCase().includes(searchQuery.toLowerCase()) ||
            (app.keywords && app.keywords.toLowerCase().includes(searchQuery.toLowerCase()));
          }
        )
      : [],
  );

  // 固定的应用 (开始菜单)
  let pinnedApps = $derived(apps.pinnedStartMenuApps);

  // 最近使用的应用
  let recentApps = $derived(apps.recentAppsList);

  // 是否显示搜索结果
  let isSearching = $derived(searchQuery.trim().length > 0);

  // 打开应用
  async function openApp(app: ExtendedAppDefinition) {
    await apps.launch(app.id);
    close();
  }

  // 关闭启动器
  function close() {
    visible = false;
    searchQuery = "";
    showAllApps = false;
  }

  // 返回主视图
  function goBack() {
    showAllApps = false;
  }

  // 显示全部应用
  function showAll() {
    showAllApps = true;
  }

  // 显示应用右键菜单
  function showAppContextMenu(e: MouseEvent, app: ExtendedAppDefinition) {
    e.preventDefault();
    e.stopPropagation();
    contextMenu = { x: e.clientX, y: e.clientY, app };
  }

  // 关闭应用右键菜单
  function closeContextMenu() {
    contextMenu = null;
  }

  // 打开用户设置
  function openUserSettings() {
    close();
    windows.open("settings", { section: "account" });
  }

  // 处理键盘事件
  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      if (showModuleDisabledModal) {
        closeModuleDisabledModal();
      } else if (showConfirmModal) {
        showConfirmModal = false;
        confirmAction = null;
      } else if (showAllApps) {
        showAllApps = false;
      } else {
        close();
      }
    }
  }

  // 显示确认弹窗
  function showConfirm(action: "shutdown" | "reboot" | "logout") {
    confirmAction = action;
    showConfirmModal = true;
  }

  // 执行确认的操作
  async function executeAction() {
    if (!confirmAction) return;

    actionLoading = true;
    try {
      switch (confirmAction) {
        case "shutdown":
          await systemService.shutdown();
          break;
        case "reboot":
          await systemService.reboot();
          break;
        case "logout":
          await authService.logout();
          user.logout();
          window.location.href = "/login";
          break;
      }
    } catch (err) {
      alert(err instanceof Error ? err.message : $t("desktop.launcher.actionFailed"));
    } finally {
      actionLoading = false;
      showConfirmModal = false;
      confirmAction = null;
      close();
    }
  }

  // 获取确认弹窗的标题和描述
  function getConfirmInfo(action: "shutdown" | "reboot" | "logout" | null) {
    switch (action) {
      case "shutdown":
        return {
          title: $t("desktop.launcher.shutdownTitle"),
          desc: $t("desktop.launcher.shutdownDesc"),
          icon: "mdi:power",
          color: "#dc3545",
        };
      case "reboot":
        return {
          title: $t("desktop.launcher.rebootTitle"),
          desc: $t("desktop.launcher.rebootDesc"),
          icon: "mdi:restart",
          color: "#fd7e14",
        };
      case "logout":
        return {
          title: $t("desktop.launcher.logoutTitle"),
          desc: $t("desktop.launcher.logoutDesc"),
          icon: "mdi:logout",
          color: "#6c757d",
        };
      default:
        return { title: "", desc: "", icon: "", color: "" };
    }
  }

  // 打开时聚焦搜索框
  $effect(() => {
    if (visible && searchInput) {
      setTimeout(() => searchInput?.focus(), 100);
    }
  });

  // 开始菜单位置类
  let positionClass = $derived(
    apps.startMenuPosition === "left" ? "position-left" : "position-center",
  );
</script>

{#if visible}
  <div
    class="launcher-overlay {positionClass}"
    onclick={close}
    onkeydown={handleKeydown}
    role="dialog"
    tabindex="-1"
  >
    <div class="launcher" onclick={(e) => e.stopPropagation()}>
      <!-- 搜索栏 -->
      <div class="search-bar">
        <Icon icon="mdi:magnify" width="20" />
        <input
          type="text"
          bind:this={searchInput}
          bind:value={searchQuery}
          placeholder={$t("desktop.launcher.searchPlaceholder")}
          onkeydown={(e) => {
            if (e.key === "Escape") close();
          }}
        />
        {#if searchQuery}
          <button class="clear-btn" onclick={() => (searchQuery = "")}>
            <Icon icon="mdi:close" width="16" />
          </button>
        {/if}
      </div>

      <!-- 主内容区 -->
      <div class="launcher-content">
        {#if isSearching}
          <!-- 搜索结果 -->
          <div class="search-results">
            {#if filteredApps.length === 0}
              <div class="empty">
                <Icon icon="mdi:magnify-close" width="48" />
                <p>{$t("desktop.launcher.noMatchingApps")}</p>
              </div>
            {:else}
              <div class="apps-grid">
                {#each filteredApps as app (app.id)}
                  <button
                    class="app-item"
                    onclick={() => openApp(app)}
                    oncontextmenu={(e) => showAppContextMenu(e, app)}
                  >
                    <div class="app-icon">
                      <img
                        src={app.icon}
                        alt={getAppDisplayName(app)}
                        onerror={(e) =>
                          ((e.currentTarget as HTMLImageElement).src = "/icons/default.svg")}
                      />
                    </div>
                    <span class="app-name">{getAppDisplayName(app)}</span>
                  </button>
                {/each}
              </div>
            {/if}
          </div>
        {:else if showAllApps}
          <!-- 全部应用视图 -->
          <div class="all-apps-view">
            <div class="view-header">
              <button class="back-btn" onclick={goBack}>
                <Icon icon="mdi:arrow-left" width="20" />
                <span>{$t("desktop.launcher.back")}</span>
              </button>
              <h2 class="view-title">{$t("desktop.launcher.allApps")}</h2>
            </div>
            <div class="categories-list">
              {#each apps.sortedCategories as { category, info, apps: categoryApps } (category)}
                <div class="category-section">
                  <h3 class="category-title">
                    <Icon icon={info.icon || "mdi:apps"} width="16" />
                    {info.name}
                  </h3>
                  <div class="category-apps">
                    {#each categoryApps as app (app.id)}
                      <button
                        class="app-row"
                        onclick={() => openApp(app)}
                        oncontextmenu={(e) => showAppContextMenu(e, app)}
                      >
                        <div class="app-icon-small">
                          <img
                            src={app.icon}
                            alt={getAppDisplayName(app)}
                            onerror={(e) =>
                              ((e.currentTarget as HTMLImageElement).src = "/icons/default.svg")}
                          />
                        </div>
                        <span class="app-name">{getAppDisplayName(app)}</span>
                      </button>
                    {/each}
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {:else}
          <!-- 主视图：固定应用 + 推荐 -->
          <div class="main-view">
            <!-- 固定应用区 -->
            <div class="pinned-section">
              <div class="section-header">
                <h3>{$t("desktop.launcher.pinned")}</h3>
                <button class="section-action" onclick={showAll}>
                  {$t("desktop.launcher.allApps")}
                  <Icon icon="mdi:chevron-right" width="18" />
                </button>
              </div>
              <div class="apps-grid pinned-grid">
                {#each pinnedApps as app (app.id)}
                  <button
                    class="app-item"
                    onclick={() => openApp(app)}
                    oncontextmenu={(e) => showAppContextMenu(e, app)}
                  >
                    <div class="app-icon">
                      <img
                        src={app.icon}
                        alt={getAppDisplayName(app)}
                        onerror={(e) =>
                          ((e.currentTarget as HTMLImageElement).src = "/icons/default.svg")}
                      />
                    </div>
                    <span class="app-name">{getAppDisplayName(app)}</span>
                  </button>
                {/each}
                {#if pinnedApps.length === 0}
                  <div class="empty-hint">
                    <p>{$t("desktop.launcher.rightClickToPin")}</p>
                  </div>
                {/if}
              </div>
            </div>

            <!-- 推荐区（最近使用） -->
            <div class="recommended-section">
              <div class="section-header">
                <h3>{$t("desktop.launcher.recentlyUsed")}</h3>
              </div>
              <div class="apps-grid recent-grid">
                {#if recentApps.length > 0}
                  {#each recentApps as app (app.id)}
                    <button
                      class="app-item"
                      onclick={() => openApp(app)}
                      oncontextmenu={(e) => showAppContextMenu(e, app)}
                    >
                      <div class="app-icon">
                        <img
                          src={app.icon}
                          alt={getAppDisplayName(app)}
                          onerror={(e) =>
                            ((e.currentTarget as HTMLImageElement).src = "/icons/default.svg")}
                        />
                      </div>
                      <span class="app-name">{getAppDisplayName(app)}</span>
                    </button>
                  {/each}
                {:else}
                  <div class="empty-hint">
                    <p>{$t("desktop.launcher.recentAppsHint")}</p>
                  </div>
                {/if}
              </div>
            </div>
          </div>
        {/if}
      </div>

      <!-- 底部用户栏 -->
      <div class="launcher-footer">
        <button class="user-info" onclick={openUserSettings} title={$t("desktop.launcher.userSettings")}>
          <div class="user-avatar">
            <img src={getAvatarUrl(user.user)} alt={$t("desktop.launcher.avatar")} class="avatar-img" />
          </div>
          <span class="user-name">{user.user?.username || $t("desktop.launcher.user")}</span>
        </button>
        <div class="power-actions">
          {#if user.isAdmin}
            <button class="power-btn" onclick={() => showConfirm("shutdown")} title={$t("desktop.launcher.shutdown")}>
              <Icon icon="mdi:power" width="18" />
            </button>
            <button class="power-btn" onclick={() => showConfirm("reboot")} title={$t("desktop.launcher.reboot")}>
              <Icon icon="mdi:restart" width="18" />
            </button>
          {/if}
          <button class="power-btn" onclick={() => showConfirm("logout")} title={$t("desktop.launcher.logout")}>
            <Icon icon="mdi:logout" width="18" />
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- 确认弹窗 -->
{#if showConfirmModal}
  {@const info = getConfirmInfo(confirmAction)}
  <div
    class="confirm-overlay"
    onclick={() => {
      showConfirmModal = false;
      confirmAction = null;
    }}
    onkeydown={handleKeydown}
    role="dialog"
    tabindex="-1"
  >
    <div class="confirm-modal" onclick={(e) => e.stopPropagation()}>
      <div class="confirm-icon" style="background: {info.color}20; color: {info.color}">
        <Icon icon={info.icon} width="32" />
      </div>
      <h3 class="confirm-title">{info.title}</h3>
      <p class="confirm-desc">{info.desc}</p>
      <div class="confirm-actions">
        <button
          class="confirm-btn cancel"
          onclick={() => {
            showConfirmModal = false;
            confirmAction = null;
          }}
          disabled={actionLoading}
        >
          {$t("desktop.launcher.cancel")}
        </button>
        <button
          class="confirm-btn confirm"
          style="background: {info.color}"
          onclick={executeAction}
          disabled={actionLoading}
        >
          {#if actionLoading}
            <Icon icon="mdi:loading" class="spin" width="16" />
          {/if}
          {$t("desktop.launcher.confirm")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- 应用右键菜单 -->
{#if contextMenu}
  <AppContextMenu
    x={contextMenu.x}
    y={contextMenu.y}
    app={contextMenu.app}
    context="startmenu"
    onclose={closeContextMenu}
  />
{/if}

<style>
  .launcher-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.2);
    display: flex;
    z-index: 10000;
    animation: fadeIn 0.2s ease-out;

    /* 居中位置（Windows 11 风格） */
    &.position-center {
      align-items: center;
      justify-content: center;
    }

    /* 左下角位置（Windows 10 风格） */
    &.position-left {
      align-items: flex-end;
      justify-content: flex-start;
      padding: 8px;
      padding-bottom: 56px;
    }
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  .launcher {
    width: 600px;
    max-height: 70vh;
    background: var(--bg-launcher, rgba(255, 255, 255, 0.98));
    border-radius: 12px;
    box-shadow:
      0 8px 32px rgba(0, 0, 0, 0.2),
      0 0 0 1px rgba(255, 255, 255, 0.1);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: slideUp 0.25s ease-out;

    @media (prefers-color-scheme: dark) {
      background: rgba(40, 40, 45, 0.98);
      color: #fff;
    }

    .position-left & {
      animation: slideUpLeft 0.25s ease-out;
      transform-origin: bottom left;
    }
  }

  @keyframes slideUp {
    from {
      opacity: 0;
      transform: translateY(20px) scale(0.98);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }

  @keyframes slideUpLeft {
    from {
      opacity: 0;
      transform: translateY(10px) translateX(-10px) scale(0.98);
    }
    to {
      opacity: 1;
      transform: translateY(0) translateX(0) scale(1);
    }
  }

  .search-bar {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    margin: 12px 16px;
    border-radius: 8px;
    background: var(--bg-search, rgba(0, 0, 0, 0.05));
    color: var(--text-muted, #888);

    input {
      flex: 1;
      border: none;
      background: transparent;
      font-size: 14px;
      outline: none;
      color: var(--text-primary, #333);

      &::placeholder {
        color: var(--text-muted, #aaa);
      }
    }

    .clear-btn {
      width: 24px;
      height: 24px;
      border: none;
      border-radius: 50%;
      background: var(--bg-hover, rgba(0, 0, 0, 0.1));
      color: var(--text-muted, #888);
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;

      &:hover {
        background: var(--bg-active, rgba(0, 0, 0, 0.15));
      }
    }
  }

  .launcher-content {
    flex: 1;
    overflow-y: auto;
    padding: 0 16px;
  }

  .main-view {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .pinned-section,
  .recommended-section {
    .section-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 12px;

      h3 {
        font-size: 13px;
        font-weight: 600;
        color: var(--text-primary, #333);
        margin: 0;
      }
    }

    .section-action {
      display: flex;
      align-items: center;
      gap: 4px;
      padding: 4px 8px;
      border: none;
      border-radius: 6px;
      background: transparent;
      color: var(--text-muted, #666);
      font-size: 12px;
      cursor: pointer;

      &:hover {
        background: var(--bg-hover, rgba(0, 0, 0, 0.05));
        color: var(--text-primary, #333);
      }
    }
  }

  .apps-grid {
    display: grid;
    grid-template-columns: repeat(6, 1fr);
    gap: 4px;

    &.pinned-grid {
      min-height: auto;
    }
  }

  .app-item {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    padding: 12px 8px;
    border: none;
    border-radius: 8px;
    background: transparent;
    cursor: pointer;
    transition:
      background 0.15s,
      transform 0.1s;

    &:hover {
      background: var(--bg-hover, rgba(0, 0, 0, 0.05));
    }

    &:active {
      transform: scale(0.95);
    }

    &.disabled {
      opacity: 0.5;

      .app-icon img {
        filter: grayscale(100%);
      }
    }
  }

  .app-icon {
    width: 40px;
    height: 40px;
    border-radius: 8px;
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;

    img {
      width: 100%;
      height: 100%;
      object-fit: contain;
    }

  }

  .app-icon-small {
    width: 32px;
    height: 32px;
    border-radius: 6px;
    overflow: hidden;
    flex-shrink: 0;

    img {
      width: 100%;
      height: 100%;
      object-fit: contain;
    }
  }

  .app-name {
    font-size: 11px;
    color: var(--text-primary, #333);
    text-align: center;
    max-width: 100%;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    word-break: break-word;
    line-height: 1.3;
  }

  .recommended-list {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .recommended-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 12px;
    border: none;
    border-radius: 8px;
    background: transparent;
    cursor: pointer;
    text-align: left;

    &:hover {
      background: var(--bg-hover, rgba(0, 0, 0, 0.05));
    }
  }

  .empty-hint {
    grid-column: 1 / -1;
    padding: 24px;
    text-align: center;
    color: var(--text-muted, #999);
    font-size: 13px;
  }

  .all-apps-view {
    .view-header {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 16px;

      .back-btn {
        display: flex;
        align-items: center;
        gap: 4px;
        padding: 6px 12px;
        border: none;
        border-radius: 6px;
        background: var(--bg-hover, rgba(0, 0, 0, 0.05));
        color: var(--text-primary, #333);
        font-size: 13px;
        cursor: pointer;

        &:hover {
          background: var(--bg-active, rgba(0, 0, 0, 0.1));
        }
      }

      .view-title {
        font-size: 16px;
        font-weight: 600;
        margin: 0;
        color: var(--text-primary, #333);
      }
    }
  }

  .categories-list {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .category-section {
    .category-title {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 12px;
      font-weight: 600;
      color: var(--text-muted, #666);
      text-transform: uppercase;
      letter-spacing: 0.5px;
      margin: 0 0 8px;
      padding-left: 8px;
    }
  }

  .category-apps {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .app-row {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 12px;
    border: none;
    border-radius: 8px;
    background: transparent;
    cursor: pointer;
    text-align: left;
    width: 100%;

    &:hover {
      background: var(--bg-hover, rgba(0, 0, 0, 0.05));
    }

    &.disabled {
      opacity: 0.5;

      .app-icon-small img {
        filter: grayscale(100%);
      }
    }

    .app-name {
      flex: 1;
      text-align: left;
      font-size: 13px;
    }

  }

  .search-results {
    .apps-grid {
      grid-template-columns: repeat(5, 1fr);
    }
  }

  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 48px;
    color: var(--text-muted, #999);
    gap: 12px;

    p {
      margin: 0;
      font-size: 14px;
    }
  }

  .launcher-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 20px;
    border-top: 1px solid var(--border-color, rgba(0, 0, 0, 0.08));
    background: var(--bg-footer, rgba(0, 0, 0, 0.02));
  }

  .user-info {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 10px;
    border: none;
    border-radius: 8px;
    background: transparent;
    cursor: pointer;
    transition: all 0.15s;

    &:hover {
      background: var(--bg-hover, rgba(0, 0, 0, 0.06));
    }

    .user-avatar {
      color: var(--text-muted, #888);
      width: 32px;
      height: 32px;
      border-radius: 50%;
      overflow: hidden;
      flex-shrink: 0;

      .avatar-img {
        width: 100%;
        height: 100%;
        object-fit: cover;
        border-radius: 50%;
      }
    }

    .user-name {
      font-size: 13px;
      font-weight: 500;
      color: var(--text-primary, #333);
    }
  }

  .power-actions {
    display: flex;
    gap: 4px;
  }

  .power-btn {
    width: 36px;
    height: 36px;
    border: none;
    border-radius: 8px;
    background: transparent;
    color: var(--text-muted, #666);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.15s;

    &:hover {
      background: var(--bg-hover, rgba(0, 0, 0, 0.06));
      color: var(--text-primary, #333);
    }
  }

  .confirm-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10001;
    animation: fadeIn 0.15s ease-out;
  }

  .confirm-modal {
    width: 320px;
    background: var(--bg-modal, #fff);
    border-radius: 16px;
    padding: 24px;
    text-align: center;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
    animation: scaleIn 0.2s ease-out;
  }

  @keyframes scaleIn {
    from {
      opacity: 0;
      transform: scale(0.9);
    }
    to {
      opacity: 1;
      transform: scale(1);
    }
  }

  .confirm-icon {
    width: 64px;
    height: 64px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    margin: 0 auto 16px;
  }

  .confirm-title {
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary, #333);
    margin: 0 0 8px;
  }

  .confirm-desc {
    font-size: 14px;
    color: var(--text-secondary, #666);
    margin: 0 0 24px;
    line-height: 1.5;
  }

  .confirm-actions {
    display: flex;
    gap: 12px;
  }

  .confirm-btn {
    flex: 1;
    padding: 10px 16px;
    border: none;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;

    &:disabled {
      opacity: 0.6;
      cursor: not-allowed;
    }

    &.cancel {
      background: var(--bg-hover, #f0f0f0);
      color: var(--text-secondary, #666);

      &:hover:not(:disabled) {
        background: var(--bg-active, #e0e0e0);
      }
    }

    &.confirm {
      color: white;

      &:hover:not(:disabled) {
        filter: brightness(1.1);
      }
    }
  }

  :global(.spin) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }
</style>
