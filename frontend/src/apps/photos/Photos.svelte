<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "svelte-i18n";
  import Icon from "@iconify/svelte";
  import {
    photoService,
    type Photo,
    type Album,
    type Library,
    type TimelineGroup,
    type PhotoStats,
    type AIStatusResponse,
    type FaceSearchParams,
  } from "$shared/services/photos";
  import { Button, Modal, Input, Spinner, EmptyState, Tabs } from "$shared/ui";
  import PhotoGrid from "./PhotoGrid.svelte";
  import PhotoViewer from "./PhotoViewer.svelte";
  import AlbumGrid from "./AlbumGrid.svelte";
  import PhotosSetup from "./PhotosSetup.svelte";
  import PhotosSettings from "./PhotosSettings.svelte";

  let { windowId }: { windowId: string } = $props();

  // 视图状态
  type ViewType = "timeline" | "photos" | "albums" | "favorites" | "archive" | "trash" | "library" | "ai-search";
  let currentView = $state<ViewType>("timeline");
  let currentLibraryId = $state<string>("");
  let currentAlbumId = $state<string>("");

  // 数据
  let photos = $state<Photo[]>([]);
  let timelineGroups = $state<TimelineGroup[]>([]);
  let albums = $state<Album[]>([]);
  let libraries = $state<Library[]>([]);
  let stats = $state<PhotoStats | null>(null);

  // AI 搜索状态
  let aiEnabled = $state(false);
  let aiQuery = $state("");
  let aiSearchType = $state<"semantic" | "face" | "text">("semantic");
  let aiFaceParams = $state<FaceSearchParams>({});
  let aiSearching = $state(false);
  let aiResults = $state<Photo[]>([]);

  // 分页状态
  const PAGE_SIZE = 100;
  let offset = $state(0);
  let hasMore = $state(true);
  let loadingMore = $state(false);

  // 状态
  let loading = $state(false);
  let error = $state("");
  let selectedIds = $state<Set<string>>(new Set());
  let totalPhotos = $state(0);

  // 照片查看器
  let viewerOpen = $state(false);
  let viewerIndex = $state(0);
  let viewerPhotos = $state<Photo[]>([]);

  // 弹窗状态
  let showAddLibraryModal = $state(false);
  let newLibraryName = $state("");
  let newLibraryPath = $state("");
  let showCreateAlbumModal = $state(false);
  let newAlbumName = $state("");
  let newAlbumDesc = $state("");

  // 侧边栏折叠
  let sidebarCollapsed = $state(false);

  // 设置向导和设置页面
  let showSetup = $state(false);
  let showSettings = $state(false);
  let initialized = $state(false);

  // 加载数据
  async function loadData(append = false) {
    if (append) {
      loadingMore = true;
    } else {
      loading = true;
      offset = 0;
      photos = [];
      hasMore = true;
    }
    error = "";
    try {
      switch (currentView) {
        case "timeline":
          const timelineRes = await photoService.getTimeline({ group_by: "day" });
          timelineGroups = timelineRes.groups;
          totalPhotos = timelineRes.total;
          // 展开所有照片供查看器使用
          viewerPhotos = timelineGroups.flatMap(g => g.photos);
          hasMore = false;
          break;

        case "photos":
          const photosRes = await photoService.listPhotos({
            library_id: currentLibraryId || undefined,
            offset: append ? offset : 0,
            limit: PAGE_SIZE,
          });
          if (append) {
            photos = [...photos, ...photosRes.photos];
          } else {
            photos = photosRes.photos;
          }
          totalPhotos = photosRes.total;
          hasMore = photos.length < totalPhotos;
          offset = photos.length;
          viewerPhotos = photos;
          break;

        case "favorites":
          const favRes = await photoService.listPhotos({ 
            favorite: true, 
            offset: append ? offset : 0,
            limit: PAGE_SIZE 
          });
          if (append) {
            photos = [...photos, ...favRes.photos];
          } else {
            photos = favRes.photos;
          }
          totalPhotos = favRes.total;
          hasMore = photos.length < totalPhotos;
          offset = photos.length;
          viewerPhotos = photos;
          break;

        case "archive":
          const archiveRes = await photoService.listPhotos({ 
            archived: true, 
            offset: append ? offset : 0,
            limit: PAGE_SIZE 
          });
          if (append) {
            photos = [...photos, ...archiveRes.photos];
          } else {
            photos = archiveRes.photos;
          }
          totalPhotos = archiveRes.total;
          hasMore = photos.length < totalPhotos;
          offset = photos.length;
          viewerPhotos = photos;
          break;

        case "trash":
          const trashRes = await photoService.listTrash(append ? offset : 0, PAGE_SIZE);
          if (append) {
            photos = [...photos, ...trashRes.photos];
          } else {
            photos = trashRes.photos;
          }
          totalPhotos = trashRes.total;
          hasMore = photos.length < totalPhotos;
          offset = photos.length;
          viewerPhotos = photos;
          break;

        case "albums":
          albums = await photoService.listAlbums();
          hasMore = false;
          break;

        case "library":
          if (currentAlbumId) {
            const albumRes = await photoService.getAlbumPhotos(
              currentAlbumId, 
              append ? offset : 0, 
              PAGE_SIZE
            );
            if (append) {
              photos = [...photos, ...albumRes.photos];
            } else {
              photos = albumRes.photos;
            }
            totalPhotos = albumRes.total;
            hasMore = photos.length < totalPhotos;
            offset = photos.length;
            viewerPhotos = photos;
          }
          break;
      }
    } catch (e) {
      error = e instanceof Error ? e.message : $t("photos.errors.loadFailed");
    } finally {
      loading = false;
      loadingMore = false;
    }
  }

  // 加载更多
  function loadMore() {
    if (!loadingMore && hasMore && !loading) {
      loadData(true);
    }
  }

  // 内容区滚动处理
  function handleScroll(e: Event) {
    const target = e.target as HTMLElement;
    const { scrollTop, scrollHeight, clientHeight } = target;
    
    // 距离底部 200px 时加载更多
    if (scrollTop + clientHeight >= scrollHeight - 200) {
      loadMore();
    }
  }

  // 加载侧边栏数据
  async function loadSidebarData() {
    try {
      [libraries, stats] = await Promise.all([
        photoService.listLibraries(),
        photoService.getStats(),
      ]);
      // 检查 AI 状态
      const aiStatus = await photoService.getAIStatus();
      aiEnabled = aiStatus.enabled;
    } catch (e) {
      console.error("Failed to load sidebar data:", e);
    }
  }

  // AI 搜索
  async function performAISearch() {
    if (!aiQuery && aiSearchType !== "face") return;
    
    aiSearching = true;
    error = "";
    try {
      let response;
      switch (aiSearchType) {
        case "semantic":
          response = await photoService.aiSearch(aiQuery);
          break;
        case "text":
          response = await photoService.aiSearchText(aiQuery);
          break;
        case "face":
          response = await photoService.aiSearchFace(aiFaceParams);
          break;
      }
      aiResults = response.photos;
      viewerPhotos = aiResults;
      totalPhotos = response.total;
    } catch (e) {
      error = e instanceof Error ? e.message : $t("photos.errors.searchFailed");
    } finally {
      aiSearching = false;
    }
  }

  // 切换视图
  function switchView(view: ViewType, libraryId?: string, albumId?: string) {
    currentView = view;
    currentLibraryId = libraryId || "";
    currentAlbumId = albumId || "";
    selectedIds = new Set();
    loadData();
  }

  // 打开照片查看器
  function openViewer(photo: Photo) {
    const index = viewerPhotos.findIndex(p => p.id === photo.id);
    if (index >= 0) {
      viewerIndex = index;
      viewerOpen = true;
    }
  }

  // 选择照片
  function toggleSelect(id: string) {
    const newSet = new Set(selectedIds);
    if (newSet.has(id)) {
      newSet.delete(id);
    } else {
      newSet.add(id);
    }
    selectedIds = newSet;
  }

  // 全选/取消全选
  function toggleSelectAll() {
    if (selectedIds.size === viewerPhotos.length) {
      selectedIds = new Set();
    } else {
      selectedIds = new Set(viewerPhotos.map(p => p.id));
    }
  }

  // 收藏选中
  async function favoriteSelected() {
    if (selectedIds.size === 0) return;
    try {
      await photoService.batchFavorite([...selectedIds], true);
      selectedIds = new Set();
      loadData();
      loadSidebarData();
    } catch (e) {
      error = $t("photos.errors.favoriteFailed");
    }
  }

  // 删除选中
  async function deleteSelected() {
    if (selectedIds.size === 0) return;
    if (!confirm(`确定删除 ${selectedIds.size} 张照片？`)) return;
    try {
      await photoService.batchDelete([...selectedIds], currentView === "trash");
      selectedIds = new Set();
      loadData();
      loadSidebarData();
    } catch (e) {
      error = $t("photos.errors.deleteFailed");
    }
  }

  // 添加图库
  async function addLibrary() {
    if (!newLibraryName || !newLibraryPath) return;
    try {
      await photoService.createLibrary(newLibraryName, newLibraryPath);
      showAddLibraryModal = false;
      newLibraryName = "";
      newLibraryPath = "";
      loadSidebarData();
    } catch (e) {
      error = e instanceof Error ? e.message : $t("photos.errors.addFailed");
    }
  }

  // 创建相册
  async function createAlbum() {
    if (!newAlbumName) return;
    try {
      const album = await photoService.createAlbum(
        newAlbumName,
        newAlbumDesc,
        selectedIds.size > 0 ? [...selectedIds] : undefined
      );
      showCreateAlbumModal = false;
      newAlbumName = "";
      newAlbumDesc = "";
      selectedIds = new Set();
      if (currentView === "albums") {
        loadData();
      }
    } catch (e) {
      error = $t("photos.errors.createFailed");
    }
  }

  // 格式化文件大小
  function formatSize(bytes: number): string {
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
    if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + " MB";
    return (bytes / 1024 / 1024 / 1024).toFixed(2) + " GB";
  }

  async function checkSetup() {
    try {
      const libs = await photoService.listLibraries();
      if (libs.length === 0) {
        showSetup = true;
      } else {
        await loadSidebarData();
        await loadData();
      }
    } catch (e) {
      console.error("Failed to check setup:", e);
    } finally {
      initialized = true;
    }
  }

  function handleSetupComplete() {
    showSetup = false;
    loadSidebarData();
    loadData();
  }

  function handleSetupSkip() {
    showSetup = false;
    loadSidebarData();
    loadData();
  }

  onMount(() => {
    checkSetup();
  });
</script>

<div class="photos-app">
  {#if !initialized}
    <div class="loading-state">
      <Spinner size="lg" />
    </div>
  {:else if showSetup}
    <PhotosSetup onComplete={handleSetupComplete} onSkip={handleSetupSkip} />
  {:else}
  <!-- 侧边栏 -->
  <aside class="sidebar" class:collapsed={sidebarCollapsed}>
    <div class="sidebar-header">
      <button class="toggle-btn" onclick={() => sidebarCollapsed = !sidebarCollapsed}>
        <Icon icon={sidebarCollapsed ? "mdi:menu" : "mdi:menu-open"} width={20} />
      </button>
      {#if !sidebarCollapsed}
        <span class="title">{$t("photos.title")}</span>
        <button class="settings-btn" onclick={() => showSettings = true}>
          <Icon icon="mdi:cog" width={16} />
        </button>
      {/if}
    </div>

    {#if !sidebarCollapsed}
      <nav class="sidebar-nav">
        <!-- 主要导航 -->
        <div class="nav-section">
          <button
            class="nav-item"
            class:active={currentView === "timeline"}
            onclick={() => switchView("timeline")}
          >
            <Icon icon="mdi:timeline" width={20} />
            <span>{$t("photos.views.timeline")}</span>
          </button>
          <button
            class="nav-item"
            class:active={currentView === "photos"}
            onclick={() => switchView("photos")}
          >
            <Icon icon="mdi:image-multiple" width={20} />
            <span>{$t("photos.views.allPhotos")}</span>
            {#if stats}<span class="count">{stats.total_photos}</span>{/if}
          </button>
          <button
            class="nav-item"
            class:active={currentView === "favorites"}
            onclick={() => switchView("favorites")}
          >
            <Icon icon="mdi:heart" width={20} />
            <span>{$t("photos.views.favorites")}</span>
            {#if stats}<span class="count">{stats.favorite_count}</span>{/if}
          </button>
          <button
            class="nav-item"
            class:active={currentView === "albums"}
            onclick={() => switchView("albums")}
          >
            <Icon icon="mdi:folder-image" width={20} />
            <span>{$t("photos.views.albums")}</span>
            {#if stats}<span class="count">{stats.total_albums}</span>{/if}
          </button>
        </div>

        <!-- 图库 -->
        <div class="nav-section">
          <div class="section-header">
            <span>{$t("photos.views.library")}</span>
            <button class="add-btn" onclick={() => showAddLibraryModal = true}>
              <Icon icon="mdi:plus" width={16} />
            </button>
          </div>
          {#each libraries as lib}
            <button
              class="nav-item"
              class:active={currentView === "photos" && currentLibraryId === lib.id}
              onclick={() => switchView("photos", lib.id)}
            >
              <Icon icon="mdi:folder" width={20} />
              <span class="truncate">{lib.name}</span>
              <span class="count">{lib.photo_count}</span>
            </button>
          {/each}
        </div>

        <!-- 其他 -->
        <div class="nav-section">
          <button
            class="nav-item"
            class:active={currentView === "archive"}
            onclick={() => switchView("archive")}
          >
            <Icon icon="mdi:archive" width={20} />
            <span>{$t("photos.views.archive")}</span>
            {#if stats}<span class="count">{stats.archived_count}</span>{/if}
          </button>
          <button
            class="nav-item"
            class:active={currentView === "trash"}
            onclick={() => switchView("trash")}
          >
            <Icon icon="mdi:delete" width={20} />
            <span>{$t("photos.views.trash")}</span>
            {#if stats}<span class="count">{stats.trash_count}</span>{/if}
          </button>
        </div>

        <!-- AI 智能搜索 -->
        {#if aiEnabled}
          <div class="nav-section">
            <div class="section-title">AI</div>
            <button
              class="nav-item"
              class:active={currentView === "ai-search"}
              onclick={() => switchView("ai-search")}
            >
              <Icon icon="mdi:brain" width={20} />
              <span>{$t("photos.views.aiSearch")}</span>
            </button>
          </div>
        {/if}

        <!-- 存储统计 -->
        {#if stats}
          <div class="storage-info">
            <Icon icon="mdi:harddisk" width={16} />
            <span>{formatSize(stats.total_size)}</span>
          </div>
        {/if}
      </nav>
    {/if}
  </aside>

  <!-- 主内容区 -->
  <main class="main-content">
    <!-- 工具栏 -->
    <header class="toolbar">
      <div class="toolbar-left">
        <h2 class="view-title">
          {#if currentView === "timeline"}{$t("photos.views.timeline")}
          {:else if currentView === "photos"}{$t("photos.views.allPhotos")}
          {:else if currentView === "favorites"}{$t("photos.views.favorites")}
          {:else if currentView === "albums"}{$t("photos.views.albums")}
          {:else if currentView === "archive"}{$t("photos.views.archive")}
          {:else if currentView === "trash"}{$t("photos.views.trash")}
          {:else if currentView === "ai-search"}{$t("photos.views.aiSearch")}
          {/if}
          {#if totalPhotos > 0}
            <span class="photo-count">({totalPhotos})</span>
          {/if}
        </h2>
      </div>

      <div class="toolbar-right">
        {#if selectedIds.size > 0}
          <span class="selected-count">{$t("photos.selection.selected", { values: { n: selectedIds.size } })}</span>
          <Button size="sm" variant="ghost" onclick={toggleSelectAll}>
            {selectedIds.size === viewerPhotos.length ? $t("photos.selection.deselectAll") : $t("photos.selection.selectAll")}
          </Button>
          <Button size="sm" variant="ghost" onclick={favoriteSelected}>
            <Icon icon="mdi:heart" width={16} />
            {$t("photos.views.favorites")}
          </Button>
          <Button size="sm" variant="ghost" onclick={() => showCreateAlbumModal = true}>
            <Icon icon="mdi:folder-plus" width={16} />
            {$t("photos.addLibrary")}
          </Button>
          <Button size="sm" variant="ghost" onclick={deleteSelected}>
            <Icon icon="mdi:delete" width={16} />
            {$t("common.delete")}
          </Button>
        {:else}
          {#if currentView === "albums"}
            <Button size="sm" onclick={() => showCreateAlbumModal = true}>
              <Icon icon="mdi:plus" width={16} />
              {$t("photos.empty.newAlbum")}
            </Button>
          {/if}
          <Button size="sm" variant="ghost" onclick={() => loadData()}>
            <Icon icon="mdi:refresh" width={16} />
          </Button>
        {/if}
      </div>
    </header>

    <!-- 内容区 -->
    <div class="content" onscroll={handleScroll}>
      {#if loading}
        <div class="loading-state">
          <Spinner size="lg" />
          <span>{$t("photos.loading")}</span>
        </div>
      {:else if error}
        <div class="error-state">
          <Icon icon="mdi:alert-circle" width={48} />
          <span>{error}</span>
          <Button onclick={() => loadData()}>{$t("photos.retry")}</Button>
        </div>
      {:else if currentView === "timeline"}
        <!-- 时间线视图 -->
        {#if timelineGroups.length === 0}
          <EmptyState
            icon="mdi:image-off"
            title={$t("photos.empty.noPhotos")}
            description={$t("photos.empty.addLibraryToStart")}
          />
        {:else}
          <div class="timeline">
            {#each timelineGroups as group}
              <div class="timeline-group">
                <div class="timeline-date">
                  <span class="date">{group.date}</span>
                  <span class="count">{group.count} {$t("photos.photos")}</span>
                </div>
                <PhotoGrid
                  photos={group.photos}
                  {selectedIds}
                  onSelect={toggleSelect}
                  onClick={openViewer}
                />
              </div>
            {/each}
          </div>
        {/if}
      {:else if currentView === "albums"}
        <!-- 相册视图 -->
        {#if albums.length === 0}
          <EmptyState
            icon="mdi:folder-image"
            title={$t("photos.empty.noAlbums")}
            description={$t("photos.empty.createAlbumToOrganize")}
            actionLabel={$t("photos.empty.newAlbum")}
            onaction={() => showCreateAlbumModal = true}
          />
        {:else}
          <AlbumGrid
            {albums}
            onOpen={(album) => switchView("library", undefined, album.id)}
          />
        {/if}
      {:else if currentView === "ai-search"}
        <!-- AI 搜索视图 -->
        <div class="ai-search-container">
          <div class="ai-search-form">
            <div class="search-type-tabs">
              <button
                class="tab-btn"
                class:active={aiSearchType === "semantic"}
                onclick={() => aiSearchType = "semantic"}
              >
                <Icon icon="mdi:auto-fix" width={16} />
                {$t("photos.aiSearch.semanticSearch")}
              </button>
              <button
                class="tab-btn"
                class:active={aiSearchType === "face"}
                onclick={() => aiSearchType = "face"}
              >
                <Icon icon="mdi:face-recognition" width={16} />
                {$t("photos.aiSearch.faceSearch")}
              </button>
              <button
                class="tab-btn"
                class:active={aiSearchType === "text"}
                onclick={() => aiSearchType = "text"}
              >
                <Icon icon="mdi:text-search" width={16} />
                {$t("photos.aiSearch.textSearch")}
              </button>
            </div>
            
            {#if aiSearchType === "semantic"}
              <div class="search-input-group">
                <Input
                  placeholder={$t("photos.aiSearch.semanticPlaceholder")}
                  bind:value={aiQuery}
                  onkeydown={(e: KeyboardEvent) => e.key === "Enter" && performAISearch()}
                />
                <Button onclick={performAISearch} disabled={aiSearching || !aiQuery}>
                  {#if aiSearching}
                    <Spinner size="sm" />
                  {:else}
                    <Icon icon="mdi:magnify" width={18} />
                  {/if}
                  {$t("common.search")}
                </Button>
              </div>
              <p class="search-hint">{$t("photos.aiSearch.semanticHint")}</p>
            {:else if aiSearchType === "face"}
              <div class="face-search-form">
                <div class="form-row">
                  <label>{$t("photos.aiSearch.ageRange")}</label>
                  <input
                    type="number"
                    class="number-input"
                    placeholder={$t("photos.aiSearch.ageMin")}
                    min="0"
                    max="100"
                    bind:value={aiFaceParams.age_min}
                  />
                  <span>-</span>
                  <input
                    type="number"
                    class="number-input"
                    placeholder={$t("photos.aiSearch.ageMax")}
                    min="0"
                    max="100"
                    bind:value={aiFaceParams.age_max}
                  />
                </div>
                <div class="form-row">
                  <label>{$t("photos.aiSearch.gender")}</label>
                  <select bind:value={aiFaceParams.gender}>
                    <option value="">{$t("photos.aiSearch.genderAll")}</option>
                    <option value="male">{$t("photos.aiSearch.genderMale")}</option>
                    <option value="female">{$t("photos.aiSearch.genderFemale")}</option>
                  </select>
                </div>
                <Button onclick={performAISearch} disabled={aiSearching}>
                  {#if aiSearching}
                    <Spinner size="sm" />
                  {:else}
                    <Icon icon="mdi:magnify" width={18} />
                  {/if}
                  {$t("photos.aiSearch.searchFace")}
                </Button>
              </div>
            {:else}
              <div class="search-input-group">
                <Input
                  placeholder={$t("photos.aiSearch.textPlaceholder")}
                  bind:value={aiQuery}
                  onkeydown={(e: KeyboardEvent) => e.key === "Enter" && performAISearch()}
                />
                <Button onclick={performAISearch} disabled={aiSearching || !aiQuery}>
                  {#if aiSearching}
                    <Spinner size="sm" />
                  {:else}
                    <Icon icon="mdi:magnify" width={18} />
                  {/if}
                  {$t("common.search")}
                </Button>
              </div>
              <p class="search-hint">{$t("photos.aiSearch.textHint")}</p>
            {/if}
          </div>
          
          {#if aiSearching}
            <div class="loading-state">
              <Spinner size="lg" />
              <span>{$t("photos.aiSearch.searching")}</span>
            </div>
          {:else if aiResults.length > 0}
            <PhotoGrid
              photos={aiResults}
              {selectedIds}
              onSelect={toggleSelect}
              onClick={openViewer}
            />
          {:else if aiQuery || (aiFaceParams.age_min || aiFaceParams.age_max || aiFaceParams.gender)}
            <EmptyState
              icon="mdi:image-search-outline"
              title={$t("photos.aiSearch.noResults")}
              description={$t("photos.aiSearch.tryDifferent")}
            />
          {:else}
            <EmptyState
              icon="mdi:brain"
              title={$t("photos.aiSearch.title")}
              description={$t("photos.aiSearch.description")}
            />
          {/if}
        </div>
      {:else}
        <!-- 照片网格视图 -->
        {#if photos.length === 0}
          <EmptyState
            icon="mdi:image-off"
            title={$t("photos.empty.noPhotos")}
            description={currentView === "trash" ? $t("photos.empty.trashEmpty") : $t("photos.empty.noPhotosHere")}
          />
        {:else}
          <PhotoGrid
            {photos}
            {selectedIds}
            onSelect={toggleSelect}
            onClick={openViewer}
          />
          <!-- 加载更多指示器 -->
          {#if hasMore || loadingMore}
            <div class="load-more">
              {#if loadingMore}
                <Spinner size="sm" />
                <span>{$t("photos.loadMore")}</span>
              {:else}
                <span class="load-more-hint">{$t("photos.scrollToLoadMore")}</span>
              {/if}
            </div>
          {/if}
        {/if}
      {/if}
    </div>
  </main>

  <!-- 照片查看器 -->
  {#if viewerOpen}
    <PhotoViewer
      photos={viewerPhotos}
      initialIndex={viewerIndex}
      onClose={() => viewerOpen = false}
      onFavorite={async (photo) => {
        await photoService.updatePhoto(photo.id, { is_favorite: !photo.is_favorite });
        loadData();
        loadSidebarData();
      }}
      onDelete={async (photo) => {
        await photoService.deletePhoto(photo.id);
        viewerOpen = false;
        loadData();
        loadSidebarData();
      }}
    />
  {/if}

  <!-- 添加图库弹窗 -->
  <Modal
    open={showAddLibraryModal}
    title={$t("photos.modal.addLibrary")}
    onclose={() => showAddLibraryModal = false}
    showFooter={true}
  >
    <div class="modal-form">
      <Input
        label={$t("photos.modal.name")}
        bind:value={newLibraryName}
        placeholder={$t("photos.modal.myPhotos")}
      />
      <Input
        label={$t("photos.path")}
        bind:value={newLibraryPath}
        placeholder="/home/user/Pictures"
      />
    </div>
    {#snippet footer()}
      <Button variant="ghost" onclick={() => showAddLibraryModal = false}>{$t("common.cancel")}</Button>
      <Button onclick={addLibrary}>{$t("common.add")}</Button>
    {/snippet}
  </Modal>

  <!-- 创建相册弹窗 -->
  <Modal
    open={showCreateAlbumModal}
    title={$t("photos.empty.newAlbum")}
    onclose={() => showCreateAlbumModal = false}
    showFooter={true}
  >
    <div class="modal-form">
      <Input
        label={$t("photos.modal.name")}
        bind:value={newAlbumName}
        placeholder={$t("photos.views.albums")}
      />
      <Input
        label={$t("photos.settings.description") || "描述"}
        bind:value={newAlbumDesc}
        placeholder={$t("common.optional") || "可选"}
      />
      {#if selectedIds.size > 0}
        <p class="select-hint">{$t("photos.selection.selected", { values: { n: selectedIds.size } })}</p>
      {/if}
    </div>
    {#snippet footer()}
      <Button variant="ghost" onclick={() => showCreateAlbumModal = false}>{$t("common.cancel")}</Button>
      <Button onclick={createAlbum}>{$t("common.add")}</Button>
    {/snippet}
  </Modal>

  <!-- 设置弹窗 -->
  {#if showSettings}
    <div class="settings-overlay">
      <PhotosSettings onClose={() => { showSettings = false; loadSidebarData(); }} />
    </div>
  {/if}
  {/if}
</div>

<style>
  .photos-app {
    display: flex;
    width: 100%;
    height: 100%;
    background: var(--bg-window);
    color: var(--text-primary);
  }

  /* 侧边栏 */
  .sidebar {
    width: 220px;
    background: var(--bg-sidebar);
    border-right: 1px solid var(--border-color);
    display: flex;
    flex-direction: column;
    transition: width 0.2s ease;
  }

  .sidebar.collapsed {
    width: 48px;
  }

  .sidebar-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px;
    border-bottom: 1px solid var(--border-color);
  }

  .toggle-btn {
    background: none;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
  }

  .toggle-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .title {
    font-weight: 600;
    font-size: 14px;
    flex: 1;
  }

  .settings-btn {
    background: none;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
  }

  .settings-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .sidebar-nav {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
  }

  .nav-section {
    margin-bottom: 16px;
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px 4px;
    font-size: 12px;
    color: var(--text-muted);
    text-transform: uppercase;
  }

  .add-btn {
    background: none;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    padding: 2px;
    border-radius: 4px;
  }

  .add-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 8px 12px;
    background: none;
    border: none;
    border-radius: 6px;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 13px;
    text-align: left;
  }

  .nav-item:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .nav-item.active {
    background: var(--color-primary);
    color: #fff;
  }

  .nav-item .count {
    margin-left: auto;
    font-size: 11px;
    color: var(--text-muted);
  }

  .nav-item.active .count {
    color: rgba(255, 255, 255, 0.7);
  }

  .truncate {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
    min-width: 0;
  }

  .storage-info {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px;
    margin-top: auto;
    font-size: 12px;
    color: var(--text-muted);
    border-top: 1px solid var(--border-color);
  }

  /* 主内容区 */
  .main-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-color);
    background: var(--bg-sidebar);
  }

  .toolbar-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .view-title {
    font-size: 16px;
    font-weight: 600;
    margin: 0;
  }

  .photo-count {
    font-weight: 400;
    color: var(--text-secondary);
    font-size: 14px;
  }

  .toolbar-right {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .selected-count {
    font-size: 13px;
    color: var(--color-primary);
    font-weight: 500;
  }

  .content {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
  }

  /* 状态 */
  .loading-state,
  .error-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 16px;
    color: var(--text-secondary);
  }

  /* 时间线 */
  .timeline {
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .timeline-group {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .timeline-date {
    display: flex;
    align-items: baseline;
    gap: 8px;
    padding: 0 4px;
  }

  .timeline-date .date {
    font-size: 16px;
    font-weight: 600;
  }

  .timeline-date .count {
    font-size: 12px;
    color: var(--text-secondary);
  }

  /* 弹窗表单 */
  .modal-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 16px 0;
  }

  .select-hint {
    font-size: 13px;
    color: var(--text-secondary);
    margin: 0;
  }

  /* AI 搜索 */
  .ai-search-container {
    display: flex;
    flex-direction: column;
    gap: 24px;
    height: 100%;
  }

  .ai-search-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 16px;
    background: var(--bg-card);
    border-radius: 8px;
  }

  .search-type-tabs {
    display: flex;
    gap: 8px;
  }

  .tab-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 16px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.2s;
  }

  .tab-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .tab-btn.active {
    background: var(--color-primary);
    color: white;
  }

  .search-input-group {
    display: flex;
    gap: 8px;
  }

  .search-input-group :global(.input) {
    flex: 1;
  }

  .search-hint {
    font-size: 12px;
    color: var(--text-secondary);
    margin: 0;
  }

  .face-search-form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .form-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .form-row label {
    min-width: 70px;
    font-size: 14px;
    color: var(--text-secondary);
  }

  .form-row :global(.input) {
    width: 100px;
  }

  .form-row select {
    padding: 8px 12px;
    border: 1px solid var(--border-color);
    border-radius: 6px;
    background: var(--bg-window);
    color: var(--text-primary);
    font-size: 14px;
  }

  .number-input {
    width: 80px;
    padding: 8px 12px;
    border: 1px solid var(--border-color);
    border-radius: 6px;
    background: var(--bg-window);
    color: var(--text-primary);
    font-size: 14px;
  }

  .number-input:focus {
    outline: none;
    border-color: var(--color-primary);
  }

  /* 加载更多 */
  .load-more {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 24px;
    color: var(--text-secondary);
    font-size: 13px;
  }

  .load-more-hint {
    opacity: 0.6;
  }

  /* 设置覆盖层 */
  .settings-overlay {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 100;
  }
</style>
