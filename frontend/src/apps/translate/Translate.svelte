<script lang="ts">
  import { onMount } from "svelte";
  import { get } from "svelte/store";
  import { locale } from "$lib/i18n";
  import { initI18n, t } from "./i18n";
  import { Button, Select, Spinner, EmptyState } from "$shared/ui";
  import {
    translateText,
    getLanguages,
    getStatus,
    getConfig,
    type Language,
    type ServiceStatus,
    type TranslateConfig,
  } from "./service";
  import TranslateSetup from "./TranslateSetup.svelte";

  // ==================== State ====================

  let languages = $state<Language[]>([]);
  let status = $state<ServiceStatus | null>(null);
  let config = $state<TranslateConfig | null>(null);

  let sourceText = $state("");
  let targetText = $state("");
  let sourceLang = $state("auto");
  let targetLang = $state("en");
  let detectedLang = $state("");

  let loading = $state(true);
  let translating = $state(false);
  let error = $state("");

  let showSetup = $state(false);

  // ==================== 初始化 ====================

  onMount(async () => {
    // 先初始化 i18n，确保翻译键被注册
    await initI18n();
    await checkStatus();
  });

  async function checkStatus() {
    loading = true;
    error = "";

    try {
      // 检查服务状态
      status = await getStatus();

      if (status.available) {
        // 服务可用，加载语言列表和配置
        const currentLocale = get(locale) || "en-US";
        const [langs, cfg] = await Promise.all([
          getLanguages(),
          getConfig(currentLocale),
        ]);
        languages = langs;
        config = cfg;

        // 设置默认语言
        sourceLang = "auto";
        targetLang = cfg.defaultTarget || "en";
      } else {
        // 服务不可用，显示安装引导
        showSetup = true;
      }
    } catch (e) {
      error = e instanceof Error ? e.message : "加载失败";
      showSetup = true;
    } finally {
      loading = false;
    }
  }

  // ==================== 翻译 ====================

  async function handleTranslate() {
    if (!sourceText.trim()) return;

    translating = true;
    error = "";
    detectedLang = "";

    try {
      const result = await translateText({
        text: sourceText,
        source: sourceLang === "auto" ? "" : sourceLang,
        target: targetLang,
      });

      targetText = result.translatedText;
      if (result.detectedLang) {
        detectedLang = result.detectedLang;
      }
    } catch (e) {
      error = e instanceof Error ? e.message : "翻译失败";
    } finally {
      translating = false;
    }
  }

  // 交换语言
  function swapLanguages() {
    if (sourceLang === "auto") {
      // 如果源语言是自动检测，使用检测到的语言或目标语言
      sourceLang = detectedLang || targetLang;
    } else {
      // 正常交换
      [sourceLang, targetLang] = [targetLang, sourceLang];
    }
    // 交换文本
    [sourceText, targetText] = [targetText, sourceText];
    detectedLang = "";
  }

  // 清空
  function clearAll() {
    sourceText = "";
    targetText = "";
    detectedLang = "";
    error = "";
  }

  // 复制结果
  async function copyResult() {
    if (targetText) {
      await navigator.clipboard.writeText(targetText);
    }
  }

  // 获取语言名称
  function getLangName(code: string): string {
    if (code === "auto") return get(t)("autoDetect");
    const lang = languages.find((l) => l.code === code);
    return lang?.name || code;
  }

  // 语言选项
  $effect(() => {
    // 当语言列表加载后，确保选择的语言有效
    if (languages.length > 0) {
      if (targetLang && !languages.find((l) => l.code === targetLang)) {
        targetLang = languages[0]?.code || "en";
      }
    }
  });
</script>

<div class="translate-app">
  {#if loading}
    <div class="loading-container">
      <Spinner size="lg" />
      <p>{$t("loading")}</p>
    </div>
  {:else if showSetup}
    <TranslateSetup
      {status}
      onRetry={async () => {
        showSetup = false;
        await checkStatus();
      }}
    />
  {:else}
    <!-- 主界面 -->
    <div class="translate-header">
      <!-- 语言选择器 -->
      <div class="language-selector">
        <Select
          bind:value={sourceLang}
          options={[
            { value: "auto", label: $t("autoDetect") },
            ...languages.map((l) => ({ value: l.code, label: l.name })),
          ]}
        />

        <button class="swap-btn" onclick={swapLanguages} title={$t("swap")}>
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M7 16l-4-4m0 0l4-4m-4 4h18M17 8l4 4m0 0l-4 4m4-4H3" />
          </svg>
        </button>

        <Select
          bind:value={targetLang}
          options={languages.map((l) => ({ value: l.code, label: l.name }))}
        />
      </div>

      <!-- 操作按钮 -->
      <div class="header-actions">
        <Button variant="ghost" size="sm" onclick={clearAll}>
          {$t("clear")}
        </Button>
      </div>
    </div>

    <!-- 翻译区域 -->
    <div class="translate-content">
      <!-- 源文本 -->
      <div class="text-panel source-panel">
        <div class="panel-header">
          <span class="panel-title">
            {sourceLang === "auto" ? $t("sourceText") : getLangName(sourceLang)}
            {#if detectedLang}
              <span class="detected-lang">({$t("detected")}: {getLangName(detectedLang)})</span>
            {/if}
          </span>
          <span class="char-count">{sourceText.length}</span>
        </div>
        <textarea
          class="text-input"
          bind:value={sourceText}
          placeholder={$t("enterText")}
          onkeydown={(e) => {
            if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
              handleTranslate();
            }
          }}
        ></textarea>
      </div>

      <!-- 翻译按钮 -->
      <div class="translate-action">
        <Button
          variant="primary"
          size="lg"
          onclick={handleTranslate}
          disabled={translating || !sourceText.trim()}
        >
          {#if translating}
            <Spinner size="sm" />
            {$t("translating")}
          {:else}
            {$t("translate")}
          {/if}
        </Button>
        <span class="shortcut-hint">Ctrl + Enter</span>
      </div>

      <!-- 目标文本 -->
      <div class="text-panel target-panel">
        <div class="panel-header">
          <span class="panel-title">{getLangName(targetLang)}</span>
          {#if targetText}
            <button class="copy-btn" onclick={copyResult} title={$t("copy")}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
              </svg>
            </button>
          {/if}
        </div>
        <div class="text-output" class:empty={!targetText}>
          {#if targetText}
            {targetText}
          {:else}
            <span class="placeholder">{$t("translationWillAppear")}</span>
          {/if}
        </div>
      </div>
    </div>

    <!-- 错误提示 -->
    {#if error}
      <div class="error-message">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10" />
          <line x1="12" y1="8" x2="12" y2="12" />
          <line x1="12" y1="16" x2="12.01" y2="16" />
        </svg>
        {error}
      </div>
    {/if}

    <!-- 底部信息 -->
    <div class="translate-footer">
      <span class="powered-by">Powered by LibreTranslate</span>
      <button class="status-btn" onclick={() => (showSetup = true)}>
        <span class="status-dot" class:online={status?.available}></span>
        {status?.available ? $t("serviceOnline") : $t("serviceOffline")}
      </button>
    </div>
  {/if}
</div>

<style>
  .translate-app {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--color-bg-primary);
    color: var(--color-text-primary);
  }

  .loading-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 1rem;
    color: var(--color-text-secondary);
  }

  /* Header */
  .translate-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 1rem 1.5rem;
    border-bottom: 1px solid var(--color-border);
    background: var(--color-bg-secondary);
  }

  .language-selector {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .swap-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    border: none;
    border-radius: 50%;
    background: var(--color-bg-tertiary);
    color: var(--color-text-secondary);
    cursor: pointer;
    transition: all 0.2s;
  }

  .swap-btn:hover {
    background: var(--color-primary);
    color: white;
  }

  .header-actions {
    display: flex;
    gap: 0.5rem;
  }

  /* Content */
  .translate-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    padding: 1.5rem;
    gap: 1rem;
    overflow: hidden;
  }

  .text-panel {
    flex: 1;
    display: flex;
    flex-direction: column;
    background: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    border-radius: 12px;
    overflow: hidden;
    min-height: 150px;
  }

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--color-border);
    background: var(--color-bg-tertiary);
  }

  .panel-title {
    font-weight: 500;
    color: var(--color-text-primary);
  }

  .detected-lang {
    font-size: 0.85em;
    color: var(--color-text-tertiary);
    margin-left: 0.5rem;
  }

  .char-count {
    font-size: 0.85rem;
    color: var(--color-text-tertiary);
  }

  .text-input {
    flex: 1;
    width: 100%;
    padding: 1rem;
    border: none;
    background: transparent;
    color: var(--color-text-primary);
    font-size: 1rem;
    line-height: 1.6;
    resize: none;
    outline: none;
  }

  .text-input::placeholder {
    color: var(--color-text-tertiary);
  }

  .text-output {
    flex: 1;
    padding: 1rem;
    font-size: 1rem;
    line-height: 1.6;
    overflow-y: auto;
    white-space: pre-wrap;
    word-break: break-word;
  }

  .text-output.empty {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .text-output .placeholder {
    color: var(--color-text-tertiary);
  }

  .copy-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0.25rem;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: var(--color-text-secondary);
    cursor: pointer;
    transition: all 0.2s;
  }

  .copy-btn:hover {
    background: var(--color-bg-hover);
    color: var(--color-primary);
  }

  /* Translate Action */
  .translate-action {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0;
  }

  .shortcut-hint {
    font-size: 0.75rem;
    color: var(--color-text-tertiary);
  }

  /* Error */
  .error-message {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.75rem 1.5rem;
    background: var(--color-error-bg);
    color: var(--color-error);
    font-size: 0.875rem;
  }

  /* Footer */
  .translate-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.75rem 1.5rem;
    border-top: 1px solid var(--color-border);
    background: var(--color-bg-secondary);
  }

  .powered-by {
    font-size: 0.75rem;
    color: var(--color-text-tertiary);
  }

  .status-btn {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.25rem 0.5rem;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: var(--color-text-secondary);
    font-size: 0.75rem;
    cursor: pointer;
    transition: all 0.2s;
  }

  .status-btn:hover {
    background: var(--color-bg-hover);
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--color-error);
  }

  .status-dot.online {
    background: var(--color-success);
  }
</style>
