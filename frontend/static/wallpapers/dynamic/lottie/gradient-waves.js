/**
 * Lottie 动画 - 波浪渐变
 */

export function initGradientWaves(container) {
  const canvas = document.createElement('canvas');
  canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;';
  container.appendChild(canvas);
  const ctx = canvas.getContext('2d');

  let raf;
  let time = 0;

  function resize() {
    canvas.width = container.clientWidth * devicePixelRatio;
    canvas.height = container.clientHeight * devicePixelRatio;
  }

  function render() {
    const w = container.clientWidth;
    const h = container.clientHeight;
    time += 0.01;
    
    ctx.save();
    ctx.scale(devicePixelRatio, devicePixelRatio);
    
    // Background gradient
    const bgGrad = ctx.createLinearGradient(0, 0, w, h);
    bgGrad.addColorStop(0, `hsl(${(time * 20) % 360}, 70%, 10%)`);
    bgGrad.addColorStop(1, `hsl(${(time * 20 + 60) % 360}, 70%, 15%)`);
    ctx.fillStyle = bgGrad;
    ctx.fillRect(0, 0, w, h);
    
    // Draw waves
    const waveCount = 5;
    for (let i = 0; i < waveCount; i++) {
      ctx.beginPath();
      
      const baseY = h * (0.4 + i * 0.12);
      const amplitude = 30 + i * 10;
      const frequency = 0.005 - i * 0.0005;
      const speed = 1 + i * 0.3;
      
      ctx.moveTo(0, h);
      
      for (let x = 0; x <= w; x += 5) {
        const y = baseY + Math.sin(x * frequency + time * speed) * amplitude +
                  Math.sin(x * frequency * 2 + time * speed * 1.5) * amplitude * 0.5;
        ctx.lineTo(x, y);
      }
      
      ctx.lineTo(w, h);
      ctx.closePath();
      
      const hue = (time * 30 + i * 40) % 360;
      const grad = ctx.createLinearGradient(0, baseY - amplitude, 0, h);
      grad.addColorStop(0, `hsla(${hue}, 80%, 50%, 0.6)`);
      grad.addColorStop(1, `hsla(${hue + 30}, 80%, 30%, 0.3)`);
      ctx.fillStyle = grad;
      ctx.fill();
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
