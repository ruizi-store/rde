<script lang="ts">
  import { onMount } from "svelte";
  import Icon from "@iconify/svelte";
  import { t } from "svelte-i18n";
  import { Button, Spinner, EmptyState } from "$shared/ui";
  import {
    dockerStoreService,
    categoryIcons,
    type StoreAppItem,
    type StoreCategory,
  } from "./store-service";

  // ==================== Props ====================
  interface Props {
    onSelectApp?: (appId: string) => void;
    installedAppIds?: Set<string>;
  }
  let { onSelectApp, installedAppIds = new Set() }: Props = $props();

  // ==================== 状态 ====================
  let apps = $state<StoreAppItem[]>([]);
  let categories = $state<StoreCategory[]>([]);
  let loading = $state(true);
  let error = $state("");
  let selectedCategory = $state("");
  let searchQuery = $state("");
  let searchTimer: ReturnType<typeof setTimeout> | null = null;

  // ==================== 生命周期 ====================

  onMount(async () => {
    await loadData();
  });

  // ==================== 方法 ====================

  async function loadData() {
    loading = true;
    error = "";
    try {
      const [appList, catList] = await Promise.all([
        dockerStoreService.getApps(),
        dockerStoreService.getCategories(),
      ]);
      apps = appList;
      categories = catList.sort((a, b) => b.count - a.count);
    } catch (e: any) {
      error = e.message || $t("docker.loadStoreFailed");
    } finally {
      loading = false;
    }
  }

  async function filterApps() {
    try {
      apps = await dockerStoreService.getApps(
        selectedCategory || undefined,
        searchQuery || undefined,
      );
    } catch {}
  }

  function handleCategoryClick(catId: string) {
    selectedCategory = selectedCategory === catId ? "" : catId;
    filterApps();
  }

  function handleSearchInput(e: Event) {
    const input = e.target as HTMLInputElement;
    searchQuery = input.value;
    if (searchTimer) clearTimeout(searchTimer);
    searchTimer = setTimeout(() => filterApps(), 300);
  }

  function handleAppClick(app: StoreAppItem) {
    if (app.compatible === false) return;
    onSelectApp?.(app.id);
  }

  function getCategoryIcon(catId: string): string {
    return categoryIcons[catId] || "mdi:package-variant";
  }

  function getCategoryName(catId: string): string {
    const cat = categories.find((c) => c.id === catId);
    return cat?.name || catId;
  }
</script>

<div class="store">
  {#if loading}
    <div class="store-loading">
      <Spinner center />
    </div>
  {:else if error}
    <EmptyState icon="mdi:alert-circle" title={$t("docker.loadFailed")} description={error} actionLabel={$t("docker.retry")} onaction={loadData} />
  {:else}
    <!-- 搜索栏 -->
    <div class="search-bar">
      <div class="search-input-wrap">
        <Icon icon="mdi:magnify" width="18" class="search-icon" />
        <input
          type="text"
          placeholder={$t("docker.searchApps")}
          value={searchQuery}
          oninput={handleSearchInput}
          class="search-input"
        />
        {#if searchQuery}
          <button class="search-clear" onclick={() => { searchQuery = ""; filterApps(); }}>
            <Icon icon="mdi:close" width="16" />
          </button>
        {/if}
      </div>
      <span class="app-count">{$t("docker.appCount", { values: { n: apps.length } })}</span>
    </div>

    <!-- 分类标签 -->
    <div class="categories">
      <button
        class="cat-tag"
        class:active={selectedCategory === ""}
        onclick={() => { selectedCategory = ""; filterApps(); }}
      >
        <Icon icon="mdi:apps" width="14" />
        {$t("docker.all")}
      </button>
      {#each categories as cat (cat.id)}
        <button
          class="cat-tag"
          class:active={selectedCategory === cat.id}
          onclick={() => handleCategoryClick(cat.id)}
        >
          <Icon icon={getCategoryIcon(cat.id)} width="14" />
          {cat.name}
          <span class="cat-count">{cat.count}</span>
        </button>
      {/each}
    </div>

    <!-- 应用网格 -->
    {#if apps.length === 0}
      <EmptyState icon="mdi:magnify" title={$t("docker.noAppsFound")} description={$t("docker.tryOtherKeyword")} />
    {:else}
      <div class="app-grid">
        {#each apps as app (app.id)}
          <button class="app-card" class:incompatible={app.compatible === false} onclick={() => handleAppClick(app)}>
            <img
              src={dockerStoreService.getIconUrl(app.icon)}
              alt={app.name}
              class="app-icon"
              onerror={(e) => { (e.target as HTMLImageElement).src = ""; (e.target as HTMLImageElement).style.display = "none"; }}
            />
            <div class="app-info">
              <div class="app-name">
                {app.title || app.name}
                {#if installedAppIds.has(app.id)}
                  <span class="installed-badge">{$t("docker.installed")}</span>
                {/if}
                {#if app.compatible === false}
                  <span class="incompatible-badge">{$t("docker.incompatibleArch")}</span>
                {/if}
              </div>
              <div class="app-desc">{app.description}</div>
              <div class="app-meta">
                <span class="app-tag">
                  <Icon icon={getCategoryIcon(app.category)} width="12" />
                  {getCategoryName(app.category)}
                </span>
                <span class="app-version">v{app.version}</span>
              </div>
            </div>
          </button>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  .store {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .store-loading {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  /* 搜索栏 */
  .search-bar {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 0 0 14px 0;
  }

  .search-input-wrap {
    flex: 1;
    position: relative;
    display: flex;
    align-items: center;
  }

  .search-input-wrap :global(.search-icon) {
    position: absolute;
    left: 10px;
    color: var(--text-muted, #999);
    pointer-events: none;
  }

  .search-input {
    width: 100%;
    padding: 8px 32px 8px 34px;
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 8px;
    font-size: 14px;
    background: var(--bg-input, white);
    color: var(--text-primary, #333);
    outline: none;
    transition: border-color 0.2s;
  }
  .search-input:focus {
    border-color: var(--color-primary, #4a90d9);
  }
  .search-input::placeholder {
    color: var(--text-muted, #bbb);
  }

  .search-clear {
    position: absolute;
    right: 8px;
    background: none;
    border: none;
    color: var(--text-muted, #999);
    cursor: pointer;
    padding: 2px;
    display: flex;
    align-items: center;
  }
  .search-clear:hover {
    color: var(--text-primary, #333);
  }

  .app-count {
    font-size: 13px;
    color: var(--text-muted, #999);
    white-space: nowrap;
  }

  /* 分类标签 */
  .categories {
    display: flex;
    gap: 6px;
    padding: 0 0 14px 0;
    overflow-x: auto;
    scrollbar-width: none;
    flex-shrink: 0;
  }
  .categories::-webkit-scrollbar {
    display: none;
  }

  .cat-tag {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 5px 10px;
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 16px;
    background: var(--bg-card, white);
    color: var(--text-secondary, #666);
    font-size: 12px;
    cursor: pointer;
    white-space: nowrap;
    transition: all 0.15s;
  }
  .cat-tag:hover {
    border-color: var(--color-primary, #4a90d9);
    color: var(--color-primary, #4a90d9);
  }
  .cat-tag.active {
    background: var(--color-primary, #4a90d9);
    border-color: var(--color-primary, #4a90d9);
    color: white;
  }

  .cat-count {
    font-size: 11px;
    opacity: 0.7;
  }

  /* 应用网格 */
  .app-grid {
    flex: 1;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 10px;
    overflow-y: auto;
    padding-bottom: 16px;
  }

  .app-card {
    display: flex;
    gap: 12px;
    padding: 14px;
    background: var(--bg-card, white);
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 10px;
    cursor: pointer;
    transition: all 0.15s;
    text-align: left;
    align-items: flex-start;
  }
  .app-card:hover {
    border-color: var(--color-primary, #4a90d9);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  }

  .app-icon {
    width: 48px;
    height: 48px;
    border-radius: 10px;
    object-fit: contain;
    flex-shrink: 0;
    background: var(--bg-secondary, #f5f5f5);
  }

  .app-info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .app-name {
    font-weight: 600;
    font-size: 14px;
    color: var(--text-primary, #333);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .app-desc {
    font-size: 12px;
    color: var(--text-muted, #999);
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    line-height: 1.4;
  }

  .app-meta {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 4px;
  }

  .app-version {
    font-size: 11px;
    color: var(--text-muted, #bbb);
  }

  .installed-badge {
    display: inline-block;
    padding: 1px 6px;
    border-radius: 3px;
    background: var(--color-success, #27ae60);
    color: white;
    font-size: 10px;
    font-weight: 500;
    margin-left: 6px;
    vertical-align: middle;
  }

  .incompatible-badge {
    display: inline-block;
    padding: 1px 6px;
    border-radius: 3px;
    background: var(--color-warning, #e67e22);
    color: white;
    font-size: 10px;
    font-weight: 500;
    margin-left: 6px;
    vertical-align: middle;
  }

  .app-card.incompatible {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .app-tag {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    padding: 2px 7px;
    border-radius: 4px;
    background: var(--bg-secondary, #f0f0f0);
    color: var(--text-secondary, #666);
    font-size: 11px;
    white-space: nowrap;
  }
</style>
