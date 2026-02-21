/**
 * Lottie 动画 - 星空穿梭
 */

export function initStarTravel(container) {
  const canvas = document.createElement('canvas');
  canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;';
  container.appendChild(canvas);
  const ctx = canvas.getContext('2d');

  let raf;
  const stars = [];
  const numStars = 400;

  function resize() {
    canvas.width = container.clientWidth * devicePixelRatio;
    canvas.height = container.clientHeight * devicePixelRatio;
    ctx.scale(devicePixelRatio, devicePixelRatio);
  }

  function initStars() {
    const w = container.clientWidth;
    const h = container.clientHeight;
    for (let i = 0; i < numStars; i++) {
      stars.push({
        x: Math.random() * w - w/2,
        y: Math.random() * h - h/2,
        z: Math.random() * 1000
      });
    }
  }

  function render() {
    const w = container.clientWidth;
    const h = container.clientHeight;
    
    ctx.fillStyle = 'rgba(0, 0, 10, 0.2)';
    ctx.fillRect(0, 0, w, h);
    
    const cx = w / 2;
    const cy = h / 2;
    
    for (let star of stars) {
      star.z -= 5;
      if (star.z <= 0) {
        star.x = Math.random() * w - w/2;
        star.y = Math.random() * h - h/2;
        star.z = 1000;
      }
      
      const k = 200 / star.z;
      const sx = star.x * k + cx;
      const sy = star.y * k + cy;
      const size = (1 - star.z / 1000) * 3;
      
      if (sx >= 0 && sx <= w && sy >= 0 && sy <= h) {
        const alpha = 1 - star.z / 1000;
        ctx.beginPath();
        ctx.arc(sx, sy, size, 0, Math.PI * 2);
        ctx.fillStyle = `rgba(255, 255, 255, ${alpha})`;
        ctx.fill();
        
        // Trail
        const px = star.x * (200 / (star.z + 20)) + cx;
        const py = star.y * (200 / (star.z + 20)) + cy;
        ctx.beginPath();
        ctx.moveTo(sx, sy);
        ctx.lineTo(px, py);
        ctx.strokeStyle = `rgba(100, 150, 255, ${alpha * 0.5})`;
        ctx.lineWidth = size * 0.5;
        ctx.stroke();
      }
    }
    
    raf = requestAnimationFrame(render);
  }

  resize();
  initStars();
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
