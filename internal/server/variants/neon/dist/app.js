// neon variant — cyberpunk grafana.
//
// Polls /dashboard/api/state every 2s. Builds a rolling per-account
// request-count history (last 30 samples) to drive canvas sparklines
// with native shadowBlur glow. Palette opens via ⌘K. No framework.
// See .sisyphus/plans/variant-neon-manifesto.md.

const POLL_MS = 2000;
const API = "/dashboard/api/state";
const SPARK_LEN = 30;

const COLORS = {
  magenta: "#ff006e",
  lime: "#39ff14",
  grid: "rgba(91, 107, 213, 0.20)",
  fill: "rgba(255, 0, 110, 0.18)",
};

const el = {
  ver: document.querySelector("[data-ver]"),
  upt: document.querySelector("[data-upt]"),
  ready: document.querySelector("[data-ready]"),
  vault: document.querySelector("[data-vault]"),
  chipR: document.querySelector("[data-chip-rdy]"),
  chipV: document.querySelector("[data-chip-vlt]"),
  totreq: document.querySelector("[data-totreq]"),
  toterr: document.querySelector("[data-toterr]"),
  sparkT: /** @type {HTMLCanvasElement} */ (document.querySelector("[data-spark-total]")),
  sparkE: /** @type {HTMLCanvasElement} */ (document.querySelector("[data-spark-err]")),
  heroErr: document.querySelector("[data-hero-err]"),
  stamp: document.querySelector("[data-stamp]"),
  rows: document.getElementById("rows"),
  empty: document.getElementById("empty"),
  palette: /** @type {HTMLDialogElement} */ (document.getElementById("palette")),
  paletteInput: /** @type {HTMLInputElement} */ (document.getElementById("palette-input")),
  paletteList: document.getElementById("palette-list"),
  paletteTrigger: document.querySelector("[data-palette-trigger]"),
};

const hist = {
  total: [],
  err: [],
  perAcct: new Map(), // id -> array
};
const seen = new Set();

function push(arr, v) {
  arr.push(v);
  if (arr.length > SPARK_LEN) arr.shift();
}

function fmtUptime(s) {
  if (!Number.isFinite(s) || s < 0) return "—";
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d > 0) return `${d}d${h}h`;
  if (h > 0) return `${h}h${m}m`;
  return `${m}m`;
}

/**
 * Draw a sparkline on canvas with a drop-shadow glow. Line color drives
 * both the stroke and the shadow. Values can be any finite numbers; the
 * line is scaled to min/max of the visible window.
 *
 * @param {HTMLCanvasElement} canvas
 * @param {number[]} values
 * @param {string} color
 */
function spark(canvas, values, color) {
  if (!canvas) return;
  const dpr = window.devicePixelRatio || 1;
  const cssW = canvas.clientWidth || canvas.width;
  const cssH = canvas.clientHeight || canvas.height;
  if (canvas.width !== cssW * dpr || canvas.height !== cssH * dpr) {
    canvas.width = Math.max(1, Math.floor(cssW * dpr));
    canvas.height = Math.max(1, Math.floor(cssH * dpr));
  }
  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, cssW, cssH);

  if (values.length < 2) return;
  const pad = 2;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;

  const step = (cssW - pad * 2) / (SPARK_LEN - 1);
  const startX = pad + (SPARK_LEN - values.length) * step;

  // Fill under the line — faint magenta wash.
  ctx.beginPath();
  ctx.moveTo(startX, cssH - pad);
  values.forEach((v, i) => {
    const x = startX + i * step;
    const y = pad + (1 - (v - min) / range) * (cssH - pad * 2);
    if (i === 0) ctx.lineTo(x, y); else ctx.lineTo(x, y);
  });
  ctx.lineTo(startX + (values.length - 1) * step, cssH - pad);
  ctx.closePath();
  ctx.fillStyle = COLORS.fill;
  ctx.fill();

  // The line itself with glow.
  ctx.beginPath();
  values.forEach((v, i) => {
    const x = startX + i * step;
    const y = pad + (1 - (v - min) / range) * (cssH - pad * 2);
    if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
  });
  ctx.strokeStyle = color;
  ctx.lineWidth = 1.5;
  ctx.shadowColor = color;
  ctx.shadowBlur = 8;
  ctx.lineJoin = "round";
  ctx.lineCap = "round";
  ctx.stroke();
  ctx.shadowBlur = 0;
}

function stateFor(a) {
  if (a.cooldown_until) return { cls: "warn", text: "cooldown" };
  if (a.last_error)     return { cls: "err", text: "error" };
  if (!a.enabled)       return { cls: "off", text: "paused" };
  return { cls: "ok", text: "active" };
}

function rowFor(a) {
  const tr = document.createElement("tr");
  if (!seen.has(a.id)) { tr.dataset.new = "1"; seen.add(a.id); }

  const tdId = document.createElement("td");
  const idSpan = document.createElement("span");
  idSpan.className = "id";
  idSpan.textContent = a.id;
  tdId.appendChild(idSpan);

  const tdSt = document.createElement("td");
  const st = stateFor(a);
  const wrap = document.createElement("span");
  wrap.className = "state";
  const dot = document.createElement("span");
  dot.className = `s-dot ${st.cls}`;
  dot.setAttribute("aria-hidden", "true");
  const txt = document.createElement("span");
  txt.className = "s-text";
  txt.textContent = st.text;
  wrap.append(dot, txt);
  tdSt.appendChild(wrap);

  const tdReq = document.createElement("td");
  tdReq.className = "n";
  tdReq.textContent = String(a.requests ?? 0);

  const tdErr = document.createElement("td");
  tdErr.className = "n" + ((a.errors ?? 0) > 0 ? " is-err" : "");
  tdErr.textContent = String(a.errors ?? 0);

  const tdSpark = document.createElement("td");
  const c = document.createElement("canvas");
  c.width = 140;
  c.height = 22;
  tdSpark.appendChild(c);
  const h = hist.perAcct.get(a.id) || [];
  spark(c, h, COLORS.magenta);

  const tdNote = document.createElement("td");
  tdNote.className = "note";
  if (a.cooldown_until) tdNote.textContent = `cooling ${a.cooldown_until}`;
  else if (a.last_error) tdNote.textContent = a.last_error;
  else if (!a.enabled) tdNote.textContent = "paused";
  else tdNote.textContent = "—";

  tr.append(tdId, tdSt, tdReq, tdErr, tdSpark, tdNote);
  return tr;
}

function setChip(node, text, mod) {
  const b = node.querySelector("b");
  b.textContent = text;
  node.classList.remove("is-ok", "is-bad");
  if (mod === "ok") node.classList.add("is-ok");
  else if (mod === "bad") node.classList.add("is-bad");
}

async function tick() {
  try {
    const r = await fetch(API, { headers: { accept: "application/json" } });
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    render(await r.json());
  } catch (err) {
    el.stamp.textContent = `ERR ${(err && err.message) || err}`;
  }
}

function render(snap) {
  const now = new Date();
  el.stamp.textContent = `last.sync ${now.toTimeString().slice(0, 8)}`;

  el.ver.textContent = snap.version || "—";
  el.upt.textContent = fmtUptime(snap.uptime_s);
  setChip(el.chipR, snap.ready ? "ready" : "down", snap.ready ? "ok" : "bad");
  setChip(el.chipV, snap.vault_ok ? "open" : "sealed", snap.vault_ok ? "ok" : "bad");

  const accts = snap.accounts || [];
  const totReq = accts.reduce((n, a) => n + (a.requests || 0), 0);
  const totErr = accts.reduce((n, a) => n + (a.errors || 0), 0);
  el.totreq.textContent = totReq.toLocaleString();
  el.toterr.textContent = totErr.toLocaleString();
  el.heroErr.classList.toggle("is-bad", totErr > 0);

  push(hist.total, totReq);
  push(hist.err, totErr);
  spark(el.sparkT, hist.total, COLORS.magenta);
  spark(el.sparkE, hist.err, totErr > 0 ? COLORS.magenta : COLORS.lime);

  // Per-account history (track deltas to make idle lines go flat, not
  // exponential — sparkline should reflect activity, not cumulative total).
  const present = new Set(accts.map((a) => a.id));
  for (const a of accts) {
    const prev = hist.perAcct.get(a.id) || [];
    push(prev, a.requests || 0);
    hist.perAcct.set(a.id, prev);
  }
  for (const id of hist.perAcct.keys()) if (!present.has(id)) hist.perAcct.delete(id);
  for (const id of seen) if (!present.has(id)) seen.delete(id);

  el.rows.replaceChildren();
  if (accts.length === 0) {
    el.empty.hidden = false;
  } else {
    el.empty.hidden = true;
    for (const a of accts) el.rows.appendChild(rowFor(a));
  }
}

/* ---- Palette ---- */
const COMMANDS = [
  { id: "refresh", label: "Refresh snapshot", run: () => tick() },
  { id: "import", label: "Import accounts (see CLI hint)", run: () => {
    alert("Import endpoint is v1.1. Run: kiroxy import-json < tokens.json");
  }},
  { id: "v-classic", label: "Switch to /dashboard",         run: () => (location.href = "/dashboard") },
  { id: "v-next",    label: "Switch to /dashboard-next",    run: () => (location.href = "/dashboard-next") },
  { id: "v-mansion", label: "Switch to /dashboard-mansion", run: () => (location.href = "/dashboard-mansion") },
  { id: "v-brutal",  label: "Switch to /dashboard-brutal",  run: () => (location.href = "/dashboard-brutal") },
  { id: "v-paper",   label: "Switch to /dashboard-paper",   run: () => (location.href = "/dashboard-paper") },
  { id: "v-nord",    label: "Switch to /dashboard-nord",    run: () => (location.href = "/dashboard-nord") },
];
let pSel = 0;

function renderPalette(q) {
  const qq = q.trim().toLowerCase();
  const m = qq === "" ? COMMANDS : COMMANDS.filter((c) => c.label.toLowerCase().includes(qq));
  el.paletteList.replaceChildren();
  pSel = Math.min(pSel, Math.max(0, m.length - 1));
  m.forEach((c, i) => {
    const li = document.createElement("li");
    li.setAttribute("role", "option");
    li.setAttribute("aria-selected", String(i === pSel));
    li.textContent = c.label;
    li.addEventListener("click", () => { c.run(); el.palette.close(); });
    el.paletteList.appendChild(li);
  });
  return m;
}

function openPalette() {
  pSel = 0;
  el.paletteInput.value = "";
  renderPalette("");
  el.palette.showModal();
  el.paletteInput.focus();
}
el.paletteTrigger.addEventListener("click", (e) => { e.preventDefault(); openPalette(); });
el.paletteInput.addEventListener("input", (e) => { pSel = 0; renderPalette(e.target.value); });
el.paletteInput.addEventListener("keydown", (e) => {
  const m = renderPalette(el.paletteInput.value);
  if (e.key === "ArrowDown") { e.preventDefault(); pSel = Math.min(pSel + 1, m.length - 1); renderPalette(el.paletteInput.value); }
  else if (e.key === "ArrowUp") { e.preventDefault(); pSel = Math.max(pSel - 1, 0); renderPalette(el.paletteInput.value); }
  else if (e.key === "Enter") { e.preventDefault(); const c = m[pSel]; if (c) { c.run(); el.palette.close(); } }
});
addEventListener("keydown", (e) => {
  const t = e.target;
  const typing = t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable);
  if ((e.metaKey || e.ctrlKey) && e.key === "k") { e.preventDefault(); openPalette(); return; }
  if (typing) return;
  if (e.key === "r") { e.preventDefault(); tick(); }
});

/* Redraw sparklines on resize (debounced). */
let _resizeT;
addEventListener("resize", () => {
  clearTimeout(_resizeT);
  _resizeT = setTimeout(() => {
    spark(el.sparkT, hist.total, COLORS.magenta);
    spark(el.sparkE, hist.err, COLORS.lime);
  }, 100);
});

tick();
setInterval(tick, POLL_MS);
