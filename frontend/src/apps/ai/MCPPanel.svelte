<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "./i18n";
  import { Button, Input, Modal, Spinner, Switch, Select, Alert as AlertBox } from "$shared/ui";
  import { aiService, type MCPServerConfig } from "./service";

  let servers = $state<MCPServerConfig[]>([]);
  let tools = $state<{ name: string; description?: string }[]>([]);
  let loading = $state(false);
  let error = $state("");
  let success = $state("");

  let showEdit = $state(false);
  let editing = $state<MCPServerConfig | null>(null);
  let saving = $state(false);
  let testing = $state<string | null>(null);
  let testResults = $state<Record<string, { ok: boolean; msg: string }>>({});

  onMount(async () => {
    await Promise.all([load(), loadTools()]);
  });

  async function load() {
    loading = true;
    error = "";
    try {
      servers = await aiService.listMCPServers();
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function loadTools() {
    try {
      const defs = await aiService.listMCPTools();
      tools = defs.map((d) => ({ name: d.function.name, description: d.function.description }));
    } catch {
      tools = [];
    }
  }

  function openCreate() {
    editing = {
      name: "",
      description: "",
      transport: "http",
      url: "",
      command: "",
      args: [],
      env: {},
      enabled: true,
    };
    showEdit = true;
  }

  function openEdit(s: MCPServerConfig) {
    editing = { ...s, args: [...(s.args || [])], env: { ...(s.env || {}) } };
    showEdit = true;
  }

  async function save() {
    if (!editing) return;
    saving = true;
    error = "";
    try {
      await aiService.upsertMCPServer(editing);
      showEdit = false;
      await Promise.all([load(), loadTools()]);
      success = $t("ai.mcp.saved");
      setTimeout(() => (success = ""), 2000);
    } catch (e: any) {
      error = e.message;
    } finally {
      saving = false;
    }
  }

  async function remove(name: string) {
    if (!confirm($t("ai.mcp.deleteConfirm", { values: { name } }))) return;
    try {
      await aiService.deleteMCPServer(name);
      await Promise.all([load(), loadTools()]);
    } catch (e: any) {
      error = e.message;
    }
  }

  async function toggleEnabled(s: MCPServerConfig) {
    try {
      await aiService.upsertMCPServer({ ...s, enabled: !s.enabled });
      await Promise.all([load(), loadTools()]);
    } catch (e: any) {
      error = e.message;
    }
  }

  async function test(name: string) {
    testing = name;
    try {
      const r = await aiService.testMCPServer(name);
      testResults[name] = r.success
        ? { ok: true, msg: $t("ai.mcp.testOk", { values: { n: (r.tools || []).length } }) }
        : { ok: false, msg: r.error || "failed" };
      if (r.success) await loadTools();
    } catch (e: any) {
      testResults[name] = { ok: false, msg: e.message };
    } finally {
      testing = null;
    }
  }

  let argsText = $derived(editing?.args?.length ? editing.args.join("\n") : "");
  let envText = $derived(
    editing?.env ? Object.entries(editing.env).map(([k, v]) => `${k}=${v}`).join("\n") : "",
  );

  function syncArgs(v: string) {
    if (!editing) return;
    editing.args = v.split("\n").map((s) => s.trim()).filter(Boolean);
  }
  function syncEnv(v: string) {
    if (!editing) return;
    const env: Record<string, string> = {};
    for (const line of v.split("\n")) {
      const idx = line.indexOf("=");
      if (idx > 0) env[line.slice(0, idx).trim()] = line.slice(idx + 1).trim();
    }
    editing.env = env;
  }

  // 编辑已存在服务器（名字不可改）
  let editingExisting = $derived(
    !!editing && servers.some((s) => s.name === editing!.name),
  );
</script>

<div class="space-y-4">
  <div class="flex items-center justify-between">
    <div>
      <h3 class="text-sm font-medium text-[var(--text-primary)]">{$t("ai.mcp.title")}</h3>
      <p class="text-xs text-[var(--text-tertiary)]">{$t("ai.mcp.description")}</p>
    </div>
    <Button size="sm" variant="primary" onclick={openCreate}>{$t("ai.mcp.add")}</Button>
  </div>

  {#if error}<AlertBox type="error">{error}</AlertBox>{/if}
  {#if success}<AlertBox type="success">{success}</AlertBox>{/if}

  {#if loading}
    <div class="flex justify-center p-8"><Spinner /></div>
  {:else if servers.length === 0}
    <p class="text-xs text-[var(--text-tertiary)] p-4">{$t("ai.mcp.empty")}</p>
  {:else}
    <div class="space-y-2">
      {#each servers as s (s.name)}
        <div class="border border-[var(--border-primary)] rounded-lg p-3 space-y-2">
          <div class="flex items-center justify-between">
            <div class="min-w-0">
              <span class="text-sm font-medium text-[var(--text-primary)]">{s.name}</span>
              <span class="ml-2 text-xs px-1.5 py-0.5 rounded bg-[var(--bg-tertiary)] text-[var(--text-tertiary)]">
                {s.transport === "stdio" ? "stdio" : "HTTP"}
              </span>
              {#if s.description}
                <p class="text-xs text-[var(--text-tertiary)] truncate">{s.description}</p>
              {/if}
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <Switch checked={s.enabled} onclick={(e) => { e.preventDefault(); toggleEnabled(s); }} />
              <Button size="sm" variant="secondary" onclick={() => test(s.name)} disabled={testing === s.name}>
                {#if testing === s.name}<Spinner size="sm" />{:else}{$t("ai.mcp.test")}{/if}
              </Button>
              <Button size="sm" variant="ghost" onclick={() => openEdit(s)}>{$t("ai.mcp.edit")}</Button>
              <Button size="sm" variant="danger" onclick={() => remove(s.name)}>✕</Button>
            </div>
          </div>
          {#if s.transport === "http"}
            <p class="text-xs text-[var(--text-tertiary)] truncate">{s.url}</p>
          {:else}
            <p class="text-xs text-[var(--text-tertiary)] truncate">{s.command} {(s.args || []).join(" ")}</p>
          {/if}
          {#if testResults[s.name]}
            <p class="text-xs {testResults[s.name].ok ? 'text-green-600' : 'text-red-500'}">
              {testResults[s.name].msg}
            </p>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

  {#if tools.length > 0}
    <details class="border border-[var(--border-primary)] rounded-lg">
      <summary class="px-4 py-3 cursor-pointer text-sm font-medium text-[var(--text-primary)] bg-[var(--bg-primary)] rounded-t-lg">
        {$t("ai.mcp.tools", { values: { n: tools.length } })}
      </summary>
      <div class="p-4 space-y-1">
        {#each tools as tool (tool.name)}
          <p class="text-xs text-[var(--text-secondary)]">
            <span class="font-mono">{tool.name}</span>
            {#if tool.description}<span class="text-[var(--text-tertiary)]"> — {tool.description}</span>{/if}
          </p>
        {/each}
      </div>
    </details>
  {/if}
</div>

<!-- 新增/编辑服务器 -->
<Modal open={showEdit} title={editingExisting ? $t("ai.mcp.editTitle") : $t("ai.mcp.addTitle")} onclose={() => (showEdit = false)} size="md">
  {#if editing}
    <div class="space-y-3">
      <div>
        <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.mcp.name")}</label>
        <Input bind:value={editing.name} placeholder="weather" disabled={editingExisting} />
      </div>
      <div>
        <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.mcp.transport")}</label>
        <Select
          options={[{ value: "http", label: "HTTP (Streamable)" }, { value: "stdio", label: "stdio" }]}
          bind:value={editing.transport}
        />
      </div>
      {#if editing.transport === "http"}
        <div>
          <label class="text-xs text-[var(--text-tertiary)]">URL</label>
          <Input bind:value={editing.url} placeholder="https://mcp.example.com/mcp" />
        </div>
      {:else}
        <div>
          <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.mcp.command")}</label>
          <Input bind:value={editing.command} placeholder="uvx / npx / ..." />
        </div>
        <div>
          <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.mcp.args")}</label>
          <textarea class="w-full text-sm p-2 border border-[var(--border-primary)] rounded bg-[var(--bg-primary)] text-[var(--text-primary)]" rows="3" placeholder="-y&#10;@modelcontextprotocol/server-weather" value={argsText} oninput={(e) => syncArgs(e.currentTarget.value)}></textarea>
        </div>
        <div>
          <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.mcp.env")}</label>
          <textarea class="w-full text-sm p-2 border border-[var(--border-primary)] rounded bg-[var(--bg-primary)] text-[var(--text-primary)]" rows="2" placeholder="API_KEY=xxx" value={envText} oninput={(e) => syncEnv(e.currentTarget.value)}></textarea>
        </div>
      {/if}
      <div>
        <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.mcp.serverDescription")}</label>
        <Input bind:value={editing.description} />
      </div>
      <Switch bind:checked={editing.enabled} label={$t("ai.mcp.enabled")} />
      <div class="flex justify-end gap-2 pt-2">
        <Button variant="secondary" onclick={() => (showEdit = false)}>{$t("common.cancel")}</Button>
        <Button variant="primary" onclick={save} disabled={saving || !editing.name.trim()}>
          {#if saving}<Spinner size="sm" />{:else}{$t("common.save")}{/if}
        </Button>
      </div>
    </div>
  {/if}
</Modal>
