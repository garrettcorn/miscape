import { Renderer } from './renderer.js';
import { World } from './world.js';
import { meshChunk, meshChunkLOD } from './mesher.js';
import { Body, step, raycast, cameraBoom, WALK, SPRINT } from './physics.js';
import { Input, glyph } from './input.js';
import { Net, WorkerNet, PROTO, CLIENT } from './net.js';
import { UI, esc, applyAccessibility } from './ui.js';
import { settings, quality, save } from './settings.js';
import { drawPerson, drawDropped, drawShot, drawFlag, MOB_MODELS } from './models.js';
import { project } from './mat.js';
import { showJoin } from './join.js';
import { MiniMap } from './minimap.js';
import { WorldMap } from './worldmap.js';
import { B, BLOCKS, isWater, COLLECT } from './blocks.js';

// Station blocks (must match game.stationBlocks).
const STATIONS = { anvil: [B.ANVIL], fire: [B.CAMPFIRE, B.RANGE] };

const $ = (s) => document.querySelector(s);
const params = new URLSearchParams(location.search);
const BROWSER_WORLD = document.querySelector('meta[name="lanternfall-mode"]')?.content === 'browser';

function fatal(msg) {
  const f = $('#fatal');
  f.hidden = false;
  f.textContent = msg;
}

class Game {
  constructor() {
    this.canvas = $('#gl');
    try { this.r = new Renderer(this.canvas); } catch (e) { fatal(e.message); throw e; }
    this.input = new Input(this.canvas);
    // Browser-world mode: the world runs in a Web Worker on this page (the
    // build marks the page with <meta name="lanternfall-mode" content="browser">).
    this.browserWorld = BROWSER_WORLD;
    this.net = BROWSER_WORLD ? new WorkerNet() : new Net();
    if (BROWSER_WORLD) {
      this.net.onStatus = (m) => {
        if (m.t === 'saveError') this.ui.toast('Couldn’t save to this browser: ' + m.text, 'warn');
        if (m.t === 'storage') this.storagePersisted = m.persisted;
        if (m.t === 'saved') this.lastSaved = m.at;
      };
      // Save when the tab is hidden or closed.
      addEventListener('visibilitychange', () => { if (document.visibilityState === 'hidden') this.net.flush(); });
      addEventListener('pagehide', () => this.net.flush());
    }
    this.ui = new UI(this);
    this.input.onUI = (d) => this.ui.nav(d);
    this.me = null;
    this.others = new Map();
    this.mobs = new Map();
    this.drops = new Map();
    this.yaw = 0; this.pitch = 0.25;
    this.time = 0;
    this.frameTimes = [];
    this.lastMoveSent = 0;
    this.lastSent = [0, 0, 0, 0];
    this.nextAttack = 0; this.nextGather = 0; this.nextUse = 0;
    this.swing = 0;
    this.walk = 0;
    this.chunkPending = new Set();
    this.dirty = new Set();
    this.tags = new Map();
    this.minimap = new MiniMap(this, $('#minimap'));
    this.worldMap = new WorldMap(this);
    this.shake = 0;
    this.netWindow = { t: performance.now(), inB: 0, outB: 0, inRate: 0, outRate: 0 };
    applyAccessibility();
    this.applyQuality();
    addEventListener('resize', () => this.r.resize(this.q.resScale));
  }

  applyQuality() {
    this.q = quality();
    applyAccessibility();
    this.r.resize(this.q.resScale);
    if (this.world) for (const key of [...this.r.chunks.keys()]) { const [cx, cz, lod] = key.split(',').map(Number); if (!lod) this.dirty.add(`${cx},${cz},0`); }
  }

  // Any item with the content flag "light" lights your way (not just lanterns).
  carriesLight(inv) {
    for (const id in inv) if (inv[id] > 0 && this.items[id] && this.items[id].light) return true;
    return false;
  }

  // The drawing for a creature, from its content "model" (falls back to a Bramblet).
  mobModel(m) {
    const def = this.mobDefs && this.mobDefs[m.kind];
    return MOB_MODELS[def && def.model] || MOB_MODELS.bramblet;
  }

  realmText() {
    const r = this.realm;
    if (!r) return '';
    if (r.kind === 'browser') return 'Browser world: it runs and saves entirely in this browser on this device. Clearing site data or using private browsing can erase it, so export a copy now and then. It never transfers to hosted or official worlds.';
    return r.kind === 'official'
      ? `Official world “${r.name}”. Characters and progress here belong to this official world only.`
      : `Private world “${r.name}” hosted by a player. Characters, items and levels here are saved by the host and stay in this world — they never transfer to official worlds${r.modded ? '. This world uses mods: ' + (r.mods || []).join(', ') : ''}.`;
  }

  // ---------- join screen ----------
  showJoin(err) { return showJoin(this, err); }

  // ---------- game start ----------
  start(w) {
    this.realm = w.realm;
    if (w.token) {
      try { localStorage.setItem('lf.char.' + w.realm.id, JSON.stringify({ token: w.token, name: this.net.hello.name })); } catch { }
      this.net.hello = { t: 'hello', proto: PROTO, client: CLIENT, token: w.token };
    }
    this.myId = w.you;
    const W = w.world;
    if (!this.world || this.world.sx !== W.sx || this.world.sz !== W.sz) {
      this.world = new World(W.sx, W.sy, W.sz, W.chunk);
      for (const k of [...this.r.chunks.keys()]) { const [cx, cz, lod] = k.split(',').map(Number); this.r.dropChunk(cx, cz, lod); }
      this.minimap.reset();
    }
    this.worldInfo = W;
    this.zones = W.zones || [];
    this.anvil = W.anvil;
    this.spawn = W.spawn;
    const c = w.content;
    this.items = Object.fromEntries(c.items.map((i) => [i.id, i]));
    this.recipes = c.recipes;
    this.quests = Object.fromEntries(c.quests.map((q) => [q.id, q]));
    this.npcs = Object.fromEntries(c.npcs.map((n) => [n.id, n]));
    this.mobDefs = Object.fromEntries(c.mobs.map((m) => [m.id, m]));
    this.nodes = Object.fromEntries(c.nodes.map((n) => [n.block, n]));
    this.skills = c.skills || [];
    this.charms = c.charms || [];
    this.courses = c.courses || [];
    this.body = new Body(w.pos[0], w.pos[1], w.pos[2]);
    this.yaw = w.pos[3] || 0;
    $('#screen-join').hidden = true;
    $('#hud').hidden = false;
    $('#realmbadge').textContent = { official: 'Official world', browser: 'Browser world · saved on this device' }[w.realm.kind] || `Private world · progress stays here${w.realm.modded ? ' · modded' : ''}`;
    $('#netstatus').hidden = true;
    this.input.setUIMode(false);
    this.input.onUI = (d) => this.ui.nav(d);
    for (const m of w.chat || []) this.ui.chat(m);
    this.bindNet();
    this.chunkPending.clear();
    if (!this.running) { this.running = true; this.last = performance.now(); requestAnimationFrame((t) => this.frame(t)); }
    if (params.get('bench')) this.startBench();
    window.lf = this; // handy for debugging and automated checks
  }

  bindNet() {
    const n = this.net, ui = this.ui;
    n.on('me', (m) => { const first = !this.me; this.me = m; ui.updateHUD(m); if (first) ui.toast(m.zoneName ? `Welcome to ${m.zoneName}!` : `Welcome to ${this.worldInfo.name}!`, 'good'); });
    n.on('snap', (m) => this.onSnap(m));
    n.on('binary', (b) => {
      if (b[0] !== 1) return;
      const cx = b[1] | (b[2] << 8), cz = b[3] | (b[4] << 8); // uint16 chunk coordinates
      this.world.applyChunk(cx, cz, b.subarray(5));
      this.minimap.invalidate(cx, cz);
      this.chunkPending.delete(`${cx},${cz}`);
      for (const [dx, dz] of [[0, 0], [1, 0], [-1, 0], [0, 1], [0, -1]]) {
        const nx = this.world.wrapC(cx + dx), nz = cz + dz;
        if (this.world.chunkLoaded(nx, nz)) { this.dirty.add(`${nx},${nz},0`); this.dirty.add(`${nx},${nz},1`); }
      }
    });
    n.on('block', (m) => {
      this.world.set(m.x, m.y, m.z, m.b);
      this.minimap.invalidate(Math.floor(this.world.wrapX(m.x) / this.world.cs), Math.floor(m.z / this.world.cs));
      const w = this.world, cs = w.cs, cx = Math.floor(w.wrapX(m.x) / cs), cz = Math.floor(m.z / cs);
      const lx = w.wrapX(m.x) - cx * cs, lz = m.z - cz * cs;
      this.urgent = this.urgent || new Set();
      this.urgent.add(`${cx},${cz},0`);
      if (lx === 0) this.urgent.add(`${w.wrapC(cx - 1)},${cz},0`);
      if (lx === cs - 1) this.urgent.add(`${w.wrapC(cx + 1)},${cz},0`);
      if (lz === 0) this.urgent.add(`${cx},${cz - 1},0`);
      if (lz === cs - 1) this.urgent.add(`${cx},${cz + 1},0`);
      this.dirty.add(`${cx},${cz},1`);
    });
    n.on('correct', (m) => {
      this.body.x = m.x; this.body.y = m.y; this.body.z = m.z; this.body.vy = 0;
      this.lastSent = [m.x, m.y, m.z, this.yaw];
    });
    n.on('hit', (m) => this.onHit(m));
    n.on('shot', (m) => this.onShot(m));
    n.on('die', (m) => { const e = this.mobs.get(m.id); if (e) e.dead = true; });
    n.on('ko', (m) => { ui.toast(m.text, 'danger'); });
    n.on('toast', (m) => ui.toast(m.text, m.kind));
    n.on('pickup', (m) => ui.toast(`+${m.count} ${ui.item(m.item).name}`, 'good'));
    n.on('chat', (m) => ui.chat(m));
    n.on('levelup', () => { }); // the server also sends a toast naming what it unlocks
    n.on('xp', (m) => ui.xpPopup(m));
    n.on('dialog', (m) => ui.showDialog(m));
    n.on('quest_done', (m) => ui.toast(m.text, 'quest'));
    n.on('trade_end', (m) => ui.toast(m.text, m.ok ? 'good' : 'warn'));
    n.on('chip', (m) => { this.chip = { key: `${m.x},${m.y},${m.z}`, text: `${m.hit}/${m.of}`, t: this.time }; });
    n.on('disconnect', (d) => this.onDisconnect(d));
  }

  onDisconnect(d) {
    const code = d.server && d.server.code;
    if (['session_replaced', 'banned', 'kicked', 'server_stopping'].includes(code)) {
      $('#netstatus').hidden = false;
      $('#netstatus').textContent = d.server.text;
      this.ui.toast(d.server.text, 'warn');
      if (code !== 'server_stopping') return;
    }
    $('#netstatus').hidden = false;
    $('#netstatus').textContent = 'Connection lost — reconnecting…';
    const delay = Math.min(10000, 500 * 2 ** this.net.retry++);
    setTimeout(async () => {
      try { const w = await this.net.connect(this.net.hello); this.start(w); this.ui.toast('Reconnected.', 'good'); }
      catch (e) { if (e.code && e.code !== 'connect_failed') { $('#netstatus').textContent = e.text; if (e.code === 'bad_token' || e.code === 'banned') return; } this.onDisconnect({}); }
    }, delay);
  }

  onSnap(m) {
    const seen = new Set();
    for (const p of m.p || []) {
      seen.add(p.id);
      let e = this.others.get(p.id);
      if (!e) { e = { id: p.id, x: p.x, y: p.y, z: p.z, yaw: p.r, walk: 0, flash: 0 }; this.others.set(p.id, e); }
      Object.assign(e, { tx: p.x, ty: p.y, tz: p.z, tyaw: p.r, name: p.n, color: p.c, hp: p.hp, mhp: p.mhp, weapon: p.w, lantern: p.l });
    }
    for (const id of this.others.keys()) if (!seen.has(id)) this.others.delete(id);
    const ms = new Set();
    for (const q of m.m || []) {
      ms.add(q.id);
      let e = this.mobs.get(q.id);
      if (!e || e.dead) { e = { id: q.id, x: q.x, y: q.y, z: q.z, yaw: q.r, flash: 0 }; this.mobs.set(q.id, e); }
      Object.assign(e, { tx: q.x, ty: q.y, tz: q.z, tyaw: q.r, kind: q.k, hp: q.hp, mhp: q.mhp, aggro: q.a });
    }
    for (const id of this.mobs.keys()) if (!ms.has(id)) this.mobs.delete(id);
    this.drops.clear();
    for (const d of m.d || []) this.drops.set(d.id, { id: d.id, item: d.i, count: d.n, x: d.x, y: d.y, z: d.z, own: d.own });
  }

  entityPos(id) {
    if (id === this.myId) return [this.body.x, this.body.y + 2, this.body.z];
    const e = this.others.get(id) || this.mobs.get(id);
    return e ? [e.x, e.y + 1.8, e.z] : null;
  }

  // A shot from a bow, sling or wand: draw it flying to the target. The
  // server has already decided the hit; this is only the picture of it.
  onShot(m) {
    const a = m.src === this.myId ? [this.body.x, this.body.y + 1.4, this.body.z] : this.entityPos(m.src);
    const t = this.entityPos(m.dst);
    if (!a || !t) return;
    if (m.src === this.myId) this.swing = 1;
    (this.shots ||= []).push({ a: [a[0], a[1] - (m.src === this.myId ? 0 : 0.4), a[2]], b: [t[0], t[1] - 0.9, t[2]], t0: this.time, kind: m.kind, color: m.color });
  }

  onHit(m) {
    const p = this.entityPos(m.dst);
    const e = this.others.get(m.dst) || this.mobs.get(m.dst);
    if (e) e.flash = 0.15;
    if (m.dst === this.myId) {
      this.myFlash = 0.15;
      if (!settings.reduceMotion) { this.shake = 0.18; const v = $('#vignette'); v.classList.add('hurt'); setTimeout(() => v.classList.remove('hurt'), 160); }
    }
    if (p && this.vp) {
      const s = project(this.vp, ...this.r.rel(p[0], p[1], p[2]), innerWidth, innerHeight);
      if (s) {
        const el = document.createElement('div');
        el.className = 'dmg' + (m.dst === this.myId ? ' me' : '');
        el.textContent = '-' + m.dmg;
        el.style.left = s[0] + 'px'; el.style.top = s[1] + 'px';
        $('#tags').appendChild(el);
        setTimeout(() => el.remove(), 900);
      }
    }
  }

  // ---------- per-frame ----------
  frame(now) {
    requestAnimationFrame((t) => this.frame(t));
    const dt = Math.min(0.1, (now - this.last) / 1000);
    this.last = now;
    this.time += dt;
    const t0 = performance.now();
    if (this.world && this.me) this.update(dt, now);
    this.streamChunks();
    this.meshBudget(this.q.name === 'low' ? 3 : 5);
    if (this.world && this.body && this.me) this.draw(dt);
    this.recordPerf(performance.now() - t0, dt * 1000);
  }

  update(dt, now) {
    const inp = this.input.poll(dt);
    const ui = this.ui;
    if (this.bench) { this.benchStep(dt); return; }
    // Camera.
    this.yaw -= inp.look[0];
    this.pitch = Math.max(-1.2, Math.min(1.35, this.pitch + inp.look[1]));
    if (!ui.open) {
      if (inp.pressed.has('menu')) ui.openMenu();
      else if (inp.pressed.has('pause')) ui.openMenu('settings');
      else if (inp.pressed.has('map')) ui.openMenu('map');
      if (inp.pressed.has('chat') && this.input.device !== 'touch') { ui.openChat(); }
      if (inp.pressed.has('hotPrev')) ui.cycleHot(-1);
      if (inp.pressed.has('hotNext')) ui.cycleHot(1);
      for (let i = 1; i <= 8; i++) if (inp.pressed.has('slot' + i)) { ui.hot = i - 1; ui.renderHotbar(this.me); }
      if (inp.pressed.has('drop')) ui.dropHot();
      if (inp.pressed.has('hearth')) this.net.request({ t: 'hearth' }).then((r) => { if (!r.ok) ui.toast(r.err, 'warn'); });
    }
    // Movement relative to camera yaw. Forward = (sin yaw, cos yaw).
    const [mx, mz] = ui.open ? [0, 0] : inp.move;
    // Agility adds 1% sprint speed per level (the server allows the same).
    const speed = inp.sprint ? SPRINT * (1 + 0.01 * ((this.me.skills.agility || { lvl: 1 }).lvl)) : WALK;
    const fx = Math.sin(this.yaw), fz = Math.cos(this.yaw);
    const rx = -fz, rz = fx; // right vector
    const vx = (fx * mz + rx * mx) * speed, vz = (fz * mz + rz * mx) * speed;
    step(this.world, this.body, vx, vz, !ui.open && inp.jump, dt);
    const moving = Math.hypot(vx, vz) > 0.1;
    if (moving) { this.walk += dt * (inp.sprint ? 13 : 10); this.bodyYaw = Math.atan2(vx, vz); }
    else if (this.body.inWater) this.walk += dt * 4; // treading water
    this.moving = moving;
    if (this.bodyYaw === undefined) this.bodyYaw = this.yaw;
    // Send position at 20 Hz when changed.
    if (now - this.lastMoveSent > 50) {
      const b = this.body, l = this.lastSent;
      if (Math.abs(b.x - l[0]) + Math.abs(b.y - l[1]) + Math.abs(b.z - l[2]) > 0.01 || Math.abs(this.bodyYaw - l[3]) > 0.05) {
        this.net.send({ t: 'move', x: +b.x.toFixed(3), y: +b.y.toFixed(3), z: +b.z.toFixed(3), yaw: +this.bodyYaw.toFixed(3) });
        this.lastSent = [b.x, b.y, b.z, this.bodyYaw];
        this.lastMoveSent = now;
      }
    }
    this.findTargets();
    if (ui.open) return;
    // Actions.
    const tnow = performance.now();
    if (inp.primary || inp.pressed.has('primary')) {
      if (this.targetMob && tnow > this.nextAttack) {
        const wpn = this.items[this.me.eq];
        this.nextAttack = tnow + (wpn && wpn.cooldownMs ? wpn.cooldownMs : 700) + 30;
        this.swing = 1;
        this.net.request({ t: 'attack', target: this.targetMob.id }).then((r) => { if (!r.ok && r.err !== 'Too soon.') ui.toast(r.err, 'warn'); });
      } else if (!this.targetMob && this.targetBlock && tnow > this.nextGather) {
        const tb = this.targetBlock;
        this.nextGather = tnow + 460;
        this.swing = 1;
        const placeable = tb.block === B.PLANK || tb.block === B.COBBLE;
        const msg = this.nodes[tb.block] || COLLECT.has(tb.block) ? { t: 'gather', bx: tb.x, by: tb.y, bz: tb.z } : placeable ? { t: 'break', bx: tb.x, by: tb.y, bz: tb.z } : null;
        if (msg) this.net.request(msg).then((r) => { if (!r.ok && !/Steady/.test(r.err)) ui.toast(r.err, 'warn'); });
      } else if (!this.targetMob && !this.targetBlock && inp.pressed.has('primary') && this.targetPlayer && this.realm.pvp) {
        this.net.request({ t: 'attack', target: this.targetPlayer.id }).then((r) => { if (!r.ok) ui.toast(r.err, 'warn'); });
      }
    }
    if ((inp.use || inp.pressed.has('use')) && tnow > this.nextUse) {
      const sel = ui.hotSel, it = sel && this.items[sel];
      if (sel && sel.startsWith('charm:') && inp.pressed.has('use')) { this.nextUse = tnow + 400; this.swing = 1; this.net.request({ t: 'cast', charm: sel.slice(6) }).then((r) => { if (!r.ok) ui.toast(r.err, 'warn'); }); }
      else       if (it && it.kind === 'food' && inp.pressed.has('use')) { this.nextUse = tnow + 400; this.net.request({ t: 'eat', item: sel }).then((r) => { if (!r.ok) ui.toast(r.err, 'warn'); }); }
      else if (it && (it.trap || it.crop) && this.targetBlock && inp.pressed.has('use')) {
        const tb = this.targetBlock, px = tb.x + tb.face[0], py = tb.y + tb.face[1], pz = tb.z + tb.face[2];
        this.nextUse = tnow + 450;
        this.swing = 1;
        this.net.request({ t: it.trap ? 'set_trap' : 'plant', bx: px, by: py, bz: pz, item: sel }).then((r) => { if (!r.ok) ui.toast(r.err, 'warn'); });
      }       else if (it && it.burn && this.targetBlock && inp.pressed.has('use')) {
        const tb = this.targetBlock, px = tb.x + tb.face[0], py = tb.y + tb.face[1], pz = tb.z + tb.face[2];
        this.nextUse = tnow + 600;
        this.swing = 1;
        this.net.request({ t: 'light_fire', bx: px, by: py, bz: pz, item: sel }).then((r) => { if (!r.ok) ui.toast(r.err, 'warn'); });
      }       else if (it && it.kind === 'block' && this.targetBlock) {
        const tb = this.targetBlock, px = tb.x + tb.face[0], py = tb.y + tb.face[1], pz = tb.z + tb.face[2];
        this.nextUse = tnow + 250;
        this.swing = 1;
        this.net.request({ t: 'place', bx: px, by: py, bz: pz, item: sel }).then((r) => { if (!r.ok) ui.toast(r.err, 'warn'); });
      } else if (inp.pressed.has('use') && !it) ui.toast('Nothing selected. Craft planks or pick berries to fill your hotbar.', 'info');
    }
    if (inp.pressed.has('interact')) {
      if (this.nearNPC) this.ui.talk(this.nearNPC.id);
      else if (this.targetPlayer) this.net.request({ t: 'trade_request', target: this.targetPlayer.id }).then((r) => { if (r.ok) this.ui.openTrade(); else ui.toast(r.err, 'warn'); });
    }
  }

  eye() { return [this.body.x, this.body.y + 1.55, this.body.z]; }

  nearAnvil() { return this.nearStation('anvil'); }

  // nearStation mirrors the server's check (game.nearStation): a station block
  // within 4.5 blocks across, a few above or below.
  nearStation(st) {
    const blocks = STATIONS[st];
    if (!blocks || !this.body) return false;
    const cx = Math.floor(this.body.x), cy = Math.floor(this.body.y), cz = Math.floor(this.body.z);
    for (let dy = -2; dy <= 3; dy++) for (let dz = -5; dz <= 5; dz++) for (let dx = -5; dx <= 5; dx++) {
      if (blocks.includes(this.world.get(cx + dx, cy + dy, cz + dz)) && Math.hypot(cx + dx + 0.5 - this.body.x, cz + dz + 0.5 - this.body.z) <= 4.5) return true;
    }
    return false;
  }

  // The nearest danger zone ([x, z, radius]) to the camera, for darkness and fog.
  nearestDanger(x, z) {
    let best = null, bd = Infinity;
    for (const zn of this.zones) {
      if (zn.kind !== 'danger') continue;
      const d = this.world.dist(x, z, zn.x, zn.z) - zn.r;
      if (d < bd) { bd = d; best = [zn.x, zn.z, zn.r]; }
    }
    return best;
  }

  findTargets() {
    const b = this.body, [ex, ey, ez] = this.eye();
    const fx = Math.sin(this.yaw), fz = Math.cos(this.yaw);
    // Soft-lock the nearest creature in front (controller & touch friendly).
    // Bows, slings and wands reach further but need you to face the target.
    const wpn = this.me && this.items[this.me.eq];
    const reach = wpn && wpn.reach && this.me.inv[this.me.eq] ? wpn.reach - 0.3 : 3.2;
    let best = null, bs = Infinity;
    for (const m of this.mobs.values()) {
      if (m.dead || m.hp <= 0) continue;
      const dx = this.world.dx(b.x, m.x), dz = m.z - b.z, d = Math.hypot(dx, dz);
      if (d > reach || Math.abs(m.y - b.y) > (d > 3.2 ? 6 : 2.4)) continue;
      const facing = d < 0.8 ? 1 : (dx * fx + dz * fz) / d;
      if (facing < (d > 3.2 ? 0.9 : 0.2)) continue;
      const s = d - facing * 1.5;
      if (s < bs) { bs = s; best = m; }
    }
    this.targetMob = best;
    let bp = null; bs = Infinity;
    for (const p of this.others.values()) {
      const dx = this.world.dx(b.x, p.x), dz = p.z - b.z, d = Math.hypot(dx, dz);
      if (d > 4 || d < 0.01) continue;
      const facing = (dx * fx + dz * fz) / d;
      if (facing < 0.4) continue;
      if (d - facing < bs) { bs = d - facing; bp = p; }
    }
    this.targetPlayer = bp;
    // Block under the crosshair (from the camera), within reach of the character.
    const cam = this.camEye || [ex, ey, ez];
    const dir = this.camDir || [fx, 0, fz];
    const hit = raycast(this.world, cam[0], cam[1], cam[2], dir[0], dir[1], dir[2], 14);
    this.targetBlock = null;
    if (hit) {
      const d = Math.hypot(hit.x + 0.5 - ex, hit.y + 0.5 - ey, hit.z + 0.5 - ez);
      if (d <= 5.0) this.targetBlock = { ...hit, x: this.world.wrapX(hit.x) }; // canonical x for the server
    }
    this.nearNPC = null;
    for (const n of Object.values(this.npcs)) {
      if (this.world.dist(b.x, b.z, n.x, n.z) < 4.2 && Math.abs(n.y - b.y) < 3) this.nearNPC = n;
    }
    // HUD target + prompt.
    const tEl = $('#target'), pEl = $('#prompt');
    const dev = this.input.device;
    if (best) {
      const def = this.mobDefs[best.kind];
      tEl.innerHTML = `<b>${esc(def ? def.name : best.kind)}</b><div class="thp"><div style="width:${best.hp / best.mhp * 100}%"></div></div>`;
    } else if (this.targetBlock) {
      const tb = this.targetBlock, node = this.nodes[tb.block];
      let txt = BLOCKS[tb.block] ? BLOCKS[tb.block].name : '';
      if (node) {
        const lvl = this.me.skills[node.skill].lvl;
        txt = `${esc(node.name)} · ${node.skill} ${node.level}${lvl < node.level ? ' <span style="color:var(--danger)">(you: ' + lvl + ')</span>' : ''}`;
        if (this.chip && this.chip.key === `${tb.x},${tb.y},${tb.z}` && this.time - this.chip.t < 2) txt += ` · ${this.chip.text}`;
      }
      tEl.innerHTML = txt;
    } else tEl.innerHTML = '';
    let prompt = '';
    if (this.nearNPC) prompt = `${glyph(dev, 'interact')} Talk to ${this.nearNPC.name}`;
    else if (bp) prompt = `${glyph(dev, 'interact')} Trade with ${bp.name}`;
    else if (this.me.zone === 'danger' && this.me.hp < this.me.mhp * 0.4) prompt = `${glyph(dev, 'hearth')} Hearth Lantern to return home`;
    if (pEl.textContent !== prompt) pEl.textContent = prompt;
  }

  // ---------- chunks ----------
  streamChunks() {
    if (!this.world || !this.body || this.net.state !== 'open') return;
    const w = this.world, cs = w.cs;
    const pcx = this.body.x / cs, pcz = this.body.z / cs;
    const R = this.q.renderDist + 1;
    // Forget chunks well outside the view (the server can always resend them).
    if ((this.evictTick = (this.evictTick || 0) + 1) % 30 === 0) {
      for (const [cx, cz] of w.evict(pcx, pcz, R + 3)) { this.r.dropChunk(cx, cz, 0); this.r.dropChunk(cx, cz, 1); this.minimap.drop(cx, cz); }
    }
    if (this.chunkPending.size >= 8) return;
    const need = [];
    const c0 = Math.floor(pcx), r = Math.ceil(R);
    for (let cz = Math.max(0, Math.floor(pcz) - r); cz <= Math.min(w.cz - 1, Math.floor(pcz) + r); cz++) {
      for (let i = -r; i <= r; i++) {
        const cx = w.wrapC(c0 + i);
        if (w.chunkLoaded(cx, cz) || this.chunkPending.has(`${cx},${cz}`)) continue;
        const d = w.chunkDist(cx, cz, pcx, pcz);
        if (d <= R) need.push([d, cx, cz]);
      }
    }
    if (!need.length) return;
    need.sort((a, b) => a[0] - b[0]);
    const batch = need.slice(0, 8 - this.chunkPending.size).map(([, cx, cz]) => { this.chunkPending.add(`${cx},${cz}`); return [cx, cz]; });
    this.net.send({ t: 'chunks', chunks: batch });
  }

  meshBudget(ms) {
    if (!this.world || !this.body) return;
    const t0 = performance.now();
    const w = this.world, cs = w.cs;
    const pcx = this.body.x / cs, pcz = this.body.z / cs;
    const dist = (cx, cz) => w.chunkDist(cx, cz, pcx, pcz);
    const want = (cx, cz, lod) => {
      if (!w.chunkLoaded(cx, cz)) return false;
      const d = dist(cx, cz);
      if (d > this.q.renderDist + 1.5) return false;
      return lod ? d > this.q.lodDist - 0.8 : d <= this.q.lodDist + 1.2;
    };
    // Block edits first so building feels instant.
    if (this.urgent) {
      for (const k of this.urgent) { const [cx, cz, lod] = k.split(',').map(Number); if (w.chunkLoaded(cx, cz)) this.r.setChunk(cx, cz, lod, meshChunk(w, cx, cz, { ao: this.q.ao })); }
      this.urgent.clear();
    }
    // Mesh loaded chunks that are now wanted; drop meshes that aren't.
    for (const k of w.chunks.keys()) {
      const cx = k % w.cx, cz = Math.floor(k / w.cx);
      for (const lod of [0, 1]) if (want(cx, cz, lod) && !this.r.chunks.has(`${cx},${cz},${lod}`)) this.dirty.add(`${cx},${cz},${lod}`);
    }
    for (const ch of [...this.r.chunks.values()]) {
      const d = dist(ch.cx, ch.cz);
      if (d > this.q.renderDist + 2.5 || (ch.lod === 0 && d > this.q.lodDist + 2.5) || (ch.lod === 1 && d < this.q.lodDist - 2)) this.r.dropChunk(ch.cx, ch.cz, ch.lod);
    }
    const list = [...this.dirty].map((k) => { const [cx, cz, lod] = k.split(',').map(Number); return [dist(cx, cz), cx, cz, lod, k]; }).sort((a, b) => a[0] - b[0]);
    for (const [, cx, cz, lod, k] of list) {
      if (performance.now() - t0 > ms) break;
      this.dirty.delete(k);
      if (!want(cx, cz, lod)) continue;
      const mesh = lod ? meshChunkLOD(w, cx, cz) : meshChunk(w, cx, cz, { ao: this.q.ao });
      this.r.setChunk(cx, cz, lod, mesh);
    }
  }

  // ---------- drawing ----------
  draw(dt) {
    const r = this.r, b = this.body, q = this.q;
    // Smooth remote entities toward their latest snapshot.
    const k = 1 - Math.exp(-dt * 12);
    for (const e of [...this.others.values(), ...this.mobs.values()]) {
      if (e.tx === undefined) continue;
      const dx = this.world.dx(e.x, e.tx), dz = e.tz - e.z; // the short way across the date line
      e.moving = Math.hypot(dx, dz) > 0.02;
      e.swim = isWater(this.world.get(Math.floor(e.x), Math.floor(e.y + 0.4), Math.floor(e.z)));
      if (e.moving || e.swim) e.walk = (e.walk || 0) + dt * (e.moving ? 10 : 4);
      e.x = this.world.wrapX(e.x + dx * k); e.y += (e.ty - e.y) * k; e.z += dz * k;
      let dy = e.tyaw - e.yaw; dy = Math.atan2(Math.sin(dy), Math.cos(dy)); e.yaw += dy * k;
      e.flash = Math.max(0, (e.flash || 0) - dt);
    }
    // Camera: over-the-shoulder, collides with terrain.
    const cp = Math.cos(this.pitch), sp = Math.sin(this.pitch);
    const fx = Math.sin(this.yaw) * cp, fy = -sp, fz = Math.cos(this.yaw) * cp;
    const head = [b.x, b.y + 1.6, b.z];
    const boomDir = [-fx, -fy, -fz];
    const boom = cameraBoom(this.world, head[0], head[1], head[2], boomDir[0], boomDir[1], boomDir[2], 4.2);
    const side = 0.45 * Math.min(1, boom / 2);
    const rx = -Math.cos(this.yaw), rz = Math.sin(this.yaw);
    let eye = [head[0] + boomDir[0] * boom + rx * side, head[1] + boomDir[1] * boom + 0.2, head[2] + boomDir[2] * boom + rz * side];
    if (this.shake > 0) { this.shake -= dt; const s = this.shake * 0.25; eye = eye.map((v) => v + (Math.random() - 0.5) * s); }
    this.camEye = eye;
    this.camDir = [fx, fy, fz];
    const target = [eye[0] + fx, eye[1] + fy, eye[2] + fz];
    if (this.bench) { eye = this.bench.eye; this.camEye = eye; }
    const view = this.bench ? { eye, target: this.bench.target, fov: 1.2, far: (q.renderDist + 1) * 16 + 24, pitch: 0 } : { eye, target, fov: 1.2, far: (q.renderDist + 1) * 16 + 24, pitch: this.pitch };
    // Entities.
    r.beginBoxes();
    this.swing = Math.max(0, this.swing - dt * 3.5);
    this.myFlash = Math.max(0, (this.myFlash || 0) - dt);
    const wpn = this.me && this.items[this.me.eq];
    const hasLantern = this.me && this.carriesLight(this.me.inv);
    if (boom > 0.9 && !this.bench) drawPerson(r, { x: b.x, y: b.y, z: b.z, yaw: this.bodyYaw || 0, color: this.me.color || '#4a90d9', walk: this.walk, moving: this.moving, swing: this.swing, held: wpn || null, lantern: hasLantern, flash: this.myFlash, swim: b.inWater });
    for (const p of this.others.values()) drawPerson(r, { ...p, held: p.weapon ? this.items[p.weapon] : null });
    for (const d of this.drops.values()) drawDropped(r, this.items[d.item] || { color: "#888888" }, d.x, d.y, d.z, this.time, d.id);
    for (const n of Object.values(this.npcs)) drawPerson(r, { x: n.x, y: n.y, z: n.z, yaw: n.yaw + Math.sin(this.time * 0.5 + n.x) * 0.2, color: n.color, hat: n.hat, moving: false });
    for (const m of this.mobs.values()) {
      if (m.dead) continue;
      this.mobModel(m).draw(r, m, this.time);
    }
    this.drawCourses(r);
    this.shots = (this.shots || []).filter((s) => drawShot(r, s, this.time, this.world));
    const inGloam = this.me.zone === 'danger';
    const playerLight = hasLantern ? [b.x, b.y + 1.2, b.z, inGloam ? 11 : 6] : (inGloam ? [b.x, b.y + 1.2, b.z, 3.5] : [0, 0, 0, 0]);
    const hl = this.targetBlock && !this.targetMob ? [this.targetBlock.x, this.targetBlock.y, this.targetBlock.z] : null;
    this.vp = r.render(this.world, view, {
      renderDist: q.renderDist, lodDist: q.lodDist, maxLights: q.maxLights, sway: q.sway && !settings.reduceMotion,
      time: this.time, fogNear: q.renderDist * 16 * 0.55, fogFar: q.renderDist * 16 + 8, danger: this.nearestDanger(eye[0], eye[2]),
      playerLight, highlight: hl,
    });
    this.drawTags();
    if (settings.showMinimap !== false && (this.mmTick = (this.mmTick || 0) + 1) % 3 === 0) this.minimap.draw();
  }

  // Agility course flags: the start is gold, the next checkpoint of the
  // course you're running glows green, the rest are white.
  drawCourses(r) {
    const run = this.me && this.me.run, b = this.body;
    for (const co of this.courses) {
      (co.points || []).forEach((pt, i) => {
        if (this.world.dist(b.x, b.z, pt[0], pt[1]) > 60) return;
        const y = this.flagY(pt);
        if (y == null) return;
        const next = run && run.id === co.id && run.next === i;
        const col = next ? [0.35, 1, 0.45] : i === 0 ? [1, 0.8, 0.25] : [0.95, 0.95, 0.95];
        drawFlag(r, pt[0], y, pt[1], col, next || (!run && i === 0), this.time);
      });
    }
  }

  flagY(pt) {
    const k = `${Math.floor(pt[0])},${Math.floor(pt[1])}`;
    this.flagYs ||= new Map();
    let y = this.flagYs.get(k);
    if (y == null) { y = this.world.surfaceY(Math.floor(pt[0]), Math.floor(pt[1])); if (y > 0) this.flagYs.set(k, y); }
    return y > 0 ? y : null;
  }

  drawTags() {
    const W = innerWidth, H = innerHeight, vp = this.vp, b = this.body;
    const want = new Map();
    const add = (key, x, y, z, html, cls) => {
      if (this.world.dist(b.x, b.z, x, z) > 22) return;
      const s = project(vp, ...this.r.rel(x, y, z), W, H);
      if (!s || s[0] < -50 || s[0] > W + 50 || s[1] < -50 || s[1] > H + 50) return;
      want.set(key, { s, html, cls });
    };
    for (const p of this.others.values()) add('p' + p.id, p.x, p.y + 2.15, p.z, `${esc(p.name)}<div class="bar"><div style="width:${p.hp / p.mhp * 100}%"></div></div>`, 'tag');
    for (const m of this.mobs.values()) if (!m.dead && (m.hp < m.mhp || m === this.targetMob || m.aggro)) add('m' + m.id, m.x, m.y + this.mobModel(m).tag, m.z, `<div class="bar"><div style="width:${m.hp / m.mhp * 100}%"></div></div>`, 'tag');
    // Agility: name the start flags, and count down to the next checkpoint.
    const run = this.me && this.me.run;
    if (run && this.runSeen !== this.me) { this.runSeen = this.me; this.runEnd = this.time + run.left; }
    for (const co of this.courses) {
      const pts = co.points || [];
      if (!pts.length) continue;
      if (run && run.id === co.id) {
        const pt = pts[run.next], y = pt && this.flagY(pt);
        if (y != null) add('run', pt[0], y + 2.9, pt[1], `<b>Flag ${run.next}/${pts.length - 1}</b> · ${Math.max(0, this.runEnd - this.time).toFixed(1)}s`, 'tag npc');
      } else if (!run) {
        const y = this.flagY(pts[0]);
        if (y != null) add('co' + co.id, pts[0][0], y + 2.9, pts[0][1], `🏁 ${esc(co.name)} · Agility ${co.level}`, 'tag npc');
      }
    }
    for (const n of Object.values(this.npcs)) {
      let mark = '';
      for (const q of Object.values(this.quests)) if (q.giver === n.id) {
        const st = this.me && this.me.quests[q.id];
        if (!st && (!q.requires || (this.me.quests[q.requires] && this.me.quests[q.requires].status === 'done'))) mark = '<span class="q">!</span><br>';
        else if (st && st.status === 'active') mark = mark || '<span class="q" style="color:var(--quest)">?</span><br>';
      }
      add('n' + n.id, n.x, n.y + 2.35, n.z, `${mark}${esc(n.name)}`, 'tag npc');
    }
    for (const d of this.drops.values()) if (this.world.dist(b.x, b.z, d.x, d.z) < 7) add('d' + d.id, d.x, d.y + 0.95, d.z, `${esc(this.ui.item(d.item).name)}${d.count > 1 ? ' ×' + d.count : ''}`, 'tag drop');
    for (const [k, el] of this.tags) if (!want.has(k)) { el.remove(); this.tags.delete(k); }
    for (const [k, v] of want) {
      let el = this.tags.get(k);
      if (!el) { el = document.createElement('div'); $('#tags').appendChild(el); this.tags.set(k, el); }
      if (el._h !== v.html) { el.innerHTML = v.html; el._h = v.html; }
      el.className = v.cls;
      el.style.left = v.s[0] + 'px'; el.style.top = v.s[1] + 'px';
    }
  }

  // ---------- performance ----------
  recordPerf(cpuMs, frameMs) {
    const ft = this.frameTimes;
    ft.push(frameMs);
    if (ft.length > 240) ft.shift();
    (this.cpuTimes = this.cpuTimes || []).push(cpuMs);
    if (this.cpuTimes.length > 240) this.cpuTimes.shift();
    const nw = this.netWindow, now = performance.now();
    if (now - nw.t > 1000) {
      nw.inRate = (this.net.bytesIn - nw.inB) / ((now - nw.t) / 1000);
      nw.outRate = (this.net.bytesOut - nw.outB) / ((now - nw.t) / 1000);
      nw.inB = this.net.bytesIn; nw.outB = this.net.bytesOut; nw.t = now;
      if (this.bench) this.bench.net.push([nw.inRate, nw.outRate]);
    }
    if (!settings.showPerf || (this.perfTick = (this.perfTick || 0) + 1) % 15) return;
    const s = this.perfStats();
    $('#perf').textContent = `${s.fps.toFixed(0)} fps · frame ${s.avg.toFixed(1)} ms (p95 ${s.p95.toFixed(1)}) · cpu ${s.cpu.toFixed(1)} ms
draws ${s.draws} · tris ${(s.tris / 1000).toFixed(0)}k · chunks ${s.chunks}/${s.chunksTotal} · ents ${s.inst}
gpu ~${s.gpuMB.toFixed(1)} MB · js heap ${s.heapMB ? s.heapMB.toFixed(0) + ' MB' : 'n/a'} · net ↓${(s.netIn / 1024).toFixed(1)} ↑${(s.netOut / 1024).toFixed(1)} KB/s
quality ${this.q.name} · ${this.canvas.width}×${this.canvas.height} · rd ${this.q.renderDist} · lod>${this.q.lodDist}`;
  }

  perfStats() {
    const ft = [...this.frameTimes].sort((a, b) => a - b);
    const avg = ft.reduce((a, b) => a + b, 0) / Math.max(1, ft.length);
    const cpu = (this.cpuTimes || []).reduce((a, b) => a + b, 0) / Math.max(1, (this.cpuTimes || []).length);
    const st = this.r.stats;
    return {
      fps: 1000 / avg, avg, p95: ft[Math.floor(ft.length * 0.95)] || 0, max: ft[ft.length - 1] || 0, cpu,
      draws: st.draws, tris: st.tris, chunks: st.chunks, chunksTotal: st.chunksTotal, inst: this.r.nInst, gpuMB: st.gpuBytes / 1048576,
      heapMB: performance.memory ? performance.memory.usedJSHeapSize / 1048576 : 0,
      netIn: this.netWindow.inRate, netOut: this.netWindow.outRate, quality: this.q.name, res: [this.canvas.width, this.canvas.height],
    };
  }

  // ?bench=1 : scripted camera flight for reproducible measurements.
  startBench() {
    const wait = () => {
      if (this.chunkPending.size || this.dirty.size) { setTimeout(wait, 200); return; }
      const [sx, , sz] = this.spawn;
      this.bench = { t: 0, frames: [], net: [], cx: sx, cz: sz, eye: [sx, 30, sz], target: [sx, 12, sz] };
      this.frameTimes.length = 0;
      console.log('[bench] start', this.q.name);
    };
    setTimeout(wait, 500);
  }

  benchStep(dt) {
    const bn = this.bench;
    bn.t += dt;
    const a = bn.t * 0.31;
    bn.eye = [bn.cx + Math.cos(a) * 30, 24 + Math.sin(a * 0.7) * 4, bn.cz + Math.sin(a) * 30];
    bn.target = [bn.cx + Math.cos(a + 1.2) * 10, 13, bn.cz + Math.sin(a + 1.2) * 10];
    if (bn.t > 1) bn.frames.push(this.frameTimes[this.frameTimes.length - 1]);
    if (bn.t > 21 && !bn.done) {
      bn.done = true;
      const f = [...bn.frames].sort((x, y) => x - y);
      const avg = f.reduce((x, y) => x + y, 0) / f.length;
      const s = this.perfStats();
      window.__bench = { quality: this.q.name, frames: f.length, avgMs: +avg.toFixed(2), p50: +f[Math.floor(f.length / 2)].toFixed(2), p95: +f[Math.floor(f.length * 0.95)].toFixed(2), p99: +f[Math.floor(f.length * 0.99)].toFixed(2), max: +f[f.length - 1].toFixed(2), cpuMs: +s.cpu.toFixed(2), draws: s.draws, tris: s.tris, chunks: s.chunks, chunksTotal: s.chunksTotal, gpuMB: +s.gpuMB.toFixed(1), heapMB: +s.heapMB.toFixed(1), res: s.res };
      console.log('[bench] result', JSON.stringify(window.__bench));
      this.bench = null;
    }
  }
}

const game = new Game();
game.showJoin();
