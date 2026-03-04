<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "./i18n";
  import { Button, Input, Spinner, Alert } from "$shared/ui";
  import { aiService, type SkillResponse } from "./service";

  let loading = $state(false);
  let result = $state<SkillResponse | null>(null);
  let activeSkill = $state("");
  let error = $state("");

  const skillGroups = $derived([
    {
      title: $t("ai.skills.systemMonitor"),
      skills: [
        { id: "system_info", label: $t("ai.skills.systemInfo"), icon: "🖥️", action: () => runSystemInfo() },
        { id: "storage", label: $t("ai.skills.storageAnalysis"), icon: "💾", action: () => runStorage() },
        { id: "files", label: $t("ai.skills.fileSearch"), icon: "🔍", action: () => runFileSearch() },
      ],
    },
    {
      title: "AI Tools",
      skills: [
        { id: "docker_status", label: $t("ai.skills.containerStatus"), icon: "🐳", action: () => runSkill("docker_status", "status") },
        { id: "network", label: $t("ai.skills.networkStatus"), icon: "🌐", action: () => runSkill("network_status", "check") },
        { id: "temperature", label: $t("ai.skills.temperatureMonitor"), icon: "🌡️", action: () => runSkill("temperature", "read") },
      ],
    },
  ]);

  let searchQuery = $state("");

  async function runSystemInfo() {
    activeSkill = "system_info";
    loading = true;
    error = "";
    result = null;
    try {
      result = await aiService.getSystemInfo();
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function runStorage() {
    activeSkill = "storage";
    loading = true;
    error = "";
    result = null;
    try {
      result = await aiService.getStorageAnalysis("/");
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function runFileSearch() {
    if (!searchQuery.trim()) return;
    activeSkill = "files";
    loading = true;
    error = "";
    result = null;
    try {
      result = await aiService.searchFiles(searchQuery);
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function runSkill(skillId: string, action: string) {
    activeSkill = skillId;
    loading = true;
    error = "";
    result = null;
    try {
      result = await aiService.executeSkill(skillId, action);
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function formatResult(data: unknown): string {
    if (!data) return "";
    try {
      return JSON.stringify(data, null, 2);
    } catch {
      return String(data);
    }
  }
</script>

<div class="flex flex-col h-full">
  <!-- Skill buttons -->
  <div class="p-3 space-y-3 border-b border-[var(--border-primary)]">
    {#each skillGroups as group}
      <div>
        <div class="text-xs font-medium text-[var(--text-tertiary)] mb-1.5">{group.title}</div>
        <div class="flex flex-wrap gap-2">
          {#each group.skills as skill}
            <button
              class="px-3 py-1.5 text-sm rounded-lg border border-[var(--border-primary)] hover:bg-[var(--bg-hover)] transition-colors flex items-center gap-1.5
                {activeSkill === skill.id ? 'bg-blue-50 border-blue-300 dark:bg-blue-900/20 dark:border-blue-700' : 'bg-[var(--bg-primary)]'}"
              onclick={skill.action}
              disabled={loading}
            >
              <span>{skill.icon}</span>
              <span class="text-[var(--text-primary)]">{skill.label}</span>
            </button>
          {/each}
        </div>
      </div>
    {/each}

    <!-- File Search -->
    <div class="flex gap-2">
      <div class="flex-1"><Input size="sm" placeholder={$t("ai.skills.searchFilename")} bind:value={searchQuery} onenter={runFileSearch} /></div>
      <Button size="sm" onclick={runFileSearch} disabled={loading || !searchQuery.trim()}>{$t("ai.skills.search")}</Button>
    </div>
  </div>

  <!-- Result -->
  <div class="flex-1 overflow-y-auto p-3">
    {#if loading}
      <div class="flex items-center justify-center p-8"><Spinner /><span class="ml-2 text-sm">{$t("ai.skills.executing")}</span></div>
    {:else if error}
      <Alert type="error">{error}</Alert>
    {:else if result}
      {#if result.summary}
        <Alert type="info">{result.summary}</Alert>
      {/if}
      {#if result.error}
        <Alert type="error">{result.error}</Alert>
      {/if}
      {#if result.data}
        <pre class="text-xs bg-[var(--bg-primary)] border border-[var(--border-primary)] rounded p-3 overflow-x-auto whitespace-pre-wrap font-mono text-[var(--text-primary)]">{formatResult(result.data)}</pre>
      {/if}
    {:else}
      <div class="flex flex-col items-center justify-center h-full text-[var(--text-tertiary)]">
        <div class="text-3xl mb-2">🛠️</div>
        <div class="text-sm">{$t("ai.skills.selectSkill")}</div>
      </div>
    {/if}
  </div>
</div>
