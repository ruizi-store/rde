/**
 * Lottie 动画 - 霓虹线条
 */

export function initNeonLines(container) {
  const canvas = document.createElement('canvas');
  canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;';
  container.appendChild(canvas);
  const ctx = canvas.getContext('2d');

  let raf;
  const lines = [];
  const numLines = 20;
  let time = 0;

  function resize() {
    canvas.width = container.clientWidth * devicePixelRatio;
    canvas.height = container.clientHeight * devicePixelRatio;
  }

  function initLines() {
    const h = container.clientHeight;
    for (let i = 0; i < numLines; i++) {
      lines.push({
        y: Math.random() * h,
        speed: 0.5 + Math.random() * 2,
        hue: Math.random() * 360,
        width: 1 + Math.random() * 3,
        amplitude: 20 + Math.random() * 40,
        frequency: 0.01 + Math.random() * 0.02,
        phase: Math.random() * Math.PI * 2
      });
    }
  }

  function render() {
    const w = container.clientWidth;
    const h = container.clientHeight;
    time += 0.02;
    
    ctx.fillStyle = 'rgba(10, 5, 20, 0.1)';
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    
    ctx.save();
    ctx.scale(devicePixelRatio, devicePixelRatio);
    
    for (let line of lines) {
      ctx.beginPath();
      
      for (let x = 0; x < w; x += 5) {
        const y = line.y + Math.sin(x * line.frequency + time * line.speed + line.phase) * line.amplitude;
        if (x === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      
      const hue = (line.hue + time * 20) % 360;
      ctx.strokeStyle = `hsla(${hue}, 100%, 60%, 0.8)`;
      ctx.lineWidth = line.width;
      ctx.shadowColor = `hsla(${hue}, 100%, 60%, 1)`;
      ctx.shadowBlur = 20;
      ctx.stroke();
    }
    
    ctx.restore();
    raf = requestAnimationFrame(render);
  }

  resize();
  initLines();
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
