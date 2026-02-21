<script lang="ts">
  import { goto } from "$app/navigation";
  import { _ as t } from "svelte-i18n";
  import { onDestroy } from "svelte";
  import { Button, Input, Progress, Spinner } from "$shared/ui";
  import { cloudRestoreApi, type CloudBackupItem, type RestoreProgress } from "$shared/services/setup";

  import SetupCard from "../SetupCard.svelte";

  // 阶段: login -> backups -> restoring -> done
  type RestoreStage = "login" | "backups" | "restoring" | "done";

  let stage = $state<RestoreStage>("login");
  let error = $state("");
  let loading = $state(false);

  // 登录
  let email = $state("");
  let emailCode = $state("");
  let codeSent = $state(false);
  let codeCountdown = $state(0);
  let cloudToken = $state("");

  // 备份列表
  let backups = $state<CloudBackupItem[]>([]);
  let selectedBackup = $state<CloudBackupItem | null>(null);
  let restorePassword = $state("");

  // 恢复进度
  let progress = $state<RestoreProgress>({ stage: "idle", percent: 0 });
  let progressTimer: ReturnType<typeof setInterval> | null = null;
  let countdownTimer: ReturnType<typeof setInterval> | null = null;

  const stageLabels: Record<string, string> = {
    connecting: "连接云端...",
    downloading: "下载备份文件...",
    decrypting: "解密中...",
    extracting: "解压中...",
    importing: "导入数据库...",
    restoring_files: "恢复配置文件...",
    completed: "恢复完成！",
    error: "恢复失败",
  };

  // 发送验证码
  async function sendCode() {
    if (!email || !email.includes("@")) {
      error = $t("setup.cloudRestore.invalidEmail");
      return;
    }
    error = "";
    loading = true;
    try {
      await cloudRestoreApi.sendEmailCode(email);
      codeSent = true;
      codeCountdown = 60;
      countdownTimer = setInterval(() => {
        codeCountdown--;
        if (codeCountdown <= 0) {
          if (countdownTimer) clearInterval(countdownTimer);
          countdownTimer = null;
        }
      }, 1000);
    } catch (e) {
      error = e instanceof Error ? e.message : "发送失败";
    } finally {
      loading = false;
    }
  }

  // 登录
  async function handleLogin() {
    if (!email || !emailCode) {
      error = $t("setup.cloudRestore.inputEmailAndCode");
      return;
    }
    error = "";
    loading = true;
    try {
      cloudToken = await cloudRestoreApi.login(email, emailCode);
      // 自动获取备份列表
      backups = await cloudRestoreApi.listBackups(cloudToken);
      stage = "backups";
    } catch (e) {
      error = e instanceof Error ? e.message : "登录失败";
    } finally {
      loading = false;
    }
  }

  // 开始恢复
  async function startRestore() {
    if (!selectedBackup || !restorePassword) {
      error = $t("setup.cloudRestore.inputPassword");
      return;
    }
    error = "";
    loading = true;
    try {
      await cloudRestoreApi.startRestore(cloudToken, selectedBackup.id, restorePassword);
      stage = "restoring";
      // 轮询进度
      progressTimer = setInterval(async () => {
        try {
          progress = await cloudRestoreApi.getProgress();
          if (progress.stage === "completed") {
            if (progressTimer) clearInterval(progressTimer);
            progressTimer = null;
            stage = "done";
          } else if (progress.stage === "error") {
            if (progressTimer) clearInterval(progressTimer);
            progressTimer = null;
            error = progress.error || "恢复失败";
            stage = "backups";
          }
        } catch {
          // 忽略轮询错误
        }
      }, 1000);
    } catch (e) {
      error = e instanceof Error ? e.message : "启动恢复失败";
    } finally {
      loading = false;
    }
  }

  function formatSize(bytes: number): string {
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
    return (bytes / (1024 * 1024)).toFixed(1) + " MB";
  }

  function goBack() {
    goto("/setup/step1");
  }

  onDestroy(() => {
    if (progressTimer) clearInterval(progressTimer);
    if (countdownTimer) clearInterval(countdownTimer);
  });
</script>

<SetupCard
  header={{
    icon: "☁️",
    title: $t("setup.cloudRestore.title"),
    description: $t("setup.cloudRestore.description"),
  }}
  {error}
>
  {#if stage === "login"}
    <!-- 邮箱登录 -->
    <div class="flex flex-col gap-4">
      <div class="flex flex-col gap-2">
        <label class="text-sm font-medium">{$t("setup.cloudRestore.email")}</label>
        <Input
          type="email"
          placeholder="user@example.com"
          bind:value={email}
        />
      </div>

      <div class="flex gap-2 items-end">
        <div class="flex flex-col gap-2 flex-1">
          <label class="text-sm font-medium">{$t("setup.cloudRestore.verifyCode")}</label>
          <Input
            type="text"
            placeholder="123456"
            bind:value={emailCode}
            maxlength={6}
          />
        </div>
        <Button
          variant="secondary"
          onclick={sendCode}
          disabled={loading || codeCountdown > 0}
        >
          {#if codeCountdown > 0}
            {codeCountdown}s
          {:else}
            {$t("setup.cloudRestore.sendCode")}
          {/if}
        </Button>
      </div>
    </div>

  {:else if stage === "backups"}
    <!-- 备份列表 -->
    {#if backups.length === 0}
      <div class="text-center py-8">
        <p class="text-lg opacity-60">☁️</p>
        <p class="text-sm opacity-40 mt-2">{$t("setup.cloudRestore.noBackups")}</p>
      </div>
    {:else}
      <div class="flex flex-col gap-3">
        <p class="text-sm opacity-60">{$t("setup.cloudRestore.selectBackup")}</p>
        {#each backups as backup}
          <button
            class="backup-item"
            class:selected={selectedBackup?.id === backup.id}
            onclick={() => (selectedBackup = backup)}
          >
            <div class="flex items-center gap-3">
              <span class="text-xl">💾</span>
              <div class="text-left">
                <p class="font-medium text-sm">{backup.id}</p>
                <p class="text-xs opacity-50">
                  {backup.created_at} · {formatSize(backup.size)}
                </p>
              </div>
            </div>
            {#if selectedBackup?.id === backup.id}
              <span class="text-green-500">✓</span>
            {/if}
          </button>
        {/each}
      </div>

      {#if selectedBackup}
        <div class="mt-4 flex flex-col gap-2">
          <label class="text-sm font-medium">{$t("setup.cloudRestore.backupPassword")}</label>
          <Input
            type="password"
            placeholder={$t("setup.cloudRestore.backupPasswordHint")}
            bind:value={restorePassword}
          />
        </div>
      {/if}
    {/if}

  {:else if stage === "restoring"}
    <!-- 恢复进度 -->
    <div class="flex flex-col items-center gap-6 py-8">
      <Spinner size="lg" />
      <div class="text-center">
        <p class="font-medium">{stageLabels[progress.stage] || progress.stage}</p>
        <p class="text-sm opacity-50 mt-1">{progress.percent}%</p>
      </div>
      <Progress value={progress.percent} showLabel />
    </div>

  {:else if stage === "done"}
    <!-- 恢复完成 -->
    <div class="flex flex-col items-center gap-6 py-8">
      <span class="text-5xl">🎉</span>
      <div class="text-center">
        <p class="text-lg font-semibold">{$t("setup.cloudRestore.restoreComplete")}</p>
        <p class="text-sm opacity-50 mt-2">{$t("setup.cloudRestore.restoreCompleteHint")}</p>
      </div>
    </div>
  {/if}

  {#snippet footer()}
    {#if stage === "login"}
      <Button variant="ghost" onclick={goBack}>
        {$t("setup.prevStep")}
      </Button>
      <Button variant="primary" onclick={handleLogin} {loading} disabled={!email || !emailCode}>
        {$t("setup.cloudRestore.login")}
      </Button>
    {:else if stage === "backups"}
      <Button variant="ghost" onclick={() => (stage = "login")}>
        {$t("setup.prevStep")}
      </Button>
      <Button
        variant="primary"
        onclick={startRestore}
        {loading}
        disabled={!selectedBackup || !restorePassword}
      >
        {$t("setup.cloudRestore.startRestore")}
      </Button>
    {:else if stage === "done"}
      <Button variant="primary" onclick={() => goto("/setup/step2")}>
        {$t("setup.cloudRestore.continueSetup")}
      </Button>
    {/if}
  {/snippet}
</SetupCard>

<style>
  .backup-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    border-radius: 12px;
    border: 1px solid rgba(0, 0, 0, 0.08);
    background: rgba(255, 255, 255, 0.5);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .backup-item:hover {
    border-color: rgba(0, 0, 0, 0.15);
    background: rgba(255, 255, 255, 0.8);
  }

  .backup-item.selected {
    border-color: var(--color-primary, #3b82f6);
    background: rgba(59, 130, 246, 0.05);
  }

  :global(.dark) .backup-item {
    border-color: rgba(255, 255, 255, 0.08);
    background: rgba(255, 255, 255, 0.03);
  }

  :global(.dark) .backup-item:hover {
    border-color: rgba(255, 255, 255, 0.15);
    background: rgba(255, 255, 255, 0.06);
  }

  :global(.dark) .backup-item.selected {
    border-color: var(--color-primary, #3b82f6);
    background: rgba(59, 130, 246, 0.1);
  }
</style>
