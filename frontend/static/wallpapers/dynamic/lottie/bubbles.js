/**
 * Lottie 动画 - 泡泡升腾
 */

export function initBubbles(container) {
  const canvas = document.createElement('canvas');
  canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;';
  container.appendChild(canvas);
  const ctx = canvas.getContext('2d');

  let raf;
  const bubbles = [];

  function resize() {
    canvas.width = container.clientWidth * devicePixelRatio;
    canvas.height = container.clientHeight * devicePixelRatio;
  }

  function spawnBubble() {
    const w = container.clientWidth;
    const h = container.clientHeight;
    bubbles.push({
      x: Math.random() * w,
      y: h + 50,
      size: 10 + Math.random() * 40,
      speed: 0.5 + Math.random() * 1.5,
      wobble: Math.random() * Math.PI * 2,
      wobbleSpeed: 0.02 + Math.random() * 0.03,
      hue: Math.random() * 360
    });
  }

  function render() {
    const w = container.clientWidth;
    const h = container.clientHeight;
    
    // Gradient background
    ctx.save();
    ctx.scale(devicePixelRatio, devicePixelRatio);
    
    const bgGrad = ctx.createLinearGradient(0, 0, 0, h);
    bgGrad.addColorStop(0, '#1a1a3a');
    bgGrad.addColorStop(1, '#0a2a4a');
    ctx.fillStyle = bgGrad;
    ctx.fillRect(0, 0, w, h);
    
    // Spawn bubbles
    if (bubbles.length < 30 && Math.random() < 0.05) {
      spawnBubble();
    }
    
    // Update and draw bubbles
    for (let i = bubbles.length - 1; i >= 0; i--) {
      const b = bubbles[i];
      
      b.y -= b.speed;
      b.wobble += b.wobbleSpeed;
      const wobbleX = Math.sin(b.wobble) * 2;
      b.x += wobbleX;
      
      // Draw bubble
      ctx.beginPath();
      ctx.arc(b.x, b.y, b.size, 0, Math.PI * 2);
      
      // Gradient fill
      const grad = ctx.createRadialGradient(
        b.x - b.size * 0.3, b.y - b.size * 0.3, 0,
        b.x, b.y, b.size
      );
      grad.addColorStop(0, `hsla(${b.hue}, 80%, 80%, 0.3)`);
      grad.addColorStop(0.5, `hsla(${b.hue}, 70%, 60%, 0.2)`);
      grad.addColorStop(1, `hsla(${b.hue}, 60%, 50%, 0.1)`);
      ctx.fillStyle = grad;
      ctx.fill();
      
      // Highlight
      ctx.beginPath();
      ctx.arc(b.x - b.size * 0.3, b.y - b.size * 0.3, b.size * 0.2, 0, Math.PI * 2);
      ctx.fillStyle = 'rgba(255, 255, 255, 0.5)';
      ctx.fill();
      
      // Border
      ctx.beginPath();
      ctx.arc(b.x, b.y, b.size, 0, Math.PI * 2);
      ctx.strokeStyle = `hsla(${b.hue}, 80%, 70%, 0.4)`;
      ctx.lineWidth = 1;
      ctx.stroke();
      
      // Remove if off screen
      if (b.y < -b.size * 2) {
        bubbles.splice(i, 1);
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
