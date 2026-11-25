import { settings, save, PRESETS, ACTIONS, DEFAULT_KEYS, quality } from './settings.js';
import { glyph } from './input.js';

const $ = (s) => document.querySelector(s);
export const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const SKILL_LABEL = { woodcutting: 'Woodcutting', mining: 'Mining', crafting: 'Crafting', combat: 'Combat' };
const SKILL_ICON = { woodcutting: '🪓', mining: '⛏', crafting: '🔨', combat: '⚔' };

export function applyAccessibility() {
  document.documentElement.style.setProperty('--ui-scale', settings.textScale);
  document.body.classList.toggle('reduce-motion', !!settings.reduceMotion);
  // Background blur costs GPU time over the 3D scene: use solid panels when asked, or on the Low preset.
  document.body.classList.toggle('no-glass', !!settings.reduceTransparency || quality().name === 'low');
  $('#perf').hidden = !settings.showPerf;
}

export class UI {
  constructor(game) {
    this.g = game;
    this.panel = null; // {kind, tab}
    this.dialog = null;
    this.hot = 0;
    $('#panel').addEventListener('click', (e) => {
      const b = e.target.closest('[data-close]');
      if (b) this.close();
    });
    $('#chatinput').addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        const v = e.target.value.trim();
        e.target.value = '';
        e.target.hidden = true;
        e.target.blur();
        if (v) this.g.net.request({ t: 'chat', text: v });
        e.stopPropagation();
      }
    });
  }

  // ---------- HUD ----------
  toast(text, kind = 'info') {
    const el = document.createElement('div');
    el.className = 'toast ' + kind;
    el.textContent = text;
    $('#toasts').appendChild(el);
    while ($('#toasts').children.length > 4) $('#toasts').firstChild.remove();
    setTimeout(() => el.remove(), 3300);
  }

  chat(m) {
    const el = document.createElement('div');
    if (m.kind === 'system') { el.className = 'sys'; el.textContent = m.text; }
    else { const b = document.createElement('b'); b.textContent = m.from + ': '; el.append(b, document.createTextNode(m.text)); }
    const log = $('#chatlog');
    log.appendChild(el);
    while (log.children.length > 8) log.firstChild.remove();
  }

  openChat() {
    const i = $('#chatinput');
    i.hidden = false;
    i.focus();
  }

  item(id) { return this.g.items[id] || { name: id + ' (missing mod)', color: '#666666', kind: 'missing' }; }

  icon(id) { return `<span class="ic" style="background:${esc(this.item(id).color)}"></span>`; }

  updateHUD(me) {
    const g = this.g;
    $('#hpfill').style.width = `${Math.max(0, me.hp / me.mhp * 100)}%`;
    $('#hptext').textContent = `${me.hp}/${me.mhp}`;
    const z = $('#zone');
    const zn = { village: '🏡 Wickhollow — safe', wild: '🌿 The Meadows', gloam: '⚠ DANGER · The Gloam' }[me.zone] || me.zone;
    z.textContent = zn;
    z.className = me.zone === 'gloam' ? 'gloam' : '';
    $('#vignette').classList.toggle('on', me.zone === 'gloam' && !settings.reduceMotion);
    // Quest tracker: first active quest.
    const tr = $('#tracker');
    let html = '';
    for (const [qid, st] of Object.entries(me.quests || {})) {
      if (st.status !== 'active') continue;
      const q = g.quests[qid];
      if (!q) continue;
      html += `<h4>${esc(q.name)}</h4><ul style="margin:0;padding-left:1.1em">` +
        q.steps.map((s, i) => {
          const have = s.kind === 'deliver' ? Math.min(s.count, (me.inv[s.target] || 0)) : (st.progress[i] || 0);
          return `<li class="${have >= s.count ? 'done' : ''}">${esc(s.text)} (${have}/${s.count})</li>`;
        }).join('') + '</ul>';
      break;
    }
    if (!html && !(me.quests && me.quests.lights_for_wickhollow)) html = `<h4>Welcome!</h4>Find <b>Mabel the Lamplighter</b> near the plaza and press ${esc(glyph(g.input.device, 'interact'))} to talk.`;
    tr.innerHTML = html;
    // Hearth progress.
    const hb = $('#hearthbar');
    hb.hidden = !(me.hearth > 0);
    $('#hearthfill').style.width = `${Math.min(1, me.hearth) * 100}%`;
    this.renderHotbar(me);
    if (this.panel && this.panel.kind === 'menu') this.renderPanel();
    if (this.panel && this.panel.kind === 'trade') this.renderTrade();
    if (this.panel && this.panel.kind === 'dialog' && this.dialog) this.renderDialog();
    if (me.trade && !(this.panel && this.panel.kind === 'trade')) this.openTrade();
    if (!me.trade && this.panel && this.panel.kind === 'trade') this.close();
  }

  hotItems(me) {
    return Object.keys(me.inv).filter((id) => { const k = this.item(id).kind; return k === 'block' || k === 'food'; }).sort();
  }

  renderHotbar(me) {
    const items = this.hotItems(me);
    if (this.hot >= items.length) this.hot = Math.max(0, items.length - 1);
    this.hotSel = items[this.hot] || null;
    $('#hotbar').innerHTML = items.slice(0, 8).map((id, i) =>
      `<button class="slot ${i === this.hot ? 'sel' : ''}" data-hot="${i}" title="${esc(this.item(id).name)}" tabindex="-1"><span class="k">${i + 1}</span>${this.icon(id)}<span class="n">${me.inv[id]}</span></button>`).join('');
    for (const b of document.querySelectorAll('#hotbar .slot')) b.onclick = () => { this.hot = +b.dataset.hot; this.renderHotbar(this.g.me); };
  }

  cycleHot(d) {
    const n = this.hotItems(this.g.me).length;
    if (n) { this.hot = (this.hot + d + n) % n; this.renderHotbar(this.g.me); }
  }

  // ---------- panels ----------
  get open() { return !!this.panel; }

  show(kind, title, tabs, tab) {
    this.panel = { kind, tab };
    $('#paneltitle').textContent = title;
    $('#tabs').innerHTML = (tabs || []).map(([id, label]) => `<button class="tab" role="tab" data-tab="${id}" aria-selected="${id === tab}">${label}</button>`).join('');
    for (const b of document.querySelectorAll('#tabs .tab')) b.onclick = () => { this.panel.tab = b.dataset.tab; this.show(kind, title, tabs, b.dataset.tab); };
    $('#panel').hidden = false;
    this.g.input.setUIMode(true);
    this.renderPanel();
    this.focusFirst();
  }

  close() {
    if (this.panel && this.panel.kind === 'trade' && this.g.me.trade) {
      this.g.net.request({ t: 'trade_cancel', trade: this.g.me.trade.id });
    }
    this.panel = null; this.dialog = null;
    $('#panel').hidden = true;
    this.g.input.setUIMode(false);
  }

  openMenu(tab = 'pack') {
    this.show('menu', 'Adventurer\'s Pack', [['pack', '🎒 Pack'], ['craft', '🔨 Craft'], ['skills', '⭐ Skills'], ['quests', '📜 Quests'], ['settings', '⚙ Settings'], ['controls', '🎮 Controls']], tab);
  }

  renderPanel() {
    if (!this.panel) return;
    const body = $('#panelbody');
    const keepFocus = document.activeElement && body.contains(document.activeElement) ? document.activeElement.dataset.fid : null;
    if (this.panel.kind === 'menu') {
      const tab = this.panel.tab;
      body.innerHTML = this['tab_' + tab]();
      this.wire(body);
    } else if (this.panel.kind === 'dialog') return this.renderDialog();
    else if (this.panel.kind === 'trade') return this.renderTrade();
    if (keepFocus) { const el = body.querySelector(`[data-fid="${keepFocus}"]`); if (el) el.focus(); }
  }

  wire(root) {
    for (const el of root.querySelectorAll('[data-do]')) {
      el.onclick = () => this.act(el.dataset.do, el.dataset.arg, el);
    }
    for (const el of root.querySelectorAll('[data-set]')) {
      el.onchange = () => this.setSetting(el.dataset.set, el);
    }
  }

  async act(what, arg, el) {
    const g = this.g, n = g.net;
    let r;
    switch (what) {
      case 'equip': r = await n.request({ t: 'equip', item: arg }); break;
      case 'eat': r = await n.request({ t: 'eat', item: arg }); break;
      case 'craft': r = await n.request({ t: 'craft', recipe: arg }); break;
      case 'accept': r = await n.request({ t: 'quest_accept', quest: arg }); if (r.ok) this.talk(this.dialog.npc); break;
      case 'turnin': r = await n.request({ t: 'quest_turnin', quest: arg }); if (r.ok) this.talk(this.dialog.npc); break;
      case 'buy': r = await n.request({ t: 'buy', item: arg, count: 1 }); break;
      case 'sell': r = await n.request({ t: 'sell', item: arg, count: +el.dataset.n || 1 }); break;
      case 'hearth': this.close(); r = await n.request({ t: 'hearth' }); break;
      case 'offer': {
        const tr = g.me.trade; if (!tr) return;
        const o = { ...(tr.mine || {}) };
        const [id, d] = arg.split('|');
        o[id] = Math.max(0, Math.min(g.me.inv[id] || 0, (o[id] || 0) + +d));
        if (!o[id]) delete o[id];
        r = await n.request({ t: 'trade_offer', trade: tr.id, offer: o });
        break;
      }
      case 'lock': r = await n.request({ t: 'trade_lock', trade: g.me.trade.id }); break;
      case 'confirm': r = await n.request({ t: 'trade_confirm', trade: g.me.trade.id }); break;
      case 'tradeAccept': r = await n.request({ t: 'trade_respond', trade: g.me.trade.id, accept: true }); break;
      case 'tradeDecline': r = await n.request({ t: 'trade_respond', trade: g.me.trade.id, accept: false }); this.close(); break;
      case 'tradeCancel': this.close(); return;
      case 'preset': settings.quality = arg; settings.custom = null; save(); this.g.applyQuality(); this.renderPanel(); return;
      case 'rebind': this.rebind(arg, el); return;
      case 'resetKeys': settings.keys = { ...DEFAULT_KEYS }; save(); this.renderPanel(); return;
      case 'leave': location.reload(); return;
      case 'exportWorld': {
        const bytes = await this.g.net.exportWorld();
        const a = document.createElement('a');
        a.href = URL.createObjectURL(new Blob([bytes], { type: 'application/json' }));
        a.download = `lanternfall-world-${new Date().toISOString().slice(0, 10)}.json`;
        document.body.appendChild(a); a.click(); a.remove();
        setTimeout(() => URL.revokeObjectURL(a.href), 5000);
        this.toast('World exported. Keep the file somewhere safe.', 'good');
        return;
      }
      case 'importWorld': {
        const input = document.createElement('input');
        input.type = 'file'; input.accept = '.json,application/json';
        input.onchange = async () => {
          const f = input.files && input.files[0];
          if (!f) return;
          if (f.size > 32 << 20) { this.toast('That file is too large to be a world save.', 'warn'); return; }
          if (!confirm('Replace this browser’s world with the imported one? Your current world is kept as a backup in this browser.')) return;
          const res = await this.g.net.importWorld(new Uint8Array(await f.arrayBuffer()));
          if (!res.ok) { this.toast(res.text, 'warn'); return; }
          this.toast('World imported. Reloading…', 'good');
          setTimeout(() => location.reload(), 600);
        };
        input.click();
        return;
      }
    }
    if (r && !r.ok && r.err) this.toast(r.err, 'warn');
  }

  setSetting(key, el) {
    const v = el.type === 'checkbox' ? el.checked : el.type === 'range' ? +el.value : el.value;
    if (['renderDist', 'resScale', 'ao', 'sway', 'maxLights'].includes(key)) {
      settings.custom = { ...(settings.custom || {}), [key]: v };
    } else settings[key] = v;
    save();
    applyAccessibility();
    this.g.applyQuality();
    this.renderPanel();
  }

  rebind(action, el) {
    el.textContent = 'Press a key…';
    this.g.input.capture = (code) => {
      this.g.input.capture = null;
      if (code !== 'Escape') { settings.keys[action] = code; save(); }
      this.renderPanel();
    };
  }

  tab_pack() {
    const me = this.g.me;
    const ids = Object.keys(me.inv).filter((i) => i !== 'coin').sort();
    const cell = (id) => {
      const it = this.item(id);
      let act = '';
      if (it.kind === 'weapon') act = me.eq === id ? '<small>Equipped</small>' : `<small>Ⓐ Equip</small>`;
      else if (it.kind === 'food') act = `<small>Ⓐ Eat (+${it.heal} HP)</small>`;
      else if (it.kind === 'missing') act = '<small>From a disabled mod — kept safe</small>';
      else act = `<small>${esc(it.desc || it.kind)}</small>`;
      const doAttr = it.kind === 'weapon' ? `data-do="equip" data-arg="${esc(id)}"` : it.kind === 'food' ? `data-do="eat" data-arg="${esc(id)}"` : '';
      return `<button class="item ${me.eq === id ? 'eq' : ''}" data-fid="i-${esc(id)}" ${doAttr}>${this.icon(id)}<span><b>${esc(it.name)}</b> ×${me.inv[id]}${act}</span></button>`;
    };
    return `<div class="row"><b>🪙 ${me.inv.coin || 0} coins</b> · <span class="note">${me.slots}/${me.maxSlots} slots used</span>
      <span style="flex:1"></span><button class="btn" data-do="hearth" data-fid="hearth">🏮 Hearth Lantern (return home)</button></div>
      <div class="grid">${ids.map(cell).join('') || '<p class="note">Your pack is empty. Try chopping a tree in Mossmeadow!</p>'}</div>`;
  }

  tab_craft() {
    const me = this.g.me, lvl = me.skills.crafting.lvl;
    return this.g.recipes.map((r) => {
      const missing = Object.entries(r.inputs).some(([k, v]) => (me.inv[k] || 0) < v);
      const lvOK = lvl >= r.level;
      const req = Object.entries(r.inputs).map(([k, v]) => `<span class="${(me.inv[k] || 0) < v ? 'miss' : ''}">${v}× ${esc(this.item(k).name)} (${me.inv[k] || 0})</span>`).join(', ');
      const st = r.station === 'anvil' ? ' · <span class="' + (this.g.nearAnvil() ? '' : 'miss') + '">at the Anvil</span>' : '';
      return `<div class="recipe">${this.icon(r.output)}<div class="info"><b>${esc(this.item(r.output).name)}${r.count > 1 ? ' ×' + r.count : ''}</b>
        <div class="req">${req}${st} · <span class="${lvOK ? '' : 'miss'}">Crafting ${r.level}</span> · +${r.xp} XP</div></div>
        <button class="btn ${!missing && lvOK ? 'primary' : ''}" data-do="craft" data-arg="${esc(r.id)}" data-fid="c-${esc(r.id)}" ${missing || !lvOK ? 'aria-disabled="true"' : ''}>Craft</button></div>`;
    }).join('');
  }

  tab_skills() {
    const me = this.g.me;
    return Object.entries(me.skills).map(([s, v]) => {
      const pct = v.lvl >= 10 ? 100 : ((v.xp - v.cur) / Math.max(1, v.next - v.cur)) * 100;
      return `<div class="skill" tabindex="0" data-fid="s-${s}"><b>${SKILL_ICON[s] || ''} ${SKILL_LABEL[s] || s}</b> — level ${v.lvl}${v.lvl >= 10 ? ' (max)' : ''}
        <span class="note">${v.xp} XP${v.lvl < 10 ? ` · ${v.next - v.xp} to next` : ''}</span><div class="xp"><div style="width:${pct}%"></div></div></div>`;
    }).join('') + `<p class="note">Combat level raises your max health and damage. Higher Mining unlocks Copper (2) and Glimmer (4); Woodcutting 3 lets you chop Gloam Deadwood.</p>`;
  }

  tab_quests() {
    const me = this.g.me;
    const list = Object.values(this.g.quests).map((q) => {
      const st = me.quests[q.id];
      const status = !st ? 'Not started' : st.status === 'done' ? '✓ Complete' : 'In progress';
      const steps = q.steps.map((s, i) => {
        const have = st ? (s.kind === 'deliver' ? Math.min(s.count, me.inv[s.target] || 0) : st.progress[i] || 0) : 0;
        return `<li class="${have >= s.count ? 'done' : ''}">${esc(s.text)} ${st ? `(${have}/${s.count})` : ''}</li>`;
      }).join('');
      const giver = this.g.npcs[q.giver];
      return `<div class="quest" tabindex="0" data-fid="q-${esc(q.id)}"><h4>${esc(q.name)} <span class="note">— ${status}</span></h4>
        <div class="note">From ${esc(giver ? giver.name : q.giver)}${q.requires ? ' · after ' + esc(this.g.quests[q.requires]?.name) : ''}</div><ul>${steps}</ul>
        <div class="note">Reward: ${q.reward.coins} coins${Object.entries(q.reward.items || {}).map(([k, v]) => `, ${v}× ${esc(this.item(k).name)}`).join('')}${Object.entries(q.reward.xp || {}).map(([k, v]) => `, ${v} ${SKILL_LABEL[k] || k} XP`).join('')}</div></div>`;
    });
    return list.join('');
  }

  tab_settings() {
    const q = quality();
    const opt = (k, v, label) => `<option value="${v}" ${settings[k] === v ? 'selected' : ''}>${label}</option>`;
    return `<div class="settings">
      <h4>Graphics</h4>
      <div class="row">${['auto', 'low', 'medium', 'high'].map((p) => `<button class="btn ${settings.quality === p && !settings.custom ? 'primary' : ''}" data-do="preset" data-arg="${p}" data-fid="p-${p}">${p === 'auto' ? 'Auto (' + q.name + ')' : PRESETS[p].label.split(' ')[0]}</button>`).join('')}</div>
      <label>Render distance (chunks) <input type="range" min="2" max="8" step="1" value="${q.renderDist}" data-set="renderDist" data-fid="rd"></label>
      <label>Resolution scale <input type="range" min="0.4" max="1" step="0.05" value="${q.resScale}" data-set="resScale" data-fid="rs"></label>
      <label>Ambient occlusion <input type="checkbox" ${q.ao ? 'checked' : ''} data-set="ao" data-fid="ao"></label>
      <label>Swaying leaves &amp; water <input type="checkbox" ${q.sway ? 'checked' : ''} data-set="sway" data-fid="sw"></label>
      <h4>Accessibility</h4>
      <label>Text size <select data-set="textScale" data-fid="ts">${[0.85, 1, 1.15, 1.3, 1.5].map((v) => `<option value="${v}" ${+settings.textScale === v ? 'selected' : ''}>${Math.round(v * 100)}%</option>`).join('')}</select></label>
      <label>Reduce motion &amp; effects (no vignette, shake, sway, floating numbers) <input type="checkbox" ${settings.reduceMotion ? 'checked' : ''} data-set="reduceMotion" data-fid="rm"></label>
      <label>Reduce transparency (solid panels instead of glass; always on with Low graphics) <input type="checkbox" ${settings.reduceTransparency ? 'checked' : ''} data-set="reduceTransparency" data-fid="rt"></label>
      <label>Look sensitivity <input type="range" min="0.3" max="2.5" step="0.1" value="${settings.sensitivity}" data-set="sensitivity" data-fid="se"></label>
      <label>Invert look Y <input type="checkbox" ${settings.invertY ? 'checked' : ''} data-set="invertY" data-fid="iy"></label>
      <label>Show performance overlay <input type="checkbox" ${settings.showPerf ? 'checked' : ''} data-set="showPerf" data-fid="pf"></label>
      <h4>World</h4>
      <p class="note">${esc(this.g.realmText())}</p>
      ${this.g.browserWorld ? `<div class="row">
        <button class="btn" data-do="exportWorld" data-fid="ex">⬇ Export world</button>
        <button class="btn" data-do="importWorld" data-fid="im">⬆ Import world…</button>
      </div>
      <p class="note">Saved automatically every 20 seconds, after important moments, and when you switch away.
        ${this.g.lastSaved ? 'Last saved ' + new Date(this.g.lastSaved).toLocaleTimeString() + '. ' : ''}
        ${this.g.storagePersisted ? 'This browser agreed to keep your world’s storage.' : 'This browser may clear the world’s storage when space runs low, so export a copy now and then.'}</p>` : ''}
      <button class="btn danger" data-do="leave" data-fid="leave">Leave world</button>
    </div>`;
  }

  tab_controls() {
    const rows = Object.entries(ACTIONS).map(([a, label]) =>
      `<label>${esc(label)} <button class="btn bind" data-do="rebind" data-arg="${a}" data-fid="k-${a}">${esc(glyph('kb', a))}</button></label>`).join('');
    return `<div class="settings"><h4>Controller</h4>
      <p class="note">Left stick move · Right stick look · Ⓐ jump · RT attack/gather · LT use/place · Ⓧ talk/trade · Ⓨ menu · LB/RB hotbar · D-pad ↓ Hearth · L3 sprint · Start settings · Back chat</p>
      <h4>Touch</h4><p class="note">Left stick to move, drag the right side to look. ⚔ attack/gather (auto-targets the nearest creature), ✋ use/place, 💬 talk/trade.</p>
      <h4>Keyboard &amp; mouse (click to remap)</h4>${rows}<button class="btn" data-do="resetKeys" data-fid="rk">Reset to defaults</button></div>`;
  }

  // ---------- NPC dialog ----------
  async talk(npc) {
    const r = await this.g.net.request({ t: 'talk', npc });
    if (!r.ok) this.toast(r.err, 'warn');
  }

  showDialog(d) {
    this.dialog = d;
    if (!this.panel || this.panel.kind !== 'dialog') {
      this.panel = { kind: 'dialog' };
      $('#tabs').innerHTML = '';
      $('#panel').hidden = false;
      this.g.input.setUIMode(true);
      this.renderDialog();
      this.focusFirst();
    } else this.renderDialog();
  }

  renderDialog() {
    const d = this.dialog, me = this.g.me;
    $('#paneltitle').textContent = d.name;
    let html = `<div class="dialog">`;
    let spoke = false;
    for (const qs of d.quests || []) {
      const q = this.g.quests[qs.id];
      if (!q) continue;
      if (qs.status === 'available') { html += `<p>“${esc(q.intro)}”</p><button class="btn primary" data-do="accept" data-arg="${esc(q.id)}" data-fid="qa">Accept: ${esc(q.name)}</button>`; spoke = true; }
      else if (qs.status === 'ready') { html += `<p>“Oh! Is that everything?”</p><button class="btn primary" data-do="turnin" data-arg="${esc(q.id)}" data-fid="qt">Complete: ${esc(q.name)}</button>`; spoke = true; }
      else if (qs.status === 'active') { html += `<p>“How's it going with <b>${esc(q.name)}</b>? Check your quest tracker.”</p>`; spoke = true; }
      else if (qs.status === 'locked') { html += `<p class="note">${esc(q.name)} unlocks after “${esc(this.g.quests[q.requires]?.name)}”.</p>`; }
    }
    if (!spoke) html += `<p>“${esc((d.lines || [])[0] || 'Hello!')}”</p>`;
    if (d.shop && d.shop.length) {
      html += `<h4>Buy</h4><div class="grid">` + d.shop.map((s) => `<button class="item" data-do="buy" data-arg="${esc(s.item)}" data-fid="b-${esc(s.item)}">${this.icon(s.item)}<span><b>${esc(this.item(s.item).name)}</b><small>${s.price} coins · you have ${me.inv.coin || 0}</small></span></button>`).join('') + '</div>';
      const sellable = Object.keys(me.inv).filter((id) => { const it = this.g.items[id]; return it && it.value > 0 && it.tradeable && id !== 'coin'; });
      if (sellable.length) html += `<h4>Sell</h4><div class="grid">` + sellable.map((id) => `<button class="item" data-do="sell" data-arg="${esc(id)}" data-n="${me.inv[id]}" data-fid="s-${esc(id)}">${this.icon(id)}<span><b>${esc(this.item(id).name)} ×${me.inv[id]}</b><small>${this.item(id).value} coin each — sell all</small></span></button>`).join('') + '</div>';
    }
    html += `</div>`;
    const body = $('#panelbody');
    const keep = document.activeElement && body.contains(document.activeElement) ? document.activeElement.dataset.fid : null;
    body.innerHTML = html;
    this.wire(body);
    if (keep) { const el = body.querySelector(`[data-fid="${keep}"]`); if (el) el.focus(); else this.focusFirst(); }
  }

  // ---------- trade ----------
  openTrade() {
    this.panel = { kind: 'trade' };
    $('#tabs').innerHTML = '';
    $('#panel').hidden = false;
    this.g.input.setUIMode(true);
    this.renderTrade();
    this.focusFirst();
  }

  renderTrade() {
    const tr = this.g.me.trade, me = this.g.me;
    if (!tr) return;
    $('#paneltitle').textContent = `Trade with ${tr.with}`;
    let html;
    if (!tr.open) {
      html = tr.incoming
        ? `<p><b>${esc(tr.with)}</b> would like to trade with you.</p><div class="row"><button class="btn primary" data-do="tradeAccept" data-fid="ta">Accept</button><button class="btn" data-do="tradeDecline" data-fid="td">Decline</button></div>`
        : `<p>Waiting for <b>${esc(tr.with)}</b> to accept…</p><button class="btn" data-do="tradeCancel" data-fid="tc">Cancel</button>`;
    } else {
      const lines = (o, mine) => Object.entries(o || {}).map(([k, v]) => `<div class="line">${this.icon(k)} <span style="flex:1">${esc(this.item(k).name)} ×${v}</span>${mine ? `<button class="btn" data-do="offer" data-arg="${esc(k)}|-1" data-fid="m-${esc(k)}" aria-label="Remove one">−</button>` : ''}</div>`).join('') || '<span class="note">Nothing yet</span>';
      const mine = tr.mine || {};
      const addable = Object.keys(me.inv).filter((id) => { const it = this.g.items[id]; return it && it.tradeable && (me.inv[id] || 0) > (mine[id] || 0); });
      html = `<div class="trade"><div><h4>Your offer ${tr.myLock ? '<span class="locked">✓ locked</span>' : ''}</h4><div class="offer">${lines(mine, true)}</div>
          <h4>Add from your pack</h4><div class="grid">${addable.map((id) => `<button class="item" data-do="offer" data-arg="${esc(id)}|${id === 'coin' ? 5 : 1}" data-fid="a-${esc(id)}">${this.icon(id)}<span>${esc(this.item(id).name)}<small>+${id === 'coin' ? 5 : 1} (have ${me.inv[id]})</small></span></button>`).join('')}</div></div>
        <div><h4>${esc(tr.with)} offers ${tr.theirLock ? '<span class="locked">✓ locked</span>' : ''}</h4><div class="offer">${lines(tr.theirs, false)}</div>
          <p class="note">Changing an offer unlocks both sides. Nothing moves until you <b>both</b> lock and confirm — then the swap happens all at once, or not at all.</p>
          <div class="row">${!tr.myLock ? `<button class="btn primary" data-do="lock" data-fid="lk">Lock offer</button>` :
            tr.myLock && tr.theirLock && !tr.myConfirm ? `<button class="btn primary" data-do="confirm" data-fid="cf">Confirm trade</button>` :
              `<span class="note">${tr.myConfirm ? 'Waiting for ' + esc(tr.with) + ' to confirm…' : 'Waiting for ' + esc(tr.with) + ' to lock…'}</span>`}
          <button class="btn" data-do="tradeCancel" data-fid="tc">Cancel</button></div></div></div>`;
    }
    const body = $('#panelbody');
    const keep = document.activeElement && body.contains(document.activeElement) ? document.activeElement.dataset.fid : null;
    body.innerHTML = html;
    this.wire(body);
    if (keep) { const el = body.querySelector(`[data-fid="${CSS.escape(keep)}"]`); if (el) el.focus(); else this.focusFirst(); }
  }

  // ---------- gamepad / keyboard spatial navigation ----------
  focusables() {
    return [...document.querySelectorAll('#panel button, #panel input, #panel select, #panel [tabindex="0"]')].filter((e) => e.offsetParent !== null);
  }
  focusFirst() {
    const f = this.focusables().filter((e) => !e.classList.contains('close') && !e.classList.contains('tab'));
    (f[0] || this.focusables()[0])?.focus();
  }
  nav(dir) {
    if (!this.panel) return;
    if (dir === 'back') { this.close(); return; }
    if (dir === 'ok') { const a = document.activeElement; if (a && a.type === 'checkbox') { a.checked = !a.checked; a.dispatchEvent(new Event('change')); } else a?.click(); return; }
    if (dir === 'tabPrev' || dir === 'tabNext') {
      const tabs = [...document.querySelectorAll('#tabs .tab')];
      if (!tabs.length) return;
      const i = tabs.findIndex((t) => t.getAttribute('aria-selected') === 'true');
      tabs[(i + (dir === 'tabNext' ? 1 : -1) + tabs.length) % tabs.length].click();
      this.focusFirst();
      return;
    }
    const cur = document.activeElement;
    if (cur && (cur.type === 'range' || cur.tagName === 'SELECT') && (dir === 'left' || dir === 'right')) {
      if (cur.type === 'range') { cur.value = +cur.value + (dir === 'right' ? 1 : -1) * +cur.step; }
      else { cur.selectedIndex = Math.max(0, Math.min(cur.options.length - 1, cur.selectedIndex + (dir === 'right' ? 1 : -1))); }
      cur.dispatchEvent(new Event('change'));
      return;
    }
    const els = this.focusables();
    if (!cur || !els.includes(cur)) { this.focusFirst(); return; }
    const r = cur.getBoundingClientRect(), cx = r.left + r.width / 2, cy = r.top + r.height / 2;
    let best = null, bd = Infinity;
    for (const e of els) {
      if (e === cur) continue;
      const q = e.getBoundingClientRect(), ex = q.left + q.width / 2, ey = q.top + q.height / 2;
      const dx = ex - cx, dy = ey - cy;
      const ok = { up: dy < -4, down: dy > 4, left: dx < -4, right: dx > 4 }[dir];
      if (!ok) continue;
      const primary = dir === 'up' || dir === 'down' ? Math.abs(dy) : Math.abs(dx);
      const side = dir === 'up' || dir === 'down' ? Math.abs(dx) : Math.abs(dy);
      const d = primary + side * 2.5;
      if (d < bd) { bd = d; best = e; }
    }
    if (best) { best.focus(); best.scrollIntoView({ block: 'nearest' }); }
  }
}
