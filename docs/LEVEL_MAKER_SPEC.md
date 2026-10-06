# Level Maker & World Format — Technical Specification

**Version**: 1.0
**Status**: **Proposal as of 2026-10-05; build order steps 1 (`shared/world`) and 2 (`cmd/blobtemplate`) built 2026-10-05, steps 3–4 (game loads, draws and validates against levels) built 2026-10-06** — see the checkboxes under [Build order](#build-order). Nothing renders or loads levels in the game yet. It merges three sources: the level-file sketch in [ARCHITECTURE.md](ARCHITECTURE.md#level-file-toml) (which this replaces), an associate designer's tilemap/autotile draft (Python, reviewed 2026-10-05), and a decision interview held the same day. Every decision below records where it came from so it can be revisited knowingly.
**Language**: Go
**Program**: `cmd/levelmaker` — a separate executable inside `module game`, rendering through the same `client/render` package the client uses
**Rendering**: raylib (`github.com/gen2brain/raylib-go`), same as the client
**File format**: one TOML file plus one gzip-compressed binary grid file per level

---

## The model in brief

1. **The artist paints the ground layer, and the engine handles ground transitions.** Terrain is brushed on, and the blob-47 solver picks every edge and corner tile (D9, D23). Where the artist wants a transition the solver wouldn't choose (a 2×2 dirt hole made only of edge pieces), they **lock** ground cells and pick the tile and what shows through it by hand. The engine leaves locked cells alone (D3). Overlays like cracks or moss go on the `decor` layer (D28).
2. **The artist builds on top with the `decor`, `ysort` and `overhead` layers, placing every tile by hand.** Tiles may use transparency, so the ground shows through and the piece sits naturally on any terrain. A tile without transparency gives a hard edge, a solid block meeting whatever is beside it. Which pieces and variants go where is always the artist's choice. The engine never picks or swaps upper-layer tiles (D24, D27).

Interactive things (doors, chests, cuttable bushes, NPCs) are script objects, not tiles (D26).

---

## Table of Contents

1. [Decisions](#decisions)
2. [World model](#world-model)
3. [Layers](#layers)
4. [Terrains and tiles](#terrains-and-tiles)
5. [Autotiling: blob-47, per terrain, layered by priority](#autotiling-blob-47-per-terrain-layered-by-priority)
6. [Cell properties: blocking, surface, interaction](#cell-properties-blocking-surface-interaction)
7. [File formats](#file-formats)
8. [Runtime: loading, the server, streaming](#runtime-loading-the-server-streaming)
9. [Editor v1](#editor-v1)
10. [Later versions and deferred work](#later-versions-and-deferred-work)
11. [Build order](#build-order)
12. [Open questions](#open-questions)
13. [What happened to the designer's draft](#what-happened-to-the-designers-draft)

---

## Decisions

Confirmed in the 2026-10-05 interview unless marked otherwise.

| # | Decision | Source |
|---|---|---|
| D1 | Go only. The designer's Python is treated as a spec, not code to keep. | Interview |
| D2 | Grids are stored as dense per-layer arrays, not per-cell objects. A cell's tile is a `uint16` index into a tile table (the existing `VisualLayer.Tiles` shape), not a string. | Interview — changes the designer's draft |
| D3 | **Lock flag, with a hand-picked underlay.** A locked ground cell keeps three things: its **terrain** (what neighbours and the property defaults see), a hand-picked **tile**, and a hand-picked **underlay terrain** (what shows through the tile's transparent parts). The solver never rewrites a locked cell. Example: a 2×2 dirt hole in grass made of four grass-edge pieces. The cells stay `grass` so the surrounding grass doesn't react, and each gets a grass-edge tile over a dirt underlay. Painting terrain over a locked cell unlocks it. If neighbours are repainted later, keeping locked cells looking right is the artist's job. | Designer; reinstated in the interview after briefly being dropped for `decor`, which can't show a different terrain through a piece's transparent parts |
| D4 | `flip_h` / `flip_v` flags per cell. Each tile in the tile table says whether it may be flipped, since flipping breaks directional shading. | Designer, caveat added |
| D5 | Layers are a named, ordered list, each with a z-range, merging the designer's named layers with the existing z-range model. See [Layers](#layers). | Both |
| D6 | **Sound and speed belong to the cell's `surface` and `interaction`, not to each tile's art.** Terrains and tiles declare defaults, and the artist overrides per cell (D29). Speed multipliers live on interaction types and must be known to the server's speed check. | Interview, revised: superseded the per-terrain `footstep_sfx`/`speed_multiplier` and the sparse `cell_tags` table |
| D7 | Game-side level loading is built alongside the editor, so every saved map can be played on the real client and server immediately. | Interview |
| D8 | Tile art is PNG. (`assets/tileset.png` is currently a JPEG, which smears pixel art and has no transparency.) | Interview |
| D9 | Autotile = **blob-47, one sheet per terrain, layered by terrain priority** (not one sheet per terrain *pair*). | Interview — changes the designer's draft |
| D10 | **Cell properties are derived from defaults plus per-cell overrides** (the designer's `type_override`, generalised to all three maps in D29), compiled on save into fast per-cell grids. | Interview |
| D11 | Elevation (multi-tier walking, bridges) is **deferred**. The file format has room for it but stores nothing yet. | Interview |
| D12 | The tool is a separate program in the game module (`cmd/levelmaker`). Level rendering lives in a `client/render` package (next to `client/anim`) used by both client and editor. | Interview |
| D13 | Editor v1 scope is **terrain painting + autotile** only. The format still stores flip and override from day one; their tools come later. | Interview |
| D14 | The world is **one seamless plane**. Levels are **any size, placed freely** at a world position, and may not overlap. | Interview |
| D15 | Clients get level data **streamed from the server** over an **HTTP side-channel with a hash-keyed disk cache**. UDP only announces which levels and versions are nearby. | Interview |
| D16 | The editor shows **one level plus read-only strips of its neighbours**. Saving re-solves the border tiles on both sides of an edge. | Interview |
| D17 | **Every interior is a separate level reached by a warp tile**: ground floors, basements, upper floors, caves, dungeons. To build one, the artist copies the exterior (house plus surroundings) into a new level with the editor's *copy area to new level* tool, removes the roof by hand, and builds the interior. No roof-fade, no floor/stacking concept. | Interview (revised same day; replaced an in-place roof-region design) |
| D18 | **For now, load every exterior level in full.** The server and every client hold the whole overworld (all non-`isolated` levels), plus the one interior the player is in. Load radius, partial loading and maximum level size are deferred until world size makes them matter. | Interview (replaced a load-radius design) |
| D19 | Water is a terrain, not its own layer. The designer's `water_1` layer is folded into terrain priority. | Follows from D9 |
| D20 | Out-of-world cells count as "joined" for autotile, so the edge of the world never draws a border. Empty cells inside a level do **not** count as joined. This fixes the designer's solver, which treated every missing neighbour as matching. | Fix to draft |
| D21 | **Blacked-out surroundings in interiors**: copied surroundings outside the building are black and blocked. Outside the level's rectangle this is just void on a black background. Inside it, the artist paints an ordinary **`black` terrain** (one plain tile, collision `blocked`, no edges), so no special fog feature is needed. | Interview (revised same day; replaced a per-cell fog grid) |
| D22 | Interior levels are marked `isolated`: no neighbours, no neighbour loading, and no cross-edge autotile while inside. | Follows from D17/D21 |
| D23 | **Only the ground layer has terrain and transition tiles.** Upper layers (`ysort`, `overhead`, extras) hold hand-placed tiles only, with no terrain or underlay grids. | Interview |
| D24 | **Only ground is brush-painted. Everything else is hand-placed by the artist**: cliffs, fences, walls, trees, bushes, rocks. Placement uses single tiles or multi-tile **stamps** (a rectangle picked from a sheet, e.g. a whole tree). There is no cliff tool or auto-connecting fences. | Interview |
| D25 | **Tiles can declare property defaults** in the tile table: a fence tile blocks ground, a bridge plank has a `wood` surface. This moved from "later" to editor v2, because hand-placed scenery has to block movement. | Follows from D24 |
| D26 | Bushes, rocks and pots are **plain scenery tiles**. Anything a player interacts with (cut, lift, open, talk) is a **script object**, a `GameObject` with a script, not a tile. The scripting system is its own future design and out of scope here. | Interview |
| D27 | **Only ground uses transition sets. Every other transition is transparency plus the artist's choice.** Upper-layer art (fences, cliffs, trees, bushes, roofs) is drawn on a transparent background, so the ground beneath shows through and the generic pieces work on any terrain. Where a piece meets the ground, the artist may draw a **few base variants** (a fence post with grass growing up it, a post sunk into water or swamp, rocks bedded in dirt), also with transparency, and **picks one by hand** for each spot. The editor never chooses variants and no full per-terrain set is needed. Cliffs are never ground. | Interview (refined same day: base variants allowed) |
| D28 | **A `decor` layer for flat overlays.** It sits between `ground` and `ysort`, is always drawn under players and is never sorted. Its tiles may declare property defaults like any other (D29); a rug might set a `wood` surface. It's for transparent detail that should show the ground beneath: cracks, moss, puddles, rugs. Hand-arranged *terrain transitions* belong in locked ground cells (D3), not decor, because a transparent edge piece on decor would show the ground's own terrain through it. Extra decor layers (`decor_2`…) are allowed. | Interview |
| D29 | **Three per-cell property maps**, each with defaults and per-cell artist overrides. **`blocking`**: flags for what can't pass (ground, projectile, flight, jump, roll, ethereal, magic). **`surface`**: the material, which drives footstep sound and effects (dust, splash). **`interaction`**: how movement behaves (normal, swimming, slow…). Surface and interaction lists are extendable. | Interview |
| D30 | **How defaults combine.** `blocking` uses **strictest wins**: the terrain's flags OR every placed tile's flags, so anything that blocks, blocks. A bridge over a blocking chasm therefore needs a hand-painted override. `surface` and `interaction` use **topmost wins**: the topmost tile that declares one, else the ground terrain. An artist override beats everything. | Interview |
| D31 | **Hazards (lava, spikes, poison) are scripted**, not tile or terrain properties (see D26). | Interview |
| D32 | **Authority split.** `blocking` and `interaction` are **server-authoritative**: the server compiles them from its own copy of the level files, validates movement against them (blocked passage, speed multipliers), and never trusts a client's values. Client prediction uses the same values only to stay in step. `surface` is **client-only**: it drives sound and effects, so the server ignores it and a tampered value only changes what that player hears and sees. | Interview |
| D33 | **One shared camera** in `client/render`, with two modes. **Follow**: the game's player camera, with the player's zoom limits. **Free**: fly anywhere, ignoring walls; zoom out further than players can; jump to a spot; follow any chosen target. The level maker uses free mode now. The same camera serves a future observer/spectator view. | Interview |
| D34 | **Live spectating is not built now.** When it is, it's **localhost-only**: the server accepts spectators only from its own machine, and there are no tokens or accounts. A spectator is invisible, has no character, and can **fly, zoom, follow a player and show debug overlays** (property maps, level borders, player IDs, suspicion scores). It **cannot act on the world** (no teleport, kick or live edits); that would be a separate admin-tools design. | Interview |

---

## World model

```
World (one plane, world tile coordinates, origin at 0,0, y grows down)
├── world/terrains.toml        terrain definitions + tile table, shared by every level
└── levels/
    ├── meadow.level.toml      pos = [0, 0],    size = [96, 64]
    ├── meadow.grid.gz
    ├── lake.level.toml        pos = [96, 0],   size = [128, 128]
    ├── lake.grid.gz
    ├── house_1_basement...    pos = [5000, 5000]  (reached by warp only)
    └── ...
```

- **A level is a rectangle on the world plane.** Its TOML gives `pos` (world tile coordinates of its top-left corner) and `size` in tiles. World pixel position = world tile × `shared.TileSize` (16).
- **Levels may not overlap.** The editor refuses to save an overlapping level and the loader rejects one. Every world cell belongs to at most one level, so "which collision applies here" always has a single answer.
- **Neighbours are found by geometry.** Two levels are neighbours if their rectangles share an edge or touch at a corner. There is no list of links to maintain.
- **There is no central index.** The server and the editor scan `levels/` for `*.level.toml`. Adding a level means adding two files.
- **Gaps between levels are void.** A void cell blocks everything, draws nothing, and counts as "joined" for autotile (D20).
- **Coordinate limits.** Positions stay `float32` world pixels on the wire (`shared.Vec2`). Precision is still 1/32 px about 16,000 tiles from the origin, so keep the world within ±16,000 tiles. Interiors parked "off to the side" (D17) should sit well clear of the overworld but inside that bound.
- **Interiors.** A warp tile (a `GameObject` of kind `warp` with a `destination` level and position) moves the player instantly. Warps are server-authoritative: the server moves the player, and a client-side "I warped" claim is never trusted.
- **Interiors are copies, not views.** An interior level starts as a copy of an area of the exterior (all layers, terrain, flags, overrides) placed elsewhere on the plane, and the artist then edits it freely: removing the roof, building rooms, adding the return warp. Consequences, accepted 2026-10-05:
  - **The copy is a snapshot.** Later changes to the exterior don't reach the interior. If the street outside is repainted, the artist updates the interior by hand or re-copies.
  - **The surroundings in an interior are scenery.** It's a different level, so players and NPCs walking past outside are not visible from inside, and vice versa.
- **Black terrain hides the copied surroundings** (D21). Anything outside the building in an interior is blacked out and blocked:
  - **Outside the level's rectangle**, this happens on its own: void draws nothing over a black background and is blocked.
  - **Inside the rectangle but outside the building** (the corners around an L-shaped house, a garden the copy picked up), the artist paints the `black` terrain with the normal terrain brush, and erases anything left on the upper layers there (stray tree canopy, roof edges).
  - The *copy area to new level* tool can do this in one step: given a second, building-shaped selection, it copies nothing outside it and fills that area with `black`.
- **Interior levels are isolated** (D22). A level with `isolated = true`:
  - is never treated as anyone's neighbour;
  - doesn't load neighbouring levels while the player is in it;
  - doesn't autotile across its edges.

  So two interiors parked next to each other on the plane never show through each other's darkness, and the client holds only that one level while inside.

---

## Layers

Each level has an ordered list of layers. Default set:

| Layer | z-range | Holds | Terrain + autotile? | Designer's name |
|---|---|---|---|---|
| `ground` | 0–5 | terrain (grass, dirt, sand, water…) | yes | `ground_0`, `water_1` |
| `decor` | 6–10 | flat hand-placed overlays drawn under players: cracks, moss, puddles, rugs (D28) | no: hand-placed tiles | (new) |
| `ysort` | 11–50 | tall tiles drawn depth-sorted with players and NPCs (tree trunks, fences, walls) | no: hand-placed tiles | `ysort_2` |
| `overhead` | 51–70 | drawn above players (tree canopy, roofs) | no: hand-placed tiles | `overhead_3` |

The z-ranges are the existing ones from [ARCHITECTURE.md](ARCHITECTURE.md) and `client/renderer.go` (`SortObjectsForRendering` sorts z 11–50). Layers are not limited to these four: a level may add more (a second decor layer, say), each with its own z-range. In v1 only `ground` is painted by the editor.

**What goes where.** Ground is the flat surface you walk on, which fills areas and blends into its neighbours. Anything with height, an outline, or something you could walk behind is hand-placed on another layer (D24):

| Thing | Layer | How |
|---|---|---|
| Grass, dirt, sand, water, paths, floors, `black` | `ground` | terrain brush, autotiled |
| Cracks, moss, puddles, rugs | `decor` | hand-placed tiles/stamps; never sorted; usually declares nothing (a rug might set a `wood` surface) |
| Hand-arranged terrain transitions (a 2×2 dirt hole of edge pieces) | `ground`, locked cells | pick tile + underlay by hand (D3) |
| Fences, walls, signposts, bushes, rocks, pots | `ysort` | hand-placed tiles/stamps; blocking from the tile table |
| Cliffs (face and top lip) | `ysort` / `overhead`, never `ground` | hand-placed; blocked. No one walks on top until elevation (D11) |
| Trees | trunk on `ysort`, canopy on `overhead` | two stamps, one per layer (a saved multi-layer "prefab" stamp is a possible later convenience) |
| Roofs, archways | `overhead` | hand-placed |
| Anything interactive (cuttable bush, chest, door, NPC) | object list | script object (D26), not a tile |

**Transitions off the ground are transparency** (D27). Only terrain on `ground` blends through blob-47 sheets and underlays. Hand-placed art is drawn on a transparent background, so the ground beneath shows through. For example, a 4×2 fence:

```
||——||      top row:    post, rail, rail, post     (generic, transparent)
||——||      bottom row: post base, rail, rail, post base
```

The rails and post tops are reused everywhere. Whether a post base looks any different, and which version goes where, is **the artist's choice**: a plain base, grass growing up it, or sunk into water. The artist draws only the bases they want, each with transparency, and stamps whichever one they choose at each spot. Nothing about the ground underneath selects or changes a tile automatically. A few hand-picked variants plus transparency replace a full transition set. This is also why tile art must be PNG with alpha (D8).

**Only `ground` has terrain** (D23). Terrain, transition tiles and the underlay exist only on the ground layer. Every other layer is just hand-placed tiles plus flags. There is exactly one ground layer, always first in the list. If auto-connecting pieces on upper layers (fences, walls) are ever wanted, they'll be designed as their own feature rather than by stretching terrain.

---

## Terrains and tiles

Both live in `world/terrains.toml`, not per level, because autotiling across a level edge only works if both levels agree on what "grass" is.

```toml
# Tile sheets: each is a PNG cut into 16x16 cells, numbered left-to-right, top-to-bottom.
# A tile's global index = sheet.base + cell. Index 0 is reserved for "no tile".
[[sheets]]
name = "grass_blob47"
path = "assets/tiles/grass_blob47.png"
base = 1            # global tiles 1..47
cells = 47          # cells in use: reserves base..base+cells-1, so ranges can be checked for overlap
flippable = false   # D4: default for every cell on this sheet

[[sheets]]
name = "dirt_blob47"
path = "assets/tiles/dirt_blob47.png"
base = 48
cells = 47

# Terrains: id 0 is reserved for "empty". Priority decides who draws on top at a border.
[[terrains]]
id = 1
name = "water"
priority = 10                 # lowest: everything draws over water
sheet = "water_blob47"
blocks = []                   # D29 defaults
surface = "water"
interaction = "swimming"

[[terrains]]
id = 2
name = "dirt"
priority = 20
sheet = "dirt_blob47"
surface = "dirt"              # blocks and interaction omitted = nothing blocked, "normal"

[[terrains]]
id = 3
name = "grass"
priority = 30
sheet = "grass_blob47"
surface = "grass"

[[terrains]]
id = 4
name = "black"            # D21 - blacks out copied surroundings in interiors
tile = "black_tiles:0"    # a single tile instead of a blob-47 sheet
edges = false             # no edge art, and neighbours treat it as joined, so it meets them with a hard edge
blocks = ["ground", "projectile", "flight", "jump", "roll", "ethereal", "magic"]
surface = "none"

# D29 property lists. Ids are stored in the grids; names are what files use. Both lists are extendable.
[[surfaces]]
id = 0
name = "none"

[[surfaces]]
id = 1
name = "grass"
footstep_sfx = "step_grass"
effect = "grass_rustle"     # particles; optional

[[surfaces]]
id = 2
name = "water"
footstep_sfx = "water_splash"
effect = "splash"
# ... dirt, sand, wood, stone, marble

[[interactions]]
id = 0
name = "normal"
speed_multiplier = 1.0

[[interactions]]
id = 1
name = "swimming"
speed_multiplier = 0.5      # must be honoured by client prediction AND the server's speed check

[[interactions]]
id = 2
name = "slow"
speed_multiplier = 0.6

# Tile property defaults (D25). A tile with no entry declares nothing.
[[tile_props]]
sheet = "fences"
cells = [0, 1, 2, 3, 8, 9, 10, 11]   # the 4x2 fence
blocks = ["ground", "roll"]          # jumpable, arrows fly over

[[tile_props]]
sheet = "bridges"
cells = [0, 1, 2]
surface = "wood"
interaction = "normal"
```

- Every sheet declares `cells`, so `LoadDefs` can check that tile ranges don't overlap without opening the PNG. A tile can also set `flippable` in its `tile_props` entry, overriding its sheet's default (D4).
- Terrains with edges must have **distinct priorities**: two equal ones would join each other both ways and draw no border at all. `LoadDefs` rejects that.
- Surface id 0 and interaction id 0 must exist; they're what a cell compiles to when nothing declares a value (conventionally `none` and `normal`).
- Terrain ids are `uint8` (255 terrains). Tile indices are `uint16` (65,535 tiles across all sheets).
- Surface sounds, effects and speed multipliers are accepted from day one, but nothing honours them until movement and audio code exists. Speed is the risky one: client prediction and the server's `CheckSpeed` must apply the same multiplier, or honest players get flagged.

---

## Autotiling: blob-47, per terrain, layered by priority

**What the artist draws:** one 47-cell sheet per terrain, showing that terrain's edges and corners over transparency. No `grass_to_dirt` pair sheets.

**What the solver does, per cell of terrain T on the ground layer:**

1. **Neighbour mask.** Each of the 8 neighbours counts as *joined* if it is a world-void cell (D20), has an `edges = false` terrain such as `black`, or its terrain's priority is **≥** T's. A cell whose own terrain has `edges = false` skips the solver and just draws its single tile. Neighbours across a level edge are read from the neighbouring level. Bits are N=1, NE=2, E=4, SE=8, S=16, SW=32, W=64, NW=128.
2. **Reduce to 47.** A corner bit (NE, SE, SW, NW) is kept only if both edges next to it are also set. For example, NE counts only when N and E are both joined, because otherwise the corner is already covered by an edge. This turns 256 raw masks into exactly 47. *(The designer's solver skipped this step, which is why its tile ids ran to 256.)*
3. **Pick the tile.** The 47 reduced masks are sorted ascending. A mask's position in that list is its cell on the sheet, so tile = `sheet.base + state`. The template generator draws the same order, so art and solver can't disagree.
4. **Underlay.** If any neighbour has a lower-priority terrain, the cell also gets an *under* tile: the full centre tile (state 46, mask 255) of the highest-priority such terrain. That's what shows through the transparent parts of T's edge tile. The renderer draws `under`, then `tile`.
5. **Locked cells** (D3) are skipped: their tile and underlay are the artist's. They still take part in their neighbours' masks through their terrain.

**Three terrains meeting** (grass over dirt over water): the grass cell gets a dirt underlay and a grass edge, and the dirt cell gets a water underlay and a dirt edge. No three-way tiles are needed. The known artefact: a grass cell diagonally touching water, with dirt on its other sides, shows grass-over-dirt at that corner rather than grass-over-water. Small at 16 px, and fixable with a locked cell. Or, if it matters somewhere, the artist covers it with custom art on a layer above (D3).

**Re-solving is local.** Painting a cell re-solves that cell and its 8 neighbours, never the whole layer.

**Template generator** (`cmd/blobtemplate`): writes a labelled 8×6 PNG (47 used cells and 1 spare) for a terrain. Each cell shows a mini-diagram of which neighbours are joined, rather than a name, so labels can't drift from the solver. It's the Go replacement for the designer's `generate_blob_47_template`.

---

## Cell properties: blocking, surface, interaction

Every cell has three properties (D29). Each is compiled on save into its own per-level grid, separate from the art, so the server and client can read them quickly.

| Map | Stored as | Values | Read by |
|---|---|---|---|
| `blocking` | `u16` bit flags | `ground`, `projectile`, `flight`, `jump`, `roll`, `ethereal`, `magic` (7 used, 9 spare); **server-authoritative** (D32) | server movement validation and anti-cheat, client prediction, later combat and pathfinding |
| `surface` | `u8` id | `none`, `dirt`, `sand`, `grass`, `wood`, `stone`, `marble`, `water`… (extendable); **client-only** (D32) | footstep sound, dust/splash effects |
| `interaction` | `u8` id | `normal`, `swimming`, `slow`… (extendable); each defines a speed multiplier; **server-authoritative** (D32) | movement on both client and server |

**How a cell's value is decided** (D30):

```
blocking[cell]    = override            if the artist set one
                  = terrain.blocks | tile.blocks for every tile in the cell, on any layer
                    (strictest wins: anything that blocks, blocks)

surface[cell]     = override            if set
interaction[cell] = override            if set
                  = the topmost tile in the cell that declares one
                  = the ground terrain's value
                  = none / normal       if nothing declares one
```

- **Empty ground or void** blocks everything (all flags set). `black` terrain declares the same.
- **Overrides** are per cell and per map: the artist can override just the surface of a cell and leave its blocking alone. They're stored from day one, but the brush arrives in editor v2 (D13).
- **The anti-cheat fast path** is the `ground` bit. `MovementValidator.CheckWallPhase` and client prediction test it for walking. Movement types that clear obstacles (the server already has `MovementTypeJump`) test their own bit instead. This replaces `shared.TileType` and `CollisionLayer.IsBlocked`, which mix "can I pass" with "what happens here" (`walkable`, `swimable`, `lava`).
- **What the artist can rely on:** a bridge over water works automatically. Water doesn't block, and the bridge tile sets a `wood` surface and `normal` interaction, which win because the bridge is on top. A bridge over a *blocking* chasm needs a blocking override painted under it, because strictest-wins never lets a tile unblock.
- **Hazards are not properties** (D31). Lava, spikes and poison are script objects.
- **NPCs and monsters use the same flags as everyone else.** They're script objects (D26), and each moves as one of the existing movement kinds (ground, flight, ethereal…), so it's blocked by that kind's flag. There is no NPC-only flag. Keeping an NPC in its area (a shopkeeper behind the counter, wolves out of town) is the NPC's script, not the map's job.
- **Spare flags:** 9 of the 16 are unused. A `sight` flag (blocks line of sight) is deliberately left out until stealth or sight mechanics are designed. Decide it then. Adding a flag needs no format change: existing cells simply read "doesn't block" until painted.

---

## File formats

### `levels/<name>.level.toml` — the readable half (diffs cleanly in git)

```toml
format = 1
name = "meadow"
pos = [0, 0]          # world tiles, top-left
size = [96, 64]       # tiles
isolated = false      # true for interiors (D22): no neighbours, nothing else loaded while inside

[[layers]]
name = "ground"
z_min = 0
z_max = 5

[[layers]]
name = "decor"
z_min = 6
z_max = 10

[[layers]]
name = "ysort"
z_min = 11
z_max = 50

[[layers]]
name = "overhead"
z_min = 51
z_max = 70

[[objects]]                 # shared.GameObject; x/y in level pixels
id = 1
kind = "warp"
x = 744.0
y = 448.0
z = 0
collision_type = "passthrough"
[objects.properties]
destination = "house_1_basement"
dest_x = "64.0"
dest_y = "32.0"
```

### `levels/<name>.grid.gz` — the bulk half (gzip, little-endian)

```
"LVLG"            4 bytes magic
version           u16   = 1
width, height     u32, u32         must match the TOML size
layer_count       u8               must match the TOML layer list, same order
ground layer (always layer 0, D23):
  terrain         [w*h]u8          0 = empty
  tile            [w*h]u16         0 = none
  under           [w*h]u16         0 = none (locked cells: the hand-picked underlay)
  flags           [w*h]u8          bit0 locked; bits 1-7 reserved (elevation, D11)
each other layer:
  tile            [w*h]u16         0 = none
  flags           [w*h]u8          bit0 flip_h, bit1 flip_v; bits 2-7 reserved
override_mask     [w*h]u8          bit0 blocking, bit1 surface, bit2 interaction overridden (D29)
override_blocking [w*h]u16         used where bit0 set
override_surface  [w*h]u8          used where bit1 set
override_interact [w*h]u8          used where bit2 set
blocking          [w*h]u16         compiled (D30)
surface           [w*h]u8          compiled
interaction       [w*h]u8          compiled
```

- For scale, a 128×128 level with 3 layers is about 115 KB raw and typically a few KB gzipped. Terrain grids compress very well.
- **Version hash** = SHA-256 of the TOML bytes followed by the grid bytes. This is the cache key for streaming (D15).
- `shared.Level` and `shared.VisualLayer` gain the fields this needs (`Pos`, `Terrain`, `Under`, `Flags` and so on). Their JSON tags become irrelevant, because nothing serialises a level as JSON.

---

## Runtime: loading, the server, streaming

**Loading (both sides).** A new pure-Go package (no raylib, so the server can import it), e.g. `shared/world`, provides:
- reading and writing level files;
- the autotile solver, so the editor and tests share one implementation;
- a `World` that holds loaded levels and answers `TileAt(worldX, worldY)` / `IsBlocked` across level boundaries.

**Server.**
- At startup the server loads every level and compiles `blocking` and `interaction` from its own files (D32). It never accepts these values from a client, and it ignores `surface`.
- `MovementValidator` asks the `World` instead of one hardcoded `shared.CollisionLayer`, testing the compiled `blocking` grid (D29), which makes the wall-phase check work across level edges. This replaces `shared.NewCollisionLayer(100, 100)` at `server/main.go:32`.
- Warps are handled server-side.

**Client.**
- The client loads levels near the player into its own `World`, and prediction collides against it. This replaces the placeholder grid at `client/main.go:59`.
- A new `client/render` package (raylib) draws visible layers with a 2D camera and y-sorts the `ysort` layer together with players. Void cells show the black background, so the clear colour is black, not the current `rl.RayWhite`. **This is new code, not an extraction:** today the client draws no tiles, has no camera, and `client/renderer.go` only sorts `GameObject`s.

**Streaming (D15).**
- The server runs a small HTTP file server alongside its UDP socket, serving `GET /levels/<name>/<hash>.toml` and `/levels/<name>/<hash>.grid.gz`. Putting the hash in the path makes responses cacheable forever.
- A new UDP message, sent on join and whenever the world changes, lists every exterior level as `{name, hash, pos, size}` (D18). Entering an interior sends that one interior's entry.
- The client fetches whatever isn't in its disk cache (keyed by hash). With the whole overworld loaded, nothing is dropped except interiors the player has left.
- Until a neighbour level has arrived, its cells count as blocked for prediction. The server never trusts the client's copy anyway.
- Phase 1 can skip streaming: both sides read `levels/` from disk, and the HTTP path comes later without changing the file format.

The new UDP message gets a row in [PROTOCOL_REFERENCE.md](PROTOCOL_REFERENCE.md) when it is defined in `shared/protocol.go`, not before.

---

## Editor v1

`cmd/levelmaker` — raylib, rendering through `client/render`, so it looks exactly like the game.

**In v1 (D13: terrain painting + autotile):**
- New level (name, position, size; refused if it overlaps another), open, save.
- Terrain palette listing the terrains from `world/terrains.toml`.
- Brush (sizes 1/3/5), rectangle fill and flood fill, painting terrain on `ground`. Each stroke re-solves locally.
- Eraser (sets terrain to empty).
- Pan and zoom.
- Read-only strips of neighbouring levels drawn around the edited level (D16). On save, border cells on both sides are re-solved, and the affected neighbour grid files are rewritten too.
- Undo/redo per stroke.
- Every error and crash goes to a session log, following the animaker convention.

**Not in v1:** manual tile stamping, flip tools, property overlays and override painting, the `ysort`/`overhead` layers, objects, warps, copy area to new level. (The `isolated` flag is read and honoured by the game from step 1; only the tool that sets it comes later. The `black` terrain can be painted in v1 like any other.)

---

## Later versions and deferred work

| When | What |
|---|---|
| Editor v2 | Hand placement (D24): single tiles and multi-tile stamps from a tileset palette, on upper layers, and on `ground` as locked cells with a hand-picked underlay (D3); flip tools; per-tile property defaults in the tile table (D25); overlays for the three property maps plus override brush |
| Editor v3 | Objects (NPCs, warps with a destination picker, doors), **copy area to new level** (select a rectangle, choose a free world position, get a new `isolated` level containing a copy of every layer; D17/D22). Optionally a second building-shaped selection fills everything outside it with `black` (D21), and the tool offers to create the warp pair (door to interior, exit back to door) in the same step |
| Later | **Live spectator (D34)**: a localhost-only connection with a new keep-alive `Spectate` message (a client only counts as joined while it sends `Move`, so spectators need their own). The server includes spectators in its broadcast without giving them a character. Adds the follow-player picker and live overlays (player IDs, suspicion scores, which needs a new server-to-spectator message). Builds on the camera and overlays from steps 5 and 7 |
| Later | Whole-world canvas (editing across levels on one canvas); animated tiles (water); script objects (D26, needs a scripting design first) |
| Deferred (D11) | Elevation and multi-tier walking (bridges, over/under). Needs per-elevation collision, a value in player state, and changes to server validation and the protocol |
| Deferred | Asset compiler / packing; zone servers (ARCHITECTURE.md "Scaling") |

---

## Build order

Each step is one or more PRs and ends **tested and playable**:
- `go vet ./...` and `go test -race ./...` pass, which is what CI runs.
- The `map/` cards are updated for whatever the step changed (the repo convention: `map/` is the live index of the code).
- The game still runs via `build_local.ps1`. Until levels exist, the client and server keep their current blank 100×100 grid, so nothing breaks mid-way.

```
1 shared/world ──┬── 3 client render + load ──┬── 5 levelmaker v1 ── 7 editor v2 ── 8 editor v3 + warps
                 ├── 4 server load + validate ┴── 6 streaming
2 blobtemplate ──┘  (2 can run in parallel with 1; 3 and 4 are independent of each other)
```

### Step 0: housekeeping (before code)

- [x] 0.1 Commit this spec and the doc updates (ARCHITECTURE.md, docs/CONTEXT.md, CLAUDE.md).
- [x] 0.2 CI's gofmt step now runs `gofmt -l .` over the whole tree (it used to list `client server shared animaker` by hand, so `cmd/` would have been skipped silently).

### Step 1: `shared/world`, the world model (pure Go, no raylib)

**Built 2026-10-05.** Code: `shared/world/` (`defs.go`, `level.go`, `file.go`, `autotile.go`, `props.go`, `world.go`); card: [map/objects/world.md](../map/objects/world.md). Small additions beyond the list below: `World.CheckPlacement` (for the editor's new-level overlap check), `World.SolveLevel`, `LoadDir(root)` (loads `world/` + `levels/`, or reports that there are none so callers keep their placeholder grid), and `DecodeLevel`/`EncodeLevel` on bytes for the future streaming client.

The server imports this, so no graphics dependencies. Everything after it builds on it, so it gets the heaviest testing.

- [x] 1.1 **Definitions** (`defs.go`): types for sheets, terrains, surfaces, interactions and tile props. `LoadDefs("world/terrains.toml")` checks that ids are unique, sheet index ranges don't overlap, every name a terrain or prop uses exists, and that `edges = false` terrains have a `tile`. Blocking flags are `uint16` constants (`BlockGround`, `BlockProjectile`, `BlockFlight`, `BlockJump`, `BlockRoll`, `BlockEthereal`, `BlockMagic`).
- [x] 1.2 **Level model** (`level.go`):
  - `Level{Name, Pos, Size, Isolated, Layers, Objects}`
  - a ground grid: terrain, tile, under, flags
  - upper-layer grids: tile, flags
  - override grids
  - compiled property grids
  - cell accessors by level-local `(x, y)`
- [x] 1.3 **File IO** (`file.go`):
  - `LoadLevel` and `SaveLevel` for `.level.toml` and `.grid.gz` (gzip, little-endian, layout as in [File formats](#file-formats)).
  - Writes are atomic (temp file, then rename), so a crash never leaves half a level.
  - The version hash is SHA-256 over both files.
  - Tests: round-trip; a size or layer-count mismatch is an error; a bad magic number or version is an error; the hash is stable.
- [x] 1.4 **Autotile solver** (`autotile.go`):
  - the 256→47 reduction table and the state order;
  - `SolveCell` and `SolveRect`, which read neighbours through a `TerrainSource` interface so the same code solves across level edges;
  - skips locked cells and handles `edges = false`.
  - Tests:
    - exactly 47 states exist;
    - a single grass cell in dirt gives the isolated state, and 2×2 grass in dirt gives the four outer corners;
    - the underlay is the highest-priority lower terrain, and three terrains meeting behaves as described above;
    - locked cells are untouched but still count in their neighbours' masks;
    - void counts as joined, and empty cells don't.
- [x] 1.5 **Property compiler** (`props.go`): `Compile(level, defs)` runs the D30 rules. Blocking = OR of all flags, surface and interaction = topmost tile that declares one, then overrides; empty ground blocks everything. Tests: a fence blocks ground but not jump; a bridge over water becomes wood/normal; a bridge over a blocking chasm still blocks until overridden; an override on one map leaves the other two alone.
- [x] 1.6 **World** (`world.go`):
  - `LoadWorld(levelsDir, defs)` scans for `*.level.toml`.
  - It rejects overlapping levels, naming both, and finds neighbours by geometry; `isolated` levels have none.
  - It answers queries in world tile coordinates as **signed `int`**: `LevelAt`, `BlockingAt`, `InteractionAt`, `SurfaceAt`. Negative positions are normal, and void blocks everything.
  - A helper converts world pixels to tiles with `floor`, never a `uint32` cast.
  - Tests: lookups across a boundary; void; negative coordinates; overlap error; isolated levels have no neighbours.
- [x] 1.7 **Border solving**: `SolveBorders(level, world)` re-solves the edge cells on both sides of every neighbour boundary. The editor uses it on save. Test: two levels painted independently have no seam after solving.
- [x] 1.8 **Sample content** for development and tests:
  - `world/terrains.toml` with placeholder terrains (water, dirt, grass, black) and the surface and interaction lists;
  - two small adjoining exterior levels and one isolated interior in `levels/`, generated by a test helper so they always match the format. (Built as `meadow`, `lake` and `house_1`; regenerate with `go test ./shared/world -run TestSampleLevels -update`.)

### Step 2: `cmd/blobtemplate`, art templates and placeholder art

**2.1–2.3 built 2026-10-05** (`cmd/blobtemplate/`, pure Go, no raylib). Committed output: `assets/templates/blob47_template.png` (+ `_x4` reference copy) and placeholder sheets for every sample terrain in `assets/tiles/`, regenerated with:

```
go run ./cmd/blobtemplate -out assets/templates            # and -scale 4
go run ./cmd/blobtemplate -terrain water -placeholder 3a6fd0
go run ./cmd/blobtemplate -terrain dirt  -placeholder 9c7448
go run ./cmd/blobtemplate -terrain grass -placeholder 4a9c3b
go run ./cmd/blobtemplate -terrain black -placeholder 000000   # edges = false: fills its single tile
```

`-placeholder` writes to the terrain's sheet path from `world/terrains.toml`, so names can't disagree. The template's layout is the same for every terrain, so `-terrain` is optional there and only names the file. A test fails if the committed placeholder sheets stop matching the generator.

- [x] 2.1 `blobtemplate -terrain grass -out assets/templates/` writes the 8×6 template PNG: 47 cells in solver order plus 1 spare, each with a mini-diagram of its joined neighbours. The diagrams come from the solver's own table, so the template can't drift from it.
- [x] 2.2 `-placeholder <colour>` writes a usable flat-colour blob-47 sheet (edges drawn as a solid colour over transparency). The game and editor can then run with real-looking transitions before any art exists.
- [x] 2.3 Test: the generated sheet has the right size, and each cell's diagram matches its state's mask.
- [ ] 2.4 Convert or replace `assets/tileset.png`, which is a JPEG, so all tile art is PNG (D8). *Not done in code on purpose (2026-10-05):* a straight conversion triples the file (0.8 → 2.2 MB) while keeping the JPEG smearing, and nothing reads it. Replace it when real tile art arrives; until then the placeholder sheets in `assets/tiles/` are what levels use.

### Step 3: client, rendering and loading levels

**Built 2026-10-06, together with step 4** (3.6 and 4.2 had to ship together). Both binaries take `-root` (default `.`) and call `world.LoadMap`, which loads `world/` + `levels/`, recompiles every level's properties from the files (D32), and finds the spawn. Without those folders both fall back to the open 100×100 grid at `shared.SpawnPoint`; a world that is present but broken is fatal for both, since predicting and validating on different geometry flags honest players. Movement code takes `world.Collider` (`Blocks(tx, ty, flag)` plus `SpeedMultiplier(tx, ty)`), implemented by `World` and by `world.Grid` (the fallback and a test helper). Prediction collides a 12 px box anchored at the position: it contains the one point the server checks, so prediction is never looser than validation. Checked by hand on 2026-10-06 under a virtual display: walking from the spawn, swimming the meadow pond at half speed, stopping at the world's edge, and crossing from meadow into lake all ran with no server flags.

- [x] 3.1 **`client/render` package** (next to `client/anim`, importable by the editor):
  - a tile atlas that loads sheets and finds the source rect for a global tile index, including flips;
  - the shared camera (D33) in follow mode, tracking the local player (there's no camera today);
  - culled drawing of the visible cells per layer: ground (under, then tile) and decor;
  - `ysort` tiles emitted as sortable items, keyed by the bottom edge of their cell;
  - `overhead` drawn last;
  - background clear colour black instead of `rl.RayWhite`.
- [x] 3.2 **Y-sorting with players**: merge ysort tiles and players into one sort by foot position, replacing the players-only sort in `client/main.go`.
- [x] 3.3 **Load at startup**: if `world/` and `levels/` exist, the client loads the whole world (D18, from disk in this step); otherwise it falls back to today's blank grid.
- [x] 3.4 **Prediction against the world**: `PlayerController` takes a small `Collider` interface (`Blocks(tileX, tileY int, flag uint16) bool`) instead of `shared.CollisionLayer`, and walking tests the `ground` flag. Coordinates switch to signed and floored, fixing the `uint32` wraparound. Existing tests move to a fake collider.
- [x] 3.5 **Spawn point**: a `spawn` object in a level replaces the `shared.SpawnPoint` constant (100, 100). The server already creates new players there and validates their first move from it, and the client starts its prediction there, so both must read the same spawn from the world.
- [x] 3.6 **Speed from interaction**: prediction multiplies speed by the cell's interaction `speed_multiplier`. **Ship this in the same PR as 4.2**, or honest swimmers get flagged by the server.

### Step 4: server, real geometry for anti-cheat

- [x] 4.1 The server loads the world at startup (path flag, same defaults as the client) and compiles `blocking` and `interaction` from its own files (D32). It falls back to the blank grid if there are no levels.
- [x] 4.2 `MovementValidator` takes the same `Collider`. `CheckWallPhase` uses signed tile coordinates and tests the `ground` flag for walking and the `jump` flag for `MovementTypeJump`. The speed check allows the larger multiplier of the start and end cells, to avoid false positives at water edges.
- [x] 4.3 Tests: wall-phase is caught across a level boundary; jumping a fence (ground+roll flags) is allowed but walking through it is caught; swimming at swim speed is clean while walking speed in water is flagged.
- [ ] 4.4 Retire `shared.TileType` and `shared.CollisionLayer` once nothing uses them, or keep `CollisionLayer` only as a test helper that satisfies `Collider`. Update ARCHITECTURE.md's "Level loading" roadmap item and the `map/` cards. *Partly done 2026-10-06:* the client and server no longer use them (`world.Grid` is the test helper), and the docs and cards are updated. The types themselves stay for now, because `shared.Level` embeds them and the unwired `ServerLevelLoadedMsg` embeds `shared.Level`; removing them is a protocol change best made with step 6, which defines the real level-streaming messages.

### Step 5: `cmd/levelmaker` v1 (terrain painting + autotile)

The editing logic lives in a non-raylib package (e.g. `cmd/levelmaker/edit`) so it's testable in CI. The raylib side only draws and routes input.

- [ ] 5.1 **Skeleton**: a raylib window that renders through `client/render`, loads `world/` and `levels/`, and has a level picker. A session log records every error and crash (as animaker does).
- [ ] 5.2 **UI widgets**: try `raygui` (`github.com/gen2brain/raylib-go/raygui`) for buttons, lists and text fields, after checking it builds with our raylib-go version (v0.60.1). Fallback: a few hand-made immediate-mode widgets. Text fields select their contents when tabbed into (animaker convention).
- [ ] 5.3 **New level**: name, position, size and isolated, refused with a clear message if it overlaps another level.
- [ ] 5.4 **View**: the shared camera (D33) in free mode, with pan, zoom, jump-to and a wider zoom range than the game; grid toggle; read-only dimmed strips of neighbouring levels (D16).
- [ ] 5.5 **Painting**: terrain palette from the definitions; brush sizes 1/3/5, rectangle fill, flood fill and eraser. Each stroke re-solves locally, including across into neighbouring levels held in memory.
- [ ] 5.6 **Undo/redo** per stroke, storing each changed cell's before and after values (terrain, tile, under, plus any neighbour-level cells touched).
- [ ] 5.7 **Save**: compile properties, then write the level and any neighbour levels whose border cells changed. Show which files were written; this is open question 2's default until decided. Unsaved-changes marker and confirm-on-quit.
- [ ] 5.8 Add `levelmaker.exe` to `build_local.ps1` (built and launched with the others).
- [ ] 5.9 Tests for the edit package: brush and fill results, undo/redo, cross-level stroke, and save then reload being identical.

### Step 6: streaming levels from the server (D15)

- [ ] 6.1 The server serves `GET /levels/<name>/<hash>.toml` and `.grid.gz` over HTTP on a configurable port (open question 3; default: UDP port + 1).
- [ ] 6.2 A new UDP message, sent on join, carries the world manifest: every exterior level's `{name, hash, pos, size}`. Entering an interior sends that interior's entry. Define it in `shared/protocol.go` and add a row to PROTOCOL_REFERENCE.md.
- [ ] 6.3 The client keeps a disk cache keyed by hash, fetches what's missing, and treats unfetched cells as blocked. Loading from disk stays available as a dev mode.
- [ ] 6.4 Tests: an `httptest` server; manifest → fetch → second run is a cache hit; a changed level fetches only the changed files.

### Step 7: editor v2 (hand placement + properties)

- [ ] 7.1 **Tileset palette**: pick a single tile or drag a rectangle to make a stamp; layer selector (`decor`, `ysort`, `overhead`, extra decor layers); flip tools.
- [ ] 7.2 **Locked ground cells**: stamp a tile on ground and choose the underlay terrain. Painting terrain over the cell unlocks it (D3).
- [ ] 7.3 **Tile props in the editor**: assign blocking, surface and interaction defaults to tiles and save them to `world/terrains.toml`. Until this exists, they're edited by hand in the TOML.
- [ ] 7.4 **Property overlays and override brushes**, one per map (blocking flags, surface, interaction), showing the compiled result. The overlay drawing lives in `client/render`, not the editor, so a future spectator view (D34) reuses it.

### Step 8: editor v3 + warps in the game

- [ ] 8.1 **Objects**: place, move and delete; properties panel; `spawn` and `warp` kinds.
- [ ] 8.2 **Warps in the game**: the server detects a player entering a warp and moves them (server-authoritative, D17). The client loads the destination interior (via the manifest from 6.2, or from disk before step 6).
- [ ] 8.3 **Copy area to new level**: rectangle plus optional building-shaped selection. Outside the building, nothing is copied and the area is filled with `black` (D21). The new level is `isolated` and placed at a free spot; the tool offers to create the warp pair.

---

## Open questions

Recorded here; none of them block step 1.

1. **Terrain priority order:** the artist's call. The examples above (water < dirt < grass) are placeholders.
2. **Rewriting neighbours on save (D16):** this changes files the artist didn't open. Acceptable, or should the editor ask first?
3. **HTTP port and hosting:** same host and a fixed port next to UDP, or configurable?

**Deferred, not open:** load radius and maximum level size (D18). Revisit when the overworld is big enough that loading all of it costs too much memory or join time.

---

## What happened to the designer's draft

| Draft element | Outcome |
|---|---|
| `tile_id` (string) | Kept as a concept; stored as a `uint16` index into the tile table (D2) |
| `layer_id` | Kept; layers are named and carry z-ranges (D5) |
| `terrain_type` | Kept; a `uint8` id into `world/terrains.toml` |
| `type_override` | Kept and generalised: per-cell overrides for each of `blocking`, `surface` and `interaction` (D10, D29); the brush comes in editor v2 |
| `is_locked` | Kept, and extended with a hand-picked underlay so hand-arranged transition pieces work (D3). Flat overlays use the new `decor` layer instead (D28) |
| `elevation` | Deferred, with reserved flag bits (D11) |
| `flip_h` / `flip_v` | Kept, with a per-tile "may flip" flag (D4) |
| `custom_tags` | Replaced by the `surface` and `interaction` maps with their own definitions and overrides (D6, D29). Speed multipliers moved to interaction types |
| Blob-47 template generator | Rewritten in Go (`cmd/blobtemplate`). The draft listed only 36 of the 47 states |
| Per-pair sheets (`grass_to_dirt`) | Replaced by per-terrain sheets layered by priority (D9) |
| 8-bit neighbour mask | Kept, plus the corner reduction to 47 the draft lacked, plus the empty-cell fix (D20) |
| Fixed four layers incl. `water_1` | Water became a terrain (D19); the layer list is open-ended |
| JSON export of every cell | Replaced by TOML + binary grid (D15 and file formats above) |
