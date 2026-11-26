// Small SVG item icons, drawn from the same model names as the 3D items
// (models.js) and tinted with the item's color. Add a shape here when you add
// a model there.
import { modelFor } from './models.js';

const safeColor = (c) => (/^#[0-9a-fA-F]{6}$/.test(c || '') ? c : '#888888');

function shade(hex, k) {
  const n = parseInt(hex.slice(1), 16);
  const f = (v) => Math.max(0, Math.min(255, Math.round(v * k))).toString(16).padStart(2, '0');
  return '#' + f(n >> 16) + f((n >> 8) & 255) + f(n & 255);
}

const GRIP = '#6b4423', DARK = '#3a3434';

const SHAPES = {
  sword: (c, d) => `<path d="M20 3 L23 6 L11 18 L8 15 Z" fill="${c}" stroke="${d}" stroke-width="1"/>
    <path d="M5 13 L13 21" stroke="${d}" stroke-width="2.6" stroke-linecap="round"/>
    <path d="M8 18 L4 22" stroke="${GRIP}" stroke-width="2.6" stroke-linecap="round"/>`,
  axe: (c, d) => `<path d="M6 21 L17 6" stroke="${GRIP}" stroke-width="2.4" stroke-linecap="round"/>
    <path d="M13 5 Q20 2 22 8 Q18 12 15 10 Z" fill="${c}" stroke="${d}" stroke-width="1"/>`,
  pickaxe: (c, d) => `<path d="M7 21 L16 8" stroke="${GRIP}" stroke-width="2.4" stroke-linecap="round"/>
    <path d="M5 7 Q14 1 23 9 Q14 5 5 7 Z" fill="${c}" stroke="${d}" stroke-width="1.2"/>`,
  club: (c, d) => `<path d="M6 20 L10 16" stroke="${GRIP}" stroke-width="2.6" stroke-linecap="round"/>
    <path d="M9 15 L17 5 Q21 3 21 7 L12 18 Z" fill="${c}" stroke="${d}" stroke-width="1"/>`,
  lantern: (c, d) => `<rect x="9" y="4" width="8" height="2.5" rx="1" fill="${DARK}"/>
    <rect x="8" y="7" width="10" height="11" rx="2" fill="${c}" stroke="${d}" stroke-width="1"/>
    <rect x="7" y="18" width="12" height="2.5" rx="1" fill="${DARK}"/><path d="M11 4 Q13 0 15 4" stroke="${DARK}" fill="none"/>`,
  gem: (c, d) => `<path d="M13 3 L21 11 L13 22 L5 11 Z" fill="${c}" stroke="${d}" stroke-width="1"/>
    <path d="M5 11 L21 11 M13 3 L10 11 L13 22 M13 3 L16 11 L13 22" stroke="${d}" stroke-width=".7" fill="none"/>`,
  food: (c, d) => `<circle cx="13" cy="14" r="7" fill="${c}" stroke="${d}" stroke-width="1"/>
    <path d="M13 7 Q15 3 19 4 Q17 8 13 7" fill="#4c9a3c"/><circle cx="10.5" cy="12" r="1.6" fill="#fff" opacity=".45"/>`,
  block: (c, d) => `<path d="M13 3 L22 8 L13 13 L4 8 Z" fill="${shade(c, 1.2)}"/><path d="M4 8 L13 13 L13 23 L4 18 Z" fill="${c}"/>
    <path d="M22 8 L13 13 L13 23 L22 18 Z" fill="${d}"/>`,
  coin: (c, d) => `<circle cx="13" cy="13" r="8.5" fill="${c}" stroke="${d}" stroke-width="1.4"/><circle cx="13" cy="13" r="5" fill="none" stroke="${d}" stroke-width=".9"/>`,
  rod: (c, d) => `<path d="M5 22 L21 3" stroke="${c}" stroke-width="2" stroke-linecap="round"/><circle cx="8" cy="18" r="2" fill="${DARK}"/>
    <path d="M21 3 Q23 12 19 18" stroke="#e8e6dc" stroke-width=".8" fill="none"/><path d="M19 18 l-1 2 l2 0 z" fill="${d}"/>`,
  fish: (c, d) => `<path d="M4 13 Q11 5 19 13 Q11 21 4 13 Z" fill="${c}" stroke="${d}" stroke-width="1"/>
    <path d="M18 13 L24 8 L24 18 Z" fill="${d}"/><circle cx="8" cy="12" r="1.2" fill="${DARK}"/>`,
  tinderbox: (c, d) => `<rect x="4" y="9" width="18" height="11" rx="2" fill="${c}" stroke="${d}"/><rect x="4" y="7" width="18" height="4" rx="1.5" fill="${d}"/>
    <path d="M13 3 Q16 6 13 8 Q10 6 13 3" fill="#ff9a3a"/>`,
  log: (c, d) => `<rect x="3" y="8" width="17" height="10" rx="2" fill="${c}" stroke="${d}"/><ellipse cx="20" cy="13" rx="3.5" ry="5" fill="#d8aa6a" stroke="${d}"/>
    <ellipse cx="20" cy="13" rx="1.5" ry="2.4" fill="none" stroke="${d}" stroke-width=".7"/>`,
  bundle: (c, d) => `<rect x="4" y="6" width="18" height="6" rx="3" fill="${c}" stroke="${d}"/><rect x="4" y="14" width="18" height="6" rx="3" fill="${c}" stroke="${d}"/>
    <circle cx="20" cy="9" r="2" fill="${d}"/><circle cx="20" cy="17" r="2" fill="${d}"/>`,
};

const cache = new Map();

// itemIcon returns inline SVG markup for an item definition.
export function itemIcon(it) {
  const color = safeColor(it && it.color);
  const model = it && it.kind === 'missing' ? 'missing' : modelFor(it);
  const key = model + color + (it && it.glow ? 'g' : '');
  let svg = cache.get(key);
  if (!svg) {
    const body = model === 'missing'
      ? `<text x="13" y="18" text-anchor="middle" font-size="14" fill="#ccc">?</text>`
      : SHAPES[model](color, shade(color, 0.55));
    const glow = it && it.glow ? ` style="filter:drop-shadow(0 0 3px ${color})"` : '';
    svg = `<svg class="ic" viewBox="0 0 26 26" aria-hidden="true"${glow}>${body}</svg>`;
    cache.set(key, svg);
  }
  return svg;
}
