<script lang="ts">
  import { t } from "svelte-i18n";
  import { goto } from "$app/navigation";
  import { authService } from "$shared/services/auth";
  import { user } from "$shared/stores/user.svelte";
  import Icon from "@iconify/svelte";

  let username = $state("");
  let password = $state("");
  let loading = $state(false);
  let error = $state("");
  let showPassword = $state(false);

  // 2FA 状态
  let show2FA = $state(false);
  let tempToken = $state("");
  let totpCode = $state("");
  let rememberDevice = $state(true); // 默认勾选记住此设备

  function completeLogin(data: NonNullable<Awaited<ReturnType<typeof authService.login>>["data"]>) {
    if (data.access_token) {
      localStorage.setItem("auth_token", data.access_token);
    }
    const userData = data.user;
    if (userData) {
      user.login(
        {
          id: userData.id,
          username: userData.username,
          nickname: userData.nickname,
          role: userData.role as "admin" | "user",
        },
        data.access_token,
      );
    }
    goto("/");
  }

  async function handleLogin(e: Event) {
    e.preventDefault();

    if (!username.trim() || !password) {
      error = $t("auth.enterUsernamePassword");
      return;
    }

    loading = true;
    error = "";

    try {
      const response = await authService.login({ username: username.trim(), password });

      if (response.success && response.data) {
        // 需要 2FA 验证
        if (response.data.require_2fa && response.data.temp_token) {
          tempToken = response.data.temp_token;
          show2FA = true;
          totpCode = "";
          loading = false;
          return;
        }

        completeLogin(response.data);
      } else {
        error = response.message || $t("auth.loginFailed");
      }
    } catch (err) {
      error = err instanceof Error ? err.message : $t("auth.loginFailedCheck");
    } finally {
      loading = false;
    }
  }

  async function handle2FA(e: Event) {
    e.preventDefault();
    if (!totpCode.trim()) {
      error = $t("auth.enterTotpCode");
      return;
    }

    loading = true;
    error = "";

    try {
      const response = await authService.verify2FA(totpCode.trim(), tempToken, rememberDevice);
      if (response.success && response.data) {
        completeLogin(response.data);
      } else {
        error = response.message || $t("auth.totpError");
      }
    } catch (err) {
      error = err instanceof Error ? err.message : $t("auth.verifyFailed");
    } finally {
      loading = false;
    }
  }

  function cancel2FA() {
    show2FA = false;
    tempToken = "";
    totpCode = "";
    error = "";
  }
</script>

<svelte:head>
  <title>{$t("auth.loginTitle")}</title>
</svelte:head>

<div class="login-page">
  <div class="login-container">
    <!-- Logo -->
    <div class="logo">
      <div class="logo-icon">
        <Icon icon="mdi:server" width="48" height="48" />
      </div>
      <h1>RDE</h1>
      <p class="tagline">{$t("auth.tagline")}</p>
    </div>

    <!-- 登录表单 / 2FA 表单 -->
    {#if show2FA}
      <form class="login-form" onsubmit={handle2FA}>
        <div class="two-fa-header">
          <Icon icon="mdi:shield-key" width="32" />
          <h2>{$t("auth.twoFactorAuth")}</h2>
          <p>{$t("auth.enterAuthCode")}</p>
        </div>

        {#if error}
          <div class="error-message">
            <Icon icon="mdi:alert-circle" />
            <span>{error}</span>
          </div>
        {/if}

        <div class="form-group">
          <label for="totp-code">
            <Icon icon="mdi:numeric" />
            <span>{$t("auth.verificationCode")}</span>
          </label>
          <input
            type="text"
            id="totp-code"
            bind:value={totpCode}
            placeholder="000000"
            autocomplete="one-time-code"
            inputmode="numeric"
            maxlength="8"
            disabled={loading}
            style="text-align: center; letter-spacing: 0.5em; font-size: 1.4em; font-variant-numeric: tabular-nums;"
          />
        </div>

        <div class="remember-device">
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={rememberDevice} disabled={loading} />
            <span>{$t("auth.rememberDevice")}</span>
          </label>
          <p class="hint">{$t("auth.rememberDeviceHint")}</p>
        </div>

        <button type="submit" class="login-btn" disabled={loading || !totpCode.trim()}>
          {#if loading}
            <Icon icon="mdi:loading" class="spin" />
            <span>{$t("auth.verifying")}</span>
          {:else}
            <Icon icon="mdi:check-circle" />
            <span>{$t("auth.verify")}</span>
          {/if}
        </button>

        <button type="button" class="back-btn" onclick={cancel2FA} disabled={loading}>
          <Icon icon="mdi:arrow-left" />
          <span>{$t("auth.backToLogin")}</span>
        </button>
      </form>
    {:else}
      <form class="login-form" onsubmit={handleLogin}>
        {#if error}
          <div class="error-message">
            <Icon icon="mdi:alert-circle" />
            <span>{error}</span>
          </div>
        {/if}

      <div class="form-group">
        <label for="username">
          <Icon icon="mdi:account" />
          <span>{$t("auth.username")}</span>
        </label>
        <input
          type="text"
          id="username"
          bind:value={username}
          placeholder={$t("auth.usernamePlaceholder")}
          autocomplete="username"
          disabled={loading}
        />
      </div>

      <div class="form-group">
        <label for="password">
          <Icon icon="mdi:lock" />
          <span>{$t("auth.password")}</span>
        </label>
        <div class="password-input">
          <input
            type={showPassword ? "text" : "password"}
            id="password"
            bind:value={password}
            placeholder={$t("auth.passwordPlaceholder")}
            autocomplete="current-password"
            disabled={loading}
          />
          <button
            type="button"
            class="toggle-password"
            onclick={() => (showPassword = !showPassword)}
            tabindex="-1"
          >
            <Icon icon={showPassword ? "mdi:eye-off" : "mdi:eye"} />
          </button>
        </div>
      </div>

      <button type="submit" class="login-btn" disabled={loading}>
        {#if loading}
          <Icon icon="mdi:loading" class="spin" />
          <span>{$t("auth.loggingIn")}</span>
        {:else}
          <Icon icon="mdi:login" />
          <span>{$t("auth.login")}</span>
        {/if}
      </button>
    </form>
    {/if}

    <!-- 版权信息 -->
    <footer class="login-footer">
      <p>&copy; 2026 RDE. All rights reserved.</p>
    </footer>
  </div>

  <!-- 背景装饰 -->
  <div class="bg-decoration">
    <div class="circle circle-1"></div>
    <div class="circle circle-2"></div>
    <div class="circle circle-3"></div>
  </div>
</div>

<style>
  .login-page {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
    position: relative;
    overflow: hidden;
  }

  .login-container {
    width: 100%;
    max-width: 400px;
    padding: 40px;
    background: rgba(255, 255, 255, 0.05);
    backdrop-filter: blur(10px);
    border-radius: 16px;
    border: 1px solid rgba(255, 255, 255, 0.1);
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
    z-index: 1;
  }

  .logo {
    text-align: center;
    margin-bottom: 32px;

    .logo-icon {
      width: 80px;
      height: 80px;
      background: linear-gradient(135deg, #0066cc, #004499);
      border-radius: 20px;
      display: flex;
      align-items: center;
      justify-content: center;
      margin: 0 auto 16px;
      color: white;
    }

    h1 {
      font-size: 28px;
      font-weight: 700;
      color: white;
      margin: 0 0 8px;
    }

    .tagline {
      font-size: 14px;
      color: rgba(255, 255, 255, 0.6);
      margin: 0;
    }
  }

  .login-form {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .error-message {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    background: rgba(220, 53, 69, 0.2);
    border: 1px solid rgba(220, 53, 69, 0.4);
    border-radius: 8px;
    color: #ff6b6b;
    font-size: 14px;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 8px;

    label {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 14px;
      font-weight: 500;
      color: rgba(255, 255, 255, 0.8);
    }

    input {
      width: 100%;
      padding: 12px 16px;
      font-size: 15px;
      color: white;
      background: rgba(255, 255, 255, 0.1);
      border: 1px solid rgba(255, 255, 255, 0.2);
      border-radius: 8px;
      outline: none;
      transition:
        border-color 0.2s,
        background 0.2s;

      &::placeholder {
        color: rgba(255, 255, 255, 0.4);
      }

      &:focus {
        border-color: #0066cc;
        background: rgba(255, 255, 255, 0.15);
      }

      &:disabled {
        opacity: 0.6;
        cursor: not-allowed;
      }
    }
  }

  .password-input {
    position: relative;

    input {
      padding-right: 44px;
    }

    .toggle-password {
      position: absolute;
      right: 12px;
      top: 50%;
      transform: translateY(-50%);
      background: none;
      border: none;
      color: rgba(255, 255, 255, 0.5);
      cursor: pointer;
      padding: 4px;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: color 0.2s;

      &:hover {
        color: rgba(255, 255, 255, 0.8);
      }
    }
  }

  .login-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    width: 100%;
    padding: 14px;
    font-size: 16px;
    font-weight: 600;
    color: white;
    background: linear-gradient(135deg, #0066cc, #004499);
    border: none;
    border-radius: 8px;
    cursor: pointer;
    transition:
      opacity 0.2s,
      transform 0.1s;

    &:hover:not(:disabled) {
      opacity: 0.9;
    }

    &:active:not(:disabled) {
      transform: scale(0.98);
    }

    &:disabled {
      opacity: 0.7;
      cursor: not-allowed;
    }

    :global(.spin) {
      animation: spin 1s linear infinite;
    }
  }

  @keyframes spin {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }

  .login-footer {
    margin-top: 32px;
    text-align: center;

    p {
      font-size: 12px;
      color: rgba(255, 255, 255, 0.4);
      margin: 0;
    }
  }

  .two-fa-header {
    text-align: center;
    margin-bottom: 16px;
    color: rgba(255, 255, 255, 0.9);

    h2 {
      margin: 8px 0 4px;
      font-size: 20px;
      font-weight: 600;
    }

    p {
      font-size: 14px;
      color: rgba(255, 255, 255, 0.6);
      margin: 0;
    }
  }

  .back-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    width: 100%;
    padding: 12px;
    margin-top: 8px;
    font-size: 14px;
    color: rgba(255, 255, 255, 0.7);
    background: transparent;
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: 8px;
    cursor: pointer;
    transition: background 0.2s;

    &:hover:not(:disabled) {
      background: rgba(255, 255, 255, 0.1);
    }

    &:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }
  }

  .remember-device {
    display: flex;
    flex-direction: column;
    gap: 4px;

    .checkbox-label {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 14px;
      color: rgba(255, 255, 255, 0.8);
      cursor: pointer;

      input[type="checkbox"] {
        width: 16px;
        height: 16px;
        accent-color: #0066cc;
        cursor: pointer;
      }
    }

    .hint {
      font-size: 12px;
      color: rgba(255, 255, 255, 0.5);
      margin: 0;
      padding-left: 24px;
    }
  }

  /* 背景装饰 */
  .bg-decoration {
    position: absolute;
    inset: 0;
    overflow: hidden;
    pointer-events: none;

    .circle {
      position: absolute;
      border-radius: 50%;
      background: linear-gradient(135deg, rgba(0, 102, 204, 0.3), rgba(0, 68, 153, 0.1));
      filter: blur(40px);
    }

    .circle-1 {
      width: 400px;
      height: 400px;
      top: -100px;
      right: -100px;
    }

    .circle-2 {
      width: 300px;
      height: 300px;
      bottom: -50px;
      left: -50px;
    }

    .circle-3 {
      width: 200px;
      height: 200px;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%);
      opacity: 0.5;
    }
  }

  /* 响应式 */
  @media (max-width: 480px) {
    .login-container {
      margin: 16px;
      padding: 24px;
    }

    .logo {
      .logo-icon {
        width: 64px;
        height: 64px;
      }

      h1 {
        font-size: 24px;
      }
    }
  }
</style>
