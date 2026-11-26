// Per-device settings (localStorage is only used for conveniences like these
// and the per-world character token).
export const PRESETS = {
  low: { renderDist: 3, lodDist: 2, resScale: 0.65, ao: false, maxLights: 4, sway: false, label: 'Low (older phones, integrated GPUs)' },
  medium: { renderDist: 5, lodDist: 3, resScale: 0.85, ao: true, maxLights: 8, sway: true, label: 'Medium' },
  high: { renderDist: 8, lodDist: 5, resScale: 1.0, ao: true, maxLights: 16, sway: true, label: 'High' },
};

export const ACTIONS = {
  forward: 'Move forward', back: 'Move back', left: 'Strafe left', right: 'Strafe right', jump: 'Jump', sprint: 'Sprint',
  primary: 'Attack / gather', use: 'Use item / place block', interact: 'Talk / trade', menu: 'Open menu', hearth: 'Hearth Lantern',
  chat: 'Chat', pause: 'Settings / close', hotPrev: 'Previous hotbar slot', hotNext: 'Next hotbar slot', drop: 'Drop selected item', map: 'World map',
};

export const DEFAULT_KEYS = {
  forward: 'KeyW', back: 'KeyS', left: 'KeyA', right: 'KeyD', jump: 'Space', sprint: 'ShiftLeft',
  primary: 'Mouse0', use: 'Mouse2', interact: 'KeyE', menu: 'Tab', hearth: 'KeyH', chat: 'Enter', pause: 'Escape',
  hotPrev: 'KeyQ', hotNext: 'KeyR', drop: 'KeyG', map: 'KeyM',
};

const DEFAULTS = {
  quality: 'auto', textScale: 1, showMinimap: true, reduceMotion: false, reduceTransparency: false, showPerf: false, invertY: false, sensitivity: 1,
  keys: { ...DEFAULT_KEYS }, custom: null,
};

function load() {
  try {
    const s = JSON.parse(localStorage.getItem('lf.settings') || '{}');
    return { ...DEFAULTS, ...s, keys: { ...DEFAULT_KEYS, ...(s.keys || {}) } };
  } catch { return { ...DEFAULTS, keys: { ...DEFAULT_KEYS } }; }
}

export const settings = load();
if (window.matchMedia && matchMedia('(prefers-reduced-motion: reduce)').matches && localStorage.getItem('lf.settings') === null) settings.reduceMotion = true;
if (window.matchMedia && matchMedia('(prefers-reduced-transparency: reduce)').matches && localStorage.getItem('lf.settings') === null) settings.reduceTransparency = true;

export function save() {
  try { localStorage.setItem('lf.settings', JSON.stringify(settings)); } catch { /* private mode */ }
}

// Pick a starting preset from cheap device hints.
export function autoQuality() {
  const mem = navigator.deviceMemory || 4, cores = navigator.hardwareConcurrency || 4;
  const mobile = /Android|iPhone|iPad|Mobile/i.test(navigator.userAgent) || matchMedia('(pointer: coarse)').matches;
  if (mem <= 2 || cores <= 2) return 'low';
  if (mobile || mem <= 4) return 'medium';
  return 'high';
}

export function quality() {
  const q = settings.quality === 'auto' ? autoQuality() : settings.quality;
  return { name: q, ...PRESETS[q] || PRESETS.medium, ...(settings.custom || {}) };
}
