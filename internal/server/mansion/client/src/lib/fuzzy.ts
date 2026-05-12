/**
 * Tiny fuzzy scorer. Case-insensitive subsequence match with a bonus for
 * consecutive character matches and match starts on word boundaries.
 *
 * Not an algorithmic marvel — the purpose is to rank command-palette
 * items against short queries. For dashboard scale (dozens of items,
 * not millions) O(n·m) is fine.
 */

export interface Scored<T> {
  item: T;
  score: number;
}

export function fuzzyScore(query: string, haystack: string): number {
  if (!query) return 0;
  const q = query.toLowerCase();
  const h = haystack.toLowerCase();
  let qi = 0;
  let score = 0;
  let streak = 0;
  let lastMatchIdx = -1;
  for (let i = 0; i < h.length && qi < q.length; i++) {
    if (h[i] === q[qi]) {
      let add = 8;
      if (i === 0 || isBoundary(h[i - 1])) add += 6;
      if (lastMatchIdx === i - 1) {
        streak += 1;
        add += streak * 3;
      } else {
        streak = 0;
      }
      score += add;
      lastMatchIdx = i;
      qi += 1;
    }
  }
  if (qi < q.length) return 0;
  // Prefer shorter haystacks when scores tie.
  score -= h.length * 0.05;
  return score;
}

function isBoundary(ch: string): boolean {
  return ch === " " || ch === "/" || ch === "-" || ch === "_" || ch === ".";
}

export function filterAndRank<T>(query: string, items: T[], key: (t: T) => string): Scored<T>[] {
  if (!query.trim()) return items.map((item) => ({ item, score: 0 }));
  const out: Scored<T>[] = [];
  for (const item of items) {
    const s = fuzzyScore(query, key(item));
    if (s > 0) out.push({ item, score: s });
  }
  out.sort((a, b) => b.score - a.score);
  return out;
}
