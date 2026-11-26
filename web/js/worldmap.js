// The world map (a tab in the menu): the level's terrain image, which wraps
// east–west like the world, with towns, danger zones and where you are. Pan
// by dragging, the D-pad/arrow keys or the left stick; zoom with the wheel,
// pinch, or the +/− buttons.
import { drawArrow } from './minimap.js';
import { esc } from './ui.js';

const MIN_ZOOM = 0.15, MAX_ZOOM = 6; // screen pixels per block

// Resource hotspot icons, shared with the minimap.
export const SITE_ICON = { mine: '⛏', wood: '🌲', fishing: '🎣', hunting: '🐾' };
export const SITE_NAME = { mine: 'Mine', wood: 'Wood', fishing: 'Fishing ground', hunting: 'Hunting ground' };

// dangerAt mirrors world.Danger's distance part: the great-circle angle from
// the spawn town, spread over 1..max.
export function dangerAt(world, dv, x, z) {
  if (!dv) return 1;
  const r = Math.PI / 180, lon = (world.wrapX(x) / world.sx) * 360 - 180, lat = 90 - (z / world.sz) * 180;
  const dp = (lat - dv.lat) * r, dl = (lon - dv.lon) * r;
  const h = Math.sin(dp / 2) ** 2 + Math.cos(lat * r) * Math.cos(dv.lat * r) * Math.sin(dl / 2) ** 2;
  const a = 2 * Math.asin(Math.min(1, Math.sqrt(h))) / r;
  return 1 + Math.round((dv.max - 1) * a / 180);
}

// Danger colors: green near home through yellow to deep red far away.
export function dangerColor(d, max) {
  const t = Math.min(1, Math.max(0, (d - 1) / Math.max(1, max - 1)));
  const hue = 120 * (1 - t);
  return `hsl(${hue.toFixed(0)}, 85%, ${(50 - t * 12).toFixed(0)}%)`;
}

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
        <button class="btn ${this.showDanger ? 'primary' : ''}" data-do="mapDanger" data-fid="wm-danger" aria-pressed="${!!this.showDanger}">⚠ Danger</button>
        <button class="btn ${this.showSites !== false ? 'primary' : ''}" data-do="mapSites" data-fid="wm-sites" aria-pressed="${this.showSites !== false}">⛏ Hotspots</button>
        <span class="grow"></span>
        <span class="note" id="wmwhere">${esc(me && me.zoneName ? me.zoneName + ' · ' : '')}${esc(ll)}</span>
      </div>
      <div class="wmlegend note"><span class="k town"></span>Town <span class="k danger"></span>Danger zone <span class="k me"></span>You <span class="k player"></span>Players · ⛏ mine 🌲 wood 🎣 fishing 🐾 hunting${this.showDanger ? ' · <span class="dgrad"></span> danger 1 → ' + ((this.g.worldInfo && this.g.worldInfo.danger && this.g.worldInfo.danger.max) || 10) : ''}</div>
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
    const sites = (this.g.worldInfo && this.g.worldInfo.sites) || [];
    const site = this.showSites !== false && sites.find((s) => w.dist(x, z, s.x, s.z) <= Math.max(s.r, 8 / this.zoom));
    let text;
    if (site) text = `${SITE_ICON[site.kind]} ${SITE_NAME[site.kind]}${site.what && site.what.length ? ': ' + site.what.join(', ') : ''} · danger ${site.danger}`;
    else if (zone && zone.kind === 'town') text = `${zone.name}${zone.known && zone.known.length ? ' · known for ' + zone.known.map((k) => this.g.ui.skillName(k)).join(', ') : ''}${(this.g.me.visited || []).includes(zone.id) ? ' · visited' : ''}`;
    else text = `${zone ? zone.name + ' · ' : ''}danger ${dangerAt(w, this.g.worldInfo && this.g.worldInfo.danger, x, z)}`;
    el.textContent = `${text} · ${this.latLon(x, z)}`;
  }

  // A tint over the land by danger level, from a small image built once
  // (one pixel per 16 blocks) and stretched over the map.
  drawDanger(ctx, W, H, z, cx, cz) {
    const w = this.g.world, dv = this.g.worldInfo && this.g.worldInfo.danger;
    if (!w || !dv) return;
    if (!this.dangerImg) {
      const cw = Math.ceil(w.sx / 16), ch = Math.ceil(w.sz / 16);
      const c = document.createElement('canvas'); c.width = cw; c.height = ch;
      const cc = c.getContext('2d');
      for (let j = 0; j < ch; j++) for (let i = 0; i < cw; i++) {
        cc.fillStyle = dangerColor(dangerAt(w, dv, i * 16 + 8, j * 16 + 8), dv.max);
        cc.fillRect(i, j, 1, 1);
      }
      this.dangerImg = c;
    }
    const iw = w.sx * z, ih = w.sz * z;
    let x0 = W / 2 - cx * z;
    x0 -= Math.ceil(x0 / iw) * iw;
    ctx.save();
    ctx.globalAlpha = 0.38;
    ctx.imageSmoothingEnabled = true;
    for (let x = x0; x < W; x += iw) ctx.drawImage(this.dangerImg, x, H / 2 - cz * z, iw, ih);
    ctx.restore();
  }

  // Continent and ocean names. Every copy of the wrapping map that's on screen
  // gets its labels; small seas only appear once zoomed in. A label is skipped
  // where it would cover a town or zone name, your marker, or another label
  // (boxes: [centerX, centerY, width, height]); continents win over seas.
  drawRegionLabels(ctx, W, H, z, dpr, toX, toY, boxes) {
    const regions = (this.g.worldInfo && this.g.worldInfo.regions) || [];
    const iw = this.g.world.sx * z;
    ctx.save();
    ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
    const hits = (b) => boxes.some((o) => Math.abs(o[0] - b[0]) < (o[2] + b[2]) / 2 && Math.abs(o[1] - b[1]) < (o[3] + b[3]) / 2);
    for (const r of [...regions].sort((a, b) => a.sea - b.sea)) {
      if (r.minZoom && this.zoom < r.minZoom) continue;
      const size = Math.round(Math.min(22, Math.max(11, 9 + this.zoom * 6)) * dpr * (r.sea ? 0.85 : 1));
      ctx.font = r.sea ? `italic 600 ${size}px Georgia, serif` : `800 ${size}px system-ui, sans-serif`;
      const text = r.sea ? r.name : r.name.toUpperCase();
      const py = toY(r.z);
      if (py < -size || py > H + size) continue;
      for (let px = toX(r.x) - Math.ceil(W / iw + 1) * iw; px < W + iw; px += iw) {
        if (px < -W || px > 2 * W) continue;
        if ('letterSpacing' in ctx) ctx.letterSpacing = r.sea ? '0px' : `${Math.round(size * 0.18)}px`;
        const box = [px, py, ctx.measureText(text).width + 6 * dpr, size * 1.2];
        if (hits(box)) continue;
        boxes.push(box);
        ctx.lineWidth = 3 * dpr; ctx.strokeStyle = r.sea ? 'rgba(10,30,60,.55)' : 'rgba(0,0,0,.55)';
        ctx.strokeText(text, px, py);
        ctx.fillStyle = r.sea ? 'rgba(190,225,255,.85)' : 'rgba(255,255,255,.92)';
        ctx.fillText(text, px, py);
      }
    }
    ctx.restore();
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
    if (this.showDanger) this.drawDanger(ctx, W, H, z, cx, cz);
    // Zones and town names (a name is skipped if it would overlap one already drawn).
    ctx.textAlign = 'center'; ctx.textBaseline = 'bottom';
    const labels = [];
    // Zoomed out, continent names matter more than town names; zoomed in, the
    // other way round. Whichever is drawn first wins any overlap.
    const regionsFirst = this.zoom < 0.5;
    if (regionsFirst) { this.drawRegionLabels(ctx, W, H, z, dpr, toX, toY, labels); ctx.textAlign = 'center'; ctx.textBaseline = 'bottom'; }
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
      const box = [px, ly - lh / 2, tw, lh];
      if (labels.some((o) => Math.abs(o[0] - box[0]) < (o[2] + box[2]) / 2 && Math.abs(o[1] - box[1]) < (o[3] + box[3]) / 2)) continue;
      labels.push(box);
      ctx.lineWidth = 3 * dpr; ctx.strokeStyle = 'rgba(0,0,0,.7)';
      ctx.strokeText(zn.name, px, py - r - 3 * dpr);
      ctx.fillStyle = danger ? '#ffc2dc' : '#fff1c8';
      ctx.fillText(zn.name, px, py - r - 3 * dpr);
    }
    if (this.showSites !== false) {
      // Hotspot icons: big enough to read when zoomed in, dots when zoomed out.
      const sites = (g.worldInfo && g.worldInfo.sites) || [];
      ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
      for (const s of sites) {
        const px = toX(s.x), py = toY(s.z);
        if (px < -20 || py < -20 || px > W + 20 || py > H + 20) continue;
        if (this.zoom < 0.35) {
          ctx.beginPath(); ctx.arc(px, py, 2 * dpr, 0, Math.PI * 2);
          ctx.fillStyle = { mine: '#c9c3b8', wood: '#4fbf5a', fishing: '#7cc4ff', hunting: '#e0a060' }[s.kind] || '#fff'; ctx.fill();
          continue;
        }
        ctx.font = `${Math.round(13 * dpr)}px system-ui, sans-serif`;
        ctx.fillText(SITE_ICON[s.kind] || '•', px, py);
      }
      ctx.textAlign = 'center'; ctx.textBaseline = 'bottom';
    }
    if (g.body) labels.push([toX(g.body.x), toY(g.body.z), 22 * dpr, 22 * dpr]);
    if (!regionsFirst) this.drawRegionLabels(ctx, W, H, z, dpr, toX, toY, labels);
    for (const p of g.others.values()) {
      ctx.beginPath(); ctx.arc(toX(p.x), toY(p.z), 4 * dpr, 0, Math.PI * 2);
      ctx.fillStyle = '#7cc4ff'; ctx.fill(); ctx.strokeStyle = 'rgba(0,0,0,.7)'; ctx.lineWidth = dpr; ctx.stroke();
    }
    if (g.body) drawArrow(ctx, toX(g.body.x), toY(g.body.z), g.bodyYaw || 0, 9 * dpr, '#ffe27a');
  }
}
