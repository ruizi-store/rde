<!-- VisualizerEffects.svelte - 全屏可视化特效 -->
<!-- 支持: bars(频谱柱), circle(环形), kaleidoscope(万花筒) -->
<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { musicPlayer, type VisualizerMode } from "$shared/stores/music-player.svelte";

  let { mode = "circle" as VisualizerMode, initialized = false } = $props();

  let canvas: HTMLCanvasElement | null = null;
  let ctx: CanvasRenderingContext2D | null = null;
  let animationId: number | null = null;
  let width = $state(0);
  let height = $state(0);

  // 万花筒角度
  let kaleidoscopeAngle = 0;

  // 颜色配置
  const colors = {
    primary: "#4a90d9",
    secondary: "#8b5cf6",
    accent: "#ec4899",
    cyan: "#06b6d4",
    green: "#10b981",
  };

  onMount(() => {
    if (canvas) {
      ctx = canvas.getContext("2d");
      handleResize();
      window.addEventListener("resize", handleResize);
      startAnimation();
    }
  });

  onDestroy(() => {
    if (animationId) {
      cancelAnimationFrame(animationId);
    }
    window.removeEventListener("resize", handleResize);
  });

  function handleResize() {
    if (!canvas) return;
    // 使用画布容器的尺寸（如果存在），否则使用 window 尺寸
    const container = canvas.parentElement;
    if (container) {
      const rect = container.getBoundingClientRect();
      width = rect.width;
      height = rect.height;
    } else {
      width = window.innerWidth;
      height = window.innerHeight;
    }
    canvas.width = width * window.devicePixelRatio;
    canvas.height = height * window.devicePixelRatio;
    if (ctx) {
      ctx.scale(window.devicePixelRatio, window.devicePixelRatio);
    }
  }

  function startAnimation() {
    const draw = () => {
      if (!ctx || !canvas) {
        animationId = requestAnimationFrame(draw);
        return;
      }

      // 确保宽高有效
      if (width <= 0 || height <= 0) {
        handleResize();
        animationId = requestAnimationFrame(draw);
        return;
      }

      // 清空画布（半透明以产生拖尾效果）
      ctx.fillStyle = "rgba(0, 0, 0, 0.1)";
      ctx.fillRect(0, 0, width, height);

      const frequencyData = musicPlayer.getFrequencyData();

      switch (mode) {
        case "bars":
          drawBars(frequencyData);
          break;
        case "circle":
          drawCircle(frequencyData);
          break;
        case "kaleidoscope":
          drawKaleidoscope(frequencyData);
          break;
        case "off":
          // 关闭特效时清空画布
          ctx.clearRect(0, 0, width, height);
          break;
      }

      animationId = requestAnimationFrame(draw);
    };

    draw();
  }

  // 频谱柱
  function drawBars(data: Uint8Array | null) {
    if (!ctx) return;

    const barCount = 128;
    const barWidth = width / barCount;
    const barGap = 2;

    // 生成柱状数据
    const bars: number[] = [];
    if (data && data.length > 0) {
      const step = Math.max(1, Math.floor(data.length / barCount));
      for (let i = 0; i < barCount; i++) {
        let sum = 0;
        for (let j = 0; j < step; j++) {
          const idx = i * step + j;
          if (idx < data.length) sum += data[idx];
        }
        bars.push(sum / step / 255);
      }
    } else {
      for (let i = 0; i < barCount; i++) {
        bars.push(Math.sin(i / barCount * Math.PI) * 0.3);
      }
    }

    // 绘制柱状图（从底部向上）
    for (let i = 0; i < barCount; i++) {
      const barHeight = bars[i] * height * 0.7 + 4;
      const x = i * barWidth;
      const hue = (i / barCount) * 60 + 200; // 蓝紫色渐变

      // 创建渐变
      const gradient = ctx.createLinearGradient(x, height, x, height - barHeight);
      gradient.addColorStop(0, `hsla(${hue}, 80%, 60%, 0.9)`);
      gradient.addColorStop(0.5, `hsla(${hue + 30}, 80%, 50%, 0.8)`);
      gradient.addColorStop(1, `hsla(${hue + 60}, 80%, 70%, 0.6)`);

      ctx.fillStyle = gradient;
      ctx.fillRect(x + barGap / 2, height - barHeight, barWidth - barGap, barHeight);

      // 顶部反光
      ctx.fillStyle = `hsla(${hue}, 100%, 80%, 0.5)`;
      ctx.fillRect(x + barGap / 2, height - barHeight, barWidth - barGap, 3);
    }

    // 镜像效果（顶部）
    ctx.save();
    ctx.globalAlpha = 0.3;
    ctx.scale(1, -1);
    ctx.translate(0, -height);
    for (let i = 0; i < barCount; i++) {
      const barHeight = bars[i] * height * 0.3;
      const x = i * barWidth;
      const hue = (i / barCount) * 60 + 200;
      ctx.fillStyle = `hsla(${hue}, 80%, 60%, 0.5)`;
      ctx.fillRect(x + barGap / 2, height - barHeight, barWidth - barGap, barHeight);
    }
    ctx.restore();
  }

  // 环形频谱
  function drawCircle(data: Uint8Array | null) {
    if (!ctx) return;

    const centerX = width / 2;
    const centerY = height / 2;
    const baseRadius = Math.max(50, Math.min(width, height) * 0.25);
    const barCount = 180;

    // 生成数据
    const bars: number[] = [];
    if (data && data.length > 0) {
      const step = Math.max(1, Math.floor(data.length / barCount));
      for (let i = 0; i < barCount; i++) {
        let sum = 0;
        for (let j = 0; j < step; j++) {
          const idx = i * step + j;
          if (idx < data.length) sum += data[idx];
        }
        bars.push(sum / step / 255);
      }
    } else {
      const time = Date.now() / 1000;
      for (let i = 0; i < barCount; i++) {
        bars.push(Math.sin(i / 10 + time) * 0.3 + 0.3);
      }
    }

    // 绘制外圈
    for (let i = 0; i < barCount; i++) {
      const angle = (i / barCount) * Math.PI * 2 - Math.PI / 2;
      const barLength = bars[i] * baseRadius + 10;
      const hue = (i / barCount) * 360;

      const x1 = centerX + Math.cos(angle) * baseRadius;
      const y1 = centerY + Math.sin(angle) * baseRadius;
      const x2 = centerX + Math.cos(angle) * (baseRadius + barLength);
      const y2 = centerY + Math.sin(angle) * (baseRadius + barLength);

      ctx.beginPath();
      ctx.moveTo(x1, y1);
      ctx.lineTo(x2, y2);
      ctx.strokeStyle = `hsla(${hue}, 80%, 60%, 0.8)`;
      ctx.lineWidth = 3;
      ctx.lineCap = "round";
      ctx.stroke();

      // 发光效果
      ctx.shadowColor = `hsla(${hue}, 100%, 50%, 0.5)`;
      ctx.shadowBlur = 10;
    }
    ctx.shadowBlur = 0;

    // 绘制内圈（反向）
    ctx.globalAlpha = 0.5;
    for (let i = 0; i < barCount; i++) {
      const angle = (i / barCount) * Math.PI * 2 - Math.PI / 2;
      const barLength = bars[i] * baseRadius * 0.5 + 5;
      const hue = (i / barCount) * 360;

      const x1 = centerX + Math.cos(angle) * (baseRadius * 0.8);
      const y1 = centerY + Math.sin(angle) * (baseRadius * 0.8);
      const x2 = centerX + Math.cos(angle) * (baseRadius * 0.8 - barLength);
      const y2 = centerY + Math.sin(angle) * (baseRadius * 0.8 - barLength);

      ctx.beginPath();
      ctx.moveTo(x1, y1);
      ctx.lineTo(x2, y2);
      ctx.strokeStyle = `hsla(${hue}, 80%, 70%, 0.6)`;
      ctx.lineWidth = 2;
      ctx.stroke();
    }
    ctx.globalAlpha = 1;

    // 中心圆
    const avgLevel = bars.length > 0 ? bars.reduce((a, b) => a + b, 0) / bars.length : 0;
    const pulseRadius = Math.max(10, baseRadius * 0.5 + avgLevel * 50);

    // 安全检查
    if (!isFinite(pulseRadius) || pulseRadius <= 0) return;

    const gradient = ctx.createRadialGradient(
      centerX, centerY, 0,
      centerX, centerY, pulseRadius
    );
    gradient.addColorStop(0, "rgba(255, 255, 255, 0.3)");
    gradient.addColorStop(0.5, "rgba(139, 92, 246, 0.2)");
    gradient.addColorStop(1, "rgba(74, 144, 217, 0)");

    ctx.beginPath();
    ctx.arc(centerX, centerY, pulseRadius, 0, Math.PI * 2);
    ctx.fillStyle = gradient;
    ctx.fill();
  }

  // 万花筒
  function drawKaleidoscope(data: Uint8Array | null) {
    if (!ctx) return;

    const centerX = width / 2;
    const centerY = height / 2;
    const segments = 12;
    const angleStep = (Math.PI * 2) / segments;

    // 计算能量
    let energy = 0;
    if (data && data.length > 0) {
      for (let i = 0; i < data.length; i++) {
        energy += data[i];
      }
      energy = energy / data.length / 255;
    } else {
      energy = 0.3;
    }

    kaleidoscopeAngle += 0.005 + energy * 0.02;

    ctx.save();
    ctx.translate(centerX, centerY);

    // 绘制每个段
    for (let seg = 0; seg < segments; seg++) {
      ctx.save();
      ctx.rotate(seg * angleStep + kaleidoscopeAngle);

      // 镜像奇数段
      if (seg % 2 === 1) {
        ctx.scale(-1, 1);
      }

      // 绘制形状
      const maxRadius = Math.min(width, height) * 0.45;
      const shapeCount = 8;

      for (let i = 0; i < shapeCount; i++) {
        const dataIdx = data ? Math.floor((i / shapeCount) * data.length) : 0;
        const value = data ? data[dataIdx] / 255 : Math.sin(Date.now() / 500 + i) * 0.5 + 0.5;
        const radius = (i / shapeCount) * maxRadius + value * 50;
        const hue = (i / shapeCount) * 60 + 200 + kaleidoscopeAngle * 10;

        ctx.beginPath();
        ctx.moveTo(0, 0);
        ctx.lineTo(
          Math.cos(angleStep / 2) * radius,
          Math.sin(angleStep / 2) * radius
        );
        ctx.lineTo(
          Math.cos(-angleStep / 2) * radius,
          Math.sin(-angleStep / 2) * radius
        );
        ctx.closePath();

        ctx.fillStyle = `hsla(${hue}, 70%, ${50 + value * 20}%, ${0.1 + value * 0.2})`;
        ctx.fill();

        // 边框发光
        ctx.strokeStyle = `hsla(${hue}, 80%, 60%, ${0.3 + value * 0.3})`;
        ctx.lineWidth = 1 + value * 2;
        ctx.stroke();
      }

      ctx.restore();
    }

    ctx.restore();

    // 中心光晕
    const glowRadius = Math.max(10, 100 + energy * 50);
    
    // 安全检查
    if (!isFinite(glowRadius) || !isFinite(centerX) || !isFinite(centerY)) return;

    const gradient = ctx.createRadialGradient(
      centerX, centerY, 0,
      centerX, centerY, glowRadius
    );
    gradient.addColorStop(0, `rgba(255, 255, 255, ${energy * 0.5 + 0.2})`);
    gradient.addColorStop(0.5, `hsla(260, 80%, 70%, ${energy * 0.3})`);
    gradient.addColorStop(1, "transparent");

    ctx.beginPath();
    ctx.arc(centerX, centerY, glowRadius, 0, Math.PI * 2);
    ctx.fillStyle = gradient;
    ctx.fill();
  }

  // 音乐喷泉特效
  function drawFountain(data: Uint8Array | null) {
    if (!ctx) return;

    const centerX = width / 2;
    const baseY = height * 0.85; // 喷泉底部位置
    const time = Date.now() / 1000;
    
    // 计算整体能量
    let energy = 0;
    if (data && data.length > 0) {
      for (let i = 0; i < data.length; i++) {
        energy += data[i];
      }
      energy = energy / data.length / 255;
    } else {
      energy = 0.3;
    }

    // 喷泉水柱数量
    const streamCount = 32;
    const arcStreams = 12; // 弧形水柱数量

    // 绘制中心喷泉底座光晕
    const baseRadius = 60 + energy * 40;
    if (isFinite(baseRadius) && isFinite(centerX) && isFinite(baseY)) {
      const baseGradient = ctx.createRadialGradient(
        centerX, baseY, 0,
        centerX, baseY, baseRadius
      );
      baseGradient.addColorStop(0, `rgba(255, 255, 255, ${0.3 + energy * 0.3})`);
      baseGradient.addColorStop(0.5, `rgba(100, 200, 255, ${0.2 + energy * 0.2})`);
      baseGradient.addColorStop(1, "transparent");
      
      ctx.beginPath();
      ctx.arc(centerX, baseY, baseRadius, 0, Math.PI * 2);
      ctx.fillStyle = baseGradient;
      ctx.fill();
    }

    // 绘制垂直喷射的水柱
    for (let i = 0; i < streamCount; i++) {
      const dataIdx = data ? Math.floor((i / streamCount) * data.length) : 0;
      const value = data ? data[dataIdx] / 255 : Math.sin(time + i) * 0.5 + 0.5;
      
      // 水柱从中心向两侧散开
      const spreadAngle = (i / streamCount - 0.5) * Math.PI * 0.8; // -72° 到 72°
      const baseHeight = height * 0.4;
      const streamHeight = baseHeight * (0.3 + value * 0.7);
      
      // 计算水柱路径（抛物线）
      const startX = centerX + Math.sin(spreadAngle) * 30;
      const startY = baseY;
      const endX = centerX + Math.sin(spreadAngle) * streamHeight * 0.8;
      const endY = baseY - streamHeight;
      const controlX = (startX + endX) / 2;
      const controlY = baseY - streamHeight * 1.2;
      
      // 颜色渐变（彩虹色）
      const hue = (i / streamCount) * 180 + 180 + time * 30; // 蓝到粉
      const saturation = 80 + value * 20;
      const lightness = 50 + value * 20;
      
      // 绘制水柱（使用二次贝塞尔曲线）
      ctx.beginPath();
      ctx.moveTo(startX, startY);
      ctx.quadraticCurveTo(controlX, controlY, endX, endY);
      
      const streamWidth = 2 + value * 4;
      ctx.lineWidth = streamWidth;
      ctx.strokeStyle = `hsla(${hue}, ${saturation}%, ${lightness}%, ${0.6 + value * 0.4})`;
      ctx.lineCap = "round";
      ctx.stroke();
      
      // 发光效果
      ctx.shadowColor = `hsla(${hue}, 100%, 60%, 0.8)`;
      ctx.shadowBlur = 10 + value * 15;
      ctx.stroke();
      ctx.shadowBlur = 0;
      
      // 水花粒子效果（在顶端）
      if (value > 0.3) {
        const particleCount = Math.floor(value * 5);
        for (let p = 0; p < particleCount; p++) {
          const px = endX + (Math.random() - 0.5) * 30 * value;
          const py = endY + (Math.random() - 0.5) * 20 * value;
          const pSize = 1 + Math.random() * 3 * value;
          
          ctx.beginPath();
          ctx.arc(px, py, pSize, 0, Math.PI * 2);
          ctx.fillStyle = `hsla(${hue}, ${saturation}%, ${lightness + 20}%, ${0.5 + value * 0.5})`;
          ctx.fill();
        }
      }
    }

    // 绘制弧形水柱（向外喷射的彩色弧线）
    for (let i = 0; i < arcStreams; i++) {
      const dataIdx = data ? Math.floor((i / arcStreams) * (data.length / 2)) : 0;
      const value = data ? data[dataIdx] / 255 : Math.sin(time * 2 + i * 0.5) * 0.5 + 0.5;
      
      // 左右对称
      for (let side = -1; side <= 1; side += 2) {
        const angle = (Math.PI * 0.3) + (i / arcStreams) * (Math.PI * 0.4);
        const arcLength = height * 0.3 * (0.4 + value * 0.6);
        
        const startX = centerX + side * 20;
        const startY = baseY - 10;
        const endX = centerX + side * arcLength * Math.sin(angle);
        const endY = baseY - arcLength * Math.cos(angle) * 0.6;
        const controlX = startX + side * arcLength * 0.5 * Math.sin(angle * 0.5);
        const controlY = baseY - arcLength * 0.8;
        
        const hue = (i / arcStreams) * 60 + (side > 0 ? 200 : 280) + time * 20;
        
        ctx.beginPath();
        ctx.moveTo(startX, startY);
        ctx.quadraticCurveTo(controlX, controlY, endX, endY);
        
        ctx.lineWidth = 3 + value * 5;
        ctx.strokeStyle = `hsla(${hue}, 85%, 55%, ${0.5 + value * 0.5})`;
        ctx.lineCap = "round";
        ctx.shadowColor = `hsla(${hue}, 100%, 60%, 0.7)`;
        ctx.shadowBlur = 8 + value * 12;
        ctx.stroke();
        ctx.shadowBlur = 0;
      }
    }

    // 中心高喷泉（主柱）
    const mainValue = data ? (data[0] + data[1] + data[2]) / 3 / 255 : energy;
    const mainHeight = height * 0.5 * (0.4 + mainValue * 0.6);
    
    // 主水柱渐变
    const mainGradient = ctx.createLinearGradient(centerX, baseY, centerX, baseY - mainHeight);
    mainGradient.addColorStop(0, "rgba(100, 200, 255, 0.9)");
    mainGradient.addColorStop(0.3, "rgba(150, 100, 255, 0.8)");
    mainGradient.addColorStop(0.6, "rgba(255, 100, 200, 0.7)");
    mainGradient.addColorStop(1, "rgba(255, 255, 255, 0.3)");
    
    ctx.beginPath();
    ctx.moveTo(centerX - 8, baseY);
    ctx.lineTo(centerX - 3, baseY - mainHeight);
    ctx.lineTo(centerX + 3, baseY - mainHeight);
    ctx.lineTo(centerX + 8, baseY);
    ctx.closePath();
    ctx.fillStyle = mainGradient;
    ctx.fill();
    
    // 主水柱发光
    ctx.shadowColor = "rgba(100, 200, 255, 0.8)";
    ctx.shadowBlur = 20 + mainValue * 30;
    ctx.fill();
    ctx.shadowBlur = 0;
  }
</script>

<canvas bind:this={canvas} class="visualizer-canvas"></canvas>

<style>
  .visualizer-canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
  }
</style>
