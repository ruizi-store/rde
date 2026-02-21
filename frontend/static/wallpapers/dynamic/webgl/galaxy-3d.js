/**
 * 3D 银河星系动态壁纸 - 纯 WebGL 实现
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

#define PI 3.14159265359

mat2 rotate(float a) {
  float s = sin(a), c = cos(a);
  return mat2(c, -s, s, c);
}

float hash21(vec2 p) {
  p = fract(p * vec2(234.34, 435.345));
  p += dot(p, p + 34.23);
  return fract(p.x * p.y);
}

float star(vec2 uv, float flare) {
  float d = length(uv);
  float m = 0.05 / d;
  float rays = max(0.0, 1.0 - abs(uv.x * uv.y * 1000.0));
  m += rays * flare;
  uv *= rotate(PI / 4.0);
  rays = max(0.0, 1.0 - abs(uv.x * uv.y * 1000.0));
  m += rays * 0.3 * flare;
  m *= smoothstep(1.0, 0.2, d);
  return m;
}

vec3 starLayer(vec2 uv, float t) {
  vec3 col = vec3(0.0);
  vec2 gv = fract(uv) - 0.5;
  vec2 id = floor(uv);
  
  for (int y = -1; y <= 1; y++) {
    for (int x = -1; x <= 1; x++) {
      vec2 offs = vec2(float(x), float(y));
      float n = hash21(id + offs);
      float size = fract(n * 345.32);
      float sv = star(gv - offs - vec2(n, fract(n * 34.0)) + 0.5, smoothstep(0.9, 1.0, size) * 0.6);
      vec3 color = sin(vec3(0.2, 0.3, 0.9) * fract(n * 2345.2) * 123.2) * 0.5 + 0.5;
      color = color * vec3(1.0, 0.5, 1.0 + size) + vec3(0.2, 0.2, 0.1);
      sv *= sin(t * 3.0 + n * 6.28) * 0.5 + 1.0;
      col += sv * size * color;
    }
  }
  return col;
}

vec3 galaxyCore(vec2 uv, float t) {
  float d = length(uv);
  vec3 col = vec3(0.0);
  col += vec3(1.0, 0.8, 0.6) * 0.1 / (d + 0.1);
  col += vec3(0.8, 0.4, 1.0) * 0.05 / (d * d + 0.05);
  float angle = atan(uv.y, uv.x);
  float spiral = sin(angle * 3.0 - d * 10.0 + t) * 0.5 + 0.5;
  spiral *= smoothstep(0.8, 0.1, d) * smoothstep(0.0, 0.1, d);
  col += vec3(0.5, 0.3, 1.0) * spiral * 0.3;
  return col;
}

void main() {
  vec2 uv = (gl_FragCoord.xy - 0.5 * u_resolution) / min(u_resolution.x, u_resolution.y);
  float t = u_time * 0.3;
  
  vec3 col = vec3(0.02, 0.01, 0.05);
  
  vec2 galaxyUV = uv * rotate(t * 0.1);
  col += galaxyCore(galaxyUV, t);
  
  for (float i = 0.0; i < 4.0; i++) {
    float depth = fract(i / 4.0 + t * 0.05);
    float scale = mix(10.0, 0.5, depth);
    float fade = depth * smoothstep(1.0, 0.9, depth);
    vec2 layerUV = uv * scale * rotate(i * 0.5 + t * 0.1);
    layerUV += vec2(i * 12.34, i * 56.78);
    col += starLayer(layerUV, t) * fade;
  }
  
  gl_FragColor = vec4(col, 1.0);
}`;

export function initGalaxy(canvas) {
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
