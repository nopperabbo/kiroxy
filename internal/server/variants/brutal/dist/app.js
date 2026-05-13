// brutal variant — terminal dashboard app.
//
// Philosophy: information is the product. Keep this file small, keep
// DOM updates to textContent (no innerHTML, no framework), keep the
// hot loop predictable.
//
// Data surface: polls GET /dashboard/api/state every 2s. No SSE
// reconnect heuristics — the backend doesn't expose a stream today.
//
// See .sisyphus/plans/variant-brutal-manifesto.md for the full spec.

/** @typedef {{
 *   id: string,
 *   enabled: boolean,
 *   requests: number,
 *   errors: number,
 *   cooldown_until?: string,
 *   last_error?: string,
 * }} Acct */

/** @typedef {{
 *   version: string,
 *   uptime_s: number,
 *   ready: boolean,
 *   ready_detail?: string,
 *   vault_ok: boolean,
 *   vault_path?: string,
 *   accounts: Acct[],
 * }} Snap */

const POLL_MS = 2000;
const API = "/dashboard/api/state";

/** Known IDs from the previous tick — used to flash newly-arrived rows. */
const seenIds = new Set();

/** Reference DOM nodes once to avoid re-querying each tick. */
const el = {
  ver: document.querySelector("[data-ver]"),
  upt: document.querySelector("[data-upt]"),
  ready: document.querySelector("[data-ready-glyph]"),
  vault: document.querySelector("[data-vault-glyph]"),
  tbl: document.getElementById("tbl"),
  empty: document.getElementById("empty"),
  sysReady: document.querySelector("[data-sys-ready]"),
  sysVault: document.querySelector("[data-sys-vault]"),
  sysReq: document.querySelector("[data-sys-req]"),
  sysErr: document.querySelector("[data-sys-err]"),
  palette: /** @type {HTMLDialogElement} */ (document.getElementById("palette")),
  paletteInput: /** @type {HTMLInputElement} */ (document.getElementById("palette-input")),
  paletteList: document.getElementById("palette-list"),
  importDlg: /** @type {HTMLDialogElement} */ (document.getElementById("import")),
  helpDlg: /** @type {HTMLDialogElement} */ (document.getElementById("help")),
};

/**
 * Format seconds as a compact "1d 02h" style string.
 * @param {number} s
 */
function fmtUptime(s) {
  if (!Number.isFinite(s) || s < 0) return "-";
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d > 0) return `${d}d ${String(h).padStart(2, "0")}h`;
  if (h > 0) return `${h}h ${String(m).padStart(2, "0")}m`;
  return `${m}m`;
}

/**
 * Pad/truncate a string to N chars so the ASCII grid stays aligned.
 * @param {string} s
 * @param {number} n
 */
function col(s, n) {
  const v = s == null ? "-" : String(s);
  if (v.length > n) return v.slice(0, n - 1) + "…";
  return v.padEnd(n, " ");
}

/** Render one row into the table. */
function renderRow(a, isNew) {
  const row = document.createElement("div");
  row.className = "row";
  row.setAttribute("role", "row");
  if (isNew) row.dataset.new = "1";

  const glyph = a.enabled ? (a.errors > 0 ? "✗" : "✓") : "·";
  const state = a.cooldown_until
    ? `COOLDOWN ${a.cooldown_until}`
    : a.last_error
      ? `ERR ${a.last_error}`
      : a.enabled
        ? "OK"
        : "OFF";

  // Deliberately textContent-only on every span so we cannot accidentally
  // inject HTML from a hostile API response.
  const spans = [
    mkSpan(col(a.id, 30), a.enabled ? "" : "disabled"),
    mkSpan(glyph, "glyph"),
    mkSpan(String(a.requests ?? 0)),
    mkSpan(String(a.errors ?? 0), a.errors > 0 ? "err" : ""),
    mkSpan(state, a.cooldown_until || a.last_error ? "warn" : ""),
  ];
  for (const s of spans) row.appendChild(s);
  return row;
}

/** @param {string} text @param {string} [cls] */
function mkSpan(text, cls) {
  const s = document.createElement("span");
  if (cls) s.className = cls;
  s.textContent = text;
  return s;
}

/** @param {Snap} snap */
function render(snap) {
  el.ver.textContent = `VER ${snap.version || "?"}`;
  el.upt.textContent = `UPT ${fmtUptime(snap.uptime_s)}`;
  el.ready.textContent = snap.ready ? "✓" : "✗";
  el.vault.textContent = snap.vault_ok ? "✓" : "✗";
  el.sysReady.textContent = snap.ready ? "OK" : `DOWN ${snap.ready_detail || ""}`.trim();
  el.sysVault.textContent = snap.vault_ok ? "OK" : "DOWN";

  const totReq = (snap.accounts || []).reduce((n, a) => n + (a.requests || 0), 0);
  const totErr = (snap.accounts || []).reduce((n, a) => n + (a.errors || 0), 0);
  el.sysReq.textContent = String(totReq);
  el.sysErr.textContent = String(totErr);

  // Pool table rewrite. Rows are disposable — simpler than diffing.
  el.tbl.replaceChildren();
  const accts = snap.accounts || [];
  if (accts.length === 0) {
    el.empty.hidden = false;
  } else {
    el.empty.hidden = true;
    for (const a of accts) {
      const isNew = !seenIds.has(a.id);
      seenIds.add(a.id);
      el.tbl.appendChild(renderRow(a, isNew));
    }
    // Drop ids no longer present so re-added ones flash again.
    for (const id of seenIds) {
      if (!accts.some((a) => a.id === id)) seenIds.delete(id);
    }
  }
}

/** One poll tick. */
async function tick() {
  try {
    const r = await fetch(API, { headers: { accept: "application/json" } });
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    /** @type {Snap} */
    const snap = await r.json();
    render(snap);
  } catch (err) {
    // On failure: show the error in the meta line, don't wipe the table.
    el.upt.textContent = `UPT ? (fetch: ${(err && err.message) || err})`;
  }
}

/* ---- Command palette ---- */

/** @typedef {{id: string, label: string, run: () => void}} Cmd */

/** @type {Cmd[]} */
const COMMANDS = [
  { id: "refresh",   label: "refresh snapshot",                 run: () => tick() },
  { id: "import",    label: "show import instructions",          run: () => el.importDlg.showModal() },
  { id: "help",      label: "show keyboard shortcuts",           run: () => el.helpDlg.showModal() },
  { id: "classic",   label: "go to /dashboard (phase H htmx)",   run: () => (location.href = "/dashboard") },
  { id: "next",      label: "go to /dashboard-next (cyan min.)", run: () => (location.href = "/dashboard-next") },
  { id: "mansion",   label: "go to /dashboard-mansion (amber)",  run: () => (location.href = "/dashboard-mansion") },
];

let paletteSelected = 0;

function paletteRender(query) {
  const q = query.trim().toLowerCase();
  const matched = q === ""
    ? COMMANDS
    : COMMANDS.filter((c) => c.label.toLowerCase().includes(q) || c.id.includes(q));
  el.paletteList.replaceChildren();
  paletteSelected = Math.min(paletteSelected, Math.max(0, matched.length - 1));
  matched.forEach((c, i) => {
    const li = document.createElement("li");
    li.setAttribute("role", "option");
    li.setAttribute("aria-selected", String(i === paletteSelected));
    li.dataset.id = c.id;
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
  paletteSelected = 0;
  el.paletteInput.value = "";
  paletteRender("");
  el.palette.showModal();
  el.paletteInput.focus();
}

el.paletteInput.addEventListener("input", (e) => {
  paletteSelected = 0;
  paletteRender(/** @type {HTMLInputElement} */(e.target).value);
});
el.paletteInput.addEventListener("keydown", (e) => {
  const matched = paletteRender(el.paletteInput.value);
  if (e.key === "ArrowDown") {
    e.preventDefault();
    paletteSelected = Math.min(paletteSelected + 1, matched.length - 1);
    paletteRender(el.paletteInput.value);
  } else if (e.key === "ArrowUp") {
    e.preventDefault();
    paletteSelected = Math.max(paletteSelected - 1, 0);
    paletteRender(el.paletteInput.value);
  } else if (e.key === "Enter") {
    e.preventDefault();
    const pick = matched[paletteSelected];
    if (pick) {
      pick.run();
      el.palette.close();
    }
  }
});

/* ---- Global hotkeys (single-key — fits the aesthetic) ---- */
addEventListener("keydown", (e) => {
  const t = /** @type {HTMLElement} */ (e.target);
  const typing = t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable);
  if (typing) return;
  if (e.key === "k") { e.preventDefault(); openPalette(); }
  else if (e.key === "i") { e.preventDefault(); el.importDlg.showModal(); }
  else if (e.key === "r") { e.preventDefault(); tick(); }
  else if (e.key === "?") { e.preventDefault(); el.helpDlg.showModal(); }
});

/* ---- Kick off ---- */
tick();
setInterval(tick, POLL_MS);
