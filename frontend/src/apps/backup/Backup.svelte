<script lang="ts">
  import { t } from "svelte-i18n";
  import { onMount, onDestroy } from "svelte";
  import Icon from "@iconify/svelte";
  import { Button, Modal, Spinner, EmptyState, Tabs, Switch, FolderBrowser } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import {
    backupService,
    formatBytes,
    getStatusLabel,
    getStatusColor,
    getTargetTypeLabel,
    getMigrateStatusLabel,
    getMigrateStatusColor,
    type BackupTask,
    type BackupRecord,
    type BackupOverview,
    type TargetType,
    type BackupType,
    type MigrateSession,
    type MigrateProgress,
    type MigrateContent,
  } from "./service";

  // ==================== 状态 ====================

  let overview = $state<BackupOverview | null>(null);
  let tasks = $state<BackupTask[]>([]);
  let records = $state<BackupRecord[]>([]);
  let loading = $state(true);
  let activeTab = $state("tasks");

  // 创建任务
  let showCreateTask = $state(false);
  let newTask = $state({
    name: "",
    description: "",
    type: "full" as BackupType,
    sources: [""],
    target_type: "local" as TargetType,
    schedule: "",
    scheduleMode: "manual" as "manual" | "preset" | "custom",
    retention: 7,
    compression: true,
    encryption: false,
    // 动态存储配置
    local: { path: "/var/backups/rde" },
    webdav: { url: "", username: "", password: "", path: "/" },
    s3: { endpoint: "", bucket: "", region: "us-east-1", access_key: "", secret_key: "", path: "/" },
    sftp: { host: "", port: 22, username: "", password: "", key_path: "", auth_type: "password" as "password" | "key", path: "/" },
  });
  let creating = $state(false);
  let testingConnection = $state(false);

  // 路径选择器
  let showFolderBrowser = $state(false);
  let folderBrowserCallback = $state<((path: string) => void) | null>(null);

  // 还原
  let showRestore = $state(false);
  let restoreRecordId = $state("");
  let restoreTargetPath = $state("");
  let restoring = $state(false);

  // P2P 迁移
  let showMigrate = $state(false);
  let migrateStep = $state(1); // 1: 选择角色, 2: 配对, 3: 选择内容, 4: 传输中
  let migrateRole = $state<"source" | "target" | null>(null);
  let migrateSession = $state<MigrateSession | null>(null);
  let migrateProgress = $state<MigrateProgress | null>(null);
  let migratePairCode = $state("");
  let migrateTargetUrl = $state("");
  let migrateContent = $state<MigrateContent>({
    system_config: true,
    users: true,
    docker: true,
    network: true,
    samba: true,
    files: [],
    apps: [],
  });
  let migrateLoading = $state(false);
  let migrateProgressTimer: ReturnType<typeof setInterval> | null = null;

  let refreshTimer: ReturnType<typeof setInterval>;

  const tabs = $derived([
    { id: "tasks", label: $t("backup.tasks") },
    { id: "records", label: $t("backup.records") },
    { id: "migrate", label: $t("backup.migrate") },
  ]);

  const backupTypes = $derived([
    { value: "full", label: $t("backup.full") },
    { value: "incremental", label: $t("backup.incremental") },
    { value: "config", label: $t("backup.config") },
  ]);

  const targetTypes = $derived([
    { value: "local", label: $t("backup.localStorage") },
    { value: "webdav", label: "WebDAV" },
    { value: "s3", label: "S3 / MinIO" },
    { value: "sftp", label: "SFTP" },
  ]);

  const schedulePresets = $derived([
    { value: "", label: $t("backup.manual") },
    { value: "0 0 2 * * *", label: $t("backup.daily2am") },
    { value: "0 0 3 * * *", label: $t("backup.daily3am") },
    { value: "0 0 0 * * 0", label: $t("backup.weeklySun") },
    { value: "0 0 0 1 * *", label: "Monthly 1st 0:00" },
    { value: "0 0 */6 * * *", label: "Every 6 hours" },
    { value: "0 0 */12 * * *", label: "Every 12 hours" },
  ]);

  // ==================== 生命周期 ====================

  onMount(() => {
    refresh();
    refreshTimer = setInterval(refresh, 5000);
  });

  onDestroy(() => {
    clearInterval(refreshTimer);
    if (migrateProgressTimer) clearInterval(migrateProgressTimer);
  });

  // ==================== 方法 ====================

  async function refresh() {
    try {
      const [o, t, r] = await Promise.all([
        backupService.getOverview(),
        backupService.getTasks(),
        backupService.getRecords(),
      ]);
      overview = o;
      tasks = t.data;
      records = r.data;
    } catch {}
    finally { loading = false; }
  }

  // 构建目标配置JSON
  function buildTargetConfig(): string {
    switch (newTask.target_type) {
      case "local":
        return JSON.stringify({ path: newTask.local.path });
      case "webdav":
        return JSON.stringify({
          url: newTask.webdav.url,
          username: newTask.webdav.username,
          password: newTask.webdav.password,
          path: newTask.webdav.path,
        });
      case "s3":
        return JSON.stringify({
          endpoint: newTask.s3.endpoint,
          bucket: newTask.s3.bucket,
          region: newTask.s3.region,
          access_key: newTask.s3.access_key,
          secret_key: newTask.s3.secret_key,
          path: newTask.s3.path,
        });
      case "sftp":
        return JSON.stringify({
          host: newTask.sftp.host,
          port: newTask.sftp.port,
          username: newTask.sftp.username,
          password: newTask.sftp.auth_type === "password" ? newTask.sftp.password : undefined,
          key_path: newTask.sftp.auth_type === "key" ? newTask.sftp.key_path : undefined,
          path: newTask.sftp.path,
        });
      default:
        return "{}";
    }
  }

  async function createTask() {
    if (!newTask.name || newTask.sources.length === 0 || !newTask.sources[0]) return;
    creating = true;
    try {
      await backupService.createTask({
        name: newTask.name,
        description: newTask.description,
        type: newTask.type,
        sources: newTask.sources.filter(s => s),
        target_type: newTask.target_type,
        target_config: buildTargetConfig(),
        schedule: newTask.schedule || undefined,
        retention: newTask.retention,
        compression: newTask.compression,
        encryption: newTask.encryption,
      });
      showCreateTask = false;
      resetNewTask();
      showToast($t("backup.taskCreated"), "success");
      await refresh();
    } catch (e: any) {
      showToast($t("backup.createFailed") + " " + e.message, "error");
    } finally { creating = false; }
  }

  async function testConnection() {
    testingConnection = true;
    try {
      const result = await backupService.testTarget({
        type: newTask.target_type,
        config: buildTargetConfig(),
      });
      if (result.success) {
        showToast($t("backup.connectionSuccess") + (result.message || ""), "success");
      } else {
        showToast($t("backup.connectionFailed") + " " + result.message, "error");
      }
    } catch (e: any) {
      showToast($t("backup.testFailed") + " " + e.message, "error");
    } finally {
      testingConnection = false;
    }
  }

  function openFolderBrowser(callback: (path: string) => void) {
    folderBrowserCallback = callback;
    showFolderBrowser = true;
  }

  function handleFolderSelect(path: string) {
    if (folderBrowserCallback) {
      folderBrowserCallback(path);
      folderBrowserCallback = null;
    }
    showFolderBrowser = false;
  }

  function resetNewTask() {
    newTask = {
      name: "",
      description: "",
      type: "full",
      sources: [""],
      target_type: "local",
      schedule: "",
      scheduleMode: "manual",
      retention: 7,
      compression: true,
      encryption: false,
      local: { path: "/var/backups/rde" },
      webdav: { url: "", username: "", password: "", path: "/" },
      s3: { endpoint: "", bucket: "", region: "us-east-1", access_key: "", secret_key: "", path: "/" },
      sftp: { host: "", port: 22, username: "", password: "", key_path: "", auth_type: "password", path: "/" },
    };
  }

  async function deleteTask(id: string) {
    try {
      await backupService.deleteTask(id);
      showToast($t("backup.taskDeleted"), "success");
      await refresh();
    } catch (e: any) { showToast($t("backup.deleteFailed") + " " + e.message, "error"); }
  }

  async function toggleTask(task: BackupTask) {
    try {
      await backupService.updateTask(task.id, { enabled: !task.enabled });
      await refresh();
    } catch (e: any) { showToast(e.message, "error"); }
  }

  async function runBackup(taskId: string) {
    try {
      await backupService.runTask(taskId);
      showToast($t("backup.backupStarted"), "success");
      await refresh();
    } catch (e: any) { showToast($t("backup.startFailed") + " " + e.message, "error"); }
  }

  async function deleteRecord(id: string) {
    try {
      await backupService.deleteRecord(id);
      showToast($t("backup.recordDeleted"), "success");
      await refresh();
    } catch (e: any) { showToast($t("backup.deleteFailed") + " " + e.message, "error"); }
  }

  async function restore() {
    if (!restoreRecordId) return;
    restoring = true;
    try {
      await backupService.restore({
        record_id: restoreRecordId,
        target_path: restoreTargetPath || undefined,
        overwrite: true,
      });
      showRestore = false;
      restoreRecordId = "";
      restoreTargetPath = "";
      showToast($t("backup.restoreStarted"), "success");
    } catch (e: any) { showToast($t("backup.restoreFailed") + " " + e.message, "error"); }
    finally { restoring = false; }
  }

  function formatDate(ds: string | undefined): string {
    if (!ds) return "-";
    return new Date(ds).toLocaleString("zh-CN");
  }

  function addSource() {
    newTask.sources = [...newTask.sources, ""];
  }

  function removeSource(index: number) {
    newTask.sources = newTask.sources.filter((_, i) => i !== index);
  }

  // ==================== 迁移相关方法 ====================

  function openMigrate(role: "source" | "target") {
    migrateRole = role;
    migrateStep = 2;
    showMigrate = true;
    
    if (role === "target") {
      startPairing();
    }
  }

  async function startPairing() {
    migrateLoading = true;
    try {
      const result = await backupService.generatePairCode();
      migrateSession = {
        id: result.session_id,
        pair_code: result.pair_code,
        role: "target",
        status: "pairing",
        expires_at: result.expires_at,
        created_at: new Date().toISOString(),
      };
      // 开始轮询状态
      startMigrateStatusPolling();
    } catch (e: any) {
      showToast($t("backup.generatePairCodeFailed") + " " + e.message, "error");
    } finally {
      migrateLoading = false;
    }
  }

  async function connectWithCode() {
    if (!migratePairCode || !migrateTargetUrl) {
      showToast($t("backup.enterPairCodeAndTarget"), "warning");
      return;
    }
    
    migrateLoading = true;
    try {
      // 先验证配对码
      await backupService.validatePairCode(migratePairCode);
      
      // 连接到目标
      const session = await backupService.connectToSource(migratePairCode, migrateTargetUrl);
      migrateSession = session;
      migrateStep = 3; // 选择内容
      
      // 开始轮询状态
      startMigrateStatusPolling();
    } catch (e: any) {
      showToast($t("backup.connectionFailed") + " " + e.message, "error");
    } finally {
      migrateLoading = false;
    }
  }

  async function startMigrateTransfer() {
    if (!migrateSession) return;
    
    migrateLoading = true;
    try {
      await backupService.startMigrateTransfer(migrateSession.id, migrateContent);
      migrateStep = 4; // 传输中
      showToast($t("backup.migrateStarted"), "success");
    } catch (e: any) {
      showToast($t("backup.startTransferFailed") + " " + e.message, "error");
    } finally {
      migrateLoading = false;
    }
  }

  function startMigrateStatusPolling() {
    if (migrateProgressTimer) clearInterval(migrateProgressTimer);
    
    migrateProgressTimer = setInterval(async () => {
      if (!migrateSession) return;
      
      try {
        const session = await backupService.getMigrateSession(migrateSession.id);
        // 保留原有的 pair_code（后端轮询不返回）
        const pairCode = migrateSession.pair_code;
        migrateSession = { ...session, pair_code: pairCode };
        
        if (session.status === "connected" && migrateRole === "target" && migrateStep === 2) {
          migrateStep = 3; // 对方已连接，等待选择内容
        }
        
        if (session.status === "transferring" || session.status === "completed") {
          const progress = await backupService.getMigrateProgress(migrateSession.id);
          migrateProgress = progress;
          
          if (session.status === "completed") {
            stopMigratePolling();
            showToast($t("backup.migrateCompleted"), "success");
          }
        }
        
        if (session.status === "failed" || session.status === "cancelled") {
          stopMigratePolling();
          if (session.status === "failed") {
            showToast($t("backup.migrateFailed"), "error");
          }
        }
      } catch {}
    }, 2000);
  }

  function stopMigratePolling() {
    if (migrateProgressTimer) {
      clearInterval(migrateProgressTimer);
      migrateProgressTimer = null;
    }
  }

  async function cancelMigrate() {
    if (!migrateSession) return;
    
    try {
      await backupService.cancelMigrate(migrateSession.id);
      showToast($t("backup.migrateCancelled"), "info");
    } catch {}
    
    closeMigrateModal();
  }

  function closeMigrateModal() {
    stopMigratePolling();
    showMigrate = false;
    migrateStep = 1;
    migrateRole = null;
    migrateSession = null;
    migrateProgress = null;
    migratePairCode = "";
    migrateTargetUrl = "";
  }

  function getMigrateProgressPercent(): number {
    if (!migrateProgress || migrateProgress.total_size === 0) return 0;
    return Math.round((migrateProgress.transferred_size / migrateProgress.total_size) * 100);
  }
</script>

<div class="backup-manager">
  <header class="header">
    <h2>{$t("backup.title")}</h2>
    {#if overview}
      <div class="stats-row">
        <span>{$t("backup.taskCount", { values: { n: overview.total_tasks } })}</span>
        <span>{$t("backup.recordCount", { values: { n: overview.total_records } })}</span>
        <span>{$t("backup.totalSize", { values: { size: formatBytes(overview.total_size) } })}</span>
        <span class="success-count">✓ {overview.success_count}</span>
        <span class="failed-count">✕ {overview.failed_count}</span>
      </div>
    {/if}
  </header>

  {#if loading}
    <Spinner center />
  {:else}
    <div class="content">
      <Tabs {tabs} bind:activeTab variant="underline" size="sm">
        {#snippet children(tab)}
          {#if tab === "tasks"}
            <div class="panel-header">
              <Button variant="primary" size="sm" onclick={() => (showCreateTask = true)}>
                <Icon icon="mdi:plus" width="16" /> {$t("backup.newTask")}
              </Button>
            </div>
            {#if tasks.length === 0}
              <EmptyState icon="mdi:backup-restore" title={$t("backup.noTasks")} description={$t("backup.createTaskToStart")} />
            {:else}
              <div class="task-list">
                {#each tasks as task (task.id)}
                  <div class="task-card">
                    <div class="task-top">
                      <div class="task-info">
                        <div class="task-name">{task.name}</div>
                        <div class="task-paths">
                          <span><Icon icon="mdi:folder-outline" width="14" /> {task.sources.join(", ")}</span>
                          <Icon icon="mdi:arrow-right" width="14" />
                          <span><Icon icon="mdi:cloud-outline" width="14" /> {getTargetTypeLabel(task.target_type)}</span>
                        </div>
                        {#if task.description}
                          <div class="task-desc">{task.description}</div>
                        {/if}
                      </div>
                      <div class="task-actions">
                        <Switch checked={task.enabled} size="sm" onchange={() => toggleTask(task)} />
                        <Button variant="ghost" size="sm" onclick={() => runBackup(task.id)}>
                          <Icon icon="mdi:play" width="16" />
                        </Button>
                        <Button variant="ghost" size="sm" onclick={() => deleteTask(task.id)}>
                          <Icon icon="mdi:delete-outline" width="16" />
                        </Button>
                      </div>
                    </div>
                    <div class="task-bottom">
                      <span class="type-badge">{task.type === 'full' ? $t("backup.fullType") : task.type === 'incremental' ? $t("backup.incrementalType") : $t("backup.configType")}</span>
                      {#if task.schedule}<span class="task-schedule">⏰ {task.schedule}</span>{/if}
                      {#if task.last_run_at}<span class="task-time">{$t("backup.lastRun")} {formatDate(task.last_run_at)}</span>{/if}
                      {#if task.next_run_at}<span class="task-time">{$t("backup.nextRun")} {formatDate(task.next_run_at)}</span>{/if}
                      <span class="task-retention">{$t("backup.keepCopies", { values: { n: task.retention } })}</span>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}

          {:else if tab === "records"}
            {#if records.length === 0}
              <EmptyState icon="mdi:history" title={$t("backup.noRecords")} />
            {:else}
              <div class="record-list">
                {#each records as record (record.id)}
                  <div class="record-card">
                    <div class="record-top">
                      <div class="record-info">
                        <span class="record-name">{record.task_name || record.task_id}</span>
                        <span class="status-badge {record.status}">{getStatusLabel(record.status)}</span>
                      </div>
                      <div class="record-actions">
                        {#if record.status === 'success'}
                          <Button variant="ghost" size="sm" onclick={() => { restoreRecordId = record.id; showRestore = true; }}>
                            <Icon icon="mdi:restore" width="16" /> {$t("backup.restoreBtn")}
                          </Button>
                        {/if}
                        <Button variant="ghost" size="sm" onclick={() => deleteRecord(record.id)}>
                          <Icon icon="mdi:delete-outline" width="16" />
                        </Button>
                      </div>
                    </div>
                    <div class="record-meta">
                      <span>{formatDate(record.started_at)}</span>
                      {#if record.size}<span>{formatBytes(record.size)}</span>{/if}
                      {#if record.file_count}<span>{$t("backup.filesCount", { values: { n: record.file_count } })}</span>{/if}
                      {#if record.status === 'running'}<span class="progress-text">{record.progress}%</span>{/if}
                      {#if record.message}<span class="message-text">{record.message}</span>{/if}
                      {#if record.error}<span class="error-text">{record.error}</span>{/if}
                    </div>
                  </div>
                {/each}
              </div>
            {/if}

          {:else if tab === "migrate"}
            <div class="migrate-panel">
              <div class="migrate-intro">
                <Icon icon="mdi:swap-horizontal" width="48" class="migrate-icon" />
                <h3>{$t("backup.migrateTitle")}</h3>
                <p>{$t("backup.migrateDescription")}</p>
              </div>
              
              <div class="migrate-options">
                <div class="migrate-card" role="button" tabindex="0" onclick={() => openMigrate("source")} onkeypress={(e) => e.key === "Enter" && openMigrate("source")}>
                  <Icon icon="mdi:upload" width="32" />
                  <div class="card-content">
                    <h4>{$t("backup.sendData")}</h4>
                    <p>{$t("backup.sendDataDesc")}</p>
                  </div>
                  <Icon icon="mdi:chevron-right" width="20" />
                </div>
                
                <div class="migrate-card" role="button" tabindex="0" onclick={() => openMigrate("target")} onkeypress={(e) => e.key === "Enter" && openMigrate("target")}>
                  <Icon icon="mdi:download" width="32" />
                  <div class="card-content">
                    <h4>{$t("backup.receiveData")}</h4>
                    <p>{$t("backup.receiveDataDesc")}</p>
                  </div>
                  <Icon icon="mdi:chevron-right" width="20" />
                </div>
              </div>
            </div>
          {/if}
        {/snippet}
      </Tabs>
    </div>
  {/if}
</div>
<!-- 创建任务 -->
<Modal bind:open={showCreateTask} title={$t("backup.newBackupTask")} size="lg">
  <form class="modal-form" onsubmit={(e) => { e.preventDefault(); createTask(); }}>
    <div class="form-row">
      <div class="form-group">
        <label for="task-name">{$t("backup.taskName")}</label>
        <input id="task-name" bind:value={newTask.name} required placeholder={$t("backup.dailyBackup")} />
      </div>
      <div class="form-group">
        <label for="task-type">{$t("backup.backupType")}</label>
        <select id="task-type" bind:value={newTask.type}>
          {#each backupTypes as t}
            <option value={t.value}>{t.label}</option>
          {/each}
        </select>
      </div>
    </div>

    <div class="form-group">
      <label>{$t("backup.sourcePathLabel")}</label>
      {#each newTask.sources as source, i}
        <div class="source-row">
          <input bind:value={newTask.sources[i]} required placeholder="/home/user/data" />
          <Button variant="ghost" size="sm" onclick={() => openFolderBrowser((p) => { newTask.sources[i] = p; })}>
            <Icon icon="mdi:folder-open-outline" width="16" />
          </Button>
          {#if newTask.sources.length > 1}
            <Button variant="ghost" size="sm" onclick={() => removeSource(i)}>
              <Icon icon="mdi:close" width="16" />
            </Button>
          {/if}
        </div>
      {/each}
      <Button variant="ghost" size="sm" onclick={addSource}>
        <Icon icon="mdi:plus" width="16" /> {$t("backup.addPath")}
      </Button>
    </div>

    <!-- 存储目标区域 -->
    <div class="section-title">
      <Icon icon="mdi:cloud-outline" width="16" /> {$t("backup.storageTarget")}
    </div>

    <div class="form-row">
      <div class="form-group">
        <label for="task-target">{$t("backup.storageType")}</label>
        <select id="task-target" bind:value={newTask.target_type}>
          {#each targetTypes as t}
            <option value={t.value}>{t.label}</option>
          {/each}
        </select>
      </div>
      <div class="form-group">
        <label for="task-retention">{$t("backup.retentionCount")}</label>
        <input id="task-retention" type="number" bind:value={newTask.retention} min="1" max="100" />
      </div>
    </div>

    <!-- 动态目标配置 -->
    <div class="target-config-section">
      {#if newTask.target_type === "local"}
        <div class="form-group">
          <label>{$t("backup.localPath")}</label>
          <div class="input-with-button">
            <input bind:value={newTask.local.path} required placeholder="/var/backups/rde" />
            <Button variant="ghost" size="sm" onclick={() => openFolderBrowser((p) => { newTask.local.path = p; })}>
              <Icon icon="mdi:folder-open-outline" width="16" />
            </Button>
          </div>
        </div>

      {:else if newTask.target_type === "webdav"}
        <div class="form-group">
          <label>WebDAV URL *</label>
          <input bind:value={newTask.webdav.url} required placeholder="https://dav.jianguoyun.com/dav/" />
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>{$t("backup.username")}</label>
            <input bind:value={newTask.webdav.username} required placeholder="username" />
          </div>
          <div class="form-group">
            <label>{$t("backup.passwordLabel")}</label>
            <input type="password" bind:value={newTask.webdav.password} required placeholder="••••••••" />
          </div>
        </div>
        <div class="form-group">
          <label>{$t("backup.remotePath")}</label>
          <input bind:value={newTask.webdav.path} placeholder="/backups/rde" />
        </div>

      {:else if newTask.target_type === "s3"}
        <div class="form-row">
          <div class="form-group">
            <label>Endpoint *</label>
            <input bind:value={newTask.s3.endpoint} required placeholder="https://s3.amazonaws.com" />
            <small>{$t("backup.s3EndpointHint")}</small>
          </div>
          <div class="form-group">
            <label>Bucket *</label>
            <input bind:value={newTask.s3.bucket} required placeholder="my-backup-bucket" />
          </div>
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>Access Key *</label>
            <input bind:value={newTask.s3.access_key} required placeholder="AKIAXXXXXXXX" />
          </div>
          <div class="form-group">
            <label>Secret Key *</label>
            <input type="password" bind:value={newTask.s3.secret_key} required placeholder="••••••••" />
          </div>
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>Region</label>
            <input bind:value={newTask.s3.region} placeholder="us-east-1" />
          </div>
          <div class="form-group">
            <label>{$t("backup.pathPrefix")}</label>
            <input bind:value={newTask.s3.path} placeholder="/backups" />
          </div>
        </div>

      {:else if newTask.target_type === "sftp"}
        <div class="form-row">
          <div class="form-group" style="flex: 2">
            <label>{$t("backup.hostAddress")}</label>
            <input bind:value={newTask.sftp.host} required placeholder="192.168.1.100" />
          </div>
          <div class="form-group" style="flex: 1">
            <label>{$t("backup.port")}</label>
            <input type="number" bind:value={newTask.sftp.port} min="1" max="65535" />
          </div>
        </div>
        <div class="form-group">
          <label>{$t("backup.username")}</label>
          <input bind:value={newTask.sftp.username} required placeholder="backup" />
        </div>
        <div class="form-group">
          <label>{$t("backup.authMethod")}</label>
          <div class="radio-group">
            <label class="radio-label">
              <input type="radio" name="sftp-auth" value="password" bind:group={newTask.sftp.auth_type} />
              {$t("backup.password")}
            </label>
            <label class="radio-label">
              <input type="radio" name="sftp-auth" value="key" bind:group={newTask.sftp.auth_type} />
              {$t("backup.sshKey")}
            </label>
          </div>
        </div>
        {#if newTask.sftp.auth_type === "password"}
          <div class="form-group">
            <label>{$t("backup.passwordLabel")}</label>
            <input type="password" bind:value={newTask.sftp.password} required placeholder="••••••••" />
          </div>
        {:else}
          <div class="form-group">
            <label>{$t("backup.privateKeyPath")}</label>
            <input bind:value={newTask.sftp.key_path} required placeholder="~/.ssh/id_rsa" />
          </div>
        {/if}
        <div class="form-group">
          <label>{$t("backup.remotePath")}</label>
          <input bind:value={newTask.sftp.path} placeholder="/backups/rde" />
        </div>
      {/if}

      <Button variant="outline" size="sm" loading={testingConnection} onclick={testConnection}>
        <Icon icon="mdi:connection" width="16" /> {$t("backup.testConnection")}
      </Button>
    </div>

    <!-- 定时计划 -->
    <div class="section-title">
      <Icon icon="mdi:clock-outline" width="16" /> {$t("backup.schedule")}
    </div>

    <div class="form-group">
      <label>{$t("backup.scheduleMode")}</label>
      <div class="radio-group">
        <label class="radio-label">
          <input type="radio" name="schedule-mode" value="manual" bind:group={newTask.scheduleMode} onchange={() => newTask.schedule = ""} />
          {$t("backup.manualTrigger")}
        </label>
        <label class="radio-label">
          <input type="radio" name="schedule-mode" value="preset" bind:group={newTask.scheduleMode} />
          {$t("backup.presetTime")}
        </label>
        <label class="radio-label">
          <input type="radio" name="schedule-mode" value="custom" bind:group={newTask.scheduleMode} />
          {$t("backup.customCron")}
        </label>
      </div>
    </div>
    {#if newTask.scheduleMode === "preset"}
      <div class="form-group">
        <select bind:value={newTask.schedule}>
          {#each schedulePresets.slice(1) as preset}
            <option value={preset.value}>{preset.label}</option>
          {/each}
        </select>
      </div>
    {:else if newTask.scheduleMode === "custom"}
      <div class="form-group">
        <input bind:value={newTask.schedule} placeholder="0 0 2 * * *" />
        <small>{$t("backup.cronFormat")}</small>
      </div>
    {/if}

    <!-- 选项 -->
    <div class="form-row options">
      <label class="checkbox-label">
        <input type="checkbox" bind:checked={newTask.compression} /> {$t("backup.enableCompression")}
      </label>
      <label class="checkbox-label">
        <input type="checkbox" bind:checked={newTask.encryption} /> {$t("backup.enableEncryption")}
      </label>
    </div>

    <Button variant="primary" fullWidth loading={creating} onclick={createTask}>{$t("backup.createTask")}</Button>
  </form>
</Modal>

<!-- 文件夹选择器 -->
<FolderBrowser
  bind:open={showFolderBrowser}
  title={$t("backup.selectFolder")}
  onConfirm={handleFolderSelect}
  onClose={() => showFolderBrowser = false}
/>

<!-- 还原 -->
<Modal bind:open={showRestore} title={$t("backup.restoreBackup")} size="md">
  <form class="modal-form" onsubmit={(e) => { e.preventDefault(); restore(); }}>
    <div class="form-group">
      <label for="restore-path">{$t("backup.restoreTo")}</label>
      <input id="restore-path" bind:value={restoreTargetPath} placeholder="/home/user/restore" />
    </div>
    <Button variant="primary" fullWidth loading={restoring} onclick={restore}>{$t("backup.startRestore")}</Button>
  </form>
</Modal>

<!-- 迁移向导 -->
<Modal bind:open={showMigrate} title={$t("backup.migrateTitle")} size="lg" onclose={closeMigrateModal}>
  <div class="migrate-wizard">
    <!-- 步骤指示器 -->
    <div class="wizard-steps">
      <div class="step" class:active={migrateStep >= 1} class:done={migrateStep > 1}>
        <span class="step-num">1</span>
        <span class="step-label">{$t("backup.stepSelectMode")}</span>
      </div>
      <div class="step-line" class:active={migrateStep > 1}></div>
      <div class="step" class:active={migrateStep >= 2} class:done={migrateStep > 2}>
        <span class="step-num">2</span>
        <span class="step-label">{$t("backup.stepPairConnection")}</span>
      </div>
      <div class="step-line" class:active={migrateStep > 2}></div>
      <div class="step" class:active={migrateStep >= 3} class:done={migrateStep > 3}>
        <span class="step-num">3</span>
        <span class="step-label">{$t("backup.stepSelectContent")}</span>
      </div>
      <div class="step-line" class:active={migrateStep > 3}></div>
      <div class="step" class:active={migrateStep >= 4}>
        <span class="step-num">4</span>
        <span class="step-label">{$t("backup.stepTransfer")}</span>
      </div>
    </div>

    <!-- 步骤 2: 配对 -->
    {#if migrateStep === 2}
      <div class="wizard-content">
        {#if migrateRole === "target"}
          <!-- 接收方：显示配对码 -->
          <div class="pair-code-display">
            <p>{$t("backup.enterPairCodeOnSource")}</p>
            {#if migrateLoading}
              <Spinner />
            {:else if migrateSession?.pair_code}
              <div class="pair-code">{migrateSession.pair_code}</div>
              <p class="pair-hint">{$t("backup.pairCodeValidHint")}</p>
              <p class="pair-status">
                {$t("backup.status")} <span class={getMigrateStatusColor(migrateSession.status)}>{getMigrateStatusLabel(migrateSession.status)}</span>
              </p>
            {/if}
          </div>
        {:else}
          <!-- 发送方：输入配对码 -->
          <div class="pair-code-input">
            <p>{$t("backup.generatePairCodeOnTarget")}</p>
            <div class="form-group">
              <label for="pair-code">{$t("backup.pairCode")}</label>
              <input id="pair-code" bind:value={migratePairCode} placeholder="ABC-123-XYZ" class="pair-input" />
            </div>
            <div class="form-group">
              <label for="target-url">{$t("backup.targetRdeAddress")}</label>
              <input id="target-url" bind:value={migrateTargetUrl} placeholder="http://192.168.1.100:9876" />
            </div>
            <Button variant="primary" fullWidth loading={migrateLoading} onclick={connectWithCode}>
              <Icon icon="mdi:link" width="18" /> {$t("backup.connect")}
            </Button>
          </div>
        {/if}
      </div>

    <!-- 步骤 3: 选择内容 -->
    {:else if migrateStep === 3}
      <div class="wizard-content">
        <p class="content-hint">{$t("backup.selectMigrateContent")}</p>
        <div class="content-selection">
          <label class="content-item">
            <input type="checkbox" bind:checked={migrateContent.system_config} />
            <Icon icon="mdi:cog" width="20" />
            <span>{$t("backup.systemConfig")}</span>
          </label>
          <label class="content-item">
            <input type="checkbox" bind:checked={migrateContent.users} />
            <Icon icon="mdi:account-group" width="20" />
            <span>{$t("backup.userAccounts")}</span>
          </label>
          <label class="content-item">
            <input type="checkbox" bind:checked={migrateContent.docker} />
            <Icon icon="mdi:docker" width="20" />
            <span>Docker</span>
          </label>
          <label class="content-item">
            <input type="checkbox" bind:checked={migrateContent.network} />
            <Icon icon="mdi:network" width="20" />
            <span>{$t("backup.networkConfig")}</span>
          </label>
          <label class="content-item">
            <input type="checkbox" bind:checked={migrateContent.samba} />
            <Icon icon="mdi:folder-network" width="20" />
            <span>Samba</span>
          </label>
        </div>
        
        {#if migrateRole === "source"}
          <div class="wizard-actions">
            <Button variant="primary" fullWidth loading={migrateLoading} onclick={startMigrateTransfer}>
              <Icon icon="mdi:send" width="18" /> {$t("backup.startMigrate")}
            </Button>
          </div>
        {:else}
          <p class="wait-hint">{$t("backup.waitingForSource")}</p>
        {/if}
      </div>

    <!-- 步骤 4: 传输中 -->
    {:else if migrateStep === 4}
      <div class="wizard-content">
        <div class="transfer-progress">
          {#if migrateProgress}
            <div class="progress-header">
              <span class="phase">{migrateProgress.phase}</span>
              <span class="percent">{getMigrateProgressPercent()}%</span>
            </div>
            <div class="progress-bar">
              <div class="progress-fill" style="width: {getMigrateProgressPercent()}%"></div>
            </div>
            <div class="progress-details">
              <span>{formatBytes(migrateProgress.transferred_size)} / {formatBytes(migrateProgress.total_size)}</span>
              {#if migrateProgress.speed > 0}
                <span>{formatBytes(migrateProgress.speed)}/s</span>
              {/if}
              {#if migrateProgress.eta > 0}
                <span>{$t("backup.remaining")} {Math.floor(migrateProgress.eta / 60)}:{String(migrateProgress.eta % 60).padStart(2, '0')}</span>
              {/if}
            </div>
            <div class="current-file">
              {#if migrateProgress.current_file}
                <Icon icon="mdi:file-outline" width="14" />
                <span>{migrateProgress.current_file}</span>
              {/if}
            </div>
            <div class="file-count">
              {migrateProgress.transferred_files} / {$t("backup.filesCount", { values: { n: migrateProgress.total_files } })}
            </div>
          {:else}
            <Spinner center />
            <p>{$t("backup.preparing")}</p>
          {/if}
        </div>
        
        {#if migrateSession?.status !== "completed"}
          <Button variant="ghost" fullWidth onclick={cancelMigrate}>{$t("backup.cancelMigrate")}</Button>
        {:else}
          <div class="success-message">
            <Icon icon="mdi:check-circle" width="48" class="success-icon" />
            <h4>{$t("backup.migrateCompleted")}</h4>
            <Button variant="primary" onclick={closeMigrateModal}>{$t("common.close")}</Button>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</Modal>

<style>
  .backup-manager {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--bg-primary, #f5f5f5);
  }

  .header {
    padding: 16px 20px;
    background: var(--bg-card, white);
    border-bottom: 1px solid var(--border-color, #e0e0e0);

    h2 { margin: 0 0 6px; font-size: 18px; }
  }

  .stats-row {
    display: flex;
    gap: 16px;
    font-size: 12px;
    color: var(--text-muted, #999);
  }

  .success-count { color: #059669; }
  .failed-count { color: #dc2626; }

  .content {
    flex: 1;
    padding: 16px 20px;
    overflow-y: auto;
  }

  .panel-header { margin-bottom: 12px; }

  .task-list, .record-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .task-card, .record-card {
    background: var(--bg-card, white);
    border-radius: 8px;
    padding: 14px 16px;
  }

  .task-top, .record-top {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
  }

  .task-name, .record-name { font-weight: 500; font-size: 14px; }

  .task-paths {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 4px;
    font-size: 12px;
    color: var(--text-muted, #999);
  }

  .task-desc {
    margin-top: 4px;
    font-size: 12px;
    color: var(--text-muted, #999);
  }

  .task-actions, .record-actions {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .record-info {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .task-bottom, .record-meta {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-top: 8px;
    font-size: 12px;
    color: var(--text-muted, #999);
    flex-wrap: wrap;
  }

  .task-schedule, .task-time, .task-retention { font-size: 11px; }

  .type-badge {
    padding: 1px 6px;
    border-radius: 3px;
    font-size: 10px;
    background: var(--bg-tertiary, #f0f0f0);
    color: var(--text-secondary, #666);
  }

  .status-badge {
    padding: 1px 8px;
    border-radius: 4px;
    font-size: 11px;
    font-weight: 500;

    &.pending { background: #fef3c7; color: #92400e; }
    &.running { background: #dbeafe; color: #1d4ed8; }
    &.success { background: #d1fae5; color: #065f46; }
    &.failed { background: #fee2e2; color: #991b1b; }
    &.cancelled { background: var(--bg-tertiary, #f0f0f0); color: var(--text-secondary, #666); }
  }

  .progress-text { color: #1d4ed8; font-weight: 500; }
  .message-text { color: var(--text-secondary, #666); }
  .error-text { color: #dc2626; }

  .modal-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .form-row {
    display: flex;
    gap: 12px;

    &.options { gap: 20px; }

    .form-group { flex: 1; }
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;

    label { font-size: 13px; font-weight: 500; color: var(--text-secondary, #666); }
    small { font-size: 11px; color: var(--text-muted, #999); }

    input, select {
      padding: 10px 12px;
      border: 1px solid var(--border-color, #e0e0e0);
      border-radius: 6px;
      font-size: 14px;
      background: var(--bg-input, white);
      color: var(--text-primary, #333);
      &:focus { outline: none; border-color: var(--color-primary, #4a90d9); box-shadow: 0 0 0 3px rgba(74,144,217,0.15); }
    }
  }

  .source-row {
    display: flex;
    gap: 8px;
    input { flex: 1; }
  }

  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    cursor: pointer;

    input[type="checkbox"] {
      width: 16px;
      height: 16px;
    }
  }

  .section-title {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    font-weight: 600;
    color: var(--text-secondary, #666);
    padding-bottom: 8px;
    border-bottom: 1px solid var(--border-color, #e0e0e0);
    margin-top: 8px;
  }

  .target-config-section {
    background: var(--bg-secondary, #f9f9f9);
    padding: 14px;
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .input-with-button {
    display: flex;
    gap: 8px;

    input { flex: 1; }
  }

  .radio-group {
    display: flex;
    gap: 16px;
  }

  .radio-label {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    cursor: pointer;

    input[type="radio"] {
      width: 16px;
      height: 16px;
      accent-color: var(--color-primary, #4a90d9);
    }
  }

  /* 迁移面板 */
  .migrate-panel {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 32px 16px;
    gap: 24px;
  }

  .migrate-intro {
    text-align: center;

    :global(.migrate-icon) {
      color: var(--color-primary, #4a90d9);
      margin-bottom: 12px;
    }

    h3 {
      margin: 0 0 8px;
      font-size: 18px;
    }

    p {
      margin: 0;
      color: var(--text-muted, #888);
      font-size: 13px;
    }
  }

  .migrate-options {
    display: flex;
    gap: 16px;
    width: 100%;
    max-width: 500px;
  }

  .migrate-card {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px;
    background: var(--bg-card, white);
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.2s;

    &:hover {
      border-color: var(--color-primary, #4a90d9);
      box-shadow: 0 2px 8px rgba(0,0,0,0.1);
    }

    :global(svg:first-child) {
      color: var(--color-primary, #4a90d9);
    }

    .card-content {
      flex: 1;

      h4 { margin: 0 0 4px; font-size: 14px; }
      p { margin: 0; font-size: 12px; color: var(--text-muted, #888); }
    }
  }

  /* 迁移向导 */
  .migrate-wizard {
    padding: 8px 0;
  }

  .wizard-steps {
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 24px;
  }

  .step {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;

    .step-num {
      width: 28px;
      height: 28px;
      border-radius: 50%;
      background: var(--bg-secondary, #e0e0e0);
      color: var(--text-muted, #999);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 13px;
      font-weight: 600;
    }

    .step-label {
      font-size: 11px;
      color: var(--text-muted, #999);
    }

    &.active .step-num {
      background: var(--color-primary, #4a90d9);
      color: white;
    }

    &.active .step-label {
      color: var(--text-primary, #333);
    }

    &.done .step-num {
      background: #10b981;
      color: white;
    }
  }

  .step-line {
    width: 40px;
    height: 2px;
    background: var(--bg-secondary, #e0e0e0);
    margin: 0 8px 20px;

    &.active { background: var(--color-primary, #4a90d9); }
  }

  .wizard-content {
    min-height: 200px;
  }

  .pair-code-display, .pair-code-input {
    text-align: center;
    padding: 16px;

    p { margin: 0 0 16px; color: var(--text-secondary, #666); }
  }

  .pair-code {
    font-size: 32px;
    font-weight: 700;
    font-family: monospace;
    letter-spacing: 4px;
    color: var(--color-primary, #4a90d9);
    padding: 16px 24px;
    background: var(--bg-secondary, #f5f5f5);
    border-radius: 8px;
    margin: 16px 0;
  }

  .pair-hint {
    font-size: 12px;
    color: var(--text-muted, #999) !important;
  }

  .pair-status {
    margin-top: 12px;
    font-size: 13px;
  }

  .pair-input {
    font-size: 18px !important;
    text-align: center;
    letter-spacing: 2px;
    text-transform: uppercase;
  }

  .content-hint {
    margin: 0 0 16px;
    color: var(--text-secondary, #666);
  }

  .content-selection {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 12px;
  }

  .content-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px;
    background: var(--bg-secondary, #f9f9f9);
    border-radius: 8px;
    cursor: pointer;
    transition: background 0.2s;

    &:hover { background: var(--bg-hover, #f0f0f0); }

    input[type="checkbox"] {
      width: 18px;
      height: 18px;
    }

    :global(svg) {
      color: var(--text-muted, #888);
    }

    span {
      font-size: 13px;
    }
  }

  .wizard-actions {
    margin-top: 24px;
  }

  .wait-hint {
    text-align: center;
    color: var(--text-muted, #888);
    margin-top: 24px;
    font-style: italic;
  }

  .transfer-progress {
    padding: 16px;
    background: var(--bg-secondary, #f9f9f9);
    border-radius: 8px;
  }

  .progress-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 8px;

    .phase {
      font-size: 13px;
      color: var(--text-secondary, #666);
      text-transform: capitalize;
    }

    .percent {
      font-size: 14px;
      font-weight: 600;
      color: var(--color-primary, #4a90d9);
    }
  }

  .progress-bar {
    height: 8px;
    background: var(--bg-tertiary, #e0e0e0);
    border-radius: 4px;
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: var(--color-primary, #4a90d9);
    transition: width 0.3s;
  }

  .progress-details {
    display: flex;
    gap: 16px;
    margin-top: 8px;
    font-size: 12px;
    color: var(--text-muted, #888);
  }

  .current-file {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 12px;
    font-size: 12px;
    color: var(--text-muted, #888);

    span {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .file-count {
    margin-top: 8px;
    font-size: 12px;
    color: var(--text-secondary, #666);
  }

  .success-message {
    text-align: center;
    padding: 24px;

    :global(.success-icon) {
      color: #10b981;
      margin-bottom: 12px;
    }

    h4 {
      margin: 0 0 16px;
      color: #10b981;
    }
  }
</style>
