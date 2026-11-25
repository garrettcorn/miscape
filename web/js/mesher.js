import { B, BLOCKS } from './blocks.js';

// Vertex format: 8 bytes.
//   a0 = [x*4, y*4, z*4, light]   chunk-local position in quarter-block units, light 0..255
//   a1 = [u*4, v*4, layer, flags] texture coords (repeat units), atlas layer, flags
export const FLAG_EMISSIVE = 1, FLAG_WATER = 2, FLAG_CUTOUT = 4, FLAG_SWAY = 8;

const FACES = [
  { n: [1, 0, 0], c: [[1, 0, 0], [1, 1, 0], [1, 1, 1], [1, 0, 1]], shade: 0.8, tile: 'side' },
  { n: [-1, 0, 0], c: [[0, 0, 1], [0, 1, 1], [0, 1, 0], [0, 0, 0]], shade: 0.8, tile: 'side' },
  { n: [0, 1, 0], c: [[0, 1, 0], [0, 1, 1], [1, 1, 1], [1, 1, 0]], shade: 1.0, tile: 'top' },
  { n: [0, -1, 0], c: [[0, 0, 0], [1, 0, 0], [1, 0, 1], [0, 0, 1]], shade: 0.5, tile: 'bottom' },
  { n: [0, 0, 1], c: [[1, 0, 1], [1, 1, 1], [0, 1, 1], [0, 0, 1]], shade: 0.65, tile: 'side' },
  { n: [0, 0, -1], c: [[0, 0, 0], [0, 1, 0], [1, 1, 0], [1, 0, 0]], shade: 0.65, tile: 'side' },
];
// Fix winding to be counter-clockwise seen from outside (robust to typos above).
for (const f of FACES) {
  const [a, b, c] = f.c;
  const u = [b[0] - a[0], b[1] - a[1], b[2] - a[2]], v = [c[0] - a[0], c[1] - a[1], c[2] - a[2]];
  const cr = [u[1] * v[2] - u[2] * v[1], u[2] * v[0] - u[0] * v[2], u[0] * v[1] - u[1] * v[0]];
  if (cr[0] * f.n[0] + cr[1] * f.n[1] + cr[2] * f.n[2] < 0) f.c.reverse();
  // Tangent axes for AO and UVs.
  f.axes = [0, 1, 2].filter((i) => f.n[i] === 0);
}

function uvFor(f, corner) {
  if (f.n[1] !== 0) return [corner[0], corner[2]];
  const horiz = f.n[0] !== 0 ? corner[2] : corner[0];
  return [horiz, 1 - corner[1]];
}

class Buf {
  constructor() { this.a = new Uint8Array(1 << 16); this.n = 0; }
  reset() { this.n = 0; }
  push8(x, y, z, l, u, v, layer, flags) {
    if (this.n + 8 > this.a.length) { const b = new Uint8Array(this.a.length * 2); b.set(this.a); this.a = b; }
    const a = this.a, n = this.n;
    a[n] = x; a[n + 1] = y; a[n + 2] = z; a[n + 3] = l; a[n + 4] = u; a[n + 5] = v; a[n + 6] = layer; a[n + 7] = flags;
    this.n += 8;
  }
  take() { return this.a.slice(0, this.n); }
}
const opaqueBuf = new Buf(), waterBuf = new Buf();

const opaqueCube = (b) => { const d = BLOCKS[b]; return d && d.opaque; };

// Build a full-detail mesh for chunk (cx, cz).
export function meshChunk(world, cx, cz, { ao = true } = {}) {
  opaqueBuf.reset(); waterBuf.reset();
  const cs = world.cs, ox = cx * cs, oz = cz * cs, sy = world.sy;
  const get = (x, y, z) => world.get(x, y, z);
  for (let y = 0; y < sy; y++) for (let lz = 0; lz < cs; lz++) for (let lx = 0; lx < cs; lx++) {
    const x = ox + lx, z = oz + lz;
    const b = world.blocks[world.idx(x, y, z)];
    if (b === B.AIR) continue;
    const d = BLOCKS[b];
    if (!d) continue;
    if (d.kind === 'cross') { cross(opaqueBuf, lx, y, lz, d.side); continue; }
    const isWater = d.kind === 'water';
    for (const f of FACES) {
      const nb = get(x + f.n[0], y + f.n[1], z + f.n[2]);
      if (isWater) { if (nb !== B.AIR && BLOCKS[nb].kind !== 'cross') continue; }
      else if (opaqueCube(nb)) continue;
      const layer = d[f.tile];
      let flags = d.emissive ? FLAG_EMISSIVE : 0;
      if (b === B.LEAVES || b === B.BERRY) flags |= FLAG_SWAY;
      const out = isWater ? waterBuf : opaqueBuf;
      if (isWater) flags |= FLAG_WATER;
      const lights = [0, 0, 0, 0];
      for (let i = 0; i < 4; i++) {
        let a = 3;
        if (ao && !isWater && !d.emissive) {
          const c = f.c[i];
          const s = [0, 0, 0], t = [0, 0, 0];
          s[f.axes[0]] = c[f.axes[0]] ? 1 : -1;
          t[f.axes[1]] = c[f.axes[1]] ? 1 : -1;
          const bx = x + f.n[0], by = y + f.n[1], bz = z + f.n[2];
          const s1 = opaqueCube(get(bx + s[0], by + s[1], bz + s[2])) ? 1 : 0;
          const s2 = opaqueCube(get(bx + t[0], by + t[1], bz + t[2])) ? 1 : 0;
          const cc = opaqueCube(get(bx + s[0] + t[0], by + s[1] + t[1], bz + s[2] + t[2])) ? 1 : 0;
          a = s1 && s2 ? 0 : 3 - (s1 + s2 + cc);
        }
        const shade = d.emissive ? 1 : f.shade;
        lights[i] = Math.round(255 * shade * (0.5 + a / 6));
      }
      // Flip the quad diagonal when it reduces AO artifacts.
      const flip = lights[0] + lights[2] < lights[1] + lights[3];
      const order = flip ? [1, 2, 3, 0] : [0, 1, 2, 3];
      for (const i of order) {
        const c = f.c[i];
        let py = (y + c[1]) * 4;
        if (isWater && c[1] === 1 && get(x, y + 1, z) !== B.WATER) py -= 1; // water surface sits a bit lower
        const [u, v] = uvFor(f, c);
        out.push8((lx + c[0]) * 4, py, (lz + c[2]) * 4, lights[i], u * 4, v * 4, layer, flags);
      }
    }
  }
  return { opaque: opaqueBuf.take(), water: waterBuf.take() };
}

function cross(out, x, y, z, layer) {
  const quads = [
    [[0, 0, 0], [0, 1, 0], [1, 1, 1], [1, 0, 1]],
    [[1, 0, 1], [1, 1, 1], [0, 1, 0], [0, 0, 0]],
    [[1, 0, 0], [1, 1, 0], [0, 1, 1], [0, 0, 1]],
    [[0, 0, 1], [0, 1, 1], [1, 1, 0], [1, 0, 0]],
  ];
  const uv = [[0, 1], [0, 0], [1, 0], [1, 1]];
  for (const q of quads) for (let i = 0; i < 4; i++) {
    const c = q[i];
    out.push8((x + c[0]) * 4, (y + c[1]) * 4, (z + c[2]) * 4, 230, uv[i][0] * 4, uv[i][1] * 4, layer, FLAG_CUTOUT | FLAG_SWAY);
  }
}

// Level-of-detail mesh: 2x2x2 cells, each represented by its top-most opaque
// block (so surfaces keep their colors). No AO; ~4-8x fewer triangles.
export function meshChunkLOD(world, cx, cz) {
  opaqueBuf.reset(); waterBuf.reset();
  const cs = world.cs, L = cs / 2, H = Math.ceil(world.sy / 2);
  const ox = cx * cs, oz = cz * cs;
  // Representative grid with a 1-cell border.
  const W = L + 2, D = L + 2, HH = H + 2;
  const grid = new Uint8Array(W * HH * D);
  const gi = (x, y, z) => (y * D + z) * W + x;
  for (let gy = 0; gy < HH; gy++) for (let gz = 0; gz < D; gz++) for (let gx = 0; gx < W; gx++) {
    const bx = ox + (gx - 1) * 2, by = (gy - 1) * 2, bz = oz + (gz - 1) * 2;
    let rep = B.AIR, water = false;
    for (let dy = 1; dy >= 0 && rep === B.AIR; dy--) for (let dz = 0; dz < 2 && rep === B.AIR; dz++) for (let dx = 0; dx < 2; dx++) {
      const b = world.get(bx + dx, by + dy, bz + dz);
      if (opaqueCube(b)) { rep = b; break; }
      if (b === B.WATER) water = true;
    }
    grid[gi(gx, gy, gz)] = rep !== B.AIR ? rep : (water ? B.WATER : B.AIR);
  }
  for (let y = 1; y <= H; y++) for (let z = 1; z <= L; z++) for (let x = 1; x <= L; x++) {
    const b = grid[gi(x, y, z)];
    if (b === B.AIR) continue;
    const d = BLOCKS[b];
    const isWater = b === B.WATER;
    for (const f of FACES) {
      const nb = grid[gi(x + f.n[0], y + f.n[1], z + f.n[2])];
      if (isWater ? nb !== B.AIR : opaqueCube(nb)) continue;
      const flags = (d.emissive ? FLAG_EMISSIVE : 0) | (isWater ? FLAG_WATER : 0);
      const l = Math.round(255 * (d.emissive ? 1 : f.shade) * 0.92);
      const out = isWater ? waterBuf : opaqueBuf;
      for (let i = 0; i < 4; i++) {
        const c = f.c[i];
        const [u, v] = uvFor(f, c);
        out.push8((x - 1 + c[0]) * 4, (y - 1 + c[1]) * 4, (z - 1 + c[2]) * 4, l, u * 8, v * 8, d[f.tile], flags);
      }
    }
  }
  return { opaque: opaqueBuf.take(), water: waterBuf.take() };
}
