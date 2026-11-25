import { Renderer } from './renderer.js';
import { World } from './world.js';
import { meshChunk, meshChunkLOD } from './mesher.js';
import { Body, step, raycast, cameraBoom, WALK, SPRINT } from './physics.js';
import { Input, glyph } from './input.js';
import { Net, PROTO, CLIENT } from './net.js';
import { UI, esc, applyAccessibility } from './ui.js';
import { settings, quality, save } from './settings.js';
import { drawPerson, drawBramblet, drawWisp } from './models.js';
import { project } from './mat.js';
import { B, BLOCKS } from './blocks.js';

const $ = (s) => document.querySelector(s);
const params = new URLSearchParams(location.search);
const NPC_HATS = { mabel: 'tall', bertram: 'cap', ott: 'helm' };
const COLORS = ['#4a90d9', '#e26d5a', '#5cb85c', '#f0ad4e', '#9b59b6', '#1abc9c', '#e84393', '#f5f0e1'];

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
    this.net = new Net();
    this.ui = new UI(this);
    this.input.onUI = (d) => this.ui.nav(d);
    this.me = null;
    this.others = new Map();
    this.mobs = new Map();
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

  realmText() {
    const r = this.realm;
    if (!r) return '';
    return r.kind === 'official'
      ? `Official world “${r.name}”. Characters and progress here belong to this official world only.`
      : `Private world “${r.name}” hosted by a player. Characters, items and levels here are saved by the host and stay in this world — they never transfer to official worlds${r.modded ? '. This world uses mods: ' + (r.mods || []).join(', ') : ''}.`;
  }

  // ---------- join screen ----------
  async showJoin(err) {
    $('#screen-join').hidden = false;
    const wi = $('#worldinfo');
    let info;
    try {
      info = await (await fetch('/api/info', { cache: 'no-store' })).json();
    } catch {
      wi.innerHTML = `<b>Can't reach the world server.</b><p class="note">If you're joining a friend, make sure <code>lanternfall join</code> is still running.</p>`;
      $('#joinform').innerHTML = `<button class="btn primary" id="retry">Try again</button>`;
      $('#retry').onclick = () => this.showJoin();
      return;
    }
    this.info = info;
    if (/^https?:\/\//.test(info.source || '')) {
      $('#sourcelink').innerHTML = `Free software under ${esc(info.license || 'AGPL-3.0')} · <a href="${esc(info.source)}" target="_blank" rel="noopener">Source code</a>`;
    }
    const kind = info.kind === 'official' ? '<span class="badge official">Official world</span>' : '<span class="badge private">Private world</span>';
    wi.innerHTML = `<h3>${esc(info.name)}</h3>${kind}${info.modded ? '<span class="badge modded">Modded</span>' : ''}
      <span class="note">${info.players}/${info.max} players · PvP ${info.pvp ? 'on' : 'off'} · trading ${info.trading ? 'on' : 'off'}</span>
      ${info.desc ? `<p>${esc(info.desc)}</p>` : ''}
      <p class="sep">${info.kind === 'official'
        ? 'Your character here is an official-world character.'
        : 'Characters you make here are <b>saved by the host</b> and <b>stay in this world</b>. They are separate from official-world characters, and nothing from this world can be moved into an official one.'}</p>`;
    // Companion diagnostics (only present when joining through `lanternfall join`).
    let companion = null;
    try {
      const r = await fetch('/companion/status', { cache: 'no-store' });
      if (r.ok && (r.headers.get('content-type') || '').includes('json')) companion = await r.json();
    } catch { /* direct connection */ }
    const dg = $('#diag');
    if (companion) {
      dg.hidden = false;
      dg.className = 'diag' + (companion.stage === 'failed' ? ' bad' : '');
      const path = companion.path === 'direct' ? '✓ Direct peer-to-peer connection' : companion.path === 'relay' ? '↔ Connected through a relay (a little extra lag)' : 'Checking connection…';
      dg.innerHTML = `<b>Tunnel to your friend's world:</b> ${path}${companion.latencyMs ? ` · ${Math.round(companion.latencyMs)} ms` : ''}${companion.problem ? `<br>${esc(companion.problem)}` : ''}`;
      try {
        if (!sessionStorage.getItem('lf.invite')) {
          const r = await fetch('/companion/invite', { method: 'POST', headers: { 'X-Lanternfall': '1' } });
          const j = await r.json();
          if (j.code) sessionStorage.setItem('lf.invite', j.code);
        }
      } catch { /* ignore */ }
    }
    const saved = this.savedChar(info.realm);
    const invite = sessionStorage.getItem('lf.invite') || '';
    const form = $('#joinform');
    const showNew = !saved || this.forceNew;
    form.innerHTML = `<div class="form">
      ${saved && !this.forceNew ? `<button class="btn primary" id="continue">Continue as ${esc(saved.name)}</button>
        <button class="btn" id="newchar">Make a new character</button>` : ''}
      ${showNew ? `<label>Character name<input id="name" maxlength="16" autocomplete="off" placeholder="3–16 letters or digits" value=""></label>
        <label>Color<div class="colors">${COLORS.map((c, i) => `<button class="swatch" style="background:${c}" data-c="${c}" aria-pressed="${i === 0}" aria-label="Color ${i + 1}"></button>`).join('')}</div></label>
        ${info.access === 'invite' ? `<label>Invite code <input id="invite" maxlength="24" autocomplete="off" placeholder="e.g. MOSS-ABCD234" value="${esc(invite)}"></label>
          <span class="note">This world is invite-only. The host can create codes on their admin screen. (Not needed if you're the host on this computer.)</span>` : ''}
        <button class="btn primary" id="join">Enter ${esc(info.name)}</button>` : ''}
      <div class="err" id="joinerr" role="alert">${esc(err || '')}</div></div>`;
    let color = COLORS[0];
    for (const s of form.querySelectorAll('.swatch')) s.onclick = () => { color = s.dataset.c; for (const o of form.querySelectorAll('.swatch')) o.setAttribute('aria-pressed', o === s); };
    const go = async (hello) => {
      $('#joinerr').textContent = 'Connecting…';
      try {
        const w = await this.net.connect(hello);
        this.start(w);
      } catch (e) {
        $('#joinerr').textContent = e.text || 'Could not connect.';
        if (e.code === 'bad_token') { this.forgetChar(info.realm); this.forceNew = true; setTimeout(() => this.showJoin(e.text + ' Make a new character to play here.'), 1200); }
      }
    };
    if ($('#continue')) $('#continue').onclick = () => go({ t: 'hello', proto: PROTO, client: CLIENT, token: saved.token });
    if ($('#newchar')) $('#newchar').onclick = () => { this.forceNew = true; this.showJoin(); };
    if ($('#join')) {
      $('#join').onclick = () => {
        const name = $('#name').value.trim();
        if (!/^[A-Za-z][A-Za-z0-9_]{2,15}$/.test(name)) { $('#joinerr').textContent = 'Names are 3–16 letters, digits or _, starting with a letter.'; return; }
        const inv = $('#invite') ? $('#invite').value.trim().toUpperCase() : '';
        go({ t: 'hello', proto: PROTO, client: CLIENT, name, color, ...(inv ? { invite: inv } : {}) });
      };
      $('#name').onkeydown = (e) => { if (e.key === 'Enter') $('#join').click(); };
    }
    (form.querySelector('#continue') || form.querySelector('#name'))?.focus();
    // Gamepad navigation on the join screen.
    this.input.setUIMode(true);
    this.input.onUI = (d) => {
      if (!this.me) {
        const els = [...form.querySelectorAll('button, input')];
        const i = els.indexOf(document.activeElement);
        if (d === 'down' || d === 'right') els[(i + 1) % els.length]?.focus();
        if (d === 'up' || d === 'left') els[(i - 1 + els.length) % els.length]?.focus();
        if (d === 'ok') document.activeElement?.click();
      } else this.ui.nav(d);
    };
    if (!this.joinLoop) { this.joinLoop = true; const pl = () => { if (!this.me) { this.input.poll(0.016); requestAnimationFrame(pl); } }; pl(); }
    if (params.get('autojoin')) {
      // Test/automation convenience: ?autojoin=Name joins immediately (never used for secrets).
      if (saved) go({ t: 'hello', proto: PROTO, client: CLIENT, token: saved.token });
      else go({ t: 'hello', proto: PROTO, client: CLIENT, name: params.get('autojoin'), color: COLORS[(Math.random() * COLORS.length) | 0] });
    }
  }

  savedChar(realm) { try { return JSON.parse(localStorage.getItem('lf.char.' + realm) || 'null'); } catch { return null; } }
  forgetChar(realm) { try { localStorage.removeItem('lf.char.' + realm); } catch { } }

  // ---------- game start ----------
  start(w) {
    this.realm = w.realm;
    if (w.token) {
      try { localStorage.setItem('lf.char.' + w.realm.id, JSON.stringify({ token: w.token, name: this.net.hello.name })); } catch { }
      this.net.hello = { t: 'hello', proto: PROTO, client: CLIENT, token: w.token };
    }
    this.myId = w.you;
    const W = w.world;
    if (!this.world) {
      this.world = new World(W.sx, W.sy, W.sz, W.chunk);
      this.gloamX = W.gloamX;
      this.anvil = W.anvil;
      this.spawn = W.spawn;
    }
    const c = w.content;
    this.items = Object.fromEntries(c.items.map((i) => [i.id, i]));
    this.recipes = c.recipes;
    this.quests = Object.fromEntries(c.quests.map((q) => [q.id, q]));
    this.npcs = Object.fromEntries(c.npcs.map((n) => [n.id, n]));
    this.mobDefs = Object.fromEntries(c.mobs.map((m) => [m.id, m]));
    this.nodes = Object.fromEntries(c.nodes.map((n) => [n.block, n]));
    this.body = new Body(w.pos[0], w.pos[1], w.pos[2]);
    this.yaw = w.pos[3] || 0;
    $('#screen-join').hidden = true;
    $('#hud').hidden = false;
    $('#realmbadge').textContent = w.realm.kind === 'official' ? 'Official world' : `Private world · progress stays here${w.realm.modded ? ' · modded' : ''}`;
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
    n.on('me', (m) => { const first = !this.me; this.me = m; ui.updateHUD(m); if (first) ui.toast('Welcome to Wickhollow!', 'good'); });
    n.on('snap', (m) => this.onSnap(m));
    n.on('binary', (b) => {
      if (b[0] !== 1) return;
      const cx = b[1], cz = b[2];
      this.world.applyChunk(cx, cz, b.subarray(3));
      this.chunkPending.delete(`${cx},${cz}`);
      for (const [dx, dz] of [[0, 0], [1, 0], [-1, 0], [0, 1], [0, -1]]) {
        if (this.world.chunkLoaded(cx + dx, cz + dz)) { this.dirty.add(`${cx + dx},${cz + dz},0`); this.dirty.add(`${cx + dx},${cz + dz},1`); }
      }
    });
    n.on('block', (m) => {
      this.world.set(m.x, m.y, m.z, m.b);
      const cs = this.world.cs, cx = Math.floor(m.x / cs), cz = Math.floor(m.z / cs);
      const lx = m.x - cx * cs, lz = m.z - cz * cs;
      this.urgent = this.urgent || new Set();
      this.urgent.add(`${cx},${cz},0`);
      if (lx === 0) this.urgent.add(`${cx - 1},${cz},0`);
      if (lx === cs - 1) this.urgent.add(`${cx + 1},${cz},0`);
      if (lz === 0) this.urgent.add(`${cx},${cz - 1},0`);
      if (lz === cs - 1) this.urgent.add(`${cx},${cz + 1},0`);
      this.dirty.add(`${cx},${cz},1`);
    });
    n.on('correct', (m) => {
      this.body.x = m.x; this.body.y = m.y; this.body.z = m.z; this.body.vy = 0;
      this.lastSent = [m.x, m.y, m.z, this.yaw];
    });
    n.on('hit', (m) => this.onHit(m));
    n.on('die', (m) => { const e = this.mobs.get(m.id); if (e) e.dead = true; });
    n.on('ko', (m) => { ui.toast(m.text, 'danger'); });
    n.on('toast', (m) => ui.toast(m.text, m.kind));
    n.on('chat', (m) => ui.chat(m));
    n.on('levelup', () => { });
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
  }

  entityPos(id) {
    if (id === this.myId) return [this.body.x, this.body.y + 2, this.body.z];
    const e = this.others.get(id) || this.mobs.get(id);
    return e ? [e.x, e.y + 1.8, e.z] : null;
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
      const s = project(this.vp, p[0], p[1], p[2], innerWidth, innerHeight);
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
      if (inp.pressed.has('chat') && this.input.device !== 'touch') { ui.openChat(); }
      if (inp.pressed.has('hotPrev')) ui.cycleHot(-1);
      if (inp.pressed.has('hotNext')) ui.cycleHot(1);
      for (let i = 1; i <= 8; i++) if (inp.pressed.has('slot' + i)) { ui.hot = i - 1; ui.renderHotbar(this.me); }
      if (inp.pressed.has('hearth')) this.net.request({ t: 'hearth' }).then((r) => { if (!r.ok) ui.toast(r.err, 'warn'); });
    }
    // Movement relative to camera yaw. Forward = (sin yaw, cos yaw).
    const [mx, mz] = ui.open ? [0, 0] : inp.move;
    const speed = inp.sprint ? SPRINT : WALK;
    const fx = Math.sin(this.yaw), fz = Math.cos(this.yaw);
    const rx = -fz, rz = fx; // right vector
    const vx = (fx * mz + rx * mx) * speed, vz = (fz * mz + rz * mx) * speed;
    step(this.world, this.body, vx, vz, !ui.open && inp.jump, dt);
    const moving = Math.hypot(vx, vz) > 0.1;
    if (moving) { this.walk += dt * (inp.sprint ? 13 : 10); this.bodyYaw = Math.atan2(vx, vz); }
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
        const msg = this.nodes[tb.block] ? { t: 'gather', bx: tb.x, by: tb.y, bz: tb.z } : placeable ? { t: 'break', bx: tb.x, by: tb.y, bz: tb.z } : null;
        if (msg) this.net.request(msg).then((r) => { if (!r.ok && !/Steady/.test(r.err)) ui.toast(r.err, 'warn'); });
      } else if (!this.targetMob && !this.targetBlock && inp.pressed.has('primary') && this.targetPlayer && this.realm.pvp) {
        this.net.request({ t: 'attack', target: this.targetPlayer.id }).then((r) => { if (!r.ok) ui.toast(r.err, 'warn'); });
      }
    }
    if ((inp.use || inp.pressed.has('use')) && tnow > this.nextUse) {
      const sel = ui.hotSel, it = sel && this.items[sel];
      if (it && it.kind === 'food' && inp.pressed.has('use')) { this.nextUse = tnow + 400; this.net.request({ t: 'eat', item: sel }).then((r) => { if (!r.ok) ui.toast(r.err, 'warn'); }); }
      else if (it && it.kind === 'block' && this.targetBlock) {
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

  nearAnvil() { return !!this.anvil && Math.hypot(this.body.x - this.anvil[0] - 0.5, this.body.z - this.anvil[2] - 0.5) <= 4.5; }

  findTargets() {
    const b = this.body, [ex, ey, ez] = this.eye();
    const fx = Math.sin(this.yaw), fz = Math.cos(this.yaw);
    // Soft-lock the nearest creature in front (controller & touch friendly).
    let best = null, bs = Infinity;
    for (const m of this.mobs.values()) {
      if (m.dead || m.hp <= 0) continue;
      const dx = m.x - b.x, dz = m.z - b.z, d = Math.hypot(dx, dz);
      if (d > 3.2 || Math.abs(m.y - b.y) > 2.4) continue;
      const facing = d < 0.8 ? 1 : (dx * fx + dz * fz) / d;
      if (facing < 0.2) continue;
      const s = d - facing * 1.5;
      if (s < bs) { bs = s; best = m; }
    }
    this.targetMob = best;
    let bp = null; bs = Infinity;
    for (const p of this.others.values()) {
      const dx = p.x - b.x, dz = p.z - b.z, d = Math.hypot(dx, dz);
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
      if (d <= 5.0) this.targetBlock = hit;
    }
    this.nearNPC = null;
    for (const n of Object.values(this.npcs)) {
      if (Math.hypot(n.x - b.x, n.z - b.z) < 4.2 && Math.abs(n.y - b.y) < 3) this.nearNPC = n;
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
    else if (this.me.zone === 'gloam' && this.me.hp < this.me.mhp * 0.4) prompt = `${glyph(dev, 'hearth')} Hearth Lantern to return home`;
    if (pEl.textContent !== prompt) pEl.textContent = prompt;
  }

  // ---------- chunks ----------
  streamChunks() {
    if (!this.world || !this.body || this.net.state !== 'open') return;
    const w = this.world, cs = w.cs;
    const pcx = this.body.x / cs, pcz = this.body.z / cs;
    const need = [];
    const R = this.q.renderDist + 1;
    for (let cz = 0; cz < w.cz; cz++) for (let cx = 0; cx < w.cx; cx++) {
      if (w.chunkLoaded(cx, cz) || this.chunkPending.has(`${cx},${cz}`)) continue;
      const d = Math.hypot(cx + 0.5 - pcx, cz + 0.5 - pcz);
      if (d <= R) need.push([d, cx, cz]);
    }
    if (!need.length || this.chunkPending.size >= 8) return;
    need.sort((a, b) => a[0] - b[0]);
    const batch = need.slice(0, 8 - this.chunkPending.size).map(([, cx, cz]) => { this.chunkPending.add(`${cx},${cz}`); return [cx, cz]; });
    this.net.send({ t: 'chunks', chunks: batch });
  }

  meshBudget(ms) {
    if (!this.world) return;
    const t0 = performance.now();
    const w = this.world, cs = w.cs;
    const pcx = (this.body ? this.body.x : 64) / cs, pcz = (this.body ? this.body.z : 64) / cs;
    const want = (cx, cz, lod) => {
      if (!w.chunkLoaded(cx, cz)) return false;
      const d = Math.hypot(cx + 0.5 - pcx, cz + 0.5 - pcz);
      if (d > this.q.renderDist + 1.5) return false;
      return lod ? d > this.q.lodDist - 0.8 : d <= this.q.lodDist + 1.2;
    };
    // Block edits first so building feels instant.
    if (this.urgent) {
      for (const k of this.urgent) { const [cx, cz, lod] = k.split(',').map(Number); if (w.chunkLoaded(cx, cz)) this.r.setChunk(cx, cz, lod, meshChunk(w, cx, cz, { ao: this.q.ao })); }
      this.urgent.clear();
    }
    // Add newly wanted meshes.
    for (let cz = 0; cz < w.cz; cz++) for (let cx = 0; cx < w.cx; cx++) for (const lod of [0, 1]) {
      const k = `${cx},${cz},${lod}`;
      const has = this.r.chunks.has(k);
      if (want(cx, cz, lod) && !has) this.dirty.add(k);
      if (!want(cx, cz, lod) && has) {
        const d = Math.hypot(cx + 0.5 - pcx, cz + 0.5 - pcz);
        if (d > this.q.renderDist + 2.5 || (lod === 0 && d > this.q.lodDist + 2.5) || (lod === 1 && d < this.q.lodDist - 2)) this.r.dropChunk(cx, cz, lod);
      }
    }
    const list = [...this.dirty].map((k) => { const [cx, cz, lod] = k.split(',').map(Number); return [Math.hypot(cx + 0.5 - pcx, cz + 0.5 - pcz), cx, cz, lod, k]; }).sort((a, b) => a[0] - b[0]);
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
      const dx = e.tx - e.x, dz = e.tz - e.z;
      e.moving = Math.hypot(dx, dz) > 0.02;
      if (e.moving) e.walk = (e.walk || 0) + dt * 10;
      e.x += dx * k; e.y += (e.ty - e.y) * k; e.z += dz * k;
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
    const hasLantern = this.me && this.me.inv.lantern > 0;
    if (boom > 0.9 && !this.bench) drawPerson(r, { x: b.x, y: b.y, z: b.z, yaw: this.bodyYaw || 0, color: this.me.color || '#4a90d9', walk: this.walk, moving: this.moving, swing: this.swing, weapon: wpn ? wpn.color : null, weaponGlow: this.me?.eq === 'glimmer_blade', lantern: hasLantern, flash: this.myFlash });
    for (const p of this.others.values()) drawPerson(r, { ...p, weapon: p.weapon && this.items[p.weapon] ? this.items[p.weapon].color : null });
    for (const n of Object.values(this.npcs)) drawPerson(r, { x: n.x, y: n.y, z: n.z, yaw: n.yaw + Math.sin(this.time * 0.5 + n.x) * 0.2, color: n.color, hat: NPC_HATS[n.id] || 'cap', moving: false });
    for (const m of this.mobs.values()) {
      if (m.dead) continue;
      if (m.kind === 'gloamwisp') drawWisp(r, m, this.time); else drawBramblet(r, m, this.time);
    }
    const inGloam = b.x > this.gloamX - 2;
    const playerLight = hasLantern ? [b.x, b.y + 1.2, b.z, inGloam ? 11 : 6] : (inGloam ? [b.x, b.y + 1.2, b.z, 3.5] : [0, 0, 0, 0]);
    const hl = this.targetBlock && !this.targetMob ? [this.targetBlock.x, this.targetBlock.y, this.targetBlock.z] : null;
    this.vp = r.render(this.world, view, {
      renderDist: q.renderDist, lodDist: q.lodDist, maxLights: q.maxLights, sway: q.sway && !settings.reduceMotion,
      time: this.time, fogNear: q.renderDist * 16 * 0.55, fogFar: q.renderDist * 16 + 8, gloamX: this.gloamX,
      playerLight, highlight: hl,
    });
    this.drawTags();
  }

  drawTags() {
    const W = innerWidth, H = innerHeight, vp = this.vp, b = this.body;
    const want = new Map();
    const add = (key, x, y, z, html, cls) => {
      if (Math.hypot(x - b.x, z - b.z) > 22) return;
      const s = project(vp, x, y, z, W, H);
      if (!s || s[0] < -50 || s[0] > W + 50 || s[1] < -50 || s[1] > H + 50) return;
      want.set(key, { s, html, cls });
    };
    for (const p of this.others.values()) add('p' + p.id, p.x, p.y + 2.15, p.z, `${esc(p.name)}<div class="bar"><div style="width:${p.hp / p.mhp * 100}%"></div></div>`, 'tag');
    for (const m of this.mobs.values()) if (!m.dead && (m.hp < m.mhp || m === this.targetMob || m.aggro)) add('m' + m.id, m.x, m.y + (m.kind === 'gloamwisp' ? 1.7 : 1.15), m.z, `<div class="bar"><div style="width:${m.hp / m.mhp * 100}%"></div></div>`, 'tag');
    for (const n of Object.values(this.npcs)) {
      let mark = '';
      for (const q of Object.values(this.quests)) if (q.giver === n.id) {
        const st = this.me && this.me.quests[q.id];
        if (!st && (!q.requires || (this.me.quests[q.requires] && this.me.quests[q.requires].status === 'done'))) mark = '<span class="q">!</span><br>';
        else if (st && st.status === 'active') mark = mark || '<span class="q" style="color:var(--quest)">?</span><br>';
      }
      add('n' + n.id, n.x, n.y + 2.35, n.z, `${mark}${esc(n.name)}`, 'tag npc');
    }
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
      this.bench = { t: 0, frames: [], net: [], eye: [64, 30, 64], target: [64, 12, 64] };
      this.frameTimes.length = 0;
      console.log('[bench] start', this.q.name);
    };
    setTimeout(wait, 500);
  }

  benchStep(dt) {
    const bn = this.bench;
    bn.t += dt;
    const a = bn.t * 0.31;
    bn.eye = [64 + Math.cos(a) * 30, 24 + Math.sin(a * 0.7) * 4, 64 + Math.sin(a) * 30];
    bn.target = [64 + Math.cos(a + 1.2) * 10, 13, 64 + Math.sin(a + 1.2) * 10];
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
