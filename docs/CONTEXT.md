# docs/ — design reference (factory)

These are design documents, not source of truth for the running code.
When a doc and the code disagree, the code wins — update the doc, don't
trust it blindly.

| File | Covers | Status |
|---|---|---|
| [ARCHITECTURE.md](ARCHITECTURE.md) | Overall system design, network model, anti-cheat | Live — rewritten 2026-09-17 to Go framing with inline callouts on every section that describes a designed-but-not-yet-built feature (attacks, level loading, logging, bans, auth, rate limiting). |
| [IMPLEMENTATION_NOTES.md](IMPLEMENTATION_NOTES.md) | Why Go was chosen, GC tuning, anti-cheat scoring design | Live — matches the code |
| [PROTOCOL_REFERENCE.md](PROTOCOL_REFERENCE.md) | Wire message shapes | Live — rewritten 2026-09-17 with real Go struct definitions from `shared/protocol.go` and a per-message "live / defined-not-wired-up" status. |
| [ANI_MAKER_SPEC.md](ANI_MAKER_SPEC.md) | Design spec for the `animaker` tool | Partially live — `animaker/` implements an early subset (editor shell, spritesheet/keyframe/timeline UI). Don't assume every feature described here exists; check `animaker/pkg/`. |

**Historical note:** `ARCHITECTURE.md` and `PROTOCOL_REFERENCE.md` originally
described the project as Rust-based with Rust-syntax code samples, left over
from an early draft before `IMPLEMENTATION_NOTES.md` settled on Go. Both have
been rewritten in place to Go and checked against the actual code in
`client/`, `server/`, `shared/` — every design element that doesn't have a
corresponding Go implementation yet is now called out inline instead of
implied. If you find another Rust-flavored leftover, treat it the same way:
fix the framing and note what's actually built vs. designed.
