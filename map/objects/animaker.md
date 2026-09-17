# Animaker editor

A standalone Fyne desktop app for authoring sprite animations, entirely
separate from the game's Go module.

## Why this shape

`docs/ANI_MAKER_SPEC.md` designs it as a general-purpose sprite-sheet /
keyframe / timeline animation tool with its own file format, independent
of the game so it can be built, versioned, and run without the game's
raylib/networking dependencies. It is its own `module animaker` for
exactly that reason.

## Shape

- Entry point wires a dark editor theme into a Fyne app and delegates to
  `app.New(a).Run()`. `animaker/main.go`.
- `pkg/app/app.go` — application shell.
- `pkg/editor/` — domain model: `animation.go`, `keyframe.go`,
  `project.go`, `spritesheet.go`, `events.go`, `undo.go`.
- `pkg/ui/` — Fyne widgets: `canvas.go`, `timeline.go`, `properties.go`,
  `dialogs.go`, `window.go`, `theme.go`.
- `pkg/file/` — persistence: `toml.go` (project save/load via
  `BurntSushi/toml`), `image.go` (spritesheet loading).

This card intentionally doesn't line-cite every method — the file list
above is the accurate current inventory; open the specific `pkg/`
subpackage for the feature you're touching.

## Connected to

- Nothing in `client/`, `server/`, `shared/` imports `animaker/`, and
  `animaker/` imports nothing from them. They share no code.
- **Ghost link:** `docs/ANI_MAKER_SPEC.md` frames this tool's output as
  feeding the game's animation system, but no code on the game side
  reads animaker's project/export format. If you're asked to "load an
  animaker file in the game," that loader doesn't exist yet — it's a
  new integration, not a bug fix.

## If you change this

- **Hits:** only files within `animaker/pkg/` and `animaker/main.go`.
- **Does not hit:** `client/`, `server/`, `shared/` — verified no shared
  imports either direction.

## Surfaces

Desktop GUI app, run via `cd animaker && go run .` or its own build.
Human-facing (artists/animators), unlike the rest of this map.

## See

`animaker/main.go`, `animaker/pkg/`
