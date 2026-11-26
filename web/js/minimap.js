// Minimap (HUD corner) and shared map helpers. The minimap is drawn from the
// chunks the client already has: each chunk becomes a cached 16x16 image of
// its top blocks, colored from the game's own block textures, so it costs
// nothing extra to download.
import { BLOCKS, B, isWater } from './blocks.js';
import { TILE } from './atlas.js';

// Average color of each block's top texture (water and leaves tuned so the
// map reads clearly).
export function blockColors(atlasData) {
  const out = [];
  for (let b = 0; b < BLOCKS.length; b++) {
    const d = BLOCKS[b];
    if (!d || d.kind === 'air') { out[b] = null; continue; }
    const layer = d.top, o = layer * TILE * TILE * 4;
    let r = 0, g = 0, bl = 0, n = 0;
    for (let i = 0; i < TILE * TILE; i++) {
      const a = atlasData[o + i * 4 + 3];
      if (a < 40) continue;
      r += atlasData[o + i * 4]; g += atlasData[o + i * 4 + 1]; bl += atlasData[o + i * 4 + 2]; n++;
    }
    out[b] = n ? [r / n, g / n, bl / n] : [128, 128, 128];
  }
  out[B.WATER] = out[B.FISHSPOT] = [52, 118, 196];
  return out;
}

const MARK = { me: '#ffe27a', npc: '#ffcf6b', player: '#7cc4ff', mob: '#ff7a95', drop: '#ffffff' };

export class MiniMap {
  constructor(game, canvas) {
    this.g = game;
    this.canvas = canvas;
    this.ctx = canvas.getContext('2d');
    this.tiles = new Map(); // chunk key -> canvas (16x16)
    this.radius = 48; // blocks shown each side of the player
    this.colors = null;
  }

  // A chunk arrived or changed: its tile is rebuilt on the next draw.
  invalidate(cx, cz) { if (this.g.world) this.tiles.delete(this.g.world.key(cx, cz)); }
  drop(cx, cz) { this.invalidate(cx, cz); }
  reset() { this.tiles.clear(); }

  tile(cx, cz) {
    const w = this.g.world, k = w.key(cx, cz);
    let t = this.tiles.get(k);
    if (t) return t;
    const data = w.chunkData(cx, cz);
    if (!data) return null;
    if (!this.colors) this.colors = blockColors(this.g.r.atlasData);
    const cs = w.cs, ground = (this.g.worldInfo && this.g.worldInfo.groundY) || 12;
    t = document.createElement('canvas');
    t.width = t.height = cs;
    const tc = t.getContext('2d'), img = tc.createImageData(cs, cs);
    for (let z = 0; z < cs; z++) for (let x = 0; x < cs; x++) {
      let b = 0, y = w.sy - 1;
      for (; y >= 0; y--) { b = data[(y * cs + z) * cs + x]; if (b !== B.AIR && b !== B.FLOWERS) break; }
      const c = this.colors[b] || [0, 0, 0];
      // Higher things (trees, walls) are lighter, lower (sea bed) darker.
      const k2 = isWater(b) ? 1 : Math.max(0.6, Math.min(1.35, 1 + (y - ground) * 0.06));
      const i = (z * cs + x) * 4;
      img.data[i] = c[0] * k2; img.data[i + 1] = c[1] * k2; img.data[i + 2] = c[2] * k2; img.data[i + 3] = 255;
    }
    tc.putImageData(img, 0, 0);
    this.tiles.set(k, t);
    return t;
  }

  draw() {
    const g = this.g, w = g.world, b = g.body;
    if (!w || !b) return;
    const cv = this.canvas, ctx = this.ctx;
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    const size = Math.round(cv.clientWidth * dpr);
    if (!size) return;
    if (cv.width !== size) { cv.width = cv.height = size; }
    const s = size / (2 * this.radius); // pixels per block
    ctx.imageSmoothingEnabled = false;
    ctx.fillStyle = '#1d2a3a';
    ctx.fillRect(0, 0, size, size);
    const cs = w.cs, R = this.radius;
    const toX = (x) => size / 2 + w.dx(b.x, x) * s, toY = (z) => size / 2 + (z - b.z) * s;
    for (let cz = Math.floor((b.z - R) / cs); cz <= Math.floor((b.z + R) / cs); cz++) {
      if (cz < 0 || cz >= w.cz) continue;
      for (let i = Math.floor((b.x - R) / cs); i <= Math.floor((b.x + R) / cs); i++) {
        const cx = w.wrapC(i), t = this.tile(cx, cz);
        if (!t) continue;
        ctx.drawImage(t, Math.floor(toX(cx * cs)), Math.floor(toY(cz * cs)), Math.ceil(cs * s) + 1, Math.ceil(cs * s) + 1);
      }
    }
    // Zone edges.
    for (const zn of g.zones || []) {
      ctx.beginPath();
      ctx.arc(toX(zn.x), toY(zn.z), zn.r * s, 0, Math.PI * 2);
      ctx.strokeStyle = zn.kind === 'danger' ? 'rgba(255,90,140,.9)' : 'rgba(255,226,122,.75)';
      ctx.lineWidth = Math.max(1.5, dpr * 1.5);
      ctx.setLineDash(zn.kind === 'danger' ? [4 * dpr, 3 * dpr] : []);
      ctx.stroke();
    }
    ctx.setLineDash([]);
    const dot = (x, z, col, r) => {
      const px = toX(x), py = toY(z);
      if (px < -4 || py < -4 || px > size + 4 || py > size + 4) return;
      ctx.beginPath(); ctx.arc(px, py, r * dpr, 0, Math.PI * 2);
      ctx.fillStyle = col; ctx.fill();
      ctx.lineWidth = dpr; ctx.strokeStyle = 'rgba(0,0,0,.6)'; ctx.stroke();
    };
    for (const d of g.drops.values()) dot(d.x, d.z, MARK.drop, 1.5);
    for (const m of g.mobs.values()) if (!m.dead) dot(m.x, m.z, MARK.mob, 2.2);
    for (const n of Object.values(g.npcs || {})) dot(n.x, n.z, MARK.npc, 2.8);
    for (const p of g.others.values()) dot(p.x, p.z, MARK.player, 2.8);
    drawArrow(ctx, size / 2, size / 2, g.bodyYaw || 0, 6 * dpr, MARK.me);
    // North marker.
    ctx.font = `700 ${11 * dpr}px system-ui, sans-serif`;
    ctx.fillStyle = 'rgba(255,255,255,.85)'; ctx.textAlign = 'center'; ctx.textBaseline = 'top';
    ctx.fillText('N', size / 2, 3 * dpr);
  }
}

// An arrowhead pointing the way the character faces (yaw 0 = south, +z).
export function drawArrow(ctx, x, y, yaw, r, col) {
  const fx = Math.sin(yaw), fz = Math.cos(yaw);
  ctx.beginPath();
  ctx.moveTo(x + fx * r, y + fz * r);
  ctx.lineTo(x - fx * r * 0.7 + fz * r * 0.7, y - fz * r * 0.7 - fx * r * 0.7);
  ctx.lineTo(x - fx * r * 0.3, y - fz * r * 0.3);
  ctx.lineTo(x - fx * r * 0.7 - fz * r * 0.7, y - fz * r * 0.7 + fx * r * 0.7);
  ctx.closePath();
  ctx.fillStyle = col; ctx.fill();
  ctx.lineWidth = Math.max(1, r / 6); ctx.strokeStyle = 'rgba(0,0,0,.7)'; ctx.stroke();
}
