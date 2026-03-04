<!--
  AppShell — 插件 SPA 的顶层布局容器
  
  提供：
  - 顶部导航栏（应用标题 + 可选操作区 + 返回桌面按钮）
  - 全屏响应式内容区域
  - 统一的暗色主题背景
  - 自带 ToastContainer
-->
<script lang="ts">
  import type { Snippet } from "svelte";
  import ToastContainer from "./ToastContainer.svelte";

  interface Props {
    /** 应用标题 */
    title: string;
    /** 应用图标（emoji 或 URL） */
    icon?: string;
    /** 是否显示返回桌面按钮 */
    showBack?: boolean;
    /** 导航栏右侧操作区 */
    actions?: Snippet;
    /** 主内容区域 */
    children: Snippet;
  }

  let {
    title,
    icon = "",
    showBack = true,
    actions,
    children,
  }: Props = $props();

  function goBack() {
    // 如果有历史记录则返回，否则尝试关闭标签页
    if (window.history.length > 1) {
      window.history.back();
    } else {
      window.close();
      // 如果无法关闭（非 window.open 打开），跳转到主页
      setTimeout(() => {
        window.location.href = "/";
      }, 100);
    }
  }
</script>

<div class="app-shell">
  <header class="app-navbar">
    <div class="navbar-left">
      {#if showBack}
        <button class="back-btn" onclick={goBack} title="返回桌面">
          ← 
        </button>
      {/if}
      {#if icon}
        <span class="app-icon">{icon}</span>
      {/if}
      <h1 class="app-title">{title}</h1>
    </div>
    {#if actions}
      <div class="navbar-actions">
        {@render actions()}
      </div>
    {/if}
  </header>

  <main class="app-main">
    {@render children()}
  </main>

  <ToastContainer />
</div>

<style>
  .app-shell {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100vw;
    background: #1a1a2e;
    color: #e0e0e0;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", sans-serif;
    overflow: hidden;
  }

  .app-navbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 48px;
    padding: 0 16px;
    background: #16162a;
    border-bottom: 1px solid #2a2a4a;
    flex-shrink: 0;
    -webkit-app-region: drag; /* 可拖拽（PWA 模式下） */
  }

  .navbar-left {
    display: flex;
    align-items: center;
    gap: 10px;
    -webkit-app-region: no-drag;
  }

  .back-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: #a0a0c0;
    font-size: 16px;
    cursor: pointer;
    transition: all 0.15s;
  }

  .back-btn:hover {
    background: #2a2a4a;
    color: #e0e0e0;
  }

  .app-icon {
    font-size: 20px;
    line-height: 1;
  }

  .app-title {
    font-size: 15px;
    font-weight: 600;
    color: #cdd6f4;
    margin: 0;
    white-space: nowrap;
  }

  .navbar-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    -webkit-app-region: no-drag;
  }

  .app-main {
    flex: 1;
    overflow: hidden;
    position: relative;
  }
</style>
