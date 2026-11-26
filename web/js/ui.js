import { settings, save, PRESETS, ACTIONS, DEFAULT_KEYS, quality } from './settings.js';
import { glyph } from './input.js';
import { itemIcon } from './icons.js';

const $ = (s) => document.querySelector(s);
export const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const KIND_LABEL = { weapon: 'Weapon', food: 'Food', block: 'Building block', material: 'Material', tool: 'Tool', currency: 'Currency', missing: 'Unknown item' };
const STATION_LABEL = { anvil: '⚒ at the Anvil', fire: '🔥 at a campfire or range' };

// ---------- extension points ----------
// Menu tabs, in order. To add a screen, push {id, label, render(ui) → html}.
// Buttons inside use data-do / data-arg (handled by UI.act) and data-fid
// (a stable id so focus survives re-renders).
export const MENU_TABS = [
  { id: 'pack', label: '🎒 Pack', render: (ui) => ui.tab_pack() },
  { id: 'map', label: '🗺 Map', render: (ui) => ui.g.worldMap.html(), after: (ui) => ui.g.worldMap.attach(), live: false },
  { id: 'craft', label: '🔨 Craft', render: (ui) => ui.tab_craft() },
  { id: 'skills', label: '⭐ Skills', render: (ui) => ui.tab_skills() },
  { id: 'quests', label: '📜 Quests', render: (ui) => ui.tab_quests() },
  { id: 'settings', label: '⚙ Settings', render: (ui) => ui.tab_settings() },
  { id: 'controls', label: '🎮 Controls', render: (ui) => ui.tab_controls() },
];

// Pack filters: {id, label, test(item)}. Empty filters are hidden.
export const PACK_FILTERS = [
  { id: 'all', label: 'All', test: () => true },
  { id: 'gear', label: 'Gear', test: (it) => it.kind === 'weapon' || it.kind === 'tool' },
  { id: 'food', label: 'Food', test: (it) => it.kind === 'food' },
  { id: 'materials', label: 'Materials', test: (it) => it.kind === 'material' || it.kind === 'missing' },
  { id: 'blocks', label: 'Blocks', test: (it) => it.kind === 'block' },
];

// What you can do with an item in your pack. Each action says when it
// applies and which request it sends; the server checks every rule again.
//   {id, label(item, ctx), when(item, ctx), primary?, request(id, ctx), toast?(item, ctx)}
//   ctx = {me, count, equipped}
const droppable = (it) => it.tradeable && it.kind !== 'missing';
export const ITEM_ACTIONS = [
  { id: 'equip', primary: true, label: () => 'Equip', when: (it, c) => it.kind === 'weapon' && !c.equipped, request: (id) => ({ t: 'equip', item: id }) },
  { id: 'unequip', label: () => 'Unequip', when: (it, c) => it.kind === 'weapon' && c.equipped, request: () => ({ t: 'equip', item: '' }) },
  { id: 'eat', primary: true, label: (it) => `Eat (+${it.heal} HP)`, when: (it) => it.kind === 'food', request: (id) => ({ t: 'eat', item: id }) },
  { id: 'drop1', label: (it, c) => (c.count > 1 ? 'Drop 1' : 'Drop'), when: droppable, request: (id) => ({ t: 'drop', item: id, count: 1 }), toast: (it) => `Dropped ${it.name}.` },
  { id: 'dropAll', label: (it, c) => `Drop all ${c.count}`, when: (it, c) => droppable(it) && c.count > 1, request: (id, c) => ({ t: 'drop', item: id, count: c.count }), toast: (it, c) => `Dropped ${c.count}× ${it.name}.` },
];

// Short facts about an item, shared by the pack, crafting and shop screens.
export function itemFacts(it) {
  const f = [];
  if (it.kind === 'weapon') f.push(`⚔ ${it.damage} damage`, `⏱ ${(it.cooldownMs / 1000).toFixed(2)}s per swing`);
  if (it.kind === 'food') f.push(`❤ restores ${it.heal} health`);
  if (it.light) f.push('🏮 lights your way in the Gloam while you carry it');
  if (it.kind === 'block') f.push('🧱 pick it on the hotbar to place it');
  if (it.burn) f.push(`🔥 burns for ${it.burn.sec}s on a campfire (Firemaking ${it.burn.level}); pick it on the hotbar and use it on the ground`);
  if (it.kindle) f.push('🔥 lights campfires while you carry it');
  if (it.trap) f.push(`🪤 set it on open ground in the countryside (Hunting ${it.trap.level}); it catches something in about ${it.trap.sec}s`);
  if (it.crop) f.push(`🌱 plant it in allotment soil (Farming ${it.crop.level}); ready in about ${Math.round(it.crop.sec / 60)} min`);
  if (it.value > 0) f.push(`🪙 sells for ${it.value}`);
  if (it.kind !== 'missing' && !it.tradeable) f.push('🔒 can’t be traded or dropped');
  if (it.stack > 1 && it.kind !== 'currency') f.push(`stacks to ${it.stack}`);
  if (it.kind === 'missing') f.push('From a mod that is turned off. It’s kept safe and works again if the mod returns.');
  return f;
}

export function applyAccessibility() {
  document.documentElement.style.setProperty('--ui-scale', settings.textScale);
  document.body.classList.toggle('reduce-motion', !!settings.reduceMotion);
  // Background blur costs GPU time over the 3D scene: use solid panels when asked, or on the Low preset.
  document.body.classList.toggle('no-glass', !!settings.reduceTransparency || quality().name === 'low');
  $('#perf').hidden = !settings.showPerf;
  document.body.classList.toggle('no-minimap', settings.showMinimap === false);
}

export class UI {
  constructor(game) {
    this.g = game;
    this.panel = null; // {kind, tab}
    this.dialog = null;
    this.modal = null; // open confirm dialog
    this.hot = 0;
    this.lastTab = 'pack';
    this.pack = { sel: null, filter: 'all' };
    this.craftOnly = false;
    $('#panel').addEventListener('click', (e) => {
      const b = e.target.closest('[data-close]');
      if (b) this.close();
    });
    $('#modalback').addEventListener('click', () => this.closeModal(false));
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

  item(id) { return this.g.items[id] || { id, name: id + ' (missing mod)', color: '#666666', kind: 'missing' }; }

  icon(id) { return itemIcon(this.item(id)); }

  // Send a request; show the server's reason if it says no.
  async req(msg) {
    const r = await this.g.net.request(msg);
    if (r && !r.ok && r.err) this.toast(r.err, 'warn');
    return r || {};
  }

  // Active charm effects as small chips by the health bar, counting down.
  renderFx() {
    const el = $('#fx');
    if (!el) return;
    const now = performance.now(), parts = [];
    for (const [k, end] of Object.entries(this.fxEnds || {})) {
      const s = Math.ceil((end - now) / 1000);
      if (s <= 0) continue;
      const label = k === 'ward' ? '🌿 Warded' : k.startsWith('bane:') ? `🧂 Bane of ${esc((this.g.mobDefs[k.slice(5)] || { name: k.slice(5) }).name)}s` : esc(k);
      parts.push(`<span class="fxchip">${label} ${s}s</span>`);
    }
    const html = parts.join('');
    if (el.innerHTML !== html) el.innerHTML = html;
    clearTimeout(this.fxTimer);
    if (parts.length) this.fxTimer = setTimeout(() => this.renderFx(), 500);
  }

  updateHUD(me) {
    const g = this.g;
    $('#hpfill').style.width = `${Math.max(0, me.hp / me.mhp * 100)}%`;
    $('#hptext').textContent = `${me.hp}/${me.mhp}`;
    this.fxEnds = Object.fromEntries(Object.entries(me.fx || {}).map(([k, s]) => [k, performance.now() + s * 1000]));
    this.renderFx();
    const z = $('#zone');
    // Zone kinds come from the server; names come from the world's map files.
    const cap = (s) => s.charAt(0).toUpperCase() + s.slice(1);
    const wild = `${me.sea ? '🌊' : '🌿'} ${cap(me.zoneName || 'the countryside')}`; // e.g. "The European countryside"
    const zn = me.zone === 'town' ? `🏙 ${me.zoneName} — safe` : me.zone === 'danger' ? `⚠ DANGER · ${me.zoneName}` : wild;
    z.textContent = zn;
    z.className = me.zone === 'danger' ? 'gloam' : '';
    $('#vignette').classList.toggle('on', me.zone === 'danger' && !settings.reduceMotion);
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
    if (!html) {
      // No active quest: point at whoever offers the next one you can take.
      const next = Object.values(g.quests).find((q) => !me.quests[q.id] && (!q.requires || me.quests[q.requires]?.status === 'done'));
      const giver = next && g.npcs[next.giver];
      if (giver) html = `<h4>${Object.keys(me.quests).length ? 'New quest' : 'Welcome!'}</h4>Find <b>${esc(giver.name)}</b> and press ${esc(glyph(g.input.device, 'interact'))} to talk.`;
    }
    if (me.bounty) {
      const b = me.bounty, m = g.mobDefs[b.mob];
      html += `<h4>📜 Warrant</h4>${b.left > 0 ? `See off ${b.left} more ${esc(m ? m.name : b.mob)}${b.left > 1 ? 's' : ''} (${b.total - b.left}/${b.total})` : `Done! Report to ${esc(g.npcs[b.giver]?.name || 'the magistrate')}.`}`;
    }
    tr.innerHTML = html;
    // Hearth progress.
    const hb = $('#hearthbar');
    hb.hidden = !(me.hearth > 0);
    $('#hearthfill').style.width = `${Math.min(1, me.hearth) * 100}%`;
    this.renderHotbar(me);
    // Tabs re-render with fresh stats, except ones that keep their own state (the map).
    if (this.panel && this.panel.kind === 'menu') {
      const tab = MENU_TABS.find((t) => t.id === this.panel.tab);
      if (tab && tab.live === false) this.g.worldMap.draw(); else this.renderPanel();
    }
    if (this.panel && this.panel.kind === 'trade') this.renderTrade();
    if (this.panel && this.panel.kind === 'dialog' && this.dialog) this.renderDialog();
    if (me.trade && !(this.panel && this.panel.kind === 'trade')) this.openTrade();
    if (!me.trade && this.panel && this.panel.kind === 'trade') this.close();
  }

  // The hotbar holds blocks, food, fuel, and the charms you know ("charm:<id>").
  hotItems(me) {
    const items = Object.keys(me.inv).filter((id) => { const k = this.item(id).kind; const it = this.item(id); return k === 'block' || k === 'food' || !!it.burn || !!it.trap || !!it.crop; }).sort();
    const magic = (me.skills.magic || { lvl: 1 }).lvl;
    return items.concat((this.g.charms || []).filter((c) => magic >= c.level).map((c) => 'charm:' + c.id));
  }

  renderHotbar(me) {
    const items = this.hotItems(me);
    if (this.hot >= items.length) this.hot = Math.max(0, items.length - 1);
    this.hotSel = items[this.hot] || null;
    $('#hotbar').innerHTML = items.slice(0, 8).map((id, i) =>
      this.slotHTML(me, id, i)).join('');
    for (const b of document.querySelectorAll('#hotbar .slot')) b.onclick = () => { this.hot = +b.dataset.hot; this.renderHotbar(this.g.me); };
  }

  slotHTML(me, id, i) {
    const sel = i === this.hot ? 'sel' : '';
    const ch = id.startsWith('charm:') && (this.g.charms || []).find((c) => 'charm:' + c.id === id);
    if (ch) {
      const can = Object.entries(ch.reagents).every(([k, v]) => (me.inv[k] || 0) >= v);
      const tip = `${ch.name}: ${ch.desc} Uses ${Object.entries(ch.reagents).map(([k, v]) => v + ' ' + this.item(k).name).join(', ')}.`;
      return `<button class="slot charm ${sel} ${can ? '' : 'dim'}" data-hot="${i}" title="${esc(tip)}" tabindex="-1"><span class="k">${i + 1}</span><span class="charmic">${esc(ch.icon)}</span></button>`;
    }
    return `<button class="slot ${sel}" data-hot="${i}" title="${esc(this.item(id).name)}" tabindex="-1"><span class="k">${i + 1}</span>${this.icon(id)}<span class="n">${me.inv[id]}</span></button>`;
  }

  cycleHot(d) {
    const n = this.hotItems(this.g.me).length;
    if (n) { this.hot = (this.hot + d + n) % n; this.renderHotbar(this.g.me); }
  }

  // Drop one of the selected hotbar item (the in-game drop button).
  async dropHot() {
    const id = this.hotSel;
    if (!id) { this.toast('Select something on the hotbar to drop it, or drop items from your pack.', 'info'); return; }
    if (id.startsWith('charm:')) { this.toast('Charms can\'t be dropped. Pick an item on the hotbar first.', 'info'); return; }
    const r = await this.req({ t: 'drop', item: id, count: 1 });
    if (r.ok) this.toast(`Dropped ${this.item(id).name}.`, 'info');
  }

  // ---------- panels ----------
  get open() { return !!this.panel || !!this.modal; }

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

  openPanel(kind) {
    this.panel = { kind };
    $('#tabs').innerHTML = '';
    $('#panel').hidden = false;
    this.g.input.setUIMode(true);
  }

  close() {
    if (this.modal) { this.closeModal(false); return; }
    if (this.panel && this.panel.kind === 'trade' && this.g.me.trade) {
      this.g.net.request({ t: 'trade_cancel', trade: this.g.me.trade.id });
    }
    this.panel = null; this.dialog = null;
    $('#panel').hidden = true;
    this.g.input.setUIMode(false);
  }

  // Opens the menu on `tab`, or on the tab you last used.
  openMenu(tab) {
    tab = MENU_TABS.some((t) => t.id === tab) ? tab : this.lastTab;
    this.lastTab = tab;
    this.show('menu', 'Adventurer’s Pack', MENU_TABS.map((t) => [t.id, t.label]), tab);
  }

  // Button hints along the bottom of the panel, for the device in use.
  footHints() {
    const d = this.g.input.device === 'pad' ? 'pad' : this.g.input.device === 'touch' ? null : 'kb';
    const el = $('#panelfoot');
    if (!d) { el.textContent = ''; el.hidden = true; return; }
    el.hidden = false;
    const p = this.panel || {};
    const h = d === 'pad' ? ['Ⓐ choose', 'Ⓑ close'] : ['Enter choose', 'Esc close'];
    if (p.kind === 'menu') h.push(d === 'pad' ? 'LB/RB switch tabs' : 'Q/R switch tabs');
    if (p.kind === 'menu' && p.tab === 'pack') h.push(`${glyph(d, 'alt')} drop one`, d === 'pad' ? 'Ⓐ twice to use' : 'double-click to use');
    el.textContent = h.join(' · ');
  }

  renderPanel() {
    if (!this.panel) return;
    if (this.panel.kind === 'dialog') return this.renderDialog();
    if (this.panel.kind === 'trade') return this.renderTrade();
    this.lastTab = this.panel.tab;
    const tab = MENU_TABS.find((t) => t.id === this.panel.tab) || MENU_TABS[0];
    this.setBody(tab.render(this));
    if (tab.after) tab.after(this);
  }

  // Replace the panel body, keeping scroll position and the focused control.
  setBody(html) {
    const body = $('#panelbody');
    const keep = document.activeElement && body.contains(document.activeElement) ? document.activeElement.dataset.fid : null;
    const top = body.scrollTop;
    body.innerHTML = html;
    body.scrollTop = top;
    this.wire(body);
    this.footHints();
    if (keep) { const el = body.querySelector(`[data-fid="${CSS.escape(keep)}"]`); if (el) el.focus({ preventScroll: true }); else this.focusFirst(); }
  }

  wire(root) {
    for (const el of root.querySelectorAll('[data-do]')) {
      if (el.type === 'checkbox') el.onchange = () => this.act(el.dataset.do, el.dataset.arg, el);
      else el.onclick = () => this.act(el.dataset.do, el.dataset.arg, el);
    }
    for (const el of root.querySelectorAll('[data-set]')) {
      el.onchange = () => this.setSetting(el.dataset.set, el);
    }
    for (const el of root.querySelectorAll('[data-dbl]')) {
      el.ondblclick = () => this.act(el.dataset.dbl, el.dataset.arg, el);
    }
  }

  async act(what, arg, el) {
    const g = this.g, n = g.net;
    let r;
    switch (what) {
      case 'select':
        // With a controller, Ⓐ on the selected tile uses it (Ⓐ elsewhere selects).
        if (this.pack.sel === arg && g.input.device === 'pad') return this.act('primary', arg);
        this.pack.sel = arg; this.renderPanel(); return;
      case 'filter': this.pack.filter = arg; this.pack.sel = null; this.renderPanel(); return;
      case 'primary': {
        const a = this.itemActions(arg).find((x) => x.primary);
        if (a) await this.runItemAction(a.id, arg);
        return;
      }
      case 'item': { const [aid, id] = arg.split('|'); return this.runItemAction(aid, id); }
      case 'craftOnly': this.craftOnly = !this.craftOnly; this.renderPanel(); return;
      case 'mapZoom': g.worldMap.setZoom(g.worldMap.zoom * (+arg > 0 ? 1.5 : 1 / 1.5)); return;
      case 'mapCenter': g.worldMap.center = null; g.worldMap.draw(); return;
      case 'craft': {
        r = await n.request({ t: 'craft', recipe: arg });
        const rec = g.recipes.find((x) => x.id === arg);
        if (r.ok && rec) this.toast(`Made ${rec.count > 1 ? rec.count + '× ' : ''}${this.item(rec.output).name}.`, 'good');
        break;
      }
      case 'steal': r = await n.request({ t: 'steal', npc: this.dialog.npc }); if (r.ok) this.renderDialog(); break;
      case 'bountyTake': r = await n.request({ t: 'bounty_take', npc: this.dialog.npc }); if (r.ok) this.talk(this.dialog.npc); break;
      case 'bountyClaim': r = await n.request({ t: 'bounty_claim', npc: this.dialog.npc }); if (r.ok) this.talk(this.dialog.npc); break;
      case 'accept': r = await n.request({ t: 'quest_accept', quest: arg }); if (r.ok) this.talk(this.dialog.npc); break;
      case 'turnin': r = await n.request({ t: 'quest_turnin', quest: arg }); if (r.ok) this.talk(this.dialog.npc); break;
      case 'buy': r = await n.request({ t: 'buy', item: arg, count: 1 }); break;
      case 'sell': {
        const it = this.item(arg), cnt = +el.dataset.n || 1;
        if ((it.kind === 'weapon' || it.kind === 'tool') && !await this.confirm({ title: `Sell ${it.name}?`, text: `You’ll get ${it.value * cnt} coins.`, ok: 'Sell' })) return;
        r = await n.request({ t: 'sell', item: arg, count: cnt });
        break;
      }
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
      case 'leave':
        if (!await this.confirm({ title: 'Leave this world?', text: g.browserWorld ? 'Your world is saved before you go.' : 'Your character is saved by the world.', ok: 'Leave' })) return;
        if (g.browserWorld && n.flush) await n.flush();
        location.reload();
        return;
      case 'exportWorld': {
        const bytes = await n.exportWorld();
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
          if (!await this.confirm({ title: 'Replace this world?', text: 'The imported world replaces the one in this browser. Your current world is kept as a backup here.', ok: 'Import', danger: true })) return;
          const res = await n.importWorld(new Uint8Array(await f.arrayBuffer()));
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

  // ---------- item actions ----------
  itemCtx(id) { const me = this.g.me; return { me, count: me.inv[id] || 0, equipped: me.eq === id }; }

  itemActions(id) {
    const it = this.item(id), c = this.itemCtx(id);
    return ITEM_ACTIONS.filter((a) => a.when(it, c));
  }

  async runItemAction(aid, id) {
    const a = ITEM_ACTIONS.find((x) => x.id === aid);
    const it = this.item(id), c = this.itemCtx(id);
    if (!a || !a.when(it, c)) return;
    const r = await this.req(a.request(id, c));
    if (r.ok && a.toast) this.toast(a.toast(it, c), 'info');
  }

  // The secondary button (Ⓧ / drop key) while a panel is open.
  alt() {
    if (this.modal || !this.panel || this.panel.kind !== 'menu' || this.panel.tab !== 'pack') return;
    const f = document.activeElement;
    const id = f && f.classList.contains('tile') ? f.dataset.arg : this.pack.sel;
    if (id) this.runItemAction('drop1', id);
  }

  // ---------- confirm dialog (controller, keyboard and touch friendly) ----------
  confirm({ title, text = '', ok = 'OK', cancel = 'Cancel', danger = false }) {
    if (this.modal) this.closeModal(false);
    const wasUI = this.g.input.uiMode;
    this.g.input.setUIMode(true);
    const m = $('#modal');
    m.innerHTML = `<h3 id="modaltitle">${esc(title)}</h3>${text ? `<p>${esc(text)}</p>` : ''}
      <div class="row end"><button class="btn" data-m="0">${esc(cancel)}</button><button class="btn ${danger ? 'danger' : 'primary'}" data-m="1">${esc(ok)}</button></div>`;
    m.hidden = false;
    $('#modalback').hidden = false;
    return new Promise((resolve) => {
      this.modal = { resolve, wasUI, back: document.activeElement, backFid: document.activeElement && document.activeElement.dataset.fid };
      for (const b of m.querySelectorAll('[data-m]')) b.onclick = () => this.closeModal(b.dataset.m === '1');
      m.querySelector('[data-m="1"]').focus();
    });
  }

  closeModal(ok) {
    const md = this.modal;
    if (!md) return;
    this.modal = null;
    $('#modal').hidden = true;
    $('#modalback').hidden = true;
    if (!md.wasUI && !this.panel) this.g.input.setUIMode(false);
    const again = md.backFid && document.querySelector(`#panel [data-fid="${CSS.escape(md.backFid)}"]`);
    if (again) again.focus(); else if (md.back && document.body.contains(md.back)) md.back.focus();
    md.resolve(ok);
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
      if (code !== 'Escape') {
        // Keep bindings unique: an action already on this key takes this action's old key.
        const prev = Object.keys(settings.keys).find((a) => a !== action && settings.keys[a] === code);
        if (prev) settings.keys[prev] = settings.keys[action];
        settings.keys[action] = code;
        save();
        if (prev) this.toast(`${ACTIONS[prev] || prev} moved to ${glyph('kb', prev)}.`, 'info');
      }
      this.renderPanel();
    };
  }

  // ---------- menu tabs ----------
  tab_pack() {
    const me = this.g.me;
    const all = Object.keys(me.inv).filter((i) => i !== 'coin' && me.inv[i] > 0)
      .sort((a, b) => (me.eq === b) - (me.eq === a) || this.item(a).kind.localeCompare(this.item(b).kind) || this.item(a).name.localeCompare(this.item(b).name));
    const filters = PACK_FILTERS.filter((f) => f.id === 'all' || all.some((id) => f.test(this.item(id))));
    if (!filters.some((f) => f.id === this.pack.filter)) this.pack.filter = 'all';
    const filter = PACK_FILTERS.find((f) => f.id === this.pack.filter);
    const ids = all.filter((id) => filter.test(this.item(id)));
    if (!ids.includes(this.pack.sel)) this.pack.sel = ids[0] || null;
    const used = me.slots, maxS = me.maxSlots;
    const chips = filters.length > 2 ? `<div class="chips" role="radiogroup" aria-label="Show">${filters.map((f) =>
      `<button class="chip" role="radio" aria-checked="${f.id === this.pack.filter}" data-do="filter" data-arg="${f.id}" data-fid="f-${f.id}">${f.label}</button>`).join('')}</div>` : '';
    const tiles = ids.map((id) => {
      const it = this.item(id), eq = me.eq === id;
      return `<button class="tile ${id === this.pack.sel ? 'sel' : ''} ${eq ? 'eq' : ''}" data-do="select" data-dbl="primary" data-arg="${esc(id)}" data-fid="i-${esc(id)}"
        aria-label="${esc(it.name)}, ${me.inv[id]}${eq ? ', equipped' : ''}" aria-pressed="${id === this.pack.sel}" title="${esc(it.name)}">${itemIcon(it)}${me.inv[id] > 1 ? `<span class="n">${me.inv[id]}</span>` : ''}${eq ? '<span class="eqmark" aria-hidden="true">E</span>' : ''}</button>`;
    }).join('');
    return `<div class="packhead">
        <span class="coins">${this.icon('coin')}<b>${me.inv.coin || 0}</b> coins</span>
        <span class="slots"><span class="meter" aria-hidden="true"><span class="${used >= maxS ? 'full' : ''}" style="width:${Math.min(100, used / maxS * 100)}%"></span></span>${used}/${maxS} slots${used >= maxS ? ' · <b class="warn">full</b>' : ''}</span>
        <span class="grow"></span><button class="btn" data-do="hearth" data-fid="hearth">🏮 Hearth home</button></div>
      ${chips}
      ${all.length ? `<div class="pack"><div class="tiles" aria-label="Items">${tiles}</div>${this.itemDetails(this.pack.sel)}</div>`
        : '<p class="note">Your pack is empty. Try chopping a tree out in the countryside!</p>'}`;
  }

  itemDetails(id) {
    if (!id) return '<div class="details"></div>';
    const it = this.item(id), c = this.itemCtx(id);
    const facts = itemFacts(it);
    const acts = this.itemActions(id);
    return `<div class="details">
      <div class="dhead">${itemIcon(it)}<div><h3>${esc(it.name)}${c.count > 1 ? ` <span class="note">×${c.count}</span>` : ''}</h3>
        <span class="kind">${esc(KIND_LABEL[it.kind] || it.kind)}${c.equipped ? ' · <b class="good">Equipped</b>' : ''}</span></div></div>
      ${it.desc ? `<p class="desc">${esc(it.desc)}</p>` : ''}
      ${facts.length ? `<ul class="facts">${facts.map((f) => `<li>${esc(f)}</li>`).join('')}</ul>` : ''}
      ${acts.length ? `<div class="actions">${acts.map((a) => `<button class="btn ${a.primary ? 'primary' : ''}" data-do="item" data-arg="${a.id}|${esc(id)}" data-fid="a-${a.id}">${esc(a.label(it, c))}</button>`).join('')}</div>` : ''}
      ${acts.some((a) => a.id === 'drop1') ? '<p class="note">Dropped items stay on the ground for 10 minutes. Walk over them to pick them back up.</p>' : ''}
    </div>`;
  }

  tab_craft() {
    const me = this.g.me, lvl = (s) => (me.skills[s] || { lvl: 1 }).lvl;
    const rows = this.g.recipes.map((r) => {
      const missing = Object.entries(r.inputs).some(([k, v]) => (me.inv[k] || 0) < v);
      const sk = r.skill || 'crafting', needs = Object.entries(r.needs || {});
      const lvOK = lvl(sk) >= r.level && needs.every(([k, v]) => lvl(k) >= v), stOK = !r.station || this.g.nearStation(r.station);
      return { r, can: !missing && lvOK && stOK, lvOK, stOK };
    }).sort((a, b) => b.can - a.can || a.r.level - b.r.level);
    const shown = this.craftOnly ? rows.filter((x) => x.can) : rows;
    const list = shown.map(({ r, can, lvOK, stOK }) => {
      const out = this.item(r.output);
      const req = Object.entries(r.inputs).map(([k, v]) => `<span class="${(me.inv[k] || 0) < v ? 'miss' : ''}">${this.icon(k)}${v}× ${esc(this.item(k).name)} <small>(have ${me.inv[k] || 0})</small></span>`).join('');
      const st = r.station ? `<span class="${stOK ? '' : 'miss'}">${STATION_LABEL[r.station] || esc(r.station)}</span>` : '';
      const burn = r.safeAt && lvl(sk) < r.safeAt ? `<span class="miss">may burn until ${esc(this.skillName(sk))} ${r.safeAt}</span>` : '';
      const fact = itemFacts(out)[0];
      return `<div class="recipe ${can ? 'can' : ''}">${itemIcon(out)}<div class="info"><b>${esc(out.name)}${r.count > 1 ? ' ×' + r.count : ''}</b>${fact ? ` <span class="note">· ${esc(fact)}</span>` : ''}
        <div class="req">${req}${st}${burn}<span class="${lvl(r.skill || 'crafting') >= r.level ? '' : 'miss'}">${esc(this.skillName(r.skill || 'crafting'))} ${r.level}</span>${Object.entries(r.needs || {}).map(([k, v]) => `<span class="${lvl(k) >= v ? '' : 'miss'}">${esc(this.skillName(k))} ${v}</span>`).join('')}<span>+${r.xp} ${esc(this.skillName(r.skill || 'crafting'))} XP</span></div></div>
        <button class="btn ${can ? 'primary' : ''}" data-do="craft" data-arg="${esc(r.id)}" data-fid="c-${esc(r.id)}" ${can ? '' : 'aria-disabled="true"'}>${sk === 'cooking' ? 'Cook' : 'Craft'}</button></div>`;
    }).join('');
    return `<div class="row"><span class="note">Crafting level ${lvl('crafting')}</span><span class="grow"></span>
      <label class="toggle"><input type="checkbox" ${this.craftOnly ? 'checked' : ''} data-do="craftOnly" data-fid="co"> Only what I can make now</label></div>
      ${list || '<p class="note">Nothing you can make right now. Gather more materials, or untick the filter to see every recipe.</p>'}`;
  }

  // Skills come from content: grouped, with progress toward the next level,
  // the world's cap, and what the next level unlocks.
  skillName(id) { const s = (this.g.skills || []).find((k) => k.id === id); return s ? s.name : id; }

  nextUnlock(skill, lvl) {
    const g = this.g, found = [];
    const add = (level, name) => { if (level > lvl) found.push([level, name]); };
    for (const r of g.recipes || []) { if ((r.skill || 'crafting') === skill) add(r.level, this.item(r.output).name); if (r.needs && r.needs[skill]) add(r.needs[skill], this.item(r.output).name); }
    for (const n of Object.values(g.nodes || {})) { if (n.skill === skill) add(n.level, n.name); if (n.needs && n.needs[skill]) add(n.needs[skill], n.name); }
    for (const it of Object.values(g.items || {})) if (it.needs && it.needs[skill]) add(it.needs[skill], it.name);
    for (const q of Object.values(g.quests || {})) if (q.needs && q.needs[skill]) add(q.needs[skill], q.name);
    if (!found.length) return '';
    const next = Math.min(...found.map((f) => f[0]));
    return `Level ${next}: ${[...new Set(found.filter((f) => f[0] === next).map((f) => f[1]))].join(', ')}`;
  }

  tab_skills() {
    const me = this.g.me, cap = me.cap || 10;
    const groups = { combat: '⚔ Combat', gathering: '🌿 Gathering', production: '🔨 Production', support: '🧭 Support' };
    const total = Object.values(me.skills).reduce((a, v) => a + v.lvl, 0);
    let html = `<div class="row"><b>Combat level ${me.combat || 1}</b><span class="note">· total level ${total} · skills go up to <b>${cap}</b> in this world; the next world raises the cap</span></div>`;
    for (const [gid, gname] of Object.entries(groups)) {
      const list = (this.g.skills || []).filter((s) => s.group === gid);
      if (!list.length) continue;
      html += `<h4 class="skillgroup">${gname}</h4><div class="skills">`;
      for (const s of list) {
        const v = me.skills[s.id] || { xp: 0, lvl: 1, cur: 0, next: 50 };
        const atCap = v.lvl >= cap;
        const pct = atCap ? 100 : ((v.xp - v.cur) / Math.max(1, v.next - v.cur)) * 100;
        const unlock = atCap ? '' : this.nextUnlock(s.id, v.lvl);
        html += `<div class="skill ${atCap ? 'capped' : ''}" tabindex="0" data-fid="s-${esc(s.id)}" title="${esc(s.desc)}">
          <div class="srow"><span class="sicon">${esc(s.icon)}</span><b>${esc(s.name)}</b><span class="grow"></span><span class="slvl">${v.lvl}<small>/${cap}</small></span></div>
          <div class="xp"><div style="width:${pct}%"></div></div>
          <div class="note">${atCap ? 'At this world’s cap' : `${v.next - v.xp} XP to level ${v.lvl + 1}`}${unlock ? ` · next: ${esc(unlock)}` : ''}</div></div>`;
      }
      html += '</div>';
    }
    return html;
  }

  // A small "+18 Mining" that floats up near the top of the screen.
  xpPopup(m) {
    const s = (this.g.skills || []).find((k) => k.id === m.skill);
    const el = document.createElement('div');
    el.className = 'xpdrop';
    el.textContent = `${s ? s.icon + ' ' : ''}+${m.amount} ${s ? s.name : m.skill}`;
    const box = $('#xpdrops');
    box.appendChild(el);
    while (box.children.length > 5) box.firstChild.remove();
    setTimeout(() => el.remove(), 1800);
  }
  tab_quests() {
    const me = this.g.me;
    const order = (q) => { const st = me.quests[q.id]; return !st ? 1 : st.status === 'done' ? 2 : 0; };
    return Object.values(this.g.quests).sort((a, b) => order(a) - order(b)).map((q) => {
      const st = me.quests[q.id];
      const status = !st ? 'Not started' : st.status === 'done' ? '✓ Complete' : 'In progress';
      const steps = q.steps.map((s, i) => {
        const have = st ? (s.kind === 'deliver' ? Math.min(s.count, me.inv[s.target] || 0) : st.progress[i] || 0) : 0;
        return `<li class="${have >= s.count ? 'done' : ''}">${esc(s.text)} ${st ? `(${have}/${s.count})` : ''}</li>`;
      }).join('');
      const giver = this.g.npcs[q.giver];
      return `<div class="quest ${st ? st.status : 'new'}" tabindex="0" data-fid="q-${esc(q.id)}"><h4>${esc(q.name)} <span class="note">— ${status}</span></h4>
        <div class="note">From ${esc(giver ? giver.name : q.giver)}${q.requires ? ' · after ' + esc(this.g.quests[q.requires]?.name) : ''}</div>${st && st.status === 'done' ? '' : `<ul>${steps}</ul>`}
        <div class="note">Reward: ${q.reward.coins} coins${Object.entries(q.reward.items || {}).map(([k, v]) => `, ${v}× ${esc(this.item(k).name)}`).join('')}${Object.entries(q.reward.xp || {}).map(([k, v]) => `, ${v} ${esc(this.skillName(k))} XP`).join('')}</div></div>`;
    }).join('');
  }

  tab_settings() {
    const q = quality();
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
      <label>Show minimap <input type="checkbox" ${settings.showMinimap !== false ? 'checked' : ''} data-set="showMinimap" data-fid="mm"></label>
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
      <p class="note">Left stick move · Right stick look · Ⓐ jump · RT attack/gather · LT use/place · Ⓧ talk/trade · Ⓨ menu · LB/RB hotbar · D-pad → map · D-pad ↓ Hearth · D-pad ← drop hotbar item · L3 sprint · Start settings · Back chat.
        In menus: Ⓐ choose · Ⓑ back · Ⓧ drop one · LB/RB switch tabs.</p>
      <h4>Touch</h4><p class="note">Left stick to move, drag the right side to look. ⚔ attack/gather (auto-targets the nearest creature), ✋ use/place, 💬 talk/trade. Drop items from the ☰ menu’s Pack.</p>
      <h4>Keyboard &amp; mouse (click to remap)</h4>${rows}<button class="btn" data-do="resetKeys" data-fid="rk">Reset to defaults</button></div>`;
  }

  // ---------- NPC dialog ----------
  async talk(npc) {
    const r = await this.g.net.request({ t: 'talk', npc });
    if (!r.ok) this.toast(r.err, 'warn');
  }

  showDialog(d) {
    // Each new conversation with an NPC moves on to their next line.
    const fresh = !this.panel || this.panel.kind !== 'dialog' || !this.dialog || this.dialog.npc !== d.npc;
    this.talks = this.talks || {};
    if (fresh) this.talks[d.npc] = (this.talks[d.npc] ?? -1) + 1;
    d.line = (d.lines || [])[(this.talks[d.npc] || 0) % Math.max(1, (d.lines || []).length)];
    this.dialog = d;
    if (!this.panel || this.panel.kind !== 'dialog') {
      this.openPanel('dialog');
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
    if (!spoke) html += `<p>“${esc(d.line || 'Hello!')}”</p>`;
    if (d.bounties) {
      const b = me.bounty, mine = b && b.giver === d.npc;
      html += `<h4>📜 Warrants</h4>`;
      if (mine && b.left === 0) html += `<button class="btn primary" data-do="bountyClaim" data-fid="bc">Report back: claim your reward</button>`;
      else if (b) html += `<p class="note">Your warrant: ${b.left} ${esc(this.g.mobDefs[b.mob]?.name || b.mob)} left. Slaying ${(me.skills.slaying || { lvl: 1 }).lvl}.</p>`;
      else html += `<button class="btn" data-do="bountyTake" data-fid="bt">Ask for a warrant (Slaying ${(me.skills.slaying || { lvl: 1 }).lvl})</button>`;
    }
    if (d.pocket) {
      const lvl = (me.skills.thieving || { lvl: 1 }).lvl, can = lvl >= d.pocket;
      html += `<button class="btn ${can ? '' : 'dim'}" data-do="steal" data-fid="st" title="Getting caught means a fine and a few seconds seeing stars">🎩 Pick their pocket (Thieving ${d.pocket}${can ? '' : ', you have ' + lvl})</button>`;
    }
    if (d.shop && d.shop.length) {
      html += `<h4>Buy <span class="note">· you have ${me.inv.coin || 0} coins</span></h4><div class="grid">` + d.shop.map((s) => {
        const it = this.item(s.item), afford = (me.inv.coin || 0) >= s.price;
        return `<button class="item" data-do="buy" data-arg="${esc(s.item)}" data-fid="b-${esc(s.item)}" ${afford ? '' : 'aria-disabled="true"'}>${itemIcon(it)}<span><b>${esc(it.name)}</b><small class="${afford ? '' : 'miss'}">${s.price} coins</small><small>${esc(itemFacts(it)[0] || it.desc || '')}</small></span></button>`;
      }).join('') + '</div>';
      const sellable = Object.keys(me.inv).filter((id) => { const it = this.g.items[id]; return it && it.value > 0 && it.tradeable && id !== 'coin'; }).sort();
      if (sellable.length) html += `<h4>Sell</h4><div class="grid">` + sellable.map((id) => `<button class="item ${me.eq === id ? 'eq' : ''}" data-do="sell" data-arg="${esc(id)}" data-n="${me.inv[id]}" data-fid="s-${esc(id)}">${this.icon(id)}<span><b>${esc(this.item(id).name)} ×${me.inv[id]}</b><small>${this.item(id).value} coin each · sell all for ${this.item(id).value * me.inv[id]}</small></span></button>`).join('') + '</div>';
    }
    html += `</div>`;
    this.setBody(html);
  }

  // ---------- trade ----------
  openTrade() {
    this.openPanel('trade');
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
    this.setBody(html);
  }

  // ---------- gamepad / keyboard spatial navigation ----------
  root() { return this.modal ? $('#modal') : $('#panel'); }
  focusables() {
    return [...this.root().querySelectorAll('button, input, select, [tabindex="0"]')].filter((e) => e.offsetParent !== null && !e.disabled);
  }
  focusFirst() {
    const f = this.focusables().filter((e) => !e.classList.contains('close') && !e.classList.contains('tab'));
    const pref = f.find((e) => e.classList.contains('sel'));
    (pref || f[0] || this.focusables()[0])?.focus();
  }
  nav(dir) {
    if (this.modal) {
      if (dir === 'back') { this.closeModal(false); return; }
      if (dir === 'tabPrev' || dir === 'tabNext' || dir === 'alt') return;
    }
    if (!this.panel && !this.modal) return;
    if (dir === 'back') { this.close(); return; }
    if (dir === 'alt') { this.alt(); return; }
    if (dir === 'ok') { const a = document.activeElement; if (a && a.type === 'checkbox') { a.checked = !a.checked; a.dispatchEvent(new Event('change')); } else a?.click(); return; }
    if (dir === 'tabPrev' || dir === 'tabNext') {
      const tabs = [...document.querySelectorAll('#tabs .tab')];
      if (!tabs.length) return;
      const i = tabs.findIndex((t) => t.getAttribute('aria-selected') === 'true');
      tabs[(i + (dir === 'tabNext' ? 1 : -1) + tabs.length) % tabs.length].click();
      this.focusFirst();
      return;
    }
    if (document.activeElement && document.activeElement.id === 'wmcanvas' && ['up', 'down', 'left', 'right'].includes(dir)) { this.g.worldMap.nav(dir); return; }
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
