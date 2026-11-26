// The join screen: world info, connection diagnostics, and the
// continue / new character form. Works for hosted and browser worlds.
import { PROTO, CLIENT } from './net.js';
import { esc } from './ui.js';

const $ = (s) => document.querySelector(s);
const params = new URLSearchParams(location.search);
export const COLORS = ['#4a90d9', '#e26d5a', '#5cb85c', '#f0ad4e', '#9b59b6', '#1abc9c', '#e84393', '#f5f0e1'];

// The browser remembers one character per world (realm) for "Continue as …".
export function savedChar(realm) { try { return JSON.parse(localStorage.getItem('lf.char.' + realm) || 'null'); } catch { return null; } }
export function forgetChar(realm) { try { localStorage.removeItem('lf.char.' + realm); } catch { /* private mode */ } }

export async function showJoin(game, err) {
  $('#screen-join').hidden = false;
  const wi = $('#worldinfo');
  let info;
  if (game.browserWorld) {
    wi.innerHTML = '<p class="note">Starting your world in this browser…</p>';
    try {
      info = await game.net.start();
    } catch (e) {
      wi.innerHTML = `<b>Your saved world couldn't be opened.</b><p class="note">${esc(e.text || e)}</p>`;
      $('#joinform').innerHTML = `<div class="form"><button class="btn primary" id="fresh">Start a new world</button>
        <span class="note">Your old world data is kept aside in this browser, not deleted.</span></div>`;
      $('#fresh').onclick = () => game.net.startFresh();
      return;
    }
  }
  try {
    if (!info) info = await (await fetch('/api/info', { cache: 'no-store' })).json();
  } catch {
    wi.innerHTML = `<b>Can't reach the world server.</b><p class="note">If you're joining a friend, make sure <code>lanternfall join</code> is still running.</p>`;
    $('#joinform').innerHTML = `<button class="btn primary" id="retry">Try again</button>`;
    $('#retry').onclick = () => showJoin(game);
    return;
  }
  game.info = info;
  if (/^https?:\/\//.test(info.source || '')) {
    $('#sourcelink').innerHTML = `Free software under ${esc(info.license || 'AGPL-3.0')} · <a href="${esc(info.source)}" target="_blank" rel="noopener">Source code</a>`;
  }
  const kind = info.kind === 'official' ? '<span class="badge official">Official world</span>'
    : info.kind === 'browser' ? '<span class="badge private">Browser world</span>' : '<span class="badge private">Private world</span>';
  wi.innerHTML = `<h3>${esc(info.name)}</h3>${kind}${info.modded ? '<span class="badge modded">Modded</span>' : ''}
    <span class="note">${info.kind === 'browser' ? 'Single-player · nothing to install' : `${info.players}/${info.max} players · PvP ${info.pvp ? 'on' : 'off'} · trading ${info.trading ? 'on' : 'off'}`}</span>
    ${info.desc ? `<p>${esc(info.desc)}</p>` : ''}
    <p class="sep">${info.kind === 'official'
      ? 'Your character here is an official-world character.'
      : info.kind === 'browser'
        ? 'This world runs and is <b>saved in this browser</b> on this device. Use <b>Settings → Export world</b> to keep a backup. Progress here stays in this world.'
        : 'Characters you make here are <b>saved by the host</b> and <b>stay in this world</b>. They are separate from official-world characters, and nothing from this world can be moved into an official one.'}</p>
    ${info.recovered ? '<p class="err">Your latest save was damaged, so the world was restored from an earlier backup.</p>' : ''}`;
  // Companion diagnostics (only present when joining through `lanternfall join`).
  let companion = null;
  if (!game.browserWorld) {
    try {
      const r = await fetch('/companion/status', { cache: 'no-store' });
      if (r.ok && (r.headers.get('content-type') || '').includes('json')) companion = await r.json();
    } catch { /* direct connection */ }
  }
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
  // In a browser world the player owns the whole save, so characters found in it can be resumed by name.
  const saved = savedChar(info.realm) || (game.browserWorld && info.characters && info.characters.length ? { name: info.characters[0], resume: true } : null);
  const invite = sessionStorage.getItem('lf.invite') || '';
  const form = $('#joinform');
  const showNew = !saved || game.forceNew;
  form.innerHTML = `<div class="form">
    ${saved && !game.forceNew ? `<button class="btn primary" id="continue">Continue as ${esc(saved.name)}</button>
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
      const w = await game.net.connect(hello);
      game.start(w);
    } catch (e) {
      $('#joinerr').textContent = e.text || 'Could not connect.';
      if (e.code === 'bad_token') { forgetChar(info.realm); game.forceNew = true; setTimeout(() => showJoin(game, e.text + ' Make a new character to play here.'), 1200); }
    }
  };
  if ($('#continue')) $('#continue').onclick = () => go(saved.resume ? { t: 'hello', proto: PROTO, client: CLIENT, name: saved.name } : { t: 'hello', proto: PROTO, client: CLIENT, token: saved.token });
  if ($('#newchar')) $('#newchar').onclick = () => { game.forceNew = true; showJoin(game); };
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
  game.input.setUIMode(true);
  game.input.onUI = (d) => {
    if (!game.me) {
      const els = [...form.querySelectorAll('button, input')];
      const i = els.indexOf(document.activeElement);
      if (d === 'down' || d === 'right') els[(i + 1) % els.length]?.focus();
      if (d === 'up' || d === 'left') els[(i - 1 + els.length) % els.length]?.focus();
      if (d === 'ok') document.activeElement?.click();
    } else game.ui.nav(d);
  };
  if (!game.joinLoop) { game.joinLoop = true; const pl = () => { if (!game.me) { game.input.poll(0.016); requestAnimationFrame(pl); } }; pl(); }
  if (params.get('autojoin')) {
    // Test/automation convenience: ?autojoin=Name joins immediately (never used for secrets).
    if (saved) go(saved.resume ? { t: 'hello', proto: PROTO, client: CLIENT, name: saved.name } : { t: 'hello', proto: PROTO, client: CLIENT, token: saved.token });
    else go({ t: 'hello', proto: PROTO, client: CLIENT, name: params.get('autojoin'), color: COLORS[(Math.random() * COLORS.length) | 0] });
  }
}
