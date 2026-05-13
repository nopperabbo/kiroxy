// nord variant — arctic calm palette, slow motion, single density.
//
// Polls /dashboard/api/state every 2s and rewrites the pool table.
// Palette opens via ⌘K. No framework; DOM updates via textContent/
// appendChild. See .sisyphus/plans/variant-nord-manifesto.md.

const POLL_MS = 2000;
const API = "/dashboard/api/state";

const el = {
  ver: document.querySelector("[data-ver]"),
  upt: document.querySelector("[data-upt]"),
  ready: document.querySelector("[data-ready]"),
  vault: document.querySelector("[data-vault]"),
  totreq: document.querySelector("[data-totreq]"),
  toterr: document.querySelector("[data-toterr]"),
  rows: document.getElementById("rows"),
  empty: document.getElementById("empty"),
  stamp: document.querySelector("[data-stamp]"),
  liveDot: document.querySelector("[data-live-dot]"),
  palette: /** @type {HTMLDialogElement} */ (document.getElementById("palette")),
  paletteInput: /** @type {HTMLInputElement} */ (document.getElementById("palette-input")),
  paletteList: document.getElementById("palette-list"),
  paletteTrigger: document.querySelector("[data-palette-trigger]"),
};

const seen = new Set();

function fmtUptime(s) {
  if (!Number.isFinite(s) || s < 0) return "—";
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

function statusFor(a) {
  if (a.cooldown_until) return { mod: "warn", label: "cooldown" };
  if (a.last_error)     return { mod: "err",  label: "error" };
  if (!a.enabled)       return { mod: "off",  label: "paused" };
  return { mod: "ok", label: "active" };
}

function row(a) {
  const tr = document.createElement("tr");
  if (!seen.has(a.id)) { tr.dataset.new = "1"; seen.add(a.id); }

  const cId = document.createElement("td");
  cId.className = "mono";
  cId.textContent = a.id;

  const cState = document.createElement("td");
  const s = statusFor(a);
  const dot = document.createElement("span");
  dot.className = `dot ${s.mod}`;
  dot.setAttribute("aria-hidden", "true");
  cState.appendChild(dot);
  cState.appendChild(document.createTextNode(" " + s.label));

  const cReq = document.createElement("td");
  cReq.className = "num";
  cReq.textContent = String(a.requests ?? 0);

  const cErr = document.createElement("td");
  cErr.className = "num";
  cErr.textContent = String(a.errors ?? 0);
  if ((a.errors ?? 0) > 0) cErr.classList.add("is-err");

  const cNote = document.createElement("td");
  cNote.className = "note";
  if (a.cooldown_until) cNote.textContent = `until ${a.cooldown_until}`;
  else if (a.last_error) cNote.textContent = a.last_error;
  else if (!a.enabled) cNote.textContent = "manually paused";
  else cNote.textContent = "—";

  tr.append(cId, cState, cReq, cErr, cNote);
  return tr;
}

function setMetric(node, text, mod) {
  node.textContent = text;
  node.classList.remove("is-ok", "is-bad");
  if (mod === "ok") node.classList.add("is-ok");
  else if (mod === "bad") node.classList.add("is-bad");
}

async function tick() {
  try {
    const r = await fetch(API, { headers: { accept: "application/json" } });
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    render(await r.json());
    el.liveDot.classList.remove("is-bad");
  } catch (err) {
    el.stamp.textContent = `fetch failed · ${(err && err.message) || err}`;
    el.liveDot.classList.add("is-bad");
  }
}

function render(snap) {
  const now = new Date();
  const hh = String(now.getHours()).padStart(2, "0");
  const mm = String(now.getMinutes()).padStart(2, "0");
  const ss = String(now.getSeconds()).padStart(2, "0");
  el.stamp.textContent = `last sync ${hh}:${mm}:${ss}`;

  setMetric(el.ver, snap.version || "—");
  setMetric(el.upt, fmtUptime(snap.uptime_s));
  setMetric(el.ready, snap.ready ? "ready" : `down${snap.ready_detail ? " · " + snap.ready_detail : ""}`,
            snap.ready ? "ok" : "bad");
  setMetric(el.vault, snap.vault_ok ? "open" : "sealed", snap.vault_ok ? "ok" : "bad");

  const accts = snap.accounts || [];
  const totReq = accts.reduce((n, a) => n + (a.requests || 0), 0);
  const totErr = accts.reduce((n, a) => n + (a.errors || 0), 0);
  setMetric(el.totreq, totReq.toLocaleString());
  setMetric(el.toterr, totErr.toLocaleString(), totErr > 0 ? "bad" : "");

  el.rows.replaceChildren();
  if (accts.length === 0) {
    el.empty.hidden = false;
  } else {
    el.empty.hidden = true;
    const present = new Set(accts.map((a) => a.id));
    for (const id of seen) if (!present.has(id)) seen.delete(id);
    for (const a of accts) el.rows.appendChild(row(a));
  }
}

/* ---- Palette ---- */
const COMMANDS = [
  { id: "refresh", label: "Refresh snapshot", run: () => tick() },
  { id: "import", label: "How to import accounts", run: showImport },
  { id: "v-classic", label: "Switch to /dashboard",          run: () => (location.href = "/dashboard") },
  { id: "v-next",    label: "Switch to /dashboard-next",     run: () => (location.href = "/dashboard-next") },
  { id: "v-mansion", label: "Switch to /dashboard-mansion",  run: () => (location.href = "/dashboard-mansion") },
  { id: "v-brutal",  label: "Switch to /dashboard-brutal",   run: () => (location.href = "/dashboard-brutal") },
  { id: "v-paper",   label: "Switch to /dashboard-paper",    run: () => (location.href = "/dashboard-paper") },
];

let pSel = 0;

function renderList(q) {
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
  renderList("");
  el.palette.showModal();
  el.paletteInput.focus();
}

function showImport() {
  openPalette();
  el.paletteInput.value = "";
  const li = document.createElement("li");
  li.className = "is-hint";
  li.innerHTML = "";
  li.textContent = "Import endpoint is v1.1. Use CLI: kiroxy import-json < tokens.json";
  el.paletteList.replaceChildren(li);
}

el.paletteTrigger.addEventListener("click", (e) => { e.preventDefault(); openPalette(); });
el.paletteInput.addEventListener("input", (e) => { pSel = 0; renderList(e.target.value); });
el.paletteInput.addEventListener("keydown", (e) => {
  const m = renderList(el.paletteInput.value);
  if (e.key === "ArrowDown") { e.preventDefault(); pSel = Math.min(pSel + 1, m.length - 1); renderList(el.paletteInput.value); }
  else if (e.key === "ArrowUp") { e.preventDefault(); pSel = Math.max(pSel - 1, 0); renderList(el.paletteInput.value); }
  else if (e.key === "Enter") { e.preventDefault(); const c = m[pSel]; if (c) { c.run(); el.palette.close(); } }
});

addEventListener("keydown", (e) => {
  const t = e.target;
  const typing = t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable);
  if ((e.metaKey || e.ctrlKey) && e.key === "k") { e.preventDefault(); openPalette(); return; }
  if (typing) return;
  if (e.key === "r") { e.preventDefault(); tick(); }
});

tick();
setInterval(tick, POLL_MS);
