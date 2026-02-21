<script lang="ts">
  import { t } from "svelte-i18n";
  import { Button, Input, Card, Spinner, EmptyState, FolderBrowser } from "$shared/ui";
  import Icon from "@iconify/svelte";
  import { photoService, type Library } from "$shared/services/photos";
  import { fileService } from "$shared/services/files";
  import { api } from "$shared/services/api";

  interface Props {
    onComplete: () => void;
    onSkip: () => void;
  }

  let { onComplete, onSkip }: Props = $props();

  let step = $state<"welcome" | "select" | "creating">("welcome");
  let libraryName = $state($t("photos.myPhotos"));
  let libraryPath = $state("");
  let defaultPicturesPath = $state("");
  let loading = $state(false);
  let error = $state("");
  let existingLibraries = $state<Library[]>([]);
  let showFolderBrowser = $state(false);

  // 检查是否有现有图库
  async function checkExisting() {
    // 加载现有图库列表
    try {
      existingLibraries = await photoService.listLibraries();
    } catch (e) {
      console.error("Failed to check existing libraries", e);
    }
      
    // 获取当前用户信息来确定 Pictures 目录
    try {
      const userResp = await api.get<{ data: { username: string } }>("/users/current");
      if (userResp.data?.username) {
        defaultPicturesPath = `/home/${userResp.data.username}/Pictures`;
        return; // 成功获取，直接返回
      }
    } catch (e) {
      console.error("Failed to get current user", e);
    }

    // 回退方案1：尝试检测 /home 下的用户目录
    try {
      const homeResponse = await fileService.list("/home", false);
      if (homeResponse.data?.content) {
        const userDirs = homeResponse.data.content.filter((f: any) => f.is_dir && !f.name.startsWith("."));
        if (userDirs.length > 0) {
          defaultPicturesPath = `/home/${userDirs[0].name}/Pictures`;
          return;
        }
      }
    } catch (e) {
      console.error("Failed to detect home directories", e);
    }

    // 回退方案2：使用固定默认路径
    defaultPicturesPath = "/home/user/Pictures";
  }

  // 创建图库
  async function createLibrary() {
    if (!libraryPath) {
      error = $t("photos.selectPath");
      return;
    }

    loading = true;
    error = "";
    step = "creating";

    try {
      await photoService.createLibrary(libraryName || $t("photos.myPhotos"), libraryPath, true);
      onComplete();
    } catch (e) {
      error = e instanceof Error ? e.message : $t("photos.createFailed");
      step = "select";
    } finally {
      loading = false;
    }
  }

  // 使用默认路径
  function useDefaultPath() {
    libraryPath = defaultPicturesPath;
    libraryName = $t("photos.defaultName");
  }

  // 跳过设置，使用默认配置
  async function skipSetup() {
    loading = true;
    step = "creating";
    try {
      // 使用检测到的默认图片路径创建图库
      const picturePath = defaultPicturesPath || "/home/user/Pictures";
      await photoService.createLibrary($t("photos.defaultName"), picturePath, true);
      onSkip();
    } catch (e) {
      // 如果创建失败，显示错误并让用户手动选择
      error = e instanceof Error ? e.message : $t("photos.createDefaultFailed");
      step = "select";
      loading = false;
    }
  }

  function handleFolderSelect(path: string) {
    libraryPath = path;
    showFolderBrowser = false;
  }

  $effect(() => {
    checkExisting();
  });
</script>

<div class="setup-container">
  {#if step === "welcome"}
    <div class="setup-card">
      <div class="setup-icon">
        <Icon icon="mdi:image-multiple" width={64} />
      </div>
      <h1>{$t("photos.welcome")}</h1>
      <p class="setup-desc">
        {$t("photos.libraryDesc")}
      </p>

      {#if existingLibraries.length > 0}
        <div class="existing-notice">
          <Icon icon="mdi:information" width={20} />
          <span>{$t("photos.existingLibraries", { values: { n: existingLibraries.length } })}</span>
        </div>
        <div class="setup-actions">
          <Button variant="primary" onclick={() => onComplete()}>
            {$t("photos.continueUsing")}
          </Button>
          <Button variant="ghost" onclick={() => step = "select"}>
            {$t("photos.addNewLibrary")}
          </Button>
        </div>
      {:else}
        <div class="setup-actions">
          <Button variant="primary" onclick={() => step = "select"}>
            <Icon icon="mdi:folder-plus" width={18} />
            {$t("photos.selectLibrary")}
          </Button>
          <Button variant="ghost" onclick={skipSetup}>
            {$t("photos.skipUseDefault")}
          </Button>
        </div>
      {/if}
    </div>

  {:else if step === "select"}
    <div class="setup-card">
      <button class="back-btn" onclick={() => step = "welcome"}>
        <Icon icon="mdi:arrow-left" width={20} />
        {$t("common.back")}
      </button>

      <h2>{$t("photos.setupLibrary")}</h2>
      <p class="setup-desc">{$t("photos.libraryDesc")}</p>

      <div class="form-group">
        <label>{$t("photos.libraryName")}</label>
        <Input bind:value={libraryName} placeholder={$t("photos.myPhotos")} />
      </div>

      <div class="form-group">
        <label>{$t("photos.libraryPath")}</label>
        <div class="path-input">
          <Input bind:value={libraryPath} placeholder="/home/user/Pictures" />
          <Button size="sm" variant="outline" onclick={() => showFolderBrowser = true}>
            <Icon icon="mdi:folder-open" width={16} />
            {$t("photos.browse")}
          </Button>
        </div>
        {#if defaultPicturesPath}
          <button class="default-path-hint" onclick={useDefaultPath}>
            <Icon icon="mdi:lightbulb" width={14} />
            {$t("photos.useSystemPictures")}{defaultPicturesPath}
          </button>
        {/if}
        <span class="hint">{$t("photos.scanHint")}</span>
      </div>

      {#if error}
        <div class="error-msg">
          <Icon icon="mdi:alert-circle" width={16} />
          {error}
        </div>
      {/if}

      <div class="setup-actions">
        <Button variant="primary" onclick={createLibrary} disabled={loading || !libraryPath}>
          {#if loading}
            <Spinner size="sm" />
          {:else}
            <Icon icon="mdi:check" width={18} />
          {/if}
          {$t("photos.setupLibrary")}
        </Button>
        <Button variant="ghost" onclick={skipSetup}>
          {$t("common.skip")}
        </Button>
      </div>
    </div>

  {:else if step === "creating"}
    <div class="setup-card creating">
      <Spinner size="lg" />
      <h2>{$t("photos.creating")}</h2>
      <p>{$t("photos.creatingHint")}</p>
    </div>
  {/if}
</div>

<FolderBrowser 
  bind:open={showFolderBrowser}
  title={$t("photos.selectFolder")}
  initialPath={defaultPicturesPath || "/home"}
  onConfirm={handleFolderSelect}
  onClose={() => showFolderBrowser = false}
/>

<style>
  .setup-container {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 100%;
    background: var(--bg-window);
    padding: 32px;
  }

  .setup-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    max-width: 480px;
    width: 100%;
    padding: 48px 32px;
    background: var(--bg-card);
    border-radius: 16px;
    border: 1px solid var(--border-color);
    text-align: center;
  }

  .setup-card.creating {
    gap: 16px;
  }

  .setup-icon {
    color: var(--color-primary);
    margin-bottom: 24px;
  }

  h1 {
    font-size: 24px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 12px 0;
  }

  h2 {
    font-size: 20px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 12px 0;
  }

  .setup-desc {
    font-size: 14px;
    color: var(--text-secondary);
    line-height: 1.6;
    margin: 0 0 24px 0;
  }

  .existing-notice {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    background: var(--color-primary-light);
    border-radius: 8px;
    color: var(--color-primary);
    font-size: 14px;
    margin-bottom: 24px;
  }

  .setup-actions {
    display: flex;
    flex-direction: column;
    gap: 12px;
    width: 100%;
    max-width: 280px;
  }

  .back-btn {
    position: absolute;
    top: 16px;
    left: 16px;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 8px 12px;
    background: none;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    border-radius: 6px;
    font-size: 14px;
  }

  .back-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .form-group {
    width: 100%;
    text-align: left;
    margin-bottom: 20px;
  }

  .form-group label {
    display: block;
    font-size: 14px;
    font-weight: 500;
    color: var(--text-primary);
    margin-bottom: 8px;
  }

  .path-input {
    display: flex;
    gap: 8px;
  }

  .path-input :global(.input-wrapper) {
    flex: 1;
  }

  .hint {
    display: block;
    font-size: 12px;
    color: var(--text-muted);
    margin-top: 6px;
  }

  .default-path-hint {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 8px;
    padding: 8px 12px;
    background: var(--bg-hover);
    border: none;
    border-radius: 6px;
    color: var(--color-primary);
    font-size: 13px;
    cursor: pointer;
    width: 100%;
    text-align: left;
  }

  .default-path-hint:hover {
    background: var(--color-primary-light);
  }

  .error-msg {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 12px;
    background: rgba(220, 53, 69, 0.1);
    border-radius: 8px;
    color: var(--color-danger);
    font-size: 14px;
    margin-bottom: 16px;
    width: 100%;
  }
</style>
