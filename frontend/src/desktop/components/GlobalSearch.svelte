<script lang="ts">
  import { t } from "svelte-i18n";
  import Icon from "@iconify/svelte";
  import { apps, type ExtendedAppDefinition } from "$desktop/stores/apps.svelte";
  import { windows } from "$desktop/stores/windows.svelte";

  let { visible = $bindable(false) }: { visible: boolean } = $props();

  let searchQuery = $state("");
  let searchInput = $state<HTMLInputElement | null>(null);
  let selectedIndex = $state(0);

  // 获取应用的本地化名称
  function getAppDisplayName(app: ExtendedAppDefinition): string {
    const key = `apps.names.${app.id}`;
    const translated = $t(key);
    return translated === key ? app.name : translated;
  }

  interface SearchResult {
    id: string;
    type: "app" | "file" | "setting" | "action";
    title: string;
    description?: string;
    icon: string;
    action: () => void;
  }

  /* 搜索设置项 */
  const settingItems = $derived<Omit<SearchResult, "type">[]>([
    {
      id: "setting-account",
      title: $t("search.accountSettings"),
      description: $t("search.manageAccount"),
      icon: "mdi:account-circle",
      action: () => openSetting("account"),
    },
    {
      id: "setting-network",
      title: $t("search.networkSettings"),
      description: $t("search.configNetwork"),
      icon: "mdi:wifi",
      action: () => openSetting("network"),
    },
    {
      id: "setting-appearance",
      title: $t("search.appearanceSettings"),
      description: $t("search.themeWallpaper"),
      icon: "mdi:palette",
      action: () => openSetting("appearance"),
    },
    {
      id: "setting-security",
      title: $t("search.securitySettings"),
      description: $t("search.passwordPermission"),
      icon: "mdi:shield-lock",
      action: () => openSetting("security"),
    },
  ]);

  /* 快捷操作 */
  const quickActions = $derived<Omit<SearchResult, "type">[]>([
    {
      id: "action-logout",
      title: $t("search.logout"),
      icon: "mdi:logout",
      action: () => console.log("logout"),
    },
    {
      id: "action-restart",
      title: $t("search.restartSystem"),
      icon: "mdi:restart",
      action: () => console.log("restart"),
    },
    {
      id: "action-shutdown",
      title: $t("search.shutdownSystem"),
      icon: "mdi:power",
      action: () => console.log("shutdown"),
    },
  ]);

  function openSetting(section: string) {
    windows.open("settings");
    close();
  }

  /* 搜索结果 */
  let results = $derived.by<SearchResult[]>(() => {
    if (!searchQuery.trim()) {
      /* 默认显示最近使用和快捷操作 */
      return [
        ...apps.list.slice(0, 4).map((app) => ({
          id: app.id,
          type: "app" as const,
          title: getAppDisplayName(app),
          description: $t("search.app"),
          icon: app.icon,
          action: () => {
            windows.open(app.id);
            close();
          },
        })),
        ...quickActions.slice(0, 2).map((a) => ({ ...a, type: "action" as const })),
      ];
    }

    const query = searchQuery.toLowerCase();
    const filtered: SearchResult[] = [];

    /* 搜索应用 */
    apps.list.forEach((app) => {
      const displayName = getAppDisplayName(app);
      if (displayName.toLowerCase().includes(query) || app.name.toLowerCase().includes(query) || app.id.toLowerCase().includes(query)) {
        filtered.push({
          id: app.id,
          type: "app",
          title: displayName,
          description: $t("search.app"),
          icon: app.icon,
          action: () => {
            windows.open(app.id);
            close();
          },
        });
      }
    });

    /* 搜索设置 */
    settingItems.forEach((item) => {
      if (
        item.title.toLowerCase().includes(query) ||
        item.description?.toLowerCase().includes(query)
      ) {
        filtered.push({ ...item, type: "setting" });
      }
    });

    /* 搜索操作 */
    quickActions.forEach((item) => {
      if (item.title.toLowerCase().includes(query)) {
        filtered.push({ ...item, type: "action" });
      }
    });

    return filtered.slice(0, 8);
  });

  function close() {
    visible = false;
    searchQuery = "";
    selectedIndex = 0;
  }

  function handleKeydown(e: KeyboardEvent) {
    switch (e.key) {
      case "Escape":
        close();
        break;
      case "ArrowDown":
        e.preventDefault();
        selectedIndex = Math.min(selectedIndex + 1, results.length - 1);
        break;
      case "ArrowUp":
        e.preventDefault();
        selectedIndex = Math.max(selectedIndex - 1, 0);
        break;
      case "Enter":
        e.preventDefault();
        if (results[selectedIndex]) {
          results[selectedIndex].action();
        }
        break;
    }
  }

  function getTypeLabel(type: SearchResult["type"]): string {
    const labels: Record<string, string> = {
      app: $t("search.app"),
      file: $t("search.file"),
      setting: $t("search.setting"),
      action: $t("search.action"),
    };
    return labels[type];
  }

  /* 打开时聚焦输入框 */
  $effect(() => {
    if (visible && searchInput) {
      setTimeout(() => searchInput?.focus(), 50);
    }
  });

  /* 重置选中索引 */
  $effect(() => {
    searchQuery;
    selectedIndex = 0;
  });
</script>

{#if visible}
  <div class="search-overlay" onclick={close} onkeydown={handleKeydown} role="dialog" tabindex="-1">
    <div class="search-container" onclick={(e) => e.stopPropagation()}>
      <!-- 搜索输入框 -->
      <div class="search-input-wrapper">
        <Icon icon="mdi:magnify" width="24" />
        <input
          type="text"
          bind:this={searchInput}
          bind:value={searchQuery}
          placeholder={$t("search.placeholder")}
          onkeydown={handleKeydown}
        />
        {#if searchQuery}
          <button class="clear-btn" onclick={() => (searchQuery = "")}>
            <Icon icon="mdi:close" width="18" />
          </button>
        {/if}
        <span class="shortcut">ESC</span>
      </div>

      <!-- 搜索结果 -->
      <div class="search-results">
        {#if results.length === 0}
          <div class="no-results">
            <Icon icon="mdi:magnify-close" width="48" />
            <p>{$t("search.noResults")}</p>
          </div>
        {:else}
          {#each results as result, index (result.id)}
            <button
              class="result-item"
              class:selected={index === selectedIndex}
              onclick={result.action}
              onmouseenter={() => (selectedIndex = index)}
            >
              <div class="result-icon" class:is-image={result.type === "app"}>
                {#if result.type === "app"}
                  <img
                    src={result.icon}
                    alt=""
                    onerror={(e) => ((e.currentTarget as HTMLImageElement).style.display = "none")}
                  />
                {:else}
                  <Icon icon={result.icon} width="20" />
                {/if}
              </div>
              <div class="result-content">
                <span class="result-title">{result.title}</span>
                {#if result.description}
                  <span class="result-desc">{result.description}</span>
                {/if}
              </div>
              <span class="result-type">{getTypeLabel(result.type)}</span>
            </button>
          {/each}
        {/if}
      </div>

      <!-- 底部提示 -->
      <div class="search-footer">
        <span><kbd>↑</kbd><kbd>↓</kbd> 导航</span>
        <span><kbd>Enter</kbd> 打开</span>
        <span><kbd>Esc</kbd> 关闭</span>
      </div>
    </div>
  </div>
{/if}

<style>
  .search-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding-top: 15vh;
    z-index: 10001;
    animation: fadeIn 0.15s ease-out;
  }

  .search-container {
    width: 560px;
    background: var(--bg-card, white);
    border-radius: 16px;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
    overflow: hidden;
    animation: slideDown 0.2s ease-out;
  }

  .search-input-wrapper {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-color, #e0e0e0);
    color: var(--text-muted, #888);

    input {
      flex: 1;
      border: none;
      background: none;
      font-size: 18px;
      color: var(--text-primary, #333);
      outline: none;

      &::placeholder {
        color: var(--text-muted, #adb5bd);
      }
    }

    .clear-btn {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 28px;
      height: 28px;
      border: none;
      border-radius: 6px;
      background: var(--bg-tertiary, #f0f0f0);
      color: var(--text-muted, #888);
      cursor: pointer;

      &:hover {
        background: var(--bg-hover, #e0e0e0);
      }
    }

    .shortcut {
      padding: 4px 8px;
      background: var(--bg-tertiary, #f0f0f0);
      border-radius: 4px;
      font-size: 12px;
      color: var(--text-muted, #888);
    }
  }

  .search-results {
    max-height: 400px;
    overflow-y: auto;
    padding: 8px;
  }

  .result-item {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    padding: 12px;
    border: none;
    border-radius: 10px;
    background: transparent;
    cursor: pointer;
    text-align: left;
    transition: background 0.1s;

    &:hover,
    &.selected {
      background: var(--bg-hover, #f5f5f5);
    }

    &.selected {
      background: rgba(74, 144, 217, 0.1);
    }
  }

  .result-icon {
    width: 40px;
    height: 40px;
    border-radius: 10px;
    background: var(--bg-tertiary, #f0f0f0);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted, #666);
    flex-shrink: 0;

    &.is-image {
      background: transparent;
    }

    img {
      width: 32px;
      height: 32px;
      object-fit: contain;
    }
  }

  .result-content {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .result-title {
    font-size: 14px;
    font-weight: 500;
    color: var(--text-primary, #333);
  }

  .result-desc {
    font-size: 12px;
    color: var(--text-muted, #888);
  }

  .result-type {
    font-size: 12px;
    color: var(--text-muted, #aaa);
    padding: 2px 8px;
    background: var(--bg-tertiary, #f0f0f0);
    border-radius: 4px;
  }

  .no-results {
    text-align: center;
    padding: 40px 20px;
    color: var(--text-muted, #888);

    p {
      margin: 12px 0 0;
      font-size: 14px;
    }
  }

  .search-footer {
    display: flex;
    justify-content: center;
    gap: 24px;
    padding: 12px;
    border-top: 1px solid var(--border-color, #e0e0e0);
    font-size: 12px;
    color: var(--text-muted, #888);

    kbd {
      display: inline-block;
      padding: 2px 6px;
      background: var(--bg-tertiary, #f0f0f0);
      border-radius: 4px;
      font-family: inherit;
      margin-right: 4px;
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

  @keyframes slideDown {
    from {
      opacity: 0;
      transform: translateY(-20px) scale(0.98);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }
</style>
