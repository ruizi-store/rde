<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { t } from "./i18n";
  import Icon from "@iconify/svelte";
  import { Button, Modal, Spinner, EmptyState, Badge, Select } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import {
    vmService,
    type VM,
    type ISOFile,
    type Snapshot,
    type StorageInfo,
  } from "./service";
  import VNCConsole from "./VNCConsole.svelte";
  import VMCreate from "./VMCreate.svelte";
  import VMBackups from "./VMBackups.svelte";

  // ==================== 状态 ====================

  let vms = $state<VM[]>([]);
  let isos = $state<ISOFile[]>([]);
  let loading = $state(true);
  let refreshTimer: ReturnType<typeof setInterval> | null = null;

  // 创建虚拟机
  let showCreate = $state(false);

  // 快照管理
  let showSnapshots = $state(false);
  let snapshotVM = $state<VM | null>(null);
  let snapshots = $state<Snapshot[]>([]);
  let newSnapshotName = $state("");

  // VNC 控制台 - 使用 Modal（独立 SPA 模式）
  let showVNC = $state(false);
  let vncVM = $state<VM | null>(null);

  // ISO 管理
  let showISO = $state(false);

  // 备份管理
  let showBackups = $state(false);
  let backupsVM = $state<VM | null>(null);

  // P3: 批量操作
  let selectedVMs = $state<Set<string>>(new Set());
  let selectMode = $state(false);

  // P3: 克隆
  let showClone = $state(false);
  let cloneVM = $state<VM | null>(null);
  let cloneName = $state("");

  // P3: 存储信息
  let showStorage = $state(false);
  let storageInfo = $state<StorageInfo | null>(null);

  // P5: 导入导出
  let showImport = $state(false);
  let showExport = $state(false);
  let exportVM = $state<VM | null>(null);
  let importPath = $state("");
  let importName = $state("");
  let exportFormat = $state<string>("ova");
  let exporting = $state(false);
  let importing = $state(false);

  // P6: SSH 终端集成
  let showSSH = $state(false);
  let sshVM = $state<VM | null>(null);
  let sshInfo = $state<{ host: string; port: number } | null>(null);
  let sshUsername = $state("root");
  let sshPassword = $state("");

  // P6: 资源监控
  let showResources = $state(false);
  let resourcesVM = $state<VM | null>(null);
  let resourceStats = $state<import("./service").VMResourceStats | null>(null);
  let resourcesLoading = $state(false);
  let resourcesTimer: ReturnType<typeof setInterval> | null = null;

  // ==================== 生命周期 ====================

  onMount(async () => {
    await refresh();
    refreshTimer = setInterval(refreshVMs, 10000);
  });

  onDestroy(() => {
    if (refreshTimer) clearInterval(refreshTimer);
  });

  // ==================== 方法 ====================

  async function refresh() {
    try {
      const [v, i] = await Promise.all([
        vmService.listVMs(),
        vmService.listISOs(),
      ]);
      vms = v;
      isos = i;
    } catch {}
    finally { loading = false; }
  }

  async function refreshVMs() {
    try { vms = await vmService.listVMs(); } catch {}
  }

  async function startVM(vm: VM) {
    try { await vmService.startVM(vm.id); showToast($t("vm.toast.starting", { values: { name: vm.name } }), "success"); await refreshVMs(); }
    catch (e: any) { showToast(e.message, "error"); }
  }

  async function stopVM(vm: VM, force = false) {
    try { await vmService.stopVM(vm.id, force); showToast($t("vm.toast.stopped", { values: { name: vm.name } }), "success"); await refreshVMs(); }
    catch (e: any) { showToast(e.message, "error"); }
  }

  async function deleteVM(vm: VM) {
    if (!confirm($t("vm.confirm.deleteVM", { values: { name: vm.name } }))) return;
    try { await vmService.deleteVM(vm.id); showToast($t("vm.toast.deleted"), "success"); await refreshVMs(); }
    catch (e: any) { showToast(e.message, "error"); }
  }

  function openVNC(vm: VM) {
    // 在 Modal 中打开 VNC 控制台（独立 SPA 模式）
    vncVM = vm;
    showVNC = true;
  }

  async function openSnapshots(vm: VM) {
    snapshotVM = vm;
    showSnapshots = true;
    newSnapshotName = "";
    try { snapshots = await vmService.listSnapshots(vm.id); }
    catch (e: any) { showToast(e.message, "error"); }
  }

  async function createSnapshot() {
    if (!snapshotVM || !newSnapshotName) return;
    try {
      await vmService.createSnapshot(snapshotVM.id, newSnapshotName);
      newSnapshotName = "";
      showToast($t("vm.toast.snapshotCreated"), "success");
      snapshots = await vmService.listSnapshots(snapshotVM.id);
    } catch (e: any) { showToast(e.message, "error"); }
  }

  async function revertSnapshot(tag: string) {
    if (!snapshotVM) return;
    try {
      await vmService.revertSnapshot(snapshotVM.id, tag);
      showToast($t("vm.toast.snapshotReverted"), "success");
    } catch (e: any) { showToast(e.message, "error"); }
  }

  async function deleteSnapshot(tag: string) {
    if (!snapshotVM) return;
    try {
      await vmService.deleteSnapshot(snapshotVM.id, tag);
      snapshots = await vmService.listSnapshots(snapshotVM.id);
      showToast($t("vm.toast.snapshotDeleted"), "success");
    } catch (e: any) { showToast(e.message, "error"); }
  }

  async function deleteISO(name: string) {
    // ISO 删除需后端实现 DELETE /vm/isos/:name
    showToast($t("vm.toast.isoDeleteNotImplemented"), "info");
  }

  function statusColor(s: string): "default" | "secondary" | "success" | "warning" | "error" {
    switch (s) {
      case "running": return "success";
      case "stopped": return "secondary";
      case "paused": return "warning";
      case "starting": case "stopping": return "warning";
      case "error": return "error";
      default: return "secondary";
    }
  }

  function statusText(s: string): string {
    const map: Record<string, string> = { running: $t("vm.status.running"), stopped: $t("vm.status.stopped"), starting: $t("vm.status.starting"), stopping: $t("vm.status.stopping"), paused: $t("vm.status.paused"), error: $t("vm.status.error") };
    return map[s] || s;
  }

  function formatSize(mb: number): string {
    return mb >= 1024 ? `${(mb / 1024).toFixed(1)} GB` : `${mb} MB`;
  }

  function formatDate(dateStr: string): string {
    try {
      const d = new Date(dateStr);
      return d.toLocaleDateString("zh-CN", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
    } catch { return ""; }
  }

  function getNetworkLabel(mode?: string): string {
    const labels: Record<string, string> = { user: $t("vm.network.nat"), bridge: $t("vm.network.bridge"), none: $t("vm.network.none") };
    return labels[mode || "user"] || $t("vm.network.nat");
  }

  function getOSIcon(vm: VM): string {
    const name = (vm.name + (vm.os_type || "")).toLowerCase();
    if (name.includes("ubuntu")) return "logos:ubuntu";
    if (name.includes("debian")) return "logos:debian";
    if (name.includes("centos")) return "logos:centos-icon";
    if (name.includes("redhat") || name.includes("fedora")) return "logos:fedora";
    if (name.includes("arch")) return "logos:archlinux";
    if (name.includes("windows")) return "logos:microsoft-windows-icon";
    if (name.includes("mac") || name.includes("darwin")) return "logos:apple";
    return "logos:linux-tux";
  }

  // ==================== P3: 批量操作 ====================

  function toggleSelect(vmId: string) {
    if (selectedVMs.has(vmId)) {
      selectedVMs.delete(vmId);
    } else {
      selectedVMs.add(vmId);
    }
    selectedVMs = new Set(selectedVMs);
  }

  function selectAll() {
    if (selectedVMs.size === vms.length) {
      selectedVMs = new Set();
    } else {
      selectedVMs = new Set(vms.map(vm => vm.id));
    }
  }

  async function batchStart() {
    if (selectedVMs.size === 0) return;
    try {
      const results = await vmService.batchStart([...selectedVMs]);
      const failed = results.filter(r => !r.success);
      if (failed.length === 0) {
        showToast($t("vm.toast.batchStarted", { values: { n: results.length } }), "success");
      } else {
        showToast($t("vm.toast.batchPartial", { values: { success: results.length - failed.length, failed: failed.length } }), "warning");
      }
      selectedVMs = new Set();
      selectMode = false;
      await refreshVMs();
    } catch (e: any) { showToast(e.message, "error"); }
  }

  async function batchStop() {
    if (selectedVMs.size === 0) return;
    try {
      const results = await vmService.batchStop([...selectedVMs]);
      const failed = results.filter(r => !r.success);
      if (failed.length === 0) {
        showToast($t("vm.toast.batchStopped", { values: { n: results.length } }), "success");
      } else {
        showToast($t("vm.toast.batchPartial", { values: { success: results.length - failed.length, failed: failed.length } }), "warning");
      }
      selectedVMs = new Set();
      selectMode = false;
      await refreshVMs();
    } catch (e: any) { showToast(e.message, "error"); }
  }

  async function batchDelete() {
    if (selectedVMs.size === 0) return;
    if (!confirm($t("vm.confirm.deleteBatch", { values: { n: selectedVMs.size } }))) return;
    try {
      const results = await vmService.batchDelete([...selectedVMs]);
      const failed = results.filter(r => !r.success);
      if (failed.length === 0) {
        showToast($t("vm.toast.batchDeleted", { values: { n: results.length } }), "success");
      } else {
        showToast($t("vm.toast.batchPartial", { values: { success: results.length - failed.length, failed: failed.length } }), "warning");
      }
      selectedVMs = new Set();
      selectMode = false;
      await refreshVMs();
    } catch (e: any) { showToast(e.message, "error"); }
  }

  // ==================== P3: 克隆 ====================

  function openClone(vm: VM) {
    cloneVM = vm;
    cloneName = `${vm.name} (${$t("vm.button.clone")})`;
    showClone = true;
  }

  async function doClone() {
    if (!cloneVM || !cloneName) return;
    try {
      await vmService.cloneVM(cloneVM.id, cloneName);
      showToast($t("vm.toast.cloneSuccess"), "success");
      showClone = false;
      cloneVM = null;
      cloneName = "";
      await refreshVMs();
    } catch (e: any) { showToast(e.message, "error"); }
  }

  // ==================== P3: 存储信息 ====================

  async function openStorage() {
    showStorage = true;
    try {
      storageInfo = await vmService.getStorageInfo();
    } catch (e: any) { showToast(e.message, "error"); }
  }

  // ==================== P5: 导入导出 ====================

  function openExportDialog(vm: VM) {
    exportVM = vm;
    exportFormat = "ova";
    showExport = true;
  }

  async function doExport() {
    if (!exportVM) return;
    exporting = true;
    try {
      const result = await vmService.exportVM({
        vm_id: exportVM.id,
        format: exportFormat as any,
        compress: true,
      });
      showToast($t("vm.toast.exportSuccess", { values: { path: result.path } }), "success");
      showExport = false;
    } catch (e: any) { 
      showToast($t("vm.toast.exportFailed", { values: { error: e.message } }), "error"); 
    } finally {
      exporting = false;
    }
  }

  async function doImport() {
    if (!importPath) {
      showToast($t("vm.toast.enterFilePath"), "error");
      return;
    }
    importing = true;
    try {
      const vm = await vmService.importVM({
        path: importPath,
        name: importName || undefined,
      });
      showToast($t("vm.toast.importSuccess", { values: { name: vm.name } }), "success");
      showImport = false;
      importPath = "";
      importName = "";
      await refreshVMs();
    } catch (e: any) {
      showToast($t("vm.toast.importFailed", { values: { error: e.message } }), "error");
    } finally {
      importing = false;
    }
  }

  // ==================== P5: 自动启动 ====================

  async function toggleAutoStart(vm: VM) {
    try {
      const newState = !vm.auto_start;
      await vmService.setAutoStart(vm.id, newState, vm.start_order || 0, vm.start_delay || 0);
      showToast(newState ? $t("vm.toast.autoStartEnabled") : $t("vm.toast.autoStartDisabled"), "success");
      await refreshVMs();
    } catch (e: any) {
      showToast(e.message, "error");
    }
  }

  // ==================== P6: SSH 终端集成 ====================

  async function openSSH(vm: VM) {
    try {
      const info = await vmService.getVMSSHInfo(vm.id);
      sshVM = vm;
      sshInfo = { host: info.host, port: info.port };
      sshUsername = "root";
      sshPassword = "";
      showSSH = true;
    } catch (e: any) {
      showToast(e.message, "error");
    }
  }

  function connectSSH() {
    if (!sshInfo || !sshUsername) return;
    // 打开终端应用并传递 SSH 连接参数
    // 通过 URL 参数打开终端，让终端应用处理 SSH 连接
    const params = new URLSearchParams({
      host: sshInfo.host,
      port: sshInfo.port.toString(),
      username: sshUsername,
      password: sshPassword,
      vm_id: sshVM?.id || "",
      vm_name: sshVM?.name || ""
    });
    // 使用窗口管理器打开终端应用（如果有）或新窗口
    window.open(`/terminal?ssh=true&${params.toString()}`, "_blank");
    showSSH = false;
  }

  // ==================== P6: 资源监控 ====================

  async function openResources(vm: VM) {
    resourcesVM = vm;
    showResources = true;
    resourceStats = null;
    resourcesLoading = true;
    
    try {
      resourceStats = await vmService.getVMResources(vm.id);
    } catch (e: any) {
      showToast(e.message, "error");
    } finally {
      resourcesLoading = false;
    }

    // 启动定时刷新
    resourcesTimer = setInterval(async () => {
      if (resourcesVM && showResources) {
        try {
          resourceStats = await vmService.getVMResources(resourcesVM.id);
        } catch { /* 忽略刷新错误 */ }
      }
    }, 3000);
  }

  function closeResources() {
    showResources = false;
    resourcesVM = null;
    resourceStats = null;
    if (resourcesTimer) {
      clearInterval(resourcesTimer);
      resourcesTimer = null;
    }
  }

  function formatBytes(bytes: number): string {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  }
</script>

<div class="vm-manager">
  <header class="header">
    <h2>{$t("vm.title")}</h2>
    <div class="header-actions">
      {#if selectMode}
        <span class="select-info">{$t("vm.batch.selected", { values: { n: selectedVMs.size } })}</span>
        <Button variant="ghost" size="sm" onclick={selectAll}>
          {selectedVMs.size === vms.length ? $t("vm.button.deselectAll") : $t("vm.button.selectAll")}
        </Button>
        <Button variant="success" size="sm" onclick={batchStart} disabled={selectedVMs.size === 0}>
          {$t("vm.button.batchStart")}
        </Button>
        <Button variant="ghost" size="sm" onclick={batchStop} disabled={selectedVMs.size === 0}>
          {$t("vm.button.batchStop")}
        </Button>
        <Button variant="danger" size="sm" onclick={batchDelete} disabled={selectedVMs.size === 0}>
          {$t("vm.button.batchDelete")}
        </Button>
        <Button variant="ghost" size="sm" onclick={() => { selectMode = false; selectedVMs = new Set(); }}>
          {$t("common.cancel")}
        </Button>
      {:else}
        <Button variant="ghost" size="sm" onclick={openStorage}>
          <Icon icon="mdi:harddisk" width="16" /> {$t("vm.button.storage")}
        </Button>
        <Button variant="ghost" size="sm" onclick={() => (showImport = true)}>
          <Icon icon="mdi:import" width="16" /> {$t("vm.button.import")}
        </Button>
        <Button variant="ghost" size="sm" onclick={() => (selectMode = true)} disabled={vms.length === 0}>
          <Icon icon="mdi:checkbox-multiple-marked-outline" width="16" /> {$t("vm.button.batch")}
        </Button>
        <Button variant="ghost" size="sm" onclick={() => (showISO = true)}>
          <Icon icon="mdi:disc" width="16" /> ISO
        </Button>
        <Button variant="primary" size="sm" onclick={() => (showCreate = true)}>
          <Icon icon="mdi:plus" width="16" /> {$t("vm.button.create")}
        </Button>
      {/if}
    </div>
  </header>

  {#if loading}
    <Spinner center />
  {:else if vms.length === 0}
    <EmptyState icon="mdi:desktop-tower" title={$t("vm.empty.title")} description={$t("vm.empty.description")} actionLabel={$t("vm.empty.action")} onaction={() => (showCreate = true)} />
  {:else}
    <div class="vm-grid">
      {#each vms as vm (vm.id)}
        <div class="vm-card" class:selected={selectedVMs.has(vm.id)}>
          {#if selectMode}
            <div class="vm-select">
              <input type="checkbox" checked={selectedVMs.has(vm.id)} onchange={() => toggleSelect(vm.id)} />
            </div>
          {/if}
          <div class="vm-header">
            <div class="vm-icon">
              <Icon icon={getOSIcon(vm)} width="32" />
            </div>
            <div class="vm-title">
              <div class="vm-top">
                <div class="vm-name">{vm.name}</div>
                <Badge variant={statusColor(vm.status)}>{statusText(vm.status)}</Badge>
              </div>
              <div class="vm-meta">
                <span class="vm-id">#{vm.id.slice(-6)}</span>
                {#if vm.created_at}
                  <span class="vm-time">{formatDate(vm.created_at)}</span>
                {/if}
              </div>
            </div>
          </div>
          <div class="vm-specs">
            <span><Icon icon="mdi:memory" width="14" /> {formatSize(vm.memory)}</span>
            <span><Icon icon="mdi:cpu-64-bit" width="14" /> {vm.cpu} {$t("vm.cores")}</span>
            <span><Icon icon="mdi:harddisk" width="14" /> {vm.disk_size} GB</span>
            <span><Icon icon="mdi:lan" width="14" /> {getNetworkLabel(vm.network_mode)}</span>
          </div>
          {#if vm.description}
            <div class="vm-desc">{vm.description}</div>
          {/if}
          {#if vm.port_forwards && vm.port_forwards.length > 0}
            <div class="vm-ports">
              {#each vm.port_forwards as pf}
                <span class="port-tag" class:active={vm.status === "running"}>
                  {pf.name || pf.guest_port}:{pf.host_port}
                </span>
              {/each}
            </div>
          {/if}
          <div class="vm-actions">
            {#if vm.status === "stopped"}
              <div class="action-group primary">
                <Button variant="success" size="sm" onclick={() => startVM(vm)}>
                  <Icon icon="mdi:play" width="16" /> {$t("vm.button.start")}
                </Button>
              </div>
              <div class="action-group">
                <Button variant="ghost" size="sm" onclick={() => openClone(vm)} title={$t("vm.tooltip.clone")}>
                  <Icon icon="mdi:content-copy" width="16" />
                </Button>
                <Button variant="ghost" size="sm" onclick={() => openExportDialog(vm)} title={$t("vm.tooltip.export")}>
                  <Icon icon="mdi:export" width="16" />
                </Button>
                <Button variant="ghost" size="sm" onclick={() => { backupsVM = vm; showBackups = true; }} title={$t("vm.tooltip.backup")}>
                  <Icon icon="mdi:backup-restore" width="16" />
                </Button>
                <Button variant="ghost" size="sm" onclick={() => openSnapshots(vm)} title={$t("vm.tooltip.snapshot")}>
                  <Icon icon="mdi:camera" width="16" />
                </Button>
              </div>
              <div class="action-group end">
                <button 
                  class="auto-start-btn" 
                  class:enabled={vm.auto_start}
                  title={vm.auto_start ? $t("vm.tooltip.autoStartOn") : $t("vm.tooltip.autoStartOff")}
                  onclick={() => toggleAutoStart(vm)}
                >
                  <Icon icon="mdi:power-settings-new" width="16" />
                  {vm.auto_start ? $t("vm.button.autoStartLabel") : ""}
                </button>
                <Button variant="ghost" size="sm" onclick={() => deleteVM(vm)} title={$t("vm.tooltip.delete")}>
                  <Icon icon="mdi:delete-outline" width="16" />
                </Button>
              </div>
            {:else if vm.status === "running"}
              <div class="action-group primary">
                <Button variant="primary" size="sm" onclick={() => openVNC(vm)}>
                  <Icon icon="mdi:monitor" width="16" /> {$t("vm.button.console")}
                </Button>
              </div>
              <div class="action-group">
                <Button variant="ghost" size="sm" onclick={() => openSSH(vm)} title={$t("vm.tooltip.sshTerminal")}>
                  <Icon icon="mdi:console" width="16" />
                </Button>
                <Button variant="ghost" size="sm" onclick={() => openResources(vm)} title={$t("vm.tooltip.resourceMonitor")}>
                  <Icon icon="mdi:chart-line" width="16" />
                </Button>
                <Button variant="ghost" size="sm" onclick={() => openSnapshots(vm)} title={$t("vm.tooltip.snapshot")}>
                  <Icon icon="mdi:camera" width="16" />
                </Button>
              </div>
              <div class="action-group end">
                <Button variant="ghost" size="sm" onclick={() => stopVM(vm)}>{$t("vm.button.stop")}</Button>
                <Button variant="danger" size="sm" onclick={() => stopVM(vm, true)} title={$t("vm.tooltip.forceStop")}>
                  <Icon icon="mdi:stop" width="16" />
                </Button>
              </div>
            {:else if vm.status === "paused"}
              <Button variant="success" size="sm" onclick={() => vmService.resumeVM(vm.id).then(refreshVMs)}>
                <Icon icon="mdi:play" width="16" /> {$t("vm.button.resume")}
              </Button>
              <Button variant="ghost" size="sm" onclick={() => openSnapshots(vm)} title={$t("vm.tooltip.snapshot")}>
                <Icon icon="mdi:camera" width="16" />
              </Button>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- 创建虚拟机 -->
<VMCreate
  bind:open={showCreate}
  {isos}
  onclose={() => { showCreate = false; }}
  oncreate={() => { refreshVMs(); }}
/>

<!-- 快照管理 -->
<Modal bind:open={showSnapshots} title={$t("vm.modal.snapshotManagement", { values: { name: snapshotVM?.name } })} size="md">
  <div class="snapshot-form">
    <input bind:value={newSnapshotName} placeholder={$t("vm.form.snapshotName")} />
    <Button variant="primary" size="sm" onclick={createSnapshot} disabled={!newSnapshotName}>{$t("vm.button.createSnapshot")}</Button>
  </div>
  {#if snapshots.length === 0}
    <p class="text-muted" style="text-align:center;padding:20px;">{$t("vm.snapshot.noSnapshots")}</p>
  {:else}
    <div class="snapshot-list">
      {#each snapshots as snap (snap.id || snap.name)}
        <div class="snapshot-row">
          <div>
            <div class="snap-name">{snap.name || snap.tag}</div>
            <div class="snap-time">{new Date(snap.created_at).toLocaleString()}</div>
          </div>
          <div class="snap-actions">
            <Button variant="ghost" size="sm" onclick={() => revertSnapshot(snap.name || snap.tag || snap.id)}>{$t("vm.snapshot.restore")}</Button>
            <Button variant="ghost" size="sm" onclick={() => deleteSnapshot(snap.name || snap.tag || snap.id)}>
              <Icon icon="mdi:delete-outline" width="16" />
            </Button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</Modal>

<!-- ISO 管理 -->
<Modal bind:open={showISO} title={$t("vm.modal.isoImages")} size="md">
  {#if isos.length === 0}
    <p class="text-muted" style="text-align:center;padding:20px;">{$t("vm.iso.noISOFiles")}</p>
  {:else}
    <div class="iso-list">
      {#each isos as iso (iso.name)}
        <div class="iso-row">
          <div>
            <div class="iso-name">{iso.name}</div>
            <div class="iso-info">{(iso.size / 1073741824).toFixed(2)} GB</div>
          </div>
          <Button variant="ghost" size="sm" onclick={() => deleteISO(iso.name)}>
            <Icon icon="mdi:delete-outline" width="16" />
          </Button>
        </div>
      {/each}
    </div>
  {/if}
</Modal>

<!-- 备份管理 -->
<Modal bind:open={showBackups} title={$t("vm.modal.backupManagement")} size="lg">
  {#if backupsVM}
    <VMBackups 
      vm={backupsVM} 
      onClose={() => { showBackups = false; backupsVM = null; }} 
      onRestore={(restoredVM) => { refreshVMs(); showToast($t("vm.toast.restoredAs", { values: { name: restoredVM.name } }), "success"); }}
    />
  {/if}
</Modal>

<!-- P3: 克隆虚拟机 -->
<Modal bind:open={showClone} title={$t("vm.modal.cloneVM")} size="sm">
  {#if cloneVM}
    <div class="clone-form">
      <p>{$t("vm.clone.willClone")} <strong>{cloneVM.name}</strong></p>
      <label>
        <span>{$t("vm.form.newVMName")}</span>
        <input type="text" bind:value={cloneName} placeholder={$t("vm.form.enterNewName")} />
      </label>
      <div class="form-actions">
        <Button variant="primary" onclick={doClone} disabled={!cloneName}>{$t("vm.button.clone")}</Button>
        <Button variant="ghost" onclick={() => { showClone = false; cloneVM = null; }}>{$t("common.cancel")}</Button>
      </div>
    </div>
  {/if}
</Modal>

<!-- P3: 存储信息 -->
<Modal bind:open={showStorage} title={$t("vm.modal.storageUsage")} size="md">
  {#if storageInfo}
    <div class="storage-info">
      <div class="storage-overview">
        <div class="storage-bar">
          <div class="used" style="width: {(storageInfo.used_space / storageInfo.total_space) * 100}%"></div>
        </div>
        <div class="storage-text">
          {$t("vm.storage.used")} {formatBytes(storageInfo.used_space)} / {$t("vm.storage.total")} {formatBytes(storageInfo.total_space)}
          <span class="free">（{$t("vm.storage.free")} {formatBytes(storageInfo.free_space)}）</span>
        </div>
      </div>
      <div class="storage-details">
        <div class="detail-row">
          <span class="label"><Icon icon="mdi:harddisk" width="16" /> {$t("vm.storage.vmDisk")}</span>
          <span class="value">{formatBytes(storageInfo.vm_disk_usage)}</span>
        </div>
        <div class="detail-row">
          <span class="label"><Icon icon="mdi:disc" width="16" /> {$t("vm.storage.isoImages")}</span>
          <span class="value">{formatBytes(storageInfo.iso_usage)}</span>
        </div>
        <div class="detail-row">
          <span class="label"><Icon icon="mdi:backup-restore" width="16" /> {$t("vm.storage.backups")}</span>
          <span class="value">{formatBytes(storageInfo.backup_usage)}</span>
        </div>
        <div class="detail-row">
          <span class="label"><Icon icon="mdi:camera" width="16" /> {$t("vm.storage.snapshots")}</span>
          <span class="value">{formatBytes(storageInfo.snapshot_usage)}</span>
        </div>
      </div>
      <div class="vm-summary">
        <span>{$t("vm.storage.vmCount")}: <strong>{storageInfo.vm_count}</strong></span>
        <span>{$t("vm.storage.running")}: <strong>{storageInfo.running_vm_count}</strong></span>
      </div>
    </div>
  {:else}
    <Spinner center />
  {/if}
</Modal>

<!-- P5: 导入虚拟机 -->
<Modal bind:open={showImport} title={$t("vm.modal.importVM")} size="md">
  <div class="import-form">
    <div class="form-group">
      <label for="import-path">{$t("vm.form.filePath")}</label>
      <input 
        id="import-path" 
        type="text" 
        bind:value={importPath} 
        placeholder={$t("vm.form.filePathExample")}
      />
      <p class="hint">{$t("vm.form.supportedFormats")}</p>
    </div>
    <div class="form-group">
      <label for="import-name">{$t("vm.form.vmNameOptional")}</label>
      <input 
        id="import-name" 
        type="text" 
        bind:value={importName} 
        placeholder={$t("vm.form.autoNameFromFile")}
      />
    </div>
    <div class="form-actions">
      <Button variant="ghost" onclick={() => { showImport = false; }}>{$t("common.cancel")}</Button>
      <Button variant="primary" onclick={doImport} disabled={importing || !importPath}>
        {importing ? $t("vm.form.importing") : $t("vm.button.import")}
      </Button>
    </div>
  </div>
</Modal>

<!-- P5: 导出虚拟机 -->
<Modal bind:open={showExport} title={$t("vm.modal.exportVM", { values: { name: exportVM?.name } })} size="md">
  {#if exportVM}
    <div class="export-form">
      <div class="form-group">
        <label for="export-format">{$t("vm.form.exportFormat")}</label>
        <select id="export-format" bind:value={exportFormat}>
          <option value="ova">{$t("vm.form.ovaFormat")}</option>
          <option value="qcow2">{$t("vm.form.qcow2Format")}</option>
          <option value="raw">{$t("vm.form.rawFormat")}</option>
        </select>
      </div>
      <div class="export-info">
        <p><Icon icon="mdi:information" width="16" /> {$t("vm.form.exportHint")}</p>
        {#if exportVM.status !== "stopped"}
          <p class="warning"><Icon icon="mdi:alert" width="16" /> {$t("vm.form.stopVMFirst")}</p>
        {/if}
      </div>
      <div class="form-actions">
        <Button variant="ghost" onclick={() => { showExport = false; exportVM = null; }}>{$t("common.cancel")}</Button>
        <Button variant="primary" onclick={doExport} disabled={exporting || exportVM.status !== "stopped"}>
          {exporting ? $t("vm.form.exporting") : $t("vm.button.export")}
        </Button>
      </div>
    </div>
  {/if}
</Modal>

<!-- P6: SSH 终端连接 -->
<Modal bind:open={showSSH} title={$t("vm.modal.sshConnection", { values: { name: sshVM?.name } })} size="sm">
  {#if sshInfo}
    <div class="ssh-form">
      <div class="ssh-info">
        <p><strong>{$t("vm.form.connectionAddress")}:</strong> {sshInfo.host}:{sshInfo.port}</p>
        <p class="hint">{$t("vm.form.sshPortHint")}</p>
      </div>
      <label>
        <span>{$t("vm.form.username")}</span>
        <input type="text" bind:value={sshUsername} placeholder="root" />
      </label>
      <label>
        <span>{$t("vm.form.password")}</span>
        <input type="password" bind:value={sshPassword} placeholder={$t("vm.form.enterPassword")} />
      </label>
      <div class="form-actions">
        <Button variant="ghost" onclick={() => { showSSH = false; sshVM = null; }}>{$t("common.cancel")}</Button>
        <Button variant="primary" onclick={connectSSH} disabled={!sshUsername}>
          <Icon icon="mdi:console" width="16" /> {$t("vm.button.connect")}
        </Button>
      </div>
    </div>
  {/if}
</Modal>

<!-- P6: 资源监控仪表板 -->
<Modal bind:open={showResources} title={$t("vm.modal.resourceMonitor", { values: { name: resourcesVM?.name } })} size="lg" onclose={closeResources}>
  <div class="resources-dashboard">
    {#if resourcesLoading}
      <div class="loading-center">
        <Spinner />
        <p>{$t("vm.resources.loadingData")}</p>
      </div>
    {:else if resourceStats}
      <div class="stats-grid">
        <!-- CPU -->
        <div class="stat-card">
          <div class="stat-header">
            <Icon icon="mdi:cpu-64-bit" width="20" />
            <span>{$t("vm.resources.cpu")}</span>
          </div>
          <div class="stat-value">{resourceStats.cpu.usage_percent.toFixed(1)}%</div>
          <div class="stat-detail">{$t("vm.resources.vcpus")}: {resourceStats.cpu.vcpus}</div>
          <div class="progress-bar">
            <div class="progress" style="width: {resourceStats.cpu.usage_percent}%"></div>
          </div>
        </div>

        <!-- 内存 -->
        <div class="stat-card">
          <div class="stat-header">
            <Icon icon="mdi:memory" width="20" />
            <span>{$t("vm.resources.memory")}</span>
          </div>
          <div class="stat-value">{resourceStats.memory.used_percent.toFixed(1)}%</div>
          <div class="stat-detail">{resourceStats.memory.used} / {resourceStats.memory.total} MB</div>
          <div class="progress-bar">
            <div class="progress" style="width: {resourceStats.memory.used_percent}%"></div>
          </div>
        </div>

        <!-- 磁盘 I/O -->
        <div class="stat-card wide">
          <div class="stat-header">
            <Icon icon="mdi:harddisk" width="20" />
            <span>{$t("vm.resources.diskIO")}</span>
          </div>
          {#if resourceStats.disks && resourceStats.disks.length > 0}
            {#each resourceStats.disks as disk}
              <div class="disk-stat">
                <span class="disk-name">{disk.device || "disk"}</span>
                <span>{$t("vm.resources.read")}: {formatBytes(disk.bytes_read)}</span>
                <span>{$t("vm.resources.write")}: {formatBytes(disk.bytes_written)}</span>
              </div>
            {/each}
          {:else}
            <div class="stat-detail">{$t("vm.resources.noData")}</div>
          {/if}
        </div>

        <!-- 网络 -->
        <div class="stat-card wide">
          <div class="stat-header">
            <Icon icon="mdi:ethernet" width="20" />
            <span>{$t("vm.resources.network")}</span>
          </div>
          {#if resourceStats.networks && resourceStats.networks.length > 0}
            {#each resourceStats.networks as net}
              <div class="net-stat">
                <span class="net-name">{net.device}</span>
                <span>{$t("vm.resources.rx")}: {formatBytes(net.bytes_rx)}</span>
                <span>{$t("vm.resources.tx")}: {formatBytes(net.bytes_tx)}</span>
              </div>
            {/each}
          {:else}
            <div class="stat-detail">{$t("vm.resources.noData")}</div>
          {/if}
        </div>
      </div>

      <div class="stat-footer">
        <span class="timestamp">{$t("vm.resources.updatedAt")}: {new Date(resourceStats.timestamp * 1000).toLocaleTimeString()}</span>
      </div>
    {:else}
      <div class="loading-center">
        <p>{$t("vm.resources.cannotGetData")}</p>
      </div>
    {/if}
  </div>
</Modal>

<!-- VNC Console Modal -->
<Modal open={showVNC} title="{$t('vm.button.console')} - {vncVM?.name ?? ''}" onclose={() => { showVNC = false; vncVM = null; }} size="fullscreen">
  {#if vncVM}
    <VNCConsole vm={vncVM} onclose={() => { showVNC = false; vncVM = null; }} />
  {/if}
</Modal>

<style>
  .vm-manager {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--bg-primary, #f5f5f5);
  }

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    background: var(--bg-card, white);
    border-bottom: 1px solid var(--border-color, #e0e0e0);
    h2 { margin: 0; font-size: 18px; }
  }
  .header-actions { display: flex; gap: 8px; }

  .vm-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: 16px;
    padding: 20px;
    overflow-y: auto;
    flex: 1;
  }

  .vm-card {
    background: var(--bg-card, white);
    border-radius: 10px;
    padding: 18px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .vm-header {
    display: flex;
    gap: 12px;
    align-items: flex-start;
  }
  .vm-icon {
    width: 40px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-tertiary, #f5f5f5);
    border-radius: 8px;
    color: var(--text-secondary, #666);
    flex-shrink: 0;
  }
  .vm-title {
    flex: 1;
    min-width: 0;
  }
  .vm-top { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
  .vm-name { 
    font-weight: 600; 
    font-size: 15px; 
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .vm-meta {
    display: flex;
    gap: 8px;
    font-size: 11px;
    color: var(--text-muted, #999);
    margin-top: 2px;
  }
  .vm-id {
    font-family: monospace;
    background: var(--bg-tertiary, #f0f0f0);
    padding: 1px 4px;
    border-radius: 3px;
  }
  .vm-specs {
    display: flex; 
    gap: 12px; 
    font-size: 12px; 
    color: var(--text-secondary, #666);
    flex-wrap: wrap;
    span { display: flex; align-items: center; gap: 4px; }
  }
  .vm-desc { 
    font-size: 12px; 
    color: var(--text-muted, #999); 
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .vm-ports {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .port-tag {
    display: inline-flex;
    padding: 2px 8px;
    background: var(--bg-tertiary, #f0f0f0);
    border-radius: 4px;
    font-size: 11px;
    color: var(--text-muted, #999);
    font-family: monospace;
  }
  .port-tag.active {
    background: var(--color-success-light, #e8f5e9);
    color: var(--color-success, #4caf50);
  }
  .vm-actions { 
    display: flex; 
    gap: 4px; 
    flex-wrap: wrap; 
    align-items: center;
    padding-top: 8px;
    border-top: 1px solid var(--border-color, #eee);
  }
  .action-group {
    display: flex;
    gap: 2px;
    align-items: center;
  }
  .action-group.primary {
    margin-right: 4px;
  }
  .action-group.end {
    margin-left: auto;
  }

  /* 快照 */
  .snapshot-form { display: flex; gap: 8px; margin-bottom: 16px; }
  .snapshot-form input { flex: 1; padding: 8px 12px; border: 1px solid var(--border-color, #ddd); border-radius: 6px; font-size: 14px; }
  .snapshot-list { display: flex; flex-direction: column; gap: 8px; }
  .snapshot-row { display: flex; justify-content: space-between; align-items: center; padding: 10px; background: var(--bg-tertiary, #f5f5f5); border-radius: 6px; }
  .snap-name { font-weight: 500; font-size: 14px; }
  .snap-time { font-size: 12px; color: var(--text-muted, #999); margin-top: 2px; }
  .snap-actions { display: flex; gap: 4px; }

  /* ISO */
  .iso-list { display: flex; flex-direction: column; gap: 8px; }
  .iso-row { display: flex; justify-content: space-between; align-items: center; padding: 10px; background: var(--bg-tertiary, #f5f5f5); border-radius: 6px; }
  .iso-name { font-weight: 500; }
  .iso-info { font-size: 12px; color: var(--text-muted, #999); margin-top: 2px; }

  .text-muted { color: var(--text-muted, #999); }

  /* P3: 批量选择 */
  .select-info { font-size: 14px; color: var(--text-secondary); margin-right: 8px; }
  .vm-card.selected { border: 2px solid var(--primary, #0066cc); }
  .vm-select { position: absolute; top: 12px; left: 12px; }
  .vm-select input { width: 18px; height: 18px; cursor: pointer; }
  .vm-card { position: relative; }

  /* P3: 克隆 */
  .clone-form { display: flex; flex-direction: column; gap: 16px; }
  .clone-form label { display: flex; flex-direction: column; gap: 4px; }
  .clone-form label span { font-weight: 500; }
  .clone-form input { padding: 8px 12px; border: 1px solid var(--border-color, #ddd); border-radius: 6px; font-size: 14px; }
  .form-actions { display: flex; gap: 8px; justify-content: flex-end; }

  /* P3: 存储信息 */
  .storage-info { display: flex; flex-direction: column; gap: 20px; }
  .storage-overview { display: flex; flex-direction: column; gap: 8px; }
  .storage-bar { height: 24px; background: var(--bg-tertiary, #e0e0e0); border-radius: 12px; overflow: hidden; }
  .storage-bar .used { height: 100%; background: var(--primary, #0066cc); transition: width 0.3s; }
  .storage-text { font-size: 14px; text-align: center; }
  .storage-text .free { color: var(--text-muted, #999); }
  .storage-details { display: flex; flex-direction: column; gap: 8px; }
  .detail-row { display: flex; justify-content: space-between; align-items: center; padding: 8px 12px; background: var(--bg-tertiary, #f5f5f5); border-radius: 6px; }
  .detail-row .label { display: flex; align-items: center; gap: 8px; color: var(--text-secondary, #666); }
  .detail-row .value { font-weight: 500; }
  .vm-summary { display: flex; gap: 24px; justify-content: center; padding-top: 12px; border-top: 1px solid var(--border-color, #eee); color: var(--text-secondary, #666); }

  /* P5: 导入导出 */
  .import-form, .export-form { display: flex; flex-direction: column; gap: 16px; }
  .form-group { display: flex; flex-direction: column; gap: 6px; }
  .form-group label { font-weight: 500; font-size: 14px; color: var(--text-secondary, #666); }
  .form-group input, .form-group select {
    padding: 10px 12px;
    border: 1px solid var(--border-color, #ddd);
    border-radius: 6px;
    font-size: 14px;
    background: var(--bg-input, white);
  }
  .form-group .hint { font-size: 12px; color: var(--text-muted, #999); margin: 4px 0 0; }
  .export-info { 
    padding: 12px; 
    background: var(--bg-tertiary, #f5f5f5); 
    border-radius: 6px;
    font-size: 13px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .export-info p { display: flex; align-items: center; gap: 8px; margin: 0; }
  .export-info .warning { color: var(--color-warning, #f57c00); }

  /* P5: 自动启动按钮 */
  .auto-start-btn {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 8px;
    border: none;
    background: var(--bg-tertiary, #f5f5f5);
    border-radius: 4px;
    cursor: pointer;
    color: var(--text-muted, #999);
    font-size: 12px;
    transition: all 0.2s;
  }
  .auto-start-btn:hover { background: var(--bg-hover, #e8e8e8); }
  .auto-start-btn.enabled { 
    color: var(--color-success, #4caf50); 
    background: var(--color-success-light, #e8f5e9);
  }

  /* P6: SSH 表单 */
  .ssh-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .ssh-info {
    background: var(--bg-secondary, #fafafa);
    padding: 12px;
    border-radius: 8px;
  }
  .ssh-info p {
    margin: 4px 0;
  }
  .ssh-info .hint {
    font-size: 12px;
    color: var(--text-muted, #666);
  }
  .ssh-form label {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .ssh-form label span {
    font-size: 13px;
    color: var(--text-secondary, #666);
  }
  .ssh-form input {
    padding: 8px 12px;
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 6px;
    font-size: 14px;
  }

  /* P6: 资源监控仪表板 */
  .resources-dashboard {
    min-height: 300px;
  }
  .loading-center {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 200px;
    gap: 12px;
    color: var(--text-muted, #666);
  }
  .stats-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
  }
  .stat-card {
    background: var(--bg-secondary, #fafafa);
    padding: 16px;
    border-radius: 12px;
    border: 1px solid var(--border-color, #e0e0e0);
  }
  .stat-card.wide {
    grid-column: span 2;
  }
  .stat-header {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 500;
    margin-bottom: 8px;
    color: var(--text-primary, #333);
  }
  .stat-value {
    font-size: 28px;
    font-weight: 700;
    color: var(--color-primary, #1976d2);
    margin-bottom: 4px;
  }
  .stat-detail {
    font-size: 13px;
    color: var(--text-muted, #666);
    margin-bottom: 8px;
  }
  .progress-bar {
    height: 8px;
    background: var(--bg-tertiary, #e0e0e0);
    border-radius: 4px;
    overflow: hidden;
  }
  .progress-bar .progress {
    height: 100%;
    background: linear-gradient(90deg, var(--color-primary, #1976d2), var(--color-success, #4caf50));
    transition: width 0.3s ease;
  }
  .disk-stat, .net-stat {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 8px 0;
    border-bottom: 1px solid var(--border-color, #e0e0e0);
    font-size: 13px;
  }
  .disk-stat:last-child, .net-stat:last-child {
    border-bottom: none;
  }
  .disk-name, .net-name {
    font-weight: 500;
    min-width: 80px;
    color: var(--text-primary, #333);
  }
  .stat-footer {
    margin-top: 16px;
    padding-top: 12px;
    border-top: 1px solid var(--border-color, #e0e0e0);
    text-align: right;
  }
  .timestamp {
    font-size: 12px;
    color: var(--text-muted, #888);
  }
</style>
