# timeline

Vertical chronological list of events. Home for request-lifecycle traces, token-refresh histories, account audit logs. Dense, subgrid-aligned, monospaced timestamps.

**Pattern inheritance:** PlanetScale's deploy-request activity feed, GitHub's PR timeline, Linear's activity stream, Stripe's Workbench event list.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §3.3 (mono for data), §4.1 (4px grid), `docs/components/live-request-stream-block.md` (sibling primitive for "live" case).

---

## Anatomy

```
<Timeline>
  ├── [optional] <TimelineHeader>          ← label + range filter
  ├── <TimelineList>                       ← role="list" or <ol>
  │   └── <TimelineEvent>*
  │       ├── <TimelineRail>               ← vertical line; dot at event
  │       ├── <TimelineDot>                ← colored by data-intent
  │       ├── <TimelineTime>               ← mono; relative + absolute tooltip
  │       ├── <TimelineTitle>              ← one-liner; mono for IDs
  │       └── [optional] <TimelineBody>    ← expandable detail (Drawer-lite)
  └── [optional] <TimelineLoadMore>        ← for paginated history
</Timeline>
```

Markup template:

```html
<ol class="kx-timeline" role="list" aria-label="Refresh events">
  <li class="kx-timeline__event" data-intent="success">
    <div class="kx-timeline__rail" aria-hidden="true">
      <span class="kx-timeline__dot" data-intent="success"></span>
    </div>
    <time class="kx-timeline__time mono" datetime="2026-05-13T12:42:18Z">
      <span class="kx-timeline__relative">2m ago</span>
    </time>
    <div class="kx-timeline__content">
      <p class="kx-timeline__title">
        Token refreshed
        <code class="mono">acct_01H8XJK9M2</code> · expires in
        <time datetime="PT59M" class="mono">59m</time>
      </p>
    </div>
  </li>
  <li class="kx-timeline__event" data-intent="danger">
    <div class="kx-timeline__rail" aria-hidden="true">
      <span class="kx-timeline__dot" data-intent="danger"></span>
    </div>
    <time class="kx-timeline__time mono" datetime="2026-05-13T12:38:45Z">5m ago</time>
    <div class="kx-timeline__content">
      <p class="kx-timeline__title">
        Upstream returned <code class="mono">502</code> — retrying…
      </p>
    </div>
  </li>
</ol>
```

---

## API

| Attribute (wrapper) | Values | Default | Description |
|---|---|---|---|
| `data-density` | (inherited) | `comfortable` | Event row height 32px (comfortable) / 24px (compact) |
| `data-direction` | `newest-first` \| `oldest-first` | `newest-first` | Affects default scroll position |
| `data-loading` | `false` \| `true` | `false` | Shows skeleton events at top/bottom |

| Attribute (event) | Values | Default | Description |
|---|---|---|---|
| `data-intent` | `success` \| `warning` \| `danger` \| `info` \| `neutral` | `neutral` | Dot color |
| `data-expanded` | `true` \| `false` | `false` | Whether `TimelineBody` is visible |

---

## Variants

- **`default`** — one-line events; dot + time + title.
- **`expandable`** — events with `TimelineBody` drilling into raw payload or error detail. `Enter` expands; `Esc` collapses.
- **`grouped`** — events grouped by day or logical unit (e.g. "Today", "Yesterday", "Last week"). Group labels get their own row with subdued styling.

---

## States

| State | Trigger | Visual |
|---|---|---|
| Idle | default | Event row; dot; time; title |
| Hover | pointer over | Background `--color-elevated` across full width |
| Focus-visible | keyboard focus | Ring on event row |
| Expanded | `Enter` | Body visible below title; dot upgrades to filled chevron |
| Loading | `data-loading="true"` | Skeleton rows at the correct end (top if `newest-first`) |
| Empty | no events | Centered prose: "No refresh events in the last 7 days." |

---

## Accessibility

- Wrapper is `<ol>` or `role="list"`.
- Each event is `<li>` or `role="listitem"`.
- Expandable events upgrade to `<button>` or `role="button"` + `aria-expanded`.
- Dot carries `aria-hidden="true"` (decorative); intent is announced via the title text ("Upstream returned 502").
- `<time datetime="…">` for every timestamp — both the relative label AND a full datetime attribute.

**Keyboard:**
- `j` / `k` / `ArrowDown` / `ArrowUp` — navigate events.
- `Enter` — expand/collapse event body (if expandable).
- `Home` / `End` — first / last event.

---

## Motion

| Transition | Token | Notes |
|---|---|---|
| Expand body | `--dur-quick` | Height auto via `interpolate-size` + `allow-discrete` (Chrome 129+); fall back to instant |
| New event arrives (live) | `--dur-moderate` | Fade + 4px translateY via `@starting-style`; dot pulses 1x at 600ms via `@property` |
| Load-more fetch | `--dur-quick` | Skeleton replacement fade |

---

## Composition

**Contains:** `TimelineHeader`, `TimelineEvent`, `TimelineDot`, `TimelineTime`, `TimelineBody`.
**Contained by:** `Drawer` (account drill-down), `Dialog` body (request lifecycle), route view (Logs page).
**Paired with:** `RelativeTimeCard` (time column), `CopyableValue` (for IDs in titles), `StatusPill` (for terminal event status).

---

## Anti-patterns

- ❌ **Horizontal timeline.** Reads badly, hard to scan dozens of events. Vertical only.
- ❌ **Large iconography per event.** Dots only; keep visual density low.
- ❌ **Time as relative-only** (no `datetime` attribute). SRs need the full timestamp; so does anyone debugging across timezones.
- ❌ **Expansion via separate "Details" button.** Enter on the row expands; click on the row expands. No secondary affordance needed.
- ❌ **Animated dot pulses on every event.** Only the newly-arrived event (live append) gets a single pulse.
- ❌ **Unbounded scroll.** After 200 events, paginate or virtualize — CLS risk on load.

---

## Reference

- **PlanetScale** deploy-request activity.
- **GitHub** PR timeline.
- **Linear** activity stream.

---

## Example usage

**Refresh history (expandable for failures):**

```html
<ol class="kx-timeline" role="list" aria-label="Refresh events">
  <li class="kx-timeline__event" data-intent="success" data-expandable="false">
    <div class="kx-timeline__rail" aria-hidden="true">
      <span class="kx-timeline__dot" data-intent="success"></span>
    </div>
    <time class="kx-timeline__time mono" datetime="2026-05-13T12:42:18Z">2m ago</time>
    <p class="kx-timeline__title">
      Refreshed <code class="mono">acct_01H8XJK9M2</code> · new token expires in <time datetime="PT59M" class="mono">59m</time>
    </p>
  </li>
  <li class="kx-timeline__event" data-intent="danger" data-expandable="true" data-expanded="false">
    <button type="button" class="kx-timeline__toggle" aria-expanded="false" aria-controls="ev-3-body">
      <div class="kx-timeline__rail" aria-hidden="true">
        <span class="kx-timeline__dot" data-intent="danger"></span>
      </div>
      <time class="kx-timeline__time mono" datetime="2026-05-13T12:38:45Z">5m ago</time>
      <p class="kx-timeline__title">Refresh failed — upstream 502</p>
    </button>
    <section id="ev-3-body" class="kx-timeline__body" hidden>
      <pre class="mono"><code>POST https://prod.us-east-1.auth.desktop.kiro.dev/refreshToken
HTTP/1.1 502 Bad Gateway
…</code></pre>
    </section>
  </li>
</ol>
```
