<script lang="ts">
  import { t } from "svelte-i18n";
  import { page } from "$app/stores";
  import { goto } from "$app/navigation";
  import { onMount } from "svelte";
  import { setupApi, type SetupStatus } from "$shared/services/setup";

  import { Spinner, Button } from "$shared/ui";

  let { children } = $props();

  // 步骤配置 (简化为 3 步)
  const steps = $derived([
    { path: "step1", title: $t("setup.step1Title"), icon: "🌐", description: $t("setup.step1Desc") },
    { path: "step2", title: $t("setup.step2Title"), icon: "👤", description: $t("setup.step2Desc") },
    { path: "step3", title: $t("setup.step3Title"), icon: "✅", description: $t("setup.step3Desc") },
  ]);

  let setupStatus = $state<SetupStatus | null>(null);
  let loading = $state(true);
  let error = $state("");

  // 计算当前步骤
  let currentStepIndex = $derived(steps.findIndex((s) => $page.url.pathname.includes(s.path)));

  // 检查步骤是否完成
  function isStepCompleted(stepIndex: number): boolean {
    if (!setupStatus) return false;
    return setupStatus.completed_steps.includes(stepIndex + 1);
  }

  // 检查步骤是否可访问（所有前置步骤都必须完成）
  function isStepAccessible(stepIndex: number): boolean {
    if (stepIndex === 0) return true;
    for (let i = 0; i < stepIndex; i++) {
      if (!isStepCompleted(i)) return false;
    }
    return true;
  }

  // 找到第一个未完成的步骤索引
  function getFirstIncompleteStep(): number {
    for (let i = 0; i < steps.length; i++) {
      if (!isStepCompleted(i)) return i;
    }
    return steps.length - 1;
  }

  // 导航到步骤
  function navigateToStep(stepIndex: number) {
    if (isStepAccessible(stepIndex)) {
      goto(`/setup/${steps[stepIndex].path}`);
    }
  }

  onMount(async () => {
    try {
      setupStatus = await setupApi.getStatus();

      // 如果已完成，跳转到主页
      if (setupStatus.completed) {
        goto("/");
        return;
      }

      // 如果在 /setup 根路径，跳转到当前步骤
      if ($page.url.pathname === "/setup" || $page.url.pathname === "/setup/") {
        const stepPath = steps[setupStatus.current_step - 1]?.path || "step1";
        goto(`/setup/${stepPath}`);
      } else {
        // 路由守卫：阻止直接访问未解锁的步骤
        const visitingStepIndex = steps.findIndex((s) => $page.url.pathname.includes(s.path));
        if (visitingStepIndex > 0 && !isStepAccessible(visitingStepIndex)) {
          // 重定向到第一个未完成的步骤
          const correctIndex = getFirstIncompleteStep();
          goto(`/setup/${steps[correctIndex].path}`);
        }
      }
    } catch (e) {
      error = e instanceof Error ? e.message : $t("setup.fetchStatusFailed");
    } finally {
      loading = false;
    }
  });
</script>

<div class="setup-background relative flex min-h-full items-center justify-center max-md:flex-col">
  <!-- 动态光晕背景 -->
  <div class="pointer-events-none absolute inset-0 overflow-hidden">
    <div class="orb orb-1"></div>
    <div class="orb orb-2"></div>
    <div class="orb orb-3"></div>
  </div>

  {#if loading}
    <div class="flex flex-col gap-4">
      <Spinner size="xl" />
      <p>{$t("setup.preparingWizard")}</p>
    </div>
  {:else if error}
    <div class="glass-card p-10 text-center">
      <p class="text-danger mb-4">{error}</p>
      <Button variant="primary" onclick={() => location.reload()}>{$t("setup.retry")}</Button>
    </div>
  {:else}
    <!-- 侧边栏 -->
    <aside class="setup-sidebar flex shrink-0 flex-col self-stretch">
      <div class="mb-10 max-md:mb-4 max-md:flex max-md:items-center max-md:gap-4">
        <div class="flex items-center gap-3">
          <span class="text-3xl">🌟</span>
          <span class="logo-text">RDE</span>
        </div>
        <div class="text-secondary md:mt-2">{$t("setup.subtitle")}</div>
      </div>

      <nav class="flex grow flex-col max-md:flex-row max-md:gap-2 max-md:overflow-x-auto">
        {#each steps as step, i}
          <button
            class="step-item"
            class:active={i === currentStepIndex}
            class:completed={isStepCompleted(i)}
            class:accessible={isStepAccessible(i)}
            disabled={!isStepAccessible(i)}
            onclick={() => navigateToStep(i)}
          >
            <span class="step-indicator">
              {#if isStepCompleted(i)}
                <svg
                  class="check-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="3"
                >
                  <polyline points="20 6 9 17 4 12"></polyline>
                </svg>
              {:else}
                <span class="step-number">{i + 1}</span>
              {/if}
            </span>
            <div class="step-content">
              <span class="step-title">{step.title}</span>
              <span class="step-desc">{step.description}</span>
            </div>
          </button>

          {#if i < steps.length - 1}
            <div class="step-connector max-md:hidden" class:active={isStepCompleted(i)}>
              <div class="connector-fill"></div>
            </div>
          {/if}
        {/each}
      </nav>

      <div class="sidebar-footer max-md:hidden">
        <p class="version">v1.0.0</p>
      </div>
    </aside>

    <!-- 主内容区 -->
    <main class="flex grow items-center justify-center p-10 max-md:p-5">
      <div class="glass-card step-card">
        {@render children()}
      </div>
    </main>
  {/if}
</div>

<style>
  .setup-background {
    background: linear-gradient(
      135deg,
      #e0f2fe 0%,
      #f0f9ff 25%,
      #faf5ff 50%,
      #fdf4ff 75%,
      #fff1f2 100%
    );
  }

  .orb {
    position: absolute;
    border-radius: 50%;
    filter: blur(80px);
    opacity: 0.5;
    animation: float 20s ease-in-out infinite;
  }

  .orb-1 {
    width: 600px;
    height: 600px;
    background: radial-gradient(circle, #3b82f6 0%, transparent 70%);
    top: -200px;
    right: -100px;
  }

  .orb-2 {
    width: 500px;
    height: 500px;
    background: radial-gradient(circle, #8b5cf6 0%, transparent 70%);
    bottom: -150px;
    left: -100px;
    animation-delay: -7s;
  }

  .orb-3 {
    width: 400px;
    height: 400px;
    background: radial-gradient(circle, #06b6d4 0%, transparent 70%);
    top: 50%;
    left: 50%;
    animation-delay: -14s;
  }

  @keyframes float {
    0%,
    100% {
      transform: translate(0, 0) scale(1);
    }
    33% {
      transform: translate(30px, -30px) scale(1.05);
    }
    66% {
      transform: translate(-20px, 20px) scale(0.95);
    }
  }

  /* 侧边栏 */
  .setup-sidebar {
    width: 300px;
    background: rgba(255, 255, 255, 0.5);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    border-right: 1px solid rgba(255, 255, 255, 0.6);
    padding: 32px 24px;
  }

  .logo-text {
    font-size: 24px;
    font-weight: 700;
    background: linear-gradient(135deg, #3b82f6, #8b5cf6);
    background-clip: text;
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .step-item {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 12px 16px;
    border-radius: 12px;
    transition: all 0.2s ease;
    text-align: left;
    width: 100%;
  }

  .step-item:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }

  .step-item.accessible:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.5);
  }

  .step-item.active {
    background: rgba(59, 130, 246, 0.1);
  }

  .step-indicator {
    width: 32px;
    height: 32px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #e2e8f0;
    color: #64748b;
    font-size: 14px;
    font-weight: 600;
    transition: all 0.2s ease;
    flex-shrink: 0;
  }

  .step-item.active .step-indicator {
    background: #3b82f6;
    color: white;
  }

  .step-item.completed .step-indicator {
    background: #10b981;
    color: white;
  }

  .check-icon {
    width: 16px;
    height: 16px;
  }

  .step-content {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .step-title {
    font-size: 15px;
    font-weight: 500;
    color: #1e293b;
  }

  .step-item.active .step-title {
    color: #3b82f6;
  }

  .step-item.completed .step-title {
    color: #10b981;
  }

  .step-desc {
    font-size: 12px;
    color: #94a3b8;
  }

  /* 步骤连接线 */
  .step-connector {
    width: 2px;
    height: 24px;
    background: #e2e8f0;
    margin-left: 39px;
    border-radius: 2px;
    overflow: hidden;
  }

  .step-connector .connector-fill {
    height: 0;
    background: linear-gradient(180deg, #3b82f6, #10b981);
    transition: height 0.5s ease;
  }

  .step-connector.active .connector-fill {
    height: 100%;
  }

  .sidebar-footer {
    margin-top: auto;
    padding-top: 20px;
  }

  .version {
    font-size: 12px;
    color: #94a3b8;
  }

  /* 毛玻璃卡片 */
  .glass-card {
    background: rgba(255, 255, 255, 0.7);
    backdrop-filter: blur(20px);
    -webkit-backdrop-filter: blur(20px);
    border: 1px solid rgba(255, 255, 255, 0.8);
    border-radius: 24px;
    box-shadow:
      0 4px 6px -1px rgba(0, 0, 0, 0.05),
      0 10px 15px -3px rgba(0, 0, 0, 0.05),
      0 20px 25px -5px rgba(0, 0, 0, 0.03),
      inset 0 1px 0 rgba(255, 255, 255, 0.6);
  }

  .step-card {
    width: 100%;
    max-width: 600px;
    padding: 48px;
    animation: step-enter 0.4s cubic-bezier(0.16, 1, 0.3, 1);
  }

  @keyframes step-enter {
    from {
      opacity: 0;
      transform: translateX(20px);
    }
    to {
      opacity: 1;
      transform: translateX(0);
    }
  }

  :global(:root[data-theme="dark"]) {
    /* 背景 */
    .setup-background {
      background: linear-gradient(
        135deg,
        #0c1a2d 0%,
        #111827 25%,
        #1e1b4b 50%,
        #2e1065 75%,
        #3f1d47 100%
      );
    }

    /* 动态光晕 */
    .orb-1 {
      background: radial-gradient(circle, rgba(59, 130, 246, 0.3) 0%, transparent 70%);
    }

    .orb-2 {
      background: radial-gradient(circle, rgba(139, 92, 246, 0.3) 0%, transparent 70%);
    }

    .orb-3 {
      background: radial-gradient(circle, rgba(6, 182, 212, 0.3) 0%, transparent 70%);
    }

    /* 侧边栏 */
    .setup-sidebar {
      background: rgba(15, 23, 42, 0.5);
      border-color: rgba(255, 255, 255, 0.1);
    }

    .logo-text {
      background: linear-gradient(135deg, #60a5fa, #a78bfa);
      background-clip: text;
    }

    /* 步骤导航 */
    .step-item.accessible:hover:not(:disabled) {
      background: rgba(255, 255, 255, 0.05);
    }

    .step-item.active {
      background: rgba(59, 130, 246, 0.15);
    }

    .step-indicator {
      background: rgba(255, 255, 255, 0.1);
      color: #cbd5e1;
    }

    .step-item.active .step-indicator {
      background: #3b82f6;
      color: white;
    }

    .step-item.completed .step-indicator {
      background: #10b981;
      color: white;
    }

    .step-title {
      color: #f1f5f9;
    }

    .step-item.active .step-title {
      color: #60a5fa;
    }

    .step-item.completed .step-title {
      color: #34d399;
    }

    .step-desc {
      color: #94a3b8;
    }

    /* 步骤连接线 */
    .step-connector {
      background: rgba(255, 255, 255, 0.1);
    }

    .step-connector .connector-fill {
      background: linear-gradient(180deg, #60a5fa, #34d399);
    }

    /* 毛玻璃卡片 */
    .glass-card {
      background: rgba(15, 23, 42, 0.7);
      border: 1px solid rgba(255, 255, 255, 0.1);
      box-shadow:
        0 4px 6px -1px rgba(0, 0, 0, 0.3),
        0 10px 15px -3px rgba(0, 0, 0, 0.25),
        0 20px 25px -5px rgba(0, 0, 0, 0.2),
        inset 0 1px 0 rgba(255, 255, 255, 0.1);
    }
  }

  /* 响应式 */
  @media (max-width: 768px) {
    .setup-sidebar {
      width: 100%;
      border-right: none;
      border-bottom: 1px solid rgba(255, 255, 255, 0.6);
      padding: 20px;
    }

    .step-item {
      width: auto;
      flex-shrink: 0;
      padding: 8px 12px;
    }

    .step-card {
      padding: 24px;
    }
  }
</style>
