<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { t } from "svelte-i18n";
  import { musicPlayer } from "$shared/stores/music-player.svelte";

  // 可视化模式
  type VisualizerMode = "bars" | "wave" | "circle";
  
  let { mode = "bars" as VisualizerMode, height = 80 } = $props();
  
  let canvas: HTMLCanvasElement | null = null;
  let ctx: CanvasRenderingContext2D | null = null;
  let animationId: number | null = null;
  let initialized = $state(false);

  // 颜色配置
  const colors = {
    primary: "#4a90d9",
    secondary: "#8b5cf6",
    accent: "#ec4899",
  };

  onMount(() => {
    if (canvas) {
      ctx = canvas.getContext("2d");
      resizeCanvas();
      window.addEventListener("resize", resizeCanvas);
    }
  });

  onDestroy(() => {
    if (animationId) {
      cancelAnimationFrame(animationId);
    }
    window.removeEventListener("resize", resizeCanvas);
  });

  function resizeCanvas() {
    if (!canvas) return;
    const rect = canvas.getBoundingClientRect();
    canvas.width = rect.width * window.devicePixelRatio;
    canvas.height = rect.height * window.devicePixelRatio;
    if (ctx) {
      ctx.scale(window.devicePixelRatio, window.devicePixelRatio);
    }
  }

  // 初始化可视化（需要用户交互触发）
  function initVisualizer() {
    if (initialized) return;
    musicPlayer.initAudioContext();
    initialized = true;
    startAnimation();
  }

  function startAnimation() {
    if (animationId) return;
    
    const draw = () => {
      if (!ctx || !canvas) {
        animationId = requestAnimationFrame(draw);
        return;
      }

      const width = canvas.width / window.devicePixelRatio;
      const heightPx = canvas.height / window.devicePixelRatio;

      // 清空画布
      ctx.clearRect(0, 0, width, heightPx);

      if (!musicPlayer.visualizerReady || !musicPlayer.isPlaying) {
        // 未初始化或未播放时绘制静态效果
        drawIdleState(width, heightPx);
        animationId = requestAnimationFrame(draw);
        return;
      }

      switch (mode) {
        case "bars":
          drawBars(width, heightPx);
          break;
        case "wave":
          drawWave(width, heightPx);
          break;
        case "circle":
          drawCircle(width, heightPx);
          break;
      }

      animationId = requestAnimationFrame(draw);
    };

    draw();
  }

  function drawIdleState(width: number, height: number) {
    if (!ctx) return;
    
    // 绘制静态柱状图
    const barCount = 32;
    const barWidth = width / barCount - 2;
    const gradient = ctx.createLinearGradient(0, height, 0, 0);
    gradient.addColorStop(0, colors.primary);
    gradient.addColorStop(0.5, colors.secondary);
    gradient.addColorStop(1, colors.accent);
    
    ctx.fillStyle = gradient;
    ctx.globalAlpha = 0.3;
    
    for (let i = 0; i < barCount; i++) {
      const barHeight = Math.sin(i / barCount * Math.PI) * height * 0.3 + 4;
      const x = i * (barWidth + 2);
      ctx.fillRect(x, height - barHeight, barWidth, barHeight);
    }
    
    ctx.globalAlpha = 1;
  }

  function drawBars(width: number, height: number) {
    if (!ctx) return;
    
    const data = musicPlayer.getFrequencyData();
    if (!data) return;

    const barCount = 64;
    const step = Math.floor(data.length / barCount);
    const barWidth = width / barCount - 2;

    // 创建渐变
    const gradient = ctx.createLinearGradient(0, height, 0, 0);
    gradient.addColorStop(0, colors.primary);
    gradient.addColorStop(0.5, colors.secondary);
    gradient.addColorStop(1, colors.accent);

    ctx.fillStyle = gradient;

    for (let i = 0; i < barCount; i++) {
      // 取样本中的最大值
      let max = 0;
      for (let j = 0; j < step; j++) {
        const idx = i * step + j;
        if (idx < data.length && data[idx] > max) {
          max = data[idx];
        }
      }

      const barHeight = (max / 255) * height * 0.9 + 2;
      const x = i * (barWidth + 2);

      // 绘制圆角矩形
      ctx.beginPath();
      ctx.roundRect(x, height - barHeight, barWidth, barHeight, 2);
      ctx.fill();
    }
  }

  function drawWave(width: number, height: number) {
    if (!ctx) return;
    
    const data = musicPlayer.getWaveformData();
    if (!data) return;

    const gradient = ctx.createLinearGradient(0, 0, width, 0);
    gradient.addColorStop(0, colors.primary);
    gradient.addColorStop(0.5, colors.secondary);
    gradient.addColorStop(1, colors.accent);

    ctx.strokeStyle = gradient;
    ctx.lineWidth = 2;
    ctx.beginPath();

    const sliceWidth = width / data.length;

    for (let i = 0; i < data.length; i++) {
      const v = data[i] / 128.0;
      const y = (v * height) / 2;
      const x = i * sliceWidth;

      if (i === 0) {
        ctx.moveTo(x, y);
      } else {
        ctx.lineTo(x, y);
      }
    }

    ctx.stroke();

    // 绘制镜像波形
    ctx.globalAlpha = 0.3;
    ctx.beginPath();
    for (let i = 0; i < data.length; i++) {
      const v = data[i] / 128.0;
      const y = height - (v * height) / 2;
      const x = i * sliceWidth;

      if (i === 0) {
        ctx.moveTo(x, y);
      } else {
        ctx.lineTo(x, y);
      }
    }
    ctx.stroke();
    ctx.globalAlpha = 1;
  }

  function drawCircle(width: number, height: number) {
    if (!ctx) return;
    
    const data = musicPlayer.getFrequencyData();
    if (!data) return;

    const centerX = width / 2;
    const centerY = height / 2;
    const radius = Math.min(width, height) * 0.3;

    const gradient = ctx.createRadialGradient(
      centerX, centerY, radius * 0.5,
      centerX, centerY, radius * 1.5
    );
    gradient.addColorStop(0, colors.primary);
    gradient.addColorStop(0.5, colors.secondary);
    gradient.addColorStop(1, colors.accent);

    ctx.strokeStyle = gradient;
    ctx.lineWidth = 2;

    const barCount = 60;
    const step = Math.floor(data.length / barCount);

    for (let i = 0; i < barCount; i++) {
      let max = 0;
      for (let j = 0; j < step; j++) {
        const idx = i * step + j;
        if (idx < data.length && data[idx] > max) {
          max = data[idx];
        }
      }

      const angle = (i / barCount) * Math.PI * 2 - Math.PI / 2;
      const barLength = (max / 255) * radius * 0.8 + 4;

      const x1 = centerX + Math.cos(angle) * radius;
      const y1 = centerY + Math.sin(angle) * radius;
      const x2 = centerX + Math.cos(angle) * (radius + barLength);
      const y2 = centerY + Math.sin(angle) * (radius + barLength);

      ctx.beginPath();
      ctx.moveTo(x1, y1);
      ctx.lineTo(x2, y2);
      ctx.stroke();
    }

    // 绘制中心圆
    ctx.fillStyle = "rgba(74, 144, 217, 0.1)";
    ctx.beginPath();
    ctx.arc(centerX, centerY, radius, 0, Math.PI * 2);
    ctx.fill();
  }

  // 响应播放状态变化
  $effect(() => {
    if (musicPlayer.isPlaying && initialized) {
      startAnimation();
    }
  });
</script>

<div
  class="visualizer"
  style="height: {height}px"
  onclick={initVisualizer}
  role="button"
  tabindex="0"
>
  <canvas bind:this={canvas}></canvas>
  {#if !initialized}
    <div class="init-hint">{$t('music.clickToEnableVisualizer')}</div>
  {/if}
</div>

<style>
  .visualizer {
    position: relative;
    width: 100%;
    cursor: pointer;
  }

  canvas {
    width: 100%;
    height: 100%;
    display: block;
  }

  .init-hint {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    font-size: 12px;
    color: var(--text-secondary, rgba(255, 255, 255, 0.5));
    background: rgba(0, 0, 0, 0.3);
    padding: 4px 12px;
    border-radius: 12px;

    :global([data-theme="light"]) & {
      background: rgba(255, 255, 255, 0.8);
      color: rgba(0, 0, 0, 0.5);
    }
  }
</style>
