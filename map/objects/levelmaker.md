# Level maker (`cmd/levelmaker`)

The level editor, v1 of `docs/LEVEL_MAKER_SPEC.md` (build-order step 5,
built 2026-10-06): terrain painting with blob-47 autotile on the ground
layer of one level at a time, with dimmed read-only strips of its
neighbours. It's in the game module, renders through `client/render`, and
reads and writes the same `world/` + `levels/` the client and server load.

## Why this shape

Every editing rule lives in `cmd/levelmaker/edit`, which has no raylib
dependency, so CI tests it headless. The raylib side (`cmd/levelmaker`)
only draws and routes input. Re-solving and compiling go through
`shared/world`, the same code the game uses, so the editor can't save a
level the game would read differently.

## Shape

- **`edit.Editor`** — `Open(root)` (needs `root/world/terrains.toml`,
  creates `root/levels/`; opens on the level holding the spawn, else the
  first exterior one), `Select`, `NewLevel` (refuses a bad or taken name
  or an overlap, re-solves the new level's neighbours' borders and marks
  them unsaved; not undoable), `Neighbours`.
  `cmd/levelmaker/edit/edit.go`
- **Painting** — `Paint(cells, terrain)` paints only cells inside the
  current level (terrain 0 erases), then re-solves each painted cell and
  its 8 neighbours, including cells across the edge in neighbouring levels
  (their terrain is never changed). `BeginStroke`/`EndStroke` group a drag
  into one undo step; a `Paint` outside a stroke is its own step. Shapes:
  `Brush` (1/3/5), `Rect`, `Line` (fills the gap between frames of a fast
  drag), `FloodCells`/`FloodFill` (4-connected, same terrain, never leaves
  the level). `cmd/levelmaker/edit/shapes.go`
- **Undo/redo** — each step stores every changed cell's before and after
  `Cell{Terrain, Tile, Under, Flags}` in whichever level it's in. No-op
  strokes aren't recorded; a new stroke clears redo.
- **Save** — re-solves the borders of every changed level
  (`World.SolveBorders`), compiles properties, writes every unsaved level
  (neighbours included) and returns the paths written. The UI shows them
  and names any neighbour rewritten for its border (open question 2's
  default).
- **UI** (`main.go`, `panel.go`, `canvas.go`, `ui.go`) — left panel:
  level list, New level, terrain palette (swatches drawn from the atlas),
  tools, brush size, grid, centre/jump-to, undo/redo, save, quit, status.
  Canvas: `render.NewFreeCamera` centred in the canvas, wheel zooms about
  the cursor, right/middle drag or arrows pan; draws the current level and
  a `stripTiles` (12) border of each neighbour, dimmed; grid; tool
  footprint. Status bar: hovered world tile, level-local cell, terrain.
  Keys: B/R/F tools, 1-9 terrain, E eraser, [ ] brush size, G grid, N new
  level, J jump to, Home centre, Ctrl+Z/Ctrl+Y (or Ctrl+Shift+Z), Ctrl+S.
  Esc only closes dialogs (`SetExitKey(KeyNull)`); closing with unsaved
  levels asks Save and quit / Quit without saving / Cancel.
- **Widgets** (`ui.go`, `textfield.go`) — hand-made immediate-mode
  buttons and a modal `dialog` (text fields, checkbox, buttons; Enter =
  first button, Esc = last, Tab/Shift+Tab move between fields).
  `textField` selects its contents on focus so typing replaces them
  (animaker convention). raygui wasn't used: its text box can't do that.
- **Session log** (`sessionlog.go`) — `bin\logs\levelmaker-*.log`: errors
  shown to the artist, the standard logger, recovered panics with stacks,
  a clean-exit marker, the previous session's crash reported on start, 20
  kept. Doesn't redirect stderr (unlike animaker's `applog`), so an
  unrecoverable Go fatal error isn't captured.

## Connected to

- `shared/world` (see `world.md`): `LoadDefs`, `LoadWorld`, `NewLevel`,
  `SolveCell`, `SolveBorders`, `Compile`, `SaveLevel`.
- `client/render` (see `client-prediction.md`): `Atlas` (incl.
  `DrawRect` for swatches), `Camera` in `Free` mode, `Renderer`.
- `build_local.ps1` builds and launches it with the same `-root` as the
  server and clients.

## If you change this

- **Hits:** what the game loads. Anything saved here is what the server
  validates movement against, so a bug in re-solving or compiling shows
  up as wrong walls or anti-cheat flags, not just wrong art.
- **Hits:** `client/render` changes show in the editor too.
- **Does not hit:** `animaker/` (separate module).

## Not in v1

Hand placement and stamps, flips, property overlays and override brushes
(step 7); objects, warps, copy area to new level (step 8); moving or
deleting a level. Creating a level is not undoable.

## See

`cmd/levelmaker/`, `cmd/levelmaker/edit/`

Tests: `cmd/levelmaker/edit/edit_test.go` (on a temp copy of the sample
world): shapes, brush paint and re-solve, painting never leaves the
current level, a drag stroke across meadow's edge re-solving lake and
undoing/redoing as one step in both, undo/redo history and no-op strokes,
flood fill (exact region, one undo step, stays in the level), erase, new
level refusals and neighbour re-solve, save then reload identical with
seamless borders, opening without definitions, opening on the spawn
level. `cmd/levelmaker/main_test.go`: text field select-on-focus,
filters and max length, zoom formatting, session log crash detection and
pruning. The raylib drawing and input glue is untested; it was checked by
hand under a virtual display (see the spec's step 5 note).
