<script lang="ts">
  import { t } from "svelte-i18n";
  import Icon from "@iconify/svelte";
  import { Button, Modal, Spinner } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import {
    dockerStoreService,
    type StoreAppDetail,
    type FormField,
  } from "./store-service";

  // ==================== Props ====================
  interface Props {
    app: StoreAppDetail;
    open?: boolean;
    onClose?: () => void;
    onInstalled?: () => void;
  }
  let { app, open = $bindable(false), onClose, onInstalled }: Props = $props();

  // ==================== 状态 ====================
  let step = $state(0); // 0=配置, 1=确认, 2=部署中, 3=完成/失败
  let formValues = $state<Record<string, string | number>>({});
  let portStatus = $state<Record<string, { checking: boolean; available: boolean; suggested?: number }>>({});
  let installing = $state(false);
  let installOutput = $state("");
  let installError = $state("");
  let deployLogEl: HTMLPreElement;

  // ==================== 初始化表单默认值 ====================
  $effect(() => {
    if (open && app?.form) {
      const defaults: Record<string, string | number> = {};
      for (const field of app.form) {
        if (field.default !== undefined && field.default !== null) {
          defaults[field.key] = field.default as string | number;
        } else {
          defaults[field.key] = field.type === "number" ? 0 : "";
        }
      }
      formValues = defaults;
      step = 0;
      installOutput = "";
      installError = "";
      portStatus = {};
    }
  });

  // 日志自动滚动到底部
  $effect(() => {
    if (installOutput && deployLogEl) {
      deployLogEl.scrollTop = deployLogEl.scrollHeight;
    }
  });

  // ==================== 方法 ====================

  function getFieldLabel(field: FormField): string {
    return field.label?.zh || field.label?.en || field.key;
  }

  async function checkPort(fieldKey: string, port: number) {
    if (!port || port < 1 || port > 65535) return;
    portStatus[fieldKey] = { checking: true, available: false };
    try {
      const available = await dockerStoreService.checkPort(port);
      if (!available) {
        const suggested = await dockerStoreService.suggestPort(port);
        portStatus[fieldKey] = { checking: false, available: false, suggested };
      } else {
        portStatus[fieldKey] = { checking: false, available: true };
      }
    } catch {
      portStatus[fieldKey] = { checking: false, available: true }; // 检测失败默认可用
    }
  }

  function applySuggestedPort(fieldKey: string) {
    const ps = portStatus[fieldKey];
    if (ps?.suggested) {
      formValues[fieldKey] = ps.suggested;
      portStatus[fieldKey] = { checking: false, available: true };
    }
  }

  function validateForm(): string | null {
    for (const field of app.form) {
      if (field.required) {
        const val = formValues[field.key];
        if (val === undefined || val === null || val === "") {
          return `请填写 ${getFieldLabel(field)}`;
        }
      }
    }
    return null;
  }

  function goToConfirm() {
    const err = validateForm();
    if (err) {
      showToast(err, "error");
      return;
    }
    step = 1;
  }

  function getConfigSummary(): Array<{ label: string; value: string }> {
    return app.form.map((f) => ({
      label: getFieldLabel(f),
      value: String(formValues[f.key] ?? ""),
    }));
  }

  function getPreviewCompose(): string {
    let compose = app.compose || "";
    for (const [key, val] of Object.entries(formValues)) {
      const envKey = app.form.find((f) => f.key === key)?.env_key;
      if (envKey) {
        compose = compose.replaceAll(`\${${envKey}}`, String(val));
      }
      compose = compose.replaceAll(`\${${key}}`, String(val));
    }
    return compose;
  }

  async function doInstall() {
    step = 2;
    installing = true;
    installOutput = "";
    installError = "";

    try {
      // 构建 config: key -> value (包含 env_key 映射)
      const config: Record<string, unknown> = {};
      for (const field of app.form) {
        config[field.key] = formValues[field.key];
        // 同时以 env_key 传递，后端可直接做变量替换
        if (field.env_key) {
          config[field.env_key] = formValues[field.key];
        }
      }

      // 异步安装 + 轮询进度
      const task = await dockerStoreService.installAppAsync(app.id, config);
      installOutput = $t("docker.install.pullingImage");

      const finalTask = await dockerStoreService.pollInstallTask(
        task.id,
        (t) => {
          if (t.output) installOutput = t.output;
        },
        2000,
      );

      if (finalTask.status === "success") {
        installOutput = finalTask.output || $t("docker.install.installComplete");
        step = 3;
        showToast($t("docker.install.installSuccess", { values: { name: app.title || app.name } }), "success");
      } else {
        installError = finalTask.error || $t("docker.install.installFailed");
        installOutput = finalTask.output || "";
        step = 3;
      }
    } catch (e: any) {
      installError = e.message || $t("docker.install.installFailed");
      installOutput = (e as any).output || "";
      step = 3;
    } finally {
      installing = false;
    }
  }

  function handleClose() {
    open = false;
    onClose?.();
    if (step === 3 && !installError) {
      onInstalled?.();
    }
  }

  function handleFieldInput(field: FormField, e: Event) {
    const input = e.target as HTMLInputElement;
    if (field.type === "number") {
      formValues[field.key] = Number(input.value) || 0;
      // 自动检查端口（字段名包含 port）
      if (field.key.toLowerCase().includes("port")) {
        const port = Number(input.value);
        if (port > 0 && port <= 65535) {
          checkPort(field.key, port);
        }
      }
    } else {
      formValues[field.key] = input.value;
    }
  }

  // 步骤标题
  function getStepTitle(): string {
    switch (step) {
      case 0: return $t("docker.install.configParams");
      case 1: return $t("docker.install.confirmInstall");
      case 2: return $t("docker.install.deploying");
      case 3: return installError ? $t("docker.install.installFailed") : $t("docker.install.installComplete");
      default: return "";
    }
  }
</script>

<Modal bind:open title="{$t('docker.install.install')} {app.title || app.name} - {getStepTitle()}" size="md" closable={step !== 2} closeOnOverlay={false}>
  <div class="install-wizard">
    <!-- 步骤指示器 -->
    <div class="steps">
      {#each [$t("docker.install.config"), $t("docker.install.confirm"), $t("docker.install.deploy"), $t("docker.install.complete")] as label, i}
        <div class="step" class:active={step === i} class:done={step > i}>
          <div class="step-dot">
            {#if step > i}
              <Icon icon="mdi:check" width="12" />
            {:else}
              {i + 1}
            {/if}
          </div>
          <span class="step-label">{label}</span>
        </div>
        {#if i < 3}
          <div class="step-line" class:done={step > i}></div>
        {/if}
      {/each}
    </div>

    <!-- Step 0: 配置表单 -->
    {#if step === 0}
      <div class="form-section">
        {#if app.form.length === 0}
          <div class="no-config">
            <Icon icon="mdi:check-circle-outline" width="32" />
            <p>{$t("docker.install.noConfigNeeded")}</p>
          </div>
        {:else}
          {#each app.form as field (field.key)}
            <div class="form-group">
              <label for="install-{field.key}">
                {getFieldLabel(field)}
                {#if field.required}
                  <span class="required-mark">*</span>
                {/if}
              </label>
              {#if field.type === "password"}
                <div class="input-with-action">
                  <input
                    id="install-{field.key}"
                    type="text"
                    value={formValues[field.key] ?? ""}
                    oninput={(e) => handleFieldInput(field, e)}
                    placeholder={field.default != null ? `默认: ${field.default}` : ""}
                    required={field.required}
                  />
                  <button class="gen-btn" title={$t("docker.install.generatePassword")} onclick={() => {
                    const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%";
                    formValues[field.key] = Array.from({ length: 16 }, () => chars[Math.floor(Math.random() * chars.length)]).join("");
                  }}>
                    <Icon icon="mdi:dice-5-outline" width="16" />
                  </button>
                </div>
              {:else}
                <input
                  id="install-{field.key}"
                  type={field.type === "number" ? "number" : "text"}
                  value={formValues[field.key] ?? ""}
                  oninput={(e) => handleFieldInput(field, e)}
                  placeholder={field.default != null ? `默认: ${field.default}` : ""}
                  required={field.required}
                />
              {/if}
              <!-- 端口状态 -->
              {#if field.key.toLowerCase().includes("port") && portStatus[field.key]}
                <div class="port-status">
                  {#if portStatus[field.key].checking}
                    <span class="port-checking"><Spinner /> {$t("docker.install.checkingPort")}</span>
                  {:else if portStatus[field.key].available}
                    <span class="port-ok"><Icon icon="mdi:check-circle" width="14" /> {$t("docker.install.portAvailable")}</span>
                  {:else}
                    <span class="port-conflict">
                      <Icon icon="mdi:alert" width="14" /> {$t("docker.install.portOccupied")}
                      {#if portStatus[field.key].suggested}
                        <button class="suggest-btn" onclick={() => applySuggestedPort(field.key)}>
                          {$t("docker.install.useSuggested", { values: { port: portStatus[field.key].suggested } })}
                        </button>
                      {/if}
                    </span>
                  {/if}
                </div>
              {/if}
            </div>
          {/each}
        {/if}
      </div>

      <div class="wizard-actions">
        <Button variant="ghost" onclick={handleClose}>{$t("common.cancel")}</Button>
        <Button variant="primary" onclick={goToConfirm}>
          {$t("docker.install.next")}
        </Button>
      </div>

    <!-- Step 1: 确认 -->
    {:else if step === 1}
      <div class="confirm-section">
        <div class="confirm-app">
          <img
            src={dockerStoreService.getIconUrl(app.icon)}
            alt={app.name}
            class="confirm-icon"
            onerror={(e) => { (e.target as HTMLImageElement).style.display = "none"; }}
          />
          <div>
            <div class="confirm-app-name">{app.title || app.name}</div>
            <div class="confirm-app-ver">v{app.version}</div>
          </div>
        </div>

        {#if getConfigSummary().length > 0}
          <div class="confirm-config">
            <h4>{$t("docker.install.configSummary")}</h4>
            {#each getConfigSummary() as item}
              <div class="confirm-row">
                <span class="confirm-label">{item.label}</span>
                <span class="confirm-value">{item.value}</span>
              </div>
            {/each}
          </div>
        {/if}

        <details class="compose-details">
          <summary>{$t("docker.install.viewCompose")}</summary>
          <pre class="compose-code">{getPreviewCompose()}</pre>
        </details>
      </div>

      <div class="wizard-actions">
        <Button variant="ghost" onclick={() => (step = 0)}>
          {$t("docker.install.previous")}
        </Button>
        <Button variant="primary" onclick={doInstall}>
          {$t("docker.install.startInstall")}
        </Button>
      </div>

    <!-- Step 2: 部署中 -->
    {:else if step === 2}
      <div class="deploy-section">
        <div class="deploy-anim">
          <Spinner />
          <p>{$t("docker.install.deploying")} {app.title || app.name}...</p>
        </div>
        {#if installOutput}
          <pre class="deploy-log" bind:this={deployLogEl}>{installOutput}</pre>
        {/if}
      </div>

    <!-- Step 3: 完成/失败 -->
    {:else if step === 3}
      <div class="result-section">
        {#if installError}
          <div class="result-icon error">
            <Icon icon="mdi:close-circle" width="48" />
          </div>
          <p class="result-title">{$t("docker.install.installFailed")}</p>
          <p class="result-msg">{installError}</p>
        {:else}
          <div class="result-icon success">
            <Icon icon="mdi:check-circle" width="48" />
          </div>
          <p class="result-title">{$t("docker.install.installComplete")}</p>
          <p class="result-msg">{$t("docker.install.deployedSuccess", { values: { name: app.title || app.name } })}</p>
        {/if}

        {#if installOutput}
          <details class="output-details" open={!!installError}>
            <summary>{$t("docker.install.deployLog")}</summary>
            <pre class="output-log">{installOutput}</pre>
          </details>
        {/if}
      </div>

      <div class="wizard-actions">
        {#if installError}
          <Button variant="ghost" onclick={() => (step = 1)}>
            {$t("docker.install.backToModify")}
          </Button>
        {/if}
        <Button variant="primary" onclick={handleClose}>
          {installError ? $t("docker.install.close") : $t("docker.install.complete")}
        </Button>
      </div>
    {/if}
  </div>
</Modal>

<style>
  .install-wizard {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  /* 步骤指示器 */
  .steps {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0;
    padding: 4px 0;
  }

  .step {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }

  .step-dot {
    width: 24px;
    height: 24px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 11px;
    font-weight: 600;
    background: var(--bg-secondary, #f0f0f0);
    color: var(--text-muted, #999);
    transition: all 0.2s;
  }

  .step.active .step-dot {
    background: var(--color-primary, #4a90d9);
    color: white;
  }

  .step.done .step-dot {
    background: var(--color-success, #27ae60);
    color: white;
  }

  .step-label {
    font-size: 11px;
    color: var(--text-muted, #999);
  }

  .step.active .step-label {
    color: var(--color-primary, #4a90d9);
    font-weight: 500;
  }

  .step.done .step-label {
    color: var(--color-success, #27ae60);
  }

  .step-line {
    width: 40px;
    height: 2px;
    background: var(--border-color, #e0e0e0);
    margin: 0 6px;
    margin-bottom: 18px;
    transition: background 0.2s;
  }

  .step-line.done {
    background: var(--color-success, #27ae60);
  }

  /* 表单 */
  .form-section {
    display: flex;
    flex-direction: column;
    gap: 14px;
    max-height: 400px;
    overflow-y: auto;
  }

  .no-config {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 30px 0;
    color: var(--text-muted, #999);
  }
  .no-config p {
    margin: 0;
    font-size: 14px;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .form-group label {
    font-size: 13px;
    font-weight: 500;
    color: var(--text-secondary, #666);
  }

  .required-mark {
    color: var(--color-error, #e74c3c);
    margin-left: 2px;
  }

  .form-group input {
    padding: 9px 12px;
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 6px;
    font-size: 14px;
    background: var(--bg-input, white);
    color: var(--text-primary, #333);
    font-family: inherit;
  }
  .form-group input:focus {
    outline: none;
    border-color: var(--color-primary, #4a90d9);
  }

  .input-with-action {
    display: flex;
    gap: 6px;
  }
  .input-with-action input {
    flex: 1;
  }

  .gen-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 6px;
    background: var(--bg-card, white);
    color: var(--text-secondary, #666);
    cursor: pointer;
    transition: all 0.15s;
  }
  .gen-btn:hover {
    border-color: var(--color-primary, #4a90d9);
    color: var(--color-primary, #4a90d9);
  }

  /* 端口状态 */
  .port-status {
    font-size: 12px;
  }

  .port-checking {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--text-muted, #999);
  }

  .port-ok {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--color-success, #27ae60);
  }

  .port-conflict {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--color-warning, #f39c12);
  }

  .suggest-btn {
    padding: 2px 8px;
    border: 1px solid var(--color-primary, #4a90d9);
    border-radius: 4px;
    background: transparent;
    color: var(--color-primary, #4a90d9);
    font-size: 12px;
    cursor: pointer;
  }
  .suggest-btn:hover {
    background: var(--color-primary, #4a90d9);
    color: white;
  }

  /* 确认 */
  .confirm-section {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .confirm-app {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px;
    background: var(--bg-secondary, #f5f5f5);
    border-radius: 8px;
  }

  .confirm-icon {
    width: 40px;
    height: 40px;
    border-radius: 8px;
    object-fit: contain;
    background: white;
  }

  .confirm-app-name {
    font-weight: 600;
    font-size: 15px;
    color: var(--text-primary, #333);
  }

  .confirm-app-ver {
    font-size: 12px;
    color: var(--text-muted, #999);
  }

  .confirm-config h4 {
    margin: 0 0 8px;
    font-size: 13px;
    color: var(--text-secondary, #666);
  }

  .confirm-row {
    display: flex;
    justify-content: space-between;
    padding: 6px 0;
    border-bottom: 1px solid var(--border-color, #f0f0f0);
    font-size: 13px;
  }
  .confirm-row:last-child {
    border-bottom: none;
  }

  .confirm-label {
    color: var(--text-muted, #999);
  }

  .confirm-value {
    color: var(--text-primary, #333);
    font-weight: 500;
  }

  .compose-details,
  .output-details {
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 6px;
    overflow: hidden;
  }

  .compose-details summary,
  .output-details summary {
    padding: 10px 14px;
    font-size: 13px;
    color: var(--text-secondary, #666);
    cursor: pointer;
    background: var(--bg-secondary, #f5f5f5);
  }

  .compose-code,
  .output-log {
    margin: 0;
    padding: 14px;
    background: #1a1a2e;
    color: #e0e0e0;
    font-size: 12px;
    line-height: 1.6;
    overflow-x: auto;
    white-space: pre;
    font-family: "JetBrains Mono", "Fira Code", monospace;
    max-height: 300px;
    overflow-y: auto;
  }

  /* 部署中 */
  .deploy-section {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 16px 0 8px;
  }

  .deploy-anim {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
  }

  .deploy-anim p {
    margin: 0;
    font-size: 14px;
    color: var(--text-primary, #333);
    font-weight: 500;
  }

  .deploy-log {
    width: 100%;
    max-height: 160px;
    overflow-y: auto;
    padding: 10px 12px;
    margin: 0;
    background: #1a1a2e;
    color: #8bc34a;
    border-radius: 6px;
    font-size: 12px;
    font-family: "SF Mono", "Fira Code", "Cascadia Code", monospace;
    line-height: 1.5;
    white-space: pre-wrap;
    word-break: break-all;
  }

  /* 结果 */
  .result-section {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 20px 0;
  }

  .result-icon.success {
    color: var(--color-success, #27ae60);
  }

  .result-icon.error {
    color: var(--color-error, #e74c3c);
  }

  .result-title {
    margin: 0;
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary, #333);
  }

  .result-msg {
    margin: 0;
    font-size: 13px;
    color: var(--text-muted, #999);
  }

  /* 操作区 */
  .wizard-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding-top: 8px;
    border-top: 1px solid var(--border-color, #f0f0f0);
  }
</style>
