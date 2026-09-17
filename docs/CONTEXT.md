# docs/ — design reference (factory)

These are design documents, not source of truth for the running code.
When a doc and the code disagree, the code wins — update the doc, don't
trust it blindly.

| File | Covers | Status |
|---|---|---|
| [ARCHITECTURE.md](ARCHITECTURE.md) | Overall system design, network model, anti-cheat | Live — rewritten 2026-09-17 to Go framing with inline callouts on every section that describes a designed-but-not-yet-built feature (attacks, level loading, logging, bans, auth, rate limiting); tick-rate numbers updated same day when `shared.NetworkTickRate` changed from 100ms to 50ms. |
| [IMPLEMENTATION_NOTES.md](IMPLEMENTATION_NOTES.md) | Why Go was chosen, GC tuning, anti-cheat scoring design | Live — matches the code; tick-rate/bandwidth numbers updated 2026-09-17 (see below). Performance figures elsewhere in this doc are unmeasured targets, not benchmarks. |
| [PROTOCOL_REFERENCE.md](PROTOCOL_REFERENCE.md) | Wire message shapes | Live — rewritten 2026-09-17 with real Go struct definitions from `shared/protocol.go` and a per-message "live / defined-not-wired-up" status; tick-rate numbers updated same day. |
| [ANI_MAKER_SPEC.md](ANI_MAKER_SPEC.md) | Design spec for the `animaker` tool | **Live as of 2026-09-18** — the v2 rig/props model it describes is implemented in `animaker/pkg/editor` and `pkg/ui`. Known, deliberate gaps (nested-animation preview, visual rotation, cell thumbnail picker) are listed in `map/objects/animaker.md`'s "Known gaps" section, not in this doc. The spec's own "Open questions" section (prop → multi-sheet fan-out, sword bent-state) is still genuinely unresolved in code, not just in the doc. |

**2026-09-17 latency fix:** the client's send interval, the server's
broadcast interval, and the client's remote-player interpolation window
were three independently hardcoded `100`s that had drifted apart from
`shared.NetworkTickRate` (which nothing read). Stacked together they
added ~150-300ms of perceived latency to a remote player's rendered
position, fully reproducible on localhost since none of it was real
network delay. Fixed by wiring all three to `shared.NetworkTickRate` and
halving it to 50ms (20Hz) — see `map/objects/client-prediction.md` and
`map/objects/server-anticheat.md` for the code-level detail.

**Historical note:** `ARCHITECTURE.md` and `PROTOCOL_REFERENCE.md` originally
described the project as Rust-based with Rust-syntax code samples, left over
from an early draft before `IMPLEMENTATION_NOTES.md` settled on Go. Both have
been rewritten in place to Go and checked against the actual code in
`client/`, `server/`, `shared/` — every design element that doesn't have a
corresponding Go implementation yet is now called out inline instead of
implied. If you find another Rust-flavored leftover, treat it the same way:
fix the framing and note what's actually built vs. designed.
