---
title: Rule Title Here
impact: MEDIUM
impactDescription: Optional one-line impact (e.g., "keeps node kinds extensible")
tags: tag1, tag2
---

## Rule Title Here

Brief explanation of the rule and why it matters in Containerlab. Keep it to one to three sentences. The filename prefix (`cli`, `lifecycle`, `architecture`, `contracts`, `topology`, `go`, `tests`) selects the section in `_sections.md`.

**Incorrect (what's wrong):**

```go
// concrete-type dispatch, broken contract, or unsafe lifecycle code
```

**Correct (what's right):**

```go
// delegate to the owning abstraction / preserve the contract
```

Reference: `path/to/real_file.go` (optional anchor to real code; omit line numbers, they drift)
