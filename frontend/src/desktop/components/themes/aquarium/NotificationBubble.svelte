<!-- NotificationBubble.svelte - 通知泡泡组件 -->
<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import Icon from "@iconify/svelte";
  import type { Notification, NotificationSeverity } from "$shared/services/notification";
  import { severityInfo } from "$shared/services/notification";

  interface Props {
    notification: Notification;
    initialX: number;
    initialY: number;
    size?: number;
    autoHideSeconds?: number;
    onPop: (notification: Notification, x: number, y: number) => void;
    onExpire: (notification: Notification) => void;
  }

  let {
    notification,
    initialX,
    initialY,
    size = 90,
    autoHideSeconds = 30,
    onPop,
    onExpire,
  }: Props = $props();

  // 初始位置来自 props，之后由动画控制
  // svelte-ignore state_referenced_locally
  let x = $state(initialX);
  // svelte-ignore state_referenced_locally
  let y = $state(initialY);
  let isHovered = $state(false);
  let isPopping = $state(false);
  let opacity = $state(0);
  let scale = $state(0.3);

  // 动画参数
  let wobblePhase = Math.random() * Math.PI * 2;
  let wobbleSpeed = 0.6 + Math.random() * 0.3;
  let wobbleAmplitude = 12 + Math.random() * 8;
  let floatSpeed = 5 + Math.random() * 3;
  let horizontalDrift = (Math.random() - 0.5) * 0.2;

  let animationId: number;
  let startTime: number;
  let autoHideTimer: number;

  // 根据严重级别获取颜色
  function getSeverityColor(severity: NotificationSeverity): string {
    const colors: Record<NotificationSeverity, string> = {
      info: "#4FC3F7",
      warning: "#FFB74D",
      error: "#EF5350",
      critical: "#F44336",
    };
    return colors[severity] || colors.info;
  }

  // 获取图标
  function getIcon(): string {
    if (notification.icon) return notification.icon;
    return severityInfo[notification.severity]?.icon || "mdi:bell";
  }

  function animate(currentTime: number) {
    if (!startTime) startTime = currentTime;
    const elapsed = (currentTime - startTime) / 1000;

    // 入场动画
    if (elapsed < 0.6) {
      opacity = Math.min(1, elapsed * 1.8);
      scale = 0.3 + 0.7 * Math.min(1, elapsed * 1.8);
    } else {
      opacity = 1;
      scale = 1;
    }

    // 摆动效果
    const wobble = Math.sin(elapsed * wobbleSpeed + wobblePhase) * wobbleAmplitude;
    x = initialX + wobble + horizontalDrift * elapsed * 30;

    // 缓慢上升漂浮
    y = initialY - Math.sin(elapsed * 0.5) * 15 - elapsed * floatSpeed;

    // 边界检查
    if (y < -size) {
      // 飘出屏幕，触发过期
      onExpire(notification);
      return;
    }

    if (!isPopping) {
      animationId = requestAnimationFrame(animate);
    }
  }

  function handleClick(e: MouseEvent) {
    e.stopPropagation();
    if (isPopping) return;

    isPopping = true;
    clearTimeout(autoHideTimer);

    // 通知父组件泡泡被戳破
    onPop(notification, x, y);
  }

  function handleMouseEnter() {
    isHovered = true;
  }

  function handleMouseLeave() {
    isHovered = false;
  }

  onMount(() => {
    animationId = requestAnimationFrame(animate);

    // 自动隐藏计时器
    if (autoHideSeconds > 0) {
      autoHideTimer = setTimeout(() => {
        if (!isPopping) {
          onExpire(notification);
        }
      }, autoHideSeconds * 1000) as unknown as number;
    }
  });

  onDestroy(() => {
    if (animationId) {
      cancelAnimationFrame(animationId);
    }
    clearTimeout(autoHideTimer);
  });

  // svelte-ignore state_referenced_locally
  const severityColor = getSeverityColor(notification.severity);
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
  class="notification-bubble"
  class:hovered={isHovered}
  class:popping={isPopping}
  class:critical={notification.severity === "critical"}
  class:error={notification.severity === "error"}
  style="
    left: {x}px;
    top: {y}px;
    width: {size}px;
    height: {size}px;
    opacity: {opacity};
    transform: translate(-50%, -50%) scale({isHovered ? scale * 1.1 : scale});
    --severity-color: {severityColor};
  "
  onclick={handleClick}
  onmouseenter={handleMouseEnter}
  onmouseleave={handleMouseLeave}
>
  <div class="bubble-inner">
    <div class="bubble-shine"></div>
    <div class="bubble-shine-2"></div>

    <!-- 图标 -->
    <div class="bubble-icon">
      <Icon icon={getIcon()} width="24" />
    </div>

    <!-- 标题 -->
    <span class="bubble-title">{notification.title}</span>

    <!-- 严重级别指示器 -->
    {#if notification.severity === "critical" || notification.severity === "error"}
      <div class="severity-indicator"></div>
    {/if}
  </div>
</div>

<style>
  .notification-bubble {
    position: absolute;
    cursor: pointer;
    transition: transform 0.2s ease-out;
    z-index: 5;
    pointer-events: auto;
  }

  .bubble-inner {
    width: 100%;
    height: 100%;
    border-radius: 50%;
    background: radial-gradient(
      ellipse at 30% 30%,
      rgba(255, 255, 255, 0.45) 0%,
      rgba(var(--severity-color-rgb, 135, 206, 235), 0.25) 30%,
      rgba(var(--severity-color-rgb, 64, 164, 223), 0.18) 60%,
      rgba(var(--severity-color-rgb, 30, 144, 255), 0.12) 100%
    );
    border: 1.5px solid rgba(255, 255, 255, 0.45);
    box-shadow:
      inset 0 0 20px rgba(255, 255, 255, 0.25),
      inset 0 0 40px color-mix(in srgb, var(--severity-color) 15%, transparent),
      0 0 15px color-mix(in srgb, var(--severity-color) 30%, transparent),
      0 4px 15px rgba(0, 0, 0, 0.15);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4px;
    position: relative;
    overflow: hidden;
    backdrop-filter: blur(3px);
  }

  .bubble-shine {
    position: absolute;
    top: 12%;
    left: 18%;
    width: 25%;
    height: 20%;
    background: radial-gradient(
      ellipse at center,
      rgba(255, 255, 255, 0.9) 0%,
      rgba(255, 255, 255, 0) 70%
    );
    border-radius: 50%;
    transform: rotate(-40deg);
  }

  .bubble-shine-2 {
    position: absolute;
    top: 55%;
    right: 15%;
    width: 12%;
    height: 10%;
    background: radial-gradient(
      ellipse at center,
      rgba(255, 255, 255, 0.6) 0%,
      rgba(255, 255, 255, 0) 70%
    );
    border-radius: 50%;
  }

  .bubble-icon {
    color: var(--severity-color, #fff);
    filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.3));
    z-index: 1;
  }

  .bubble-title {
    font-size: 10px;
    font-weight: 500;
    color: rgba(255, 255, 255, 0.95);
    text-align: center;
    padding: 0 8px;
    text-shadow:
      0 1px 2px rgba(0, 50, 100, 0.4),
      0 0 8px color-mix(in srgb, var(--severity-color) 50%, transparent);
    line-height: 1.2;
    max-width: 90%;
    overflow: hidden;
    text-overflow: ellipsis;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    z-index: 1;
  }

  .severity-indicator {
    position: absolute;
    bottom: 8px;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--severity-color);
    box-shadow: 0 0 8px var(--severity-color);
    animation: pulse 1.5s ease-in-out infinite;
  }

  .notification-bubble.hovered .bubble-inner {
    background: radial-gradient(
      ellipse at 30% 30%,
      rgba(255, 255, 255, 0.55) 0%,
      rgba(135, 206, 235, 0.35) 30%,
      rgba(64, 164, 223, 0.25) 60%,
      rgba(30, 144, 255, 0.18) 100%
    );
    box-shadow:
      inset 0 0 25px rgba(255, 255, 255, 0.35),
      inset 0 0 50px color-mix(in srgb, var(--severity-color) 25%, transparent),
      0 0 25px color-mix(in srgb, var(--severity-color) 45%, transparent),
      0 0 50px color-mix(in srgb, var(--severity-color) 20%, transparent),
      0 4px 20px rgba(0, 0, 0, 0.2);
  }

  /* 重要/紧急通知的特殊效果 */
  .notification-bubble.critical .bubble-inner,
  .notification-bubble.error .bubble-inner {
    animation: attention 2s ease-in-out infinite;
  }

  .notification-bubble.popping {
    animation: pop 0.3s ease-out forwards;
    pointer-events: none;
  }

  .notification-bubble.popping .bubble-inner {
    animation: pop-inner 0.3s ease-out forwards;
  }

  @keyframes pulse {
    0%,
    100% {
      opacity: 1;
      transform: scale(1);
    }
    50% {
      opacity: 0.6;
      transform: scale(1.2);
    }
  }

  @keyframes attention {
    0%,
    100% {
      box-shadow:
        inset 0 0 20px rgba(255, 255, 255, 0.25),
        inset 0 0 40px color-mix(in srgb, var(--severity-color) 15%, transparent),
        0 0 15px color-mix(in srgb, var(--severity-color) 30%, transparent),
        0 4px 15px rgba(0, 0, 0, 0.15);
    }
    50% {
      box-shadow:
        inset 0 0 25px rgba(255, 255, 255, 0.3),
        inset 0 0 50px color-mix(in srgb, var(--severity-color) 25%, transparent),
        0 0 30px color-mix(in srgb, var(--severity-color) 50%, transparent),
        0 4px 20px rgba(0, 0, 0, 0.2);
    }
  }

  @keyframes pop {
    0% {
      transform: translate(-50%, -50%) scale(1);
    }
    50% {
      transform: translate(-50%, -50%) scale(1.3);
    }
    100% {
      transform: translate(-50%, -50%) scale(0);
      opacity: 0;
    }
  }

  @keyframes pop-inner {
    0% {
      border-color: rgba(255, 255, 255, 0.45);
    }
    100% {
      border-color: rgba(255, 255, 255, 0);
      background: radial-gradient(
        circle at center,
        rgba(255, 255, 255, 0.7) 0%,
        rgba(255, 255, 255, 0) 70%
      );
    }
  }
</style>
