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

**UI reworked twice more the same day**, both times from hands-on
feedback after using the previous version:

1. First rework: replaced the original form-heavy layout (modal dialogs,
   numeric row/col entry) with a level-editor interaction — pick a part
   from a dropdown, link it to a prop right there, drag a cell off that
   part's sheet (a full tile grid) onto the canvas to create/move a
   keyframe. See `pkg/ui/sheetgrid.go` and `Application.onTileDropped`.
2. Second rework, on top of the first: the animation got an explicit
   `0,0`-to-`(CanvasWidth, CanvasHeight)` working area (`Track.CanvasWidth`/
   `CanvasHeight`); directions became plain ints (`0`=up, `1`=right,
   `2`=down, `3`=left by convention, but any int works) instead of
   free-form strings; the canvas itself became directly interactive
   (click a part to select it, drag an *existing* placed part to move an
   already-created keyframe — separate from dragging a *new* tile in from
   the sheet grid); the timeline became an actual scrubbable, per-part
   ruler (`pkg/ui/timeline.go`'s `scrubArea`) instead of a row of buttons,
   with explicit Play/Stop (not one toggle) and New/Delete Keyframe
   actions.
3. Third rework (still 2026-09-18): the standalone Rig menu (Add
   Direction/Prop/Part) was removed in favor of buttons in the panels
   each action actually affects, and the properties panel's sections were
   split into resizable `VSplit` panes instead of one long scrolling
   column, since the sheet grid needed real room.
4. Fourth rework (2026-09-19), on explicit reference to GraalOnline's
   GANI/GraalShop animation editor
   (https://graalonline.net/index.php/Creation/Dev/Gani/Tutorial — the
   same problem domain: 2D top-down multi-part sprite rigs): the sheet
   grid moved out of the right panel entirely into its own persistent
   **left column** (`PropertiesPanel.BuildPalette`), matching GraalShop's
   "Sprite Book"; the direction bar moved from a separate top strip into
   a header embedded in the right column; and the selected keyframe
   gained a nudge D-pad (X/Y ±1, Z forward/back, rotation ±5°) alongside
   the existing numeric entries, matching GraalShop's arrow-button
   nudging. `GANI`'s `setbackto` field (auto-chaining one animation into
   another on completion) was considered and explicitly **not** adopted
   — see the session's design discussion: it would duplicate the
   character controller's state machine's authority over transitions,
   which are usually context-dependent (input/health/combo state) in a
   way a static per-asset field can't express without asset duplication.
5. Import-flow fix (2026-09-19), from a user bug report: "I click import
   sprite sheet, enter the details click import then nothing happens and
   im back on the main screen". The import genuinely worked — it decoded
   the image, filled `Project.LoadedSheets`, and wrote the `.sprsh`
   sidecar — but nothing *visible* changed, because the palette only
   draws the **selected part's** sheet and a fresh track has no parts.
   The only escape was "+ Add Part" plus typing the sheet's name into a
   free-text "Fixed Sheet" box from memory. Fixed by closing both halves
   of the gap: importing now also creates a Sheet part bound to the new
   sheet and selects it (`Application.addPartForSheet`, a normal undoable
   edit), so the tiles appear immediately; and every place that names a
   sheet is now a pick-list of `Project.LoadedSheetNames()` instead of
   free text, since a typo there produced a part that silently drew
   nothing. The lesson generalizes: in this editor almost everything is
   gated on a *selected part*, so any action that doesn't end with a part
   selected reads to the artist as "nothing happened".
6. Four fixes from hands-on use (2026-09-19), all reported together:
   - **A 4x3 sheet appeared to slice into one sprite.** The slicing was
     correct (now pinned by `pkg/editor/spritesheet_test.go`); the
     palette drew cells at a hardcoded 2x, so a sheet of large cells
     overflowed the narrow left column and only the first was visible.
     `SheetGridWidget` now fits its cells to the width the layout gives
     it (`displayScale`, clamped to 0.25x-4x) and outlines each cell, and
     the palette header spells out the resulting grid ("4x3 cells of
     32x32px") — bad Cell Width/Height at import is the easiest mistake
     to make here and was otherwise invisible.
   - **The timeline couldn't scrub past the first keyframe**, so a second
     one could never be added. `Project.Seek` clamped to
     `TotalDurationMs`, which is 0 when the only keyframe is at 0ms.
     There is now a separate `Direction.EditableDurationMs` — always
     ahead of the last keyframe — that `Seek` and the ruler use, while
     playback still loops over the real `TotalDurationMs`.
   - **New tracks seed directions 0-3** rather than only 0.
   - **The canvas became unbounded** (see `canvas.go` under Shape),
     replacing the fixed `CanvasWidth`/`CanvasHeight` working area with a
     `RefBoxWidth`/`RefBoxHeight` guide drawn against a full-span origin
     crosshair, after GraalShop's equivalent.

7. **Parts moved from Direction up to Track** (2026-09-19) — the only
   structural change to the v2 data model so far, and worth reading
   before touching anything here. It took two user corrections to land:
   - Adding a part briefly added a copy to every direction, so the other
     facings wouldn't look empty. That was reverted on "it's the artists
     responsibility to manage the directions on the ani", and part
     creation was scoped back to the active direction.
   - That was the wrong inference. The follow-up — "parts should exist
     across all directions... I think direction should just change which
     canvas shows" — is the actual model.

   So `Track.Parts` is now the rig, shared by every direction, and
   `Direction` holds only `Keyframes map[int][]*Keyframe` keyed by
   `Part.ID`. Switching facing changes which keyframes you see and
   nothing else; `Selection.PartIndex` indexes the track's list and
   survives the switch, while `KeyframeIndex` is reset. A part with no
   keyframes in a facing is listed (tagged "no keyframes here") but not
   drawn.

   The distinction to hold on to: the editor shares **structure** across
   directions freely — the part list, sheet bindings, props — but never
   invents **authored content**, so it will not copy keyframes between
   facings. When a change seems to be "about directions", work out which
   of the two it touches before narrowing the behaviour.

   Keying by ID rather than by index or name is deliberate:
   `RemovePart` deletes that ID's keyframes from every direction and
   `Track.nextPartID` never reuses a freed ID, so orphaned keyframes
   can't be silently adopted by a later part, and renaming is free.
8. Two more from a screenshot (2026-09-19):
   - **Only the first cell of a sheet ever rendered** — in the palette
     *and* on the canvas, though all the cell outlines drew correctly.
     `CellImage` returned `image.SubImage`, whose `Bounds().Min` is the
     cell's offset in the parent sheet; Fyne's painter draws an image
     from (0,0) outward, so every cell except (0,0) painted pure
     transparency. Cells are now copied into fresh images with a (0,0)
     origin and pre-cropped once in `NewSpriteSheetTemplate`. The
     earlier slicing test missed this because it sampled each cell at
     `Bounds().Min` — which is precisely the coordinate the renderer
     does *not* use. It now reads (0,0) and asserts the origin, and was
     confirmed to fail against the old implementation.
   - **The canvas shifted while a sprite was being held.** The extent is
     derived from the keyframes, so dragging one changed it mid-gesture
     and slid the origin out from under the cursor; quantizing made it
     rarer but couldn't prevent it, since crossing a quantum boundary is
     what an outward drag does. `CanvasWidget.SetViewFrozen` pins the
     extent for the duration of a drag, driven from both drag sources —
     the canvas's own `Dragged`/`DragEnd`, and `SheetGridWidget`'s new
     `OnDragStart` plus the existing drop callback for palette drags.

## Shape

- Entry point wires a dark editor theme into a Fyne app and delegates to
  `app.New(a).Run()`. `animaker/main.go`.
- `pkg/app/app.go` — application shell; owns the `Project`, wires every
  UI callback, runs the ~60fps playback ticker.
- `pkg/editor/` — domain model:
  - `track.go` — `Track`/`Direction`/`Part`/`Keyframe`/`PropDef`/
    `PropBinding` types (the rig itself). `Track.Directions` is
    `map[int]*Direction` — plain ints, not free-form names, matching the
    game's own direction convention, and `NewTrack` seeds all four
    (0-3) as empty directions. **`Track.Parts` is the rig, shared by
    every direction; `Direction` holds only `Keyframes` keyed by
    `Part.ID`** — see history entry 7 under
    [Why this shape](#why-this-shape) before changing this.
    `Direction.ValueAt(partID, timeMs)` is the interpolation entry
    point. `Track.RefBoxWidth`/`RefBoxHeight`
    (48x64) size the character-shaped placement *guide* the canvas draws
    from the origin — not a working area or a clip region; the
    coordinate space is unbounded and negative coordinates are valid.
    `Direction.TotalDurationMs` is the animation's real length, while
    `Direction.EditableDurationMs` is the longer scrubbable range — see
    the timeline entry under [Why this shape](#why-this-shape).
  - `part.go` — add/remove Part/Direction/Prop helpers. `AddPart` takes
    a `*Track` (not a direction) and assigns the ID; `RemovePart` also
    clears that ID's keyframes from every direction.
  - `keyframe.go` — add/delete/duplicate/move keyframes, all taking
    `(dir, partID, ...)` and kept sorted by `TimeMs`, plus
    `Direction.ValueAt(partID, timeMs)` — the interpolation entry point
    (linear lerp on X/Y/Z/Rotation, step function on Row/Col).
  - `spritesheet.go` — `SpriteSheetTemplate`: fixed cell size + one pivot
    per sheet, `Cols()`/`Rows()` derived from image size, `CellImage`
    serves a `(row, col)` from cells pre-cropped at construction. Those
    cells always have a (0,0) origin, which is load-bearing for
    rendering, not tidiness — see history entry 8.
  - `project.go` — `Project`: current `Track`, loaded sheet templates,
    playback state, `PreviewProps` (editor-only prop overrides, never
    saved), selection state, and `ResolveActiveSheetName`/
    `ResolveActiveSheet` (the prop → sheet resolution the whole
    customization system hangs off).
  - `deepcopy.go`, `undo.go` — full-track-snapshot undo/redo.
- `pkg/ui/` — Fyne widgets:
  - `timeline.go` — `scrubArea` (unexported): a custom-drawn ruler plus
    one row per Part, keyframes as markers positioned by `TimeMs`, click
    the ruler/a row to scrub, click a marker to select it (also seeks the
    playhead there). `TimelineWidget` wraps it with Play/Stop, New/Delete
    Keyframe, speed, and loop controls. "New Keyframe" seeds the new
    keyframe from the part's current interpolated pose (`Direction.ValueAt`)
    rather than snapping to zero, so it starts as a continuation.
  - `canvas.go` — resolves every Part's transform at the current
    `ElapsedMs`, Z-sorts, draws Sheet parts as a cropped+pivoted cell
    around a full-span origin crosshair with the character-sized
    reference box in its bottom-right quadrant. There is **no fixed
    working area and no centering**: `viewBounds()` derives the extent
    from the origin, the reference box and every keyframe of every part,
    padded and quantized to 32px, and `MinSize` follows it — so art at
    negative coordinates (a raised sword) simply grows the canvas.
    Computing over *all* keyframes rather than the current frame is what
    keeps scrubbing from resizing the canvas, and the quantization keeps
    a drag from doing so continuously; either would shift the origin out
    from under the cursor. Implements `Tappable` (click a part
    to select it, `OnPartTapped`) and `Draggable` (drag an *already
    placed* part to move an *existing* keyframe at the exact current
    time — `OnPartDragStart/Dragged/DragEnd` — deliberately does **not**
    create a keyframe implicitly; use "New Keyframe" first). Both share
    `resolvedDraws()`/`hitTest()` so drawing and hit-testing can never
    disagree about where a part actually is. `LocalToAnimXY` converts a
    canvas-local point into the animation's own X/Y space (now a direct
    `local/zoom` scale — origin moved to the canvas's top-left corner
    this round, no longer widget-center-relative).
    **Rotation is stored and saved but not visually applied here** — Fyne
    has no simple rotated-image primitive, and the actual consumer of
    rotation is a future game-side (raylib) renderer, not this preview.
    Nested-animation parts draw as a labeled placeholder box, not a
    recursively-rendered sub-animation — see [Known gaps](#known-gaps-not-bugs)
    below.
  - `sheetgrid.go` — `SheetGridWidget`: renders every cell of a
    `SpriteSheetTemplate` in its real grid layout and implements
    `fyne.Draggable` to report which cell a drag started on plus the
    absolute screen position the drag ended at (`OnTileDropped`). Has no
    knowledge of the canvas — `app.go`'s `onTileDropped` is what checks
    the drop landed inside the canvas and does the coordinate conversion.
    This is the *only* way a new part gets its first keyframe/art; the
    canvas's own drag only repositions what's already there.
  - `properties.go` — **as of 2026-09-19, builds two separate panels**,
    not one (see [Why this shape](#why-this-shape) for the GraalShop
    reference this layout is borrowed from):
    - `BuildPalette()` — the **left column**: just the selected part's
      `SheetGridWidget` (its "Sprite Book" equivalent), scoped to
      whichever part is selected rather than showing every loaded sheet
      at once, since a dropped tile needs an unambiguous target part.
      A header label names the part and its resolved sheet, and carries
      the empty-state guidance ("Import a sprite sheet to begin" /
      "Select a part to show its tiles" / "<part>: <sheet> (not
      loaded)") — the palette being blank is the editor's most common
      confusing state, so it must always say *why* it's blank.
    - `Build(directionBar)` — the **right column**: takes the direction
      bar (built in `app.go`, embedded here as a fixed header via
      `container.NewBorder` rather than its own separate top strip) +
      Import/Add Part buttons, the part **list** (buttons + per-row
      Delete — deliberately not a `Select`, see
      [Known gaps](#known-gaps-not-bugs)) with `SelectPart` exported so
      canvas taps and list clicks stay in sync, inline
      governing-prop/fixed-sheet linking for the selected part (fixed
      sheet is a `Select` over `Project.LoadedSheetNames()`, not an
      entry; `sheetPickerOptions` keeps a bound-but-not-loaded name in
      the option list so opening a track without its art doesn't look
      like the part lost its sheet), the
      selected keyframe's numeric transform fields *plus* a nudge D-pad
      (X/Y ±1, Z forward/back, rotation ±5° — `buildNudgeControls`,
      also a GraalShop borrow) for fine-tuning without retyping numbers,
      a nested-bindings editor when relevant, and the props schema
      (with its own **Add Prop** button) / preview-override sections.
      Every section here is its own pane in a tree of nested
      `container.NewVSplit`s (Fyne splits only take two children each),
      so each has a draggable resize handle.
  - `dialogs.go`, `window.go`, `theme.go` — mostly self-explanatory;
    `theme.go` is untouched by the v2 rewrite (pure color/theme, no
    dependency on the domain model). `window.go`'s `BuildMainLayout` is
    now three nested splits — 20% palette / 65% canvas / 35% properties
    (two nested `HSplit`s) across the top, 75/25 that-row-vs-timeline —
    instead of the earlier two-column canvas/properties arrangement; it
    no longer takes a separate `directionBar` param since that moved
    inside the properties panel. `BuildMenuBar` **has no Rig menu**
    (removed 2026-09-18) — Add Direction/Prop/Part are buttons in the
    panels that actually need them instead of a separate menu.
- `pkg/file/` — persistence: `toml.go` (`SaveTrack`/`LoadTrack` for
  `.anif`, `SaveSheetTemplate`/`LoadSheetTemplate` for `.sprsh`),
  `image.go` (unchanged — generic image loading/cropping). The `.anif`
  layout mirrors the model: `[[parts]]` once at track level carrying an
  `id`, then `[[directions.N.keyframes]]` entries each naming a
  `part_id`. `LoadTrack` rejects a keyframe whose `part_id` names no
  part, and rejects duplicate part ids, rather than dropping them
  silently. Keyframes are written in track-part order so saves are
  stable rather than reordering with Go's map iteration.

## Known gaps (not bugs)

Deliberate scope cuts, not oversights:

- **Nested-animation parts don't actually play their nested `.anif`** in
  the canvas preview — they render as a placeholder box. The data model
  (`Part.NestedAniPath`, `Part.NestedBindings`) is fully implemented and
  saved/loaded correctly; only the recursive-load-and-render step for
  the *preview* is missing.
- **Rotation isn't visually applied** in the canvas for the same reason
  (Fyne limitation) — see above. It's captured in every keyframe and
  round-trips through save/load correctly.
- **No ghost/preview image follows the cursor during a drag** from the
  sheet grid to the canvas — the cell is picked up at drag-start and
  placed at drag-end with no visual feedback in between.
- **Dragging a part directly on the canvas never creates a keyframe**,
  only moves one that already exists at the exact current playhead
  `TimeMs` — this is intentional (matches the requested workflow: select
  a part, "New Keyframe", *then* drag), not a bug, but it means dragging
  a part when the playhead isn't sitting exactly on one of its keyframes
  silently does nothing.
- **No drag-to-retime a keyframe marker** on the timeline ruler — moving
  a keyframe in time isn't wired to any UI action yet, only its transform
  values (via the properties panel or a canvas drag).
- **No keyboard arrow-key nudging** — GraalShop supports both clicking
  its nudge arrows and pressing the keyboard arrow keys for pixel-by-pixel
  movement; only the click-buttons (`buildNudgeControls`) were built here.
  Wiring plain (non-modifier) arrow keys risks conflicting with Fyne
  `Entry` widgets' own cursor-movement handling, so it needs more care
  than a quick add.
- **The left palette shows only the selected part's active sheet**, not
  every loaded sheet the way GraalShop's Sprite Book does — a deliberate
  scoping choice, not a faithfulness gap: `SheetGridWidget.OnTileDropped`
  only carries a `(row, col)`, so the target part (and therefore which
  sheet a drop means) has to be established by which part is currently
  selected, not inferred from an unscoped, all-sheets palette.
- The two items `docs/ANI_MAKER_SPEC.md`'s own "Open questions" section
  flags (how one prop fans out to multiple physical sheets; the sword
  "bent state" mechanism) are exactly as unresolved in code as in that
  doc — nothing here should be read as having quietly decided them.

**Fixed during the same-day UI rework, not just a scope note:** the
original numeric-entry properties panel crashed on startup with any
partless track — `PropertiesPanel.refreshPartSelect` called
`Select.ClearSelected()` when nothing was selected, which re-fires the
`Select`'s own `OnChanged` (even with an empty value), which called
`Refresh()`, which called `ClearSelected()` again: unbounded recursion,
stack overflow, caught via a captured-output smoke test before merge.
See `selectPartByName`'s guard against empty names and the
`refreshDependentSections` split in `properties.go` if a similar
Fyne `Select` pattern is added elsewhere — `SetSelected`/`ClearSelected`
re-firing their own change handler is the trap.

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
  structs are hand-kept-in-sync, not generated) and the `_test.go` files
  in `editor/`, `file/` and `ui/`.
- Moving anything between `Track` and `Direction` is a wide change: the
  part/keyframe split is load-bearing in `canvas.go` (`resolvedDraws`,
  `viewBounds`), `timeline.go` (`scrubArea.parts`), `properties.go`
  (`refreshPartList`) and every callback in `app.go` — most of which go
  through `Application.directionAndPart`.

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
independence, and the track-level part model: parts shared across
directions, keyframes independent per direction, `RemovePart` clearing
every direction without touching its neighbours, IDs never reused, and
the part selection surviving a direction change.
`pkg/file/toml_test.go` — full save/load round-trip for both `.anif`
(props, all three part-authoring cases: sheet+prop-governed, sheet+fixed,
nested-with-bindings, plus a part deliberately left unposed everywhere,
and keyframes staying in the direction they were authored in) and
`.sprsh`.
`pkg/editor/spritesheet_test.go` — sheet slicing: a 4x3 sheet yields 12
cells with distinct content, each with a (0,0) origin and read at (0,0),
which is the coordinate the renderer actually uses; out-of-range
rejection; partial trailing cells dropped.
`pkg/editor/timeline_test.go` — the scrub-deadlock regression (seek and
add a second keyframe past a lone 0ms one), `EditableDurationMs` always
leading the last keyframe, playback still looping over the *real*
duration, and the four default directions.
`pkg/ui/canvas_test.go` — canvas geometry, which is easy to break
silently: the view always contains the origin and reference box, grows
for negative coordinates, stays identical across a scrub, and
`LocalToAnimXY` round-trips (so a dropped tile lands where it was
released). `pkg/ui/properties_test.go` — `sheetPickerOptions`. The rest
of `pkg/ui` and all of `pkg/app` have no automated tests, being GUI wiring
verified by manual launch. Note that launching the binary and confirming
it stays responsive is a real part of the check here, not a formality: a
`Select.ClearSelected()` recursion once shipped as a startup
stack-overflow that `go test` could not have caught.

## See

`animaker/main.go`, `animaker/pkg/`
