<!--
Thanks for opening a PR. Please complete the checklist below before requesting review.
For non-trivial changes, please open an issue first to discuss the approach.
-->

## What

<!-- Describe the change in 1-2 sentences. -->

## Why

<!-- What problem does this solve? Link any related issue: "Closes #123" -->

## How

<!-- Walk through the implementation. Highlight any non-obvious decisions. -->

## Verification

<!-- How did you verify this works? -->

- [ ] `go test ./...` passes
- [ ] `go vet ./...` clean
- [ ] `gofmt -d .` clean
- [ ] Mansion: `pnpm exec svelte-check` reports 0 errors (if dashboard touched)
- [ ] Mansion: `pnpm run build` clean (if dashboard touched)
- [ ] Manual smoke test against local kiroxy instance
- [ ] Tested at iPhone 13 viewport (390px) — if frontend touched
- [ ] Light + dark theme both verified — if visual change

## Scope check

<!-- See CONTRIBUTING.md "Project Philosophy" -->

- [ ] This is a single-user, self-hosted enhancement (not multi-tenant / multi-provider / SaaS)
- [ ] No new dependencies added — OR — dependency was discussed and approved in the linked issue
- [ ] No build artifacts (`dist/`, `node_modules/`, binaries) committed
- [ ] Brand identity preserved (warm charcoal + amber, JetBrains Mono + Inter, hairline borders)
- [ ] Amber budget respected (5 roles only) — if visual change
- [ ] Motion budget respected (4 ambient idle-loop animations max) — if animation change

## Linked issues

<!-- Closes #N or relates to #N -->
