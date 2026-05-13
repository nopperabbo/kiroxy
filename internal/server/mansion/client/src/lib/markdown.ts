/**
 * Tiny, dependency-free markdown-to-HTML pass tuned for the
 * Command Palette preview pane.
 *
 * We don't ship a full CommonMark implementation: the palette preview
 * is a tight 420-px column that shows a subset of each doc at a time.
 * Readable fidelity is the bar, not perfect rendering. Features:
 *
 *   - Headings  (# .. ######)
 *   - Paragraphs (blank-line separated)
 *   - Fenced code blocks (```)  — preserve indentation, no syntax highlight
 *   - Inline code   `like this`
 *   - Bold **x** and italic *x*
 *   - Links [text](url)
 *   - Unordered lists (- / * at line start)
 *   - Ordered lists (1. 2. …)
 *   - Blockquotes (> prefix)
 *   - Horizontal rules (--- on its own line)
 *
 * Escape-first: everything inline is HTML-escaped before inline spans
 * are injected. The output is safe to set via innerHTML because we
 * never allow raw HTML through — any literal < or > in the markdown
 * comes out as &lt;/&gt;.
 *
 * Out of scope (would be noise in a 420-px column):
 *   - Tables  — collapsed to preformatted text
 *   - Images  — rendered as their alt-text
 *   - Nested  lists beyond one level
 *   - Footnotes
 *
 * The renderer emits BEM-ish class names (.md-h1, .md-code, etc.) so
 * the palette can style them without leaking defaults into surrounding
 * components.
 */

/** Inline transformations applied to a single run of text. Order is
 * careful: code spans are extracted first so that ** inside them is
 * not interpreted as bold. */
const STASH_OPEN = String.fromCharCode(0xe000); // private-use start
const STASH_CLOSE = String.fromCharCode(0xe001);
const STASH_RE = new RegExp(`${STASH_OPEN}(\\d+)${STASH_CLOSE}`, "g");

function renderInline(s: string): string {
  const codes: string[] = [];
  let staged = escapeHtml(s).replace(/`([^`]+)`/g, (_m, c: string) => {
    codes.push(c);
    return `${STASH_OPEN}${codes.length - 1}${STASH_CLOSE}`;
  });

  staged = staged.replace(
    /\[([^\]]+)\]\(([^)\s]+)\)/g,
    (_m, text: string, url: string) =>
      `<a class="md-a" href="${escapeAttr(url)}" target="_blank" rel="noopener noreferrer">${text}</a>`,
  );

  staged = staged.replace(/\*\*([^*]+)\*\*/g, '<strong class="md-strong">$1</strong>');
  staged = staged.replace(/(^|[^*])\*([^*\n]+)\*/g, '$1<em class="md-em">$2</em>');

  staged = staged.replace(STASH_RE, (_m: string, idx: string) => {
    const i = parseInt(idx, 10);
    return `<code class="md-code">${codes[i]}</code>`;
  });

  return staged;
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function escapeAttr(s: string): string {
  // URLs get a stricter whitelist: allow http(s), mailto, /-relative, and
  // anchor-relative. Anything else collapses to "#" so we never emit a
  // javascript: link, even if the source doc contains one.
  if (/^(https?:\/\/|mailto:|\/|#|\.\/)/i.test(s)) {
    return escapeHtml(s);
  }
  return "#";
}

export function renderMarkdown(src: string): string {
  const lines = src.split(/\r?\n/);
  const out: string[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    // Fenced code block — find the closing fence verbatim, no inline
    // rendering inside.
    if (/^```/.test(line)) {
      const startLang = line.replace(/^```/, "").trim();
      const buf: string[] = [];
      i += 1;
      while (i < lines.length && !/^```/.test(lines[i])) {
        buf.push(lines[i]);
        i += 1;
      }
      i += 1; // consume closing fence (or run off end — acceptable)
      const langClass = startLang ? ` data-lang="${escapeAttr(startLang)}"` : "";
      out.push(`<pre class="md-pre"${langClass}><code>${escapeHtml(buf.join("\n"))}</code></pre>`);
      continue;
    }

    // Horizontal rule.
    if (/^\s*-{3,}\s*$/.test(line) || /^\s*\*{3,}\s*$/.test(line)) {
      out.push('<hr class="md-hr" />');
      i += 1;
      continue;
    }

    // Headings (#, ##, …).
    const h = /^(#{1,6})\s+(.*)$/.exec(line);
    if (h) {
      const level = h[1].length;
      out.push(`<h${level} class="md-h md-h${level}">${renderInline(h[2].trim())}</h${level}>`);
      i += 1;
      continue;
    }

    // Blockquote — collect consecutive > lines.
    if (/^>/.test(line)) {
      const buf: string[] = [];
      while (i < lines.length && /^>/.test(lines[i])) {
        buf.push(lines[i].replace(/^>\s?/, ""));
        i += 1;
      }
      out.push(`<blockquote class="md-quote">${renderInline(buf.join(" "))}</blockquote>`);
      continue;
    }

    // Ordered list.
    if (/^\s*\d+\.\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^\s*\d+\.\s+/, ""));
        i += 1;
      }
      out.push(
        `<ol class="md-ol">${items.map((t) => `<li>${renderInline(t)}</li>`).join("")}</ol>`,
      );
      continue;
    }

    // Unordered list.
    if (/^\s*[-*]\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\s*[-*]\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^\s*[-*]\s+/, ""));
        i += 1;
      }
      out.push(
        `<ul class="md-ul">${items.map((t) => `<li>${renderInline(t)}</li>`).join("")}</ul>`,
      );
      continue;
    }

    // Blank line — paragraph break (nothing emitted, just consumed).
    if (line.trim() === "") {
      i += 1;
      continue;
    }

    // Paragraph — accumulate until blank line or a block-level marker.
    const buf: string[] = [];
    while (
      i < lines.length &&
      lines[i].trim() !== "" &&
      !/^```/.test(lines[i]) &&
      !/^(#{1,6})\s+/.test(lines[i]) &&
      !/^>/.test(lines[i]) &&
      !/^\s*[-*]\s+/.test(lines[i]) &&
      !/^\s*\d+\.\s+/.test(lines[i]) &&
      !/^\s*-{3,}\s*$/.test(lines[i])
    ) {
      buf.push(lines[i]);
      i += 1;
    }
    out.push(`<p class="md-p">${renderInline(buf.join(" "))}</p>`);
  }

  return out.join("\n");
}
