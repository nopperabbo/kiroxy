// linear-premium variant — signature SaaS done right.
//
// Polls /dashboard/api/state every 2s. Tracks last-seen request counts
// per account so row-flash can fire only on changed rows (not every
// tick). Palette uses @starting-style enter via CSS, JS only wires
// data + keyboard. No framework, no motion library. See the manifesto
// at .sisyphus/plans/variant-linear-premium-manifesto.md.

const POLL_MS = 2000;
const API = "/dashboard/api/state";

const el = {
  totreq: document.querySelector("[data-totreq]"),
  toterr: document.querySelector("[data-toterr]"),
  upt: document.querySelector("[data-upt]"),
  pillR: document.querySelector("[data-pill-rdy]"),
  pillV: document.querySelector("[data-pill-vlt]"),
  stamp: document.querySelector("[data-stamp]"),
  acctCount: document.querySelector("[data-acct-count]"),
  rows: document.getElementById("rows"),
  empty: document.getElementById("empty"),
  palette: /** @type {HTMLDialogElement} */ (document.getElementById("palette")),
  paletteInput: /** @type {HTMLInputElement} */ (document.getElementById("palette-input")),
  paletteList: document.getElementById("palette-list"),
  paletteTrigger: document.querySelector("[data-palette-trigger]"),
};

const prevReq = new Map(); // id -> last request count
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

function stateBadge(a) {
  const span = document.createElement("span");
  span.className = "state-badge";
  const dot = document.createElement("span");
  dot.className = "sb-dot";
  dot.setAttribute("aria-hidden", "true");
  const txt = document.createElement("span");
  if (a.cooldown_until) {
    span.classList.add("is-warn");
    txt.textContent = "cooldown";
  } else if (a.last_error) {
    span.classList.add("is-bad");
    txt.textContent = "error";
  } else if (!a.enabled) {
    span.classList.add("is-off");
    txt.textContent = "paused";
  } else {
    span.classList.add("is-ok");
    txt.textContent = "active";
  }
  span.append(dot, txt);
  return span;
}

function rowFor(a) {
  const tr = document.createElement("tr");
  if (!seen.has(a.id)) {
    tr.dataset.new = "1";
    seen.add(a.id);
  } else {
    const last = prevReq.get(a.id);
    if (last !== undefined && last !== (a.requests ?? 0)) {
      tr.dataset.flash = "1";
    }
  }

  const tdId = document.createElement("td");
  const id = document.createElement("span");
  id.className = "id";
  id.textContent = a.id;
  tdId.appendChild(id);

  const tdSt = document.createElement("td");
  tdSt.appendChild(stateBadge(a));

  const tdReq = document.createElement("td");
  tdReq.className = "num";
  tdReq.textContent = (a.requests ?? 0).toLocaleString();

  const tdErr = document.createElement("td");
  tdErr.className = "num" + ((a.errors ?? 0) > 0 ? " is-err" : "");
  tdErr.textContent = (a.errors ?? 0).toLocaleString();

  const tdNote = document.createElement("td");
  const note = document.createElement("span");
  note.className = "note";
  if (a.cooldown_until) note.textContent = `cooling until ${a.cooldown_until}`;
  else if (a.last_error) note.textContent = a.last_error;
  else if (!a.enabled) note.textContent = "manually paused";
  else note.textContent = "—";
  tdNote.appendChild(note);

  tr.append(tdId, tdSt, tdReq, tdErr, tdNote);
  prevReq.set(a.id, a.requests ?? 0);
  return tr;
}

function setPill(node, text, mod) {
  node.querySelector(".pill-text").textContent = text;
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
    el.stamp.textContent = `sync failed · ${(err && err.message) || err}`;
  }
}

function render(snap) {
  const now = new Date();
  el.stamp.textContent = `last sync ${now.toTimeString().slice(0, 8)}`;

  const accts = snap.accounts || [];
  const totReq = accts.reduce((n, a) => n + (a.requests || 0), 0);
  const totErr = accts.reduce((n, a) => n + (a.errors || 0), 0);
  el.totreq.textContent = totReq.toLocaleString();
  el.toterr.textContent = totErr.toLocaleString();
  el.upt.textContent = fmtUptime(snap.uptime_s);
  setPill(el.pillR, snap.ready ? "ready" : "down", snap.ready ? "ok" : "bad");
  setPill(el.pillV, snap.vault_ok ? "vault open" : "vault sealed", snap.vault_ok ? "ok" : "bad");
  el.acctCount.textContent = `${accts.length} account${accts.length === 1 ? "" : "s"}`;

  const present = new Set(accts.map((a) => a.id));
  for (const id of seen) if (!present.has(id)) { seen.delete(id); prevReq.delete(id); }

  if (accts.length === 0) {
    el.rows.replaceChildren();
    el.empty.hidden = false;
  } else {
    el.empty.hidden = true;
    const frag = document.createDocumentFragment();
    for (const a of accts) frag.appendChild(rowFor(a));
    el.rows.replaceChildren(frag);
  }
}

/* ---- Palette ---- */
const COMMANDS = [
  { id: "refresh",       label: "Refresh snapshot",              hint: "⟳ pool + health", run: () => tick() },
  { id: "jump-pool",     label: "Go to Pool",                    hint: "#pool",          run: () => document.getElementById("pool").scrollIntoView({ behavior: "smooth" }) },
  { id: "import",        label: "How to import accounts",        hint: "CLI hint",       run: () => showHint("Import endpoint is v1.1. Use CLI:  kiroxy import-json < tokens.json") },
  { id: "v-classic",     label: "Switch to /dashboard",          hint: "classic htmx",   run: () => go("/dashboard") },
  { id: "v-next",        label: "Switch to /dashboard-next",     hint: "cyan-teal",      run: () => go("/dashboard-next") },
  { id: "v-mansion",     label: "Switch to /dashboard-mansion",  hint: "amber operator", run: () => go("/dashboard-mansion") },
  { id: "v-brutal",      label: "Switch to /dashboard-brutal",   hint: "terminal",       run: () => go("/dashboard-brutal") },
  { id: "v-paper",       label: "Switch to /dashboard-paper",    hint: "ink on cream",   run: () => go("/dashboard-paper") },
  { id: "v-nord",        label: "Switch to /dashboard-nord",     hint: "arctic calm",    run: () => go("/dashboard-nord") },
  { id: "v-neon",        label: "Switch to /dashboard-neon",     hint: "cyberpunk",      run: () => go("/dashboard-neon") },
  { id: "v-muji",        label: "Switch to /dashboard-muji",     hint: "minimalist",     run: () => go("/dashboard-muji") },
];

/**
 * Navigate using the View Transitions API if available. Falls back to
 * plain location.href on browsers without support. No third-party polyfill.
 */
function go(href) {
  if (!document.startViewTransition) {
    location.href = href;
    return;
  }
  document.startViewTransition(() => { location.href = href; });
}

let pSel = 0;

function renderList(q) {
  const qq = q.trim().toLowerCase();
  const m = qq === "" ? COMMANDS : COMMANDS.filter((c) => c.label.toLowerCase().includes(qq) || c.hint.toLowerCase().includes(qq));
  el.paletteList.replaceChildren();
  pSel = Math.min(pSel, Math.max(0, m.length - 1));
  m.forEach((c, i) => {
    const li = document.createElement("li");
    li.setAttribute("role", "option");
    li.setAttribute("aria-selected", String(i === pSel));
    const lab = document.createElement("span");
    lab.textContent = c.label;
    const hint = document.createElement("span");
    hint.style.marginLeft = "auto";
    hint.style.color = "var(--fg-faint)";
    hint.style.fontSize = "11.5px";
    hint.textContent = c.hint;
    li.append(lab, hint);
    li.addEventListener("click", () => { c.run(); el.palette.close(); });
    el.paletteList.appendChild(li);
  });
  return m;
}

function showHint(text) {
  openPalette();
  el.paletteInput.value = "";
  const li = document.createElement("li");
  li.setAttribute("aria-selected", "true");
  li.style.fontStyle = "italic";
  li.textContent = text;
  el.paletteList.replaceChildren(li);
}

function openPalette() {
  pSel = 0;
  el.paletteInput.value = "";
  renderList("");
  el.palette.showModal();
  el.paletteInput.focus();
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
