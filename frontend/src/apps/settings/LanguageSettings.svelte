<script lang="ts">
  import { t } from "svelte-i18n";
  import Icon from "@iconify/svelte";
  import { onMount } from "svelte";
  import { Select, Switch, Spinner } from "$shared/ui";
  import { i18nStore, currentLanguage } from "$lib/i18n/store";
  import { type LanguageCode } from "$lib/i18n";
  import {
    getI18nOptions,
    type I18nOptionsResponse,
  } from "$lib/i18n/api";
  import { toast } from "$shared/stores/toast.svelte";

  // 状态
  let loading = $state(true);
  let saving = $state(false);
  let options = $state<I18nOptionsResponse | null>(null);

  // 默认语言选项（API 失败时使用）
  const defaultLanguages = [
    { code: "zh-CN", name: "简体中文", native_name: "简体中文" },
    { code: "en-US", name: "English", native_name: "English" },
  ];

  // 当前设置 - 响应式跟踪 store
  let language = $state<LanguageCode>("zh-CN");

  // 中国源加速总开关
  let mirrorEnabled = $state(false);

  // 是否展开自定义配置
  let showCustomize = $state(false);

  // 本地镜像状态（从 store 同步）
  let mirrors = $state<Record<string, string>>({});

  // 是否是中文用户
  const isChineseUser = $derived(language === "zh-CN");

  // 统计启用了加速的服务数量
  const enabledCount = $derived(Object.values(mirrors).filter(Boolean).length);
  const totalCount = $derived(
    (options?.services || []).filter(
      (s) => (options?.mirrors?.cn?.[s.id]?.length ?? 0) > 0
    ).length
  );

  // 同步 store 值
  $effect(() => {
    language = $currentLanguage;
  });

  // 从 store 同步镜像设置
  $effect(() => {
    const storeMirrors = $i18nStore.mirrors;
    if (storeMirrors) {
      mirrors = { ...storeMirrors };
      // 判断是否有任何镜像启用（非 follow、非空）
      const hasAny = Object.values(storeMirrors).some(v => v && v !== "follow");
      mirrorEnabled = hasAny;
    }
  });

  // 加载选项
  onMount(async () => {
    try {
      options = await getI18nOptions();
    } catch (e) {
      console.error("Failed to load i18n options, using defaults:", e);
    } finally {
      loading = false;
    }
  });

  // 获取语言列表
  function getLanguages() {
    return options?.languages || defaultLanguages;
  }

  // 更改语言
  async function handleLanguageChange(newLang: string) {
    if (newLang === language) return;
    saving = true;
    try {
      await i18nStore.setLanguage(newLang as LanguageCode);
      language = newLang as LanguageCode;
      toast.success($t("settings.languageUpdated"));
    } catch (e) {
      toast.error($t("settings.languageUpdateFailed"));
    } finally {
      saving = false;
    }
  }

  // 切换总开关
  function handleToggleMirrors(checked: boolean) {
    mirrorEnabled = checked;
    if (checked && options?.services) {
      const newMirrors = { ...mirrors };
      for (const service of options.services) {
        if (!newMirrors[service.id] || newMirrors[service.id] === "follow") {
          const cnMirrors = options.mirrors?.cn?.[service.id];
          if (cnMirrors && cnMirrors.length > 0 && cnMirrors[0].url) {
            newMirrors[service.id] = cnMirrors[0].url;
          }
        }
      }
      mirrors = newMirrors;
      saveMirrors(newMirrors);
    } else if (!checked) {
      mirrors = {};
      showCustomize = false;
      saveMirrors({});
    }
  }

  // 获取服务名称
  function getServiceName(name: Record<string, string>): string {
    return name[language] || name["en-US"] || "";
  }

  // 获取中国镜像选项
  function getChinaMirrorOptions(serviceId: string) {
    const result: { value: string; label: string }[] = [
      { value: "", label: $t("setup.useOfficialSource") },
    ];

    const cnMirrors = options?.mirrors?.cn?.[serviceId];
    if (cnMirrors && cnMirrors.length > 0) {
      for (const m of cnMirrors) {
        if (m.url) {
          result.push({
            value: m.url,
            label: m.name,
          });
        }
      }
    }

    return result;
  }

  // 获取当前镜像设置值
  function getMirrorValue(serviceId: string): string {
    const v = mirrors[serviceId];
    if (!v || v === "follow") return "";
    return v;
  }

  // 更新镜像设置
  function handleMirrorChange(serviceId: string, value: string) {
    if (!value) {
      const newMirrors = { ...mirrors };
      delete newMirrors[serviceId];
      mirrors = newMirrors;
    } else {
      mirrors = { ...mirrors, [serviceId]: value };
    }
    saveMirrors(mirrors);
  }

  // 持久化保存到 store
  function saveMirrors(m: Record<string, string>) {
    const toSave: Record<string, string> = {};
    for (const [key, value] of Object.entries(m)) {
      toSave[key] = value || "follow";
    }
    const hasAny = Object.values(toSave).some(v => v && v !== "follow");
    i18nStore.updateSettings({
      region: hasAny ? "cn" : "intl",
      mirrors: toSave,
    });
    toast.success($t("settings.mirrorUpdated"));
  }
</script>

{#if loading}
  <div class="flex items-center justify-center p-8">
    <Spinner />
  </div>
{:else}
  <div class="language-settings">
    <!-- 语言设置 -->
    <div class="setting-section">
      <div class="section-header">
        <Icon icon="mdi:translate" width="20" />
        <span>{$t("settings.language")}</span>
      </div>
      <div class="section-content">
        <Select
          value={language}
          options={getLanguages().map((l) => ({
            value: l.code,
            label: l.native_name,
          }))}
          onchange={handleLanguageChange}
          disabled={saving}
        />
      </div>
    </div>

    <!-- 软件源设置 - 和 setup step3 统一风格 -->
    {#if isChineseUser}
      <div class="mirror-card" class:mirror-card--enabled={mirrorEnabled}>
        <!-- 卡片头部：开关行 -->
        <div class="mirror-card-header">
          <div class="mirror-card-header-info">
            <span class="mirror-card-icon">{mirrorEnabled ? "🚀" : "🌐"}</span>
            <div class="mirror-card-header-text">
              <span class="mirror-card-title">{$t("setup.enableChinaMirrors")}</span>
              <span class="mirror-card-desc">{$t("setup.enableChinaMirrorsDesc")}</span>
            </div>
          </div>
          <Switch checked={mirrorEnabled} onchange={handleToggleMirrors} />
        </div>

        <!-- 卡片体 -->
        <div class="mirror-card-body">
          {#if mirrorEnabled}
            <!-- 启用时：摘要 + 自定义链接 -->
            <div class="mirror-summary">
              <div class="mirror-summary-row">
                <span class="mirror-summary-check">✓</span>
                <span class="mirror-summary-text">
                  {$t("setup.mirrorEnabledSummary")}
                  {#if totalCount > 0}
                    <span class="mirror-summary-count">({enabledCount}/{totalCount})</span>
                  {/if}
                </span>
              </div>
              <button
                class="mirror-customize-link"
                onclick={() => (showCustomize = !showCustomize)}
              >
                {showCustomize
                  ? $t("setup.collapseMirrors")
                  : $t("setup.customizeMirrors")}
                <svg
                  class="mirror-customize-arrow"
                  class:mirror-customize-arrow--up={showCustomize}
                  width="12"
                  height="12"
                  viewBox="0 0 12 12"
                  fill="none"
                >
                  <path
                    d="M3 4.5L6 7.5L9 4.5"
                    stroke="currentColor"
                    stroke-width="1.5"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  />
                </svg>
              </button>
            </div>

            {#if showCustomize}
              <!-- 详细配置列表 -->
              <div class="mirror-detail-list">
                {#each options?.services || [] as service, i}
                  {@const cnMirrors = options?.mirrors?.cn?.[service.id]}
                  {#if cnMirrors && cnMirrors.length > 0}
                    <div
                      class="mirror-detail-item"
                      style="animation-delay: {i * 30}ms"
                    >
                      <div class="mirror-detail-info">
                        <span
                          class="mirror-detail-status"
                          class:mirror-detail-status--active={!!mirrors[service.id] && mirrors[service.id] !== "follow"}
                        >
                          {mirrors[service.id] && mirrors[service.id] !== "follow" ? "✓" : "○"}
                        </span>
                        <span class="mirror-detail-name"
                          >{getServiceName(service.name)}</span
                        >
                      </div>
                      <Select
                        value={getMirrorValue(service.id)}
                        options={getChinaMirrorOptions(service.id)}
                        onchange={(val) =>
                          handleMirrorChange(service.id, val)}
                        size="sm"
                      />
                    </div>
                  {/if}
                {/each}
              </div>
            {/if}
          {:else}
            <!-- 关闭时：简短说明 -->
            <div class="mirror-disabled-hint">
              <span>{$t("setup.mirrorDisabledSummary")}</span>
            </div>
          {/if}
        </div>
      </div>
    {:else}
      <!-- 非中文用户：显示简单的软件源列表 -->
      <div class="setting-section mirrors-section">
        <div class="section-header">
          <Icon icon="mdi:package-variant" width="20" />
          <span>{$t("settings.softwareSources")}</span>
        </div>
        <p class="section-description">
          {$t("settings.softwareSourcesDescription")}
        </p>
        <div class="section-content">
          <div class="mirror-list-simple">
            {#each options?.services || [] as service}
              <div class="mirror-item-simple">
                <div class="mirror-item-simple-info">
                  <span class="mirror-item-simple-name">{getServiceName(service.name)}</span>
                </div>
                <Select
                  value={getMirrorValue(service.id)}
                  options={getChinaMirrorOptions(service.id)}
                  onchange={(val) => handleMirrorChange(service.id, val)}
                  size="sm"
                />
              </div>
            {/each}
          </div>
        </div>
      </div>
    {/if}
  </div>
{/if}

<style>
  .language-settings {
    display: flex;
    flex-direction: column;
    gap: 2rem;
  }

  .setting-section {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .section-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-weight: 600;
    font-size: 1rem;
  }

  .section-description {
    color: var(--color-text-secondary);
    font-size: 0.875rem;
    margin: 0;
  }

  .section-content {
    margin-top: 0.5rem;
  }

  /* ===== 镜像整合卡片（和 setup step3 统一） ===== */
  .mirror-card {
    border: 1px solid var(--color-border);
    border-radius: 0.75rem;
    overflow: hidden;
    transition: border-color 0.2s, box-shadow 0.2s;
  }

  .mirror-card--enabled {
    border-color: rgba(59, 130, 246, 0.3);
    box-shadow: 0 0 0 1px rgba(59, 130, 246, 0.08);
  }

  .mirror-card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 1rem 1.25rem;
    background: var(--color-surface);
    transition: background 0.2s;
  }

  .mirror-card--enabled .mirror-card-header {
    background: rgba(59, 130, 246, 0.04);
  }

  .mirror-card-header-info {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex: 1;
    min-width: 0;
  }

  .mirror-card-icon {
    font-size: 1.25rem;
    flex-shrink: 0;
    transition: transform 0.2s;
  }

  .mirror-card--enabled .mirror-card-icon {
    transform: scale(1.1);
  }

  .mirror-card-header-text {
    display: flex;
    flex-direction: column;
    gap: 0.125rem;
  }

  .mirror-card-title {
    font-weight: 600;
    font-size: 0.9375rem;
  }

  .mirror-card-desc {
    font-size: var(--text-xs);
    color: var(--color-text-secondary);
  }

  .mirror-card-body {
    border-top: 1px solid var(--color-border);
    padding: 0.75rem 1.25rem;
  }

  .mirror-card--enabled .mirror-card-body {
    border-top-color: rgba(59, 130, 246, 0.15);
  }

  .mirror-summary {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .mirror-summary-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .mirror-summary-check {
    color: var(--color-success);
    font-weight: 600;
    font-size: 0.875rem;
  }

  .mirror-summary-text {
    font-size: var(--text-sm);
    color: var(--color-text-secondary);
  }

  .mirror-summary-count {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
  }

  .mirror-customize-link {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    background: none;
    border: none;
    cursor: pointer;
    color: var(--color-primary);
    font-size: var(--text-xs);
    padding: 0.25rem 0.5rem;
    border-radius: 0.25rem;
    transition: background 0.15s;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .mirror-customize-link:hover {
    background: rgba(59, 130, 246, 0.08);
  }

  .mirror-customize-arrow {
    transition: transform 0.2s;
  }

  .mirror-customize-arrow--up {
    transform: rotate(180deg);
  }

  .mirror-disabled-hint {
    font-size: var(--text-sm);
    color: var(--color-text-tertiary);
  }

  .mirror-detail-list {
    display: flex;
    flex-direction: column;
    gap: 0.375rem;
    margin-top: 0.75rem;
    padding-top: 0.75rem;
    border-top: 1px dashed var(--color-border);
  }

  .mirror-detail-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.5rem 0.625rem;
    border-radius: 0.375rem;
    transition: background 0.15s;
    animation: itemFadeIn 0.2s ease-out both;
  }

  .mirror-detail-item:hover {
    background: rgba(0, 0, 0, 0.02);
  }

  :global(:root[data-theme="dark"]) .mirror-detail-item:hover {
    background: rgba(255, 255, 255, 0.03);
  }

  @keyframes itemFadeIn {
    from {
      opacity: 0;
      transform: translateY(-4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .mirror-detail-info {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-shrink: 0;
  }

  .mirror-detail-status {
    font-size: 0.75rem;
    color: var(--color-text-tertiary);
    width: 1rem;
    text-align: center;
  }

  .mirror-detail-status--active {
    color: var(--color-success);
    font-weight: 600;
  }

  .mirror-detail-name {
    font-size: var(--text-sm);
    font-weight: 500;
    white-space: nowrap;
  }

  /* ===== 非中文用户简单列表 ===== */
  .mirrors-section {
    border: 1px solid var(--color-border);
    border-radius: 8px;
    padding: 1rem;
  }

  .mirror-list-simple {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .mirror-item-simple {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.5rem 0.625rem;
    border: 1px solid var(--color-border);
    border-radius: 8px;
    background: var(--color-surface);
  }

  .mirror-item-simple-info {
    display: flex;
    flex-direction: column;
    gap: 0.125rem;
    flex: 1;
    min-width: 0;
  }

  .mirror-item-simple-name {
    font-size: 0.875rem;
    font-weight: 500;
  }
</style>
