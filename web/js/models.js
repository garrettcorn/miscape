import { trs } from './mat.js';

const cache = new Map();
export function rgb(hex) {
  let c = cache.get(hex);
  if (!c) { c = [parseInt(hex.slice(1, 3), 16) / 255, parseInt(hex.slice(3, 5), 16) / 255, parseInt(hex.slice(5, 7), 16) / 255]; cache.set(hex, c); }
  return c;
}
const darker = (c, k) => [c[0] * k, c[1] * k, c[2] * k];
const LANTERN = { model: 'lantern', color: '#ffd27a', glow: true };
const SKIN = [0.96, 0.8, 0.64], EYE = [0.12, 0.1, 0.12];

// Add a box: world position (x,y,z) + yaw; local offset (ox,oy,oz) in the
// entity's rotated frame; size (w,h,d); optional pitch around a pivot
// located at the top of the box (for limbs).
function box(r, x, y, z, yaw, ox, oy, oz, w, h, d, col, a = 0, pitch = 0) {
  const c = Math.cos(yaw), s = Math.sin(yaw);
  // Rotate offset by yaw (same convention as trs: Ry).
  const tx = x + c * ox + s * oz, ty = y + oy, tz = z - s * ox + c * oz;
  r.addBox((m, o) => trs(m, o, tx, ty, tz, yaw, w, h, d, 0, -h / 2, 0, pitch), col[0], col[1], col[2], a);
}

// A friendly blocky person. Feet at y. Faces +Z in local space.
// e.swim switches the walk cycle for a front-crawl stroke and a flutter kick.
export function drawPerson(r, e) {
  const { x, y, z, yaw } = e;
  const col = rgb(e.color || '#4a90d9');
  const flash = e.flash > 0 ? 1 : 0;
  const phase = e.walk || 0;
  const walk = Math.sin(phase) * 0.7 * (e.moving ? 1 : 0);
  const swing = e.swing > 0 ? -Math.sin(e.swing * Math.PI) * 1.8 : 0;
  let legL = walk, legR = -walk, armL = -walk * 0.8, armR = walk * 0.8 + swing;
  if (e.swim) {
    if (e.moving) {
      // Front crawl: arms turn over in full circles, half a turn apart.
      armL = -(phase * 0.8 % (Math.PI * 2));
      armR = -((phase * 0.8 + Math.PI) % (Math.PI * 2)) + swing;
      legL = Math.sin(phase * 2.2) * 0.35; legR = -legL;
    } else {
      // Treading water: arms sweep out front, legs paddle gently.
      const t = Math.sin(phase * 0.6 + (e.id || 0));
      armL = -1.1 + t * 0.35; armR = -1.1 - t * 0.35 + swing;
      legL = t * 0.25; legR = -legL;
    }
  }
  // legs (pivot at hip)
  box(r, x, y, z, yaw, -0.11, 0.72, 0, 0.2, 0.72, 0.22, darker(col, 0.55), flash, legL);
  box(r, x, y, z, yaw, 0.11, 0.72, 0, 0.2, 0.72, 0.22, darker(col, 0.55), flash, legR);
  // body
  box(r, x, y, z, yaw, 0, 1.32, 0, 0.5, 0.6, 0.3, col, flash);
  // arms (pivot at shoulder)
  box(r, x, y, z, yaw, -0.34, 1.3, 0, 0.17, 0.58, 0.18, darker(col, 0.85), flash, armL);
  box(r, x, y, z, yaw, 0.34, 1.3, 0, 0.17, 0.58, 0.18, darker(col, 0.85), flash, armR);
  // head
  box(r, x, y, z, yaw, 0, 1.76, 0, 0.42, 0.42, 0.42, SKIN, flash);
  box(r, x, y, z, yaw, -0.09, 1.64, 0.212, 0.06, 0.08, 0.02, EYE, 0);
  box(r, x, y, z, yaw, 0.09, 1.64, 0.212, 0.06, 0.08, 0.02, EYE, 0);
  // hair / hat
  if (e.hat === 'tall') { box(r, x, y, z, yaw, 0, 2.12, 0, 0.3, 0.36, 0.3, [0.2, 0.15, 0.12]); box(r, x, y, z, yaw, 0, 1.8, 0, 0.5, 0.06, 0.5, [0.2, 0.15, 0.12]); }
  else if (e.hat === 'helm') box(r, x, y, z, yaw, 0, 1.84, 0, 0.46, 0.18, 0.46, [0.55, 0.6, 0.66]);
  else if (e.hat === 'cap') box(r, x, y, z, yaw, 0, 1.82, 0.03, 0.46, 0.12, 0.5, [0.85, 0.75, 0.3]);
  else box(r, x, y, z, yaw, 0, 1.8, -0.02, 0.44, 0.1, 0.44, darker(col, 0.4));
  const c = Math.cos(yaw), s = Math.sin(yaw);
  // Held item in the right hand, moving with the arm.
  if (e.held) drawHeld(r, e.held, x + c * 0.34, y + 1.3, z - s * 0.34, yaw, armR);
  // The lantern hangs from the left hand: it follows the hand as the arm
  // swings, but stays upright (with a little sway), like a real lantern.
  if (e.lantern) {
    const sx = x - c * 0.34, sz = z + s * 0.34;             // left shoulder
    const fwd = -0.56 * Math.sin(armL), down = 0.56 * Math.cos(armL); // hand, in the body's frame
    drawHeld(r, LANTERN, sx + s * fwd, y + 1.3 - down, sz + c * fwd, yaw, armL * 0.25, 0, 0.17, 0);
  }
}

export function drawBramblet(r, e, t) {
  const hop = Math.abs(Math.sin(t * 6 + e.id)) * (e.moving ? 0.25 : 0.05);
  const { x, z, yaw } = e, y = e.y + hop;
  const f = e.flash > 0 ? 1 : 0;
  box(r, x, y, z, yaw, 0, 0.62, 0, 0.62, 0.55, 0.62, [0.42, 0.6, 0.28], f);
  box(r, x, y, z, yaw, 0, 0.78, 0, 0.5, 0.16, 0.5, [0.55, 0.72, 0.32], f);
  for (const [ox, oz] of [[0.34, 0], [-0.34, 0], [0, -0.34], [0.22, 0.24], [-0.22, 0.24]]) box(r, x, y, z, yaw, ox, 0.66, oz, 0.08, 0.16, 0.08, [0.85, 0.5, 0.6], f);
  box(r, x, y, z, yaw, -0.13, 0.55, 0.315, 0.1, 0.12, 0.02, EYE);
  box(r, x, y, z, yaw, 0.13, 0.55, 0.315, 0.1, 0.12, 0.02, EYE);
  box(r, x, y, z, yaw, 0, 0.92, 0, 0.12, 0.14, 0.12, [0.95, 0.6, 0.75], f); // little flower on top
}

export function drawWisp(r, e, t) {
  const { x, z } = e, y = e.y + 0.9 + Math.sin(t * 2 + e.id) * 0.15;
  const f = e.flash > 0 ? 1 : 2;
  const col = e.flash > 0 ? [1, 1, 1] : [0.68, 0.48, 1.0];
  r.addBox((m, o) => trs(m, o, x, y, z, t * 1.5, 0.42, 0.42, 0.42), col[0], col[1], col[2], f);
  for (let i = 0; i < 3; i++) {
    const a = t * 2.2 + i * 2.094 + e.id;
    r.addBox((m, o) => trs(m, o, x + Math.cos(a) * 0.45, y - 0.2 + Math.sin(a * 1.3) * 0.1, z + Math.sin(a) * 0.45, a, 0.12, 0.12, 0.12), 0.85, 0.7, 1.0, 2);
  }
  // eyes (dark) facing yaw
  box(r, x, y - 0.06, z, e.yaw, -0.09, 0.05, 0.215, 0.08, 0.1, 0.02, [0.15, 0.05, 0.25]);
  box(r, x, y - 0.06, z, e.yaw, 0.09, 0.05, 0.215, 0.08, 0.1, 0.02, [0.15, 0.05, 0.25]);
}

// ---------- item models (held and dropped) ----------
// Each model is a list of boxes in item space: +Y runs from the grip toward
// the tip, the grip is at the origin. Colors: 'main' is the item's color;
// the rest are fixed materials. Add a model here and to content.Models.
const MAT = { grip: [0.36, 0.23, 0.13], dark: [0.22, 0.2, 0.2], metal: [0.62, 0.64, 0.68], leaf: [0.3, 0.6, 0.25], string: [0.9, 0.9, 0.85], end: [0.84, 0.66, 0.42], glowtip: [1.0, 0.92, 0.6] };
export const ITEM_MODELS = {
  sword: [
    { c: 'grip', p: [0, 0, 0], s: [0.06, 0.22, 0.06] },
    { c: 'shade', p: [0, -0.13, 0], s: [0.09, 0.07, 0.09] },
    { c: 'shade', p: [0, 0.14, 0], s: [0.34, 0.07, 0.1] },
    { c: 'main', p: [0, 0.53, 0], s: [0.17, 0.72, 0.05], glow: true },
    { c: 'main', p: [0, 0.94, 0], s: [0.09, 0.12, 0.05], glow: true },
  ],
  axe: [
    { c: 'grip', p: [0, 0.28, 0], s: [0.06, 0.78, 0.06] },
    { c: 'main', p: [0.1, 0.58, 0], s: [0.2, 0.22, 0.05], glow: true },
    { c: 'shade', p: [0.21, 0.58, 0], s: [0.04, 0.3, 0.05] },
  ],
  pickaxe: [
    { c: 'grip', p: [0, 0.28, 0], s: [0.06, 0.78, 0.06] },
    { c: 'main', p: [0, 0.64, 0], s: [0.56, 0.08, 0.07], glow: true },
  ],
  club: [
    { c: 'grip', p: [0, 0.1, 0], s: [0.07, 0.34, 0.07] },
    { c: 'main', p: [0, 0.5, 0], s: [0.15, 0.48, 0.15], glow: true },
  ],
  lantern: [
    { c: 'dark', p: [0, 0.2, 0], s: [0.12, 0.04, 0.12] },
    { c: 'main', p: [0, 0.08, 0], s: [0.16, 0.2, 0.16], glow: true, always: true },
    { c: 'dark', p: [0, -0.04, 0], s: [0.18, 0.04, 0.18] },
  ],
  gem: [{ c: 'main', p: [0, 0, 0], s: [0.2, 0.26, 0.2], glow: true }],
  food: [
    { c: 'main', p: [0, 0, 0], s: [0.2, 0.2, 0.2] },
    { c: 'leaf', p: [0.03, 0.13, 0], s: [0.1, 0.05, 0.06] },
  ],
  block: [{ c: 'main', p: [0, 0, 0], s: [0.3, 0.3, 0.3] }],
  coin: [{ c: 'main', p: [0, 0, 0], s: [0.24, 0.24, 0.05], glow: true }],
  rod: [
    { c: 'main', p: [0, 0.45, 0], s: [0.04, 1.0, 0.04] },
    { c: 'dark', p: [0.05, 0.12, 0], s: [0.08, 0.08, 0.08] },
    { c: 'string', p: [0.02, 0.6, 0.03], s: [0.01, 0.8, 0.01] },
  ],
  fish: [
    { c: 'main', p: [0, 0, 0], s: [0.1, 0.32, 0.06] },
    { c: 'shade', p: [0, -0.2, 0], s: [0.14, 0.08, 0.04] },
    { c: 'dark', p: [0.03, 0.12, 0.03], s: [0.02, 0.02, 0.01] },
  ],
  tinderbox: [
    { c: 'main', p: [0, 0, 0], s: [0.22, 0.12, 0.16] },
    { c: 'shade', p: [0, 0.07, 0], s: [0.23, 0.03, 0.17] },
  ],
  sling: [
    { c: 'grip', p: [0, 0.2, 0], s: [0.02, 0.4, 0.02] },
    { c: 'main', p: [0, 0.42, 0], s: [0.1, 0.06, 0.08] },
  ],
  bow: [
    { c: 'main', p: [0, 0.3, 0], s: [0.05, 0.3, 0.05] },
    { c: 'main', p: [0, 0.0, 0], s: [0.06, 0.3, 0.06] },
    { c: 'main', p: [0, -0.3, 0], s: [0.05, 0.3, 0.05] },
    { c: 'string', p: [0, 0, 0.12], s: [0.01, 0.9, 0.01] },
  ],
  arrow: [
    { c: 'grip', p: [0, 0.1, 0], s: [0.025, 0.5, 0.025] },
    { c: 'main', p: [0, 0.38, 0], s: [0.06, 0.08, 0.03], glow: true },
    { c: 'string', p: [0, -0.14, 0], s: [0.07, 0.08, 0.01] },
  ],
  wand: [
    { c: 'main', p: [0, 0.22, 0], s: [0.04, 0.5, 0.04] },
    { c: 'glowtip', p: [0, 0.5, 0], s: [0.07, 0.07, 0.07], glow: true, always: true },
  ],
  log: [
    { c: 'main', p: [0, 0, 0], s: [0.16, 0.4, 0.16] },
    { c: 'end', p: [0, 0.205, 0], s: [0.12, 0.01, 0.12] },
  ],
  bundle: [
    { c: 'main', p: [-0.07, 0, 0], s: [0.12, 0.36, 0.12] },
    { c: 'main', p: [0.07, 0, 0], s: [0.12, 0.36, 0.12] },
    { c: 'shade', p: [0, 0, 0], s: [0.3, 0.06, 0.14] },
  ],
};
// Center of each model, so dropped items spin around their middle.
const CENTER = { sword: 0.4, axe: 0.3, pickaxe: 0.35, club: 0.3, rod: 0.45, wand: 0.25, arrow: 0.1 };

export function modelFor(it) {
  if (!it) return 'bundle';
  if (it.model && ITEM_MODELS[it.model]) return it.model;
  return { weapon: 'sword', food: 'food', block: 'block', tool: it.light ? 'lantern' : 'pickaxe', currency: 'coin' }[it.kind] || 'bundle';
}

function partColor(part, col) {
  if (part.c === 'main') return col;
  if (part.c === 'shade') return darker(col, 0.6);
  return MAT[part.c];
}

// Draw an item in a hand. (hx,hy,hz) is the shoulder, yaw/pitch are the arm's;
// the item is held at the end of the arm, tilted forward by `tilt` and rolled
// around its own length by `roll` so a blade shows its flat side.
export function drawHeld(r, it, hx, hy, hz, yaw, pitch, tilt = 0.5, armLen = 0.52, roll = 1.0) {
  const parts = ITEM_MODELS[modelFor(it)], col = rgb(it.color || '#888888');
  // Hand position expressed in the tilted frame: Rx(-tilt) * (0, -armLen, 0).
  const py = -armLen * Math.cos(tilt), pz = armLen * Math.sin(tilt);
  // The roll turns the item frame, so express the hand offset in that frame too.
  const cr = Math.cos(roll), sr = Math.sin(roll), hxo = -sr * pz, hzo = cr * pz;
  const q = pitch + tilt;
  for (const part of parts) {
    const c = partColor(part, col), a = it.glow && part.glow || part.always ? 2 : 0;
    r.addBox((m, o) => trs(m, o, hx, hy, hz, yaw, part.s[0], part.s[1], part.s[2], hxo + part.p[0], py + part.p[1], hzo + part.p[2], q, roll), c[0], c[1], c[2], a);
  }
}

// Draw an item lying in the world: floating, bobbing and slowly turning.
export function drawDropped(r, it, x, y, z, t, seed) {
  const model = modelFor(it), parts = ITEM_MODELS[model], col = rgb(it.color || '#888888');
  const k = 0.7, cy = (CENTER[model] || 0) * k;
  const yaw = t * 1.3 + seed, by = y + 0.45 + Math.sin(t * 2.2 + seed) * 0.06;
  for (const part of parts) {
    const c = partColor(part, col), a = it.glow && part.glow || part.always ? 2 : 0;
    r.addBox((m, o) => trs(m, o, x, by, z, yaw, part.s[0] * k, part.s[1] * k, part.s[2] * k, part.p[0] * k, part.p[1] * k - cy, part.p[2] * k, 0.35), c[0], c[1], c[2], a);
  }
}

// Creature models by content "model" name (content.MobModels must match).
// tag: height of the health bar above the creature's feet.
export const MOB_MODELS = {
  bramblet: { draw: drawBramblet, tag: 1.15 },
  wisp: { draw: drawWisp, tag: 1.7 },
};

// A projectile in flight (bow, sling or wand), from s.a to s.b, started at
// s.t0. Returns false once it has landed, so callers can drop it.
export function drawShot(r, s, time, world) {
  const dx = world.dx(s.a[0], s.b[0]), dy = s.b[1] - s.a[1], dz = s.b[2] - s.a[2];
  const dist = Math.hypot(dx, dy, dz), dur = Math.min(0.5, Math.max(0.12, dist / 28));
  const k = (time - s.t0) / dur;
  if (k >= 1 || k < 0) return k < 0;
  const arc = s.kind === 'salt' ? 0 : Math.sin(k * Math.PI) * dist * 0.06;
  const x = s.a[0] + dx * k, y = s.a[1] + dy * k + arc, z = s.a[2] + dz * k;
  const yaw = Math.atan2(dx, dz), pitch = -Math.atan2(dy + Math.cos(k * Math.PI) * dist * 0.06 * Math.PI, Math.hypot(dx, dz));
  if (s.kind === 'arrow') {
    box(r, x, y, z, yaw, 0, 0, 0, 0.035, 0.035, 0.55, rgb('#6b4423'), 0, pitch);
    box(r, x, y, z, yaw, 0, 0, 0.3, 0.07, 0.07, 0.08, rgb(s.color || '#e09060'), 0, pitch);
  } else if (s.kind === 'salt') {
    // Will-light: a glowing mote with a short trail.
    for (let i = 0; i < 3; i++) {
      const kk = Math.max(0, k - i * 0.08), sz = 0.2 - i * 0.05;
      box(r, s.a[0] + dx * kk, s.a[1] + dy * kk, s.a[2] + dz * kk, yaw, 0, 0, 0, sz, sz, sz, [1.0, 0.9 - i * 0.1, 0.55], 2);
    }
  } else {
    box(r, x, y, z, yaw + time * 9, 0, 0, 0, 0.13, 0.13, 0.13, rgb(s.color || '#9aa3ad'));
  }
  return true;
}
