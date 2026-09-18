# ANIFile Animation Maker — Technical Specification (v2: Rigged/Parts Model)

**Version**: 2.0 (supersedes v1 in full — this is not an extension of the old design, it's a replacement)
**Status**: Implemented 2026-09-18 in `animaker/pkg/editor`/`pkg/ui`, then revised twice more the same day from hands-on use: Directions changed from free-form strings to plain ints; the UI was reworked into a level-editor interaction (drag tiles onto a canvas) and then further into a fully interactive canvas (click/drag placed parts) with a real scrubbable timeline — see `map/objects/animaker.md` for the UI history in detail. Revised again 2026-09-19 from hands-on use: the `CanvasWidth`/`CanvasHeight` "working area" became a `RefBoxWidth`/`RefBoxHeight` placement guide over an unbounded space, new tracks seed directions 0-3, the timeline gained a scrubbable range that extends past the last keyframe, and — the one structural change to the data model since v2 was written — Parts moved from inside each Direction up to the Track, so the rig is shared and a Direction holds only keyframes (all reflected below). Known, deliberate implementation gaps (nested-animation preview rendering, visual rotation in the canvas, no drag-to-retime on the timeline) are tracked there, not here. The [Open questions](#open-questions--genuinely-unresolved) section below is still genuinely unresolved in code, exactly as written. The [History](#history-what-v1-was) section describes the v1 flipbook model this replaced.
**Language**: Go
**GUI Framework**: Fyne
**File format**: TOML (consistent with the rest of `animaker/pkg/file`)

---

## Table of Contents

1. [Why v1 got replaced](#why-v1-got-replaced)
2. [Core concepts](#core-concepts)
3. [Part kinds](#part-kinds)
4. [Sprite sheet templates](#sprite-sheet-templates)
5. [Props](#props)
6. [File formats](#file-formats)
7. [Runtime design guidance (not built this pass)](#runtime-design-guidance-not-built-this-pass)
8. [Explicitly out of scope / deferred](#explicitly-out-of-scope--deferred)
9. [Open questions — genuinely unresolved](#open-questions--genuinely-unresolved)
10. [Editor implementation plan](#editor-implementation-plan)
11. [Worked example](#worked-example)
12. [History: what v1 was](#history-what-v1-was)

---

## Why v1 got replaced

v1 was a flipbook editor: one whole-character sprite per keyframe, single timeline, no concept of independently-moving parts. The actual need (surfaced in design discussion, 2026-09-17/18) is a **multi-part rig**, closer to Spine/DragonBones: named parts placed and tweened independently on a shared timeline, with a swappable-art system so the same rig can render hundreds of visually distinct characters (different hair, armor, weapons) without duplicating animation data per appearance.

---

## Core concepts

```
Track (= one .anif file, one named motion — "human_walk.anif")
├── Parts ([]*Part — "Body", "Hair", "Arm_Left", ... — the rig, shared by every Direction)
└── Directions (map[int]*Direction — 0=up, 1=right, 2=down, 3=left by convention)
    └── Direction
        └── Keyframes (map[Part.ID][]*Keyframe — how this facing poses each part)
```

- **Track = one file, one motion.** `human_walk.anif`, `human_idle.anif`, `human_attack_sword.anif`, `human_hurt.anif`, `base_wood_torch.anif` are each their own `Track`. There is no wrapping "character" file — see [Props](#props) for why, and what that costs.
- **Direction is first-class, not a prop, and is a plain int, not a name.** Direction keys are `int` (matching the game's own 0=up/1=right/2=down/3=left convention, decided 2026-09-18 — originally spec'd as free-form strings like `"up"`, changed once actual editor use showed a plain int was simpler to work with); a non-directional thing (a treasure chest) just uses key `0`.
- **Parts belong to the Track; Directions hold only keyframes.** The rig's slot list is declared once on the `Track` and exists in every direction. What differs per facing is the *keyframes*, which each `Direction` stores keyed by `Part.ID` — art commonly differs enough by facing (back of the head vs. front of it) that sharing one timeline across directions doesn't hold up, but the slot list itself does hold up. Switching direction in the editor therefore only changes which canvas and keyframes you see. A part with no keyframes in a given direction simply isn't drawn there; posing it in each facing is the artist's work, and which facings they pose is their call. (Corrected 2026-09-19: Parts previously lived *inside* each Direction, which meant importing a sheet produced a part visible only in direction 0 and forced the artist to maintain four parallel copies of the same slot list.)
- **A Track carries a reference box, not a working area.** `RefBoxWidth`/`RefBoxHeight` (48x64 by default) size a character-sized guide the editor draws from the origin down-right, so parts can be placed relative to a real body footprint. It is *only* a guide: the coordinate space is unbounded and parts may sit at negative coordinates above or left of the origin (a raised sword, a trailing cape). These replaced the `CanvasWidth`/`CanvasHeight` "working area" of 2026-09-18, which implied a bound that never actually existed and forced the editor's canvas to clip rather than grow. Files written with the old `canvas_width`/`canvas_height` keys load with the defaults.
- **Posing is the artist's to manage.** A new Track is seeded with four empty directions (0-3). The editor shares *structure* across directions freely (the part list, each part's sheet binding, the props) but never invents *authored content*: it does not copy keyframes between facings, so an animation may legitimately be posed in only some of them.
- **Parts are free-form per Track.** No fixed schema/template of "every character always has exactly these 10 parts" — each `.anif` declares whatever named parts it needs.
- **Keyframes share tick times within a Direction.** All Parts in one Direction are evaluated against the same timeline positions — "keyframe 3" means the same instant for every part. This was chosen for simplicity over independent per-part timing, and it pays off at runtime: resolving "current segment + interpolation fraction" happens once per instance per frame, not once per part.

```go
type Track struct {
    Metadata     TrackMetadata
    Props        []PropDef            // see Props section
    RefBoxWidth  int                  // character-sized placement guide drawn from the origin
    RefBoxHeight int                  // (a guide only - the coordinate space is unbounded)
    Parts        []*Part              // the rig, shared by every Direction
    Directions   map[int]*Direction   // 0=up, 1=right, 2=down, 3=left by convention; any int works
}

type Direction struct {
    // Keyed by Part.ID, not by index or name: removing a part can't
    // re-point another part's keyframes, and renaming a part is free.
    Keyframes map[int][]*Keyframe
}

type Part struct {
    ID            int             // stable for the life of the track; keys the keyframes above
    Name          string          // "Body", "Hair", "Arm_Left"
    Kind          PartKind        // Sheet | NestedAni — see Part kinds
    GoverningProp string          // Sheet kind: which prop selects the active sheet ("" = fixed default)
    NestedAniPath string          // NestedAni kind: path to another .anif
    NestedBindings map[string]PropBinding // NestedAni kind: see Part kinds
}

type Keyframe struct {
    TimeMs      uint32
    X, Y, Z     float32   // Z is tweened like X/Y, not static — feeds draw-order sort
    RotationDeg float32
    Row, Col    int       // Sheet kind only: which cell of the active sheet to show,
                           // explicitly chosen by the artist per keyframe — never
                           // auto-derived from the Track's name or frame index.
}
```

---

## Part kinds

### Sheet part

The common case: at each keyframe, the artist picks a `(Row, Col)` cell from whichever sprite sheet is currently active for this part (see [Props](#props) for how "active sheet" is chosen), and places it at `(X, Y, Z)` with `RotationDeg`. This is deliberately close to the old flipbook idea — swap-the-image-per-keyframe — just scoped to one part instead of the whole body.

### Nested-animation part

References another whole `.anif` file (e.g. a flickering torch nested inside a character holding it). Two things distinguish it from a Sheet part:

- **The parent's keyframes only control placement** (X, Y, Z, RotationDeg) — never `Row`/`Col`, which is meaningless here.
- **The nested animation plays on its own clock, looping independently**, not synced frame-to-frame to the parent. If an artist wants frame-precise coordination between a sub-element and the parent, that's not what this mechanism is for — they'd just add the raw sprites as an ordinary Sheet part and keyframe it by hand inside the parent Track.

If the nested `.anif` exposes its own props (including "direction," which behaves like a prop from the *outside* even though it's first-class *inside* its own Track), the parent chooses, per prop, per placement, whether to mirror one of its own props or pin a static value:

```go
type PropBinding struct {
    PassthroughFrom string // name of a prop on the PARENT Track to mirror; "" if using StaticValue
    StaticValue     string
}
```

Example: placing a torch in `human_torch_run.anif`, the artist can bind the torch's `direction` to pass through from `human_torch_run`'s own active direction (torch turns with the character), or pin it to a static value (torch always renders the same way regardless of facing) — same mechanism for any prop the nested asset happens to expose, direction included.

---

## Sprite sheet templates

```go
type SpriteSheetTemplate struct {
    Name           string  // "human_body_template_sheet", "human_hair_v2_template"
    FilePath       string
    CellW, CellH   int     // fixed at import time
    PivotX, PivotY float32 // ONE pivot, applies to every cell in this sheet
}
```

- **Fixed grid, defined once at import.** The artist imports an image, sets a cell size (e.g. 32×48) and one pivot point (e.g. center-center) for the whole sheet. Every cell in that sheet shares that size and pivot.
- **Different sheets for the same Part can have entirely different cell sizes and pivots from each other.** This is how size variety works (a giant claw sheet vs. a tiny chicken-foot sheet) — *not* arbitrary rectangles inside one shared sheet. The one hard requirement across sheets meant to be interchangeable for the same Part is that **the same `(Row, Col)` means the same conceptual pose** in every one of them — that agreement is documented/tribal knowledge between whoever defines the template and whoever authors content against it, not something the tool enforces mechanically.
- **The pivot is what makes size variety actually work.** A keyframe positions and rotates a cell *by its pivot*, never by raw image bounds — so a giant claw and a tiny foot, each with their pivot placed at the equivalent joint, both align correctly when swapped into the same Part.
- **New layout = new name, not a migration.** If a template's grid needs to change (more frames, different cell size), that's a new template name (`human_body_template_sheet` → `human_hair_v2_template`) paired with a new Track that uses it. Existing templates are never reorganized in place — old Tracks and the templates they depend on stay frozen and compatible forever. This is *why* `(Row, Col)` can be a plain position instead of needing a stable string label: nothing that already depends on a given template's layout will ever see that layout change under it.
- **This is also the moddable-content story.** A template PNG (with guide markers) is distributed to whoever's producing new hairstyles/armor/etc. — as long as their sheet fills in the same grid positions with their own art, it "just works" when a `Track` swaps to it via props, with zero edits to any `.anif` file.

Saved as a `.sprsh` TOML sidecar next to the image (existing convention from `pkg/file/toml.go`).

---

## Props

A `PropDef` is just a name and a default:

```go
type PropDef struct {
    Name    string // "arms", "hair", "torch_sheet"
    Default string // a sheet name
}
```

**A prop's value is always the name of a sheet to use.** There is no separate "variant index" vs. "hotswap name" distinction — one mechanism covers both a small dev-curated set of arm styles and a community library of thousands of hairstyles.

**One prop can govern multiple Parts** — e.g. `arms` governs both `Arm_Left` and `Arm_Right`. See [Open questions](#open-questions--genuinely-unresolved) for how a single prop value resolves to two different physical sheets.

**Props are declared per-Track file, not in a shared "character" manifest.** This was an explicit choice (see discussion 2026-09-18): a `Character` manifest owning the schema once was considered and would prevent drift, but the team is staying small and would rather keep track files self-contained. The cost this accepts: nothing *structurally* guarantees `human_walk.anif` and `human_idle.anif` agree on what `hair` means or that it exists in both. A linter validating that a family of Track files (e.g. everything matching `human_*.anif`) agree on their prop schema is planned but **not yet built** — treat it as load-bearing, not a nice-to-have, because of the next point.

**Why the linter matters at runtime, specifically:** a character's prop values (customization) live on the runtime instance, not on any one Track, and are expected to carry over automatically when a state machine swaps which Track is playing (idle → walk). If two Tracks for the same character don't agree on prop names, that swap is exactly when a customization silently stops applying — a visible bug, not a theoretical one.

---

## File formats

### `.anif` (a Track) — TOML

```toml
[metadata]
name = "human_walk"
version = "1.0"
ref_box_width = 48
ref_box_height = 64

[[props]]
name = "hair"
default = "long_blonde"

[[props]]
name = "arms"
default = "leather"

# Parts are declared once at track level - they are the rig, shared by
# every direction. Each part's id is what the directions' keyframes
# reference; it is stable for the life of the track and never reused.
[[parts]]
id = 1
name = "Body"
kind = "sheet"
fixed_sheet = "human_body_default"   # no governing prop: not customizable

[[parts]]
id = 2
name = "Hair"
kind = "sheet"
governing_prop = "hair"

[[parts]]
id = 3
name = "Torch"
kind = "nested_ani"
nested_ani_path = "base_wood_torch.anif"

  [parts.nested_bindings.direction]
  passthrough_from = "direction"   # torch turns with the character

# Direction table keys are the string form of the int key (TOML table
# headers must be strings) - "2" here is direction 2 = "down" by
# convention. Converted to/from int at the Save/LoadTrack boundary.
# A direction holds only keyframes; each names the part it poses.
[directions.2]
  [[directions.2.keyframes]]
  part_id = 1
  time_ms = 0
  row = 0
  col = 0
  x = 0.0
  y = 0.0
  z = 20.0
  rotation_deg = 0.0

  [[directions.2.keyframes]]
  part_id = 2
  time_ms = 0
  row = 0
  col = 0
  x = 0.0
  y = -12.0
  z = 25.0
  rotation_deg = 0.0

  [[directions.2.keyframes]]
  part_id = 3
  time_ms = 0
  x = 10.0
  y = -4.0
  z = 26.0
  rotation_deg = 0.0

[directions.0]
  # ... same three parts, posed independently for this facing ...
```

(Only direction `2` is posed here for brevity. `0`/`1`/`3` reference the
same three parts - the rig is shared - but each holds its own keyframes,
and a direction that poses a part nowhere simply has no keyframes for it.)

### `.sprsh` (a sprite sheet template) — TOML

```toml
name = "human_body_template_sheet"
file_path = "human_body_template_sheet.png"
cell_w = 32
cell_h = 48
pivot_x = 16.0
pivot_y = 24.0
```

---

## Runtime design guidance (not built this pass)

This session is editor + data model only — no game-side loader is being built now. But the data model above was shaped with these constraints in mind, and a future runtime implementation should hold to them:

- **Asset data (Track: parts/keyframes/sheet templates) is loaded once and shared by reference** across every on-screen instance using it. Never clone a Track per instance.
- **Per-instance state is small and separate from asset data.** Sketch of the shape a character-controller-style consumer would want:

```go
type AnimatedInstance struct {
    Track     *Track
    Direction int                // set by movement/facing logic (0=up/1=right/2=down/3=left)
    ElapsedMs uint32             // playback position within Track's active Direction
    Props     map[string]string  // "hair":"long_blonde", "arms":"chainmail", ...
    // + cached resolved sheet/row/col per part — recomputed only when Props change,
    // never every frame
}

func (a *AnimatedInstance) SetTrack(t *Track)        // state machine calls this on transition
func (a *AnimatedInstance) SetDirection(dir int)
func (a *AnimatedInstance) SetProp(name, value string)
func (a *AnimatedInstance) Advance(deltaMs uint32)   // every frame
func (a *AnimatedInstance) ResolvedParts() []ResolvedPart // every frame, for the renderer
```

- **Resolve prop → sheet once when a prop changes, not every frame.** Only transform interpolation (X/Y/Z/rotation from keyframes) needs to run every frame; the sheet lookup is comparatively rare (equip changes, not per-tick).
- **Sprite sheets are shared, externally-referenced resources — never embedded per-Track.** Hundreds of characters sharing a hairstyle should draw from the same loaded texture, not duplicate it.
- **Z-sorting hundreds of instances' parts every frame is not a real cost** — even 200 instances × ~10 parts is a few thousand comparisons, microseconds on any modern CPU. Don't design around avoiding this.
- **Texture atlas packing / batching is a separate, later concern**, built once there's real content to pack — a dedicated exporter tool, not part of this editor. The one thing that would be expensive to retrofit (and is already satisfied by the design above) is sheets being shared/named resources rather than embedded blobs, so a future packer can find and repack them.

---

## Explicitly out of scope / deferred

Dropped from v1, confirmed during design discussion, not carried into v2:

- **Hitboxes** (collision/attack boxes) and **events** (sound/particle/shake/flash) — dropped for this pass. Flagged as likely to partially return: "hit spark"-style one-off effects at a specific timeline instant are exactly what the old events system was for, and combat will probably need *something* like it. Not designed here — revisit when combat is actually being built.
- **"Slash trail"-style motion-trail effects** — identified as a renderer-level concern that reads recent position/rotation history, not a rig/editor concern. The `.anif` format doesn't need to encode this at all.
- **Character-level manifest/grouping file** — considered (see [Props](#props)), rejected in favor of per-Track schema + a future linter.
- **Game-side loader/renderer** — not built this pass; see [Runtime design guidance](#runtime-design-guidance-not-built-this-pass) for constraints a future implementation should follow.
- **Atlas packing / export pipeline** — deferred to a dedicated future session, once there's real authored content to pack.

---

## Open questions — genuinely unresolved

These came up during design and were **not** settled. Don't treat any of the following as decided:

1. **How does one prop resolve to multiple physical sheets?** `arms` governs both `Arm_Left` and `Arm_Right`, but a single prop value ("chainmail") can't literally name two different images. Working assumption floated but never confirmed: derive each Part's actual sheet name by combining the prop value with the Part's own name (e.g. `"chainmail" + "Arm_Left"` → sheet `"chainmail_arm_left"`). Needs an explicit decision before implementation.
2. **"Bent state" for weapons** (a sword's damaged-appearance variant) — raised as needing either a second, orthogonal prop-like input on the same Part (weapon *and* a damage flag both affecting which sheet/cell shows), or some other mechanism. Not resolved.
3. **Hit-spark/event system revival** — see [Explicitly out of scope](#explicitly-out-of-scope--deferred). Needs actual design once combat is in scope, not just a flag.

---

## Editor implementation plan

**Deleted** (from `animaker/pkg/editor` and `animaker/pkg/ui`): `Animation`/`KeyFrame`/`SpriteReference`/`NestedAnimation` (old shape — replaced by the new one above, not the same thing despite the similar name)/`GridConfig`+`ComputeGridSprites` (uniform-grid-only slicing)/hitbox drag UI in `canvas.go`/event dialogs/the single-row flipbook timeline in `timeline.go`.

**Kept/adapted**: image loading (`file.LoadImage`), TOML save/load scaffolding (`file/toml.go`), the undo-stack skeleton, the playback ticker, the `Project` wrapper concept. The existing hitbox drag-handle code in `canvas.go` (corner-handle drag-to-resize) is the right starting point to adapt for defining a sheet template's grid + placing its pivot.

**New editor workflow**:
1. Import a sheet → define grid cell size (W×H) → place one pivot for the whole sheet (drag-adjustable) → saved as a `.sprsh`.
2. Create/open a Track (`.anif`) → define its `Props` → add Parts per Direction (name, kind, governing prop if Sheet / nested path + bindings if NestedAni).
3. Timeline becomes multi-row: one row per Part, not one row per whole-body frame, with a shared time ruler.
4. Placing a Sheet-part keyframe: pick a cell from whichever sheet the governing prop currently resolves to, position/rotate it on the canvas.
5. Placing a NestedAni-part keyframe: position/rotate only; configure prop bindings once when the part is added.
6. A "preview props" panel to live-test different prop combinations without changing the Track's declared defaults.
7. Direction tabs to switch which Direction's timeline is being edited/previewed.

---

## Worked example

Grounding the abstract model in the concrete case that shaped it:

- `human_walk.anif` — a `Track`. Declares props `hair`, `arms`, `legs`, `head`, `body`. Has 4 `Directions` (keys `0`/`1`/`2`/`3` = up/right/down/left), each independently authored with `Parts`: `Body`, `Head`, `Hair`, `Arm_Left`, `Arm_Right`, `Leg_Left`, `Leg_Right`.
- `Hair` is a Sheet part, `governing_prop = "hair"`. Its keyframes reference cells of whichever sheet the `hair` prop currently names.
- A hairstyle content library ships `human_hair_v2_template.sprsh` — cell size and pivot fixed once. A sheet named `long_blonde` and another named `short_spikey` both conform to it: same cell size, same pivot, same `(row, col)` meaning per pose (`idle-0`, `idle-1`, `flinch`, `running-0`...`running-3`, laid out as columns = direction, rows = animation+frame). `human_walk.anif`'s `Hair` keyframes for a given walk frame reference, say, `(row=4, col=2)` — whatever hairstyle is currently equipped, that cell is used.
- An artist authoring `human_burning.anif`'s `Hair` keyframes can deliberately reference the same cell used for "flinch" elsewhere (reuse, not automation) if it looks right for a burning reaction — nothing forces a 1:1 mapping between Track names and cell rows.
- `human_torch_run.anif` — a `Track` with a `Torch` Part of kind `nested_ani`, `nested_ani_path = "base_wood_torch.anif"`. Its `direction` binding passes through from `human_torch_run`'s own active direction, so the torch turns with the character while its own 8-frame flicker loop keeps playing independently, unsynced to the walk cycle.
- `human_walk.anif`, `human_idle.anif`, `human_attack_sword.anif` are three separate files, each independently declaring the same `hair`/`arms`/`legs`/`head`/`body` props (Option C — no shared manifest). A future linter should flag it if any of them drift out of agreement, since a running `AnimatedInstance`'s `Props` map is expected to keep applying correctly across a state-machine transition between all three.

---

## History: what v1 was

The original build (before 2026-09-18) was a flipbook editor: `Animation` had one flat `KeyFrames []*KeyFrame` list, each keyframe holding exactly one `SpriteReference` (a single whole-body sprite), plus per-keyframe `HitBox`/`AttackHitBox`/`Events`, and a `NestedAnimation` concept for composing `.anif` files that's superficially similar to but mechanically different from the nested-animation Part kind in this spec (v1's nested animations weren't part of a multi-part rig — there was no such thing yet). Sprite sheets were sliced by a uniform `GridConfig` (fixed cols/rows, one tile size for the whole sheet) with no per-cell pivot concept. None of this is being carried forward; see [Editor implementation plan](#editor-implementation-plan) for exactly what's deleted.
