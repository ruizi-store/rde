<script lang="ts">
  import { onMount } from "svelte";
  import Icon from "@iconify/svelte";
  import { t } from "svelte-i18n";
  import { Button, Spinner } from "$shared/ui";
  import {
    dockerStoreService,
    categoryIcons,
    type StoreAppDetail,
  } from "./store-service";

  // ==================== Props ====================
  interface Props {
    appId: string;
    onBack?: () => void;
    onInstall?: (app: StoreAppDetail) => void;
    isInstalled?: boolean;
  }
  let { appId, onBack, onInstall, isInstalled = false }: Props = $props();

  // ==================== 状态 ====================
  let app = $state<StoreAppDetail | null>(null);
  let loading = $state(true);
  let error = $state("");
  let showCompose = $state(false);

  // ==================== 生命周期 ====================
  onMount(async () => {
    await loadApp();
  });

  // ==================== 方法 ====================
  async function loadApp() {
    loading = true;
    error = "";
    try {
      app = await dockerStoreService.getApp(appId);
    } catch (e: any) {
      error = e.message || $t("docker.loadAppDetailFailed");
    } finally {
      loading = false;
    }
  }

  function getCategoryIcon(catId: string): string {
    return categoryIcons[catId] || "mdi:package-variant";
  }

  function formatFieldDefault(val: unknown): string {
    if (val === undefined || val === null) return "";
    return String(val);
  }
</script>

<div class="detail">
  {#if loading}
    <div class="detail-loading">
      <Spinner center />
    </div>
  {:else if error}
    <div class="detail-error">
      <Icon icon="mdi:alert-circle" width="48" />
      <p>{error}</p>
      <Button variant="ghost" size="sm" onclick={onBack}>{$t("common.back")}</Button>
    </div>
  {:else if app}
    <!-- 顶部导航 -->
    <div class="detail-topbar">
      <button class="back-btn" onclick={onBack}>
        <Icon icon="mdi:arrow-left" width="20" />
        {$t("common.back")}
      </button>
    </div>

    <!-- 应用头部 -->
    <div class="detail-header">
      <img
        src={dockerStoreService.getIconUrl(app.icon)}
        alt={app.name}
        class="detail-icon"
        onerror={(e) => { (e.target as HTMLImageElement).style.display = "none"; }}
      />
      <div class="detail-heading">
        <h2 class="detail-title">{app.title || app.name}</h2>
        <p class="detail-desc">{app.description}</p>
        <div class="detail-meta-row">
          <span class="meta-tag">
            <Icon icon={getCategoryIcon(app.category)} width="13" />
            {app.category}
          </span>
          <span class="meta-item">v{app.version}</span>
          {#if app.license}
            <span class="meta-item">
              <Icon icon="mdi:license" width="13" />
              {app.license}
            </span>
          {/if}
          {#if app.author}
            <span class="meta-item">
              <Icon icon="mdi:account" width="13" />
              {app.author}
            </span>
          {/if}
        </div>
      </div>
      <div class="detail-action">
        {#if isInstalled}
          <Button variant="ghost" disabled>
            {$t("docker.installed")}
          </Button>
        {:else}
          <Button variant="primary" onclick={() => onInstall?.(app!)}>
            {$t("docker.install")}
          </Button>
        {/if}
      </div>
    </div>

    <!-- 信息面板 -->
    <div class="detail-body">
      <!-- 基本信息 -->
      <div class="info-section">
        <h3 class="section-title">{$t("docker.basicInfo")}</h3>
        <div class="info-table">
          <div class="info-row">
            <span class="info-label">{$t("docker.name")}</span>
            <span class="info-value">{app.name}</span>
          </div>
          {#if app.architectures?.length}
            <div class="info-row">
              <span class="info-label">{$t("docker.architecture")}</span>
              <span class="info-value">{app.architectures.join(", ")}</span>
            </div>
          {/if}
          {#if app.tags?.length}
            <div class="info-row">
              <span class="info-label">{$t("docker.tags")}</span>
              <span class="info-value">
                {#each app.tags as tag}
                  <span class="mini-tag">{tag}</span>
                {/each}
              </span>
            </div>
          {/if}
          {#if app.homepage}
            <div class="info-row">
              <span class="info-label">{$t("docker.homepage")}</span>
              <a href={app.homepage} target="_blank" rel="noopener noreferrer" class="info-link">
                {app.homepage}
                <Icon icon="mdi:open-in-new" width="12" />
              </a>
            </div>
          {/if}
          {#if app.repository}
            <div class="info-row">
              <span class="info-label">{$t("docker.repository")}</span>
              <a href={app.repository} target="_blank" rel="noopener noreferrer" class="info-link">
                {app.repository}
                <Icon icon="mdi:open-in-new" width="12" />
              </a>
            </div>
          {/if}
        </div>
      </div>

      <!-- 配置表单预览 -->
      {#if app.form?.length}
        <div class="info-section">
          <h3 class="section-title">{$t("docker.configFields")}</h3>
          <p class="section-hint">{$t("docker.configHint")}</p>
          <div class="form-preview">
            {#each app.form as field}
              <div class="form-field-preview">
                <span class="field-name">{field.label?.zh || field.label?.en || field.key}</span>
                <span class="field-meta">
                  <span class="field-type">{field.type}</span>
                  {#if field.required}
                    <span class="field-required">{$t("docker.required")}</span>
                  {/if}
                  {#if field.default !== undefined && field.default !== null}
                    <span class="field-default">{$t("docker.defaultValue")} {formatFieldDefault(field.default)}</span>
                  {/if}
                </span>
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <!-- Compose 预览 -->
      {#if app.compose}
        <div class="info-section">
          <button class="section-toggle" onclick={() => (showCompose = !showCompose)}>
            <h3 class="section-title">Docker Compose</h3>
            <Icon icon={showCompose ? "mdi:chevron-up" : "mdi:chevron-down"} width="20" />
          </button>
          {#if showCompose}
            <pre class="compose-preview">{app.compose}</pre>
          {/if}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .detail {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow-y: auto;
  }

  .detail-loading {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .detail-error {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    color: var(--text-muted, #999);
  }

  /* 顶部导航 */
  .detail-topbar {
    padding: 0 0 12px 0;
  }

  .back-btn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 6px 10px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--text-secondary, #666);
    font-size: 13px;
    cursor: pointer;
    transition: all 0.15s;
  }
  .back-btn:hover {
    background: var(--bg-secondary, #f0f0f0);
    color: var(--text-primary, #333);
  }

  /* 应用头部 */
  .detail-header {
    display: flex;
    gap: 16px;
    align-items: flex-start;
    padding: 16px;
    background: var(--bg-card, white);
    border-radius: 10px;
    margin-bottom: 16px;
  }

  .detail-icon {
    width: 64px;
    height: 64px;
    border-radius: 12px;
    object-fit: contain;
    background: var(--bg-secondary, #f5f5f5);
    flex-shrink: 0;
  }

  .detail-heading {
    flex: 1;
    min-width: 0;
  }

  .detail-title {
    margin: 0;
    font-size: 20px;
    font-weight: 600;
    color: var(--text-primary, #333);
  }

  .detail-desc {
    margin: 4px 0 10px;
    font-size: 13px;
    color: var(--text-muted, #999);
    line-height: 1.5;
  }

  .detail-meta-row {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  .meta-tag {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 3px 8px;
    border-radius: 4px;
    background: var(--bg-secondary, #f0f0f0);
    color: var(--text-secondary, #666);
    font-size: 12px;
  }

  .meta-item {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    font-size: 12px;
    color: var(--text-muted, #999);
  }

  .detail-action {
    flex-shrink: 0;
  }

  /* 信息面板 */
  .detail-body {
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding-bottom: 20px;
  }

  .info-section {
    background: var(--bg-card, white);
    border-radius: 10px;
    padding: 16px;
  }

  .section-title {
    margin: 0 0 10px;
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary, #333);
  }

  .section-hint {
    margin: -4px 0 10px;
    font-size: 12px;
    color: var(--text-muted, #999);
  }

  .section-toggle {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    border: none;
    background: none;
    padding: 0;
    cursor: pointer;
    color: var(--text-primary, #333);
  }
  .section-toggle .section-title {
    margin: 0;
  }

  /* 信息表格 */
  .info-table {
    display: flex;
    flex-direction: column;
    gap: 0;
  }

  .info-row {
    display: flex;
    align-items: flex-start;
    padding: 8px 0;
    border-bottom: 1px solid var(--border-color, #f0f0f0);
  }
  .info-row:last-child {
    border-bottom: none;
  }

  .info-label {
    width: 80px;
    flex-shrink: 0;
    font-size: 13px;
    color: var(--text-muted, #999);
  }

  .info-value {
    flex: 1;
    font-size: 13px;
    color: var(--text-primary, #333);
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }

  .info-link {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    font-size: 13px;
    color: var(--color-primary, #4a90d9);
    text-decoration: none;
    word-break: break-all;
  }
  .info-link:hover {
    text-decoration: underline;
  }

  .mini-tag {
    display: inline-block;
    padding: 1px 6px;
    border-radius: 3px;
    background: var(--bg-secondary, #f0f0f0);
    font-size: 11px;
    color: var(--text-secondary, #666);
  }

  /* 表单预览 */
  .form-preview {
    display: flex;
    flex-direction: column;
    gap: 0;
  }

  .form-field-preview {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 0;
    border-bottom: 1px solid var(--border-color, #f0f0f0);
  }
  .form-field-preview:last-child {
    border-bottom: none;
  }

  .field-name {
    font-size: 13px;
    color: var(--text-primary, #333);
    font-weight: 500;
  }

  .field-meta {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .field-type {
    font-size: 11px;
    padding: 1px 6px;
    border-radius: 3px;
    background: var(--bg-secondary, #f0f0f0);
    color: var(--text-muted, #999);
  }

  .field-required {
    font-size: 11px;
    color: var(--color-error, #e74c3c);
  }

  .field-default {
    font-size: 11px;
    color: var(--text-muted, #999);
  }

  /* Compose 预览 */
  .compose-preview {
    margin: 10px 0 0;
    padding: 14px;
    background: #1a1a2e;
    color: #e0e0e0;
    border-radius: 6px;
    font-size: 12px;
    line-height: 1.6;
    overflow-x: auto;
    white-space: pre;
    font-family: "JetBrains Mono", "Fira Code", monospace;
  }
</style>
