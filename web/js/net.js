export const PROTO = 6;
export const CLIENT = 'lanternfall-web/0.1.0';

// WebSocket client with request ids, safe resend on reconnect (the server
// de-duplicates by rid), and traffic accounting.
export class Net {
  constructor() {
    this.ws = null;
    this.handlers = {};
    this.rid = (Math.random() * 0x3fffffff) | 0 || 1;
    this.pending = new Map(); // rid -> {msg, resolve, sent}
    this.bytesIn = 0; this.bytesOut = 0;
    this.state = 'idle';
    this.hello = null;
    this.retry = 0;
    this.closedByServer = null;
  }
  on(t, f) { this.handlers[t] = f; }
  emit(t, m) { const h = this.handlers[t]; if (h) h(m); }

  url() { return (location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/ws'; }

  connect(hello) {
    this.hello = hello;
    this.closedByServer = null;
    return new Promise((resolve, reject) => {
      let welcomed = false;
      const ws = new WebSocket(this.url());
      ws.binaryType = 'arraybuffer';
      this.ws = ws;
      this.state = 'connecting';
      ws.onopen = () => { this.sendRaw(this.hello); };
      ws.onmessage = (ev) => {
        if (typeof ev.data !== 'string') {
          this.bytesIn += ev.data.byteLength;
          this.emit('binary', new Uint8Array(ev.data));
          return;
        }
        this.bytesIn += ev.data.length;
        let m;
        try { m = JSON.parse(ev.data); } catch { return; }
        if (m.t === 'welcome') {
          welcomed = true; this.state = 'open'; this.retry = 0;
          resolve(m);
          // Resend in-flight requests; the server answers duplicates from its rid cache.
          for (const p of this.pending.values()) this.sendRaw(p.msg);
        }
        if (m.t === 'error') {
          this.closedByServer = m;
          if (!welcomed) reject(m);
        }
        if (m.t === 'ack') {
          const p = this.pending.get(m.rid);
          if (p) { this.pending.delete(m.rid); p.resolve(m); }
        }
        this.emit(m.t, m);
      };
      ws.onclose = () => {
        const was = this.state;
        this.state = 'closed';
        if (!welcomed) { reject(this.closedByServer || { code: 'connect_failed', text: 'Could not reach the world. Is the server running?' }); return; }
        this.emit('disconnect', { was, server: this.closedByServer });
      };
    });
  }

  sendRaw(m) {
    if (this.ws && this.ws.readyState === 1) {
      const s = JSON.stringify(m);
      this.bytesOut += s.length;
      this.ws.send(s);
    }
  }
  send(m) { this.sendRaw(m); }

  // State-changing request: gets a rid and resolves with the server's ack.
  request(m) {
    this.rid = (this.rid + 1) >>> 0 || 1;
    m.rid = this.rid;
    return new Promise((resolve) => {
      this.pending.set(m.rid, { msg: m, resolve, sent: performance.now() });
      if (this.pending.size > 64) { const k = this.pending.keys().next().value; this.pending.get(k).resolve({ ok: false, err: 'timeout' }); this.pending.delete(k); }
      this.sendRaw(m);
    });
  }

  close() { if (this.ws) this.ws.close(); }
}

// Transport for the in-browser world: same interface as Net, but talks to
// the Go engine running in a Web Worker instead of a WebSocket server.
export class WorkerNet extends Net {
  constructor() {
    super();
    this.worker = null;
    this.readyInfo = null;
    this.waiters = {};
    this.onStatus = null; // (msg) => void for 'saved' | 'saveError' | 'storage'
  }

  // Starts the worker and resolves with the world's info (like /api/info).
  start() {
    if (this.started) return this.started;
    this.started = new Promise((resolve, reject) => {
      const w = new Worker('js/worker.js');
      this.worker = w;
      w.onerror = (e) => reject({ code: 'worker_failed', text: 'The in-browser world failed to start: ' + (e.message || 'unknown error') });
      w.onmessage = (ev) => {
        const m = ev.data;
        switch (m.t) {
          case 'ready': this.readyInfo = m.info; resolve(m.info); break;
          case 'fatal': reject({ code: 'world_unreadable', text: m.text }); this.emit('fatal', m); break;
          case 'text': this.onText(m.data); break;
          case 'bin': this.bytesIn += m.data.byteLength; this.emit('binary', m.data); break;
          case 'saved': case 'saveError': case 'storage': if (this.onStatus) this.onStatus(m); break;
          case 'reload': location.reload(); break;
          default: { const f = this.waiters[m.t]; if (f) { delete this.waiters[m.t]; f(m); } }
        }
      };
      w.postMessage({ t: 'start' });
    });
    return this.started;
  }

  onText(s) {
    this.bytesIn += s.length;
    let m;
    try { m = JSON.parse(s); } catch { return; }
    if (m.t === 'welcome') { this.state = 'open'; if (this.pendingHello) { this.pendingHello.resolve(m); this.pendingHello = null; } }
    if (m.t === 'error' && this.pendingHello) { this.pendingHello.reject(m); this.pendingHello = null; }
    if (m.t === 'ack') {
      const p = this.pending.get(m.rid);
      if (p) { this.pending.delete(m.rid); p.resolve(m); }
    }
    this.emit(m.t, m);
  }

  async connect(hello) {
    await this.start();
    this.hello = hello;
    return new Promise((resolve, reject) => {
      this.pendingHello = { resolve, reject };
      this.worker.postMessage({ t: 'hello', data: JSON.stringify(hello) });
    });
  }

  sendRaw(m) {
    if (!this.worker) return;
    const s = JSON.stringify(m);
    this.bytesOut += s.length;
    this.worker.postMessage({ t: 'send', data: s });
  }

  ask(t, extra, transfer) {
    return new Promise((resolve) => {
      this.waiters[t === 'import' ? 'importResult' : t] = resolve;
      this.worker.postMessage({ t, ...extra }, transfer || []);
    });
  }
  exportWorld() { return this.ask('export').then((m) => m.data); }
  importWorld(bytes) { return this.ask('import', { data: bytes }, [bytes.buffer]); }
  flush() { if (this.worker) this.worker.postMessage({ t: 'flush' }); }
  startFresh() { this.worker.postMessage({ t: 'startFresh' }); }
  close() { this.flush(); }
}
