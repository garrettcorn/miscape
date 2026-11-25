import { buildAtlas, TILE } from './atlas.js';
import { LIGHT_COLORS } from './blocks.js';
import { perspective, lookAt, mul, frustumPlanes, aabbVisible } from './mat.js';

const MAX_LIGHTS = 16;
const MAX_INSTANCES = 4096;

const COMMON = `
uniform vec3 uCam; uniform vec3 uFogDay; uniform vec3 uFogGloam; uniform float uFogNear; uniform float uFogFar; uniform float uGloamX;
uniform vec4 uLights[${MAX_LIGHTS}]; uniform vec3 uLightCol[${MAX_LIGHTS}]; uniform int uNumLights; uniform vec4 uPlayerLight;
float gloamF(float x) { return smoothstep(uGloamX - 3.0, uGloamX + 4.0, x); }
vec3 pointLights(vec3 p) {
  vec3 add = vec3(0.0);
  for (int i = 0; i < ${MAX_LIGHTS}; i++) {
    if (i >= uNumLights) break;
    vec3 d = uLights[i].xyz - p; float r = uLights[i].w;
    float a = max(0.0, 1.0 - dot(d, d) / (r * r));
    add += uLightCol[i] * a * a * 0.55;
  }
  if (uPlayerLight.w > 0.0) {
    vec3 d = uPlayerLight.xyz - p; float r = uPlayerLight.w;
    float a = max(0.0, 1.0 - dot(d, d) / (r * r));
    add += vec3(1.0, 0.82, 0.55) * a * a;
  }
  return min(add, vec3(1.25));
}
vec3 applyFog(vec3 col, vec3 p, float g) {
  float dist = length(p - uCam);
  float f = smoothstep(mix(uFogNear, uFogNear * 0.25, g), mix(uFogFar, uFogFar * 0.5, g), dist);
  return mix(col, mix(uFogDay, uFogGloam, g), f);
}`;

const TERRAIN_VS = `#version 300 es
layout(location=0) in vec4 a0;
layout(location=1) in vec4 a1;
uniform mat4 uVP; uniform vec3 uOrigin; uniform float uScale; uniform float uTime; uniform float uSway;
out vec3 vWorld; out vec2 vUV; flat out float vLayer; out float vLight; flat out int vFlags;
void main() {
  vec3 p = uOrigin + a0.xyz * 0.25 * uScale;
  int flags = int(a1.w + 0.5);
  if ((flags & 8) != 0) p.x += sin(uTime * 1.7 + p.z * 0.7 + p.y) * 0.035 * uSway;
  if ((flags & 2) != 0) p.y += (sin(uTime * 1.3 + p.x * 0.9 + p.z * 0.7) * 0.03 - 0.02) * uSway;
  vWorld = p; vUV = a1.xy * 0.25; vLayer = a1.z; vLight = a0.w / 255.0; vFlags = flags;
  gl_Position = uVP * vec4(p, 1.0);
}`;
const TERRAIN_FS = `#version 300 es
precision highp float; precision highp sampler2DArray;
uniform sampler2DArray uTex;
${COMMON}
in vec3 vWorld; in vec2 vUV; flat in float vLayer; in float vLight; flat in int vFlags;
out vec4 o;
void main() {
  vec4 t = texture(uTex, vec3(vUV, vLayer));
  if ((vFlags & 4) != 0 && t.a < 0.5) discard;
  float g = gloamF(vWorld.x);
  vec3 ambient = mix(vec3(1.0, 0.97, 0.9), vec3(0.24, 0.21, 0.38), g);
  vec3 col;
  if ((vFlags & 1) != 0) col = t.rgb * mix(1.05, 1.35, g);
  else col = t.rgb * vLight * (ambient + pointLights(vWorld) * (0.3 + g * 1.1));
  float alpha = (vFlags & 2) != 0 ? 0.74 : 1.0;
  o = vec4(applyFog(col, vWorld, g), alpha);
}`;

const BOX_VS = `#version 300 es
layout(location=0) in vec3 aPos;
layout(location=1) in vec3 aNorm;
layout(location=2) in vec4 iM0; layout(location=3) in vec4 iM1; layout(location=4) in vec4 iM2; layout(location=5) in vec4 iM3;
layout(location=6) in vec4 iCol;
uniform mat4 uVP;
out vec3 vWorld; out vec3 vN; out vec4 vCol;
void main() {
  mat4 M = mat4(iM0, iM1, iM2, iM3);
  vec4 w = M * vec4(aPos, 1.0);
  vWorld = w.xyz; vN = normalize(mat3(M) * aNorm); vCol = iCol;
  gl_Position = uVP * w;
}`;
const BOX_FS = `#version 300 es
precision highp float;
${COMMON}
in vec3 vWorld; in vec3 vN; in vec4 vCol;
out vec4 o;
void main() {
  float g = gloamF(vWorld.x);
  vec3 col;
  if (vCol.a > 1.5) col = vCol.rgb * mix(1.1, 1.5, g);           // emissive
  else {
    float sun = 0.6 + 0.4 * max(dot(vN, normalize(vec3(0.4, 1.0, 0.3))), 0.0);
    vec3 ambient = mix(vec3(1.0, 0.97, 0.9), vec3(0.3, 0.27, 0.45), g);
    col = vCol.rgb * sun * (ambient + pointLights(vWorld) * (0.3 + g * 1.1));
    if (vCol.a > 0.5) col = mix(col, vec3(1.0), 0.65);            // hit flash
  }
  o = vec4(applyFog(col, vWorld, g), 1.0);
}`;

const SKY_VS = `#version 300 es
const vec2 P[3] = vec2[3](vec2(-1.0, -1.0), vec2(3.0, -1.0), vec2(-1.0, 3.0));
out vec2 vP;
void main() { vP = P[gl_VertexID]; gl_Position = vec4(P[gl_VertexID], 0.9999, 1.0); }`;
const SKY_FS = `#version 300 es
precision highp float;
uniform float uG; uniform float uPitch; uniform vec3 uFogDay; uniform vec3 uFogGloam;
in vec2 vP; out vec4 o;
void main() {
  float h = clamp(vP.y * 0.5 + 0.5 + uPitch * 0.6, 0.0, 1.0);
  vec3 dayTop = vec3(0.45, 0.68, 0.95), gloamTop = vec3(0.07, 0.04, 0.13);
  vec3 top = mix(dayTop, gloamTop, uG), hor = mix(uFogDay, uFogGloam, uG);
  o = vec4(mix(hor, top, smoothstep(0.35, 1.0, h)), 1.0);
}`;

const LINE_VS = `#version 300 es
layout(location=0) in vec3 aPos; uniform mat4 uVP; uniform vec3 uOff;
void main() { gl_Position = uVP * vec4(aPos * 1.004 - 0.002 + uOff, 1.0); }`;
const LINE_FS = `#version 300 es
precision mediump float; uniform vec4 uColor; out vec4 o; void main() { o = uColor; }`;

function compile(gl, vs, fs) {
  const mk = (type, src) => {
    const s = gl.createShader(type);
    gl.shaderSource(s, src);
    gl.compileShader(s);
    if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) throw new Error(gl.getShaderInfoLog(s) + '\n' + src);
    return s;
  };
  const p = gl.createProgram();
  gl.attachShader(p, mk(gl.VERTEX_SHADER, vs));
  gl.attachShader(p, mk(gl.FRAGMENT_SHADER, fs));
  gl.linkProgram(p);
  if (!gl.getProgramParameter(p, gl.LINK_STATUS)) throw new Error(gl.getProgramInfoLog(p));
  const u = {};
  const n = gl.getProgramParameter(p, gl.ACTIVE_UNIFORMS);
  for (let i = 0; i < n; i++) {
    const info = gl.getActiveUniform(p, i);
    const name = info.name.replace(/\[0\]$/, '');
    u[name] = gl.getUniformLocation(p, info.name);
  }
  return { p, u };
}

// Unit cube centered on origin, with normals (36 verts).
function cubeData() {
  const f = [
    [[1, 0, 0], [[.5, -.5, -.5], [.5, .5, -.5], [.5, .5, .5], [.5, -.5, .5]]],
    [[-1, 0, 0], [[-.5, -.5, .5], [-.5, .5, .5], [-.5, .5, -.5], [-.5, -.5, -.5]]],
    [[0, 1, 0], [[-.5, .5, -.5], [-.5, .5, .5], [.5, .5, .5], [.5, .5, -.5]]],
    [[0, -1, 0], [[-.5, -.5, -.5], [.5, -.5, -.5], [.5, -.5, .5], [-.5, -.5, .5]]],
    [[0, 0, 1], [[.5, -.5, .5], [.5, .5, .5], [-.5, .5, .5], [-.5, -.5, .5]]],
    [[0, 0, -1], [[-.5, -.5, -.5], [-.5, .5, -.5], [.5, .5, -.5], [.5, -.5, -.5]]],
  ];
  const out = [];
  for (const [n, c] of f) {
    for (const i of [0, 1, 2, 0, 2, 3]) out.push(...c[i], ...n);
  }
  // Ensure CCW winding from outside.
  for (let t = 0; t < out.length; t += 18) {
    const a = out.slice(t, t + 3), b = out.slice(t + 6, t + 9), c = out.slice(t + 12, t + 15), n = out.slice(t + 3, t + 6);
    const u = [b[0] - a[0], b[1] - a[1], b[2] - a[2]], v = [c[0] - a[0], c[1] - a[1], c[2] - a[2]];
    const cr = [u[1] * v[2] - u[2] * v[1], u[2] * v[0] - u[0] * v[2], u[0] * v[1] - u[1] * v[0]];
    if (cr[0] * n[0] + cr[1] * n[1] + cr[2] * n[2] < 0) {
      for (let k = 0; k < 3; k++) { const tmp = out[t + 6 + k]; out[t + 6 + k] = out[t + 12 + k]; out[t + 12 + k] = tmp; }
    }
  }
  return new Float32Array(out);
}

export class Renderer {
  constructor(canvas) {
    const gl = canvas.getContext('webgl2', { antialias: false, alpha: false, powerPreference: 'high-performance', preserveDrawingBuffer: false });
    if (!gl) throw new Error('WebGL2 is not available on this device/browser. Lanternfall needs WebGL2 (Chrome, Edge, Firefox, or Safari 15+).');
    this.gl = gl; this.canvas = canvas;
    this.terrain = compile(gl, TERRAIN_VS, TERRAIN_FS);
    this.box = compile(gl, BOX_VS, BOX_FS);
    this.sky = compile(gl, SKY_VS, SKY_FS);
    this.line = compile(gl, LINE_VS, LINE_FS);
    this.chunks = new Map();
    this.stats = { draws: 0, tris: 0, chunks: 0, chunksTotal: 0, gpuBytes: 0 };
    this.instances = new Float32Array(MAX_INSTANCES * 20);
    this.nInst = 0;
    this.fogDay = [0.78, 0.87, 0.95];
    this.fogGloam = [0.09, 0.06, 0.14];
    this.lightsBuf = new Float32Array(MAX_LIGHTS * 4);
    this.lightCols = new Float32Array(MAX_LIGHTS * 3);
    this.initTexture();
    this.initIndex();
    this.initBox();
    this.initLines();
    this.skyVao = gl.createVertexArray();
    gl.enable(gl.DEPTH_TEST);
    gl.enable(gl.CULL_FACE);
    gl.cullFace(gl.BACK);
  }

  initTexture() {
    const gl = this.gl;
    const { data, layers } = buildAtlas();
    this.atlasData = data;
    this.tex = gl.createTexture();
    gl.bindTexture(gl.TEXTURE_2D_ARRAY, this.tex);
    gl.texImage3D(gl.TEXTURE_2D_ARRAY, 0, gl.RGBA8, TILE, TILE, layers, 0, gl.RGBA, gl.UNSIGNED_BYTE, data);
    gl.generateMipmap(gl.TEXTURE_2D_ARRAY);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MIN_FILTER, gl.NEAREST_MIPMAP_LINEAR);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_S, gl.REPEAT);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_T, gl.REPEAT);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAX_LEVEL, 3);
  }

  initIndex() {
    const gl = this.gl;
    const quads = 65536;
    const idx = new Uint32Array(quads * 6);
    for (let q = 0, v = 0; q < quads; q++, v += 4) idx.set([v, v + 1, v + 2, v, v + 2, v + 3], q * 6);
    this.ibo = gl.createBuffer();
    gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, this.ibo);
    gl.bufferData(gl.ELEMENT_ARRAY_BUFFER, idx, gl.STATIC_DRAW);
  }

  initBox() {
    const gl = this.gl;
    this.boxVao = gl.createVertexArray();
    gl.bindVertexArray(this.boxVao);
    const vb = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, vb);
    gl.bufferData(gl.ARRAY_BUFFER, cubeData(), gl.STATIC_DRAW);
    gl.enableVertexAttribArray(0); gl.vertexAttribPointer(0, 3, gl.FLOAT, false, 24, 0);
    gl.enableVertexAttribArray(1); gl.vertexAttribPointer(1, 3, gl.FLOAT, false, 24, 12);
    this.instBuf = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, this.instBuf);
    gl.bufferData(gl.ARRAY_BUFFER, this.instances.byteLength, gl.DYNAMIC_DRAW);
    for (let i = 0; i < 5; i++) {
      gl.enableVertexAttribArray(2 + i);
      gl.vertexAttribPointer(2 + i, 4, gl.FLOAT, false, 80, i * 16);
      gl.vertexAttribDivisor(2 + i, 1);
    }
    gl.bindVertexArray(null);
  }

  initLines() {
    const gl = this.gl;
    const e = [];
    const c = [[0, 0, 0], [1, 0, 0], [1, 0, 1], [0, 0, 1], [0, 1, 0], [1, 1, 0], [1, 1, 1], [0, 1, 1]];
    for (const [a, b] of [[0, 1], [1, 2], [2, 3], [3, 0], [4, 5], [5, 6], [6, 7], [7, 4], [0, 4], [1, 5], [2, 6], [3, 7]]) e.push(...c[a], ...c[b]);
    this.lineVao = gl.createVertexArray();
    gl.bindVertexArray(this.lineVao);
    const b = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, b);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array(e), gl.STATIC_DRAW);
    gl.enableVertexAttribArray(0); gl.vertexAttribPointer(0, 3, gl.FLOAT, false, 0, 0);
    gl.bindVertexArray(null);
  }

  makeMesh(bytes) {
    const gl = this.gl;
    if (!bytes.length) return null;
    const vao = gl.createVertexArray();
    gl.bindVertexArray(vao);
    const vbo = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
    gl.bufferData(gl.ARRAY_BUFFER, bytes, gl.STATIC_DRAW);
    gl.enableVertexAttribArray(0); gl.vertexAttribPointer(0, 4, gl.UNSIGNED_BYTE, false, 8, 0);
    gl.enableVertexAttribArray(1); gl.vertexAttribPointer(1, 4, gl.UNSIGNED_BYTE, false, 8, 4);
    gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, this.ibo);
    gl.bindVertexArray(null);
    return { vao, vbo, quads: bytes.length / 32, bytes: bytes.length };
  }

  freeMesh(m) {
    if (!m) return;
    this.gl.deleteBuffer(m.vbo);
    this.gl.deleteVertexArray(m.vao);
  }

  // key "cx,cz,lod"
  setChunk(cx, cz, lod, mesh) {
    const key = `${cx},${cz},${lod}`;
    const old = this.chunks.get(key);
    if (old) { this.freeMesh(old.o); this.freeMesh(old.w); }
    this.chunks.set(key, { cx, cz, lod, o: this.makeMesh(mesh.opaque), w: this.makeMesh(mesh.water) });
  }

  dropChunk(cx, cz, lod) {
    const key = `${cx},${cz},${lod}`;
    const old = this.chunks.get(key);
    if (old) { this.freeMesh(old.o); this.freeMesh(old.w); this.chunks.delete(key); }
  }

  resize(scale) {
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    const w = Math.max(1, Math.round(this.canvas.clientWidth * dpr * scale));
    const h = Math.max(1, Math.round(this.canvas.clientHeight * dpr * scale));
    if (this.canvas.width !== w || this.canvas.height !== h) { this.canvas.width = w; this.canvas.height = h; }
  }

  // Box instances for entities: matrix (16 floats) + rgba (a: 0 normal, 1 flash, 2 emissive).
  beginBoxes() { this.nInst = 0; }
  addBox(matFn, r, g, b, a) {
    if (this.nInst >= MAX_INSTANCES) return;
    const o = this.nInst * 20;
    matFn(this.instances, o);
    this.instances[o + 16] = r; this.instances[o + 17] = g; this.instances[o + 18] = b; this.instances[o + 19] = a;
    this.nInst++;
  }

  setCommon(prog, cam, lights, playerLight, opts) {
    const gl = this.gl, u = prog.u;
    gl.uniform3fv(u.uCam, cam);
    gl.uniform3fv(u.uFogDay, this.fogDay);
    gl.uniform3fv(u.uFogGloam, this.fogGloam);
    gl.uniform1f(u.uFogNear, opts.fogNear);
    gl.uniform1f(u.uFogFar, opts.fogFar);
    gl.uniform1f(u.uGloamX, opts.gloamX);
    gl.uniform4fv(u.uLights, this.lightsBuf);
    gl.uniform3fv(u.uLightCol, this.lightCols);
    gl.uniform1i(u.uNumLights, lights);
    gl.uniform4fv(u.uPlayerLight, playerLight);
  }

  pickLights(world, cam, max) {
    const ccx = Math.floor(cam[0] / world.cs), ccz = Math.floor(cam[2] / world.cs);
    const cand = [];
    for (let dz = -2; dz <= 2; dz++) for (let dx = -2; dx <= 2; dx++) {
      const list = world.lights.get((ccz + dz) * world.cx + (ccx + dx));
      if (!list || ccx + dx < 0 || ccz + dz < 0 || ccx + dx >= world.cx || ccz + dz >= world.cz) continue;
      for (const l of list) cand.push([l, (l[0] - cam[0]) ** 2 + (l[1] - cam[1]) ** 2 + (l[2] - cam[2]) ** 2]);
    }
    cand.sort((a, b) => a[1] - b[1]);
    const n = Math.min(max, cand.length, MAX_LIGHTS);
    for (let i = 0; i < n; i++) {
      const l = cand[i][0], col = LIGHT_COLORS[l[3]];
      this.lightsBuf.set([l[0], l[1], l[2], l[3] === 13 ? 7.5 : 5.0], i * 4);
      this.lightCols.set(col, i * 3);
    }
    return n;
  }

  // view: {eye, target, fov, near, far}; opts: quality-driven settings.
  render(world, view, opts) {
    const gl = this.gl, st = this.stats;
    st.draws = 0; st.tris = 0; st.chunks = 0;
    gl.viewport(0, 0, this.canvas.width, this.canvas.height);
    const proj = perspective(view.fov, this.canvas.width / this.canvas.height, 0.1, view.far);
    const viewM = lookAt(view.eye, view.target, [0, 1, 0]);
    const vp = mul(proj, viewM);
    this.vp = vp;
    const planes = frustumPlanes(vp);
    const g = Math.min(1, Math.max(0, (view.eye[0] - (opts.gloamX - 3)) / 7));

    gl.clear(gl.DEPTH_BUFFER_BIT);
    // Sky.
    gl.disable(gl.DEPTH_TEST);
    gl.useProgram(this.sky.p);
    gl.uniform1f(this.sky.u.uG, g);
    gl.uniform1f(this.sky.u.uPitch, view.pitch || 0);
    gl.uniform3fv(this.sky.u.uFogDay, this.fogDay);
    gl.uniform3fv(this.sky.u.uFogGloam, this.fogGloam);
    gl.bindVertexArray(this.skyVao);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
    st.draws++;
    gl.enable(gl.DEPTH_TEST);

    const nLights = this.pickLights(world, view.eye, opts.maxLights);
    const playerLight = opts.playerLight || [0, 0, 0, 0];

    // Terrain: choose full or LOD mesh per chunk by distance; frustum cull.
    const visible = [];
    const cs = world.cs, sy = world.sy;
    for (const ch of this.chunks.values()) {
      const x0 = ch.cx * cs, z0 = ch.cz * cs;
      const dx = Math.max(x0 - view.eye[0], 0, view.eye[0] - (x0 + cs));
      const dz = Math.max(z0 - view.eye[2], 0, view.eye[2] - (z0 + cs));
      const d = Math.hypot(dx, dz) / cs;
      if (d > opts.renderDist) continue;
      const wantLod = d > opts.lodDist ? 1 : 0;
      if (ch.lod !== wantLod) {
        // Fall back to the other LOD if the wanted one isn't built yet.
        if (this.chunks.has(`${ch.cx},${ch.cz},${wantLod}`)) continue;
      }
      if (!aabbVisible(planes, x0, 0, z0, x0 + cs, sy, z0 + cs)) continue;
      visible.push([ch, d]);
    }
    visible.sort((a, b) => a[1] - b[1]); // front-to-back for early-z
    gl.useProgram(this.terrain.p);
    const tu = this.terrain.u;
    gl.uniformMatrix4fv(tu.uVP, false, vp);
    gl.uniform1f(tu.uTime, opts.time);
    gl.uniform1f(tu.uSway, opts.sway ? 1 : 0);
    gl.uniform1i(tu.uTex, 0);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D_ARRAY, this.tex);
    this.setCommon(this.terrain, view.eye, nLights, playerLight, opts);
    for (const [ch] of visible) {
      if (!ch.o) continue;
      gl.uniform3f(tu.uOrigin, ch.cx * cs, 0, ch.cz * cs);
      gl.uniform1f(tu.uScale, ch.lod ? 2 : 1);
      gl.bindVertexArray(ch.o.vao);
      gl.drawElements(gl.TRIANGLES, ch.o.quads * 6, gl.UNSIGNED_INT, 0);
      st.draws++; st.tris += ch.o.quads * 2; st.chunks++;
    }

    // Entities (one instanced draw call).
    if (this.nInst > 0) {
      gl.useProgram(this.box.p);
      gl.uniformMatrix4fv(this.box.u.uVP, false, vp);
      this.setCommon(this.box, view.eye, nLights, playerLight, opts);
      gl.bindVertexArray(this.boxVao);
      gl.bindBuffer(gl.ARRAY_BUFFER, this.instBuf);
      gl.bufferSubData(gl.ARRAY_BUFFER, 0, this.instances, 0, this.nInst * 20);
      gl.drawArraysInstanced(gl.TRIANGLES, 0, 36, this.nInst);
      st.draws++; st.tris += this.nInst * 12;
    }

    // Target highlight.
    if (opts.highlight) {
      gl.useProgram(this.line.p);
      gl.uniformMatrix4fv(this.line.u.uVP, false, vp);
      gl.uniform3fv(this.line.u.uOff, opts.highlight);
      gl.uniform4f(this.line.u.uColor, 1, 0.95, 0.6, 1);
      gl.bindVertexArray(this.lineVao);
      gl.drawArrays(gl.LINES, 0, 24);
      st.draws++;
    }

    // Water (translucent, after opaque, back-to-front).
    gl.useProgram(this.terrain.p);
    gl.enable(gl.BLEND);
    gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
    gl.depthMask(false);
    for (let i = visible.length - 1; i >= 0; i--) {
      const ch = visible[i][0];
      if (!ch.w) continue;
      gl.uniform3f(tu.uOrigin, ch.cx * cs, 0, ch.cz * cs);
      gl.uniform1f(tu.uScale, ch.lod ? 2 : 1);
      gl.bindVertexArray(ch.w.vao);
      gl.drawElements(gl.TRIANGLES, ch.w.quads * 6, gl.UNSIGNED_INT, 0);
      st.draws++; st.tris += ch.w.quads * 2;
    }
    gl.depthMask(true);
    gl.disable(gl.BLEND);
    gl.bindVertexArray(null);

    let bytes = 0;
    for (const ch of this.chunks.values()) bytes += (ch.o ? ch.o.bytes : 0) + (ch.w ? ch.w.bytes : 0);
    st.gpuBytes = bytes + 65536 * 24 + this.instances.byteLength + this.atlasData.byteLength * 1.34;
    st.chunksTotal = this.chunks.size;
    return vp;
  }
}
