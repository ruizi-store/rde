<!-- ParticleSystem.svelte - 通用粒子系统 -->
<script lang="ts">
  import { onMount, onDestroy } from "svelte";

  interface Particle {
    x: number;
    y: number;
    vx: number;
    vy: number;
    size: number;
    opacity: number;
    color: string;
    life: number;
    maxLife: number;
    char?: string; // 可选：显示字符而不是圆点
  }

  interface Props {
    x: number;
    y: number;
    particleCount?: number;
    colors?: string[];
    chars?: string[]; // 如果提供，粒子将显示为字符
    spread?: number;
    gravity?: number;
    initialVelocity?: number;
    lifetime?: number;
    onComplete?: () => void;
  }

  let {
    x,
    y,
    particleCount = 20,
    colors = ["#fff", "#87CEEB", "#4FC3F7", "#29B6F6"],
    chars,
    spread = 360,
    gravity = 500,
    initialVelocity = 300,
    lifetime = 1.5,
    onComplete,
  }: Props = $props();

  let canvas: HTMLCanvasElement;
  let ctx: CanvasRenderingContext2D | null = null;
  let particles: Particle[] = [];
  let animationId: number;
  let lastTime = 0;
  let completed = false;

  function createParticles() {
    particles = [];
    const spreadRad = (spread * Math.PI) / 180;
    const startAngle = -Math.PI / 2 - spreadRad / 2;

    for (let i = 0; i < particleCount; i++) {
      const angle = startAngle + Math.random() * spreadRad;
      const velocity = initialVelocity * (0.5 + Math.random() * 0.5);
      const particleLifetime = lifetime * (0.7 + Math.random() * 0.3);

      particles.push({
        x: x,
        y: y,
        vx: Math.cos(angle) * velocity,
        vy: Math.sin(angle) * velocity,
        size: chars ? 14 + Math.random() * 8 : 3 + Math.random() * 4,
        opacity: 1,
        color: colors[Math.floor(Math.random() * colors.length)],
        life: particleLifetime,
        maxLife: particleLifetime,
        char: chars ? chars[i % chars.length] : undefined,
      });
    }
  }

  function update(deltaTime: number) {
    let allDead = true;

    for (const p of particles) {
      if (p.life <= 0) continue;
      allDead = false;

      // 应用重力
      p.vy += gravity * deltaTime;

      // 更新位置
      p.x += p.vx * deltaTime;
      p.y += p.vy * deltaTime;

      // 减少生命
      p.life -= deltaTime;

      // 更新透明度
      p.opacity = Math.max(0, p.life / p.maxLife);
    }

    if (allDead && !completed) {
      completed = true;
      onComplete?.();
    }
  }

  function render() {
    if (!ctx || !canvas) return;

    ctx.clearRect(0, 0, canvas.width, canvas.height);

    for (const p of particles) {
      if (p.life <= 0 || p.opacity <= 0) continue;

      ctx.globalAlpha = p.opacity;

      if (p.char) {
        // 绘制字符
        ctx.font = `bold ${p.size}px sans-serif`;
        ctx.fillStyle = p.color;
        ctx.textAlign = "center";
        ctx.textBaseline = "middle";
        ctx.fillText(p.char, p.x, p.y);
      } else {
        // 绘制圆点
        ctx.beginPath();
        ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2);
        ctx.fillStyle = p.color;
        ctx.fill();
      }
    }

    ctx.globalAlpha = 1;
  }

  function animate(currentTime: number) {
    if (!lastTime) lastTime = currentTime;
    const deltaTime = Math.min((currentTime - lastTime) / 1000, 0.1);
    lastTime = currentTime;

    update(deltaTime);
    render();

    if (!completed) {
      animationId = requestAnimationFrame(animate);
    }
  }

  onMount(() => {
    if (canvas) {
      canvas.width = window.innerWidth;
      canvas.height = window.innerHeight;
      ctx = canvas.getContext("2d");
      createParticles();
      animationId = requestAnimationFrame(animate);
    }
  });

  onDestroy(() => {
    if (animationId) {
      cancelAnimationFrame(animationId);
    }
  });
</script>

<canvas bind:this={canvas} class="particle-canvas"></canvas>

<style>
  .particle-canvas {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
    z-index: 10000;
  }
</style>
