<script lang="ts">
  import { t } from "./i18n";
  import { Button, Spinner, Switch, Alert, Input } from "$shared/ui";
  import {
    aiService,
    type Skill,
    type ProviderConfig,
  } from "./service";

  let { onclose }: { onclose: () => void } = $props();

  type StepType = "provider" | "skills" | "complete";

  let step = $state<StepType>("provider");
  let loading = $state(false);
  let error = $state("");

  // Provider
  let providers = $state<ProviderConfig[]>([]);
  let selectedProviderId = $state("");
  let apiKey = $state("");
  let providerSaving = $state(false);

  // Skills
  let skills = $state<Skill[]>([]);
  let enabledSkills = $state<string[]>([]);

  // 预定义的在线服务
  let onlineProviders = $derived([
    { id: "deepseek", name: "DeepSeek", provider: "deepseek", base_url: "https://api.deepseek.com", description: $t("ai.setup.providerDeepseekDesc") },
    { id: "openai", name: "OpenAI", provider: "openai", base_url: "https://api.openai.com", description: $t("ai.setup.providerOpenaiDesc") },
    { id: "qwen", name: $t("ai.qwen"), provider: "qwen", base_url: "https://dashscope.aliyuncs.com/compatible-mode", description: $t("ai.setup.providerQwenDesc") },
    { id: "zhipu", name: $t("ai.zhipu") + " AI", provider: "zhipu", base_url: "https://open.bigmodel.cn/api/paas", description: $t("ai.setup.providerZhipuDesc") },
  ]);

  async function loadProviders() {
    loading = true;
    try {
      providers = await aiService.getProviders();
      // 默认选中第一个启用的提供商
      const enabled = providers.find(p => p.enabled);
      if (enabled) {
        selectedProviderId = enabled.id;
        apiKey = enabled.api_key || "";
      }
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function saveProvider() {
    if (!selectedProviderId) return;
    providerSaving = true;
    error = "";
    try {
      const preset = onlineProviders.find(p => p.id === selectedProviderId);
      const existing = providers.find(p => p.id === selectedProviderId);
      
      if (existing) {
        // 更新已有提供商
        await aiService.updateProvider(selectedProviderId, { api_key: apiKey, enabled: true });
      } else if (preset) {
        // 创建新提供商
        await aiService.createProvider({
          provider: preset.provider as any,
          name: preset.name,
          base_url: preset.base_url,
          api_key: apiKey,
          enabled: true,
        });
      }
    } catch (e: any) {
      error = e.message;
    } finally {
      providerSaving = false;
    }
  }

  async function loadSkills() {
    loading = true;
    try {
      skills = await aiService.getDefaultSkills();
      enabledSkills = skills.filter((s) => s.enabled).map((s) => s.id);
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function toggleSkill(id: string) {
    if (enabledSkills.includes(id)) {
      enabledSkills = enabledSkills.filter((s) => s !== id);
    } else {
      enabledSkills = [...enabledSkills, id];
    }
  }

  async function nextStep() {
    error = "";
    try {
      if (step === "provider") {
        await saveProvider();
        if (error) return;
        await aiService.setSetupStep("skills");
        step = "skills";
        await loadSkills();
      } else if (step === "skills") {
        await aiService.completeSetup({ selected_model: "", skills_enabled: enabledSkills });
        step = "complete";
      } else {
        onclose();
      }
    } catch (e: any) {
      error = e.message;
    }
  }

  function prevStep() {
    if (step === "skills") step = "provider";
  }

  let steps = $derived<{ key: StepType; label: string }[]>([
    { key: "provider", label: $t("ai.setup.stepSelectService") },
    { key: "skills", label: $t("ai.setup.stepSelectSkills") },
    { key: "complete", label: $t("ai.setup.stepComplete") },
  ]);

  function stepIndex(s: StepType) {
    return steps.findIndex((x) => x.key === s);
  }

  // 初始化时加载提供商
  $effect(() => {
    loadProviders();
  });
</script>

<div class="space-y-6">
  <!-- Step indicator -->
  <div class="flex items-center gap-2">
    {#each steps as s, i}
      <div class="flex items-center gap-2">
        <div
          class="w-7 h-7 rounded-full flex items-center justify-center text-xs font-medium
            {stepIndex(step) >= i ? 'bg-blue-600 text-white' : 'bg-[var(--bg-primary)] text-[var(--text-tertiary)] border border-[var(--border-primary)]'}"
        >
          {i + 1}
        </div>
        <span class="text-xs text-[var(--text-secondary)] hidden sm:inline">{s.label}</span>
        {#if i < steps.length - 1}
          <div class="w-6 h-px bg-[var(--border-primary)]"></div>
        {/if}
      </div>
    {/each}
  </div>

  <!-- Step content -->
  {#if step === "provider"}
    <div class="space-y-4">
      <h3 class="text-sm font-medium text-[var(--text-primary)]">{$t("ai.setup.selectAiService")}</h3>
      <p class="text-xs text-[var(--text-tertiary)]">{$t("ai.setup.selectAiServiceDesc")}</p>

      {#if loading}
        <div class="flex items-center gap-2"><Spinner size="sm" /><span class="text-sm">{$t("common.loading")}</span></div>
      {:else}
        <div class="space-y-2 max-h-48 overflow-y-auto">
          {#each onlineProviders as preset}
            {@const existing = providers.find(p => p.id === preset.id)}
            <label
              class="flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors
                {selectedProviderId === preset.id ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20' : 'border-[var(--border-primary)] bg-[var(--bg-primary)]'}"
            >
              <input type="radio" name="provider" value={preset.id} bind:group={selectedProviderId} class="mt-1" onchange={() => (apiKey = existing?.api_key || "")} />
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-[var(--text-primary)]">{preset.name}</span>
                  {#if existing?.enabled && existing?.api_key}
                    <span class="text-xs px-1.5 py-0.5 rounded bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400">{$t("ai.setup.configured")}</span>
                  {/if}
                </div>
                <div class="text-xs text-[var(--text-tertiary)]">{preset.description}</div>
              </div>
            </label>
          {/each}
        </div>

        {#if selectedProviderId}
          <div class="space-y-2 pt-2 border-t border-[var(--border-primary)]">
            <label class="block text-sm font-medium text-[var(--text-primary)]">API Key</label>
            <Input type="password" bind:value={apiKey} placeholder={$t("ai.setup.enterApiKey")} />
            <p class="text-xs text-[var(--text-tertiary)]">
              {#if selectedProviderId === "deepseek"}
                获取 API Key: <a href="https://platform.deepseek.com/api_keys" target="_blank" class="text-blue-500 hover:underline">platform.deepseek.com</a>
              {:else if selectedProviderId === "openai"}
                获取 API Key: <a href="https://platform.openai.com/api-keys" target="_blank" class="text-blue-500 hover:underline">platform.openai.com</a>
              {:else if selectedProviderId === "qwen"}
                获取 API Key: <a href="https://dashscope.console.aliyun.com/" target="_blank" class="text-blue-500 hover:underline">dashscope.console.aliyun.com</a>
              {:else if selectedProviderId === "zhipu"}
                获取 API Key: <a href="https://open.bigmodel.cn/" target="_blank" class="text-blue-500 hover:underline">open.bigmodel.cn</a>
              {/if}
            </p>
          </div>
        {/if}
      {/if}
    </div>

  {:else if step === "skills"}
    <div class="space-y-4">
      <h3 class="text-sm font-medium text-[var(--text-primary)]">{$t("ai.setup.selectSkills")}</h3>
      <p class="text-xs text-[var(--text-tertiary)]">{$t("ai.setup.selectSkillsDesc")}</p>

      {#if loading}
        <div class="flex items-center gap-2"><Spinner size="sm" /><span class="text-sm">{$t("common.loading")}</span></div>
      {:else}
        <div class="space-y-2 max-h-64 overflow-y-auto">
          {#each skills as skill}
            <label
              class="flex items-center gap-3 p-3 rounded-lg border border-[var(--border-primary)] bg-[var(--bg-primary)] cursor-pointer hover:bg-[var(--bg-hover)]"
            >
              <Switch checked={enabledSkills.includes(skill.id)} onchange={() => toggleSkill(skill.id)} size="sm" />
              <div class="flex-1 min-w-0">
                <div class="text-sm font-medium text-[var(--text-primary)]">{skill.name}</div>
                <div class="text-xs text-[var(--text-tertiary)]">{skill.description}</div>
              </div>
              {#if skill.category}
                <span class="text-xs px-2 py-0.5 rounded bg-[var(--bg-secondary)] text-[var(--text-tertiary)]">{skill.category}</span>
              {/if}
            </label>
          {/each}
        </div>
      {/if}
    </div>

  {:else if step === "complete"}
    <div class="flex flex-col items-center py-8 gap-4">
      <div class="text-5xl">🎉</div>
      <h3 class="text-lg font-medium text-[var(--text-primary)]">{$t("ai.setup.setupComplete")}</h3>
      <p class="text-sm text-[var(--text-tertiary)]">{$t("ai.setup.setupCompleteDesc")}</p>
    </div>
  {/if}

  {#if error}
    <Alert type="error">{error}</Alert>
  {/if}

  <!-- Navigation -->
  <div class="flex justify-between pt-2 border-t border-[var(--border-primary)]">
    <div>
      {#if step === "skills"}
        <Button variant="ghost" onclick={prevStep}>{$t("ai.setup.prevStep")}</Button>
      {/if}
    </div>
    <Button variant="primary" onclick={nextStep} disabled={providerSaving || (step === "provider" && !apiKey)}>
      {#if providerSaving}
        <Spinner size="sm" />
      {:else}
        {step === "complete" ? $t("ai.setup.finish") : $t("ai.setup.nextStep")}
      {/if}
    </Button>
  </div>
</div>
