// The world map (a tab in the menu): the level's terrain image, which wraps
// east–west like the world, with towns, danger zones and where you are. Pan
// by dragging, the D-pad/arrow keys or the left stick; zoom with the wheel,
// pinch, or the +/− buttons.
import { drawArrow } from './minimap.js';
import { esc } from './ui.js';

const MIN_ZOOM = 0.15, MAX_ZOOM = 6; // screen pixels per block

export class WorldMap {
  constructor(game) {
    this.g = game;
    this.img = null;
    this.loaded = false;
    this.zoom = 1.5;
    this.center = null; // [x, z] in blocks; null = follow the player
  }

  load() {
    const m = this.g.worldInfo && this.g.worldInfo.map;
    if (!m || (this.img && this.img.dataset.url === m.url)) return;
    this.img = new Image();
    this.img.dataset.url = m.url;
    this.img.onload = () => { this.loaded = true; this.draw(); };
    this.img.onerror = () => { this.failed = true; this.draw(); };
    this.loaded = false;
    this.img.src = m.url;
    // Optional relief + ocean-depth layer (grey, 128 = flat), blended on top.
    this.relief = null;
    if (m.relief) {
      const r = new Image();
      r.onload = () => { this.relief = r; this.draw(); };
      r.src = m.relief;
    }
  }

  html() {
    const me = this.g.me, b = this.g.body;
    const ll = b ? this.latLon(b.x, b.z) : '';
    return `<div class="worldmap">
      <canvas id="wmcanvas" tabindex="0" data-fid="wm" aria-label="World map. Use the arrow keys or D-pad to pan."></canvas>
      <div class="wmbar row">
        <button class="btn" data-do="mapZoom" data-arg="1" data-fid="wm-in" aria-label="Zoom in">＋</button>
        <button class="btn" data-do="mapZoom" data-arg="-1" data-fid="wm-out" aria-label="Zoom out">－</button>
        <button class="btn" data-do="mapCenter" data-fid="wm-me">◎ Center on me</button>
        <span class="grow"></span>
        <span class="note" id="wmwhere">${esc(me && me.zoneName ? me.zoneName + ' · ' : '')}${esc(ll)}</span>
      </div>
      <div class="wmlegend note"><span class="k town"></span>Town <span class="k danger"></span>Danger zone <span class="k me"></span>You <span class="k player"></span>Players</div>
    </div>`;
  }

  // Called after the tab renders: hook up input and draw.
  attach() {
    this.load();
    const cv = document.getElementById('wmcanvas');
    if (!cv) return;
    this.canvas = cv;
    let drag = null;
    const pts = new Map();
    cv.onpointerdown = (e) => { cv.setPointerCapture(e.pointerId); pts.set(e.pointerId, [e.clientX, e.clientY]); drag = { x: e.clientX, y: e.clientY }; };
    cv.onpointermove = (e) => {
      const prev = pts.get(e.pointerId);
      if (!prev) { this.hover(e); return; }
      if (pts.size === 2) {
        // Pinch to zoom.
        const [a, b2] = [...pts.values()];
        const before = Math.hypot(a[0] - b2[0], a[1] - b2[1]);
        pts.set(e.pointerId, [e.clientX, e.clientY]);
        const [c, d] = [...pts.values()];
        const after = Math.hypot(c[0] - d[0], c[1] - d[1]);
        if (before > 0) this.setZoom(this.zoom * after / before);
        return;
      }
      pts.set(e.pointerId, [e.clientX, e.clientY]);
      if (drag) { this.pan(-(e.clientX - drag.x) / this.zoom, -(e.clientY - drag.y) / this.zoom); drag = { x: e.clientX, y: e.clientY }; }
    };
    const up = (e) => { pts.delete(e.pointerId); if (!pts.size) drag = null; };
    cv.onpointerup = up; cv.onpointercancel = up;
    cv.onwheel = (e) => { e.preventDefault(); this.setZoom(this.zoom * (e.deltaY < 0 ? 1.25 : 0.8)); };
    this.draw();
  }

  view() {
    const b = this.g.body;
    return this.center || (b ? [b.x, b.z] : [0, 0]);
  }

  pan(dx, dz) {
    const w = this.g.world, [x, z] = this.view();
    this.center = [w.wrapX(x + dx), Math.min(w.sz, Math.max(0, z + dz))];
    this.draw();
  }

  setZoom(z) { this.zoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, z)); this.draw(); }

  // Arrow keys / D-pad while the map has focus.
  nav(dir) {
    const step = 120 / this.zoom;
    const d = { left: [-step, 0], right: [step, 0], up: [0, -step], down: [0, step] }[dir];
    if (d) this.pan(d[0], d[1]);
  }

  latLon(x, z) {
    const w = this.g.world;
    if (!w) return '';
    const lon = (w.wrapX(x) / w.sx) * 360 - 180, lat = 90 - (z / w.sz) * 180;
    return `${Math.abs(lat).toFixed(1)}°${lat >= 0 ? 'N' : 'S'} ${Math.abs(lon).toFixed(1)}°${lon >= 0 ? 'E' : 'W'}`;
  }

  hover(e) {
    const cv = this.canvas, w = this.g.world, el = document.getElementById('wmwhere');
    if (!cv || !w || !el) return;
    const r = cv.getBoundingClientRect(), [cx, cz] = this.view();
    const x = cx + (e.clientX - r.left - r.width / 2) / this.zoom, z = cz + (e.clientY - r.top - r.height / 2) / this.zoom;
    if (z < 0 || z > w.sz) return;
    const zone = (this.g.zones || []).find((zn) => w.dist(x, z, zn.x, zn.z) <= zn.r);
    el.textContent = `${zone ? zone.name + ' · ' : ''}${this.latLon(x, z)}`;
  }

  draw() {
    const cv = this.canvas, g = this.g, w = g.world;
    if (!cv || !document.body.contains(cv) || !w) return;
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    const W = Math.round(cv.clientWidth * dpr), H = Math.round(cv.clientHeight * dpr);
    if (!W || !H) return;
    if (cv.width !== W || cv.height !== H) { cv.width = W; cv.height = H; }
    const ctx = cv.getContext('2d');
    ctx.fillStyle = '#16304f';
    ctx.fillRect(0, 0, W, H);
    const z = this.zoom * dpr, [cx, cz] = this.view();
    const toY = (wz) => H / 2 + (wz - cz) * z;
    // Terrain: the image covers sx x sz blocks; repeat it east–west.
    if (this.loaded) {
      ctx.imageSmoothingEnabled = this.zoom < 1;
      const iw = w.sx * z, ih = w.sz * z;
      let x0 = W / 2 - cx * z;
      x0 -= Math.ceil(x0 / iw) * iw; // first copy starting at or left of 0
      for (let x = x0; x < W; x += iw) ctx.drawImage(this.img, x, toY(0), iw, ih);
      if (this.relief) {
        // Overlay keeps the terrain colors but lightens ridges and shallow seas
        // and darkens shadowed slopes and the deep ocean.
        ctx.save();
        ctx.globalCompositeOperation = 'overlay';
        ctx.globalAlpha = 0.9;
        ctx.imageSmoothingEnabled = true;
        for (let x = x0; x < W; x += iw) ctx.drawImage(this.relief, x, toY(0), iw, ih);
        ctx.restore();
      }
    } else {
      ctx.fillStyle = 'rgba(255,255,255,.7)'; ctx.font = `${14 * dpr}px system-ui, sans-serif`; ctx.textAlign = 'center';
      ctx.fillText(this.failed ? 'The map couldn’t be loaded.' : 'Loading map…', W / 2, H / 2);
    }
    const toX = (wx) => W / 2 + w.dx(cx, wx) * z;
    // Zones and town names (a name is skipped if it would overlap one already drawn).
    ctx.textAlign = 'center'; ctx.textBaseline = 'bottom';
    const labels = [];
    for (const zn of g.zones || []) {
      const px = toX(zn.x), py = toY(zn.z), r = Math.max(zn.r * z, 4 * dpr);
      ctx.beginPath(); ctx.arc(px, py, r, 0, Math.PI * 2);
      const danger = zn.kind === 'danger';
      ctx.fillStyle = danger ? 'rgba(120,40,140,.35)' : 'rgba(255,207,107,.25)';
      ctx.fill();
      ctx.lineWidth = 2 * dpr; ctx.strokeStyle = danger ? '#ff6fa8' : '#ffcf6b';
      ctx.setLineDash(danger ? [5 * dpr, 4 * dpr] : []);
      ctx.stroke(); ctx.setLineDash([]);
      ctx.font = `700 ${12 * dpr}px system-ui, sans-serif`;
      const tw = ctx.measureText(zn.name).width, ly = py - r - 3 * dpr, lh = 14 * dpr;
      if (labels.some((l) => Math.abs(l[0] - px) < (l[2] + tw) / 2 && Math.abs(l[1] - ly) < lh)) continue;
      labels.push([px, ly, tw]);
      ctx.lineWidth = 3 * dpr; ctx.strokeStyle = 'rgba(0,0,0,.7)';
      ctx.strokeText(zn.name, px, py - r - 3 * dpr);
      ctx.fillStyle = danger ? '#ffc2dc' : '#fff1c8';
      ctx.fillText(zn.name, px, py - r - 3 * dpr);
    }
    for (const p of g.others.values()) {
      ctx.beginPath(); ctx.arc(toX(p.x), toY(p.z), 4 * dpr, 0, Math.PI * 2);
      ctx.fillStyle = '#7cc4ff'; ctx.fill(); ctx.strokeStyle = 'rgba(0,0,0,.7)'; ctx.lineWidth = dpr; ctx.stroke();
    }
    if (g.body) drawArrow(ctx, toX(g.body.x), toY(g.body.z), g.bodyYaw || 0, 9 * dpr, '#ffe27a');
  }
}
