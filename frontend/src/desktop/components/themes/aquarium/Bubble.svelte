<!-- Bubble.svelte - 单个泡泡组件 -->
<script lang="ts">
  import { onMount, onDestroy } from "svelte";

  interface Props {
    id: string;
    text: string;
    initialX: number;
    initialY: number;
    size?: number;
    onPop: (id: string, x: number, y: number, text: string) => void;
  }

  let { id, text, initialX, initialY, size = 80, onPop }: Props = $props();

  let x = $state(initialX);
  let y = $state(initialY);
  let isHovered = $state(false);
  let isPopping = $state(false);
  let opacity = $state(0);
  let scale = $state(0.3);

  // 动画参数
  let wobblePhase = Math.random() * Math.PI * 2;
  let wobbleSpeed = 0.8 + Math.random() * 0.4;
  let wobbleAmplitude = 15 + Math.random() * 10;
  let risingSpeed = 8 + Math.random() * 6;
  let horizontalDrift = (Math.random() - 0.5) * 0.3;

  let animationId: number;
  let startTime: number;

  function animate(currentTime: number) {
    if (!startTime) startTime = currentTime;
    const elapsed = (currentTime - startTime) / 1000;

    // 入场动画
    if (elapsed < 0.5) {
      opacity = Math.min(1, elapsed * 2);
      scale = 0.3 + 0.7 * Math.min(1, elapsed * 2);
    } else {
      opacity = 1;
      scale = 1;
    }

    // 摆动效果
    const wobble = Math.sin(elapsed * wobbleSpeed + wobblePhase) * wobbleAmplitude;
    x = initialX + wobble + horizontalDrift * elapsed * 50;

    // 缓慢上升
    y = initialY - elapsed * risingSpeed;

    // 如果飘出屏幕顶部，重置到底部
    if (y < -size) {
      initialY = window.innerHeight + size;
      initialX = Math.random() * (window.innerWidth - size * 2) + size;
      y = initialY;
      x = initialX;
      startTime = currentTime;
      wobblePhase = Math.random() * Math.PI * 2;
    }

    if (!isPopping) {
      animationId = requestAnimationFrame(animate);
    }
  }

  function handleClick(e: MouseEvent) {
    e.stopPropagation();
    if (isPopping) return;

    isPopping = true;

    // 通知父组件泡泡被戳破
    onPop(id, x, y, text);
  }

  function handleMouseEnter() {
    isHovered = true;
  }

  function handleMouseLeave() {
    isHovered = false;
  }

  onMount(() => {
    animationId = requestAnimationFrame(animate);
  });

  onDestroy(() => {
    if (animationId) {
      cancelAnimationFrame(animationId);
    }
  });
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
  class="bubble"
  class:hovered={isHovered}
  class:popping={isPopping}
  style="
    left: {x}px;
    top: {y}px;
    width: {size}px;
    height: {size}px;
    opacity: {opacity};
    transform: translate(-50%, -50%) scale({isHovered ? scale * 1.1 : scale});
  "
  onclick={handleClick}
  onmouseenter={handleMouseEnter}
  onmouseleave={handleMouseLeave}
>
  <div class="bubble-inner">
    <div class="bubble-shine"></div>
    <div class="bubble-shine-2"></div>
    <span class="bubble-text">{text}</span>
  </div>
</div>

<style>
  .bubble {
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
      rgba(255, 255, 255, 0.4) 0%,
      rgba(135, 206, 235, 0.2) 30%,
      rgba(64, 164, 223, 0.15) 60%,
      rgba(30, 144, 255, 0.1) 100%
    );
    border: 1.5px solid rgba(255, 255, 255, 0.4);
    box-shadow:
      inset 0 0 20px rgba(255, 255, 255, 0.2),
      inset 0 0 40px rgba(135, 206, 235, 0.1),
      0 0 10px rgba(135, 206, 235, 0.2),
      0 4px 15px rgba(0, 0, 0, 0.1);
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
    overflow: hidden;
    backdrop-filter: blur(2px);
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

  .bubble-text {
    font-size: 11px;
    font-weight: 500;
    color: rgba(255, 255, 255, 0.95);
    text-align: center;
    padding: 8px;
    text-shadow:
      0 1px 2px rgba(0, 50, 100, 0.3),
      0 0 10px rgba(135, 206, 235, 0.5);
    line-height: 1.3;
    max-width: 90%;
    word-break: keep-all;
    z-index: 1;
  }

  .bubble.hovered .bubble-inner {
    background: radial-gradient(
      ellipse at 30% 30%,
      rgba(255, 255, 255, 0.5) 0%,
      rgba(135, 206, 235, 0.3) 30%,
      rgba(64, 164, 223, 0.2) 60%,
      rgba(30, 144, 255, 0.15) 100%
    );
    box-shadow:
      inset 0 0 25px rgba(255, 255, 255, 0.3),
      inset 0 0 50px rgba(135, 206, 235, 0.2),
      0 0 20px rgba(135, 206, 235, 0.4),
      0 0 40px rgba(64, 164, 223, 0.2),
      0 4px 20px rgba(0, 0, 0, 0.15);
  }

  .bubble.popping {
    animation: pop 0.3s ease-out forwards;
    pointer-events: none;
  }

  .bubble.popping .bubble-inner {
    animation: pop-inner 0.3s ease-out forwards;
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
      border-color: rgba(255, 255, 255, 0.4);
    }
    100% {
      border-color: rgba(255, 255, 255, 0);
      background: radial-gradient(
        circle at center,
        rgba(255, 255, 255, 0.6) 0%,
        rgba(255, 255, 255, 0) 70%
      );
    }
  }
</style>
