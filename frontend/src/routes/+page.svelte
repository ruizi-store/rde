<script lang="ts">
  import { onMount } from "svelte";
  import { browser } from "$app/environment";
  import { goto } from "$app/navigation";
  import Desktop from "$desktop/components/Desktop.svelte";
  import { registerApps, initApps } from "$apps";
  import { setupApi } from "$shared/services/setup";
  import { getValidToken } from "$shared/utils/auth";
  import { reloadAllStoresForUser } from "$shared/utils/user-storage";
  import { userStore } from "$shared/stores/user.svelte";
  import { api } from "$shared/services/api";

  let ready = $state(false);
  let checkingSetup = $state(true);

  // 先同步注册应用（使用默认模块状态）
  if (browser) {
    registerApps();
  }

  // 然后异步加载真实的模块状态并更新
  onMount(async () => {
    // 优先检查 token（零网络请求）
    const token = getValidToken();

    // 没有 token：检查 setup 状态后决定跳转
    if (!token) {
      try {
        const status = await setupApi.getStatus();
        if (!status.completed) {
          goto("/setup");
        } else {
          goto("/login");
        }
      } catch (e) {
        console.error("Failed to get setup status:", e);
        goto("/login");
      }
      return;
    }

    // 有 token：先验证 token 是否被后端认可
    try {
      // 使用一个轻量级 API 验证 token
      const status = await setupApi.getStatus();
      if (!status.completed) {
        goto("/setup");
        return;
      }

      // 尝试获取用户信息验证 token
      await api.get("/users/current");
    } catch (e) {
      // token 验证失败，清除并跳转登录
      console.error("Token validation failed:", e);
      api.setToken(null);
      localStorage.removeItem("refresh_token");
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
  });

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
