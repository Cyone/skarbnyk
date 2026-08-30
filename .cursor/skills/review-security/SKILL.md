---
name: review-security
description: Review Skarbnyk changes with the Security Review subagent. Use when the user asks for a security review or runs /review-security.
---

# Review Security

Use this skill when the user asks to run `/review-security` or wants a security review of this repo.

Launch exactly one `security-review` subagent with:

- `run_in_background: false` unless explicitly asked to run in background
- `description: "Security Review"`
- `subagent_type: "security-review"`

The review subagent computes the local diff from the repository path. Do not compute the diff yourself. Use the active workspace root.

Do not provide `Base Branch` unless this branch was created from a branch other than the repo default.

If the user asks to review a specific PR or branch, check that target out locally first. If git refuses because of dirty files, explain the blocker and ask before stashing.

Use this exact prompt shape:

```text
Full Repository Path: <absolute repository path>
Diff: <one of: "branch changes", "uncommitted changes">
Base Branch: <only include this line when reviewing branch changes against a known specific base branch>
Custom Instructions: Skarbnyk: flag leaked RIA_API_KEY / TELEGRAM_BOT_TOKEN / DATABASE_URL (logs, templates, commits). Flag SQL built from request filters in internal/store. Flag RIA api_key in URLs written to logs or HTML. <append any extra user instructions>
```

Default to `branch changes`. If the user asks to review only uncommitted or dirty changes, use `uncommitted changes`.

If the subagent fails because the invocation was wrong, correct it and retry once. For any other failure, retry once with the same prompt. If it still fails, stop and report the blocker.

After it finishes:

- Empty diff: one sentence that there was nothing to review.
- No issues: one line such as "Security review found no issues".
- Findings: a markdown table sorted by severity — columns Severity, Location (`file:line`), Finding.

Do not fix findings or rerun unless the user asks.
