/**
 * Tiny subsequence fuzzy scorer for command palette.
 *
 * Inspired by Sublime/VSCode heuristics. Not as sophisticated as fzf, but
 * correct for our scale (< 500 items) and adds < 1KB to the bundle. Returns
 * a score where higher = better, and null if there's no match at all.
 *
 * Score rationale:
 *   - Consecutive matches boost heavily (word-start bonus too).
 *   - Matches at word boundaries score higher than mid-word.
 *   - Earlier matches score higher than later ones.
 */

export function fuzzyScore(needle: string, haystack: string): number | null {
  if (needle === "") return 0;
  const n = needle.toLowerCase();
  const h = haystack.toLowerCase();
  let score = 0;
  let hi = 0;
  let streak = 0;
  let lastMatchIdx = -1;

  for (let ni = 0; ni < n.length; ni++) {
    const nc = n[ni]!;
    let found = -1;
    while (hi < h.length) {
      if (h[hi] === nc) {
        found = hi;
        break;
      }
      hi++;
    }
    if (found === -1) return null;

    // Boundary / word-start bonus.
    const prev = found > 0 ? h[found - 1] ?? "" : "";
    const isBoundary =
      found === 0 || prev === " " || prev === "-" || prev === "_" || prev === "/";
    if (isBoundary) score += 6;

    // Consecutive streak bonus.
    if (lastMatchIdx >= 0 && found === lastMatchIdx + 1) {
      streak++;
      score += 5 + streak * 2;
    } else {
      streak = 0;
    }

    // Small penalty for gap from start so earlier matches beat later ones.
    score += Math.max(0, 4 - found) * 2;

    // Case-sensitive exact match bonus (user typed the actual case).
    if (haystack[found] === needle[ni]) score += 1;

    lastMatchIdx = found;
    hi++;
  }

  // Normalize by haystack length so very long strings don't dominate.
  return score - Math.floor(haystack.length / 40);
}

export interface Scored<T> {
  item: T;
  score: number;
}

export function fuzzyFilter<T>(
  items: T[],
  query: string,
  extract: (it: T) => string,
): Scored<T>[] {
  if (!query.trim()) {
    return items.map((item) => ({ item, score: 0 }));
  }
  const scored: Scored<T>[] = [];
  for (const item of items) {
    const s = fuzzyScore(query, extract(item));
    if (s != null) scored.push({ item, score: s });
  }
  scored.sort((a, b) => b.score - a.score);
  return scored;
}
