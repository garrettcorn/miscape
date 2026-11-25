import { trs } from './mat.js';

const cache = new Map();
export function rgb(hex) {
  let c = cache.get(hex);
  if (!c) { c = [parseInt(hex.slice(1, 3), 16) / 255, parseInt(hex.slice(3, 5), 16) / 255, parseInt(hex.slice(5, 7), 16) / 255]; cache.set(hex, c); }
  return c;
}
const darker = (c, k) => [c[0] * k, c[1] * k, c[2] * k];
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
export function drawPerson(r, e) {
  const { x, y, z, yaw } = e;
  const col = rgb(e.color || '#4a90d9');
  const flash = e.flash > 0 ? 1 : 0;
  const walk = Math.sin(e.walk || 0) * 0.7 * (e.moving ? 1 : 0);
  const swing = e.swing > 0 ? -Math.sin(e.swing * Math.PI) * 1.8 : 0;
  // legs (pivot at hip)
  box(r, x, y, z, yaw, -0.11, 0.72, 0, 0.2, 0.72, 0.22, darker(col, 0.55), flash, walk);
  box(r, x, y, z, yaw, 0.11, 0.72, 0, 0.2, 0.72, 0.22, darker(col, 0.55), flash, -walk);
  // body
  box(r, x, y, z, yaw, 0, 1.32, 0, 0.5, 0.6, 0.3, col, flash);
  // arms (pivot at shoulder)
  box(r, x, y, z, yaw, -0.34, 1.3, 0, 0.17, 0.58, 0.18, darker(col, 0.85), flash, -walk * 0.8);
  box(r, x, y, z, yaw, 0.34, 1.3, 0, 0.17, 0.58, 0.18, darker(col, 0.85), flash, walk * 0.8 + swing);
  // head
  box(r, x, y, z, yaw, 0, 1.76, 0, 0.42, 0.42, 0.42, SKIN, flash);
  box(r, x, y, z, yaw, -0.09, 1.64, 0.212, 0.06, 0.08, 0.02, EYE, 0);
  box(r, x, y, z, yaw, 0.09, 1.64, 0.212, 0.06, 0.08, 0.02, EYE, 0);
  // hair / hat
  if (e.hat === 'tall') { box(r, x, y, z, yaw, 0, 2.12, 0, 0.3, 0.36, 0.3, [0.2, 0.15, 0.12]); box(r, x, y, z, yaw, 0, 1.8, 0, 0.5, 0.06, 0.5, [0.2, 0.15, 0.12]); }
  else if (e.hat === 'helm') box(r, x, y, z, yaw, 0, 1.84, 0, 0.46, 0.18, 0.46, [0.55, 0.6, 0.66]);
  else if (e.hat === 'cap') box(r, x, y, z, yaw, 0, 1.82, 0.03, 0.46, 0.12, 0.5, [0.85, 0.75, 0.3]);
  else box(r, x, y, z, yaw, 0, 1.8, -0.02, 0.44, 0.1, 0.44, darker(col, 0.4));
  // weapon in right hand
  if (e.weapon) {
    const wc = rgb(e.weapon);
    const c = Math.cos(yaw), s = Math.sin(yaw);
    const hx = x + c * 0.34, hz = z - s * 0.34;
    const p = walk * 0.8 + swing;
    r.addBox((m, o) => trs(m, o, hx, y + 1.3, hz, yaw, 0.07, 0.7, 0.07, 0, -0.55 - 0.35, 0.18, p - 1.2), wc[0], wc[1], wc[2], e.weaponGlow ? 2 : 0);
  }
  if (e.lantern) {
    const c = Math.cos(yaw), s = Math.sin(yaw);
    const hx = x - c * 0.4 + s * 0.12, hz = z + s * 0.4 + c * 0.12;
    r.addBox((m, o) => trs(m, o, hx, y + 0.78, hz, yaw, 0.16, 0.2, 0.16), 1.0, 0.85, 0.5, 2);
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
