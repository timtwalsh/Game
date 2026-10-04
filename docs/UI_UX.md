# UI / UX — principles, system design, and task plan

> **Status: design proposal, written 2026-10-04. Nothing in this doc is built
> yet.** As of this date the client's entire UI is one line —
> `rl.DrawText("WASD to move, Space to jump", ...)` in `client/main.go` — drawn
> with raylib's default font into a fixed 800×600 window. There is no camera,
> no UI package, no fonts, no settings file. The chat, attack and interact
> messages this doc leans on are *defined but not wired up* in
> `shared/protocol.go` (see [PROTOCOL_REFERENCE.md](PROTOCOL_REFERENCE.md)).
> Every section below that depends on one of those says so.

The doc has three parts:

1. **[Goals, targets and industry lessons](#part-1--goals-targets-and-industry-lessons)**:
   what we want the UI to be, the numbers we'll measure it against, and what
   other games have already learned the hard way.
2. **[System design](#part-2--system-design)**: one UI system for an MMO RPG
   client. It covers unit frames, nameplates, speech bubbles, combat text,
   signs and prompts, dialogue trees, the HUD, menus and options.
3. **[Task list and priorities](#part-3--task-list-and-prioritisation)**:
   the work broken into phases, with dependencies, sizes and acceptance
   criteria.

---

## Part 1 — Goals, targets and industry lessons

### 1.1 Design pillars

These are the tie-breakers. When two UI decisions conflict, the pillar higher
in the list wins.

| # | Pillar | What it means in practice |
|---|---|---|
| 1 | **The world is the screen** | It's an ALTTP-style game: the playfield is the star, and the UI is a frame around it. The default HUD covers less than 15% of the screen. Anything not needed this second is hidden or faded until it's relevant. |
| 2 | **Readable at a glance, in motion** | During combat or movement, information must be read through peripheral vision in under ~250ms. To do that, use shape, position and colour *together*, plus numbers you don't have to parse. Paragraphs belong in menus and dialogue, never mid-fight. |
| 3 | **Honest about the network** | The server is authoritative ([ARCHITECTURE.md](ARCHITECTURE.md)). The UI never presents a predicted value as confirmed, and it never hides a rejection. Local feedback is instant. Confirmed state is shown as confirmed. When the two disagree, the UI reconciles visibly and calmly, without silently snapping. |
| 4 | **Every player can play** | Accessibility is a default, not a menu we add at the end. Text size, contrast, remapping, colour-independence and reduced motion are in from the first widget. |
| 5 | **The player owns their screen** | In an MMO, people spend hundreds of hours looking at the UI. Layout, scale, what's shown and what's hidden should be the player's choice. Defaults should still be good enough that most people never change them. |
| 6 | **Cheap enough to forget about** | The UI must never be why the game drops a frame. It has a hard per-frame budget (see 1.2) and the same zero-allocation steady-state discipline the render loop already follows (`// Reused every frame, so drawing allocates nothing once warmed up.` in `client/main.go`). |

### 1.2 Measurable targets

Targets are only useful if we can check them. Each one has a number and a way
to verify it.

#### Readability and accessibility

| Target | Value | Source / rationale | How we verify |
|---|---|---|---|
| Minimum body text at 1080p, default UI scale | **18px** cap-to-descender. We will never ship anything below **14px**. | Xbox XAG 101 / handheld guidance: never under 14px at 1080p, 18px recommended. XAG asks for 28px on TV-distance play, so the UI-scale slider must reach that. | Font metrics check in a unit test against the theme tokens |
| Text contrast | **≥ 4.5:1** for body text, **≥ 3:1** for large text (≥ 24px) and essential UI graphics such as bar fills and icons against their backgrounds | WCAG 2.x AA, also referenced by XAG 102 | Contrast check of every theme token pair, run in CI |
| Colour independence | **0** pieces of information conveyed by colour alone | Game Accessibility Guidelines (GAG), basic level | Review checklist. Every colour-coded item must also have a shape, icon or text difference. |
| UI scale range | **50%–200%**, in 10% steps | Pillar 5. Covers 720p laptops through 4K TVs. | Screenshot pass at 50/100/200% at 720p, 1080p and 4K. Nothing clipped or overlapping. |
| Remappable controls | **100%** of actions, including UI navigation | GAG basic. Remapping is one of the four most-complained-about accessibility gaps. | Each input action has a binding entry in the settings file |
| Gamepad and keyboard parity | Every screen is fully usable with keyboard only *and* with gamepad only. A mouse is optional everywhere. | FFXIV and Diablo IV-style console parity. Also helps motor accessibility. | Manual test script per screen |
| Flashing | **≤ 3 flashes per second**. No large saturated-red flashes. | Photosensitive-epilepsy guidance (Harding test, WCAG 2.3.1) | Review checklist, plus "reduce flashing" in options |
| Motion | Screen shake, floating text drift and bobbing can each be reduced or turned off | GAG intermediate | Options exist and are honoured |
| Subtitles and chat | Chat and bubble text sizes are configurable independently of the main UI scale. A background-opacity option is provided. | GAG basic: subtitle presentation | Options exist |

#### Responsiveness

The targets follow Nielsen's response-time limits: 0.1s feels instant, 1s
keeps flow, and 10s keeps attention.

| Target | Value |
|---|---|
| Input to visible UI response (button press, menu open, hover) | **Same frame**, and always **< 100ms** |
| Local action feedback (an attack swing, an ability press) | Predicted feedback on the same frame. Confirmed result once the server replies, normally within one network tick (`shared.NetworkTickRate`, 50ms) plus RTT. |
| Any wait > 1s (connecting, level load) | Shows a progress or activity indicator and a cancel/back option |
| Any wait > 10s | Shows an explanation of what is happening and what the player can do |
| Typewriter dialogue | A single press always completes the current page. A second press advances. No forced-wait text. |

#### Performance budget

The current target is 144 FPS (`rl.SetTargetFPS(144)`), which gives a 6.9ms
frame.

| Target | Value |
|---|---|
| UI update + draw, typical scene (HUD, 20 nameplates, 10 combat texts, chat) | **≤ 0.7ms** CPU (~10% of the frame) |
| UI worst case (100 nameplates, 40 combat texts, open inventory) | **≤ 1.5ms** CPU |
| Heap allocations per frame in steady state | **0** (`testing.AllocsPerRun` on update/layout paths) |
| Draw calls for the UI pass | All UI art in **one atlas texture**, plus one texture per font size. Target **< 10** texture switches per frame. |

#### Usability

| Target | Value |
|---|---|
| Inputs needed to reach any frequently used screen (inventory, map, options, chat) | **1** (a direct hotkey) |
| Depth to reach any option | **≤ 3** inputs from the pause/system menu |
| First-session comprehension | In playtests, ≥ 80% of new players can move, talk to an NPC, read a sign and open the menu with no help beyond on-screen prompts |
| Settings persistence | 100% of settings survive a restart. A corrupted or old settings file never crashes the client: it falls back to defaults and logs a warning. |

### 1.3 Industry best practices and lessons

This is a condensed set of lessons, grouped by topic. Each names the game or
source it comes from, so it can be followed up.

#### Frameworks worth borrowing

- **UI taxonomy (Fagerholt & Lorentzon, 2009, *Beyond the HUD*).** UI
  elements fall into four kinds:
  - **Diegetic:** exists in the world, and characters can see it, like a sign.
  - **Non-diegetic:** a classic overlay HUD.
  - **Spatial:** sits in the 3D/2D world but isn't part of the fiction, like
    nameplates and combat text.
  - **Meta:** fiction expressed on the screen plane, like a red vignette when
    hurt.

  We use this to decide *where* each element lives (see Part 2's layer model).
  ALTTP itself is mostly non-diegetic HUD plus diegetic signs.
- **Nielsen's 10 usability heuristics.** They apply directly to menus and
  options: system status visible, match the real world, user control (undo,
  back), consistency, error prevention, recognition over recall, and so on.
- **Hick's law and Fitts' law.** Fewer choices on screen means faster
  decisions. Bigger and closer targets are faster to hit. Keep dialogue choice
  lists to ≤ 4 options per page, and keep frequently used buttons large and
  near where the cursor or focus already is.
- **Game Accessibility Guidelines (basic / intermediate / advanced) and Xbox
  Accessibility Guidelines.** The basic tier is cheap if it's done from the
  start and expensive if it's retrofitted. Our targets in 1.2 adopt the GAG
  basic tier wholesale.

#### MMO-specific lessons

- **WoW: customisation is the long-term answer.** WoW shipped with a modest
  UI, and the addon ecosystem (unit frames, nameplates, combat text) grew
  because players' needs vary wildly by role. *Dragonflight* (2022) eventually
  brought a built-in **Edit Mode** for moving and resizing HUD elements, which
  was a 20-year admission that layout must be player-owned.
  - **Lesson:** build every HUD element on an anchor + offset + scale model
    from day one, so a layout editor is a feature we add later, not a rewrite.
- **FFXIV: HUD Layout editor and saved layouts.** FFXIV supports multiple
  saved HUD layouts (e.g. per role or per input device) and a fully
  controller-driven UI.
  - **Lesson:** layouts are data, and data can be swapped.
- **Nameplate clutter is the #1 visual problem in crowded MMOs.** WoW, GW2 and
  others all ended up with nameplate stacking and overlap avoidance, per-type
  toggles (friendly/enemy/NPC/pet), distance fading and "only show in combat"
  modes.
  - **Lesson:** design declutter rules in from the start (see 2.5.2).
- **Combat text needs aggregation and caps.** Diablo-likes and MMOs with
  damage-over-time effects quickly bury the screen in numbers. Common
  mitigations are merging same-target hits within a short window, capping
  concurrent numbers, emphasising crits by size rather than more numbers, and
  options to hide categories such as DoT ticks and pet damage.
- **Chat is social infrastructure, not a feature.** Players need tabs or
  channels, mute/ignore, report, a profanity filter toggle and timestamps.
  Speech bubbles make the world feel inhabited, but each speaker must be
  capped and they must be easy to turn off. Report and mute need to be in
  the UI from the first version of chat. `ClientReportPlayerMsg` already
  exists in the protocol for exactly this.
- **Never hide latency or disconnects.** Rubber-banding with no explanation
  reads as a bug. A small latency indicator, a clear "Reconnecting…" banner and
  a specific reason on disconnect cost little and build a lot of trust. We
  already have reasons to show: `ServerMovementRejectedMsg` and
  `ServerBannedMsg`.

#### RPG and ALTTP-lineage lessons

- **ALTTP's HUD is three things:** life (hearts), magic, and the equipped item
  with counters (rupees, bombs, arrows). Everything else is behind a single
  menu button.
  - **Lesson:** our default HUD should feel this sparse even though an MMO
    needs more. Context-sensitive elements such as a target frame or cast bar
    should appear only when relevant.
- **ALTTP's dialogue box is a bottom-anchored panel showing 3 lines,
  typewriter text and a prompt to advance.** It's instantly familiar for this
  genre, so use it as the default dialogue presentation.
- **Hearts vs bars.** Discrete units such as hearts read faster than a
  continuous bar at low counts, but they don't scale to MMO HP ranges or show
  partial damage well.
  - **Recommendation:** use bars with numeric overlay options for unit
    frames. Keep a "hearts" style as a skin option for the ALTTP feel if HP
    values are kept small.
- **Stardew Valley / Undertale-style dialogue.** Short pages, a character
  portrait, a per-character text sound or speed, and choices kept short.
  Players skim, so the important information goes in the first line.
- **Celeste / Hades: assist modes are in the options, not hidden away.**
  Lesson for us: an "Accessibility" tab is a first-class top-level options
  tab, not a sub-page of "Gameplay".
- **The Last of Us Part II (60+ accessibility options) shows the ceiling.**
  We aren't aiming there for v1. It's evidence that options like high-contrast
  mode and text-to-speech for menus are feasible and valued.

#### Engineering lessons

- **Separate UI state from UI drawing.** Games that let widgets read game
  state directly end up with UI bugs that are really race conditions. This is
  especially true here, since `receiveLoop` runs on another goroutine behind
  `client.mutex`. The UI should read a per-frame snapshot, as the render loop
  already does with `players`.
- **Localisation is cheapest on day one.**
  - Don't concatenate sentences.
  - Use string IDs.
  - Leave room for 30–40% text expansion (German, Finnish).
  - Design fonts with Unicode coverage for chat. Players *will* type
    non-ASCII on day one even if we never translate.
- **Pixel art and UI resolution are separate decisions.** Pixel-art worlds
  want integer scaling of a low internal resolution. Text wants to be crisp at
  the display's native resolution. Mixing them (scaling the whole UI with the
  world) gives blurry text or awkward non-integer jumps. Render them in
  separate passes (see 2.2).

---

## Part 2 — System design

### 2.1 Overview

```
            ┌────────────────── client goroutine: receiveLoop ──────────────────┐
 server ──► │ decode ServerXxxMsg ──► game state (under client.mutex)            │
            │                    └──► ui.Events (lock-free queue, see 2.3)       │
            └────────────────────────────────────────────────────────────────────┘

            ┌────────────────── render goroutine: main loop ────────────────────┐
 input  ──► │ 1. Input   : raw keys/pads ─► Actions ─► UI gets first refusal     │
            │ 2. Game    : prediction, interpolation (unconsumed actions only)   │
            │ 3. Snapshot: copy what UI needs out of game state (under lock)     │
            │ 4. UI      : drain Events, update view-models, layout, animate     │
            │ 5. Draw    : world pass (render texture, integer-scaled)           │
            │              world-space UI pass (nameplates, bubbles, numbers)   │
            │              screen-space UI pass (HUD, windows, dialogue, menus) │
            │              overlay pass (tooltips, drag-ghost, toasts, debug)   │
            │ 6. Intents : UI emits Commands ─► game/network (e.g. ChatSend)    │
            └────────────────────────────────────────────────────────────────────┘
```

The core rules:

- **Game state never imports UI.** The game publishes events and exposes a
  snapshot. The UI never mutates game state directly. It emits **Commands**
  (intents) that the game layer turns into network messages or local actions.
- **The UI is server-honest.** Anything the server owns is displayed from
  server-confirmed values: HP, damage, dialogue state, inventory. Predicted
  values are visually distinct. For example, predicted cooldowns show
  immediately, and a reject rolls them back with a short shake and an
  explanatory toast.
- **UI logic is testable without a window.** Layout, text wrapping, dialogue
  evaluation, combat-text aggregation and nameplate declutter are pure Go that
  takes a `TextMeasurer` interface, so `go test ./...` can cover them in CI
  without raylib opening a window. This is the same split
  `client/prediction.go` already has from drawing.

### 2.2 Rendering model and resolution

- **World pass:**
  - Render the world to a `rl.RenderTexture2D` at a fixed **internal
    resolution**. Proposal: 480×270, which scales by an exact ×4 to 1080p and
    ×8 to 4K.
  - Blit it to the window with integer scaling, letterboxing any remainder.
  - This replaces today's fixed 800×600 window drawn 1:1.
  - It needs a camera, which doesn't exist yet. That is task UI-0.2.
- **World-space UI pass (spatial):**
  - Nameplates, speech bubbles, combat text and interaction prompts.
  - Positions come from world coordinates projected through the camera, but
    they are drawn at **native resolution** so text stays crisp.
  - Each anchors to an entity's head or feet point, as the shadow already
    does with `footY`.
- **Screen-space UI pass (non-diegetic):**
  - HUD, unit frames, windows, dialogue and menus.
  - Laid out in a **reference resolution of 1920×1080 units**, then multiplied
    by `uiScale = (displayHeight / 1080) × playerUIScale`.
  - Pixel-art panels snap to integer multiples where possible. Text uses
    pre-rasterised font sizes (see 2.4).
- **Overlay pass:**
  - Tooltips, drag-and-drop ghost, toasts, the "Reconnecting…" banner and the
    debug overlay.
  - Always on top. Never takes input focus except for modal error dialogs.
- **Meta effects:** low-HP vignette, damage flash. These are drawn between the
  world and screen-space passes, and they obey the reduce-flashing and
  reduce-motion options.

### 2.3 Architecture — the `client/ui` package

The UI is a new package in the `game` module, alongside `client/anim`:

```
client/ui/
  ui.go          // UI root: owns layers, focus, event queue, Update/Draw
  events.go      // Event types (game -> UI) and Command types (UI -> game)
  theme.go       // design tokens: colours, sizes, spacing, font roles
  text.go        // TextMeasurer interface, wrap/ellipsis, rich-text spans
  layout.go      // anchors, rects, stacks, grids, 9-slice
  input.go       // actions, bindings, focus/navigation, input routing
  widgets/       // button, label, bar, icon, list, scroll, slider, toggle, textbox
  hud/           // unitframe, actionbar, castbar, buffs, minimap, xpbar
  world/         // nameplate, bubble, combattext, prompt
  dialogue/      // tree model, evaluator, presenter (box + choices)
  screens/       // pause, options, inventory, map, quests, social, login, loading
  settings/      // load/save/migrate user settings (TOML)
  loc/           // string table lookup, plural rules
```

#### Retained model, immediate-style drawing

- **View-models are retained.** A `UnitFrameVM`, `NameplateVM` or
  `DialogueVM` lives across frames. It is updated from events and the
  snapshot, and it holds animation state such as a bar's "ghost" damage
  trail, a bubble's remaining lifetime, or typewriter progress.
- **Drawing is immediate-style.** Each frame, each visible widget computes its
  rect from anchors and draws with raylib primitives, 9-slice panels and
  atlas sprites.
- **There is no deep retained widget tree with dirty-flag invalidation.** Our
  UI is small enough that full re-layout each frame is far cheaper than its
  complexity. It must stay within the 0.7ms budget and allocate nothing,
  using preallocated slices reused per frame as `main.go` already does.

**Why not raygui?** raylib-go ships `raygui`, an immediate-mode GUI. It's
good for dev and debug panels, and we should use it for those (task UI-0.12).
For the player-facing UI we need:

- 9-slice pixel-art skinning
- per-element anchors and layout persistence
- gamepad focus navigation
- localisation-aware text
- world-space elements

raygui's style system and controls don't cover these without forking, so a
thin custom widget layer on raylib primitives is less total work.

#### Events in (game → UI)

These are typed structs pushed to a bounded queue, drained once per frame:

| Event | Produced from | Status of source |
|---|---|---|
| `EntityEntered/Left{ID}` | `ServerPlayerStatesMsg` diffs (as `applyPlayerStates` already does) | live |
| `ChatReceived{From, Channel, Text}` | `ServerChatMsg` | **defined, not wired** |
| `CombatResult{Attacker, Target, Amount, Kind, Crit}` | `ServerAttackResultMsg` | **defined, not wired**. Needs `Kind` and `Crit` fields added. |
| `MoveRejected{Reason}` | `ServerMovementRejectedMsg` | **defined, not wired** |
| `Banned{Reason}` | `ServerBannedMsg` | **defined, not wired** |
| `DialogueNode{NPC, NodeID, Text, Choices}` | new message: proposed `ServerDialogueMsg` | **not defined** |
| `ConnectionState{State, RTT}` | client net layer | not built |

#### Commands out (UI → game)

The UI never sends packets itself. It returns commands, and the game layer
maps them:

| Command | Becomes |
|---|---|
| `SendChat{Channel, Text}` | `ClientChatMsg` |
| `ReportPlayer{ID, Reason}` | `ClientReportPlayerMsg` |
| `Interact{TargetID}` | `ClientInteractMsg` |
| `ChooseDialogue{NodeID, ChoiceIdx}` | new message: proposed `ClientDialogueChoiceMsg` |
| `SetTarget{ID}` | local only, plus an optional server notify later |
| `UseAbility{Slot}` | `ClientAttackMsg` (or a future ability message) |
| `ApplySettings{...}` | local: window mode, audio, bindings |

#### Input routing and focus

- **Raw input becomes Actions.** `MoveUp`, `Confirm`, `Cancel`, `OpenMenu`,
  `OpenChat`, `Ability1`… This replaces the hard-coded `rl.IsKeyDown(rl.KeyW)`
  calls. Bindings live in settings and are remappable.
- **The UI gets first refusal on actions each frame.**
  - The focused **text box** captures all printable keys and `Confirm`/`Cancel`.
  - While it has focus, WASD must type letters, not move the character.
  - A **modal** (dialogue, error) consumes everything except its own
    navigation actions.
  - Otherwise unconsumed actions fall through to gameplay.
- **Focus navigation for gamepad and keyboard:**
  - Each screen declares a focus graph, or gets a spatial nearest-neighbour
    fallback.
  - The focus ring is always visible when the last input was a pad or
    keyboard, and hidden on mouse use.
  - `Cancel` always means "back" and never quits the game without a confirm.
- **Input prompts:** prompt glyphs ("[E] Read", "Ⓐ Read") follow the last
  device used and the player's current binding, so they're never hard-coded
  text.

### 2.4 Visual language: theme tokens, fonts and art

- **Design tokens (`theme.go`).** Never put raw colours or pixel sizes in
  widget code. Every value is a named token:
  - colours: `Text.Primary`, `Text.Muted`, `Bar.Health`, `Bar.HealthGhost`,
    `Bar.Resource`, `Faction.Hostile`, `Faction.Friendly`, `Faction.Neutral`,
    `Damage.Physical`, `Damage.Fire`, …
  - spacing: `Space.XS`–`Space.XL`
  - font roles: `Font.Body`, `Font.Title`, `Font.Number`, `Font.Small`
  - **Alternative themes** (high contrast, the three colour-vision-deficiency
    palettes) are just alternative token tables.
  - The CI contrast test (1.2) iterates over every theme.
- **Fonts.** Load with `rl.LoadFontEx` at the sizes actually used after UI
  scale is applied. Re-rasterise when the scale changes. Bilinear-scaling a
  small atlas looks blurry.
  - **Pixel font** for the HUD and titles, for the ALTTP feel. It must have a
    clean digit set for numbers.
  - **Highly legible font** for body text, chat and dialogue. This is
    player-switchable, and there is an option for a dyslexia-friendly font.
  - **Glyph coverage:** at least Latin-1 + Latin Extended-A at launch,
    because chat. Missing glyphs render as a visible tofu box, never as
    silently dropped characters.
- **Art.** All UI art goes in one atlas (`assets/ui/atlas.png` plus a TOML
  slice manifest). 9-slice panels, bar fills, icons and prompt glyphs. Icons
  have a silhouette that reads without colour.
- **Motion.** Use `easings` (ships with raylib-go) for every tween. Durations
  are tokens (`Motion.Fast` = 120ms, `Motion.Normal` = 200ms). The
  reduce-motion option sets all of them to 0 and disables drift and bob.

### 2.5 Components

Each component below lists what it shows, its rules, and the options that
affect it.

#### 2.5.1 Unit frames (player, target, target-of-target, party)

- **Shows:**
  - name, level, HP bar, resource bar (mana/stamina), cast bar for target
  - buff/debuff row
  - status icons: leader, in-combat, disconnected, dead
- **Health bar behaviour:**
  - The bar drops instantly to the new value.
  - A lighter "ghost" segment then drains over ~400ms behind it, so the
    *amount* of damage is readable.
  - Heals fill instantly, with a brief highlight on the gained segment.
  - Optional numeric text formats: `current / max`, `%`, `current`, or
    hidden.
- **Low health:**
  - The bar pulses slowly, at < 1Hz, well within the flashing limit.
  - An optional edge vignette appears (meta UI).
  - There is always a non-colour cue as well: an icon and the pulse.
- **Party frames:**
  - A vertical list up to the party size.
  - Ordering: you first, then by role or join order (player choice).
  - Out-of-range members are faded.
  - Click or select to target.
- **Placement:**
  - The player frame defaults top-left (ALTTP's hearts live there).
  - The target frame sits next to it, or above the action bar (player
    choice).
  - Everything is anchor-based (2.6).
- **Dependency:** HP and resources don't exist in `PlayerState` yet. Frames can
  ship early showing name and connection status only, then gain bars when
  combat lands.

#### 2.5.2 Nameplates

- **Shows:**
  - name; guild/title on a second line, optional
  - health bar: hidden at full HP when out of combat (option)
  - cast bar for enemies
  - quest/interaction marker for NPCs
- **Anchor:** a fixed offset above the sprite's head point, in world space. It
  follows the *rendered* position, which for remote players is the
  interpolated one, so it doesn't jitter against the sprite.
- **Faction is encoded twice:**
  - colour token (`Faction.*`)
  - **plate shape/border**: hostile has an angular border, friendly a rounded
    one, neutral is borderless

  This satisfies colour independence.
- **Declutter rules, applied in order:**
  1. **Category toggles** (player options): friendly players / enemies / NPCs /
     own name. Default: own name off.
  2. **Range:** hidden beyond N tiles (default 20). Alpha fades over the last
     25% of the range.
  3. **Priority:** current target > party members > hostile-in-combat > quest
     NPCs > others. When over the cap (default 40 visible), lowest priority
     drops first.
  4. **Overlap avoidance:** sort by priority, then nudge each plate vertically
     out of collision with already-placed plates, up to 2 steps. Past that,
     hide the lower-priority plate.
     - Nudges are smoothed over ~100ms so plates don't jitter frame-to-frame.
     - This is a pure function over rects, so it's unit-testable.
  5. **Occlusion:** plates fade when the entity is behind a roof or tree layer
     (Z 51–70 from the rendering model), except the current target.
- **Selection:** the targeted entity's plate gets a scale-up (×1.15) and a
  border highlight. Clicking a plate targets that entity.

#### 2.5.3 Speech bubbles and chat

- **Chat window:**
  - Bottom-left by default.
  - Channel tabs (Say, Party, Guild, Whisper, System).
  - Scrollback (500 lines per tab, in a preallocated ring buffer).
  - Timestamps optional.
  - Clickable names open a context menu: whisper, invite, mute, report.
  - Fades to low alpha after 10s idle and returns on new message or hover.
- **Chat input:**
  - `Enter` opens it, `Enter` sends, `Esc` cancels.
  - Up/down arrows step through sent history.
  - Max length enforced client-side to match the server limit.
  - **While open, it owns the keyboard** (see input routing).
- **Speech bubbles** (Say channel only):
  - Anchor above the speaker's nameplate.
  - Max width 240 reference units; wrap; max 4 lines, then ellipsis (the
    full text is in the chat log).
  - **Lifetime** = clamp(2s + chars / 15, 3s, 10s). 15 chars/s is roughly a
    comfortable reading speed of ~180 wpm.
  - Max **2 bubbles per speaker**, stacked. A new one pushes the old one up
    and shortens its remaining life.
  - Bubbles from muted players are never shown.
  - The overlap avoidance from 2.5.2 is reused, with a lower priority than
    nameplates.
- **Safety:**
  - profanity filter (on by default, player-toggleable)
  - mute and report from any name or bubble
  - rate-limit feedback ("You are sending messages too quickly")
  - **Bubbles never render rich-text markup from other players.** Only system
    messages may use colour or icon spans.

#### 2.5.4 Damage and healing numbers (floating combat text)

- **Source:** server-confirmed `CombatResult` events only. A predicted swing
  can show a small "whiff" or impact effect immediately, but numbers wait for
  the server.
- **Style by kind:**

  | Kind | Visual | Non-colour cue |
  |---|---|---|
  | Outgoing damage | white/yellow | — |
  | Critical | larger (×1.5), brief scale-punch | `!` suffix and size |
  | Incoming damage | red, on own character | `-` prefix |
  | Healing | green | `+` prefix |
  | Miss / dodge / immune | grey text word | the word itself |
  | Damage type (fire, poison…) | type colour token | small type icon before the number |

- **Motion:**
  - Spawn at the target's head point, with a random horizontal lane offset
    of ±12 units.
  - Rise 24 units over 900ms with an ease-out curve.
  - Fade during the last 30%.
  - Reduce-motion: no rise; numbers fade in place.
- **Aggregation:** hits on the same target of the same kind within **150ms**
  merge into one number that ticks up. Damage-over-time ticks can optionally
  merge into one number per second.
- **Caps:** at most 40 alive at once, from a fixed-size pool, so nothing is
  allocated per hit. When full, the oldest non-crit text is recycled.
- **Options:**
  - per category on/off: outgoing, incoming, healing, DoT, other players'
  - size scale
  - "only my numbers"

#### 2.5.5 Popups, signs and interaction prompts

- **Interaction prompt:**
  - Appears when an interactable is within range and is the best candidate:
    the nearest one in the facing direction (ALTTP rules).
  - Shows `[binding glyph] Verb`, e.g. `[E] Read`, `[E] Talk`, `[E] Open`.
  - Only one prompt at a time.
  - Pressing it sends `Interact{TargetID}` (`ClientInteractMsg`, **defined,
    not wired**).
- **Signs:**
  - Reading a sign opens the **dialogue box** in its "sign" style: no
    portrait, no name.
  - Signs are just single-node dialogues, which avoids a second text system.
- **World popups:**
  - Small icons above entities: `!` quest available, `?` quest turn-in,
    `…` talking.
  - Spatial UI that respects nameplate declutter.
- **Toasts / notifications** (overlay):
  - Top-centre stack, max 3 visible, 4s each.
  - Kinds: info, success, warning, error, loot, achievement.
  - Same-kind duplicates merge ("Picked up Arrow ×5").
  - Errors from server rejections land here. For example, a
    `MoveRejected{Reason}` shows a friendly mapped message rather than the raw
    reason code.

#### 2.5.6 Dialogue and dialogue trees

- **Presentation** (ALTTP-style default):
  - A bottom-anchored box, 3 lines visible.
  - Speaker name tab and an optional portrait.
  - Typewriter reveal at a configurable speed (an instant option is
    included).
  - "▼" continue indicator.
  - Choices appear as a vertical list of ≤ 4 short options, navigable by
    keyboard, pad or mouse, with numeric hotkeys 1–4.
  - A **history/backlog** key shows the last 50 lines of the current
    conversation.
- **Server authority.** Dialogue drives quests, shops and rewards, so the
  **server evaluates conditions and applies effects**. The client only
  presents.

  ```
  ClientInteractMsg{Target}  ──►  server picks entry node, evaluates conditions
                             ◄──  ServerDialogueMsg{NPC, Node, Text, Choices[visible only]}
  ClientDialogueChoiceMsg{Node, Choice} ──► server validates choice is legal from Node,
                                            applies effects, sends next node or End
  ```

  A client can't skip to a reward node or pick a hidden choice, which matches
  the anti-cheat stance in [ARCHITECTURE.md](ARCHITECTURE.md). Both messages
  are **new** and would need adding to `shared/protocol.go` and
  [PROTOCOL_REFERENCE.md](PROTOCOL_REFERENCE.md).
- **Authoring format.** TOML, since `BurntSushi/toml` is already a dependency.
  Text is referenced by string ID for localisation. Here's a sketch:

  ```toml
  [dialogue.old_man_cave]
  start = "greet"

  [dialogue.old_man_cave.nodes.greet]
  speaker = "npc.old_man"
  text    = "dlg.old_man.greet"          # string-table key
  choices = [
    { text = "dlg.old_man.ask_sword", next = "sword", if = "!flag.has_sword" },
    { text = "dlg.common.bye",        next = "@end" },
  ]

  [dialogue.old_man_cave.nodes.sword]
  speaker = "npc.old_man"
  text    = "dlg.old_man.take_this"
  effects = ["give:item.wooden_sword", "set:flag.has_sword"]
  next    = "@end"
  ```

- **Evaluator.** A small, pure-Go condition language (`flag.x`, `!flag.x`,
  `item.y >= 3`, `quest.z == "done"`, `&&`, `||`). It lives in `shared/` so
  the server can run it and the tooling can lint it.
  - It is **not** a general scripting language. Anything more complex is a
    named server-side hook.
- **Validation tool.** A `go test` (or small CLI) that loads every dialogue
  file and fails on any of these:
  - unreachable nodes
  - dangling `next` references
  - unknown string IDs
  - choice lists over 4
  - conditions that don't parse
- **Movement during dialogue.** Opening dialogue stops local movement input.
  Walking-away cancellation is a server-side range check that sends `End`.

#### 2.5.7 HUD

The default contents are minimal, per pillar 1.

| Element | Default anchor | Visibility rule |
|---|---|---|
| Player unit frame | top-left | always |
| Target frame | top-left, right of player frame | when a target exists |
| Party frames | left edge, below player frame | when in a party |
| Action bar (abilities/items, 1–0 keys) | bottom-centre | always; fades out of combat (option) |
| Cast bar | above action bar | while casting |
| Buffs/debuffs (self) | top-right | when any exist |
| Minimap + coordinates/zone name | top-right | always (toggle) |
| Chat | bottom-left | always (fades when idle) |
| XP bar | very bottom, thin | always (toggle) |
| Quest tracker | right edge | when tracking quests |
| Latency / FPS | bottom-right, tiny | latency always visible when > 150ms or on loss; otherwise option |
| Interaction prompt | above player sprite (world-space) | context |

Also: a **"hide UI" toggle** (default `Alt+Z`, as in WoW) for screenshots, and
a **combat-only fade** mode for HUD elements.

#### 2.5.8 Menus and screens

- **System menu (`Esc`).** Resume, Options, Key Bindings, Help/Controls,
  Report a Problem, Log Out, Quit.
  - **It does not pause the world.** This is an MMO, so the server keeps
    simulating.
  - The menu says so subtly ("You are still in the world") and dims the
    background.
- **Game menus** each have a direct hotkey, and they share one tabbed frame so
  `Q`/`E` or LB/RB switch tabs: Inventory, Character/Equipment, Map, Quest
  Log, Social (friends/guild/party), Abilities.
- **Window rules:**
  - One full-screen menu at a time. Small windows such as a trade window can
    sit beside the HUD.
  - `Esc` closes the topmost window first, and opens the system menu only when
    nothing is open.
  - Windows remember their last tab and scroll position for the session.
- **Pre-game screens:** title/login, server select, character select/create,
  loading.
  - Loading shows the zone name and a tip and progress, and allows cancel or
    back after 10s.
  - Login errors are specific: "server unreachable", "version mismatch",
    "banned: reason", "wrong credentials".
- **Error and disconnect states (overlay):**
  - "Connection lost — reconnecting (attempt 2)…" banner, with input disabled
    for world actions but chat history readable.
  - After N failures, a modal offers Retry or Quit to title.
  - `ServerBannedMsg` shows a modal with the reason and appeal info, never a
    silent exit.

#### 2.5.9 Options

The tabs are listed in order of likely use. Each setting:

- applies **live** with a preview
- has a "Reset to default" per tab
- shows a one-line description
- is searchable (filter box at top) once the count grows past ~40

| Tab | Settings |
|---|---|
| **Accessibility** (first-class tab) | UI scale; text size (body, chat, dialogue independently); font choice (pixel / legible / dyslexia-friendly); high contrast theme; colour-vision mode (protan / deutan / tritan token tables); reduce motion; reduce flashing; screen shake 0–100%; typewriter speed / instant; hold-vs-toggle for held actions; dialogue auto-advance off/on + speed |
| **Interface** | HUD element toggles; nameplate categories, range, cap; combat text categories & scale; chat fade, timestamps, profanity filter, bubble on/off; numeric bar text format; HUD layout (later: edit mode, saved layouts) |
| **Controls** | Rebind every action (keyboard + pad separately); conflict detection with swap prompt; mouse/stick sensitivity; prompt glyph style (auto / keyboard / Xbox / PlayStation) |
| **Graphics** | Window mode, resolution, vsync, FPS cap (currently hard-coded 144), internal pixel scale (integer / fit) |
| **Audio** | Master, music, effects, UI, voice/text-blips — separate sliders (GAG basic) |
| **Gameplay** | Auto-target, click-to-move (future), confirm-before-destroy, social (whisper/invite from friends only) |

**Persistence:**

- TOML at `os.UserConfigDir()/<game>/settings.toml`, with a `version` field
  and per-version migrations.
- Unknown keys are ignored and missing keys use defaults.
- A file that won't parse is backed up as `.bak` and replaced with defaults
  plus a toast saying so.
- Key bindings live in the same file.
- HUD layouts are saved as separate named files once the layout editor
  exists.

### 2.6 Layout system

- Every screen-space element has:
  - an **anchor**, one of 9 screen points (TL, T, TR, L, C, R, BL, B, BR)
  - a **pivot**, the same 9 points on the element itself
  - an **offset** in reference units
  - a **scale** multiplier
  - a **visibility rule**

  These live in a `Layout` struct that is pure data, which is what makes a
  later WoW Edit Mode / FFXIV HUD Layout-style editor a UI over a TOML file
  rather than a refactor.
- **Safe margin** default: 2.5% of each screen edge. Adjustable 0–10% for TVs
  with overscan.
- Containers: `HStack`, `VStack`, `Grid`, `Scroll`, `NineSlicePanel`. No
  absolute pixel positions inside containers.
- **Text expansion:** every label has a min/max width policy (wrap, shrink to
  a min font size, or ellipsis plus tooltip). Pseudo-localisation (2.7)
  catches labels with no policy.

### 2.7 Localisation readiness

These are cheap now and expensive later:

- All player-facing strings go through `loc.T("key", args...)`. A dev build
  logs missing keys and renders them as `⟦key⟧`.
- Use whole-sentence templates with named args (`"{player} hit {target} for
  {amount}"`), never concatenation. Plural forms come via the table.
- Numbers and times are formatted through `loc` (thousand separators,
  durations).
- A **pseudo-locale** (`xx`) wraps every string in brackets, adds about 35%
  length and accents, e.g. `[Ïñvéñţörý···]`. It catches hard-coded strings,
  truncation and missing glyphs in one screenshot pass.

### 2.8 Testing and tooling

| Layer | How it's tested |
|---|---|
| Layout, wrap, ellipsis, anchors | table-driven unit tests with a fake `TextMeasurer` (fixed glyph width) |
| Nameplate declutter, bubble stacking, combat-text aggregation and pooling | unit tests with an `AllocsPerRun == 0` check |
| Dialogue loader, evaluator, validator | unit tests over sample TOML; the validator runs over all real dialogue files in CI |
| Theme contrast | test that computes the WCAG ratio for each declared token pair in every theme |
| Settings load and migration | tests with old-version, corrupt and partial files |
| Visual regressions | later: headless screenshot capture of each screen at 3 scales and the pseudo-locale, for manual diffing at first |
| UI perf | `go test -bench` on Update+Layout for the worst-case scene. An in-game debug overlay (`F3`) shows UI ms and draw calls per frame. |

---

## Part 3 — Task list and prioritisation

### 3.1 How to read this

- **Priority:**
  - **P0** Must, a foundation everything else needs
  - **P1** Must, for the next playable milestone
  - **P2** Should, needed once its gameplay dependency lands
  - **P3** Could, polish and long-tail
- **Size:** **S** ≤ 1 day, **M** 2–4 days, **L** 1–2 weeks (solo-dev
  estimates, rough).
- **Blocked by** names other UI tasks or non-UI work (`[net]` = needs a
  protocol message wired end-to-end on client and server; `[game]` = needs a
  gameplay system that doesn't exist yet).
- **Done when** is the acceptance criterion. Each task should also land with
  its tests and an update to `map/` per the repo's conventions.

### 3.2 Phase 0 — Foundations (P0)

These have no gameplay dependency and can start now. Order within the phase
is the suggested build order.

| ID | Task | Size | Blocked by | Done when |
|---|---|---|---|---|
| UI-0.1 | Create `client/ui` package skeleton: `UI` root with `Update(snapshot, actions)` and `Draw()`, wired into `client/main.go`'s loop as steps 4–6 of the frame diagram | S | — | Main loop calls `ui.Update`/`ui.Draw`. The old `DrawText` line moves into it. `go test ./...` green. |
| UI-0.2 | Camera + render-texture world pass at a fixed internal resolution with integer scaling; resizable window | M | — | Window resizes without stretching pixels. World draws via `RenderTexture2D`. A world→screen projection function exists and is unit-tested. |
| UI-0.3 | UI scale model: reference 1920×1080 units → screen, × player scale; safe margins | S | 0.1 | Elements land at the same relative spot at 720p, 1080p and 1440p (unit-tested). |
| UI-0.4 | Theme tokens (`theme.go`) + WCAG contrast test over all token pairs | S | — | No raw colour literals in `client/ui` outside `theme.go` (grep check in test). Contrast test passes. |
| UI-0.5 | Font loading: pixel + legible fonts, `LoadFontEx` per used size, re-rasterise on scale change, Latin-1 + Latin Ext-A glyphs | M | 0.3 | Text renders crisp at 50/100/200% scale. Missing glyph shows tofu. Fonts unloaded on exit. |
| UI-0.6 | `TextMeasurer` interface + wrapping, ellipsis, simple rich-text spans (colour, icon) | M | 0.5 | Table tests for wrap/ellipsis with a fake measurer, including multi-byte UTF-8. |
| UI-0.7 | Layout primitives: anchors/pivots/offset, H/VStack, Grid, 9-slice panel; UI atlas + TOML slice manifest | M | 0.3 | A test panel composes from stacks and renders a 9-slice frame from the atlas. Layout tests pass. |
| UI-0.8 | Input actions + bindings: replace hard-coded `rl.IsKeyDown` with actions; keyboard + gamepad; UI-first routing; text-box capture | M | 0.1 | Movement still works. With a text box focused, WASD types and doesn't move. Bindings come from a table. |
| UI-0.9 | Focus navigation (explicit graph + spatial fallback), focus ring shown on pad/keyboard use | M | 0.7, 0.8 | A test screen is fully operable by keyboard only and by pad only. |
| UI-0.10 | Event queue (game→UI) and Command return (UI→game), thread-safe w.r.t. `receiveLoop` | S | 0.1 | `receiveLoop` can push events without taking `client.mutex`. Race detector clean (`go test -race`). |
| UI-0.11 | Settings store: TOML load/save/migrate in user config dir; corrupt-file fallback | M | — | Tests for missing, corrupt, old-version and unknown-key files. Settings round-trip. |
| UI-0.12 | Debug overlay (`F3`): FPS, UI ms, draw/texture switches, RTT; raygui allowed here | S | 0.1 | The overlay shows live numbers. The UI budget can be read at a glance. |
| UI-0.13 | `loc` string table + pseudo-locale | S | 0.6 | All Phase 0/1 strings go through `loc.T`. The pseudo-locale renders. |
| UI-0.14 | Core widgets: label, button, toggle, slider, dropdown, text box, scroll list, bar, icon, tooltip | L | 0.6–0.9 | Each widget works with mouse, keyboard and pad. Zero allocations in steady state (AllocsPerRun test). |

### 3.3 Phase 1 — Playable social MVP (P1)

This phase targets the current game: multiple players walking around
together.

| ID | Task | Size | Blocked by | Done when |
|---|---|---|---|---|
| UI-1.1 | Controls/help overlay replacing the hard-coded instruction text; shows *current* bindings and device glyphs | S | 0.8, 0.14 | Rebinding a key changes the help text. Pad users see pad glyphs. |
| UI-1.2 | System menu (`Esc`) with non-pausing behaviour, Resume / Options / Quit, `Esc`-closes-topmost rule | M | 0.14 | Fully operable by keyboard and by pad. World keeps updating behind it. |
| UI-1.3 | Options screen v1: Accessibility (UI scale, text sizes, reduce motion/flashing), Graphics (window, vsync, FPS cap), Audio sliders, Controls rebinding with conflict detection | L | 0.11, 0.14, 1.2 | All settings apply live and persist across restart. Conflicting binding prompts a swap. |
| UI-1.4 | Player names: add a name to `PlayerState` / join flow, then nameplates v1 (name, faction shape, range fade, overlap avoidance, category toggles) | M | [net] name field; 0.2, 0.7 | 20 players in a clump stay readable. Declutter unit tests pass. Plates don't jitter against interpolated sprites. |
| UI-1.5 | Chat: wire `ClientChatMsg`/`ServerChatMsg` end-to-end (server relay + rate limit), chat window with Say/System tabs, input box, history | L | [net] chat; 0.8, 0.14 | Two clients can chat. Rate-limit feedback is shown. Input capture is correct. |
| UI-1.6 | Speech bubbles for Say (lifetime formula, 2-per-speaker cap, wrap/ellipsis, muted hidden) | M | 1.4, 1.5 | Bubbles track speakers, stack and expire per spec. Unit tests cover lifetime and stacking. |
| UI-1.7 | Mute + report from name/bubble context menu (wire `ClientReportPlayerMsg`) | M | 1.5; [net] report | A muted player's chat and bubbles are hidden for the session. Reports reach the server log. |
| UI-1.8 | Toasts/notifications (stack, merge, kinds) | S | 0.14 | Toasts show, merge and expire. Errors are visually distinct with an icon. |
| UI-1.9 | Connection UX: latency indicator, "Reconnecting…" banner, disconnect modal, `ServerBannedMsg` modal, `MoveRejected` toast | M | 1.8; [net] reject/ban wiring | Killing the server shows the banner and then the modal, never a frozen window or a silent exit. |
| UI-1.10 | Player unit frame v1 (name, connection/status icons; bars stubbed) | S | 0.7 | The frame renders at its anchor at every scale. |

**Milestone 1 exit criteria:** the 1.2 readability and accessibility targets
are met for all Phase 1 screens. The UI perf budget is met with 20 clients.
Keyboard-only and pad-only play-through of every Phase 1 screen.

### 3.4 Phase 2 — Combat UI (P2; starts when combat lands)

| ID | Task | Size | Blocked by | Done when |
|---|---|---|---|---|
| UI-2.1 | Extend `ServerAttackResultMsg` with kind/crit/heal semantics; emit `CombatResult` events | S | [net][game] combat | Events arrive in the UI queue with full data. |
| UI-2.2 | Floating combat text: pool (40), kinds and non-colour cues, aggregation (150ms), lanes, reduce-motion variant, category options | M | 2.1 | Aggregation and pool unit tests pass. Zero allocations per hit. A 100-hits/sec stress test stays within budget. |
| UI-2.3 | HP/resource bars on unit frames with ghost-trail drain and heal highlight; numeric format option | M | [game] HP in state; 1.10 | Bars animate per spec. Low-HP cue is non-colour and < 1Hz. |
| UI-2.4 | Target selection (click, Tab-cycle, pad cycle) + target frame + target-of-target | M | 2.3 | Targeting works with all three input methods. The target's plate is highlighted. |
| UI-2.5 | Nameplate health/cast bars and in-combat visibility rules | S | 1.4, 2.3 | Plates show HP only when damaged or in combat (default), per option. |
| UI-2.6 | Action bar with cooldown sweeps, predicted cooldown + rollback on reject, keybind labels | L | [game] abilities; 0.8 | Cooldown starts on press. A server reject rolls it back with a toast. |
| UI-2.7 | Cast bar (self + target), interrupt feedback | S | [game] casting | — |
| UI-2.8 | Buff/debuff rows with duration and stacks, tooltips | M | [game] auras; 0.14 | — |
| UI-2.9 | Meta effects: low-HP vignette, damage flash obeying flash/motion limits | S | 2.3 | Off/reduced options are honoured. Flash rate ≤ 3/s. |
| UI-2.10 | Party frames | M | [game] party; 2.3 | — |

### 3.5 Phase 3 — World interaction and narrative (P2)

| ID | Task | Size | Blocked by | Done when |
|---|---|---|---|---|
| UI-3.1 | Interaction prompt (best-candidate selection, binding glyph, single prompt) + wire `ClientInteractMsg` | M | [net] interact; 0.8 | The prompt appears for the nearest interactable in the facing direction. Pressing it sends the message. |
| UI-3.2 | Dialogue box presenter: 3-line box, name tab, portrait slot, typewriter + instant-complete, continue indicator, backlog | M | 0.6, 0.14 | All keyboard/pad/mouse paths work. Typewriter speed option is honoured. |
| UI-3.3 | Signs as single-node dialogues | S | 3.1, 3.2 | A sign in the test level is readable. |
| UI-3.4 | Dialogue data model + TOML loader + condition evaluator in `shared/` | M | — | Parser and evaluator unit tests. Usable by both client and server. |
| UI-3.5 | Dialogue validator (unreachable/dangling/unknown-string/too-many-choices) running in CI | S | 3.4, 0.13 | CI fails on a broken sample file. |
| UI-3.6 | Server-authoritative dialogue protocol (`ServerDialogueMsg`, `ClientDialogueChoiceMsg`), effects on server, range-cancel | L | 3.4; [net] | An illegal choice index is rejected server-side, with a test. Walking away ends the conversation. |
| UI-3.7 | Choice list UI (≤ 4, numeric hotkeys) | S | 3.2 | — |
| UI-3.8 | World markers (`!`, `?`, `…`) integrated with nameplate declutter | S | 1.4, [game] quests | — |
| UI-3.9 | Quest tracker HUD element | M | [game] quests | — |

### 3.6 Phase 4 — Menus and meta screens (P2/P3)

| ID | Task | Pri | Size | Blocked by |
|---|---|---|---|---|
| UI-4.1 | Shared tabbed game-menu frame (tab switching with Q/E and LB/RB, remembers tab/scroll) | P2 | M | 0.14 |
| UI-4.2 | Inventory grid (drag-drop, pad-friendly move mode, tooltips, compare) | P2 | L | 4.1; [game] items |
| UI-4.3 | Character/equipment screen | P2 | M | 4.1; [game] items/stats |
| UI-4.4 | Minimap + full map (zone name, player marker, party markers) | P2 | L | 0.2; [game] levels (`ClientLoadLevelMsg`) |
| UI-4.5 | Quest log | P2 | M | 4.1; [game] quests |
| UI-4.6 | Social: friends, ignore list, party invite flow | P3 | L | 1.5, 1.7 |
| UI-4.7 | Pre-game: title/login, character select/create, loading screen with tips and cancel | P2 | L | [net] auth (designed, not built) |
| UI-4.8 | Chat v2: Party/Guild/Whisper tabs, clickable names, timestamps | P3 | M | 1.5; [game] party/guild |

### 3.7 Phase 5 — Customisation and advanced accessibility (P3)

| ID | Task | Size | Notes |
|---|---|---|---|
| UI-5.1 | HUD Edit Mode: drag/resize/scale elements, snap grid, saved named layouts (per device) | L | Possible only because of the 0.7 / 2.6 data-driven layout. Inspired by WoW Edit Mode and FFXIV HUD Layout. |
| UI-5.2 | Colour-vision-deficiency token tables + high-contrast theme, validated by the contrast test | M | Tokens make this mostly data. |
| UI-5.3 | Dyslexia-friendly font option; per-surface text size | S | Font roles exist from 0.5. |
| UI-5.4 | Menu narration / text-to-speech hooks (screen-reader style announcements of focused widget) | L | Focus system (0.9) is the integration point. |
| UI-5.5 | Options search box | S | Once options exceed ~40. |
| UI-5.6 | Screenshot-diff harness for every screen × 3 scales × pseudo-locale | M | Turns 2.8's manual visual check into a CI artifact. |
| UI-5.7 | Real localisation pass (first non-English language) | L | `loc` exists from 0.13. Mainly translation and fonts. |
| UI-5.8 | UI usability telemetry (screen open counts, time-in-menu, option changes) for future decisions | M | Respect privacy and opt-in. |

### 3.8 Priority summary

```
NOW        P0  Foundations ───────────────► UI-0.1 … 0.14   (no gameplay deps)
NEXT       P1  Social MVP  ───────────────► UI-1.1 … 1.10   (needs: names, chat, report wiring)
WHEN READY P2  Combat UI   ◄── combat system                UI-2.x
           P2  Narrative   ◄── interact + dialogue protocol UI-3.x
           P2/3 Menus      ◄── items, quests, levels, auth  UI-4.x
LATER      P3  Customisation & advanced a11y                UI-5.x
```

**Recommended first slice** (about 2 weeks solo):

- UI-0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.10
- then UI-1.1 + UI-1.2

That replaces the hard-coded text and key checks with the real foundations.
After it, the game has crisp, scalable text, remappable input, a camera and
an `Esc` menu. Every later item builds on that slice without rework.

**Biggest risks to watch:**

1. **Retrofitting accessibility.** Mitigated by making the 1.2 checks unit
   tests in Phase 0, not a later audit.
2. **Network-dishonest UI** (showing predicted values as truth). Mitigated by
   the event/command split and the "server-confirmed only" rule for combat
   text and dialogue.
3. **UI frame-time creep as nameplates and combat text grow.** Mitigated by
   fixed pools, caps, the `F3` budget overlay and benchmark tests.
4. **Protocol work blocking UI.** Many P1/P2 items are blocked on `[net]`
   wiring, not UI effort. Schedule the protocol tasks (names, chat,
   interact/report) alongside Phase 0 so they're ready when Phase 1 starts.

---

## Sources

- Xbox Accessibility Guidelines — [XAG 101 Text display](https://devdocs.xbox.com/build/game-principles/accessibility/xag-deep-dives/xag-101-text-display.md), [XAG 102 Contrast](https://devdocs.xbox.com/build/game-principles/accessibility/xag-deep-dives/xag-102-contrast.md), [handheld guidelines](https://devdocs.xbox.com/build/gdk-and-engines/handheld/handheld-guidelines-and-testcases), [guidelines index](https://learn.microsoft.com/en-us/gaming/accessibility/guidelines)
- [Game Accessibility Guidelines — basic tier](https://gameaccessibilityguidelines.com/basic/) and [site](https://gameaccessibilityguidelines.com/)
- [IGDA GA-SIG Game Accessibility Top Ten](https://igda-gasig.org/?p=42)
- WCAG 2.x success criteria 1.4.3 (contrast), 1.4.11 (non-text contrast), 2.3.1 (three flashes)
- Nielsen Norman Group — response time limits (0.1s / 1s / 10s) and 10 usability heuristics
- Fagerholt, E. & Lorentzon, M. (2009), *Beyond the HUD: User Interfaces for Increased Player Immersion in FPS Games*, Chalmers University (diegetic / non-diegetic / spatial / meta taxonomy)
- [raylib-go](https://github.com/gen2brain/raylib-go) — `raygui` and `easings` sub-packages
