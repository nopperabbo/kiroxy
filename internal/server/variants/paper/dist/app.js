// paper variant — polls /dashboard/api/state and renders a report-style
// snapshot. Minimal JS; table rows rewritten each tick; no framework.
// See .sisyphus/plans/variant-paper-manifesto.md.

const POLL_MS = 2000;
const API = "/dashboard/api/state";

const el = {
  ver: document.querySelector("[data-ver]"),
  upt: document.querySelector("[data-upt]"),
  ready: document.querySelector("[data-ready]"),
  vault: document.querySelector("[data-vault]"),
  totreq: document.querySelector("[data-totreq]"),
  toterr: document.querySelector("[data-toterr]"),
  stamp: document.querySelector("[data-stamp]"),
  byline: document.querySelector("[data-byline]"),
  tbody: document.getElementById("tbody"),
  palette: /** @type {HTMLDialogElement} */ (document.getElementById("palette")),
  paletteInput: /** @type {HTMLInputElement} */ (document.getElementById("palette-input")),
  paletteList: document.getElementById("palette-list"),
  paletteOpen: document.querySelector("[data-palette-open]"),
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

function setDd(node, text, mod) {
  node.textContent = text;
  node.classList.remove("ok", "bad");
  if (mod === "ok") node.classList.add("ok");
  else if (mod === "bad") node.classList.add("bad");
}

function pillFor(a) {
  if (a.cooldown_until) return { cls: "cooldown", text: "cooldown" };
  if (a.last_error) return { cls: "err", text: "error" };
  if (!a.enabled) return { cls: "off", text: "paused" };
  return { cls: "ok", text: "active" };
}

function renderRow(a) {
  const tr = document.createElement("tr");
  if (!seen.has(a.id)) {
    tr.dataset.new = "1";
    seen.add(a.id);
  }

  const idTd = document.createElement("td");
  idTd.className = "id";
  idTd.textContent = a.id;

  const stateTd = document.createElement("td");
  stateTd.className = "state";
  const p = pillFor(a);
  const span = document.createElement("span");
  span.className = `pill ${p.cls}`;
  span.textContent = p.text;
  stateTd.appendChild(span);

  const reqTd = document.createElement("td");
  reqTd.className = "n";
  reqTd.textContent = String(a.requests ?? 0);

  const errTd = document.createElement("td");
  errTd.className = "n";
  errTd.textContent = String(a.errors ?? 0);

  const noteTd = document.createElement("td");
  noteTd.className = "note";
  if (a.cooldown_until) noteTd.textContent = `cooling until ${a.cooldown_until}`;
  else if (a.last_error) noteTd.textContent = a.last_error;
  else if (!a.enabled) noteTd.textContent = "manually paused";
  else noteTd.textContent = "—";

  tr.append(idTd, stateTd, reqTd, errTd, noteTd);
  return tr;
}

async function tick() {
  try {
    const r = await fetch(API, { headers: { accept: "application/json" } });
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    const snap = await r.json();
    render(snap);
  } catch (err) {
    el.stamp.textContent = `—  (fetch error: ${(err && err.message) || err})`;
  }
}

function render(snap) {
  const now = new Date();
  const hh = String(now.getHours()).padStart(2, "0");
  const mm = String(now.getMinutes()).padStart(2, "0");
  const ss = String(now.getSeconds()).padStart(2, "0");
  el.stamp.textContent = `${hh}:${mm}:${ss} local`;

  setDd(el.ver, snap.version || "—");
  setDd(el.upt, fmtUptime(snap.uptime_s));
  setDd(el.ready, snap.ready ? "ready" : `down${snap.ready_detail ? " · " + snap.ready_detail : ""}`,
        snap.ready ? "ok" : "bad");
  setDd(el.vault, snap.vault_ok ? "open" : "sealed", snap.vault_ok ? "ok" : "bad");

  const accts = snap.accounts || [];
  const totReq = accts.reduce((n, a) => n + (a.requests || 0), 0);
  const totErr = accts.reduce((n, a) => n + (a.errors || 0), 0);
  setDd(el.totreq, totReq.toLocaleString());
  setDd(el.toterr, totErr.toLocaleString(), totErr > 0 ? "bad" : "");

  el.byline.textContent = accts.length === 0
    ? "The vault is empty. Import accounts below to begin."
    : `${accts.length} account${accts.length === 1 ? "" : "s"} — ${totReq.toLocaleString()} request${totReq === 1 ? "" : "s"} served since start.`;

  el.tbody.replaceChildren();
  if (accts.length === 0) {
    const tr = document.createElement("tr");
    tr.className = "empty";
    const td = document.createElement("td");
    td.colSpan = 5;
    const em = document.createElement("em");
    em.textContent = "No accounts yet. See the Import section below.";
    td.appendChild(em);
    tr.appendChild(td);
    el.tbody.appendChild(tr);
  } else {
    const present = new Set(accts.map((a) => a.id));
    for (const id of seen) if (!present.has(id)) seen.delete(id);
    for (const a of accts) el.tbody.appendChild(renderRow(a));
  }
}

/* ---- Palette ---- */
/** @type {{id: string, label: string, run: () => void}[]} */
const COMMANDS = [
  { id: "refresh", label: "Refresh snapshot", run: () => tick() },
  { id: "jump-pool", label: "Jump to Pool", run: () => document.getElementById("pool").scrollIntoView({ behavior: "smooth" }) },
  { id: "jump-health", label: "Jump to Health", run: () => document.getElementById("health").scrollIntoView({ behavior: "smooth" }) },
  { id: "jump-import", label: "Jump to Import", run: () => document.getElementById("import").scrollIntoView({ behavior: "smooth" }) },
  { id: "variant-classic", label: "Switch to /dashboard (classic)", run: () => (location.href = "/dashboard") },
  { id: "variant-next", label: "Switch to /dashboard-next", run: () => (location.href = "/dashboard-next") },
  { id: "variant-mansion", label: "Switch to /dashboard-mansion", run: () => (location.href = "/dashboard-mansion") },
  { id: "variant-brutal", label: "Switch to /dashboard-brutal", run: () => (location.href = "/dashboard-brutal") },
];
let pSel = 0;

function renderPalette(q) {
  const qq = q.trim().toLowerCase();
  const matched = qq === "" ? COMMANDS : COMMANDS.filter((c) => c.label.toLowerCase().includes(qq));
  el.paletteList.replaceChildren();
  pSel = Math.min(pSel, Math.max(0, matched.length - 1));
  matched.forEach((c, i) => {
    const li = document.createElement("li");
    li.setAttribute("role", "option");
    li.setAttribute("aria-selected", String(i === pSel));
    li.textContent = c.label;
    li.addEventListener("click", () => {
      c.run();
      el.palette.close();
    });
    el.paletteList.appendChild(li);
  });
  return matched;
}

function openPalette() {
  pSel = 0;
  el.paletteInput.value = "";
  renderPalette("");
  el.palette.showModal();
  el.paletteInput.focus();
}

el.paletteOpen.addEventListener("click", (e) => { e.preventDefault(); openPalette(); });
el.paletteInput.addEventListener("input", (e) => {
  pSel = 0;
  renderPalette(e.target.value);
});
el.paletteInput.addEventListener("keydown", (e) => {
  const matched = renderPalette(el.paletteInput.value);
  if (e.key === "ArrowDown") {
    e.preventDefault();
    pSel = Math.min(pSel + 1, matched.length - 1);
    renderPalette(el.paletteInput.value);
  } else if (e.key === "ArrowUp") {
    e.preventDefault();
    pSel = Math.max(pSel - 1, 0);
    renderPalette(el.paletteInput.value);
  } else if (e.key === "Enter") {
    e.preventDefault();
    const pick = matched[pSel];
    if (pick) { pick.run(); el.palette.close(); }
  }
});

addEventListener("keydown", (e) => {
  if ((e.metaKey || e.ctrlKey) && e.key === "k") {
    e.preventDefault();
    openPalette();
  }
});

/* Kick off */
tick();
setInterval(tick, POLL_MS);
