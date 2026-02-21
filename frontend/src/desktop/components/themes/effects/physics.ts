// 简单物理计算模块

export interface Vector2D {
  x: number;
  y: number;
}

export interface PhysicsBody {
  position: Vector2D;
  velocity: Vector2D;
  acceleration: Vector2D;
  mass: number;
}

// 重力常量
export const GRAVITY = 980; // 像素/秒²

// 阻尼系数
export const DAMPING = 0.98;

// 更新物理体位置
export function updatePhysics(body: PhysicsBody, deltaTime: number): void {
  // 应用加速度到速度
  body.velocity.x += body.acceleration.x * deltaTime;
  body.velocity.y += body.acceleration.y * deltaTime;

  // 应用阻尼
  body.velocity.x *= DAMPING;
  body.velocity.y *= DAMPING;

  // 应用速度到位置
  body.position.x += body.velocity.x * deltaTime;
  body.position.y += body.velocity.y * deltaTime;
}

// 应用重力
export function applyGravity(body: PhysicsBody, gravityMultiplier: number = 1): void {
  body.acceleration.y = GRAVITY * gravityMultiplier;
}

// 检测与边界的碰撞
export function checkBoundaryCollision(position: Vector2D, boundaryY: number): boolean {
  return position.y >= boundaryY;
}

// 生成随机速度
export function randomVelocity(minX: number, maxX: number, minY: number, maxY: number): Vector2D {
  return {
    x: minX + Math.random() * (maxX - minX),
    y: minY + Math.random() * (maxY - minY),
  };
}

// 计算两点之间的距离
export function distance(a: Vector2D, b: Vector2D): number {
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  return Math.sqrt(dx * dx + dy * dy);
}

// 线性插值
export function lerp(start: number, end: number, t: number): number {
  return start + (end - start) * t;
}

// 缓动函数 - easeOutQuad
export function easeOutQuad(t: number): number {
  return t * (2 - t);
}

// 缓动函数 - easeInQuad
export function easeInQuad(t: number): number {
  return t * t;
}

// 缓动函数 - easeOutBounce
export function easeOutBounce(t: number): number {
  const n1 = 7.5625;
  const d1 = 2.75;

  if (t < 1 / d1) {
    return n1 * t * t;
  } else if (t < 2 / d1) {
    return n1 * (t -= 1.5 / d1) * t + 0.75;
  } else if (t < 2.5 / d1) {
    return n1 * (t -= 2.25 / d1) * t + 0.9375;
  } else {
    return n1 * (t -= 2.625 / d1) * t + 0.984375;
  }
}
