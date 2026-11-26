import { settings } from './settings.js';

// Gamepad "standard" mapping indices.
const PAD = { A: 0, B: 1, X: 2, Y: 3, LB: 4, RB: 5, LT: 6, RT: 7, BACK: 8, START: 9, L3: 10, R3: 11, UP: 12, DOWN: 13, LEFT: 14, RIGHT: 15 };
const PAD_ACTIONS = { [PAD.A]: 'jump', [PAD.X]: 'interact', [PAD.Y]: 'menu', [PAD.LB]: 'hotPrev', [PAD.RB]: 'hotNext', [PAD.LT]: 'use', [PAD.RT]: 'primary', [PAD.START]: 'pause', [PAD.BACK]: 'chat', [PAD.L3]: 'sprint', [PAD.DOWN]: 'hearth', [PAD.LEFT]: 'drop', [PAD.RIGHT]: 'map', [PAD.B]: 'back' };
const DEAD = 0.18;

const dz = (v) => (Math.abs(v) < DEAD ? 0 : (v - Math.sign(v) * DEAD) / (1 - DEAD));

export class Input {
  constructor(canvas) {
    this.canvas = canvas;
    this.keys = new Set();
    this.held = new Set();
    this.pressed = new Set();
    this.look = [0, 0];
    this.move = [0, 0];
    this.device = 'kb';
    this.padPrev = [];
    this.uiMode = false;
    this.capture = null; // key-remap capture callback
    this.sprintToggle = false;
    this.touch = { stickId: null, lookId: null, sx: 0, sy: 0, lx: 0, ly: 0, mx: 0, my: 0 };
    this.onUI = null; // (kind) => void : 'up'|'down'|'left'|'right'|'ok'|'alt'|'back'|'tabPrev'|'tabNext'
    this.bind();
  }

  codeAction(code) {
    for (const [a, c] of Object.entries(settings.keys)) if (c === code) return a;
    return null;
  }

  bind() {
    addEventListener('keydown', (e) => {
      if (this.capture) { e.preventDefault(); this.capture(e.code); return; }
      this.device = 'kb';
      const tag = document.activeElement && document.activeElement.tagName;
      const typing = tag === 'SELECT' || (tag === 'INPUT' && !['checkbox', 'range', 'radio'].includes(document.activeElement.type));
      if (typing) { if (e.code === 'Escape') document.activeElement.blur(); return; }
      if (this.uiMode) {
        const m = { ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right', Escape: 'back', KeyQ: 'tabPrev', KeyR: 'tabNext' }[e.code];
        if (m) { e.preventDefault(); this.onUI && this.onUI(m); return; }
        if (e.code === settings.keys.menu || e.code === settings.keys.map) { e.preventDefault(); this.onUI && this.onUI('back'); return; }
        if (e.code === settings.keys.drop) { e.preventDefault(); this.onUI && this.onUI('alt'); return; }
        return;
      }
      const a = this.codeAction(e.code);
      if (a) { e.preventDefault(); if (!this.held.has(a)) this.pressed.add(a); this.held.add(a); }
      if (/^Digit[1-8]$/.test(e.code)) this.pressed.add('slot' + e.code.slice(5));
    });
    addEventListener('keyup', (e) => { const a = this.codeAction(e.code); if (a) this.held.delete(a); });
    addEventListener('blur', () => this.held.clear());

    const c = this.canvas;
    c.addEventListener('mousedown', (e) => {
      this.device = 'kb';
      if (this.uiMode) return;
      if (document.pointerLockElement !== c && !this.isTouch) { c.requestPointerLock && c.requestPointerLock(); return; }
      const code = 'Mouse' + e.button;
      if (this.capture) { this.capture(code); return; }
      const a = this.codeAction(code);
      if (a) { this.pressed.add(a); this.held.add(a); }
    });
    addEventListener('mouseup', (e) => { const a = this.codeAction('Mouse' + e.button); if (a) this.held.delete(a); });
    c.addEventListener('contextmenu', (e) => e.preventDefault());
    addEventListener('mousemove', (e) => {
      if (document.pointerLockElement === c) {
        const s = 0.0024 * settings.sensitivity;
        this.look[0] += e.movementX * s;
        this.look[1] += e.movementY * s * (settings.invertY ? -1 : 1);
      }
    });
    c.addEventListener('wheel', (e) => { this.pressed.add(e.deltaY > 0 ? 'hotNext' : 'hotPrev'); }, { passive: true });
    this.bindTouch();
  }

  get isTouch() { return document.body.classList.contains('touch'); }

  bindTouch() {
    const stick = document.getElementById('stick'), knob = document.getElementById('stickknob');
    const t = this.touch;
    const enableTouch = () => { if (!this.isTouch) { document.body.classList.add('touch'); document.getElementById('touch').hidden = this.uiMode; } this.device = 'touch'; };
    addEventListener('touchstart', enableTouch, { capture: true, passive: true });
    stick.addEventListener('touchstart', (e) => {
      e.preventDefault();
      const tt = e.changedTouches[0];
      const r = stick.getBoundingClientRect();
      t.stickId = tt.identifier; t.sx = r.left + r.width / 2; t.sy = r.top + r.height / 2; t.r = r.width / 2;
      this.updateStick(tt, knob);
    }, { passive: false });
    this.canvas.addEventListener('touchstart', (e) => {
      for (const tt of e.changedTouches) if (t.lookId === null) { t.lookId = tt.identifier; t.lx = tt.clientX; t.ly = tt.clientY; t.tapStart = performance.now(); }
    }, { passive: true });
    addEventListener('touchmove', (e) => {
      for (const tt of e.changedTouches) {
        if (tt.identifier === t.stickId) { e.preventDefault(); this.updateStick(tt, knob); }
        if (tt.identifier === t.lookId) {
          const s = 0.006 * settings.sensitivity;
          this.look[0] += (tt.clientX - t.lx) * s;
          this.look[1] += (tt.clientY - t.ly) * s * (settings.invertY ? -1 : 1);
          t.lx = tt.clientX; t.ly = tt.clientY;
        }
      }
    }, { passive: false });
    const end = (e) => {
      for (const tt of e.changedTouches) {
        if (tt.identifier === t.stickId) { t.stickId = null; t.mx = t.my = 0; knob.style.transform = ''; }
        if (tt.identifier === t.lookId) t.lookId = null;
      }
    };
    addEventListener('touchend', end); addEventListener('touchcancel', end);
    for (const btn of document.querySelectorAll('#tbuttons .tb')) {
      const a = btn.dataset.act;
      btn.addEventListener('touchstart', (e) => {
        e.preventDefault(); e.stopPropagation();
        if (a === 'sprint') { this.sprintToggle = !this.sprintToggle; btn.classList.toggle('on', this.sprintToggle); return; }
        this.pressed.add(a); this.held.add(a);
      }, { passive: false });
      btn.addEventListener('touchend', (e) => { e.preventDefault(); this.held.delete(a); }, { passive: false });
      btn.addEventListener('click', () => { if (!this.isTouch) this.pressed.add(a); });
    }
  }

  updateStick(tt, knob) {
    const t = this.touch;
    let dx = (tt.clientX - t.sx) / t.r, dy = (tt.clientY - t.sy) / t.r;
    const l = Math.hypot(dx, dy);
    if (l > 1) { dx /= l; dy /= l; }
    t.mx = dx; t.my = dy;
    knob.style.transform = `translate(${dx * t.r * 0.6}px, ${dy * t.r * 0.6}px)`;
  }

  // Call once per frame; returns the frame's input snapshot.
  poll(dt) {
    let mx = 0, mz = 0;
    if (this.held.has('forward')) mz += 1;
    if (this.held.has('back')) mz -= 1;
    if (this.held.has('left')) mx -= 1;
    if (this.held.has('right')) mx += 1;
    if (this.touch.stickId !== null) { mx += this.touch.mx; mz -= this.touch.my; }
    let sprint = this.held.has('sprint') || this.sprintToggle;
    const pads = navigator.getGamepads ? navigator.getGamepads() : [];
    for (const p of pads) {
      if (!p || !p.connected) continue;
      const lx = dz(p.axes[0] || 0), ly = dz(p.axes[1] || 0), rx = dz(p.axes[2] || 0), ry = dz(p.axes[3] || 0);
      const prev = this.padPrev[p.index] || [];
      const now = p.buttons.map((b) => b.pressed || b.value > 0.5);
      if (lx || ly || rx || ry || now.some((v) => v)) this.device = 'pad';
      if (this.uiMode) {
        const edge = (i) => now[i] && !prev[i];
        const stickDir = Math.abs(ly) > 0.6 ? (ly < 0 ? 'up' : 'down') : Math.abs(lx) > 0.6 ? (lx < 0 ? 'left' : 'right') : null;
        if (stickDir && (!this.lastStick || performance.now() - this.lastStick > 220)) { this.onUI && this.onUI(stickDir); this.lastStick = performance.now(); }
        if (!stickDir) this.lastStick = 0;
        if (edge(PAD.UP)) this.onUI('up'); if (edge(PAD.DOWN)) this.onUI('down');
        if (edge(PAD.LEFT)) this.onUI('left'); if (edge(PAD.RIGHT)) this.onUI('right');
        if (edge(PAD.X)) this.onUI('alt');
        if (edge(PAD.A)) this.onUI('ok'); if (edge(PAD.B) || edge(PAD.START) || edge(PAD.Y)) this.onUI('back');
        if (edge(PAD.LB)) this.onUI('tabPrev'); if (edge(PAD.RB)) this.onUI('tabNext');
      } else {
        mx += lx; mz -= ly;
        const s = 2.6 * settings.sensitivity * dt;
        this.look[0] += rx * s;
        this.look[1] += ry * s * (settings.invertY ? -1 : 1);
        for (const [i, a] of Object.entries(PAD_ACTIONS)) {
          if (now[i] && !prev[i]) { if (a === 'sprint') this.sprintToggle = !this.sprintToggle; else this.pressed.add(a); }
          if (now[i]) this.held.add('pad:' + a); else this.held.delete('pad:' + a);
        }
        if (Math.hypot(lx, ly) < 0.2) this.sprintToggle = this.device === 'pad' ? false : this.sprintToggle;
      }
      this.padPrev[p.index] = now;
    }
    const l = Math.hypot(mx, mz);
    if (l > 1) { mx /= l; mz /= l; }
    const snap = {
      move: [mx, mz], look: this.look.slice(), sprint,
      jump: this.held.has('jump') || this.held.has('pad:jump') || this.pressed.has('jump'),
      primary: this.held.has('primary') || this.held.has('pad:primary'),
      use: this.held.has('use') || this.held.has('pad:use'),
      hearthHeld: this.held.has('hearth') || this.held.has('pad:hearth'),
      pressed: new Set(this.pressed),
    };
    this.look[0] = this.look[1] = 0;
    this.pressed.clear();
    return snap;
  }

  setUIMode(on) {
    this.uiMode = on;
    this.held.clear();
    if (on && document.pointerLockElement) document.exitPointerLock();
    document.getElementById('touch').hidden = on || !this.isTouch;
  }
}

export function glyph(device, action) {
  const pad = { primary: 'RT', use: 'LT', interact: 'Ⓧ', menu: 'Ⓨ', jump: 'Ⓐ', hearth: 'D-pad ↓', drop: 'D-pad ←', map: 'D-pad →', pause: 'Start', back: 'Ⓑ', alt: 'Ⓧ', ok: 'Ⓐ' };
  const touch = { primary: '⚔', use: '✋', interact: '💬', menu: '☰', jump: '⤒', hearth: '🏮' };
  if (device === 'pad') return pad[action] || action;
  if (device === 'touch') return touch[action] || action;
  const code = settings.keys[action === 'alt' ? 'drop' : action] || '';
  return code.replace(/^Key/, '').replace(/^Digit/, '').replace('Mouse0', 'Left click').replace('Mouse2', 'Right click').replace('ShiftLeft', 'Shift');
}
