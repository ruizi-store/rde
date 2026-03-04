<script lang="ts">
  import { t } from "./i18n";
  import { vmService, type BackupInfo, type VM } from "./service";
  import { Button, Modal } from "$shared/ui";
  import { formatBytes, formatDate } from "$shared/utils/format";

  interface Props {
    vm: VM;
    onClose: () => void;
    onRestore?: (vm: VM) => void;
  }

  let { vm, onClose, onRestore }: Props = $props();

  let backups = $state<BackupInfo[]>([]);
  let loading = $state(false);
  let creating = $state(false);
  let showCreateDialog = $state(false);
  let newBackupName = $state("");
  let newBackupDescription = $state("");

  async function loadBackups() {
    loading = true;
    try {
      backups = await vmService.listBackups(vm.id);
    } catch (err) {
      console.error("Load backups failed:", err);
    } finally {
      loading = false;
    }
  }

  async function createBackup() {
    creating = true;
    try {
      await vmService.createBackup({
        vm_id: vm.id,
        name: newBackupName || undefined,
        description: newBackupDescription || undefined,
      });
      showCreateDialog = false;
      newBackupName = "";
      newBackupDescription = "";
      await loadBackups();
    } catch (err) {
      console.error("Create backup failed:", err);
      alert($t("vm.backup.createFailed") + ": " + (err as Error).message);
    } finally {
      creating = false;
    }
  }

  async function restoreBackup(backup: BackupInfo) {
    if (!confirm($t("vm.confirm.restoreBackup", { values: { name: backup.name || backup.id } }))) return;
    
    try {
      const restoredVM = await vmService.restoreBackup(backup.id);
      alert($t("vm.backup.restoredAs", { values: { name: restoredVM.name } }));
      onRestore?.(restoredVM);
    } catch (err) {
      console.error("Restore backup failed:", err);
      alert($t("vm.backup.restoreFailed") + ": " + (err as Error).message);
    }
  }

  async function deleteBackup(backup: BackupInfo) {
    if (!confirm($t("vm.confirm.deleteBackup", { values: { name: backup.name || backup.id } }))) return;
    
    try {
      await vmService.deleteBackup(backup.id);
      await loadBackups();
    } catch (err) {
      console.error("Delete backup failed:", err);
      alert($t("vm.backup.deleteFailed") + ": " + (err as Error).message);
    }
  }

  $effect(() => {
    loadBackups();
  });
</script>

<div class="vm-backups">
  <div class="header">
    <h3>{$t("vm.backup.title", { values: { name: vm.name } })}</h3>
    <div class="actions">
      <Button 
        onclick={() => showCreateDialog = true} 
        disabled={vm.status === "running"}
      >
        {$t("vm.backup.createBackup")}
      </Button>
      <Button onclick={onClose} variant="secondary">{$t("vm.backup.close")}</Button>
    </div>
  </div>

  {#if vm.status === "running"}
    <div class="warning">
      <span class="icon">⚠️</span>
      {$t("vm.backup.shutdownFirst")}
    </div>
  {/if}

  {#if loading}
    <div class="loading">{$t("vm.backup.loadingList")}</div>
  {:else if backups.length === 0}
    <div class="empty">
      <p>{$t("vm.backup.noBackups")}</p>
      <p class="hint">{$t("vm.backup.backupHint")}</p>
    </div>
  {:else}
    <div class="backup-list">
      {#each backups as backup}
        <div class="backup-item">
          <div class="info">
            <div class="name">{backup.name || backup.id}</div>
            <div class="meta">
              <span>{formatBytes(backup.size)}</span>
              <span>{formatDate(backup.created_at)}</span>
            </div>
            {#if backup.description}
              <div class="description">{backup.description}</div>
            {/if}
          </div>
          <div class="actions">
            <Button onclick={() => restoreBackup(backup)} size="sm">{$t("vm.backup.restore")}</Button>
            <Button onclick={() => deleteBackup(backup)} size="sm" variant="danger">{$t("vm.backup.delete")}</Button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<Modal bind:open={showCreateDialog} title={$t("vm.backup.createBackup")} size="sm">
  <div class="create-form">
    <label>
      <span>{$t("vm.backup.backupName")}</span>
      <input type="text" bind:value={newBackupName} placeholder={$t("vm.backup.optional")} />
    </label>
    <label>
      <span>{$t("vm.backup.description")}</span>
      <textarea bind:value={newBackupDescription} placeholder={$t("vm.backup.optional")}></textarea>
    </label>
    <div class="form-actions">
      <Button onclick={createBackup} disabled={creating}>
        {creating ? $t("vm.backup.creating") : $t("vm.backup.createBackup")}
      </Button>
      <Button onclick={() => showCreateDialog = false} variant="secondary">{$t("common.cancel")}</Button>
    </div>
  </div>
</Modal>

<style>
  .vm-backups {
    padding: 1rem;
    max-height: 500px;
    overflow-y: auto;
  }

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .header h3 {
    margin: 0;
  }

  .header .actions {
    display: flex;
    gap: 0.5rem;
  }

  .warning {
    background: var(--warning-bg, #fff3cd);
    color: var(--warning-text, #856404);
    padding: 0.75rem 1rem;
    border-radius: 4px;
    margin-bottom: 1rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .loading, .empty {
    text-align: center;
    padding: 2rem;
    color: var(--text-secondary);
  }

  .empty .hint {
    font-size: 0.875rem;
    margin-top: 0.5rem;
  }

  .backup-list {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .backup-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem;
    background: var(--surface-1);
    border-radius: 8px;
    border: 1px solid var(--border);
  }

  .backup-item .info {
    flex: 1;
  }

  .backup-item .name {
    font-weight: 500;
  }

  .backup-item .meta {
    font-size: 0.875rem;
    color: var(--text-secondary);
    display: flex;
    gap: 1rem;
    margin-top: 0.25rem;
  }

  .backup-item .description {
    font-size: 0.875rem;
    color: var(--text-secondary);
    margin-top: 0.25rem;
  }

  .backup-item .actions {
    display: flex;
    gap: 0.5rem;
  }

  .create-form {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .create-form label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .create-form label span {
    font-weight: 500;
  }

  .create-form input, .create-form textarea {
    padding: 0.5rem;
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--input-bg);
    color: inherit;
  }

  .create-form textarea {
    min-height: 80px;
    resize: vertical;
  }

  .form-actions {
    display: flex;
    gap: 0.5rem;
    justify-content: flex-end;
    margin-top: 0.5rem;
  }
</style>
