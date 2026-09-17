# Animaker editor

A standalone Fyne desktop app for authoring rigged, multi-part sprite
animations, entirely separate from the game's Go module.

## Why this shape

It is its own `module animaker` so it can be built, versioned, and run
without the game's raylib/networking dependencies.

**v2 rewrite landed 2026-09-18**, implementing `docs/ANI_MAKER_SPEC.md`'s
Track/Direction/Part/Keyframe rig model — a full replacement of the
earlier v1 flipbook editor, not an extension of it. See that spec for the
full design (why parts are free-form, why sheets use a fixed grid+pivot,
why props resolve to a sheet name, the two open questions it flags as
unresolved).

## Shape

- Entry point wires a dark editor theme into a Fyne app and delegates to
  `app.New(a).Run()`. `animaker/main.go`.
- `pkg/app/app.go` — application shell; owns the `Project`, wires every
  UI callback, runs the ~60fps playback ticker.
- `pkg/editor/` — domain model:
  - `track.go` — `Track`/`Direction`/`Part`/`Keyframe`/`PropDef`/
    `PropBinding` types (the rig itself).
  - `part.go` — add/remove Part/Direction/Prop helpers.
  - `keyframe.go` — add/delete/duplicate/move keyframes (kept sorted by
    `TimeMs`), and `Part.ValueAt(timeMs)` — the interpolation entry point
    (linear lerp on X/Y/Z/Rotation, step function on Row/Col).
  - `spritesheet.go` — `SpriteSheetTemplate`: fixed cell size + one pivot
    per sheet, `Cols()`/`Rows()` derived from image size, `CellImage`
    crops a `(row, col)`.
  - `project.go` — `Project`: current `Track`, loaded sheet templates,
    playback state, `PreviewProps` (editor-only prop overrides, never
    saved), selection state, and `ResolveActiveSheetName`/
    `ResolveActiveSheet` (the prop → sheet resolution the whole
    customization system hangs off).
  - `deepcopy.go`, `undo.go` — full-track-snapshot undo/redo.
- `pkg/ui/` — Fyne widgets:
  - `timeline.go` — one row per Part in the active direction; keyframes
    shown as time-labeled buttons per row, click to select, "+ here"
    inserts at the current playhead.
  - `canvas.go` — resolves every Part's transform at the current
    `ElapsedMs`, Z-sorts, draws Sheet parts as a cropped+pivoted cell.
    **Rotation is stored and saved but not visually applied here** — Fyne
    has no simple rotated-image primitive, and the actual consumer of
    rotation is a future game-side (raylib) renderer, not this preview.
    Nested-animation parts draw as a labeled placeholder box, not a
    recursively-rendered sub-animation — see [Known gaps](#known-gaps-not-bugs)
    below.
  - `properties.go` — props schema editor, preview-override entries, the
    active direction's part list, and the selected keyframe's transform
    fields + either a cell `(row, col)` picker (Sheet parts) or a
    per-prop binding editor (NestedAni parts).
  - `dialogs.go`, `window.go`, `theme.go` — mostly self-explanatory;
    `theme.go` is untouched by the v2 rewrite (pure color/theme, no
    dependency on the domain model).
- `pkg/file/` — persistence: `toml.go` (`SaveTrack`/`LoadTrack` for
  `.anif`, `SaveSheetTemplate`/`LoadSheetTemplate` for `.sprsh`),
  `image.go` (unchanged — generic image loading/cropping).

## Known gaps (not bugs)

Deliberate scope cuts from the v2 implementation pass, not oversights:

- **Nested-animation parts don't actually play their nested `.anif`** in
  the canvas preview — they render as a placeholder box. The data model
  (`Part.NestedAniPath`, `Part.NestedBindings`) is fully implemented and
  saved/loaded correctly; only the recursive-load-and-render step for
  the *preview* is missing.
- **Rotation isn't visually applied** in the canvas for the same reason
  (Fyne limitation) — see above. It's captured in every keyframe and
  round-trips through save/load correctly.
- **No sheet-cell thumbnail picker** — `(Row, Col)` is set via number
  entry, with a live single-cell preview image next to it, rather than a
  clickable grid of thumbnails.
- The two items `docs/ANI_MAKER_SPEC.md`'s own "Open questions" section
  flags (how one prop fans out to multiple physical sheets; the sword
  "bent state" mechanism) are exactly as unresolved in code as in that
  doc — nothing here should be read as having quietly decided them.

## Connected to

- Nothing in `client/`, `server/`, `shared/` imports `animaker/`, and
  `animaker/` imports nothing from them. They share no code.
- **Ghost link, unchanged by this rewrite:** no code on the game side
  reads a `.anif`/`.sprsh` file. `docs/ANI_MAKER_SPEC.md`'s "Runtime
  design guidance" section describes constraints a future game-side
  loader should follow, but that loader doesn't exist — building one is
  new integration work, not a bug fix.

## If you change this

- **Hits:** only files within `animaker/pkg/` and `animaker/main.go`.
- **Does not hit:** `client/`, `server/`, `shared/` — verified no shared
  imports either direction.
- Changing `editor.Track`'s shape hits `file/toml.go` (the TOML mirror
  structs are hand-kept-in-sync, not generated) and both `_test.go`
  files in `editor/`/`file/`.

## Surfaces

Desktop GUI app, run via `cd animaker && go build` (needs `CGO_ENABLED=1`
and a C compiler — see root `CONTEXT.md`'s Build section) or via
`build_local.ps1`, which finds a compiler automatically. Human-facing
(artists/animators), unlike the rest of this map.

## Tests

`pkg/editor/track_test.go` — keyframe interpolation (including the
step-function Row/Col behavior and clamping before/after the keyframe
range), sorted-insert invariants, prop-resolution precedence
(PreviewProps override > prop default > FixedSheet), deep-copy
independence. `pkg/file/toml_test.go` — full save/load round-trip for
both `.anif` (props, all three part-authoring cases: sheet+prop-governed,
sheet+fixed, nested-with-bindings) and `.sprsh`. `pkg/ui` and
`pkg/app` have no automated tests — GUI wiring, verified by manual launch
only.

## See

`animaker/main.go`, `animaker/pkg/`
