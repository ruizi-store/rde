<script lang="ts">
  import { onMount } from "svelte";
  import { browser } from "$app/environment";
  import { goto } from "$app/navigation";
  import Desktop from "$desktop/components/Desktop.svelte";
  import { registerApps, initApps } from "$apps";
  import { setupApi } from "$shared/services/setup";
  import { clearLocalAuth, getValidToken } from "$shared/utils/auth";
  import { reloadAllStoresForUser } from "$desktop/utils/reload-stores";
  import { userStore } from "$shared/stores/user.svelte";
  import { api } from "$shared/services/api";
  import { windows } from "$desktop/stores/windows.svelte";

  let ready = $state(false);
  let checkingSetup = $state(true);

  // 先同步注册应用（使用默认模块状态）
  if (browser) {
    registerApps();
  }

  // IM 审批卡片深链：/?approval=<id> → 打开 Bot 工作台并跳到审批记录
  let approvalDeepLink: string | null = null;
  if (browser) {
    const aid = new URLSearchParams(window.location.search).get("approval");
    if (aid) {
      approvalDeepLink = aid;
      // 清掉 query 参数，避免刷新后重复打开
      window.history.replaceState({}, "", window.location.pathname);
    }
  }

  // 然后异步加载真实的模块状态并更新
  onMount(async () => {
    let authMode = "local";
    try {
      const status = await setupApi.getStatus();
      authMode = status.auth_mode || "local";

      // 免登录模式（RDE_AUTH_MODE=none，容器默认）：无需 token，直接进桌面
      if (authMode === "none") {
        enterNoAuthDesktop();
        return;
      }

      if (!status.completed) {
        goto("/setup");
        return;
      }
    } catch (e) {
      console.error("Failed to get setup status:", e);
      // 探测失败：有 token 走验证流程，没有则去登录页
    }

    // 没有 token：跳转登录
    const token = getValidToken();
    if (!token) {
      goto("/login");
      return;
    }

    // 有 token：验证 token 是否被后端认可
    try {
      await api.get("/users/current");
    } catch (e) {
      // token 验证失败，清除并跳转登录
      console.error("Token validation failed:", e);
      api.setToken(null);
      clearLocalAuth();
      goto("/login");
      return;
    }

    // token 验证成功，继续初始化
    checkingSetup = false;

    // 从 JWT 恢复用户信息到 userStore（同步，无网络请求）
    restoreUserFromToken(token);

    // 为当前用户重新加载个性化设置（主题、桌面、偏好等）
    reloadAllStoresForUser();

    // 异步加载完整用户信息（含头像等 JWT 中没有的字段）
    loadFullUserInfo();

    // token 有效，加载应用
    await initApps();
    ready = true;
    openApprovalDeepLink();
  });

  /** 打开审批深链窗口（桌面就绪后调用一次） */
  function openApprovalDeepLink() {
    if (!approvalDeepLink) return;
    const aid = approvalDeepLink;
    approvalDeepLink = null;
    windows.open("bot", { approval: aid }).catch(() => {});
  }

  /** 免登录模式直接进入桌面（后端注入固定管理员身份，无需 token） */
  async function enterNoAuthDesktop() {
    userStore.setUser({ id: "local-admin", username: "admin", role: "admin" });
    // 伪 token 让 isLoggedIn 等派生状态保持可用（请求头为空，后端不校验）
    userStore.token = "no-auth";
    checkingSetup = false;
    reloadAllStoresForUser();
    await initApps();
    ready = true;
    openApprovalDeepLink();
  }

  /** 从 JWT payload 恢复基本用户信息 */
  function restoreUserFromToken(token: string) {
    try {
      const parts = token.split(".");
      if (parts.length !== 3) return;
      const payload = JSON.parse(atob(parts[1].replace(/-/g, "+").replace(/_/g, "/")));
      const userId = payload.user_id || payload.sub;
      const username = payload.username;
      const role = payload.role || "user";
      if (userId && username) {
        userStore.setUser({ id: userId, username, role: role as "admin" | "user" });
        userStore.token = token;
      }
    } catch {
      // JWT 解码失败不影响后续流程
    }
  }

  /** 异步加载完整用户信息（含头像等） */
  async function loadFullUserInfo() {
    try {
      const resp = await api.get<{ data: { id: string; username: string; role: string; avatar?: string } }>("/users/current");
      if (resp.data) {
        userStore.updateUser({
          id: resp.data.id,
          username: resp.data.username,
          role: resp.data.role as "admin" | "user",
          avatar: resp.data.avatar,
        });
      }
    } catch {
      // 静默失败，基本信息已从 JWT 恢复
    }
  }
</script>

<svelte:head>
  <title>RDE</title>
</svelte:head>

{#if checkingSetup}
  <div class="loading-container">
    <div class="spinner"></div>
  </div>
{:else}
  <Desktop />
{/if}

<style>
  .loading-container {
    display: flex;
    justify-content: center;
    align-items: center;
    height: 100vh;
    background: #1a1a2e;
  }
  .spinner {
    width: 40px;
    height: 40px;
    border: 3px solid rgba(255, 255, 255, 0.1);
    border-top-color: #4a90d9;
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
