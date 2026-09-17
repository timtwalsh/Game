# map/ — code map for game + animaker

Catalog of what exists in `client/`, `server/`, `shared/`, `animaker/pkg/`
and what changing each thing hits. Read [CONTEXT.md](CONTEXT.md) first for
how to walk this and what "live/leftover/ghost" mean here.

## Routing

| Noun cluster | Card |
|---|---|
| Wire protocol & world model (shared by client+server) | [objects/protocol.md](objects/protocol.md) |
| Client-side prediction, interpolation, rendering | [objects/client-prediction.md](objects/client-prediction.md) |
| Server authority, movement validation, anti-cheat | [objects/server-anticheat.md](objects/server-anticheat.md) |
| Animaker editor (separate tool) | [objects/animaker.md](objects/animaker.md) |

Full one-line index: [objects/_index.md](objects/_index.md).

Planning a change? Start at [effects/CONTEXT.md](effects/CONTEXT.md) — it
says what else a change to a given file/type hits.

This map has no `processes/` yet — the movements worth documenting
(move-validate-broadcast, predict-correct-interpolate) are covered inline
on the object cards below; split them out only once a third real process
shows up.
