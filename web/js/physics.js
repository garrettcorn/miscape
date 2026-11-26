import { B, BLOCKS, isWater } from './blocks.js';

// Shared with internal/game/move.go — keep in sync.
export const WALK = 4.6, SPRINT = 6.8, HALF = 0.3, HEIGHT = 1.7;
const GRAVITY = 25, JUMP = 8.0;

export class Body {
  constructor(x, y, z) { this.x = x; this.y = y; this.z = z; this.vy = 0; this.onGround = false; this.inWater = false; this.stepAnim = 0; }
}

function blocked(world, x, y, z) {
  for (const dy of [0.02, 0.9, HEIGHT - 0.02]) for (const dx of [-HALF + 0.01, HALF - 0.01]) for (const dz of [-HALF + 0.01, HALF - 0.01]) {
    if (world.solidAt(x + dx, y + dy, z + dz)) return true;
  }
  return false;
}

function feetBlocked(world, x, y, z) {
  for (const dx of [-HALF + 0.01, HALF - 0.01]) for (const dz of [-HALF + 0.01, HALF - 0.01]) {
    if (world.solidAt(x + dx, y, z + dz)) return true;
  }
  return false;
}

// Swimming into a bank pulls you up onto it, if the ledge is within reach
// (MAX_CLIMB, matching the server's MaxJumpRise) and there is room to stand.
const MAX_CLIMB = 1.7;
function climbOut(world, b, nx, nz) {
  for (let top = Math.floor(b.y) + 1; top - b.y <= MAX_CLIMB; top++) {
    if (!blocked(world, nx, top, nz) && !blocked(world, b.x, top, b.z) && feetBlocked(world, nx, top - 0.05, nz)) {
      b.y = top; b.vy = 0; b.onGround = true; b.stepAnim = 1;
      return true;
    }
  }
  return false;
}

// Step the body by desired horizontal velocity (vx, vz) for dt seconds.
export function step(world, b, vx, vz, jump, dt) {
  // Don't fall into unloaded terrain.
  if (!world.columnLoaded(b.x, b.z)) return;
  const feet = world.get(Math.floor(b.x), Math.floor(b.y + 0.4), Math.floor(b.z));
  b.inWater = isWater(feet);
  if (b.inWater) {
    // Float: while the chest is under water you rise, above it you sink a
    // little, so a swimmer bobs with their head out. Jump swims up faster.
    const chest = isWater(world.get(Math.floor(b.x), Math.floor(b.y + 1.25), Math.floor(b.z)));
    b.vy = chest ? Math.min(b.vy + 9 * dt, 1.6) : Math.max(b.vy - GRAVITY * 0.25 * dt, -2.5);
    if (jump) b.vy = Math.max(b.vy, 3.2);
    vx *= 0.6; vz *= 0.6;
  } else {
    b.vy -= GRAVITY * dt;
    if (jump && b.onGround) { b.vy = JUMP; b.onGround = false; }
  }
  b.vy = Math.max(b.vy, -40);
  // Sub-step to avoid tunneling.
  const steps = Math.ceil(Math.max(Math.abs(vx), Math.abs(vz), Math.abs(b.vy)) * dt / 0.2) || 1;
  const h = dt / steps;
  for (let i = 0; i < steps; i++) {
    // X
    let nx = b.x + vx * h;
    if (blocked(world, nx, b.y, b.z)) {
      if (b.onGround && !blocked(world, nx, b.y + 1.01, b.z) && !blocked(world, b.x, b.y + 1.01, b.z)) { b.y += 1.01; b.stepAnim = 1; b.x = nx; }
      else if (b.inWater && climbOut(world, b, nx, b.z)) b.x = nx;
      else vx = 0;
    } else b.x = nx;
    // Z
    let nz = b.z + vz * h;
    if (blocked(world, b.x, b.y, nz)) {
      if (b.onGround && !blocked(world, b.x, b.y + 1.01, nz) && !blocked(world, b.x, b.y + 1.01, b.z)) { b.y += 1.01; b.stepAnim = 1; b.z = nz; }
      else if (b.inWater && climbOut(world, b, b.x, nz)) b.z = nz;
      else vz = 0;
    } else b.z = nz;
    // Y. Falling checks the exact sole of the feet: blocked() samples 2 cm up
    // (so walking doesn't snag on the floor), and at high frame rates a
    // frame's fall is smaller than that, which made the body sink 1–2 cm and
    // pop back up every frame — the whole view shook while standing still.
    const ny = b.y + b.vy * h;
    if (b.vy < 0 && feetBlocked(world, b.x, ny, b.z)) {
      b.y = Math.floor(ny) + 1; b.vy = 0; b.onGround = true;
    } else if (blocked(world, b.x, ny, b.z)) {
      if (b.vy < 0) { b.y = Math.floor(ny) + 1; b.onGround = true; }
      b.vy = 0;
    } else { b.y = ny; b.onGround = false; }
  }
  if (!b.onGround && b.vy <= 0 && blocked(world, b.x, b.y - 0.05, b.z)) b.onGround = true;
  b.x = world.wrapX(b.x); // east–west wraps around the world
  b.z = Math.min(Math.max(b.z, 0.35), world.sz - 0.35); // the poles are walls
  if (b.y < -5) { b.y = world.surfaceY(Math.floor(b.x), Math.floor(b.z)) + 0.1; b.vy = 0; }
}

// Voxel DDA raycast. Returns {x,y,z, face:[nx,ny,nz], dist, block} or null.
export function raycast(world, ox, oy, oz, dx, dy, dz, maxDist, pickable = (b) => b !== B.AIR && b !== B.WATER) {
  let x = Math.floor(ox), y = Math.floor(oy), z = Math.floor(oz);
  const sx = dx > 0 ? 1 : -1, sy = dy > 0 ? 1 : -1, sz = dz > 0 ? 1 : -1;
  const tdx = Math.abs(1 / dx), tdy = Math.abs(1 / dy), tdz = Math.abs(1 / dz);
  let tx = (dx > 0 ? x + 1 - ox : ox - x) * tdx;
  let ty = (dy > 0 ? y + 1 - oy : oy - y) * tdy;
  let tz = (dz > 0 ? z + 1 - oz : oz - z) * tdz;
  let face = [0, 0, 0], t = 0;
  for (let i = 0; i < 256 && t <= maxDist; i++) {
    const b = world.get(x, y, z);
    if (world.inside(x, y, z) && pickable(b)) return { x, y, z, face, dist: t, block: b };
    if (tx < ty && tx < tz) { x += sx; t = tx; tx += tdx; face = [-sx, 0, 0]; }
    else if (ty < tz) { y += sy; t = ty; ty += tdy; face = [0, -sy, 0]; }
    else { z += sz; t = tz; tz += tdz; face = [0, 0, -sz]; }
  }
  return null;
}

// Camera collision: shorten the boom so the camera never goes inside terrain.
export function cameraBoom(world, px, py, pz, dx, dy, dz, want) {
  const hit = raycast(world, px, py, pz, dx, dy, dz, want, (b) => BLOCKS[b] && BLOCKS[b].opaque);
  return hit ? Math.max(0.6, hit.dist - 0.25) : want;
}
