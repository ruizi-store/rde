/**
 * Lottie 动画 - 流星雨
 */

export function initMeteorShower(container) {
  const canvas = document.createElement('canvas');
  canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;';
  container.appendChild(canvas);
  const ctx = canvas.getContext('2d');

  let raf;
  const meteors = [];
  const stars = [];

  function resize() {
    canvas.width = container.clientWidth * devicePixelRatio;
    canvas.height = container.clientHeight * devicePixelRatio;
    initStars();
  }

  function initStars() {
    stars.length = 0;
    const w = container.clientWidth;
    const h = container.clientHeight;
    for (let i = 0; i < 200; i++) {
      stars.push({
        x: Math.random() * w,
        y: Math.random() * h,
        size: Math.random() * 2,
        twinkle: Math.random() * Math.PI * 2
      });
    }
  }

  function spawnMeteor() {
    const w = container.clientWidth;
    meteors.push({
      x: Math.random() * w * 1.5,
      y: -50,
      length: 80 + Math.random() * 120,
      speed: 8 + Math.random() * 8,
      angle: Math.PI / 4 + (Math.random() - 0.5) * 0.3,
      alpha: 0.5 + Math.random() * 0.5,
      hue: Math.random() > 0.7 ? 30 : 200 // Golden or blue
    });
  }

  function render() {
    const w = container.clientWidth;
    const h = container.clientHeight;
    
    // Dark sky
    ctx.fillStyle = '#0a0a15';
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    
    ctx.save();
    ctx.scale(devicePixelRatio, devicePixelRatio);
    
    // Stars
    const time = Date.now() / 1000;
    for (let star of stars) {
      const twinkle = 0.5 + 0.5 * Math.sin(time * 2 + star.twinkle);
      ctx.beginPath();
      ctx.arc(star.x, star.y, star.size * twinkle, 0, Math.PI * 2);
      ctx.fillStyle = `rgba(255, 255, 255, ${twinkle * 0.8})`;
      ctx.fill();
    }
    
    // Meteors
    if (Math.random() < 0.03) spawnMeteor();
    
    for (let i = meteors.length - 1; i >= 0; i--) {
      const m = meteors[i];
      
      m.x += Math.cos(m.angle) * m.speed;
      m.y += Math.sin(m.angle) * m.speed;
      
      const tailX = m.x - Math.cos(m.angle) * m.length;
      const tailY = m.y - Math.sin(m.angle) * m.length;
      
      const gradient = ctx.createLinearGradient(tailX, tailY, m.x, m.y);
      gradient.addColorStop(0, 'transparent');
      gradient.addColorStop(0.8, `hsla(${m.hue}, 100%, 70%, ${m.alpha * 0.5})`);
      gradient.addColorStop(1, `hsla(${m.hue}, 100%, 90%, ${m.alpha})`);
      
      ctx.beginPath();
      ctx.moveTo(tailX, tailY);
      ctx.lineTo(m.x, m.y);
      ctx.strokeStyle = gradient;
      ctx.lineWidth = 2;
      ctx.lineCap = 'round';
      ctx.stroke();
      
      // Glow
      ctx.beginPath();
      ctx.arc(m.x, m.y, 4, 0, Math.PI * 2);
      ctx.fillStyle = `hsla(${m.hue}, 100%, 80%, ${m.alpha})`;
      ctx.shadowColor = `hsla(${m.hue}, 100%, 70%, 1)`;
      ctx.shadowBlur = 15;
      ctx.fill();
      ctx.shadowBlur = 0;
      
      if (m.y > h + 100 || m.x < -100) {
        meteors.splice(i, 1);
      }
    }
    
    ctx.restore();
    raf = requestAnimationFrame(render);
  }

  resize();
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
