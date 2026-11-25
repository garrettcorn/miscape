import { B, isSolid } from './blocks.js';

// Client copy of the voxel world. Chunks arrive progressively from the server.
export class World {
  constructor(sx, sy, sz, chunk) {
    this.sx = sx; this.sy = sy; this.sz = sz; this.cs = chunk;
    this.cx = sx / chunk; this.cz = sz / chunk;
    this.blocks = new Uint8Array(sx * sy * sz);
    this.loaded = new Uint8Array(this.cx * this.cz);
    this.lights = new Map(); // chunk index -> [[x,y,z,block], ...]
  }
  idx(x, y, z) { return (y * this.sz + z) * this.sx + x; }
  inside(x, y, z) { return x >= 0 && y >= 0 && z >= 0 && x < this.sx && y < this.sy && z < this.sz; }
  get(x, y, z) {
    if (y < 0) return B.BEDROCK;
    if (y >= this.sy) return B.AIR;
    if (x < 0 || z < 0 || x >= this.sx || z >= this.sz) return B.STONE;
    return this.blocks[this.idx(x, y, z)];
  }
  solidAt(x, y, z) {
    const ix = Math.floor(x), iy = Math.floor(y), iz = Math.floor(z);
    if (ix < 0 || iz < 0 || ix >= this.sx || iz >= this.sz || iy < 0) return true;
    return isSolid(this.get(ix, iy, iz));
  }
  chunkLoaded(cx, cz) { return cx >= 0 && cz >= 0 && cx < this.cx && cz < this.cz && this.loaded[cz * this.cx + cx] === 1; }
  columnLoaded(x, z) { return this.chunkLoaded(Math.floor(x / this.cs), Math.floor(z / this.cs)); }

  // RLE pairs (count-1, block) in x, z, y order — see world.EncodeChunk.
  applyChunk(cx, cz, rle) {
    const cs = this.cs;
    let i = 0;
    const total = cs * cs * this.sy;
    for (let p = 0; p + 1 < rle.length; p += 2) {
      const n = rle[p] + 1, b = rle[p + 1];
      for (let k = 0; k < n && i < total; k++, i++) {
        const x = i % cs, z = Math.floor(i / cs) % cs, y = Math.floor(i / (cs * cs));
        this.blocks[this.idx(cx * cs + x, y, cz * cs + z)] = b;
      }
    }
    this.loaded[cz * this.cx + cx] = 1;
    this.scanLights(cx, cz);
  }
  set(x, y, z, b) {
    if (!this.inside(x, y, z)) return;
    this.blocks[this.idx(x, y, z)] = b;
    this.scanLights(Math.floor(x / this.cs), Math.floor(z / this.cs));
  }
  scanLights(cx, cz) {
    const cs = this.cs, list = [];
    for (let y = 0; y < this.sy; y++) for (let z = 0; z < cs; z++) for (let x = 0; x < cs; x++) {
      const b = this.blocks[this.idx(cx * cs + x, y, cz * cs + z)];
      if (b === B.LANTERN || b === B.GLIMMER) list.push([cx * cs + x + 0.5, y + 0.5, cz * cs + z + 0.5, b]);
    }
    this.lights.set(cz * this.cx + cx, list);
  }
  surfaceY(x, z) {
    for (let y = this.sy - 1; y >= 0; y--) if (isSolid(this.get(x, y, z))) return y + 1;
    return 0;
  }
}
