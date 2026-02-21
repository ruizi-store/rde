<script lang="ts">
  import { goto } from "$app/navigation";
  import { setupApi, type SetupUserRequest } from "$shared/services/setup";
  import { Button, Input, Switch } from "$shared/ui";
  import SetupCard from "../SetupCard.svelte";
  import FormField from "../FormField.svelte";
  import { _ as t } from "svelte-i18n";

  interface TwoFactorSetupResult {
    totp_secret: string;
    totp_qr_code: string;
    recovery_codes: string[];
  }

  // 表单数据
  let formData = $state<SetupUserRequest>({
    username: "",
    password: "",
    enable_2fa: false,
  });

  let password_confirm = $state("");

  // 端口模式: 'default' | 'random' | 'custom'
  let portMode = $state<"default" | "random" | "custom">("default");
  let customPort = $state("");
  let customPortError = $state("");
  let newPort = $state<number | null>(null);

  // 2FA 设置
  let showTwoFactorSetup = $state(false);
  let twoFactorResult = $state<TwoFactorSetupResult | null>(null);
  let totpCode = $state("");
  let copiedCodes = $state(false);

  // 密码强度
  let passwordStrength = $derived(calculatePasswordStrength(formData.password));

  // 表单验证
  let usernameError = $state("");
  let passwordError = $state("");
  let confirmError = $state("");

  let loading = $state(false);
  let error = $state("");

  function calculatePasswordStrength(password: string): {
    score: number;
    labelKey: string;
    color: string;
  } {
    if (!password) return { score: 0, labelKey: "setup.passwordStrengthEmpty", color: "#94a3b8" };

    let score = 0;

    // 长度
    if (password.length >= 8) score += 1;
    if (password.length >= 12) score += 1;
    if (password.length >= 16) score += 1;

    // 复杂度
    if (/[a-z]/.test(password)) score += 1;
    if (/[A-Z]/.test(password)) score += 1;
    if (/[0-9]/.test(password)) score += 1;
    if (/[^a-zA-Z0-9]/.test(password)) score += 1;

    if (score <= 2) return { score: 25, labelKey: "setup.passwordStrengthWeak", color: "#ef4444" };
    if (score <= 4) return { score: 50, labelKey: "setup.passwordStrengthMedium", color: "#f59e0b" };
    if (score <= 5) return { score: 75, labelKey: "setup.passwordStrengthStrong", color: "#10b981" };
    return { score: 100, labelKey: "setup.passwordStrengthVeryStrong", color: "#059669" };
  }

  function validateUsername(value: string): string {
    if (!value) return $t("setup.usernameRequired");
    if (value.length < 3) return $t("setup.usernameMinLength");
    if (value.length > 32) return $t("setup.usernameMaxLength");
    if (!/^[a-zA-Z][a-zA-Z0-9_-]*$/.test(value)) {
      return $t("setup.usernameInvalid");
    }
    return "";
  }

  function validatePassword(value: string): string {
    if (!value) return $t("setup.passwordRequired");
    if (value.length < 8) return $t("setup.passwordMinLength");
    return "";
  }

  function validateConfirm(value: string): string {
    if (!value) return $t("setup.confirmPasswordRequired");
    if (value !== formData.password) return $t("setup.passwordMismatch");
    return "";
  }

  function validateCustomPort(value: string): string {
    if (!value) return $t("setup.portInvalid");
    const num = parseInt(value, 10);
    if (isNaN(num) || num < 1024 || num > 65535) return $t("setup.portInvalid");
    return "";
  }

  async function handleSubmit() {
    // 验证表单
    usernameError = validateUsername(formData.username);
    passwordError = validatePassword(formData.password);
    confirmError = validateConfirm(password_confirm);

    if (portMode === "custom") {
      customPortError = validateCustomPort(customPort);
      if (customPortError) return;
    }

    if (usernameError || passwordError || confirmError) {
      return;
    }

    loading = true;
    error = "";

    try {
      const result = await setupApi.createAdmin(formData);

      // 配置安全选项（端口）
      if (portMode === "random") {
        try {
          const secResult = await setupApi.configureSecurity({ use_random_port: true });
          if (secResult.port_changed && secResult.new_port) {
            newPort = secResult.new_port;
          }
        } catch {
          // 安全配置失败不阻止流程
        }
      } else if (portMode === "custom") {
        try {
          const port = parseInt(customPort, 10);
          const secResult = await setupApi.configureSecurity({
            use_random_port: false,
            custom_port: port,
          });
          if (secResult.port_changed && secResult.new_port) {
            newPort = secResult.new_port;
          }
        } catch {
          // 安全配置失败不阻止流程
        }
      }

      if (formData.enable_2fa && result.two_factor) {
        // 显示 2FA 设置界面
        twoFactorResult = {
          totp_secret: result.two_factor.secret,
          totp_qr_code: result.two_factor.qr_code_url || "",
          recovery_codes: result.two_factor.backup_codes || [],
        };
        showTwoFactorSetup = true;
      } else {
        // 直接进入下一步
        goto("/setup/step3");
      }
    } catch (e) {
      // 管理员已存在（HTTP 409）：直接进入下一步
      if ((e as any)?.status === 409) {
        goto("/setup/step3");
        return;
      }
      const message = e instanceof Error ? e.message : $t("setup.createUserFailed");
      error = message;
    } finally {
      loading = false;
    }
  }

  async function verifyTotpAndContinue() {
    if (totpCode.length !== 6) {
      error = $t("setup.totpCodeRequired");
      return;
    }

    if (!copiedCodes) {
      error = $t("setup.saveRecoveryFirst");
      return;
    }

    // 验证 TOTP 码
    loading = true;
    error = "";

    try {
      // 这里可以添加验证 TOTP 的 API 调用
      // 暂时直接进入下一步
      goto("/setup/step3");
    } catch (e) {
      error = e instanceof Error ? e.message : $t("setup.verifyFailed");
    } finally {
      loading = false;
    }
  }

  function copyRecoveryCodes() {
    if (twoFactorResult?.recovery_codes) {
      const codes = twoFactorResult.recovery_codes.join("\n");
      navigator.clipboard.writeText(codes);
      copiedCodes = true;
    }
  }

  function goBack() {
    if (showTwoFactorSetup) {
      showTwoFactorSetup = false;
    } else {
      goto("/setup/step1");
    }
  }
</script>

<SetupCard
  header={showTwoFactorSetup
    ? { icon: "🛡️", title: $t("setup.twoFactorSetupTitle"), description: $t("setup.twoFactorSetupDesc") }
    : {
        icon: "👤",
        title: $t("setup.createAdminTitle"),
        description: $t("setup.createAdminDesc"),
      }}
  {error}
>
  {#if !showTwoFactorSetup}
    <form
      onsubmit={(e) => {
        e.preventDefault();
        handleSubmit();
      }}
    >
      <FormField icon="📛" label={$t("setup.usernameLabel")} labelFor="username" hint={$t("setup.usernameHint")}>
        <Input
          type="text"
          id="username"
          placeholder={$t("setup.usernamePlaceholder")}
          error={usernameError}
          bind:value={formData.username}
          oninput={() => (usernameError = "")}
        />
      </FormField>

      <FormField icon="🔐" label={$t("setup.passwordLabel")} labelFor="password">
        <Input
          id="password"
          type="password"
          error={passwordError}
          placeholder={$t("setup.passwordPlaceholder")}
          bind:value={formData.password}
          oninput={() => (passwordError = "")}
        />
        {#if formData.password}
          <div class="flex items-center gap-2">
            <div class="strength-bar">
              <div
                class="strength-fill"
                style:width="{passwordStrength.score}%"
                style:background={passwordStrength.color}
              ></div>
            </div>
            <span class="text-sm" style:color={passwordStrength.color}>
              {$t(passwordStrength.labelKey)}
            </span>
          </div>
        {/if}
      </FormField>

      <FormField icon="🔑" label={$t("setup.confirmPasswordLabel")} labelFor="password-confirm">
        <Input
          id="password-confirm"
          type="password"
          error={confirmError}
          placeholder={$t("setup.confirmPasswordPlaceholder")}
          bind:value={password_confirm}
          oninput={() => (confirmError = "")}
        />
      </FormField>

      <fieldset class="toggle-card">
        <div class="toggle-card-content">
          <span class="toggle-card-icon">🛡️</span>
          <div class="toggle-card-text">
            <div class="toggle-card-label">{$t("setup.enable2FA")}</div>
            <div class="toggle-card-desc">{$t("setup.enable2FADesc")}</div>
          </div>
        </div>
        <Switch id="enable-2fa" bind:checked={formData.enable_2fa} />
      </fieldset>

      <!-- 端口配置 -->
      <fieldset class="port-section">
        <div class="port-section-header">
          <span class="toggle-card-icon">🔀</span>
          <div class="toggle-card-text">
            <div class="toggle-card-label">{$t("setup.portSettingTitle")}</div>
            <div class="toggle-card-desc">{$t("setup.portSettingDesc")}</div>
          </div>
        </div>
        <div class="port-options">
          <label class="port-option" class:port-option--active={portMode === "default"}>
            <input type="radio" name="port-mode" value="default" bind:group={portMode} />
            <div class="port-option-content">
              <span class="port-option-label">{$t("setup.portDefault")}</span>
              <span class="port-option-desc">{$t("setup.portDefaultDesc")}</span>
            </div>
          </label>
          <label class="port-option" class:port-option--active={portMode === "random"}>
            <input type="radio" name="port-mode" value="random" bind:group={portMode} />
            <div class="port-option-content">
              <span class="port-option-label">{$t("setup.portRandom")}</span>
              <span class="port-option-desc">{$t("setup.portRandomDesc")}</span>
            </div>
          </label>
          <label class="port-option" class:port-option--active={portMode === "custom"}>
            <input type="radio" name="port-mode" value="custom" bind:group={portMode} />
            <div class="port-option-content">
              <span class="port-option-label">{$t("setup.portCustom")}</span>
              <span class="port-option-desc">{$t("setup.portCustomDesc")}</span>
            </div>
          </label>
        </div>
        {#if portMode === "custom"}
          <div class="port-custom-input">
            <Input
              type="text"
              placeholder={$t("setup.portCustomPlaceholder")}
              error={customPortError}
              bind:value={customPort}
              oninput={() => (customPortError = "")}
            />
          </div>
        {/if}
      </fieldset>

      {#if newPort}
        <div class="port-notice">
          <span class="text-lg">⚠️</span>
          <div>
            <div class="font-medium">{$t("setup.portChanged", { values: { port: newPort } })}</div>
            <div class="text-secondary text-sm">{$t("setup.portChangedHint", { values: { port: newPort } })}</div>
          </div>
        </div>
      {/if}
    </form>
  {:else}
    <form>
      <!-- 二维码 -->
      {#if twoFactorResult?.totp_qr_code}
        <div class="section qr-section">
          <img src={twoFactorResult.totp_qr_code} alt="TOTP QR Code" class="qr-code" />
          <p class="text-secondary text-sm">
            {$t("setup.scanQRCode")}
          </p>
        </div>
      {/if}

      <!-- 手动输入密钥 -->
      <div class="section text-center">
        <p class="mb-4 text-sm">{$t("setup.manualEntry")}</p>
        <code class="secret-code">{twoFactorResult?.totp_secret}</code>
      </div>

      <!-- 恢复码 -->
      <div class="section recovery-section flex flex-col gap-2">
        <h3 class="text-lg">🔑 {$t("setup.recoveryCodesTitle")}</h3>
        <p class="text-secondary text-sm">{$t("setup.recoveryCodesDesc")}</p>
        <div class="grid grid-cols-2 gap-2">
          {#each twoFactorResult?.recovery_codes ?? [] as code}
            <code class="secret-code">{code}</code>
          {/each}
        </div>
        <button class="self-center" onclick={copyRecoveryCodes}>
          {copiedCodes ? "✅ " + $t("setup.copied") : "📋 " + $t("setup.copyRecoveryCodes")}
        </button>
      </div>

      <FormField label={$t("setup.totpCodeLabel")} labelFor="totp-code" hint={$t("setup.totpCodeHint")}>
        <Input
          id="totp-code"
          type="mono-code"
          placeholder="000000"
          maxlength={6}
          bind:value={totpCode}
        />
      </FormField>
    </form>
  {/if}

  {#snippet footer()}
    <Button variant="secondary" onclick={goBack}>{$t("setup.prevStep")}</Button>

    {#if !showTwoFactorSetup}
      <Button variant="primary" onclick={handleSubmit} {loading}>
        {#if loading}
          {$t("setup.creating")}
        {:else}
          {formData.enable_2fa ? $t("setup.setup2FA") : $t("setup.nextStep")}
        {/if}
      </Button>
    {:else}
      <Button
        variant="primary"
        onclick={verifyTotpAndContinue}
        disabled={totpCode.length !== 6 || !copiedCodes}
        {loading}
      >
        {#if loading}
          {$t("setup.verifying")}
        {:else}
          {$t("setup.completeSetup")}
        {/if}
      </Button>
    {/if}
  {/snippet}
</SetupCard>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: calc(4 * var(--spacing));
  }

  .strength-bar {
    flex: 1;
    height: 4px;
    background: #e2e8f0;
    border-radius: 2px;
    overflow: hidden;
  }

  .strength-fill {
    height: 100%;
    border-radius: 2px;
    transition: all 0.3s ease;
  }

  /* Toggle 开关卡片 */
  .toggle-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 16px;
    border: 1px solid rgba(0, 0, 0, 0.1);
    border-radius: 12px;
    background: rgba(255, 255, 255, 0.5);
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .toggle-card:hover {
    border-color: rgba(59, 130, 246, 0.3);
    background: rgba(255, 255, 255, 0.8);
  }

  .toggle-card-content {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex: 1;
    min-width: 0;
  }

  .toggle-card-icon {
    font-size: 1.75rem;
    flex-shrink: 0;
    line-height: 1;
  }

  .toggle-card-text {
    display: flex;
    flex-direction: column;
    gap: 0.125rem;
  }

  .toggle-card-label {
    font-size: 1rem;
    font-weight: 500;
    line-height: 1.4;
  }

  .toggle-card-desc {
    font-size: var(--text-sm);
    color: var(--color-text-secondary);
    line-height: 1.4;
  }

  /* 端口配置 */
  .port-section {
    border: 1px solid rgba(0, 0, 0, 0.1);
    border-radius: 12px;
    background: rgba(255, 255, 255, 0.5);
    overflow: hidden;
    transition: border-color 0.2s;
  }

  .port-section:hover {
    border-color: rgba(59, 130, 246, 0.3);
  }

  .port-section-header {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 16px;
  }

  .port-options {
    display: flex;
    gap: 0.5rem;
    padding: 0 16px 16px;
  }

  .port-option {
    flex: 1;
    position: relative;
    cursor: pointer;
    border: 1.5px solid rgba(0, 0, 0, 0.08);
    border-radius: 8px;
    padding: 10px 12px;
    transition: all 0.15s ease;
    background: rgba(255, 255, 255, 0.6);
  }

  .port-option:hover {
    border-color: rgba(59, 130, 246, 0.3);
  }

  .port-option--active {
    border-color: var(--color-primary);
    background: rgba(59, 130, 246, 0.04);
    box-shadow: 0 0 0 1px rgba(59, 130, 246, 0.1);
  }

  .port-option input[type="radio"] {
    position: absolute;
    opacity: 0;
    width: 0;
    height: 0;
  }

  .port-option-content {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .port-option-label {
    font-size: var(--text-sm);
    font-weight: 500;
  }

  .port-option--active .port-option-label {
    color: var(--color-primary);
  }

  .port-option-desc {
    font-size: var(--text-xs);
    color: var(--color-text-tertiary);
    line-height: 1.3;
  }

  .port-custom-input {
    padding: 0 16px 16px;
    animation: fadeIn 0.15s ease-out;
  }

  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(-4px); }
    to { opacity: 1; transform: translateY(0); }
  }

  .section {
    border-radius: var(--radius-md);
    padding: calc(4 * var(--spacing));
    background: rgba(0, 0, 0, 0.03);
  }

  /* 2FA 设置 */
  .qr-section {
    text-align: center;
    background: white;
  }

  .qr-code {
    width: 180px;
    height: 180px;
    border-radius: 8px;
  }

  .secret-code {
    font-family: var(--font-mono);
    text-align: center;
    background: rgba(0, 0, 0, 0.05);
    padding: calc(2 * var(--spacing)) calc(4 * var(--spacing));
    border-radius: var(--radius-md);
    letter-spacing: 0.1rem;
  }

  .recovery-section {
    border: 1px solid var(--color-warning);
  }

  :global(:root[data-theme="dark"]) {
    /* 密码强度 */
    .strength-bar {
      background: rgba(255, 255, 255, 0.1);
    }

    /* Toggle 开关 */
    .toggle-card {
      border: 1px solid rgba(255, 255, 255, 0.1);
      background: rgba(255, 255, 255, 0.05);
    }

    .toggle-card:hover {
      border-color: rgba(59, 130, 246, 0.3);
      background: rgba(255, 255, 255, 0.1);
    }

    /* 端口配置 */
    .port-section {
      border-color: rgba(255, 255, 255, 0.1);
      background: rgba(255, 255, 255, 0.05);
    }

    .port-section:hover {
      border-color: rgba(59, 130, 246, 0.3);
    }

    .port-option {
      border-color: rgba(255, 255, 255, 0.1);
      background: rgba(255, 255, 255, 0.03);
    }

    .port-option:hover {
      border-color: rgba(59, 130, 246, 0.3);
    }

    .port-option--active {
      border-color: var(--color-primary);
      background: rgba(59, 130, 246, 0.08);
    }

    .port-notice {
      display: flex;
      align-items: flex-start;
      gap: 12px;
      padding: 12px 16px;
      border: 1px solid var(--color-warning);
      border-radius: 12px;
      background: rgba(245, 158, 11, 0.08);
    }
    .section {
      background: rgba(0, 0, 0, 0.1);
    }

    .secret-code {
      background: rgba(255, 255, 255, 0.08);
    }
  }
</style>
