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

9. **Dragging a tile now creates a new part** (2026-09-24), from
   "it doesn't let me have multiple parts on the sheet simultaneously, I
   should be able to have all of the parts on a sheet". Every drop used
   to land on the *selected* part, so dragging three tiles produced
   three keyframes of one part — and a part shows one cell at a time, so
   only ever one piece was visible. Building a rig of several pieces
   meant finding "+ Add Part" and naming each by hand first.

   The three jobs now have three gestures, which is how a level editor
   behaves and what the earlier level-editor rework was already reaching
   for:
   - **drag from the palette** -> add a new part to the rig, keyed at
     the playhead, at the drop position, showing the dragged cell;
   - **drag a part on the canvas** -> move its existing keyframe
     (unchanged);
   - **click a palette tile** -> re-cell the selected keyframe.

   Consequences worth knowing: the palette no longer follows the part
   selection (it has its own sheet picker in the left column, since a
   drag must work with nothing selected and a drop has to know which
   sheet the cell came from — `Project.PaletteSheet`); new parts are
   named `<sheet>_N` via `editor.UniquePartName`, because the part list
   and timeline are labelled by name and identical labels are unusable
   even though identity is really the ID; a dropped part's Z is set from
   the part count so later drops sit in front rather than behind; and
   **import no longer creates a part**, which it briefly did (entry 5) —
   pointing the palette at the freshly imported sheet is what makes an
   import visibly do something now, and an auto-created part would just
   be an empty one drawn nowhere.

10. **Timeline editing** (2026-10-01), requested together:
    - **Dragging a placed part auto-keys it.** A canvas drag used to move
      only a keyframe sitting *exactly* at the playhead and silently do
      nothing otherwise — so once parts were dropped at 0ms, every drag
      anywhere else on the timeline was a no-op. It now creates a
      keyframe at the playhead if there isn't one, seeded from the
      part's current pose (`editor.EnsureKeyframe`), and moves that.
      Drags also report a delta from where they started, so a sprite
      moves with the point it was grabbed by instead of snapping its
      pivot to the cursor, and both the canvas and the timeline
      hit-test the *press* point (Fyne's first `Dragged` event has
      already moved).
    - **Drag a marker to retime it**, with a **Lock timing** checkbox
      that makes marker drags scrub instead. `MoveKeyframe` refuses to
      land on a time another keyframe of the same part holds
      (`ErrKeyframeTimeTaken`), and the drag holds the `*Keyframe`
      rather than its index, because a retime re-sorts.
    - **Duplicate Keyframe** copies the selected pose to the playhead, or
      `DuplicateOffsetMs` after the source if the playhead is on it.
      Onto an already-keyed time it pastes the pose rather than stacking
      a second keyframe — `DuplicateKeyframe` previously did stack, which
      gives `ValueAt` a zero-length segment.
    - **Timeline zoom**: `-`/`+`/Fit buttons and Ctrl+wheel, anchored so
      the time under the cursor (or the playhead, for the buttons) stays
      put. Ruler ticks and labels pick a 1-2-5 interval to suit the zoom.

    Found along the way: **a playhead exactly on a middle keyframe drew
    the previous keyframe's cell.** `ValueAt` folded equality into the
    preceding segment at t=1, which gets position right but takes
    Row/Col from that segment's start. Clicking a marker seeks exactly
    onto its keyframe, so the canvas had been showing the wrong cell at
    every middle keyframe. Fixed; the regression test was confirmed to
    fail against the old code.

11. **The selection decides what a palette drop does** (2026-10-01),
    partly reversing entry 9. Reported: on direction 0, drop cell (0,0),
    scrub to ~200ms, drop cell (0,1) — it "creates a new timeline for
    that sprite, but it should just insert it onto the timeline of the
    previous sprite". Entry 9 had made every drop a new part, which
    fixed rig-building but broke frame-by-frame swaps. Both are wanted,
    and the user chose the selection as the deciding signal:
    - **a part is selected** and draws from the dropped sheet -> the
      cell becomes its keyframe at the playhead, at the drop point
      (`editor.EnsureKeyframe`, so an existing keyframe at that time is
      re-celled/moved rather than duplicated, and a new one keeps the
      interpolated Z/rotation);
    - **nothing is selected**, or the selected part draws from a
      different sheet (a part can only show cells of its own sheet) ->
      new part, exactly as entry 9.
    Deselect by clicking empty canvas (already worked) or **Esc** (new).
    The rule lives in `editor.Project.DropTile` so it is unit-tested
    (`drop_test.go`); `app.onTileDropped` only does coordinate
    conversion, undo, and selection. No "hold" keyframe is needed to keep
    the old cell up until the new one: Row/Col already step. X/Y still
    lerp between the two keyframes, so a frame dropped at a different
    spot slides there over the gap.

12. **Rename a part by double-clicking its timeline name** (2026-10-01),
    requested so a rig built from drops (`sprite_1`, `sprite_2`...) can
    become "head", "left_arm", "legs". There was no rename anywhere
    before. The name column is now one `partLabel` widget per row: click
    selects the part, double-click opens a prefilled rename dialog. It's
    a separate widget rather than `scrubArea` implementing
    `DoubleTappable` because Fyne delays every single tap on a
    double-tappable widget, which would make click-to-scrub laggy.
    `editor.RenamePart` trims and refuses empty or duplicate names (rows
    are labelled by name); a refusal shows the error and reopens the
    dialog with what was typed. Undoable.

13. **Props are declared at import; right panel regrouped** (2026-10-01).
    Reported: the user imported a sheet, clicked "+ Add Prop", typed
    `body` as the default sheet without knowing what it meant, linked
    the part to that prop, and the sprite vanished — no sheet was named
    `body`, so the part resolved to nothing. Props were only reachable
    through that free-text field, with nothing explaining them.
    - The import dialog's **Sheet Name is required**, and a **"Swappable
      art (a prop)"** tick plus prop name (defaulting to the sheet name)
      declares the prop with this sheet as its default via
      `editor.EnsureProp`. Naming a prop that already exists adds the
      sheet as another option without changing the default.
    - A new part dropped from a sheet some prop is currently set to is
      **linked to that prop** (`Project.PropForSheet`) and named after it
      (`hair_1`); `FixedSheet` is still set so unlinking keeps the art.
    - "+ Add Prop"'s default and every Preview Override are **pick-lists
      of loaded sheets**, not free text, and a duplicate prop name is
      refused. Short explanations sit in both dialogs and the PROPS pane.
    - The right panel is now **two panes**: parts, selected part link,
      selected keyframe and preview overrides in one scrolling pane (the
      user's request), props schema below it.
    - **Preview-only sheets.** Each Preview Override row has "Load
      file...": any image is sliced on the prop's *default* sheet's grid
      (cell size and pivot) — what a runtime swap does, per the spec's
      template rule — and set as that prop's override
      (`Project.LoadPreviewSheet`). These live in `Project.PreviewSheets`,
      apart from `LoadedSheets`: no `.sprsh` is written, `PreviewProps`
      was already outside the `.anif`, and they're offered only as preview
      values, never as a fixed sheet, prop default or palette sheet, so
      nothing authored can depend on them. While a preview shows, a drop
      of the prop's own default-sheet tile still keys the selected part
      (`partShowsSheet`), since the palette shows the authored sheet.

14. **Copy timing into an empty direction; type a keyframe's time**
    (2026-10-01).
    - Switching to a direction with no keyframes prompts to copy keyframe
      **times** from another (direction 0 preselected when it has any).
      The user chose times only: each copied keyframe has a zero pose
      (origin, cell 0,0), so no position or cell leaks between facings —
      the rule from the parts-move entry still holds, the editor never
      copies authored poses across directions; this copies structure,
      and only on the artist's say-so. `editor.CopyKeyframeTimes`,
      `Track.TimingSources`. The prompt fires only on a real switch,
      since the direction Select re-fires on programmatic SetSelected.
    - The Selected Keyframe section has a **Time (ms)** field, applied on
      Enter or "Set" (not per keystroke: typing 600 would pass through 6
      and 60, re-sorting and colliding on the way). It refuses a time the
      part already has a keyframe at, keeps the playhead on the keyframe,
      is undoable, and ignores "Lock timing" (which guards drags).

15. **Tab selects a field's text** (2026-10-01). Requested: tabbing into a
    field should select its contents so typing replaces them. Fyne's Entry
    just puts the caret at the end. `ui.newEntry` (`entry.go`) selects all
    on focus and is used for every text field in the editor; a click
    still places the caret, because Fyne's mouse-down positions it after
    focusing, which clears that selection. Both behaviours are tested
    against Fyne directly. Use `newEntry`, not `widget.NewEntry`, for new
    fields.

16. **Reopening a track loads its sheets** (2026-10-01). Reported: a saved
    WIP `.anif` reopened with no sprite sheet. The `.anif` names sheets
    but never said where they are, and `onOpenTrack` loaded none
    (`file.LoadSheetTemplate` existed with no caller). Now:
    - Saving writes `[[sheets]]` (name + `.sprsh` path relative to the
      `.anif`) from each loaded sheet's new `SprshPath`; preview-only
      sheets are left out. Editor hint only — names remain the reference.
    - Opening calls `file.LoadSheetsForTrack`: recorded paths first, then
      a bounded search of the `.anif`'s folder for `.sprsh` files whose
      *declared* name matches (filenames follow the image, not the
      sheet name). The user confirmed that expecting a track's art
      beside the `.anif` is reasonable, so that folder is the fallback.
    - Anything still missing (or found but unloadable, e.g. image gone)
      is listed in a dialog with the fix: re-import with that exact
      Sheet Name.
    - `LoadTrack` now returns `(track, refs, err)`; `SaveTrack` takes the
      refs.

17. **Delete a part from the timeline** (2026-10-01). Reported: no way to
    delete a layer from the timeline. Each row now has a red x at the
    left of its name (`rowDeleteButton`, a drawn glyph since a
    widget.Button is taller than a row; a sibling of the name, not
    nested in it, so it can't compete with click/double-click). It and
    the part list's Delete both go through `app.confirmDeletePart`,
    which confirms (the delete spans every direction) and is undoable.
    Found along the way: the part list's Delete left the selection
    index unchanged when deleting a part *above* the selected one, so
    the selection silently moved to the neighbouring part.
    `Project.DeletePart` now shifts it.

18. **Session logs, and a playback race fixed** (2026-10-01). Reported: the
    editor crashed with no trace. `build_local.ps1` starts it with
    `Start-Process`, so Go's crash dump went to a console window that
    closed with the process, and Go crashes skip Windows error reporting.
    - `pkg/applog`: each run writes `bin/logs/animaker-<time>.log` (20
      kept). It gets the standard logger (where Fyne reports its own
      errors), every error the editor shows (`app.showError`, which all
      error dialogs now use), and the runtime's crash output. On Windows
      that's done by pointing the process's stderr handle at the file:
      `debug.SetCrashOutput` alone records a fatal error's stack but not
      its message line. A clean shutdown appends a marker; if the last
      log lacks it, the next launch says the editor crashed and where the
      log is. Tested by re-running the test binary as a child that really
      panics / hits a concurrent map write.
    - **Likely cause of the crash:** `playbackLoop` advanced the project
      and redrew from its own goroutine while the UI goroutine edited the
      same keyframe maps — an uncatchable "concurrent map iteration and
      map write". Each tick now runs inside `fyne.Do`. Unconfirmed as the
      cause, since the crash left no log; the logs will say next time.
    - Follow-up: a second editor opened while the first was still open
      reported the first as crashed (its log isn't finished). Each log's
      first line now carries `pid=`; an unfinished log counts as a crash
      only if that process is gone (`applog.crashed`).

19. **Remove an imported sheet** (2026-10-01). Reported: an accidentally
    imported `flame.png` was stuck in the `.anif` — every loaded sheet
    is written to `[[sheets]]` on save, and nothing could unload one.
    The palette's sheet picker has a **Remove** button
    (`Project.RemoveSheet`, confirmed by `app.confirmRemoveSheet`). It
    only unloads the sheet, so the next save drops it from `[[sheets]]`;
    files on disk are untouched. It refuses while a part's fixed sheet or
    a prop's default names it (`Track.SheetUsers`,
    `SheetInUseError`), since those would otherwise draw nothing, and it
    clears preview overrides naming it. Not undoable (sheets aren't in
    the track snapshot), which is why it confirms; re-import restores it.

20. **Nested animations: import, props, live playback** (2026-10-01).
    Reported: couldn't add `torch.anif` to `walk_torch.anif` as a prop.
    Nested parts existed only behind "+ Add Part" with a typed path,
    never got a keyframe (so never showed), drew as a 24px placeholder,
    and props could only hold sheets. The user asked for nesting to work
    like importing a sheet, with the prop opt-in there or later.
    - **Import Animation** (`ui.ShowImportAnimDialog`,
      `app.onImportAnim`): loads the `.anif` via `file.LoadNestedAnim`
      (its own sheets, recursive nesting, cycle-safe via the `loaded`
      map) into `Project.LoadedAnims` keyed by `editor.AnimKey`
      (absolute path), adds a part keyed at the playhead at the origin,
      and optionally declares an `.anif`-valued prop. Naming an existing
      `.anif` prop only adds an option. Refuses nesting the track in
      itself.
    - **Props hold `.anif` paths** for nested parts (`IsAnimValue` - the
      value decides the kind, no format change). Sheet parts' prop
      picker lists only sheet props; nested parts get Prop / Animation
      pickers (`buildNestedLink`). Add Prop and Preview Overrides offer
      loaded animations; "Load file..." on an `.anif` prop loads one.
    - **Live playback**: `Project.FlattenNested` resolves a nested part
      to sheet sprites through any nesting (depth-capped at 4), with
      `childDirection` / `childProps` applying `NestedBindings`. It runs
      on `Playback.NestedClockMs`, which advances while playing but never
      wraps with the parent; Seek/Stop/direction switch reset it. `Play`
      now starts for a still parent that contains a nested animation.
      The canvas draws the sprites and sizes/hit-tests the part by
      `NestedExtent` (every keyframed pose), so it doesn't resize as the
      torch plays.
    - **Files**: nested paths and `.anif` prop defaults are absolute in
      memory, relative to the `.anif` on disk (`relAnimPath` /
      `absAnimPath`); opening a track loads its nested animations.
    - Still not shown: nested **rotation**, like every part's.

21. **Select to edit** (2026-10-01). Two requests:
    - The position panel (X/Y/Z/Rotation, nudge pad) is **always shown
      for a selected part**. It used to need a selected keyframe, and
      selecting a part never selects one, so clicking a part showed a
      hint instead. Without a keyframe at the playhead the fields show
      the part's interpolated pose there, and the first edit or nudge
      adds a keyframe at the playhead (undoable) - the same auto-keying
      as a canvas drag. Refreshing the panel doesn't change the
      selection.
    - Parts were to be "lockable, or only interactable when selected";
      the latter was chosen: a **canvas drag only moves the selected
      part** (`CanvasWidget.dragTarget`). A click still selects the
      topmost part; dragging an unselected part does nothing. Because
      the selection decides rather than Z order, a part covered by
      another can be dragged once selected from the list or timeline.

22. **Nested direction: inherit, static, or per keyframe** (2026-10-01).
    Requested as three choices. Direction had been a generic
    `NestedBindings["direction"]` typed into free-text fields, which
    covered static/inherit awkwardly and per-keyframe not at all.
    - `Part.DirectionMode` (`NestedDirInherit` default, `NestedDirStatic`
      + `StaticDirection`, `NestedDirPerKeyframe`) and
      `Keyframe.Direction`, stepped through `ResolvedTransform.Direction`
      like Row/Col. `Part.NestedDirectionAt` gives the direction to ask
      the nested track for; `flatten` takes that (`pickDirection` falls
      back to the track's first direction). Nested-in-nested parts use
      their own mode relative to their parent's direction.
    - `Project.SetNestedDirectionMode` seeds the new mode from what's
      showing, so switching never visibly jumps.
    - `NestedExtent` unions every direction a per-keyframe part uses.
    - File: part `direction_mode` / `static_direction`, keyframe
      `direction`. `migrateDirectionBinding` turns an old static
      `direction` binding into static mode (a passthrough into inherit)
      and drops it; the bindings editor no longer lists `direction`.
    - UI: Direction picker (+ "Plays:" for static) in the nested part's
      link section; a per-keyframe Direction picker in the position
      panel, which edits/creates the keyframe at the playhead like the
      other fields.

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
    one row per Part, keyframes as markers positioned by `TimeMs`. Click
    the ruler or a row to scrub; click a marker to select it (also seeks
    there); drag a marker to retime it unless timing is locked, else
    drag scrubs — decided by where the drag *started*. Zoom is
    `msPerPixel`; `zoomAnchorOffset`/`niceTickStep` are pure and tested.
    It implements `Scrollable` only so Ctrl+wheel can zoom; every other
    wheel event is forwarded to the enclosing `container.Scroll`.
    `TimelineWidget` adds Play/Stop, New/Duplicate/Delete Keyframe, Lock
    timing, speed, loop and zoom controls. New Keyframe and the canvas's
    auto-key share `editor.EnsureKeyframe`, which seeds from the
    interpolated pose and leaves an existing keyframe untouched.
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
    With nothing selected this is the only way a new part gets its first
    keyframe/art; with a part selected it keys that part instead (entry
    11). The canvas's own drag only repositions what's already there.
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
- **No keyboard arrow-key nudging** — GraalShop supports both clicking
  its nudge arrows and pressing the keyboard arrow keys for pixel-by-pixel
  movement; only the click-buttons (`buildNudgeControls`) were built here.
  Wiring plain (non-modifier) arrow keys risks conflicting with Fyne
  `Entry` widgets' own cursor-movement handling, so it needs more care
  than a quick add.
- **The palette shows one sheet at a time**, picked from a dropdown,
  rather than every loaded sheet at once the way GraalShop's Sprite Book
  does. Not a data-model limit — `onTileDropped` now receives the sheet
  name — just an unbuilt layout.
- **Timeline zoom is per-session** — it resets to 100% on restart and
  isn't saved with the track. Retiming has **no snapping**, so a dragged
  marker lands on whatever ms the cursor maps to; zooming in is the way
  to place one precisely.
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
The part list has since become plain buttons (`refreshPartList`), which
sidesteps it entirely; the `refreshDependentSections` split in
`properties.go` remains for any `Select`-driven section that refreshes
from its own handler. `SetSelected`/`ClearSelected` re-firing their own
change handler is the trap — assign `OnChanged` *after* seeding a
`Select`'s value, as every `Select` in this package now does.

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
released). `pkg/editor/keyframe_ops_test.go` — the exactly-on-a-middle-keyframe
`ValueAt` regression; `MoveKeyframe` re-sorting with the moved keyframe's
ID tracking it, and refusing occupied times; `DuplicateKeyframe`
inserting, pasting onto an occupied time, and no-op onto its own time;
`EnsureKeyframe` leaving existing keyframes untouched and seeding new
ones from the interpolated pose. `pkg/ui/timeline_test.go` — tick and
label spacing at every zoom (labels always on ticks), zoom clamping, the
anchor keeping the time under the cursor fixed, and `timeForX`/`xForTime`
round-tripping at every zoom. `pkg/ui/properties_test.go` —
`sheetPickerOptions`. The rest
of `pkg/ui` and all of `pkg/app` have no automated tests, being GUI wiring
verified by manual launch. Note that launching the binary and confirming
it stays responsive is a real part of the check here, not a formality: a
`Select.ClearSelected()` recursion once shipped as a startup
stack-overflow that `go test` could not have caught.

## See

`animaker/main.go`, `animaker/pkg/`
