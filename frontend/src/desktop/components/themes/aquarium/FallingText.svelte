<!-- FallingText.svelte - 掉落的文字组件 -->
<script lang="ts">
  import { onMount, onDestroy } from "svelte";

  interface Props {
    text: string;
    startX: number;
    startY: number;
    taskbarHeight?: number;
    onHitTaskbar: (x: number, y: number, chars: string[]) => void;
  }

  let { text, startX, startY, taskbarHeight = 48, onHitTaskbar }: Props = $props();

  // 初始位置来自 props，之后由物理引擎控制
  // svelte-ignore state_referenced_locally
  let x = $state(startX);
  // svelte-ignore state_referenced_locally
  let y = $state(startY);
  let rotation = $state(0);
  let opacity = $state(1);
  let visible = $state(true);

  // 物理参数
  let velocityY = -100; // 初始向上的速度（泡泡破裂的反弹）
  let velocityX = (Math.random() - 0.5) * 100;
  const gravity = 1200;
  const rotationSpeed = (Math.random() - 0.5) * 400;

  let animationId: number;
  let lastTime = 0;

  // 计算碰撞边界
  $effect(() => {
    // 任务栏顶部的 Y 坐标
  });

  function animate(currentTime: number) {
    if (!lastTime) lastTime = currentTime;
    const deltaTime = Math.min((currentTime - lastTime) / 1000, 0.05);
    lastTime = currentTime;

    // 应用重力
    velocityY += gravity * deltaTime;

    // 更新位置
    x += velocityX * deltaTime;
    y += velocityY * deltaTime;

    // 旋转
    rotation += rotationSpeed * deltaTime;

    // 检查是否碰到任务栏
    const taskbarTop = window.innerHeight - taskbarHeight;
    if (y >= taskbarTop - 10) {
      // 碰撞！触发碎裂效果
      visible = false;
      const chars = text.split("");
      onHitTaskbar(x, taskbarTop, chars);
      return;
    }

    // 边界检查（屏幕两侧反弹）
    if (x < 20 || x > window.innerWidth - 20) {
      velocityX *= -0.7;
      x = Math.max(20, Math.min(window.innerWidth - 20, x));
    }

    animationId = requestAnimationFrame(animate);
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

{#if visible}
  <div
    class="falling-text"
    style="
      left: {x}px;
      top: {y}px;
      transform: translate(-50%, -50%) rotate({rotation}deg);
      opacity: {opacity};
    "
  >
    {text}
  </div>
{/if}

<style>
  .falling-text {
    position: fixed;
    font-size: 14px;
    font-weight: 600;
    color: #fff;
    text-shadow:
      0 2px 4px rgba(0, 50, 100, 0.5),
      0 0 20px rgba(135, 206, 235, 0.8),
      0 0 40px rgba(64, 164, 223, 0.4);
    white-space: nowrap;
    pointer-events: none;
    z-index: 9999;
    letter-spacing: 1px;
  }
</style>
