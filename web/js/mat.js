// Minimal column-major 4x4 matrix helpers.
export function perspective(fovy, aspect, near, far) {
  const f = 1 / Math.tan(fovy / 2), nf = 1 / (near - far);
  return new Float32Array([f / aspect, 0, 0, 0, 0, f, 0, 0, 0, 0, (far + near) * nf, -1, 0, 0, 2 * far * near * nf, 0]);
}
export function lookAt(eye, center, up) {
  let zx = eye[0] - center[0], zy = eye[1] - center[1], zz = eye[2] - center[2];
  let l = Math.hypot(zx, zy, zz) || 1; zx /= l; zy /= l; zz /= l;
  let xx = up[1] * zz - up[2] * zy, xy = up[2] * zx - up[0] * zz, xz = up[0] * zy - up[1] * zx;
  l = Math.hypot(xx, xy, xz) || 1; xx /= l; xy /= l; xz /= l;
  const yx = zy * xz - zz * xy, yy = zz * xx - zx * xz, yz = zx * xy - zy * xx;
  return new Float32Array([xx, yx, zx, 0, xy, yy, zy, 0, xz, yz, zz, 0,
    -(xx * eye[0] + xy * eye[1] + xz * eye[2]), -(yx * eye[0] + yy * eye[1] + yz * eye[2]), -(zx * eye[0] + zy * eye[1] + zz * eye[2]), 1]);
}
export function mul(a, b) {
  const o = new Float32Array(16);
  for (let c = 0; c < 4; c++) for (let r = 0; r < 4; r++) {
    o[c * 4 + r] = a[r] * b[c * 4] + a[4 + r] * b[c * 4 + 1] + a[8 + r] * b[c * 4 + 2] + a[12 + r] * b[c * 4 + 3];
  }
  return o;
}
// Model matrix: translate, rotate around Y, then scale, with a pivot offset.
export function trs(out, o, tx, ty, tz, ry, sx, sy, sz, px = 0, py = 0, pz = 0, rx = 0) {
  const c = Math.cos(ry), s = Math.sin(ry), cx = Math.cos(rx), sxr = Math.sin(rx);
  // R = Ry * Rx
  const r00 = c, r01 = s * sxr, r02 = s * cx;
  const r10 = 0, r11 = cx, r12 = -sxr;
  const r20 = -s, r21 = c * sxr, r22 = c * cx;
  out[o] = r00 * sx; out[o + 1] = r10 * sx; out[o + 2] = r20 * sx; out[o + 3] = 0;
  out[o + 4] = r01 * sy; out[o + 5] = r11 * sy; out[o + 6] = r21 * sy; out[o + 7] = 0;
  out[o + 8] = r02 * sz; out[o + 9] = r12 * sz; out[o + 10] = r22 * sz; out[o + 11] = 0;
  // pivot (in rotated space) then translation
  out[o + 12] = tx + r00 * px + r01 * py + r02 * pz;
  out[o + 13] = ty + r10 * px + r11 * py + r12 * pz;
  out[o + 14] = tz + r20 * px + r21 * py + r22 * pz;
  out[o + 15] = 1;
}
export function project(vp, x, y, z, w, h) {
  const cx = vp[0] * x + vp[4] * y + vp[8] * z + vp[12];
  const cy = vp[1] * x + vp[5] * y + vp[9] * z + vp[13];
  const cw = vp[3] * x + vp[7] * y + vp[11] * z + vp[15];
  if (cw <= 0.01) return null;
  return [(cx / cw * 0.5 + 0.5) * w, (1 - (cy / cw * 0.5 + 0.5)) * h, cw];
}
export function frustumPlanes(m) {
  const p = [];
  const row = (i) => [m[i], m[4 + i], m[8 + i], m[12 + i]];
  const r0 = row(0), r1 = row(1), r2 = row(2), r3 = row(3);
  for (const [a, s] of [[r0, 1], [r0, -1], [r1, 1], [r1, -1], [r2, 1], [r2, -1]]) {
    p.push([r3[0] + s * a[0], r3[1] + s * a[1], r3[2] + s * a[2], r3[3] + s * a[3]]);
  }
  return p;
}
export function aabbVisible(planes, x0, y0, z0, x1, y1, z1) {
  for (const p of planes) {
    const x = p[0] > 0 ? x1 : x0, y = p[1] > 0 ? y1 : y0, z = p[2] > 0 ? z1 : z0;
    if (p[0] * x + p[1] * y + p[2] * z + p[3] < 0) return false;
  }
  return true;
}
