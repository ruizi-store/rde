<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "svelte-i18n";
  import Icon from "@iconify/svelte";
  import { Modal, Button, Input, Switch, Card, Tabs, Spinner, EmptyState, Badge, Tooltip, ComboBox } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import {
    sambaService,
    type SambaShare,
    type SambaUser,
    type SambaSession,
    type SambaGlobalConfig,
    type ServiceStatus,
  } from "./service";

  // ==================== 状态 ====================

  let serviceStatus = $state<ServiceStatus | null>(null);
  let shares = $state<SambaShare[]>([]);
  let users = $state<SambaUser[]>([]);
  let sessions = $state<SambaSession[]>([]);
  let config = $state<SambaGlobalConfig | null>(null);
  let loading = $state(true);
  let activeTab = $state("shares");

  // 添加共享
  let showAddShare = $state(false);
  let newShareName = $state("");
  let newSharePath = $state("");
  let newShareComment = $state("");
  let newShareWritable = $state(true);
  let newShareGuestOk = $state(false);

  // 添加用户
  let showAddUser = $state(false);
  let newUsername = $state("");
  let newPassword = $state("");
  let systemUsers = $state<string[]>([]);

  // 修改密码
  let showChangePassword = $state(false);
  let changePasswordUser = $state("");
  let changePasswordValue = $state("");

  // 编辑配置
  let showEditConfig = $state(false);
  let editWorkgroup = $state("");
  let editServerString = $state("");

  let tabItems = $derived([
    { id: "shares", label: $t("samba.tabShares"), icon: "mdi:folder-network-outline" },
    { id: "users", label: $t("samba.tabUsers"), icon: "mdi:account-group-outline" },
    { id: "sessions", label: $t("samba.tabSessions"), icon: "mdi:lan-connect" },
    { id: "settings", label: $t("samba.tabSettings"), icon: "mdi:cog-outline" },
  ]);

  // ==================== 生命周期 ====================

  onMount(() => { refresh(); });

  // ==================== 方法 ====================

  async function refresh() {
    try {
      const [rSt, rSh, rU, rSe, rC] = await Promise.allSettled([
        sambaService.getServiceStatus(),
        sambaService.listShares(),
        sambaService.listUsers(),
        sambaService.getSessions(),
        sambaService.getGlobalConfig(),
      ]);
      if (rSt.status === "fulfilled") serviceStatus = rSt.value;
      if (rSh.status === "fulfilled") shares = rSh.value;
      if (rU.status === "fulfilled") users = rU.value;
      if (rSe.status === "fulfilled") sessions = rSe.value.sessions ?? [];
      if (rC.status === "fulfilled") config = rC.value;
    } catch {}
    finally { loading = false; }
  }

  async function toggleService() {
    try {
      if (serviceStatus?.running) {
        await sambaService.stopService();
        showToast($t("samba.stopped"), "success");
      } else {
        await sambaService.startService();
        showToast($t("samba.started"), "success");
      }
      await refresh();
    } catch (e: any) { showToast(e.message, "error"); }
  }

  async function addShare() {
    if (!newShareName || !newSharePath) return;
    try {
      await sambaService.createShare({
        name: newShareName,
        path: newSharePath,
        comment: newShareComment || undefined,
        writable: newShareWritable,
        guest_ok: newShareGuestOk,
        browseable: true,
      });
      showAddShare = false;
      newShareName = ""; newSharePath = ""; newShareComment = "";
      newShareWritable = true; newShareGuestOk = false;
      showToast($t("samba.shareCreated"), "success");
      await refresh();
    } catch (e: any) { showToast($t("samba.createFailed") + ": " + e.message, "error"); }
  }

  async function deleteShare(name: string) {
    try {
      await sambaService.deleteShare(name);
      showToast($t("samba.shareDeleted"), "success");
      await refresh();
    } catch (e: any) { showToast($t("samba.deleteFailed") + ": " + e.message, "error"); }
  }

  async function addUser() {
    if (!newUsername || !newPassword) return;
    try {
      await sambaService.addUser({ username: newUsername, password: newPassword });
      showAddUser = false;
      newUsername = ""; newPassword = "";
      showToast($t("samba.userAdded"), "success");
      await refresh();
    } catch (e: any) { showToast($t("samba.addFailed") + ": " + e.message, "error"); }
  }

  async function openAddUser() {
    showAddUser = true;
    try {
      systemUsers = await sambaService.getSystemUsers();
    } catch {
      systemUsers = [];
    }
  }

  async function deleteUser(username: string) {
    try {
      await sambaService.deleteUser(username);
      showToast($t("samba.userDeleted"), "success");
      await refresh();
    } catch (e: any) { showToast(e.message, "error"); }
  }

  async function changePassword() {
    if (!changePasswordUser || !changePasswordValue) return;
    try {
      await sambaService.setUserPassword(changePasswordUser, changePasswordValue);
      showChangePassword = false;
      changePasswordUser = ""; changePasswordValue = "";
      showToast($t("samba.passwordUpdated"), "success");
    } catch (e: any) { showToast(e.message, "error"); }
  }

  async function killSession(pid: number) {
    try {
      await sambaService.killSession(pid);
      showToast($t("samba.sessionKilled"), "success");
      await refresh();
    } catch (e: any) { showToast(e.message, "error"); }
  }

  async function saveConfig() {
    try {
      await sambaService.updateGlobalConfig({
        workgroup: editWorkgroup,
        server_string: editServerString,
      });
      showEditConfig = false;
      showToast($t("samba.configSaved"), "success");
      await refresh();
    } catch (e: any) { showToast(e.message, "error"); }
  }

  function openEditConfig() {
    editWorkgroup = config?.workgroup ?? "WORKGROUP";
    editServerString = config?.server_string ?? "";
    showEditConfig = true;
  }
</script>

<div class="samba-app">
  <!-- ===== 顶部栏 ===== -->
  <header class="app-header">
    <div class="header-left">
      <Icon icon="mdi:folder-network" width="20" />
      <span class="app-title">{$t("samba.shareManagement")}</span>
      <span class="status-pill" class:running={serviceStatus?.running}>
        <span class="status-dot"></span>
        {#if loading}{$t("samba.checking")}{:else}{serviceStatus?.running ? $t("samba.running") : $t("samba.stoppedStatus")}{/if}
      </span>
    </div>
    {#if !loading}
      <Button
        variant={serviceStatus?.running ? "ghost" : "primary"}
        size="sm"
        onclick={toggleService}
      >
        {#snippet icon()}<Icon icon={serviceStatus?.running ? "mdi:stop" : "mdi:play"} width="14" />{/snippet}
        {serviceStatus?.running ? $t("samba.stopService") : $t("samba.startService")}
      </Button>
    {/if}
  </header>

  {#if loading}
    <div class="loading-area">
      <Spinner size="lg" center />
    </div>
  {:else}
    <!-- ===== Tabs + 内容 ===== -->
    <Tabs tabs={tabItems} bind:activeTab variant="underline" size="md">
      {#snippet children(tab)}
        <div class="panel-content">

          {#if tab === "shares"}
            <!-- ===== 共享面板 ===== -->
            <div class="panel-toolbar">
              <h3>{$t("samba.networkShares")}</h3>
              <Button variant="primary" size="sm" onclick={() => (showAddShare = true)}>
                {#snippet icon()}<Icon icon="mdi:plus" width="14" />{/snippet}
                {$t("samba.addShare")}
              </Button>
            </div>

            {#if shares.length === 0}
              <EmptyState
                icon="mdi:folder-network-outline"
                title={$t("samba.noShares")}
                description={$t("samba.noSharesHint")}
                actionLabel={$t("samba.addShare")}
                onaction={() => (showAddShare = true)}
              />
            {:else}
              <Card padding="none" bordered>
                <table class="data-table">
                  <thead>
                    <tr>
                      <th>{$t("samba.name")}</th>
                      <th>{$t("samba.path")}</th>
                      <th>{$t("samba.description")}</th>
                      <th class="center">{$t("samba.writable")}</th>
                      <th class="center">{$t("samba.guest")}</th>
                      <th class="center">{$t("samba.browsable")}</th>
                      <th class="right">{$t("samba.actions")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each shares as share (share.name)}
                      <tr>
                        <td class="name-cell">
                          <Icon icon="mdi:folder-outline" width="16" />
                          {share.name}
                        </td>
                        <td><code>{share.path}</code></td>
                        <td class="text-muted">{share.comment || "—"}</td>
                        <td class="center">
                          <Icon icon={share.writable ? "mdi:check-circle" : "mdi:close-circle-outline"}
                            width="16" class={share.writable ? "icon-yes" : "icon-no"} />
                        </td>
                        <td class="center">
                          <Icon icon={share.guest_ok ? "mdi:check-circle" : "mdi:close-circle-outline"}
                            width="16" class={share.guest_ok ? "icon-yes" : "icon-no"} />
                        </td>
                        <td class="center">
                          <Icon icon={share.browseable ? "mdi:check-circle" : "mdi:close-circle-outline"}
                            width="16" class={share.browseable ? "icon-yes" : "icon-no"} />
                        </td>
                        <td class="right">
                          <Tooltip text={$t("samba.deleteShare")}>
                            <button class="icon-btn danger" onclick={() => deleteShare(share.name)}>
                              <Icon icon="mdi:delete-outline" width="16" />
                            </button>
                          </Tooltip>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </Card>
            {/if}

          {:else if tab === "users"}
            <!-- ===== 用户面板 ===== -->
            <div class="panel-toolbar">
              <h3>{$t("samba.sambaUsers")}</h3>
              <Button variant="primary" size="sm" onclick={openAddUser}>
                {#snippet icon()}<Icon icon="mdi:plus" width="14" />{/snippet}
                {$t("samba.addUser")}
              </Button>
            </div>

            {#if users.length === 0}
              <EmptyState
                icon="mdi:account-group-outline"
                title={$t("samba.noUsers")}
                description={$t("samba.noUsersHint")}
                actionLabel={$t("samba.addUser")}
                onaction={openAddUser}
              />
            {:else}
              <Card padding="none" bordered>
                <table class="data-table">
                  <thead>
                    <tr>
                      <th>{$t("samba.username")}</th>
                      <th class="center">{$t("samba.status")}</th>
                      <th class="right">{$t("samba.actions")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each users as user (user.username)}
                      <tr>
                        <td class="name-cell">
                          <Icon icon="mdi:account" width="16" />
                          {user.username}
                        </td>
                        <td class="center">
                          <span class="pill" class:enabled={user.enabled !== false}>
                            {user.enabled !== false ? $t("samba.enabled") : $t("samba.disabled")}
                          </span>
                        </td>
                        <td class="right actions-cell">
                          <Tooltip text={$t("samba.changePassword")}>
                            <button class="icon-btn" onclick={() => { changePasswordUser = user.username; showChangePassword = true; }}>
                              <Icon icon="mdi:key-outline" width="16" />
                            </button>
                          </Tooltip>
                          <Tooltip text={$t("samba.deleteUser")}>
                            <button class="icon-btn danger" onclick={() => deleteUser(user.username)}>
                              <Icon icon="mdi:delete-outline" width="16" />
                            </button>
                          </Tooltip>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </Card>
            {/if}

          {:else if tab === "sessions"}
            <!-- ===== 会话面板 ===== -->
            <div class="panel-toolbar">
              <h3>{$t("samba.activeSessions")}</h3>
              <Button variant="ghost" size="sm" onclick={refresh}>
                {#snippet icon()}<Icon icon="mdi:refresh" width="14" />{/snippet}
                {$t("common.refresh")}
              </Button>
            </div>

            {#if sessions.length === 0}
              <EmptyState
                icon="mdi:lan-disconnect"
                title={$t("samba.noSessions")}
                description={$t("samba.noSessionsHint")}
              />
            {:else}
              <Card padding="none" bordered>
                <table class="data-table">
                  <thead>
                    <tr>
                      <th>{$t("samba.user")}</th>
                      <th>{$t("samba.source")}</th>
                      <th>PID</th>
                      <th>{$t("samba.protocol")}</th>
                      <th class="right">{$t("samba.actions")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each sessions as session (session.pid)}
                      <tr>
                        <td class="name-cell">
                          <Icon icon="mdi:account-circle-outline" width="16" />
                          {session.username}
                        </td>
                        <td><code>{session.machine}</code></td>
                        <td class="mono">{session.pid}</td>
                        <td>{session.protocol || "—"}</td>
                        <td class="right">
                          <Button variant="ghost" size="sm" onclick={() => killSession(session.pid)}>{$t("samba.disconnect")}</Button>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </Card>
            {/if}

          {:else if tab === "settings"}
            <!-- ===== 设置面板 ===== -->
            <div class="panel-toolbar">
              <h3>{$t("samba.globalConfig")}</h3>
              <Button variant="ghost" size="sm" onclick={openEditConfig}>
                {#snippet icon()}<Icon icon="mdi:pencil-outline" width="14" />{/snippet}
                {$t("common.edit")}
              </Button>
            </div>

            {#if config}
              <div class="settings-grid">
                <Card bordered padding="md">
                  <div class="setting-row">
                    <div class="setting-icon"><Icon icon="mdi:domain" width="20" /></div>
                    <div class="setting-body">
                      <div class="setting-label">{$t("samba.workgroup")}</div>
                      <div class="setting-value">{config.workgroup}</div>
                    </div>
                  </div>
                </Card>
                <Card bordered padding="md">
                  <div class="setting-row">
                    <div class="setting-icon"><Icon icon="mdi:text-box-outline" width="20" /></div>
                    <div class="setting-body">
                      <div class="setting-label">{$t("samba.serverDescription")}</div>
                      <div class="setting-value">{config.server_string || "—"}</div>
                    </div>
                  </div>
                </Card>
                {#if config.netbios_name}
                  <Card bordered padding="md">
                    <div class="setting-row">
                      <div class="setting-icon"><Icon icon="mdi:desktop-classic" width="20" /></div>
                      <div class="setting-body">
                        <div class="setting-label">{$t("samba.netbiosName")}</div>
                        <div class="setting-value">{config.netbios_name}</div>
                      </div>
                    </div>
                  </Card>
                {/if}
                {#if serviceStatus?.version}
                  <Card bordered padding="md">
                    <div class="setting-row">
                      <div class="setting-icon"><Icon icon="mdi:information-outline" width="20" /></div>
                      <div class="setting-body">
                        <div class="setting-label">{$t("samba.sambaVersion")}</div>
                        <div class="setting-value">{serviceStatus.version}</div>
                      </div>
                    </div>
                  </Card>
                {/if}
              </div>
            {/if}
          {/if}

        </div>
      {/snippet}
    </Tabs>
  {/if}
</div>

<!-- ===== 模态框：添加共享 ===== -->
<Modal bind:open={showAddShare} title={$t("samba.addShareTitle")} size="md">
  <div class="modal-form">
    <Input label={$t("samba.shareName")} bind:value={newShareName} placeholder={$t("samba.shareNamePlaceholder")} />
    <Input label={$t("samba.sharePath")} bind:value={newSharePath} placeholder="/srv/samba/public" />
    <Input label={$t("samba.description")} bind:value={newShareComment} placeholder={$t("samba.optional")} />
    <div class="form-switches">
      <Switch bind:checked={newShareWritable} label={$t("samba.allowWrite")} />
      <Switch bind:checked={newShareGuestOk} label={$t("samba.allowGuest")} />
    </div>
    <div class="form-actions">
      <Button variant="ghost" onclick={() => (showAddShare = false)}>{$t("common.cancel")}</Button>
      <Button variant="primary" onclick={addShare}>{$t("samba.createShare")}</Button>
    </div>
  </div>
</Modal>

<!-- ===== 模态框：添加用户 ===== -->
<Modal bind:open={showAddUser} title={$t("samba.addUserTitle")} size="sm">
  <div class="modal-form">
    <ComboBox label={$t("samba.username")} bind:value={newUsername} options={systemUsers} placeholder={$t("samba.usernamePlaceholder")} />
    <Input label={$t("samba.password")} type="password" bind:value={newPassword} />
    <div class="form-actions">
      <Button variant="ghost" onclick={() => (showAddUser = false)}>{$t("common.cancel")}</Button>
      <Button variant="primary" onclick={addUser}>{$t("samba.addUser")}</Button>
    </div>
  </div>
</Modal>

<!-- ===== 模态框：修改密码 ===== -->
<Modal bind:open={showChangePassword} title="{$t('samba.changePasswordTitle')} — {changePasswordUser}" size="sm">
  <div class="modal-form">
    <Input label={$t("samba.newPassword")} type="password" bind:value={changePasswordValue} />
    <div class="form-actions">
      <Button variant="ghost" onclick={() => (showChangePassword = false)}>{$t("common.cancel")}</Button>
      <Button variant="primary" onclick={changePassword}>{$t("samba.confirmChange")}</Button>
    </div>
  </div>
</Modal>

<!-- ===== 模态框：编辑配置 ===== -->
<Modal bind:open={showEditConfig} title={$t("samba.editConfigTitle")} size="md">
  <div class="modal-form">
    <Input label={$t("samba.workgroup")} bind:value={editWorkgroup} />
    <Input label={$t("samba.serverDescription")} bind:value={editServerString} />
    <div class="form-actions">
      <Button variant="ghost" onclick={() => (showEditConfig = false)}>{$t("common.cancel")}</Button>
      <Button variant="primary" onclick={saveConfig}>{$t("samba.saveConfig")}</Button>
    </div>
  </div>
</Modal>

<style>
  /* ===== 根布局 ===== */
  .samba-app {
    display: flex;
    flex-direction: column;
    height: 100%;
    font-size: 13px;
    color: var(--text-primary, #1a1a1a);
    background: var(--bg-window, #f3f3f3);
  }

  /* ===== 顶栏 ===== */
  .app-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 20px;
    background: var(--bg-card, rgba(255,255,255,0.7));
    border-bottom: 1px solid var(--border-light, rgba(0,0,0,0.06));
    flex-shrink: 0;
  }
  .header-left {
    display: flex;
    align-items: center;
    gap: 10px;
    color: var(--color-primary, #0078d4);
  }
  .app-title {
    font-size: 15px;
    font-weight: 600;
    color: var(--text-primary, #1a1a1a);
  }

  /* 状态药丸 */
  .status-pill {
    display: inline-flex;
    align-items: center;
    padding: 2px 10px;
    font-size: 11px;
    font-weight: 500;
    border-radius: 99px;
    background: rgba(209,52,56,0.08);
    color: #d13438;
  }
  .status-pill.running {
    background: rgba(16,124,16,0.08);
    color: #107c10;
  }
  .status-dot {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
    margin-right: 5px;
  }

  /* ===== 加载区 ===== */
  .loading-area {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  /* ===== 面板内容 ===== */
  .panel-content {
    padding: 16px 20px 24px;
  }

  /* ===== 面板工具栏 ===== */
  .panel-toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 14px;
  }
  .panel-toolbar h3 {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
  }

  /* ===== 数据表格 ===== */
  .data-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }
  .data-table thead {
    background: var(--bg-secondary, rgba(0,0,0,0.025));
  }
  .data-table th {
    padding: 10px 14px;
    text-align: left;
    font-weight: 600;
    font-size: 12px;
    color: var(--text-secondary, #616161);
    border-bottom: 1px solid var(--border-light, rgba(0,0,0,0.06));
    white-space: nowrap;
    user-select: none;
  }
  .data-table td {
    padding: 8px 14px;
    border-bottom: 1px solid rgba(0,0,0,0.04);
    vertical-align: middle;
  }
  .data-table tbody tr {
    transition: background 0.1s;
  }
  .data-table tbody tr:hover {
    background: var(--bg-hover, rgba(0,0,0,0.024));
  }
  .data-table tbody tr:last-child td {
    border-bottom: none;
  }
  .center { text-align: center; }
  .right { text-align: right; }
  .name-cell {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 500;
    white-space: nowrap;
  }
  .text-muted {
    color: var(--text-secondary, #616161);
    max-width: 180px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .actions-cell {
    display: flex;
    justify-content: flex-end;
    gap: 2px;
  }
  .mono {
    font-family: "Cascadia Code", "Consolas", monospace;
    font-size: 12px;
  }
  code {
    font-family: "Cascadia Code", "Consolas", monospace;
    font-size: 12px;
    padding: 1px 5px;
    border-radius: 3px;
    background: rgba(0,0,0,0.04);
    color: var(--text-secondary, #616161);
  }

  :global(.icon-yes) { color: #107c10; }
  :global(.icon-no) { color: #c4c4c4; }

  /* ===== 图标按钮 ===== */
  .icon-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    padding: 0;
    color: var(--text-secondary, #616161);
    background: transparent;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    transition: background 0.1s, color 0.1s;
  }
  .icon-btn:hover {
    background: rgba(0,0,0,0.06);
    color: var(--text-primary, #1a1a1a);
  }
  .icon-btn.danger:hover {
    background: rgba(209,52,56,0.08);
    color: #d13438;
  }

  /* ===== 状态药丸（表格内） ===== */
  .pill {
    display: inline-block;
    padding: 2px 10px;
    font-size: 11px;
    font-weight: 500;
    border-radius: 99px;
    background: rgba(0,0,0,0.06);
    color: var(--text-secondary, #616161);
  }
  .pill.enabled {
    background: rgba(16,124,16,0.08);
    color: #107c10;
  }

  /* ===== 设置网格 ===== */
  .settings-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 8px;
  }
  .setting-row {
    display: flex;
    align-items: center;
    gap: 14px;
  }
  .setting-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    border-radius: 8px;
    background: rgba(0,120,212,0.08);
    color: var(--color-primary, #0078d4);
    flex-shrink: 0;
  }
  .setting-body {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .setting-label {
    font-size: 12px;
    color: var(--text-secondary, #616161);
  }
  .setting-value {
    font-size: 13px;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* ===== 模态框表单 ===== */
  .modal-form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .form-switches {
    display: flex;
    gap: 24px;
    padding: 4px 0;
  }
  .form-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding-top: 4px;
  }
</style>
