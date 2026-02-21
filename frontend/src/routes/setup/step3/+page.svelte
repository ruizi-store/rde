<script lang="ts">
  import { t } from "svelte-i18n";
  import { goto } from "$app/navigation";
  import { onMount } from "svelte";
  import { Button, Select, Switch, Spinner } from "$shared/ui";
  import { setupApi } from "$shared/services/setup";
  import { i18nStore, currentLanguage } from "$lib/i18n/store";
  import {
    getI18nOptions,
    type I18nOptionsResponse,
    type MirrorOption,
  } from "$lib/i18n/api";
  import { api } from "$shared/services/api";
  import SetupCard from "../SetupCard.svelte";

  let completing = $state(false);
  let completed = $state(false);
  let error = $state("");
  let countdown = $state(5);

  // 软件源设置
  let loadingOptions = $state(true);
  let options = $state<I18nOptionsResponse | null>(null);
  let mirrors = $state<Record<string, string>>({});
  let language = $state("zh-CN");

  // 是否是中文用户
  const isChineseUser = $derived(language === "zh-CN");

  // 中国源加速总开关
  let mirrorEnabled = $state(false);

  // 是否展开自定义配置
  let showCustomize = $state(false);

  // 统计启用了加速的服务数量
  const enabledCount = $derived(Object.values(mirrors).filter(Boolean).length);
  const totalCount = $derived(
    (options?.services || []).filter(
      (s) => (options?.mirrors?.cn?.[s.id]?.length ?? 0) > 0
    ).length
  );

  // 同步语言
  $effect(() => {
    language = $currentLanguage;
  });

  onMount(async () => {
    try {
      const optionsData = await getI18nOptions();
      options = optionsData;

      // 如果是中文用户，默认开启镜像加速并选择默认中国镜像
      if (language === "zh-CN" && optionsData.services) {
        mirrorEnabled = true;
        const defaultMirrors: Record<string, string> = {};
        for (const service of optionsData.services) {
          const cnMirrors = optionsData.mirrors?.cn?.[service.id];
          if (cnMirrors && cnMirrors.length > 0 && cnMirrors[0].url) {
            defaultMirrors[service.id] = cnMirrors[0].url;
          }
        }
        mirrors = defaultMirrors;
      }
    } catch (e) {
      console.error("Failed to load options:", e);
    } finally {
      loadingOptions = false;
    }
  });

  // 切换总开关
  function handleToggleMirrors(checked: boolean) {
    mirrorEnabled = checked;
    if (checked && options?.services) {
      const newMirrors = { ...mirrors };
      for (const service of options.services) {
        if (!newMirrors[service.id]) {
          const cnMirrors = options.mirrors?.cn?.[service.id];
          if (cnMirrors && cnMirrors.length > 0 && cnMirrors[0].url) {
            newMirrors[service.id] = cnMirrors[0].url;
          }
        }
      }
      mirrors = newMirrors;
    } else if (!checked) {
      mirrors = {};
      showCustomize = false;
    }
  }

  // 获取服务名称
  function getServiceName(name: Record<string, string>): string {
    return name[language] || name["en-US"] || "";
  }

  // 获取镜像简称（不含 URL）
  function getMirrorShortName(serviceId: string): string {
    const url = mirrors[serviceId];
    if (!url) return "";
    const cnMirrors = options?.mirrors?.cn?.[serviceId];
    if (cnMirrors) {
      const found = cnMirrors.find((m) => m.url === url);
      if (found) return found.name;
    }
    return formatURL(url);
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

  // 格式化URL
  function formatURL(url: string): string {
    if (!url) return "";
    try {
      const u = new URL(url);
      return u.hostname;
    } catch {
      return url;
    }
  }

  // 获取当前镜像设置值
  function getMirrorValue(serviceId: string): string {
    return mirrors[serviceId] || "";
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
  }

  async function completeSetup() {
    completing = true;
    error = "";

    try {
      const mirrorsToSave: Record<string, string> = {};
      if (mirrorEnabled) {
        for (const [key, value] of Object.entries(mirrors)) {
          if (value) {
            mirrorsToSave[key] = value;
          }
        }
      }

      const region =
        mirrorEnabled && Object.keys(mirrorsToSave).length > 0
          ? "cn"
          : "intl";

      await i18nStore.updateSettings({
        region: region,
        mirrors: mirrorsToSave,
      });

      const response = await setupApi.complete();
      completed = true;

      // 如果有自动登录 token，保存并验证后再跳转
      if (response.auto_login_token) {
        // 使用 api.setToken() 更新 ApiClient 内部状态
        api.setToken(response.auto_login_token);
        // 保存 refresh_token
        if (response.refresh_token && typeof window !== "undefined") {
          localStorage.setItem("refresh_token", response.refresh_token);
        }
        
        // 验证 token 是否有效
        try {
          await api.get("/users/current");
          // token 有效，跳转到首页
          console.log("Auto-login token validated successfully");
          setTimeout(() => {
            goto("/");
          }, 1500);
        } catch (e) {
          // token 验证失败，清除并跳转到登录页
          console.error("Auto-login token validation failed:", e);
          api.setToken(null);
          localStorage.removeItem("refresh_token");
          setTimeout(() => {
            goto("/login");
          }, 1500);
        }
        return;
      }

      // 否则跳转到登录页
      const redirectUrl = response.redirect_url || "/login";
      const timer = setInterval(() => {
        countdown--;
        if (countdown <= 0) {
          clearInterval(timer);
          goto(redirectUrl);
        }
      }, 1000);
    } catch (e) {
      error = e instanceof Error ? e.message : $t("setup.completeFailed");
      completing = false;
    }
  }

  function goToDashboard() {
    goto("/login");
  }

  function goBack() {
    goto("/setup/step2");
  }
</script>

{#snippet footer()}
  <Button variant="secondary" onclick={goBack}>{$t("setup.prevStep")}</Button>
  <Button
    variant="success"
    onclick={completeSetup}
    loading={completing}
    disabled={loadingOptions}
  >
    {#if completing}
      {$t("setup.completing")}
    {:else}
      🚀 {$t("setup.startUsingRDE")}
    {/if}
  </Button>
{/snippet}

<SetupCard
  header={completed
    ? undefined
    : {
        icon: "🎉",
        title: $t("setup.ready"),
        description: $t("setup.readyDesc"),
      }}
  footer={completed ? undefined : footer}
  {error}
>
  {#if !completed}
    {#if loadingOptions}
      <div class="flex items-center justify-center py-8">
        <Spinner />
      </div>
    {:else}
      {#if isChineseUser}
        <!-- 中国源加速整合卡片 -->
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
                            class:mirror-detail-status--active={!!mirrors[service.id]}
                          >
                            {mirrors[service.id] ? "✓" : "○"}
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
      {/if}
    {/if}
  {:else}
    <!-- 完成后 -->
    <div class="flex flex-col items-center gap-6">
      <div class="checkmark-circle">
        <svg class="checkmark" viewBox="0 0 52 52">
          <circle
            class="checkmark-circle-bg"
            cx="26"
            cy="26"
            r="25"
            fill="none"
          />
          <path
            class="checkmark-check"
            fill="none"
            d="M14.1 27.2l7.1 7.2 16.7-16.8"
          />
        </svg>
      </div>

      <h1 class="text-4xl font-semibold">{$t("setup.setupDone")}</h1>
      <p>{$t("setup.setupDoneDesc")}</p>

      <div class="flex flex-col items-center gap-3">
        <p class="text-secondary">
          {$t("setup.autoRedirect", { values: { n: countdown } })}
        </p>
        <Button variant="primary" onclick={goToDashboard}
          >{$t("setup.enterNow")}</Button
        >
      </div>
    </div>
  {/if}
</SetupCard>

<style>
  /* ===== 完成动画 ===== */
  .checkmark-circle {
    width: 100px;
    height: 100px;
  }

  .checkmark {
    width: 100%;
    height: 100%;
  }

  .checkmark-circle-bg {
    stroke: var(--color-success);
    stroke-width: 2;
    animation: circle-fill 0.6s ease-out forwards;
  }

  .checkmark-check {
    stroke: var(--color-success);
    stroke-width: 3;
    stroke-linecap: round;
    stroke-linejoin: round;
    stroke-dasharray: 48;
    stroke-dashoffset: 48;
    animation: checkmark-draw 0.4s 0.4s ease-out forwards;
  }

  @keyframes circle-fill {
    from {
      stroke-dasharray: 0, 157;
    }
    to {
      stroke-dasharray: 157, 157;
    }
  }

  @keyframes checkmark-draw {
    to {
      stroke-dashoffset: 0;
    }
  }

  /* ===== 镜像整合卡片 ===== */
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

  /* 卡片头部 */
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

  /* 卡片体 */
  .mirror-card-body {
    border-top: 1px solid var(--color-border);
    padding: 0.75rem 1.25rem;
  }

  .mirror-card--enabled .mirror-card-body {
    border-top-color: rgba(59, 130, 246, 0.15);
  }

  /* 摘要行 */
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

  /* 自定义链接 */
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

  /* 关闭时提示 */
  .mirror-disabled-hint {
    font-size: var(--text-sm);
    color: var(--color-text-tertiary);
  }

  /* 详细配置列表 */
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
</style>
