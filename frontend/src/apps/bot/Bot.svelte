<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import Icon from "@iconify/svelte";
  import { Button, Input, Modal, Spinner, EmptyState, Tabs, Select } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import { authService, type UserInfo } from "$shared/services/auth";
  import { usersService, type User } from "$shared/services/users";
  import {
    listBots, createBot, getBot, updateBot, deleteBot,
    startBot, stopBot, restartBot, getBotLogs, execInBot,
    getBotMembers, setBotMembers, takeoverBot, releaseBot,
    getBotChannels, setBotChannels, listSkills, reloadSkills,
    botStreamUrl, type Bot, type BotMember, type ExecResult,
    type BotChannel, type SkillMeta,
  } from "$shared/services/bot";

  // ==================== 状态 ====================

  let bots = $state<Bot[]>([]);
  let selectedId = $state<string | null>(null);
  let loading = $state(true);
  let working = $state(false);
  let me = $state<UserInfo | null>(null);
  let activeTab = $state("overview");

  // 详情
  let detail = $state<Bot | null>(null);
  let logs = $state("");
  let users = $state<User[]>([]);
  let memberDraft = $state<BotMember[]>([]);
  let addUserId = $state("");
  let addRole = $state("viewer");

  // 工具执行
  let execScript = $state("");
  let execLanguage = $state("bash");
  let execResult = $state<ExecResult | null>(null);
  let execRunning = $state(false);

  // 渠道绑定（P3）
  let channelDraft = $state<BotChannel[]>([]);
  let newChannelPlatform = $state("telegram");
  let newChatId = $state("");
  let newUserId = $state("");

  // 技能清单（P3）
  let skills = $state<SkillMeta[]>([]);

  // 创建弹窗
  let showCreate = $state(false);
  let createForm = $state({ name: "", description: "", model: "", image: "" });

  // 编辑弹窗
  let showEdit = $state(false);
  let editForm = $state({ name: "", description: "", model: "" });

  let refreshTimer: ReturnType<typeof setInterval>;

  const tabs = $derived([
    { id: "overview", label: "概览" },
    { id: "workbench", label: "工作台" },
    { id: "tools", label: "工具" },
    ...(detail?.my_role === "admin" ? [{ id: "members", label: "成员" }] : []),
    ...(detail?.my_role === "admin" ? [{ id: "channels", label: "渠道" }] : []),
    { id: "skills", label: "技能" },
    { id: "logs", label: "日志" },
  ]);

  // ==================== 生命周期 ====================

  onMount(async () => {
    me = await authService.getCurrentUser();
    if (me?.role === "admin") {
      usersService.list().then((u) => (users = u)).catch(() => {});
    }
    await refresh();
    refreshTimer = setInterval(refresh, 10000);
  });

  onDestroy(() => clearInterval(refreshTimer));

  // 标签页切换时按需加载
  $effect(() => {
    const tab = activeTab;
    const id = selectedId;
    if (tab === "members" && id && detail?.my_role === "admin") {
      loadMembers(id);
    } else if (tab === "logs" && id) {
      loadLogs(id);
    } else if (tab === "channels" && id && detail?.my_role === "admin") {
      loadChannels(id);
    } else if (tab === "skills") {
      loadSkills();
    }
  });

  async function refresh() {
    try {
      bots = await listBots();
      if (selectedId && !bots.some((b) => b.id === selectedId)) {
        selectedId = null;
        detail = null;
      }
      if (selectedId) {
        // 轻量刷新：用列表数据更新，保留详情里的成员字段
        const inList = bots.find((b) => b.id === selectedId);
        if (detail && inList) detail = { ...inList, members: detail.members };
        else if (!detail) detail = inList || null;
      }
    } catch {
      // 静默重试
    } finally {
      loading = false;
    }
  }

  async function selectBot(id: string) {
    selectedId = id;
    activeTab = "overview";
    execResult = null;
    logs = "";
    try {
      detail = await getBot(id);
    } catch {
      detail = bots.find((b) => b.id === id) || null;
    }
  }

  // ==================== 动作 ====================

  async function handleCreate() {
    if (!createForm.name.trim()) return;
    working = true;
    try {
      const b = await createBot(createForm);
      showCreate = false;
      createForm = { name: "", description: "", model: "", image: "" };
      showToast(`Bot「${b.name}」已创建`, "success");
      await refresh();
      await selectBot(b.id);
    } catch (e) {
      showToast(e instanceof Error ? e.message : "创建失败", "error");
    } finally {
      working = false;
    }
  }

  async function handleStart() {
    if (!detail) return;
    working = true;
    try {
      detail = await startBot(detail.id);
      showToast("沙箱已启动", "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "启动失败", "error");
    } finally {
      working = false;
      refresh();
    }
  }

  async function handleStop() {
    if (!detail) return;
    working = true;
    try {
      detail = await stopBot(detail.id);
      showToast("沙箱已停止", "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "停止失败", "error");
    } finally {
      working = false;
      refresh();
    }
  }

  async function handleRestart() {
    if (!detail) return;
    working = true;
    try {
      detail = await restartBot(detail.id);
      showToast("沙箱已重启", "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "重启失败", "error");
    } finally {
      working = false;
      refresh();
    }
  }

  async function handleDelete() {
    if (!detail) return;
    working = true;
    try {
      await deleteBot(detail.id);
      showToast(`Bot「${detail.name}」已删除`, "success");
      selectedId = null;
      detail = null;
      refresh();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "删除失败", "error");
    } finally {
      working = false;
    }
  }

  async function handleSaveEdit() {
    if (!detail) return;
    working = true;
    try {
      detail = await updateBot(detail.id, editForm);
      showEdit = false;
      showToast("已保存", "success");
      refresh();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "保存失败", "error");
    } finally {
      working = false;
    }
  }

  async function handleExec() {
    if (!detail || !execScript.trim()) return;
    execRunning = true;
    execResult = null;
    try {
      execResult = await execInBot(detail.id, {
        script: execScript,
        language: execLanguage as "bash" | "python",
        timeout: 60,
      });
    } catch (e) {
      showToast(e instanceof Error ? e.message : "执行失败", "error");
    } finally {
      execRunning = false;
    }
  }

  async function handleTakeover() {
    if (!detail) return;
    try {
      await takeoverBot(detail.id);
      showToast("已接管（其他人将只读围观）", "success");
      refresh();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "接管失败", "error");
    }
  }

  async function handleRelease() {
    if (!detail) return;
    try {
      await releaseBot(detail.id);
      showToast("已交还", "success");
      refresh();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "交还失败", "error");
    }
  }

  async function loadLogs(id: string) {
    try {
      logs = await getBotLogs(id);
    } catch (e) {
      logs = e instanceof Error ? e.message : "日志获取失败";
    }
  }

  // 成员管理
  async function loadMembers(id: string) {
    try {
      memberDraft = await getBotMembers(id);
    } catch {
      memberDraft = [];
    }
  }

  function addMember() {
    if (!addUserId || memberDraft.some((m) => m.user_id === addUserId)) return;
    memberDraft = [...memberDraft, { user_id: addUserId, role: addRole as BotMember["role"] }];
    addUserId = "";
  }

  function removeMember(userId: string) {
    memberDraft = memberDraft.filter((m) => m.user_id !== userId);
  }

  function setMemberRole(userId: string, role: string) {
    memberDraft = memberDraft.map((m) => (m.user_id === userId ? { ...m, role: role as BotMember["role"] } : m));
  }

  async function saveMembers() {
    if (!detail) return;
    try {
      await setBotMembers(detail.id, memberDraft);
      showToast("成员已保存", "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "保存失败", "error");
    }
  }

  function usernameOf(userId: string): string {
    const u = users.find((x) => x.id === userId);
    return u ? u.nickname || u.username : userId.slice(0, 8);
  }

  // 渠道绑定（P3）
  async function loadChannels(id: string) {
    try {
      channelDraft = await getBotChannels(id);
    } catch {
      channelDraft = [];
    }
  }

  function addChannel() {
    const platform = newChannelPlatform as BotChannel["platform"];
    if (!newChatId.trim() && !newUserId.trim()) {
      showToast("Chat ID 与用户 ID 至少填一个", "warning");
      return;
    }
    if (channelDraft.some((c) => c.platform === platform && c.chat_id === newChatId.trim() && c.user_id === newUserId.trim())) return;
    channelDraft = [...channelDraft, { platform, chat_id: newChatId.trim(), user_id: newUserId.trim() }];
    newChatId = "";
    newUserId = "";
  }

  function removeChannel(index: number) {
    channelDraft = channelDraft.filter((_, i) => i !== index);
  }

  async function saveChannels() {
    const d = detail;
    if (!d) return;
    try {
      await setBotChannels(d.id, channelDraft);
      showToast("渠道绑定已保存", "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "保存失败", "error");
    }
  }

  // 技能清单（P3）
  async function loadSkills() {
    try {
      skills = await listSkills();
    } catch {
      skills = [];
    }
  }

  async function handleReloadSkills() {
    try {
      skills = await reloadSkills();
      showToast(`已重新扫描，共 ${skills.length} 个技能`, "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "扫描失败", "error");
    }
  }

  // ==================== 展示 ====================

  function statusColor(status: string): string {
    switch (status) {
      case "running": return "bg-green-500";
      case "starting": return "bg-yellow-500";
      case "error": return "bg-red-500";
      case "stopped": return "bg-gray-400";
      default: return "bg-gray-500";
    }
  }

  function statusText(status: string): string {
    switch (status) {
      case "running": return "运行中";
      case "starting": return "启动中";
      case "error": return "异常";
      case "stopped": return "已停止";
      default: return "未创建";
    }
  }

  function roleText(role: string | undefined): string {
    switch (role) {
      case "admin": return "接管";
      case "member": return "预览";
      case "viewer": return "状态";
      default: return "无权限";
    }
  }

  const roleOptions = [
    { value: "viewer", label: "状态（只看状态）" },
    { value: "member", label: "预览（围观串流）" },
    { value: "admin", label: "接管（全屏操作）" },
  ];

  const platformOptions = [
    { value: "telegram", label: "Telegram" },
    { value: "wecom", label: "企业微信" },
    { value: "webhook", label: "Webhook" },
  ];

  let streamUrl = $derived(detail ? botStreamUrl(detail.id) : "");
  let iOwnTakeover = $derived(!!detail && !!me && detail.taken_over_by !== "" && detail.taken_over_by === String(me.id));
  let someoneElseTook = $derived(!!detail?.taken_over_by && !iOwnTakeover);
</script>

<div class="flex h-full flex-col bg-gray-50 dark:bg-gray-900">
  <!-- 顶部栏 -->
  <header class="flex shrink-0 items-center justify-between border-b border-gray-200 bg-white px-4 py-3 dark:border-gray-700 dark:bg-gray-800">
    <div class="flex items-center gap-2">
      <Icon icon="mdi:robot-outline" class="text-2xl text-blue-500" />
      <h2 class="text-base font-semibold text-gray-800 dark:text-gray-100">Bot 工作台</h2>
      <span class="text-xs text-gray-400">数字员工：每个 Bot 一台自己的电脑</span>
    </div>
    {#if me?.role === "admin"}
      <Button variant="primary" size="sm" onclick={() => (showCreate = true)}>
        <Icon icon="mdi:plus" class="mr-1" />
        创建 Bot
      </Button>
    {/if}
  </header>

  <div class="flex min-h-0 flex-1">
    {#if loading}
      <div class="flex w-full items-center justify-center">
        <Spinner size="lg" />
      </div>
    {:else if bots.length === 0}
      <div class="w-full">
        <EmptyState
          icon="mdi:robot-outline"
          title="还没有 Bot"
          description={me?.role === "admin" ? "创建第一个数字员工，给它分配一台沙箱电脑" : "请联系管理员将你加入某个 Bot 的成员"}
        />
      </div>
    {:else}
      <!-- 左侧 Bot 列表 -->
      <aside class="w-64 shrink-0 overflow-y-auto border-r border-gray-200 bg-white p-2 dark:border-gray-700 dark:bg-gray-800">
        {#each bots as b (b.id)}
          <button
            class="mb-1 flex w-full items-center gap-2 rounded-lg px-3 py-2.5 text-left transition-colors
              {selectedId === b.id ? 'bg-blue-50 dark:bg-blue-900/30' : 'hover:bg-gray-100 dark:hover:bg-gray-700'}"
            onclick={() => selectBot(b.id)}
          >
            <span class="h-2.5 w-2.5 shrink-0 rounded-full {statusColor(b.status)} {b.status === 'running' ? 'animate-pulse' : ''}"></span>
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-medium text-gray-800 dark:text-gray-100">{b.name}</div>
              <div class="truncate text-xs text-gray-400">{statusText(b.status)}{b.taken_over_by ? ' · 被接管' : ''}</div>
            </div>
          </button>
        {/each}
      </aside>

      <!-- 右侧详情 -->
      <section class="flex min-w-0 flex-1 flex-col">
        {#if !detail}
          <div class="flex flex-1 items-center justify-center text-sm text-gray-400">选择左侧的 Bot 查看详情</div>
        {:else}
          <div class="flex shrink-0 items-center justify-between border-b border-gray-200 bg-white px-4 py-2.5 dark:border-gray-700 dark:bg-gray-800">
            <div class="flex items-center gap-2">
              <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-100">{detail.name}</h3>
              <span class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-gray-700 dark:text-gray-300">
                <span class="h-1.5 w-1.5 rounded-full {statusColor(detail.status)}"></span>
                {statusText(detail.status)}
              </span>
              <span class="rounded-full bg-blue-50 px-2 py-0.5 text-xs text-blue-600 dark:bg-blue-900/30 dark:text-blue-300">{roleText(detail.my_role)}</span>
              {#if someoneElseTook}
                <span class="rounded-full bg-yellow-50 px-2 py-0.5 text-xs text-yellow-600 dark:bg-yellow-900/30 dark:text-yellow-300">已被 {detail.taken_over_by_name || "其他人"} 接管</span>
              {/if}
            </div>
            {#if detail.my_role === "admin"}
              <div class="flex items-center gap-1.5">
                {#if detail.status === "running"}
                  <Button variant="outline" size="sm" onclick={handleRestart} disabled={working}>重启</Button>
                  <Button variant="outline" size="sm" onclick={handleStop} disabled={working}>停止</Button>
                {:else}
                  <Button variant="primary" size="sm" onclick={handleStart} disabled={working}>
                    <Icon icon="mdi:play" class="mr-1" />启动沙箱
                  </Button>
                {/if}
                <Button
                  variant="ghost" size="sm"
                  onclick={() => { const d = detail; if (!d) return; editForm = { name: d.name, description: d.description, model: d.model }; showEdit = true; }}
                >编辑</Button>
                <Button variant="ghost" size="sm" onclick={handleDelete} disabled={working}>
                  <span class="text-red-500">删除</span>
                </Button>
              </div>
            {/if}
          </div>

          <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
            <Tabs {tabs} bind:activeTab={activeTab} variant="underline" size="sm">
              {#snippet children(tab)}
                {#if detail}
                <div class="min-h-0 flex-1 overflow-y-auto p-4">
                  <!-- 概览 -->
                  {#if tab === "overview"}
                    <div class="grid grid-cols-2 gap-3 text-sm">
                      <div class="rounded-lg bg-white p-3 dark:bg-gray-800">
                        <div class="text-xs text-gray-400">职责</div>
                        <div class="mt-1 text-gray-700 dark:text-gray-200">{detail.description || "—"}</div>
                      </div>
                      <div class="rounded-lg bg-white p-3 dark:bg-gray-800">
                        <div class="text-xs text-gray-400">模型</div>
                        <div class="mt-1 text-gray-700 dark:text-gray-200">{detail.model || "—"}</div>
                      </div>
                      <div class="rounded-lg bg-white p-3 dark:bg-gray-800">
                        <div class="text-xs text-gray-400">沙箱镜像</div>
                        <div class="mt-1 truncate text-gray-700 dark:text-gray-200">{detail.image}</div>
                      </div>
                      <div class="rounded-lg bg-white p-3 dark:bg-gray-800">
                        <div class="text-xs text-gray-400">工作目录</div>
                        <div class="mt-1 text-gray-700 dark:text-gray-200">/workspace（持久卷）</div>
                      </div>
                    </div>
                    {#if detail.status === "error" && detail.error_message}
                      <div class="mt-3 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400">
                        {detail.error_message}
                      </div>
                    {/if}
                    <div class="mt-3 rounded-lg bg-blue-50 p-3 text-xs leading-relaxed text-blue-700 dark:bg-blue-900/20 dark:text-blue-300">
                      三级可见性：<b>状态</b>（点亮图标）→ <b>预览</b>（围观沙箱屏幕）→ <b>接管</b>（全屏操作后交还）。
                      切换到「工作台」标签页围观或接管 Bot 的电脑。
                    </div>
                  {/if}

                  <!-- 工作台（串流） -->
                  {#if tab === "workbench"}
                    <div class="flex h-full min-h-[400px] flex-col gap-2">
                      {#if detail.status !== "running"}
                        <div class="flex flex-1 items-center justify-center rounded-lg border border-dashed border-gray-300 text-sm text-gray-400 dark:border-gray-600">
                          沙箱未运行，请先启动
                        </div>
                      {:else if detail.my_role === "viewer"}
                        <div class="flex flex-1 items-center justify-center rounded-lg border border-dashed border-gray-300 text-sm text-gray-400 dark:border-gray-600">
                          你的角色只能查看状态，无围观权限
                        </div>
                      {:else}
                        {#if detail.my_role === "admin"}
                          <div class="flex shrink-0 items-center justify-between rounded-lg bg-white px-3 py-2 dark:bg-gray-800">
                            <div class="flex items-center gap-2 text-xs">
                              {#if iOwnTakeover}
                                <span class="text-green-600 dark:text-green-400">你正在接管</span>
                              {:else if someoneElseTook}
                                <span class="text-yellow-600 dark:text-yellow-400">只读围观（{detail.taken_over_by_name || "其他人"} 接管中）</span>
                              {:else}
                                <span class="text-gray-400">空闲</span>
                              {/if}
                            </div>
                            <div class="flex gap-1.5">
                              {#if iOwnTakeover}
                                <Button variant="outline" size="sm" onclick={handleRelease}>交还</Button>
                              {:else}
                                <Button variant="primary" size="sm" onclick={handleTakeover} disabled={someoneElseTook}>接管</Button>
                              {/if}
                            </div>
                          </div>
                        {/if}
                        <div class="relative min-h-0 flex-1 overflow-hidden rounded-lg border border-gray-200 bg-black dark:border-gray-700">
                          <iframe
                            src={streamUrl}
                            class="h-full w-full"
                            title="bot-workbench"
                            allow="clipboard-read; clipboard-write"
                          ></iframe>
                        </div>
                      {/if}
                    </div>
                  {/if}

                  <!-- 工具（沙箱执行） -->
                  {#if tab === "tools"}
                    {#if detail.my_role !== "admin"}
                      <div class="text-sm text-gray-400">工具调用需要 Bot 管理权限（admin 角色）</div>
                    {:else}
                      <div class="flex h-full flex-col gap-2">
                        <div class="flex items-center gap-2">
                          <Select
                            value={execLanguage}
                            onchange={(v) => (execLanguage = v)}
                            options={[{ value: "bash", label: "Bash" }, { value: "python", label: "Python" }]}
                          />
                          <span class="text-xs text-gray-400">脚本在 Bot 沙箱容器内执行（超时 60s / 输出 64KB 截断）</span>
                        </div>
                        <textarea
                          bind:value={execScript}
                          rows="6"
                          placeholder={execLanguage === "bash" ? "ls -la /workspace" : "import os\nprint(os.getcwd())"}
                          class="w-full rounded-lg border border-gray-300 bg-white p-3 font-mono text-xs text-gray-800 focus:border-blue-400 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                        ></textarea>
                        <div>
                          <Button variant="primary" size="sm" onclick={handleExec} loading={execRunning} disabled={!execScript.trim()}>
                            <Icon icon="mdi:play" class="mr-1" />执行
                          </Button>
                        </div>
                        {#if execResult}
                          <div class="min-h-0 flex-1 overflow-auto rounded-lg bg-gray-900 p-3 font-mono text-xs text-gray-100">
                            <div class="mb-1 {execResult.exit_code === 0 ? 'text-green-400' : 'text-red-400'}">
                              退出码 {execResult.exit_code}{execResult.truncated ? "（输出已截断）" : ""}
                            </div>
                            <pre class="whitespace-pre-wrap break-all">{execResult.output}</pre>
                          </div>
                        {/if}
                      </div>
                    {/if}
                  {/if}

                  <!-- 成员 -->
                  {#if tab === "members"}
                    <div class="space-y-3 text-sm">
                      <div class="flex items-center gap-2">
                        <Select
                          value={addUserId}
                          onchange={(v) => (addUserId = v)}
                          placeholder="选择成员"
                          options={users.filter((u) => !memberDraft.some((m) => m.user_id === u.id)).map((u) => ({ value: u.id, label: u.nickname || u.username }))}
                        />
                        <Select value={addRole} onchange={(v) => (addRole = v)} options={roleOptions} />
                        <Button variant="outline" size="sm" onclick={addMember} disabled={!addUserId}>添加</Button>
                        <Button variant="primary" size="sm" onclick={saveMembers}>保存</Button>
                      </div>
                      <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-700">
                        <table class="w-full">
                          <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-gray-800">
                            <tr><th class="px-3 py-2">成员</th><th class="px-3 py-2">角色</th><th class="px-3 py-2"></th></tr>
                          </thead>
                          <tbody class="divide-y divide-gray-100 bg-white dark:divide-gray-700 dark:bg-gray-800">
                            {#each memberDraft as m (m.user_id)}
                              <tr>
                                <td class="px-3 py-2 text-gray-700 dark:text-gray-200">{usernameOf(m.user_id)}</td>
                                <td class="px-3 py-2">
                                  <select
                                    value={m.role}
                                    onchange={(e) => setMemberRole(m.user_id, (e.currentTarget as HTMLSelectElement).value)}
                                    class="rounded border border-gray-300 bg-white px-2 py-1 text-xs dark:border-gray-600 dark:bg-gray-700"
                                  >
                                    {#each roleOptions as ro}
                                      <option value={ro.value}>{ro.label}</option>
                                    {/each}
                                  </select>
                                </td>
                                <td class="px-3 py-2 text-right">
                                  <button class="text-xs text-red-500 hover:underline" onclick={() => removeMember(m.user_id)}>移除</button>
                                </td>
                              </tr>
                            {/each}
                          </tbody>
                        </table>
                      </div>
                      <p class="text-xs text-gray-400">
                        留一个 admin 角色成员，否则除平台管理员外无人能管理此 Bot。
                      </p>
                    </div>
                  {/if}

                  <!-- 渠道绑定（P3：网关路由） -->
                  {#if tab === "channels"}
                    <div class="space-y-3 text-sm">
                      <div class="flex flex-wrap items-center gap-2">
                        <Select value={newChannelPlatform} onchange={(v) => (newChannelPlatform = v)} options={platformOptions} />
                        <div class="w-40">
                          <Input bind:value={newChatId} placeholder="Chat ID（群/会话）" />
                        </div>
                        <div class="w-40">
                          <Input bind:value={newUserId} placeholder="用户 ID（可选）" />
                        </div>
                        <Button variant="outline" size="sm" onclick={addChannel}>添加</Button>
                        <Button variant="primary" size="sm" onclick={saveChannels}>保存</Button>
                      </div>
                      <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-700">
                        <table class="w-full">
                          <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-gray-800">
                            <tr><th class="px-3 py-2">平台</th><th class="px-3 py-2">Chat ID</th><th class="px-3 py-2">用户 ID</th><th class="px-3 py-2"></th></tr>
                          </thead>
                          <tbody class="divide-y divide-gray-100 bg-white dark:divide-gray-700 dark:bg-gray-800">
                            {#each channelDraft as ch, i (i)}
                              <tr>
                                <td class="px-3 py-2 text-gray-700 dark:text-gray-200">{platformOptions.find((p) => p.value === ch.platform)?.label ?? ch.platform}</td>
                                <td class="px-3 py-2 text-gray-700 dark:text-gray-200">{ch.chat_id || "—"}</td>
                                <td class="px-3 py-2 text-gray-700 dark:text-gray-200">{ch.user_id || "—"}</td>
                                <td class="px-3 py-2 text-right">
                                  <button class="text-xs text-red-500 hover:underline" onclick={() => removeChannel(i)}>移除</button>
                                </td>
                              </tr>
                            {:else}
                              <tr>
                                <td colspan="4" class="px-3 py-4 text-center text-xs text-gray-400">
                                  未绑定渠道；绑定后 IM 网关收到的匹配消息将由该 Bot 应答
                                </td>
                              </tr>
                            {/each}
                          </tbody>
                        </table>
                      </div>
                      <p class="text-xs text-gray-400">
                        匹配规则：平台一致且 Chat ID 或用户 ID 命中任一即路由到该 Bot；需在「AI 助手 → 消息网关」中启用对应平台。
                      </p>
                    </div>
                  {/if}

                  <!-- 技能（P3：SKILL.md 文件化） -->
                  {#if tab === "skills"}
                    <div class="space-y-3 text-sm">
                      <div class="flex items-center justify-between">
                        <span class="text-xs text-gray-400">技能目录：/var/lib/rde/skills/&lt;名称&gt;/SKILL.md（重启或点击扫描后生效）</span>
                        <Button variant="outline" size="sm" onclick={handleReloadSkills}>
                          <Icon icon="mdi:refresh" class="mr-1" />重新扫描
                        </Button>
                      </div>
                      <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-700">
                        <table class="w-full">
                          <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-gray-800">
                            <tr><th class="px-3 py-2">技能名</th><th class="px-3 py-2">说明</th></tr>
                          </thead>
                          <tbody class="divide-y divide-gray-100 bg-white dark:divide-gray-700 dark:bg-gray-800">
                            {#each skills as sk (sk.name)}
                              <tr>
                                <td class="px-3 py-2 font-medium text-gray-700 dark:text-gray-200">{sk.name}</td>
                                <td class="px-3 py-2 text-gray-500 dark:text-gray-400">{sk.description || "—"}</td>
                              </tr>
                            {:else}
                              <tr>
                                <td colspan="2" class="px-3 py-4 text-center text-xs text-gray-400">暂无技能；可在数据目录创建 SKILL.md</td>
                              </tr>
                            {/each}
                          </tbody>
                        </table>
                      </div>
                      <p class="text-xs leading-relaxed text-gray-400">
                        Bot 对话时先看到技能名与简介，需要时自动读取技能详情并在自己的沙箱电脑里执行脚本。
                      </p>
                    </div>
                  {/if}

                  <!-- 日志 -->
                  {#if tab === "logs"}
                    <div class="flex h-full flex-col gap-2">
                      <div>
                        <Button variant="outline" size="sm" onclick={() => { const d = detail; if (d) loadLogs(d.id); }}>
                          <Icon icon="mdi:refresh" class="mr-1" />刷新
                        </Button>
                      </div>
                      <div class="min-h-0 flex-1 overflow-auto rounded-lg bg-gray-900 p-3 font-mono text-xs text-gray-100">
                        <pre class="whitespace-pre-wrap break-all">{logs || "点击刷新获取沙箱日志"}</pre>
                      </div>
                    </div>
                  {/if}
                </div>
                {/if}
              {/snippet}
            </Tabs>
          </div>
        {/if}
      </section>
    {/if}
  </div>
</div>

<!-- 创建弹窗 -->
<Modal bind:open={showCreate} title="创建 Bot" size="md">
  <div class="space-y-3">
    <div>
      <label class="mb-1 block text-xs text-gray-500">名字 *</label>
      <Input bind:value={createForm.name} placeholder="如：小明（日报助手）" />
    </div>
    <div>
      <label class="mb-1 block text-xs text-gray-500">职责</label>
      <Input bind:value={createForm.description} placeholder="如：每天早上汇总团队日志并发周报到群里" />
    </div>
    <div>
      <label class="mb-1 block text-xs text-gray-500">模型</label>
      <Input bind:value={createForm.model} placeholder="如：qwen3:8b（留空稍后配置）" />
    </div>
    <div>
      <label class="mb-1 block text-xs text-gray-500">沙箱镜像</label>
      <Input bind:value={createForm.image} placeholder="rde/bot-desktop:latest（默认）" />
    </div>
    <div class="flex justify-end gap-2 pt-2">
      <Button variant="ghost" onclick={() => (showCreate = false)}>取消</Button>
      <Button variant="primary" onclick={handleCreate} loading={working} disabled={!createForm.name.trim()}>
        创建
      </Button>
    </div>
  </div>
</Modal>

<!-- 编辑弹窗 -->
<Modal bind:open={showEdit} title="编辑 Bot" size="md">
  <div class="space-y-3">
    <div>
      <label class="mb-1 block text-xs text-gray-500">名字</label>
      <Input bind:value={editForm.name} />
    </div>
    <div>
      <label class="mb-1 block text-xs text-gray-500">职责</label>
      <Input bind:value={editForm.description} />
    </div>
    <div>
      <label class="mb-1 block text-xs text-gray-500">模型</label>
      <Input bind:value={editForm.model} />
    </div>
    <div class="flex justify-end gap-2 pt-2">
      <Button variant="ghost" onclick={() => (showEdit = false)}>取消</Button>
      <Button variant="primary" onclick={handleSaveEdit} loading={working}>保存</Button>
    </div>
  </div>
</Modal>
