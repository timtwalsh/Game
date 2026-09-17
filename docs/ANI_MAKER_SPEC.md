# ANIFile Animation Maker - Comprehensive Technical Specification
## Go + Fyne Desktop Animation Composition Tool

**Version**: 1.0  
**Status**: Design Phase  
**Language**: Go 1.21+  
**GUI Framework**: Fyne 2.4+  
**Target Platforms**: Windows, macOS, Linux  
**Build Type**: Single binary, no installer  

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Data Models](#data-models)
4. [File Formats](#file-formats)
5. [User Interface Design](#user-interface-design)
6. [Core Features](#core-features)
7. [Workflows](#workflows)
8. [Implementation Guide](#implementation-guide)
9. [API Reference](#api-reference)
10. [Code Examples](#code-examples)
11. [Build & Deployment](#build--deployment)
12. [Testing Strategy](#testing-strategy)

---

## Overview

### Purpose

ANIFile Animation Maker is a desktop tool for creating **frame-by-frame pixel art animations** with:
- Sprite sheet import and gridded sprite selection
- Visual hitbox editing (collision and attack boxes)
- Frame-level events (sound, particles, screen shake)
- Animation composition (nesting animations)
- Real-time preview with playback controls

### Target Users

- **Game animators** creating 2D pixel art animations
- **Level designers** adding character/NPC animations
- **VFX artists** creating particle/effect animations
- **Tool developers** building animation systems

### Key Design Principles

1. **Speed over perfection** - Duplicate frames and modify, not create from scratch
2. **Visual feedback** - See hitboxes, events, and preview in real-time
3. **Simplicity** - Single-window interface, minimal dialogs
4. **Portability** - One binary, no external dependencies or installers
5. **Iteration-friendly** - Keyboard shortcuts for power users

---

## Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    ANIFile Animation Maker                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              UI Layer (Fyne)                         │  │
│  │  ┌─────────┬──────────────┬──────────────────────┐   │  │
│  │  │  Menu   │   Canvas     │   Properties Panel   │   │  │
│  │  │  Bar    │   (sprites)  │   (inputs/sliders)   │   │  │
│  │  └─────────┴──────────────┴──────────────────────┘   │  │
│  │  ┌──────────────────────────────────────────────────┐   │
│  │  │        Timeline (frames, scrubber, playback)     │   │
│  │  └──────────────────────────────────────────────────┘   │
│  └──────────────────────────────────────────────────────┘  │
│                           ↓                                 │
│  ┌──────────────────────────────────────────────────────┐  │
│  │           Editor Layer (State & Logic)               │  │
│  │  ┌──────────────┐  ┌──────────────┐                 │  │
│  │  │  Animation   │  │  Keyframe    │  ┌───────────┐  │  │
│  │  │  Project     │  │  Manager     │  │SpriteSheet│  │  │
│  │  └──────────────┘  └──────────────┘  └───────────┘  │  │
│  │                           ↓                           │  │
│  │  ┌──────────────────────────────────────────────────┐  │
│  │  │        Undo/Redo Stack (History)                │  │
│  │  └──────────────────────────────────────────────────┘  │
│  └──────────────────────────────────────────────────────┘  │
│                           ↓                                 │
│  ┌──────────────────────────────────────────────────────┐  │
│  │           File Layer (Persistence)                  │  │
│  │  ┌──────────┐  ┌──────────┐  ┌─────────────────┐   │  │
│  │  │TOML Save │  │PNG/JPG   │  │Asset Management │   │  │
│  │  │/Load     │  │Loading   │  │(sounds, etc.)   │   │  │
│  │  └──────────┘  └──────────┘  └─────────────────┘   │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Layer Responsibilities

**UI Layer (Fyne)**
- Window management
- Input handling (clicks, keyboard, drag-drop)
- Canvas rendering (sprites, hitboxes, grid)
- Property widget updates
- Timeline visualization

**Editor Layer**
- Project state management
- Keyframe operations (add, delete, duplicate)
- Undo/redo stack
- Sprite sheet metadata
- Animation composition logic

**File Layer**
- TOML serialization (.anif, .sprsh)
- PNG/JPG image loading
- Asset file management
- Path resolution

### Component Diagram

```
main
├── App
│   ├── Project (current animation)
│   ├── UndoStack
│   ├── AssetManager
│   └── UI
│       ├── MainWindow
│       ├── CanvasWidget
│       ├── PropertiesPanel
│       ├── TimelineWidget
│       └── Dialogs

Project
├── Metadata
├── Animation
│   ├── KeyFrames[]
│   ├── NestedAnimations[]
│   └── Config
├── SpriteSheets (imported)
└── SavePath
```

---

## Data Models

### Core Structs

#### Animation

```go
type Animation struct {
    Metadata AnimationMetadata
    Config   AnimationConfig
    KeyFrames []*KeyFrame
    Nested   []*NestedAnimation
}

type AnimationMetadata struct {
    Name        string    // e.g., "player_slash_right"
    Version     string    // e.g., "1.0"
    Description string
    Author      string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type AnimationConfig struct {
    Loop            bool      // Does animation loop?
    DefaultSpeed    float32   // Speed multiplier (1.0 = normal)
    CharacterSize   string    // "16x16", "24x24", "32x32", "48x32", "custom"
    RootAnchor      string    // "feet" or "center"
}
```

#### KeyFrame

```go
type KeyFrame struct {
    ID              int
    Duration        uint32           // milliseconds
    Sprite          SpriteReference  // which sprite to display
    Speed           float32          // Movement speed modifier (1.0 = normal)
    
    HitBox          *Box             // Collision box (nil if none)
    AttackHitBox    *Box             // Attack/damage box (nil if none)
    
    Events          []Event          // Sound, particles, shake
    
    // Internal
    DisplayOffset   Point            // For variable sprite sizes
}

type Box struct {
    X int
    Y int
    W int
    H int
}

type Point struct {
    X int
    Y int
}

type SpriteReference struct {
    SheetName string // "player_sheet"
    Index     int    // Sprite index in grid
    
    // OR absolute (if sheet not available)
    Absolute *AbsoluteSpriteRef
}

type AbsoluteSpriteRef struct {
    FilePath string
    X, Y, W, H int
}
```

#### Event

```go
type Event interface {
    EventType() string
}

type SoundEvent struct {
    FilePath string
    Pitch    float32  // 1.0 = normal, 0.5 = lower, 2.0 = higher
}

func (e *SoundEvent) EventType() string { return "sound" }

type ParticleEvent struct {
    Type string        // "slash_spark", "dust_cloud", etc.
    X    int           // Offset from sprite origin
    Y    int
    Rotation *float32 // Optional
    Scale    *float32 // Optional
}

func (e *ParticleEvent) EventType() string { return "particle" }

type ShakeEvent struct {
    DurationMs uint32  // milliseconds
    Intensity  float32 // 0.0 - 1.0
}

func (e *ShakeEvent) EventType() string { return "shake" }

type FlashEvent struct {
    Color      string  // hex color: "#FFFFFF"
    DurationMs uint32
    Opacity    float32 // 0.0 - 1.0
}

func (e *FlashEvent) EventType() string { return "flash" }
```

#### SpriteSheet

```go
type SpriteSheet struct {
    Name       string
    FilePath   string
    Image      image.Image    // Loaded PNG/JPG
    
    GridConfig GridConfig
    Sprites    []SpriteInfo   // Computed from grid
}

type GridConfig struct {
    Cols   int // Sprites horizontally
    Rows   int // Sprites vertically
    TileW  int // Width of each sprite
    TileH  int // Height of each sprite
}

type SpriteInfo struct {
    Index   int
    X       int    // Pixel position in sheet
    Y       int
    W       int    // Width
    H       int    // Height
    Label   string // Optional: "walk_0", "attack_1", etc.
}

// Computed property
func (ss *SpriteSheet) GetSprite(index int) *image.Rectangle {
    if index < 0 || index >= len(ss.Sprites) {
        return nil
    }
    s := ss.Sprites[index]
    return &image.Rectangle{
        Min: image.Pt(s.X, s.Y),
        Max: image.Pt(s.X+s.W, s.Y+s.H),
    }
}
```

#### NestedAnimation

```go
type NestedAnimation struct {
    KeyFrameID   int    // Which keyframe this starts on
    AnimationPath string // Path to .anif file
    
    Offset Point
    Scale  float32   // 1.0 = 100%
    Opacity float32  // 0.0 - 1.0
}
```

#### Project (State Container)

```go
type Project struct {
    CurrentAnimation *Animation
    LoadedSheets     map[string]*SpriteSheet  // "player_sheet" → SpriteSheet
    SavePath         string                    // Path to .anif file
    Dirty            bool                      // Unsaved changes?
    
    UndoStack        *UndoStack
    AssetManager     *AssetManager
}

type UndoStack struct {
    Past    []*ProjectState
    Future  []*ProjectState
    
    maxSize int // Limit undo history (e.g., 100)
}

type ProjectState struct {
    Animation      *Animation
    LoadedSheets   map[string]*SpriteSheet
    Timestamp      time.Time
}
```

---

## File Formats

### .anif - Animation File (TOML)

**Purpose**: Store animation data (keyframes, events, structure)  
**Encoding**: TOML (human-readable, git-friendly)

```toml
# ============================================================================
# METADATA
# ============================================================================
[metadata]
name = "player_slash_right"
version = "1.0"
description = "Player sword slash attack, right direction"
author = "animator_name"

# ============================================================================
# ANIMATION CONFIG
# ============================================================================
[animation]
loop = true
default_speed = 1.0
character_size = "32x32"        # tiny (16x16), small (24x24), medium (32x32), large (48x32), custom
root_anchor = "feet"            # feet or center

# ============================================================================
# KEYFRAMES
# ============================================================================
[[keyframes]]
id = 0
duration_ms = 80

# Sprite reference: "sheet_name:index"
# Example: "player_sheet:0" means sprite 0 from player_sheet
# If sheet not loaded, falls back to absolute reference
sprite = "player_sheet:0"

# Optional: Absolute sprite reference (if sheet not available)
# sprite_absolute = { file = "assets/fallback.png", x = 0, y = 0, w = 32, h = 32 }

# Collision box (what the character occupies)
hitbox = { x = 6, y = 8, w = 20, h = 20 }

# Attack/damage box (where attacks connect)
attack_hitbox = { x = 10, y = 4, w = 16, h = 24 }

# Speed modifier for walk cycles, runs, etc.
speed_modifier = 1.0

# ---- EVENTS ----
sound = "sfx/slash.wav"
sound_pitch = 1.0

# Particles
[[keyframes.particles]]
type = "slash_spark"
x = 15
y = 10
rotation = 45.0       # Optional
scale = 1.0           # Optional

# Screen shake
[keyframes.shake]
duration_ms = 50
intensity = 0.5

# Screen flash
[keyframes.flash]
color = "#FFFFFF"
duration_ms = 30
opacity = 0.3


[[keyframes]]
id = 1
duration_ms = 100
sprite = "player_sheet:1"
attack_hitbox = { x = 12, y = 6, w = 14, h = 22 }
sound_pitch = 0.95


[[keyframes]]
id = 2
duration_ms = 120
sprite = "player_sheet:2"
# Recovery frame - no hitbox

# ============================================================================
# NESTED ANIMATIONS (animation composition)
# ============================================================================
[[nested]]
keyframe = 0                      # Start on this keyframe
animation = "dust_cloud.anif"     # Relative path or absolute
offset = { x = 0, y = 16 }       # Position offset
scale = 1.0
opacity = 0.8

[[nested]]
keyframe = 1
animation = "spark_burst.anif"
offset = { x = 5, y = 5 }
```

**TOML Validation Rules:**
- `keyframes[].id` must be sequential starting at 0
- `keyframes[].duration_ms` must be > 0
- `sprite` must exist (sheet:index or absolute path must be valid)
- `hitbox` and `attack_hitbox` are optional
- `nested[].keyframe` must reference a valid keyframe ID

### .sprsh - Sprite Sheet Metadata (TOML)

**Purpose**: Describe how a sprite sheet is divided into tiles  
**Created**: Automatically when importing sprite sheet  
**Usage**: Tool remembers grid setup for future use

```toml
# ============================================================================
# SHEET INFO
# ============================================================================
[sheet]
name = "player_sheet"
file = "assets/player.png"        # Relative to project root
width = 160
height = 384

# ============================================================================
# GRID LAYOUT
# ============================================================================
[grid]
cols = 5                          # 5 sprites wide
rows = 12                         # 12 sprites high
tile_width = 32                   # Each sprite is 32×32
tile_height = 32

# ============================================================================
# INDIVIDUAL SPRITES (optional, auto-generated)
# ============================================================================
# Only needed if sprites have custom labels or non-uniform sizes

[[sprites]]
index = 0
x = 0
y = 0
w = 32
h = 32
label = "idle_1"

[[sprites]]
index = 1
x = 32
y = 0
w = 32
h = 32
label = "idle_2"

# ... remaining 58 sprites (auto-generated by importer)
```

**Notes:**
- .sprsh files are generated by the tool during import
- Users can hand-edit if they want custom labels
- Grid must be regular (all tiles same size) for now
- Support for irregular grids is future work

---

## User Interface Design

### Window Layout

#### Overall Structure

```
┌─────────────────────────────────────────────────────────────────────┐
│ File  Edit  Animation  View  Help                        [_][□][X]  │ Menu Bar
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌────────────────────────────────┐  ┌─────────────────────────┐   │
│  │                                │  │   PROPERTIES PANEL      │   │
│  │                                │  │  ┌─────────────────────┐ │   │
│  │      CANVAS AREA               │  │  │ Frame 0 of 3        │ │   │
│  │                                │  │  │ Duration: [80] ms   │ │   │
│  │   (Sprite display +            │  │  │ Speed: [1.0]       │ │   │
│  │    Hitbox editing)             │  │  │                     │ │   │
│  │                                │  │  │ ┌─ COLLISION ────┐ │ │   │
│  │                                │  │  │ │ x:[6] y:[8]    │ │ │   │
│  │     [sprite 32×32]             │  │  │ │ w:[20] h:[20]  │ │ │   │
│  │                                │  │  │ │ [x] Delete     │ │ │   │
│  │  (70% of window)               │  │  │ └────────────────┘ │ │   │
│  │                                │  │  │                     │ │   │
│  │  Zoom: 100%  Grid: On          │  │  │ ┌─ ATTACK ──────┐ │ │   │
│  │  [Home]                        │  │  │ │ x:[10] y:[4]   │ │ │   │
│  │                                │  │  │ │ w:[16] h:[24]  │ │ │   │
│  │                                │  │  │ │ [x] Delete     │ │ │   │
│  │                                │  │  │ └────────────────┘ │ │   │
│  └────────────────────────────────┘  │  │                     │ │   │
│                                       │  │ SPRITE PICKER      │ │   │
│                                       │  │ [Sheet: player ▼] │ │   │
│                                       │  │ [Grid 5×12]        │ │   │
│                                       │  │                     │ │   │
│                                       │  │ [+ Add Sound]      │ │   │
│                                       │  │ [+ Add Particles]  │ │   │
│                                       │  │ [+ Add Shake]      │ │   │
│                                       │  └─────────────────────┘ │   │
│                                       └──────────────────────────┘   │
│                                                                     │
│ ┌────────────────────────────────────────────────────────────────┐ │
│ │  TIMELINE                                                      │ │
│ │  [◄◄] [◄] [►] [►►]  Speed: [100%] ▼  [✓] Loop [  ] Onion    │ │
│ │                                                                │ │
│ │  ↓ Frame 0                Frame 1            Frame 2          │ │
│ │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐          │ │
│ │  │[80ms]        │ │[100ms]       │ │[120ms]       │          │ │
│ │  │★ sound       │ │✡ particles  │ │—             │          │ │
│ │  │█ hitbox      │ │              │ │              │          │ │
│ │  │              │ │              │ │              │          │ │
│ │  └──────────────┘ └──────────────┘ └──────────────┘          │ │
│ │                                                                │ │
│ │  Frame: [0] / [3]   Total: 280ms   Loop: Yes                 │ │
│ └────────────────────────────────────────────────────────────────┘ │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

#### Column Breakdown

| Region | Width | Purpose |
|--------|-------|---------|
| Canvas Area | 70% | Sprite display, hitbox editing, grid overlay |
| Properties Panel | 30% | Keyframe properties, sprite picker, event controls |
| Timeline | 100% | Frame sequence, playback, scrubber |
| Menu Bar | 100% | File, Edit, Animation, View, Help |

### Canvas Area

**Displays:**
1. Sprite from current keyframe (scaled, centered)
2. Grid overlay (16px grid lines, 20% opacity)
3. Collision hitbox (blue border, 50% opacity)
4. Attack hitbox (red border, 50% opacity)
5. Coordinate axes (green crosshairs at 0,0)
6. Labels (X, Y, W, H values for each box)

**Interactions:**
- Left-click: Select/deselect hitbox
- Left-drag corner: Resize hitbox
- Left-drag center: Move hitbox
- Right-click: Context menu
  - "Add collision box"
  - "Remove collision box"
  - "Add attack box"
  - "Remove attack box"
  - "Show grid" / "Hide grid"
  - "Zoom in" / "Zoom out"
  - "Fit to frame"
- Scroll wheel: Zoom in/out
- Space: Play/pause animation

**Zoom Levels:**
- 50%, 100%, 200%, 400%, Fit

### Properties Panel

**Sections (scrollable):**

1. **Frame Info**
   - Frame number dropdown: "0", "1", "2", etc.
   - Duration: TextEntry (number input)
   - Speed Modifier: Slider (0.5 - 2.0)

2. **Sprite Picker**
   - Sheet selector: Select dropdown (loaded sheets)
   - Grid view: Thumbnails with click selection
   - Current sprite: Highlighted with border

3. **Collision Box**
   - X, Y, W, H: TextEntry fields
   - [Delete] button
   - Or "Add collision box" if none exists

4. **Attack Box**
   - X, Y, W, H: TextEntry fields
   - [Delete] button
   - Or "Add attack box" if none exists

5. **Events**
   - List of active events (sound, particles, shake)
   - Per event: [Edit] [Delete] buttons
   - [+ Add Sound] button
   - [+ Add Particles] button
   - [+ Add Shake] button

6. **Nested Animations**
   - List of nested animations
   - Per nested: [Edit] [Delete] buttons
   - [+ Add Nested] button

**Sizing:**
- Small inputs: ~60px (X, Y)
- Medium inputs: ~80px (W, H, Duration)
- Sliders: full width minus padding

### Timeline

**Layout:**
```
Controls:
  [◄◄] [◄] [►] [►►] | Speed: [50%] [100%] [200%] ▼
  [✓] Loop  [  ] Onion Skin  [  ] Show Events
  
Frame List:
  ↓ Scrubber
  ┌────────┐ ┌────────┐ ┌────────┐ ...
  │Frame 0 │ │Frame 1 │ │Frame 2 │
  │80ms    │ │100ms   │ │120ms   │
  │★ ✡ █  │ │✡       │ │—       │
  └────────┘ └────────┘ └────────┘
```

**Frame Box Contents:**
- Frame number (top left, small text)
- Duration (large text, center)
- Event icons (bottom)
  - ★ = sound
  - ✡ = particles
  - █ = hitbox active
  - ⚡ = shake
  - — = no events

**Interactions:**
- Click frame: Select (highlight with border)
- Double-click frame: Edit duration (inline text entry)
- Right-click frame: Context menu
  - "Duplicate frame"
  - "Delete frame"
  - "Move left" / "Move right"
  - "Copy" / "Paste after"
- Drag frame: Reorder (future feature)
- Drag scrubber: Seek to frame
- Playback controls: Standard media player (play, pause, step, loop)

---

## Core Features

### Feature 1: Sprite Sheet Import

**Goal**: Load PNG/JPG and define grid of sprites

**User Flow:**
```
File → Import Sprite Sheet
├─ File picker: Select PNG/JPG
├─ Dialog: "Import Sprite Sheet"
│  ├─ Sheet name: [player_sheet]
│  ├─ Grid
│  │  ├─ Columns: [5]
│  │  ├─ Rows: [12]
│  │  ├─ Tile Width: [32]
│  │  ├─ Tile Height: [32]
│  ├─ Preview: Image with grid overlay
│  └─ [Import] [Cancel]
└─ Result: player_sheet.sprsh created, sheet available in sprite picker
```

**Implementation:**
```go
func (app *App) ImportSpriteSheet(filePath string, config GridConfig) error {
    // Load image
    img, err := loadImage(filePath)
    if err != nil { return err }
    
    // Create sheet
    sheet := &SpriteSheet{
        Name:       extractName(filePath),
        FilePath:   filePath,
        Image:      img,
        GridConfig: config,
        Sprites:    computeGridSprites(img, config),
    }
    
    // Store
    app.Project.LoadedSheets[sheet.Name] = sheet
    
    // Save metadata
    saveSpriteSheetMetadata(sheet)
    
    return nil
}

func computeGridSprites(img image.Image, cfg GridConfig) []SpriteInfo {
    sprites := make([]SpriteInfo, 0, cfg.Cols * cfg.Rows)
    for row := 0; row < cfg.Rows; row++ {
        for col := 0; col < cfg.Cols; col++ {
            idx := row * cfg.Cols + col
            sprites = append(sprites, SpriteInfo{
                Index: idx,
                X:     col * cfg.TileW,
                Y:     row * cfg.TileH,
                W:     cfg.TileW,
                H:     cfg.TileH,
            })
        }
    }
    return sprites
}
```

### Feature 2: Keyframe Management

**Operations:**
- Add keyframe
- Delete keyframe
- Duplicate keyframe
- Reorder keyframes
- Edit keyframe properties

**Add Keyframe:**
```go
func (proj *Project) AddKeyFrame(afterID int) (*KeyFrame, error) {
    newID := afterID + 1
    
    // Create new keyframe
    kf := &KeyFrame{
        ID:       newID,
        Duration: 100,  // Default 100ms
        Sprite:   SpriteReference{},
        Speed:    1.0,
        Events:   []Event{},
    }
    
    // Insert into animation
    proj.CurrentAnimation.KeyFrames = append(proj.CurrentAnimation.KeyFrames, kf)
    
    // Record in undo stack
    proj.UndoStack.Push(proj.CurrentState())
    proj.Dirty = true
    
    return kf, nil
}
```

**Duplicate Keyframe:**
```go
func (proj *Project) DuplicateKeyFrame(id int) (*KeyFrame, error) {
    src := proj.CurrentAnimation.KeyFrames[id]
    
    // Deep copy
    newKF := *src
    newKF.ID = len(proj.CurrentAnimation.KeyFrames)
    
    // Copy events
    newKF.Events = make([]Event, len(src.Events))
    for i, e := range src.Events {
        newKF.Events[i] = copyEvent(e)
    }
    
    // Copy hitboxes
    if src.HitBox != nil {
        newKF.HitBox = &Box{src.HitBox.X, src.HitBox.Y, src.HitBox.W, src.HitBox.H}
    }
    if src.AttackHitBox != nil {
        newKF.AttackHitBox = &Box{src.AttackHitBox.X, src.AttackHitBox.Y, src.AttackHitBox.W, src.AttackHitBox.H}
    }
    
    proj.CurrentAnimation.KeyFrames = append(proj.CurrentAnimation.KeyFrames, &newKF)
    proj.UndoStack.Push(proj.CurrentState())
    proj.Dirty = true
    
    return &newKF, nil
}
```

### Feature 3: Visual Hitbox Editing

**Canvas Operations:**
1. Draw hitbox: Right-click → "Add collision box" → Click-drag to define
2. Edit hitbox: Click corners to drag-resize, click center to drag-move
3. Delete hitbox: Right-click → "Delete collision box"

**Implementation:**
```go
type CanvasWidget struct {
    currentKeyFrame *KeyFrame
    selectedBox     BoxType // BoxCollision or BoxAttack
    dragHandle      Handle  // CornerTL, CornerBR, Center, etc.
    
    onBoxChanged func(*Box)
}

func (cw *CanvasWidget) MouseMoved(me *fyne.MouseMovedEvent) {
    if cw.dragHandle == HandleNone {
        return  // No drag active
    }
    
    // Update box based on drag
    delta := me.Position.Subtract(cw.lastMousePos)
    
    box := cw.selectedBox.GetBox(cw.currentKeyFrame)
    if box == nil { return }
    
    switch cw.dragHandle {
    case HandleCornerTL:
        box.X += int(delta.X)
        box.Y += int(delta.Y)
        box.W -= int(delta.X)
        box.H -= int(delta.Y)
    case HandleCornerBR:
        box.W += int(delta.X)
        box.H += int(delta.Y)
    case HandleCenter:
        box.X += int(delta.X)
        box.Y += int(delta.Y)
    }
    
    cw.lastMousePos = me.Position
    cw.onBoxChanged(box)
    cw.Refresh()
}

type BoxType int
const (
    BoxCollision BoxType = iota
    BoxAttack
)

func (bt BoxType) GetBox(kf *KeyFrame) *Box {
    if bt == BoxCollision {
        return kf.HitBox
    }
    return kf.AttackHitBox
}
```

### Feature 4: Animation Playback

**Playback State:**
```go
type PlaybackState struct {
    IsPlaying    bool
    CurrentFrame int
    ElapsedMs    uint32
    LoopEnabled  bool
    SpeedFactor  float32  // 1.0 = normal, 2.0 = 2x fast
}

func (ps *PlaybackState) Update(deltaMs uint32) bool {
    if !ps.IsPlaying {
        return false
    }
    
    ps.ElapsedMs += uint32(float32(deltaMs) * ps.SpeedFactor)
    return true  // State changed
}

func (proj *Project) AdvancePlayback(deltaMs uint32) {
    if !proj.Playback.IsPlaying {
        return
    }
    
    anim := proj.CurrentAnimation
    totalMs := calculateAnimationDuration(anim)
    
    proj.Playback.ElapsedMs += uint32(float32(deltaMs) * proj.Playback.SpeedFactor)
    
    // Wrap or stop at end
    if proj.Playback.ElapsedMs >= totalMs {
        if proj.Playback.LoopEnabled {
            proj.Playback.ElapsedMs = 0
        } else {
            proj.Playback.IsPlaying = false
            proj.Playback.ElapsedMs = totalMs - 1
        }
    }
    
    proj.Playback.CurrentFrame = getKeyFrameAtTime(anim, proj.Playback.ElapsedMs)
}

func getKeyFrameAtTime(anim *Animation, elapsedMs uint32) int {
    time := uint32(0)
    for i, kf := range anim.KeyFrames {
        time += kf.Duration
        if elapsedMs < time {
            return i
        }
    }
    return len(anim.KeyFrames) - 1
}
```

### Feature 5: Undo/Redo

**Implementation:**
```go
type UndoStack struct {
    Past   []*ProjectState
    Future []*ProjectState
    MaxSize int
}

type ProjectState struct {
    Animation   *Animation
    Sheets      map[string]*SpriteSheet
    Timestamp   time.Time
}

func (us *UndoStack) Push(state *ProjectState) {
    us.Past = append(us.Past, state)
    us.Future = nil  // Clear future
    
    // Limit size
    if len(us.Past) > us.MaxSize {
        us.Past = us.Past[1:]
    }
}

func (us *UndoStack) Undo() *ProjectState {
    if len(us.Past) == 0 {
        return nil
    }
    
    state := us.Past[len(us.Past) - 1]
    us.Past = us.Past[:len(us.Past) - 1]
    us.Future = append(us.Future, state)
    
    return us.Past[len(us.Past) - 1]
}

func (us *UndoStack) Redo() *ProjectState {
    if len(us.Future) == 0 {
        return nil
    }
    
    state := us.Future[len(us.Future) - 1]
    us.Future = us.Future[:len(us.Future) - 1]
    us.Past = append(us.Past, state)
    
    return state
}
```

### Feature 6: File Operations

**Save Animation:**
```go
func (proj *Project) SaveAnimation(filePath string) error {
    // Convert to TOML structure
    tomlAnim := toml.Marshal(proj.CurrentAnimation)
    
    // Write to file
    return os.WriteFile(filePath, tomlAnim, 0644)
}

func (proj *Project) LoadAnimation(filePath string) error {
    // Read TOML
    data, err := os.ReadFile(filePath)
    if err != nil {
        return err
    }
    
    // Parse
    anim := &Animation{}
    if err := toml.Unmarshal(data, anim); err != nil {
        return err
    }
    
    proj.CurrentAnimation = anim
    proj.SavePath = filePath
    proj.Dirty = false
    
    // Load referenced sprite sheets
    for _, kf := range anim.KeyFrames {
        if kf.Sprite.SheetName != "" {
            proj.loadSpriteSheet(kf.Sprite.SheetName)
        }
    }
    
    return nil
}
```

---

## Workflows

### Workflow 1: Import Sprite Sheet

**Objective**: Import a PNG image and define sprite grid

**Steps:**
1. File → Import Sprite Sheet
2. Choose PNG file (player.png: 160×384px)
3. Enter sheet name: "player_sheet"
4. Enter grid: 5 cols × 12 rows, 32×32 tiles
5. Preview shows grid overlay
6. Click Import
7. Result: player_sheet.sprsh created, sheet available in sprite picker

**Expected Outcome:**
- 60 sprites (5×12) indexed 0-59
- Grid metadata saved for future use
- Sprite picker shows sheet with thumbnails

### Workflow 2: Create Simple Walk Cycle (4 frames)

**Objective**: Create a basic walk animation

**Steps:**

```
1. File → New Animation
   Name: "player_walk_right"
   Character Size: 32×32
   Loop: Yes
   
2. Add Frame 0
   - Timeline → [+ Add Frame]
   - Sprite Picker: Select "player_sheet"
   - Click sprite 0
   - Duration: 100ms
   - Speed Modifier: 1.0
   - Add sound: "footstep.wav"
   - No hitbox (walk doesn't need attack box)
   - Click canvas → Right-click → "Add collision box"
   - Draw box around feet
   - Result: Frame 0 with sound event
   
3. Duplicate Frame 0 → Frame 1
   - Timeline → Right-click Frame 0 → "Duplicate"
   - Sprite Picker: Click sprite 1
   - Duration: 100ms (auto-copied)
   - Done
   
4. Duplicate Frame 1 → Frame 2
   - Timeline → Right-click Frame 1 → "Duplicate"
   - Sprite Picker: Click sprite 2
   - Done
   
5. Duplicate Frame 2 → Frame 3
   - Timeline → Right-click Frame 2 → "Duplicate"
   - Sprite Picker: Click sprite 3
   - Done
   
6. Preview
   - Timeline → [Play] or press Space
   - Animation loops, showing walk cycle
   - Hear footstep sound on frames 0 and (if added to frame 2)
   
7. Save
   - File → Save (Ctrl+S)
   - Saves as player_walk_right.anif
```

**Total Time**: ~3 minutes

### Workflow 3: Create Attack with Hitboxes

**Objective**: Create 3-frame attack animation with collision and attack boxes

**Steps:**

```
1. New Animation
   Name: "player_slash_right"
   Loop: No
   
2. Add Frame 0
   - Add Frame
   - Sprite: player_sheet:5
   - Duration: 80ms
   - Add collision box (where player stands)
   - Add attack box (where sword is)
   - Add sound: "slash.wav"
   - Add particles: "slash_spark" at offset (15, 10)
   - Add shake: 50ms, intensity 0.5
   
3. Duplicate → Frame 1
   - Sprite: player_sheet:6
   - Adjust attack_hitbox (sword moved)
   - Sound + particles + shake auto-copied
   
4. Duplicate → Frame 2
   - Sprite: player_sheet:7
   - Remove attack_hitbox (recovery, no damage)
   - Clear sound (no sound on recovery)
   
5. Preview + Save
```

**Total Time**: ~5 minutes

### Workflow 4: Compose Animations (Nested)

**Objective**: Create "walk with dust clouds" by composing walk + dust animations

**Steps:**

```
1. Assume you have:
   - walk_right.anif (4 frames)
   - dust_cloud.anif (2 frames, loops)
   
2. Open walk_right.anif
   
3. Add Nested Animation
   - Properties → [+ Add Nested]
   - Select animation: dust_cloud.anif
   - Start keyframe: 0
   - Offset: (0, 16) - at feet level
   - Scale: 1.0
   - Opacity: 0.8
   - Repeat for frame 2 (second footstep)
   
4. Preview
   - Play walk_right
   - Watch dust clouds appear at feet on frames 0 and 2
   - Dust animation loops 2x while walk plays once
   
5. Save
```

**Result**: walk_right now includes dust effects without editing dust_cloud

---

## Implementation Guide

### Project Structure

```go
ANIFile-maker/
├── main.go                 # Entry point
│   └── func main()
│
├── cmd/
│   └── ANIFile-maker/
│       └── main.go        # Executable entry
│
├── pkg/
│   ├── app/
│   │   ├── app.go         # Main application state
│   │   ├── project.go     # Project management
│   │   └── playback.go    # Playback state
│   │
│   ├── ui/
│   │   ├── window.go      # Main window (Fyne)
│   │   ├── canvas.go      # Canvas widget for sprite display
│   │   ├── properties.go  # Properties panel
│   │   ├── timeline.go    # Timeline widget
│   │   ├── dialogs.go     # File dialogs, import dialog
│   │   └── theme.go       # Dark theme configuration
│   │
│   ├── editor/
│   │   ├── animation.go   # Animation struct + methods
│   │   ├── keyframe.go    # Keyframe operations
│   │   ├── spritesheet.go # Sprite sheet import + grid
│   │   ├── events.go      # Event types
│   │   └── undo.go        # Undo/redo stack
│   │
│   └── file/
│       ├── toml.go        # TOML marshaling
│       ├── image.go       # PNG/JPG loading
│       └── paths.go       # File utilities
│
├── assets/
│   └── (icon, samples, etc.)
│
├── go.mod
├── go.sum
└── README.md
```

### Phase 1: MVP Implementation

**Goal**: Get something clickable in 3-4 hours

**Tasks:**
1. Setup Fyne window
2. Create canvas widget (display sprite)
3. Create timeline (frame boxes)
4. Load sprite sheet manually
5. Play/pause button
6. Basic properties panel

**Code Skeleton:**

```go
// main.go
package main

import (
    "ANIFile-maker/pkg/app"
    "github.com/fyne-io/fyne/v2/app"
)

func main() {
    myApp := app.New()
    window := myApp.NewWindow()
    
    ANIFileApp := &app.Application{
        FyneApp: myApp,
    }
    
    content := ANIFileApp.BuildUI()
    window.SetContent(content)
    window.SetOnClosed(ANIFileApp.OnClose)
    
    window.ShowAndRun()
}

// pkg/app/app.go
package app

type Application struct {
    FyneApp fyne.App
    Project *Project
}

func (a *Application) BuildUI() fyne.CanvasObject {
    // Return main container with menu + canvas + properties + timeline
}
```

### Phase 2: Core Editing

**Tasks:**
1. Keyframe CRUD (add, delete, duplicate)
2. Sprite picker with grid
3. Hitbox visual editing
4. Properties panel

### Phase 3: Events & Polish

**Tasks:**
1. Sound/particle/shake events
2. Undo/redo
3. File save/load
4. Keyboard shortcuts
5. Dark theme refinement

### Phase 4: Advanced Features

**Tasks:**
1. Animation composition (nesting)
2. Frame reordering
3. Onion skin preview
4. Advanced zoom/grid

---

## API Reference

### Core Functions

#### Application

```go
type Application struct {
    FyneApp        fyne.App
    Project        *Project
    SelectedFrame  int
}

func (app *Application) BuildUI() fyne.CanvasObject
func (app *Application) OnClose()
func (app *Application) NewProject()
func (app *Application) OpenProject(path string) error
func (app *Application) SaveProject(path string) error
```

#### Project

```go
type Project struct {
    CurrentAnimation *Animation
    LoadedSheets     map[string]*SpriteSheet
    SavePath         string
    Dirty            bool
    UndoStack        *UndoStack
}

func (p *Project) AddKeyFrame(afterID int) (*KeyFrame, error)
func (p *Project) DeleteKeyFrame(id int) error
func (p *Project) DuplicateKeyFrame(id int) (*KeyFrame, error)
func (p *Project) ReorderKeyFrame(id int, newPosition int) error
func (p *Project) ImportSpriteSheet(filePath string, config GridConfig) error
func (p *Project) Save(path string) error
func (p *Project) Load(path string) error
```

#### Keyframe

```go
type KeyFrame struct {
    ID            int
    Duration      uint32
    Sprite        SpriteReference
    Speed         float32
    HitBox        *Box
    AttackHitBox  *Box
    Events        []Event
}

func (kf *KeyFrame) AddEvent(event Event) error
func (kf *KeyFrame) RemoveEvent(idx int) error
func (kf *KeyFrame) SetHitBox(box *Box) error
func (kf *KeyFrame) SetAttackHitBox(box *Box) error
```

#### SpriteSheet

```go
type SpriteSheet struct {
    Name       string
    FilePath   string
    Image      image.Image
    GridConfig GridConfig
    Sprites    []SpriteInfo
}

func LoadSpriteSheet(filePath string) (*SpriteSheet, error)
func (ss *SpriteSheet) SaveMetadata(path string) error
func (ss *SpriteSheet) GetSprite(index int) *image.Rectangle
func (ss *SpriteSheet) GetSpriteImage(index int) (image.Image, error)
```

#### Canvas Widget

```go
type CanvasWidget struct {
    CurrentKeyFrame *KeyFrame
    SelectedBox     BoxType
    OnBoxChanged    func(*Box)
}

func (cw *CanvasWidget) CreateRenderer() fyne.WidgetRenderer
func (cw *CanvasWidget) Draw(c fyne.Canvas)
func (cw *CanvasWidget) MouseDown(me *fyne.MouseDownEvent)
func (cw *CanvasWidget) MouseMoved(me *fyne.MouseMovedEvent)
func (cw *CanvasWidget) MouseUp(me *fyne.MouseUpEvent)
func (cw *CanvasWidget) SetKeyFrame(kf *KeyFrame)
func (cw *CanvasWidget) SetZoom(factor float32)
```

#### Timeline Widget

```go
type TimelineWidget struct {
    Animation       *Animation
    CurrentFrame    int
    OnFrameSelected func(int)
    OnFrameDuplicated func(int)
    OnFrameDeleted  func(int)
}

func (tw *TimelineWidget) CreateRenderer() fyne.WidgetRenderer
func (tw *TimelineWidget) MouseDown(me *fyne.MouseDownEvent)
func (tw *TimelineWidget) SetAnimation(anim *Animation)
func (tw *TimelineWidget) SetCurrentFrame(id int)
```

---

## Code Examples

### Example 1: Creating a New Project

```go
func (app *Application) NewProject() *Project {
    proj := &Project{
        CurrentAnimation: &Animation{
            Metadata: AnimationMetadata{
                Name:      "untitled",
                CreatedAt: time.Now(),
            },
            Config: AnimationConfig{
                Loop:         true,
                DefaultSpeed: 1.0,
                CharacterSize: "32x32",
            },
            KeyFrames: []*KeyFrame{},
            Nested:    []*NestedAnimation{},
        },
        LoadedSheets: make(map[string]*SpriteSheet),
        UndoStack:    &UndoStack{MaxSize: 100},
        Dirty:        false,
    }
    
    app.Project = proj
    return proj
}
```

### Example 2: Importing a Sprite Sheet

```go
func (proj *Project) ImportSpriteSheet(filePath string, config GridConfig) error {
    // Load image
    imgFile, err := os.Open(filePath)
    if err != nil {
        return fmt.Errorf("failed to open image: %w", err)
    }
    defer imgFile.Close()
    
    img, _, err := image.Decode(imgFile)
    if err != nil {
        return fmt.Errorf("failed to decode image: %w", err)
    }
    
    // Create sheet
    sheetName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
    sheet := &SpriteSheet{
        Name:       sheetName,
        FilePath:   filePath,
        Image:      img,
        GridConfig: config,
        Sprites:    computeGridSprites(img, config),
    }
    
    // Store in project
    proj.LoadedSheets[sheetName] = sheet
    
    // Save metadata
    metaPath := filePath + ".sprsh"
    if err := saveMetadataToTOML(sheet, metaPath); err != nil {
        return fmt.Errorf("failed to save metadata: %w", err)
    }
    
    return nil
}

func computeGridSprites(img image.Image, cfg GridConfig) []SpriteInfo {
    sprites := make([]SpriteInfo, cfg.Cols*cfg.Rows)
    
    for row := 0; row < cfg.Rows; row++ {
        for col := 0; col < cfg.Cols; col++ {
            idx := row*cfg.Cols + col
            sprites[idx] = SpriteInfo{
                Index: idx,
                X:     col * cfg.TileW,
                Y:     row * cfg.TileH,
                W:     cfg.TileW,
                H:     cfg.TileH,
                Label: fmt.Sprintf("sprite_%d", idx),
            }
        }
    }
    
    return sprites
}
```

### Example 3: Duplicating a Keyframe

```go
func (proj *Project) DuplicateKeyFrame(id int) (*KeyFrame, error) {
    if id < 0 || id >= len(proj.CurrentAnimation.KeyFrames) {
        return nil, fmt.Errorf("invalid keyframe id: %d", id)
    }
    
    src := proj.CurrentAnimation.KeyFrames[id]
    
    // Deep copy
    newKF := &KeyFrame{
        ID:       len(proj.CurrentAnimation.KeyFrames),
        Duration: src.Duration,
        Sprite:   src.Sprite,
        Speed:    src.Speed,
    }
    
    // Copy hitboxes
    if src.HitBox != nil {
        newKF.HitBox = &Box{src.HitBox.X, src.HitBox.Y, src.HitBox.W, src.HitBox.H}
    }
    if src.AttackHitBox != nil {
        newKF.AttackHitBox = &Box{src.AttackHitBox.X, src.AttackHitBox.Y, src.AttackHitBox.W, src.AttackHitBox.H}
    }
    
    // Copy events
    newKF.Events = make([]Event, 0, len(src.Events))
    for _, e := range src.Events {
        newKF.Events = append(newKF.Events, copyEvent(e))
    }
    
    // Add to animation
    proj.CurrentAnimation.KeyFrames = append(proj.CurrentAnimation.KeyFrames, newKF)
    
    // Record undo
    proj.UndoStack.Push(proj.currentState())
    proj.Dirty = true
    
    return newKF, nil
}

func copyEvent(e Event) Event {
    switch v := e.(type) {
    case *SoundEvent:
        return &SoundEvent{FilePath: v.FilePath, Pitch: v.Pitch}
    case *ParticleEvent:
        pe := *v
        if v.Rotation != nil {
            r := *v.Rotation
            pe.Rotation = &r
        }
        if v.Scale != nil {
            s := *v.Scale
            pe.Scale = &s
        }
        return &pe
    case *ShakeEvent:
        return &ShakeEvent{DurationMs: v.DurationMs, Intensity: v.Intensity}
    case *FlashEvent:
        return &FlashEvent{Color: v.Color, DurationMs: v.DurationMs, Opacity: v.Opacity}
    default:
        return nil
    }
}
```

### Example 4: Canvas Widget - Drawing Hitboxes

```go
func (cw *CanvasWidget) Draw(c fyne.Canvas) {
    if cw.CurrentKeyFrame == nil {
        return
    }
    
    objs := []fyne.CanvasObject{}
    
    // Draw sprite
    if cw.CurrentKeyFrame.Sprite.SheetName != "" {
        spriteImg := cw.getSpriteImage()
        objs = append(objs, canvas.NewImageFromImage(spriteImg))
    }
    
    // Draw collision hitbox (blue)
    if cw.CurrentKeyFrame.HitBox != nil {
        rect := cw.hitboxToCanvas(cw.CurrentKeyFrame.HitBox)
        outline := canvas.NewRectangle(color.RGBA{0, 100, 255, 128})
        outline.StrokeColor = color.RGBA{0, 150, 255, 255}
        outline.StrokeWidth = 2
        outline.Move(rect.Min)
        outline.Resize(rect.Max.Sub(rect.Min))
        objs = append(objs, outline)
    }
    
    // Draw attack hitbox (red)
    if cw.CurrentKeyFrame.AttackHitBox != nil {
        rect := cw.hitboxToCanvas(cw.CurrentKeyFrame.AttackHitBox)
        outline := canvas.NewRectangle(color.RGBA{255, 68, 68, 128})
        outline.StrokeColor = color.RGBA{255, 100, 100, 255}
        outline.StrokeWidth = 2
        outline.Move(rect.Min)
        outline.Resize(rect.Max.Sub(rect.Min))
        objs = append(objs, outline)
    }
    
    // Draw grid
    for x := 0; x < cw.CurrentKeyFrame.Sprite.W; x += 16 {
        line := canvas.NewLine(color.RGBA{128, 128, 128, 50})
        line.StrokeWidth = 0.5
        // ... position line
        objs = append(objs, line)
    }
}

func (cw *CanvasWidget) hitboxToCanvas(box *Box) fyne.Rectangle {
    return fyne.NewRect(
        float32(box.X), float32(box.Y),
        float32(box.W), float32(box.H),
    )
}

func (cw *CanvasWidget) MouseDown(me *fyne.MouseDownEvent) {
    if me.Button != fyne.MouseButtonLeft {
        return
    }
    
    // Check if clicking on a hitbox corner or center
    if cw.CurrentKeyFrame.HitBox != nil {
        if cw.isNearPoint(me.Position, cw.hitboxCorner(cw.CurrentKeyFrame.HitBox, Corner TL)) {
            cw.SelectedBox = BoxCollision
            cw.dragHandle = HandleCornerTL
            return
        }
        if cw.isNearPoint(me.Position, cw.hitboxCenter(cw.CurrentKeyFrame.HitBox)) {
            cw.SelectedBox = BoxCollision
            cw.dragHandle = HandleCenter
            return
        }
    }
    
    // Similar for attack hitbox...
}

func (cw *CanvasWidget) MouseMoved(me *fyne.MouseMovedEvent) {
    if cw.dragHandle == HandleNone {
        return
    }
    
    delta := me.Position.Subtract(cw.lastMousePos)
    box := cw.SelectedBox.GetBox(cw.CurrentKeyFrame)
    if box == nil {
        return
    }
    
    switch cw.dragHandle {
    case HandleCornerTL:
        box.X += int(delta.X)
        box.Y += int(delta.Y)
        box.W -= int(delta.X)
        box.H -= int(delta.Y)
    case HandleCornerBR:
        box.W += int(delta.X)
        box.H += int(delta.Y)
    case HandleCenter:
        box.X += int(delta.X)
        box.Y += int(delta.Y)
    }
    
    if cw.OnBoxChanged != nil {
        cw.OnBoxChanged(box)
    }
    cw.Refresh()
    cw.lastMousePos = me.Position
}
```

### Example 5: Saving Animation to TOML

```go
func (proj *Project) Save(path string) error {
    // Convert animation to TOML-friendly structure
    tomlData := map[string]interface{}{
        "metadata": map[string]string{
            "name":        proj.CurrentAnimation.Metadata.Name,
            "version":     proj.CurrentAnimation.Metadata.Version,
            "description": proj.CurrentAnimation.Metadata.Description,
        },
        "animation": map[string]interface{}{
            "loop":            proj.CurrentAnimation.Config.Loop,
            "default_speed":   proj.CurrentAnimation.Config.DefaultSpeed,
            "character_size":  proj.CurrentAnimation.Config.CharacterSize,
        },
        "keyframes": serializeKeyFrames(proj.CurrentAnimation.KeyFrames),
        "nested":    serializeNested(proj.CurrentAnimation.Nested),
    }
    
    // Serialize to TOML
    buf := &bytes.Buffer{}
    enc := toml.NewEncoder(buf)
    if err := enc.Encode(tomlData); err != nil {
        return fmt.Errorf("failed to encode TOML: %w", err)
    }
    
    // Write to file
    if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
        return fmt.Errorf("failed to write file: %w", err)
    }
    
    proj.SavePath = path
    proj.Dirty = false
    
    return nil
}

func serializeKeyFrames(kfs []*KeyFrame) []map[string]interface{} {
    result := make([]map[string]interface{}, len(kfs))
    
    for i, kf := range kfs {
        kfMap := map[string]interface{}{
            "id":              kf.ID,
            "duration_ms":     kf.Duration,
            "sprite":          kf.Sprite.SheetName + ":" + strconv.Itoa(kf.Sprite.Index),
            "speed_modifier":  kf.Speed,
        }
        
        if kf.HitBox != nil {
            kfMap["hitbox"] = map[string]int{
                "x": kf.HitBox.X, "y": kf.HitBox.Y,
                "w": kf.HitBox.W, "h": kf.HitBox.H,
            }
        }
        
        if kf.AttackHitBox != nil {
            kfMap["attack_hitbox"] = map[string]int{
                "x": kf.AttackHitBox.X, "y": kf.AttackHitBox.Y,
                "w": kf.AttackHitBox.W, "h": kf.AttackHitBox.H,
            }
        }
        
        // Serialize events...
        
        result[i] = kfMap
    }
    
    return result
}
```

---

## Build & Deployment

### Prerequisites

```
Go 1.21+
git clone https://github.com/yourusername/ANIFile-maker.git
cd ANIFile-maker
go mod download
```

### Build

```bash
# Linux/macOS
go build -o ANIFile-maker ./cmd/ANIFile-maker

# Windows
go build -o ANIFile-maker.exe ./cmd/ANIFile-maker

# Cross-compile for all platforms
GOOS=windows GOARCH=amd64 go build -o ANIFile-maker-windows.exe ./cmd/ANIFile-maker
GOOS=darwin GOARCH=amd64 go build -o ANIFile-maker-macos ./cmd/ANIFile-maker
GOOS=linux GOARCH=amd64 go build -o ANIFile-maker-linux ./cmd/ANIFile-maker
```

### Run

```bash
./ANIFile-maker
```

### Distribution

**Single Binary Delivery:**
- No installer needed
- One executable per platform
- Users download + run
- All dependencies bundled in binary (except system libs)

**Packaging** (optional future):
- Homebrew (macOS): `brew install ANIFile-maker`
- Chocolatey (Windows): `choco install ANIFile-maker`
- Snap (Linux): `snap install ANIFile-maker`

---

## Testing Strategy

### Unit Tests

```go
// pkg/editor/keyframe_test.go
func TestDuplicateKeyFrame(t *testing.T) {
    src := &KeyFrame{
        ID:       0,
        Duration: 100,
        Speed:    1.5,
        HitBox:   &Box{X: 5, Y: 10, W: 20, H: 20},
    }
    
    copy := duplicateKeyFrame(src)
    
    if copy.Duration != 100 {
        t.Errorf("duration mismatch: got %d, want 100", copy.Duration)
    }
    if copy.Speed != 1.5 {
        t.Errorf("speed mismatch: got %f, want 1.5", copy.Speed)
    }
    if copy.HitBox.X != 5 {
        t.Errorf("hitbox X mismatch: got %d, want 5", copy.HitBox.X)
    }
}

// pkg/file/toml_test.go
func TestSaveAndLoadAnimation(t *testing.T) {
    // Create test animation
    anim := &Animation{
        Metadata: AnimationMetadata{Name: "test"},
        KeyFrames: []*KeyFrame{
            {ID: 0, Duration: 100},
        },
    }
    
    // Save to temp file
    tmpFile, _ := ioutil.TempFile("", "*.anif")
    defer os.Remove(tmpFile.Name())
    
    if err := saveAnimationToTOML(anim, tmpFile.Name()); err != nil {
        t.Fatalf("save failed: %v", err)
    }
    
    // Load from file
    loaded, err := loadAnimationFromTOML(tmpFile.Name())
    if err != nil {
        t.Fatalf("load failed: %v", err)
    }
    
    if loaded.Metadata.Name != "test" {
        t.Errorf("name mismatch: got %s, want test", loaded.Metadata.Name)
    }
}
```

### Integration Tests

```go
// pkg/app/workflow_test.go
func TestWorkflow_CreateWalkCycle(t *testing.T) {
    app := &Application{
        Project: &Project{
            CurrentAnimation: &Animation{
                KeyFrames: []*KeyFrame{},
            },
            LoadedSheets: make(map[string]*SpriteSheet),
        },
    }
    
    // Import sheet
    sheet := importTestSheet("testdata/walk.png", GridConfig{Cols: 4, Rows: 1, TileW: 32, TileH: 32})
    app.Project.LoadedSheets["walk_sheet"] = sheet
    
    // Add 4 frames
    for i := 0; i < 4; i++ {
        kf := &KeyFrame{
            ID:       i,
            Duration: 100,
            Sprite:   SpriteReference{SheetName: "walk_sheet", Index: i},
        }
        app.Project.CurrentAnimation.KeyFrames = append(app.Project.CurrentAnimation.KeyFrames, kf)
    }
    
    // Verify
    if len(app.Project.CurrentAnimation.KeyFrames) != 4 {
        t.Errorf("expected 4 frames, got %d", len(app.Project.CurrentAnimation.KeyFrames))
    }
    
    // Calculate total duration
    totalDuration := calculateAnimationDuration(app.Project.CurrentAnimation)
    if totalDuration != 400 {
        t.Errorf("expected 400ms, got %d", totalDuration)
    }
}
```

### UI Tests

**Manual testing checklist:**
- [ ] Import sprite sheet (grid preview correct)
- [ ] Add keyframe (sprite picker works)
- [ ] Draw hitbox (visual feedback correct)
- [ ] Duplicate frame (all properties copy)
- [ ] Play animation (smooth, looping works)
- [ ] Save/load (data persists)
- [ ] Undo/redo (state restores)

---

## Configuration & Preferences

### User Preferences (config.toml)

```toml
[ui]
dark_mode = true
zoom_default = 100
grid_size = 16
show_grid = true
theme = "dark"

[editor]
undo_history_size = 100
auto_save_interval = 60  # seconds

[assets]
default_asset_path = "assets/"
recent_projects = []
```

---

## Future Enhancements

1. **Bone/Skeleton Animation** - Add skeletal animation support
2. **Timeline Compression** - Zoom timeline to see more frames
3. **Frame Interpolation** - Auto-create frames between keyframes
4. **Palette Animation** - Animate colors
5. **Layer Support** - Multiple layers per frame
6. **Animation Blending** - Blend between animations
7. **Export Formats** - Export to sprite sheet, video, other formats
8. **Plugin System** - Extensibility via plugins
9. **Collaborative Editing** - Multi-user animation editing
10. **AI-Assisted** - Auto-generate frames from description

---

## Troubleshooting

### Common Issues

**Issue**: Sprite doesn't display
- Check: File path correct?
- Check: Sprite index within range?
- Check: Sprite sheet loaded?

**Issue**: Hitbox doesn't show
- Check: Click "Add collision box" first?
- Check: Box might be outside sprite bounds?

**Issue**: Animation doesn't save
- Check: File path writable?
- Check: Disk space available?

---

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| Ctrl+N | New animation |
| Ctrl+O | Open animation |
| Ctrl+S | Save animation |
| Ctrl+Z | Undo |
| Ctrl+Shift+Z | Redo |
| Space | Play/pause |
| → | Next frame |
| ← | Previous frame |
| D | Duplicate frame |
| X | Delete frame |
| + | Increase duration |
| - | Decrease duration |
| 1 | Zoom 100% |
| 2 | Zoom 200% |
| G | Toggle grid |
| H | Toggle hitbox |

---

## Glossary

- **Animation**: Sequence of keyframes that play in order
- **Keyframe**: Single frame with sprite, duration, hitboxes, events
- **Sprite**: Image region from sprite sheet
- **Sprite Sheet**: Grid of sprites (texture atlas)
- **Hitbox**: Collision box for physics/game logic
- **Attack Box**: Damage hitbox for combat validation
- **Event**: Sound, particle, shake, flash trigger
- **Nested Animation**: Animation played alongside main animation
- **Duplication**: Copy keyframe with all properties for fast iteration

---

## References

- **Fyne Documentation**: https://pkg.go.dev/github.com/fyne-io/fyne/v2
- **Go Standard Library**: https://pkg.go.dev/std
- **TOML Specification**: https://toml.io/
- **PNG/JPEG Specs**: https://golang.org/pkg/image/

---

**Document Version**: 1.0  
**Last Updated**: 2024  
**Status**: Ready for Implementation

