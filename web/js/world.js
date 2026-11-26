import { B, isSolid, LIGHT_COLORS } from './blocks.js';

// Client copy of the voxel world: only the chunks near the player, streamed
// from the server and dropped again when far away. The world wraps east–west
// (x), so x is always folded into 0 <= x < sx; past the poles (z) is air,
// but solid for movement.
export class World {
  constructor(sx, sy, sz, chunk) {
    this.sx = sx; this.sy = sy; this.sz = sz; this.cs = chunk;
    this.cx = sx / chunk; this.cz = sz / chunk;
    this.chunks = new Map(); // key -> Uint8Array(cs*cs*sy), index (y*cs+z)*cs+x
    this.lights = new Map(); // key -> [[x,y,z,block], ...] (canonical coordinates)
  }

  key(cx, cz) { return cz * this.cx + this.wrapC(cx); }
  wrapC(cx) { return ((cx % this.cx) + this.cx) % this.cx; }
  wrapX(x) { return ((x % this.sx) + this.sx) % this.sx; }
  // Shortest east–west difference b - a (across the date line if shorter).
  dx(a, b) { let d = (b - a) % this.sx; if (d > this.sx / 2) d -= this.sx; else if (d < -this.sx / 2) d += this.sx; return d; }
  dist(ax, az, bx, bz) { return Math.hypot(this.dx(ax, bx), bz - az); }
  // Chunk distance (in chunks) from a point, wrapping east–west.
  chunkDist(cx, cz, px, pz) { let d = (cx + 0.5 - px) % this.cx; if (d > this.cx / 2) d -= this.cx; else if (d < -this.cx / 2) d += this.cx; return Math.hypot(d, cz + 0.5 - pz); }

  inside(x, y, z) { return y >= 0 && z >= 0 && y < this.sy && z < this.sz; }
  get(x, y, z) {
    if (y < 0) return B.BEDROCK;
    if (y >= this.sy || z < 0 || z >= this.sz) return B.AIR;
    x = this.wrapX(x);
    const cs = this.cs, c = this.chunks.get(Math.floor(z / cs) * this.cx + Math.floor(x / cs));
    return c ? c[(y * cs + (z % cs)) * cs + (x % cs)] : B.AIR;
  }
  // The raw blocks of a loaded chunk (for the mesher), or undefined.
  chunkData(cx, cz) { return cz >= 0 && cz < this.cz ? this.chunks.get(this.key(cx, cz)) : undefined; }
  solidAt(x, y, z) {
    const iz = Math.floor(z), iy = Math.floor(y);
    if (iz < 0 || iz >= this.sz || iy < 0) return true;
    return isSolid(this.get(Math.floor(x), iy, iz));
  }
  chunkLoaded(cx, cz) { return cz >= 0 && cz < this.cz && this.chunks.has(this.key(cx, cz)); }
  columnLoaded(x, z) { return this.chunkLoaded(Math.floor(this.wrapX(x) / this.cs), Math.floor(z / this.cs)); }

  // RLE pairs (count-1, block) in x, z, y order — see world.EncodeChunk.
  applyChunk(cx, cz, rle) {
    const cs = this.cs, total = cs * cs * this.sy, data = new Uint8Array(total);
    let i = 0;
    for (let p = 0; p + 1 < rle.length; p += 2) {
      const n = rle[p] + 1, b = rle[p + 1];
      data.fill(b, i, Math.min(total, i + n));
      i += n;
    }
    this.chunks.set(this.key(cx, cz), data);
    this.scanLights(cx, cz);
  }
  set(x, y, z, b) {
    if (!this.inside(x, y, z)) return;
    x = this.wrapX(x);
    const cs = this.cs, cx = Math.floor(x / cs), cz = Math.floor(z / cs), c = this.chunks.get(this.key(cx, cz));
    if (!c) return;
    c[(y * cs + (z % cs)) * cs + (x % cs)] = b;
    this.scanLights(cx, cz);
  }
  scanLights(cx, cz) {
    const cs = this.cs, list = [], c = this.chunkData(cx, cz);
    if (!c) return;
    cx = this.wrapC(cx);
    for (let i = 0; i < c.length; i++) {
      const b = c[i];
      if (LIGHT_COLORS[b]) {
        const x = i % cs, z = Math.floor(i / cs) % cs, y = Math.floor(i / (cs * cs));
        list.push([cx * cs + x + 0.5, y + 0.5, cz * cs + z + 0.5, b]);
      }
    }
    this.lights.set(this.key(cx, cz), list);
  }
  // Drop chunks farther than `radius` chunks from (px, pz) (in chunk units).
  // Returns the dropped [cx, cz] pairs so their meshes can be freed.
  evict(px, pz, radius) {
    const out = [];
    for (const k of this.chunks.keys()) {
      const cx = k % this.cx, cz = Math.floor(k / this.cx);
      if (this.chunkDist(cx, cz, px, pz) > radius) { this.chunks.delete(k); this.lights.delete(k); out.push([cx, cz]); }
    }
    return out;
  }
  surfaceY(x, z) {
    for (let y = this.sy - 1; y >= 0; y--) if (isSolid(this.get(x, y, z))) return y + 1;
    return 0;
  }
}
