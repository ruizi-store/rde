/**
 * Lottie 壁纸渲染器 - 呼吸光环
 * 优雅的呼吸动画效果
 */

/**
 * 初始化呼吸光环动画
 * @param {HTMLElement} container - 容器元素
 * @returns {Object} - 清理函数
 */
export function initBreathingRings(container) {
  // 创建 SVG 容器
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('viewBox', '0 0 1920 1080');
  svg.setAttribute('preserveAspectRatio', 'xMidYMid slice');
  svg.style.cssText = `
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    background: linear-gradient(135deg, #0c0c1e 0%, #1a1a3a 50%, #0a0a20 100%);
  `;
  
  // 定义渐变
  const defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs');
  
  const colors = [
    { id: 'grad1', stops: ['#4facfe', '#00f2fe'] },
    { id: 'grad2', stops: ['#fa709a', '#fee140'] },
    { id: 'grad3', stops: ['#a18cd1', '#fbc2eb'] },
    { id: 'grad4', stops: ['#667eea', '#764ba2'] },
    { id: 'grad5', stops: ['#f093fb', '#f5576c'] },
  ];
  
  colors.forEach(({ id, stops }) => {
    const gradient = document.createElementNS('http://www.w3.org/2000/svg', 'radialGradient');
    gradient.setAttribute('id', id);
    gradient.innerHTML = `
      <stop offset="0%" stop-color="${stops[0]}" stop-opacity="0.8"/>
      <stop offset="100%" stop-color="${stops[1]}" stop-opacity="0"/>
    `;
    defs.appendChild(gradient);
  });
  
  svg.appendChild(defs);
  
  // 创建呼吸光环
  const rings = [];
  const ringConfigs = [
    { cx: 960, cy: 540, baseR: 100, maxR: 400, duration: 8, delay: 0, gradient: 'grad1' },
    { cx: 400, cy: 300, baseR: 80, maxR: 300, duration: 10, delay: 2, gradient: 'grad2' },
    { cx: 1500, cy: 700, baseR: 90, maxR: 350, duration: 9, delay: 1, gradient: 'grad3' },
    { cx: 300, cy: 800, baseR: 70, maxR: 280, duration: 11, delay: 3, gradient: 'grad4' },
    { cx: 1600, cy: 200, baseR: 85, maxR: 320, duration: 7, delay: 4, gradient: 'grad5' },
    { cx: 960, cy: 540, baseR: 150, maxR: 500, duration: 12, delay: 2, gradient: 'grad1' },
  ];
  
  ringConfigs.forEach((config, i) => {
    const circle = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
    circle.setAttribute('cx', config.cx);
    circle.setAttribute('cy', config.cy);
    circle.setAttribute('r', config.baseR);
    circle.setAttribute('fill', `url(#${config.gradient})`);
    circle.style.mixBlendMode = 'screen';
    svg.appendChild(circle);
    
    rings.push({
      element: circle,
      config,
      phase: config.delay * Math.PI / 4
    });
  });
  
  container.appendChild(svg);
  
  // 动画循环
  let animationId;
  let startTime = performance.now();
  
  function animate(time) {
    const elapsed = (time - startTime) / 1000;
    
    rings.forEach(ring => {
      const { element, config, phase } = ring;
      const progress = (Math.sin(elapsed * (2 * Math.PI / config.duration) + phase) + 1) / 2;
      const r = config.baseR + (config.maxR - config.baseR) * progress;
      const opacity = 0.3 + 0.5 * (1 - progress);
      
      element.setAttribute('r', r);
      element.style.opacity = opacity;
    });
    
    animationId = requestAnimationFrame(animate);
  }
  
  animationId = requestAnimationFrame(animate);
  
  return {
    destroy() {
      cancelAnimationFrame(animationId);
      svg.remove();
    }
  };
}

export default { initBreathingRings };
