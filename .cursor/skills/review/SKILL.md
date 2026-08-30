---
name: review
description: Review Skarbnyk diffs for parser/score bugs, RIA budget, product scope, and missing tests. Use when the user asks for a code review, review of a diff, or a pull/merge request review.
---

# Review

Review the current changes in this repository. Inspect the relevant diff and surrounding code.

## Constraints

- Read-only. Do not modify files or delegate work to other agents.
- Prefer `git status`, `git diff`, `git log`, `git show`.
- Run `make test`. `go run ./cmd/skarbnyk -check` only if parse or templates changed.

## Checklist

- Parser (`internal/parse`): new regex, brand, room, or skip rule has a test. Skip rules were not dropped without evidence. Houses still land as `apt` unless the change is the house split.
- Score (`internal/score`): `Discount` stays `1 - start/market`; market `<= 0` or start `< 0` → `0`. Dutch in-auction ticks are not a markdown. `parse_confidence < 0.6` stays unscored.
- RIA: still cached by spec-hash, not per lot. No paid AI / period series. No extra RIA calls in a hot loop.
- Scope: no SETAM scrape, bidding, auth, SPA, new process, or new dependency.
- Secrets: `RIA_API_KEY`, Telegram token, `DATABASE_URL` not logged or committed.
- UI: HTMX + Ukrainian; no second frontend.

## Output

Prioritize actionable findings. Include file and line, impact, and a fix.

- **Critical**: must fix (wrong classify/score, leaked secret, data loss)
- **Warning**: should fix (missing test, RIA budget, scope creep)
- **Suggestion**: optional

If there are no findings, say so and mention only material residual testing risks.
