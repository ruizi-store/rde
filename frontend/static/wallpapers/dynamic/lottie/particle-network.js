/**
 * Lottie 动画 - 粒子连线
 */

export function initParticleNetwork(container) {
  const canvas = document.createElement('canvas');
  canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;';
  container.appendChild(canvas);
  const ctx = canvas.getContext('2d');

  let raf;
  const particles = [];
  const numParticles = 100;
  const connectionDistance = 150;

  function resize() {
    canvas.width = container.clientWidth * devicePixelRatio;
    canvas.height = container.clientHeight * devicePixelRatio;
  }

  function initParticles() {
    const w = container.clientWidth;
    const h = container.clientHeight;
    particles.length = 0;
    for (let i = 0; i < numParticles; i++) {
      particles.push({
        x: Math.random() * w,
        y: Math.random() * h,
        vx: (Math.random() - 0.5) * 0.5,
        vy: (Math.random() - 0.5) * 0.5,
        size: 2 + Math.random() * 2,
        hue: Math.random() * 60 + 180 // Blue to cyan
      });
    }
  }

  function render() {
    const w = container.clientWidth;
    const h = container.clientHeight;
    
    ctx.fillStyle = 'rgba(5, 10, 20, 0.1)';
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    
    ctx.save();
    ctx.scale(devicePixelRatio, devicePixelRatio);
    
    // Update and draw particles
    for (let p of particles) {
      p.x += p.vx;
      p.y += p.vy;
      
      if (p.x < 0 || p.x > w) p.vx *= -1;
      if (p.y < 0 || p.y > h) p.vy *= -1;
      
      ctx.beginPath();
      ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2);
      ctx.fillStyle = `hsla(${p.hue}, 100%, 70%, 0.8)`;
      ctx.fill();
    }
    
    // Draw connections
    for (let i = 0; i < particles.length; i++) {
      for (let j = i + 1; j < particles.length; j++) {
        const dx = particles[i].x - particles[j].x;
        const dy = particles[i].y - particles[j].y;
        const dist = Math.sqrt(dx * dx + dy * dy);
        
        if (dist < connectionDistance) {
          const alpha = (1 - dist / connectionDistance) * 0.5;
          ctx.beginPath();
          ctx.moveTo(particles[i].x, particles[i].y);
          ctx.lineTo(particles[j].x, particles[j].y);
          ctx.strokeStyle = `rgba(100, 200, 255, ${alpha})`;
          ctx.lineWidth = 1;
          ctx.stroke();
        }
      }
    }
    
    ctx.restore();
    raf = requestAnimationFrame(render);
  }

  resize();
  initParticles();
  render();
  window.addEventListener('resize', () => { resize(); initParticles(); });

  return {
    destroy() {
      cancelAnimationFrame(raf);
      canvas.remove();
    }
  };
}
