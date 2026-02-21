/**
 * 赛博朋克城市动态壁纸 - 纯 WebGL 实现
 */

const VERTEX_SHADER = `
attribute vec2 a_position;
void main() {
  gl_Position = vec4(a_position, 0.0, 1.0);
}`;

const FRAGMENT_SHADER = `
precision highp float;
uniform float u_time;
uniform vec2 u_resolution;

float hash(vec2 p) {
  return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
}

float noise(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  f = f * f * (3.0 - 2.0 * f);
  return mix(
    mix(hash(i), hash(i + vec2(1.0, 0.0)), f.x),
    mix(hash(i + vec2(0.0, 1.0)), hash(i + vec2(1.0, 1.0)), f.x),
    f.y
  );
}

float building(vec2 p, float seed) {
  float h = hash(vec2(floor(p.x * 4.0), seed)) * 0.5 + 0.3;
  float w = 0.15 + hash(vec2(floor(p.x * 4.0) + 100.0, seed)) * 0.1;
  vec2 bp = vec2(fract(p.x * 4.0) - 0.5, p.y);
  
  float b = step(abs(bp.x), w) * step(0.0, bp.y) * step(bp.y, h);
  
  // Windows
  if (b > 0.0) {
    vec2 wp = bp * vec2(20.0, 30.0);
    float win = step(0.3, fract(wp.x)) * step(fract(wp.x), 0.7);
    win *= step(0.2, fract(wp.y)) * step(fract(wp.y), 0.8);
    float on = step(0.5, hash(floor(wp) + seed));
    b = mix(0.1, 0.8, win * on);
  }
  
  return b;
}

vec3 neonGlow(vec2 uv, float t) {
  vec3 col = vec3(0.0);
  
  // Horizontal lines
  for (float i = 0.0; i < 5.0; i++) {
    float y = 0.2 + i * 0.15;
    float glow = 0.002 / abs(uv.y - y + sin(uv.x * 10.0 + t + i) * 0.02);
    vec3 neonCol = 0.5 + 0.5 * cos(vec3(0.0, 2.0, 4.0) + i * 1.5 + t);
    col += neonCol * glow;
  }
  
  return col;
}

void main() {
  vec2 uv = gl_FragCoord.xy / u_resolution;
  float t = u_time * 0.5;
  
  vec3 col = vec3(0.0);
  
  // Sky gradient
  col = mix(vec3(0.0, 0.0, 0.1), vec3(0.1, 0.0, 0.2), uv.y);
  
  // Rain
  float rain = 0.0;
  for (float i = 0.0; i < 3.0; i++) {
    vec2 ruv = uv * vec2(50.0 + i * 20.0, 1.0);
    ruv.y += t * (5.0 + i * 2.0);
    float r = step(0.98, hash(floor(ruv)));
    r *= smoothstep(0.0, 0.5, fract(ruv.y));
    rain += r * 0.1;
  }
  col += vec3(0.5, 0.5, 0.8) * rain;
  
  // Buildings
  float b = 0.0;
  for (float i = 0.0; i < 3.0; i++) {
    float layer = building(uv * (1.0 + i * 0.5) + vec2(i * 10.0, 0.0), i);
    float depth = 1.0 - i * 0.3;
    b = max(b, layer * depth);
  }
  
  vec3 buildingCol = mix(vec3(0.02, 0.02, 0.05), vec3(1.0, 0.8, 0.3), b);
  col = mix(col, buildingCol, step(0.01, b));
  
  // Neon glow
  col += neonGlow(uv, t) * 0.5;
  
  // Scan lines
  col *= 0.9 + 0.1 * sin(uv.y * u_resolution.y * 2.0);
  
  // Vignette
  col *= 1.0 - 0.5 * length(uv - 0.5);
  
  gl_FragColor = vec4(col, 1.0);
}`;

export function initCyberpunk(canvas) {
  const gl = canvas.getContext('webgl', { alpha: true, antialias: true });
  if (!gl) return null;

  function compile(type, src) {
    const s = gl.createShader(type);
    gl.shaderSource(s, src);
    gl.compileShader(s);
    if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) {
      console.error(gl.getShaderInfoLog(s));
      return null;
    }
    return s;
  }

  const vs = compile(gl.VERTEX_SHADER, VERTEX_SHADER);
  const fs = compile(gl.FRAGMENT_SHADER, FRAGMENT_SHADER);
  if (!vs || !fs) return null;

  const prog = gl.createProgram();
  gl.attachShader(prog, vs);
  gl.attachShader(prog, fs);
  gl.linkProgram(prog);

  const buf = gl.createBuffer();
  gl.bindBuffer(gl.ARRAY_BUFFER, buf);
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1,-1,1,-1,-1,1,-1,1,1,-1,1,1]), gl.STATIC_DRAW);

  const posLoc = gl.getAttribLocation(prog, 'a_position');
  const timeLoc = gl.getUniformLocation(prog, 'u_time');
  const resLoc = gl.getUniformLocation(prog, 'u_resolution');

  let raf, start = Date.now();

  function resize() {
    const dpr = devicePixelRatio || 1;
    canvas.width = canvas.clientWidth * dpr;
    canvas.height = canvas.clientHeight * dpr;
    gl.viewport(0, 0, canvas.width, canvas.height);
  }

  function render() {
    gl.useProgram(prog);
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.enableVertexAttribArray(posLoc);
    gl.vertexAttribPointer(posLoc, 2, gl.FLOAT, false, 0, 0);
    gl.uniform1f(timeLoc, (Date.now() - start) / 1000);
    gl.uniform2f(resLoc, canvas.width, canvas.height);
    gl.drawArrays(gl.TRIANGLES, 0, 6);
    raf = requestAnimationFrame(render);
  }

  window.addEventListener('resize', resize);
  resize();
  render();

  return {
    destroy() {
      cancelAnimationFrame(raf);
      window.removeEventListener('resize', resize);
    }
  };
}
