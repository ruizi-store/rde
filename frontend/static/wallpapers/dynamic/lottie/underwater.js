/**
 * 海底世界 - 鱼儿悠闲游动
 * 使用 Boids 算法模拟自然鱼群行为
 */

export function initUnderwater(container) {
  const canvas = document.createElement('canvas');
  canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;';
  container.appendChild(canvas);
  const ctx = canvas.getContext('2d');

  let raf;
  let width, height;
  
  // 鱼群
  const fishes = [];
  const numFish = 25;
  
  // 气泡
  const bubbles = [];
  const numBubbles = 30;
  
  // 海草
  const seaweeds = [];
  const numSeaweed = 12;
  
  // 光线
  const lightRays = [];
  const numRays = 5;

  function resize() {
    width = container.clientWidth;
    height = container.clientHeight;
    canvas.width = width * devicePixelRatio;
    canvas.height = height * devicePixelRatio;
    ctx.scale(devicePixelRatio, devicePixelRatio);
    
    // 初始化海草位置
    seaweeds.length = 0;
    for (let i = 0; i < numSeaweed; i++) {
      seaweeds.push({
        x: (i + 0.5) * (width / numSeaweed) + (Math.random() - 0.5) * 50,
        height: 80 + Math.random() * 120,
        width: 8 + Math.random() * 6,
        phase: Math.random() * Math.PI * 2,
        speed: 0.5 + Math.random() * 0.5,
        color: `hsl(${140 + Math.random() * 30}, ${60 + Math.random() * 20}%, ${25 + Math.random() * 15}%)`
      });
    }
    
    // 初始化光线
    lightRays.length = 0;
    for (let i = 0; i < numRays; i++) {
      lightRays.push({
        x: Math.random() * width,
        width: 30 + Math.random() * 60,
        opacity: 0.03 + Math.random() * 0.05,
        speed: 0.2 + Math.random() * 0.3
      });
    }
  }

  // 鱼类
  class Fish {
    constructor() {
      this.x = Math.random() * width;
      this.y = Math.random() * height * 0.7 + height * 0.1;
      this.vx = (Math.random() - 0.5) * 2;
      this.vy = (Math.random() - 0.5) * 1;
      this.size = 12 + Math.random() * 18;
      this.tailPhase = Math.random() * Math.PI * 2;
      this.tailSpeed = 0.15 + Math.random() * 0.1;
      
      // 随机鱼的颜色 - 热带鱼风格
      const colorTypes = [
        { body: '#FF6B35', fin: '#FFE66D', stripe: '#2E86AB' },  // 橙色小丑鱼
        { body: '#4ECDC4', fin: '#45B7D1', stripe: '#96E6A1' },  // 青色热带鱼
        { body: '#DDA0DD', fin: '#E6E6FA', stripe: '#9370DB' },  // 紫色神仙鱼
        { body: '#FFD93D', fin: '#FF6B6B', stripe: '#4ECDC4' },  // 黄色蝴蝶鱼
        { body: '#6BCB77', fin: '#4D96FF', stripe: '#FFD93D' },  // 绿色鹦鹉鱼
        { body: '#FF8C94', fin: '#FFEAA7', stripe: '#DFE6E9' },  // 粉色小鱼
      ];
      this.colors = colorTypes[Math.floor(Math.random() * colorTypes.length)];
    }

    update(fishes) {
      // Boids 算法参数
      const separationDist = 40;
      const alignmentDist = 80;
      const cohesionDist = 100;
      
      let sepX = 0, sepY = 0, sepCount = 0;
      let alignX = 0, alignY = 0, alignCount = 0;
      let cohX = 0, cohY = 0, cohCount = 0;

      for (const other of fishes) {
        if (other === this) continue;
        const dx = other.x - this.x;
        const dy = other.y - this.y;
        const dist = Math.sqrt(dx * dx + dy * dy);

        // 分离
        if (dist < separationDist && dist > 0) {
          sepX -= dx / dist;
          sepY -= dy / dist;
          sepCount++;
        }
        // 对齐
        if (dist < alignmentDist) {
          alignX += other.vx;
          alignY += other.vy;
          alignCount++;
        }
        // 聚合
        if (dist < cohesionDist) {
          cohX += other.x;
          cohY += other.y;
          cohCount++;
        }
      }

      // 应用 Boids 力
      if (sepCount > 0) {
        this.vx += sepX / sepCount * 0.05;
        this.vy += sepY / sepCount * 0.05;
      }
      if (alignCount > 0) {
        this.vx += (alignX / alignCount - this.vx) * 0.02;
        this.vy += (alignY / alignCount - this.vy) * 0.02;
      }
      if (cohCount > 0) {
        const targetX = cohX / cohCount;
        const targetY = cohY / cohCount;
        this.vx += (targetX - this.x) * 0.0005;
        this.vy += (targetY - this.y) * 0.0005;
      }

      // 边界避让（柔和转向）
      const margin = 100;
      if (this.x < margin) this.vx += 0.1;
      if (this.x > width - margin) this.vx -= 0.1;
      if (this.y < margin) this.vy += 0.05;
      if (this.y > height - margin) this.vy -= 0.05;

      // 限制速度
      const maxSpeed = 1.5;
      const minSpeed = 0.3;
      const speed = Math.sqrt(this.vx * this.vx + this.vy * this.vy);
      if (speed > maxSpeed) {
        this.vx = (this.vx / speed) * maxSpeed;
        this.vy = (this.vy / speed) * maxSpeed;
      } else if (speed < minSpeed) {
        this.vx = (this.vx / speed) * minSpeed;
        this.vy = (this.vy / speed) * minSpeed;
      }

      // 更新位置
      this.x += this.vx;
      this.y += this.vy;

      // 环绕边界
      if (this.x < -50) this.x = width + 50;
      if (this.x > width + 50) this.x = -50;
      if (this.y < -50) this.y = height + 50;
      if (this.y > height + 50) this.y = -50;

      // 尾巴摆动
      this.tailPhase += this.tailSpeed;
    }

    draw(ctx) {
      const angle = Math.atan2(this.vy, this.vx);
      const tailWag = Math.sin(this.tailPhase) * 0.3;

      ctx.save();
      ctx.translate(this.x, this.y);
      ctx.rotate(angle);

      const s = this.size;

      // 尾鳍
      ctx.fillStyle = this.colors.fin;
      ctx.beginPath();
      ctx.moveTo(-s * 0.3, 0);
      ctx.quadraticCurveTo(-s * 0.8, -s * 0.4 + tailWag * s * 0.3, -s * 0.9, -s * 0.3 + tailWag * s * 0.5);
      ctx.quadraticCurveTo(-s * 0.6, tailWag * s * 0.2, -s * 0.9, s * 0.3 + tailWag * s * 0.5);
      ctx.quadraticCurveTo(-s * 0.8, s * 0.4 + tailWag * s * 0.3, -s * 0.3, 0);
      ctx.fill();

      // 身体
      ctx.fillStyle = this.colors.body;
      ctx.beginPath();
      ctx.ellipse(0, 0, s * 0.5, s * 0.3, 0, 0, Math.PI * 2);
      ctx.fill();

      // 条纹
      ctx.fillStyle = this.colors.stripe;
      ctx.globalAlpha = 0.6;
      ctx.beginPath();
      ctx.ellipse(s * 0.1, 0, s * 0.08, s * 0.25, 0, 0, Math.PI * 2);
      ctx.fill();
      ctx.globalAlpha = 1;

      // 背鳍
      ctx.fillStyle = this.colors.fin;
      ctx.beginPath();
      ctx.moveTo(-s * 0.1, -s * 0.28);
      ctx.quadraticCurveTo(s * 0.05, -s * 0.5, s * 0.2, -s * 0.28);
      ctx.fill();

      // 眼睛
      ctx.fillStyle = '#fff';
      ctx.beginPath();
      ctx.arc(s * 0.25, -s * 0.05, s * 0.1, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillStyle = '#1a1a2e';
      ctx.beginPath();
      ctx.arc(s * 0.28, -s * 0.05, s * 0.05, 0, Math.PI * 2);
      ctx.fill();

      ctx.restore();
    }
  }

  // 气泡类
  class Bubble {
    constructor() {
      this.reset();
    }

    reset() {
      this.x = Math.random() * width;
      this.y = height + Math.random() * 50;
      this.size = 2 + Math.random() * 8;
      this.speed = 0.3 + Math.random() * 0.7;
      this.wobble = Math.random() * Math.PI * 2;
      this.wobbleSpeed = 0.02 + Math.random() * 0.03;
    }

    update() {
      this.y -= this.speed;
      this.wobble += this.wobbleSpeed;
      this.x += Math.sin(this.wobble) * 0.5;

      if (this.y < -20) {
        this.reset();
      }
    }

    draw(ctx) {
      ctx.save();
      ctx.globalAlpha = 0.4;
      ctx.strokeStyle = '#fff';
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.arc(this.x, this.y, this.size, 0, Math.PI * 2);
      ctx.stroke();
      
      // 高光
      ctx.globalAlpha = 0.6;
      ctx.fillStyle = '#fff';
      ctx.beginPath();
      ctx.arc(this.x - this.size * 0.3, this.y - this.size * 0.3, this.size * 0.2, 0, Math.PI * 2);
      ctx.fill();
      ctx.restore();
    }
  }

  // 绘制海草
  function drawSeaweed(sw, time) {
    ctx.save();
    ctx.fillStyle = sw.color;
    
    const segments = 8;
    const segHeight = sw.height / segments;
    
    ctx.beginPath();
    ctx.moveTo(sw.x - sw.width / 2, height);
    
    // 左边缘
    for (let i = 0; i <= segments; i++) {
      const y = height - i * segHeight;
      const wave = Math.sin(time * sw.speed + sw.phase + i * 0.5) * (i * 3);
      ctx.lineTo(sw.x - sw.width / 2 + wave, y);
    }
    
    // 顶部
    const topWave = Math.sin(time * sw.speed + sw.phase + segments * 0.5) * (segments * 3);
    ctx.quadraticCurveTo(
      sw.x + topWave, height - sw.height - 10,
      sw.x + sw.width / 2 + topWave, height - sw.height
    );
    
    // 右边缘
    for (let i = segments; i >= 0; i--) {
      const y = height - i * segHeight;
      const wave = Math.sin(time * sw.speed + sw.phase + i * 0.5) * (i * 3);
      ctx.lineTo(sw.x + sw.width / 2 + wave, y);
    }
    
    ctx.closePath();
    ctx.fill();
    ctx.restore();
  }

  // 绘制光线
  function drawLightRays(time) {
    ctx.save();
    for (const ray of lightRays) {
      const x = ray.x + Math.sin(time * ray.speed) * 30;
      const gradient = ctx.createLinearGradient(x, 0, x + ray.width, height);
      gradient.addColorStop(0, `rgba(255, 255, 200, ${ray.opacity})`);
      gradient.addColorStop(1, 'rgba(255, 255, 200, 0)');
      
      ctx.fillStyle = gradient;
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x + ray.width, 0);
      ctx.lineTo(x + ray.width * 1.5, height);
      ctx.lineTo(x - ray.width * 0.5, height);
      ctx.closePath();
      ctx.fill();
    }
    ctx.restore();
  }

  // 绘制背景渐变
  function drawBackground() {
    const gradient = ctx.createLinearGradient(0, 0, 0, height);
    gradient.addColorStop(0, '#0a4d68');
    gradient.addColorStop(0.3, '#0e6377');
    gradient.addColorStop(0.6, '#088395');
    gradient.addColorStop(1, '#05445e');
    ctx.fillStyle = gradient;
    ctx.fillRect(0, 0, width, height);
  }

  // 绘制沙底
  function drawSandBottom() {
    const gradient = ctx.createLinearGradient(0, height - 50, 0, height);
    gradient.addColorStop(0, 'rgba(194, 178, 128, 0)');
    gradient.addColorStop(0.5, 'rgba(194, 178, 128, 0.3)');
    gradient.addColorStop(1, 'rgba(194, 178, 128, 0.5)');
    ctx.fillStyle = gradient;
    ctx.fillRect(0, height - 50, width, 50);
  }

  // 初始化
  function init() {
    resize();
    
    // 创建鱼群
    fishes.length = 0;
    for (let i = 0; i < numFish; i++) {
      fishes.push(new Fish());
    }
    
    // 创建气泡
    bubbles.length = 0;
    for (let i = 0; i < numBubbles; i++) {
      const b = new Bubble();
      b.y = Math.random() * height; // 初始分布在整个屏幕
      bubbles.push(b);
    }
  }

  let time = 0;
  function animate() {
    time += 0.016;
    
    // 背景
    drawBackground();
    
    // 光线
    drawLightRays(time);
    
    // 海草（后层）
    for (const sw of seaweeds) {
      drawSeaweed(sw, time);
    }
    
    // 沙底
    drawSandBottom();
    
    // 气泡
    for (const bubble of bubbles) {
      bubble.update();
      bubble.draw(ctx);
    }
    
    // 鱼群
    for (const fish of fishes) {
      fish.update(fishes);
    }
    // 按 y 坐标排序实现简单的深度
    fishes.sort((a, b) => a.y - b.y);
    for (const fish of fishes) {
      fish.draw(ctx);
    }

    raf = requestAnimationFrame(animate);
  }

  init();
  animate();

  window.addEventListener('resize', () => {
    resize();
    // 重新分布鱼的位置
    for (const fish of fishes) {
      if (fish.x > width) fish.x = width * 0.8;
      if (fish.y > height) fish.y = height * 0.8;
    }
  });

  return {
    destroy() {
      cancelAnimationFrame(raf);
      canvas.remove();
    }
  };
}
