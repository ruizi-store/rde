/**
 * Lottie 动画 - 彩虹光圈
 */

export function initRainbowRings(container) {
  const canvas = document.createElement('canvas');
  canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;';
  container.appendChild(canvas);
  const ctx = canvas.getContext('2d');

  let raf;
  let time = 0;
  const rings = [];
  const numRings = 8;

  function resize() {
    canvas.width = container.clientWidth * devicePixelRatio;
    canvas.height = container.clientHeight * devicePixelRatio;
  }

  function initRings() {
    rings.length = 0;
    for (let i = 0; i < numRings; i++) {
      rings.push({
        baseRadius: 50 + i * 60,
        hue: i * 45,
        speed: 0.5 + i * 0.1,
        phase: i * 0.5
      });
    }
  }

  function render() {
    const w = container.clientWidth;
    const h = container.clientHeight;
    const cx = w / 2;
    const cy = h / 2;
    time += 0.02;
    
    ctx.fillStyle = 'rgba(10, 5, 15, 0.05)';
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    
    ctx.save();
    ctx.scale(devicePixelRatio, devicePixelRatio);
    
    for (let ring of rings) {
      const radius = ring.baseRadius + Math.sin(time * ring.speed + ring.phase) * 30;
      const hue = (ring.hue + time * 20) % 360;
      
      ctx.beginPath();
      ctx.arc(cx, cy, radius, 0, Math.PI * 2);
      ctx.strokeStyle = `hsla(${hue}, 100%, 60%, 0.6)`;
      ctx.lineWidth = 3 + Math.sin(time * 2 + ring.phase) * 2;
      ctx.shadowColor = `hsla(${hue}, 100%, 60%, 1)`;
      ctx.shadowBlur = 30;
      ctx.stroke();
      
      // Inner glow
      ctx.beginPath();
      ctx.arc(cx, cy, radius, 0, Math.PI * 2);
      ctx.strokeStyle = `hsla(${hue}, 100%, 80%, 0.3)`;
      ctx.lineWidth = 10;
      ctx.shadowBlur = 50;
      ctx.stroke();
    }
    
    // Center glow
    const gradient = ctx.createRadialGradient(cx, cy, 0, cx, cy, 100);
    gradient.addColorStop(0, `hsla(${(time * 30) % 360}, 100%, 70%, 0.5)`);
    gradient.addColorStop(1, 'transparent');
    ctx.fillStyle = gradient;
    ctx.beginPath();
    ctx.arc(cx, cy, 100, 0, Math.PI * 2);
    ctx.fill();
    
    ctx.restore();
    raf = requestAnimationFrame(render);
  }

  resize();
  initRings();
  render();
  window.addEventListener('resize', resize);

  return {
    destroy() {
      cancelAnimationFrame(raf);
      window.removeEventListener('resize', resize);
      canvas.remove();
    }
  };
}
