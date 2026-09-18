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

## Shape

- Entry point wires a dark editor theme into a Fyne app and delegates to
  `app.New(a).Run()`. `animaker/main.go`.
- `pkg/app/app.go` — application shell; owns the `Project`, wires every
  UI callback, runs the ~60fps playback ticker.
- `pkg/editor/` — domain model:
  - `track.go` — `Track`/`Direction`/`Part`/`Keyframe`/`PropDef`/
    `PropBinding` types (the rig itself). `Track.Directions` is
    `map[int]*Direction` — plain ints, not free-form names, matching the
    game's own direction convention. `Track.CanvasWidth`/`CanvasHeight`
    define the `0,0`-to-`(W,H)` working area parts are placed within.
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
  - `timeline.go` — `scrubArea` (unexported): a custom-drawn ruler plus
    one row per Part, keyframes as markers positioned by `TimeMs`, click
    the ruler/a row to scrub, click a marker to select it (also seeks the
    playhead there). `TimelineWidget` wraps it with Play/Stop, New/Delete
    Keyframe, speed, and loop controls. "New Keyframe" seeds the new
    keyframe from the part's current interpolated pose (`Part.ValueAt`)
    rather than snapping to zero, so it starts as a continuation.
  - `canvas.go` — resolves every Part's transform at the current
    `ElapsedMs`, Z-sorts, draws Sheet parts as a cropped+pivoted cell
    within the track's `0,0`-to-`(CanvasWidth,CanvasHeight)` bounds
    (drawn as an outline; `MinSize` equals the zoomed working area so
    wrapping it in `container.NewScroll` gives scrollbars exactly when
    content overflows the viewport). Implements `Tappable` (click a part
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
  - `properties.go` — the level-editor-style right panel: Import + **Add
    Part** buttons, a part **list** (buttons + per-row Delete —
    deliberately not a `Select`, see [Known gaps](#known-gaps-not-bugs))
    with `SelectPart` exported so canvas taps and list clicks stay in
    sync, inline governing-prop/fixed-sheet linking for the selected
    part, that part's `SheetGridWidget` (or a nested-bindings editor),
    the selected keyframe's numeric transform fields (for fine-tuning
    after a drop or a canvas drag), and the props schema (with its own
    **Add Prop** button) / preview-override sections. As of 2026-09-19,
    these aren't just stacked in one scrolling column — every section is
    its own pane in a tree of nested `container.NewVSplit`s (Fyne splits
    only take two children each), so each has a draggable resize handle.
    The PARTS section is itself split internally (part list/link vs. the
    sheet grid), defaulted to give the sheet grid the majority of the
    space, since that's what was getting squeezed down before.
  - `dialogs.go`, `window.go`, `theme.go` — mostly self-explanatory;
    `theme.go` is untouched by the v2 rewrite (pure color/theme, no
    dependency on the domain model). `window.go`'s `BuildMainLayout` is
    two nested splits: 50/50 canvas-vs-properties, 75/25 that-row-vs-timeline.
    `BuildMenuBar` **no longer has a Rig menu** (removed 2026-09-18) —
    Add Direction/Prop/Part are buttons in the panels that actually need
    them (direction bar, PARTS section, PROPS section) instead of a
    separate menu, so the action lives next to the thing it affects.
- `pkg/file/` — persistence: `toml.go` (`SaveTrack`/`LoadTrack` for
  `.anif`, `SaveSheetTemplate`/`LoadSheetTemplate` for `.sprsh`),
  `image.go` (unchanged — generic image loading/cropping).

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
