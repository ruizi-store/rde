/**
 * 极光动画动态壁纸 - 纯 WebGL 实现
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

float noise(vec2 p) {
  return fract(sin(dot(p, vec2(12.9898, 78.233))) * 43758.5453);
}

float smoothNoise(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  f = f * f * (3.0 - 2.0 * f);
  
  float a = noise(i);
  float b = noise(i + vec2(1.0, 0.0));
  float c = noise(i + vec2(0.0, 1.0));
  float d = noise(i + vec2(1.0, 1.0));
  
  return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
}

float fbm(vec2 p) {
  float v = 0.0;
  float a = 0.5;
  for (int i = 0; i < 5; i++) {
    v += a * smoothNoise(p);
    p *= 2.0;
    a *= 0.5;
  }
  return v;
}

vec3 aurora(vec2 uv, float t) {
  vec3 col = vec3(0.0);
  
  for (float i = 0.0; i < 5.0; i++) {
    float offset = i * 0.1;
    vec2 p = uv;
    p.x += sin(uv.y * 3.0 + t + i) * 0.2;
    p.y += fbm(vec2(p.x * 2.0 + t * 0.5, i)) * 0.3;
    
    float wave = sin(p.x * 5.0 + t * (1.0 + i * 0.2) + i * 2.0) * 0.5 + 0.5;
    wave *= smoothstep(0.3, 0.7, uv.y + offset);
    wave *= smoothstep(1.0, 0.5, uv.y + offset);
    
    float intensity = wave * 0.3;
    intensity *= 1.0 + 0.5 * sin(t * 2.0 + i * 1.5);
    
    vec3 auroraCol;
    if (i < 2.0) {
      auroraCol = mix(vec3(0.0, 1.0, 0.5), vec3(0.0, 0.5, 1.0), wave);
    } else if (i < 4.0) {
      auroraCol = mix(vec3(0.5, 0.0, 1.0), vec3(1.0, 0.0, 0.5), wave);
    } else {
      auroraCol = mix(vec3(0.0, 1.0, 1.0), vec3(0.0, 1.0, 0.3), wave);
    }
    
    col += auroraCol * intensity;
  }
  
  return col;
}

void main() {
  vec2 uv = gl_FragCoord.xy / u_resolution;
  float t = u_time * 0.3;
  
  // Night sky gradient
  vec3 col = mix(vec3(0.0, 0.02, 0.05), vec3(0.0, 0.0, 0.02), uv.y);
  
  // Stars
  for (float i = 0.0; i < 3.0; i++) {
    vec2 starUV = uv * (100.0 + i * 50.0);
    float star = step(0.998, noise(floor(starUV)));
    star *= 0.5 + 0.5 * sin(t * 3.0 + noise(floor(starUV)) * 10.0);
    col += vec3(1.0) * star * (0.5 + i * 0.2);
  }
  
  // Aurora
  col += aurora(uv, t);
  
  // Ground silhouette
  float ground = smoothstep(0.15, 0.1, uv.y + fbm(vec2(uv.x * 5.0, 0.0)) * 0.05);
  col = mix(col, vec3(0.0, 0.02, 0.02), ground);
  
  gl_FragColor = vec4(col, 1.0);
}`;

export function initAurora(canvas) {
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
