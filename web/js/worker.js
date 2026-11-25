// In-browser world host. Runs the Go game engine (WebAssembly) inside this
// Web Worker so the simulation never blocks rendering, and keeps saves in
// IndexedDB: the current snapshot plus a few rotating backups.
/* global Go, importScripts, indexedDB */
importScripts('../wasm_exec.js');

const DB = 'lanternfall', STORE = 'saves';
const BACKUP_EVERY_MS = 5 * 60 * 1000, KEEP_BACKUPS = 5;
let lastBackup = 0, started = false, ready = false;
const queue = []; // client messages that arrive before the world is ready

function db() {
  return new Promise((resolve, reject) => {
    const r = indexedDB.open(DB, 1);
    r.onupgradeneeded = () => r.result.createObjectStore(STORE);
    r.onsuccess = () => resolve(r.result);
    r.onerror = () => reject(r.error);
  });
}
async function tx(mode, fn) {
  const d = await db();
  return new Promise((resolve, reject) => {
    const t = d.transaction(STORE, mode);
    const out = fn(t.objectStore(STORE));
    t.oncomplete = () => resolve(out instanceof IDBRequest ? out.result : undefined);
    t.onerror = () => reject(t.error);
    t.onabort = () => reject(t.error || new Error('transaction aborted'));
  });
}
const get = (k) => tx('readonly', (s) => s.get(k));
const put = (k, v) => tx('readwrite', (s) => s.put(v, k));
const del = (k) => tx('readwrite', (s) => s.delete(k));
const keys = () => tx('readonly', (s) => s.getAllKeys());

async function backupKeys() {
  return (await keys()).filter((k) => String(k).startsWith('backup-')).sort().reverse(); // newest first
}

// Called by Go with each snapshot. The write is atomic per IndexedDB transaction.
self.lfSave = (u8) => {
  const copy = u8.slice();
  (async () => {
    await put('state', copy);
    if (Date.now() - lastBackup > BACKUP_EVERY_MS) {
      lastBackup = Date.now();
      await put('backup-' + new Date().toISOString(), copy);
      const old = (await backupKeys()).slice(KEEP_BACKUPS);
      for (const k of old) await del(k);
    }
    postMessage({ t: 'saved', at: Date.now() });
  })().catch((e) => postMessage({ t: 'saveError', text: String(e && e.message || e) }));
};
self.lfOut = (data) => {
  if (typeof data === 'string') postMessage({ t: 'text', data });
  else postMessage({ t: 'bin', data }, [data.buffer]);
};
self.lfReady = (info) => {
  ready = true;
  postMessage({ t: 'ready', info: JSON.parse(info) });
  for (const m of queue.splice(0)) handle(m);
};
self.lfFatal = (text) => postMessage({ t: 'fatal', text });

async function start(fresh) {
  if (started) return;
  started = true;
  let persisted = false;
  try { persisted = navigator.storage && navigator.storage.persist ? await navigator.storage.persist() : false; } catch { /* not supported */ }
  postMessage({ t: 'storage', persisted });
  const saves = [];
  if (!fresh) {
    const s = await get('state');
    if (s) saves.push(s);
    for (const k of await backupKeys()) saves.push(await get(k));
  }
  self.lfSaves = saves;
  const go = new Go();
  const resp = await fetch('../lanternfall.wasm');
  const { instance } = WebAssembly.instantiateStreaming
    ? await WebAssembly.instantiateStreaming(resp, go.importObject)
    : await WebAssembly.instantiate(await resp.arrayBuffer(), go.importObject);
  go.run(instance);
}

async function handle(m) {
  switch (m.t) {
    case 'hello': self.lfHello(m.data); break;
    case 'send': self.lfSend(m.data); break;
    case 'flush': self.lfFlush(); break;
    case 'export': postMessage({ t: 'export', data: self.lfExport() }); break;
    case 'import': {
      const err = self.lfValidate(m.data);
      if (err) { postMessage({ t: 'importResult', ok: false, text: err }); break; }
      const cur = self.lfExport();
      await put('backup-' + new Date().toISOString() + '-before-import', cur);
      await put('state', m.data);
      postMessage({ t: 'importResult', ok: true });
      break;
    }
  }
}

onmessage = async (ev) => {
  const m = ev.data;
  try {
    if (m.t === 'start') { await start(false); return; }
    if (m.t === 'startFresh') {
      // Keep the unreadable data aside rather than deleting it.
      const s = await get('state');
      if (s) await put('unreadable-' + new Date().toISOString(), s);
      await del('state');
      for (const k of await backupKeys()) await del(k);
      started = false;
      postMessage({ t: 'reload' });
      return;
    }
    if (!ready) { queue.push(m); return; }
    await handle(m);
  } catch (e) {
    postMessage({ t: 'fatal', text: String(e && e.message || e) });
  }
};
