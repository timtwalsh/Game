package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"game/client/anim"
	"game/client/render"
	"game/shared"
	"game/shared/world"
	"math"
	"math/rand"
	"net"
	"os"
	"slices"
	"sync"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type Client struct {
	conn          *net.UDPConn
	serverAddr    *net.UDPAddr
	playerID      uint64
	colorR        uint8
	colorG        uint8
	colorB        uint8
	controller    *PlayerController
	remotePlayers map[uint64]*PlayerInterpolation
	// assets holds every loaded track and sheet, shared by all players.
	// look is what every player is drawn as; nil if it failed to load,
	// and players are drawn as circles instead.
	assets    *anim.Library
	look      *characterLook
	localAnim *CharacterAnimator
	// remoteAnims is only touched by the main goroutine (it's drawn
	// outside the lock), unlike remotePlayers.
	remoteAnims map[uint64]*CharacterAnimator
	mutex       sync.Mutex
}

// NewClient connects to the server and starts the local player at the
// map's spawn, predicting against the map's collider.
func NewClient(serverAddrStr string, m *world.Map) *Client {
	serverAddr, err := net.ResolveUDPAddr("udp", serverAddrStr)
	if err != nil {
		panic(err)
	}

	addr, _ := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}

	c := &Client{
		conn:          conn,
		serverAddr:    serverAddr,
		playerID:      uint64(time.Now().UnixNano()), // Random ID for MVP
		colorR:        uint8(rand.Intn(256)),
		colorG:        uint8(rand.Intn(256)),
		colorB:        uint8(rand.Intn(256)),
		controller:    NewPlayerController(m.Spawn, m.Collider),
		remotePlayers: make(map[uint64]*PlayerInterpolation),
		remoteAnims:   make(map[uint64]*CharacterAnimator),
		assets:        anim.NewLibrary(),
	}
	look, err := loadLook(c.assets, playerLook)
	for _, p := range c.assets.Problems {
		fmt.Println("Asset problem:", p)
	}
	if err != nil {
		fmt.Println("Player character didn't load, drawing circles instead:", err)
	} else {
		c.look = look
		c.localAnim = NewCharacterAnimator(look)
	}

	go c.receiveLoop()
	return c
}

func (c *Client) receiveLoop() {
	buf := make([]byte, 2048)
	for {
		n, _, err := c.conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Error reading UDP:", err)
			continue
		}

		var wrapper shared.MessageWrapper
		if err := json.Unmarshal(buf[:n], &wrapper); err != nil {
			continue
		}

		if wrapper.Type == "PlayerStates" {
			var msg shared.ServerPlayerStatesMsg
			if err := json.Unmarshal(wrapper.Payload, &msg); err != nil {
				continue
			}

			c.mutex.Lock()
			c.applyPlayerStates(msg.States)
			c.mutex.Unlock()
		}
	}
}

// applyPlayerStates takes one server broadcast. The broadcast lists every
// player the server still has, so a remote player missing from it has
// left (or timed out server-side) and is dropped. Call with c.mutex held.
func (c *Client) applyPlayerStates(states []shared.PlayerState) {
	for _, state := range states {
		if state.PlayerID == c.playerID {
			c.controller.ServerCorrection(state.Position)
			continue
		}
		if interp, exists := c.remotePlayers[state.PlayerID]; exists {
			interp.ServerUpdate(state)
		} else {
			rp := NewPlayerInterpolation(state.Position)
			rp.ServerUpdate(state)
			c.remotePlayers[state.PlayerID] = rp
		}
	}
	for id := range c.remotePlayers {
		if !slices.ContainsFunc(states, func(s shared.PlayerState) bool { return s.PlayerID == id }) {
			delete(c.remotePlayers, id)
		}
	}
}

func (c *Client) SendMove(pos shared.Vec2, timeMs uint32) {
	msg := shared.ClientMoveMsg{
		PlayerID:  c.playerID,
		Position:  pos,
		TimeMs:    timeMs,
		Direction: c.controller.Direction,
		Animation: c.localAnimState(),
		AnimSeq:   c.localAnimSeq(),
		ColorR:    c.colorR,
		ColorG:    c.colorG,
		ColorB:    c.colorB,
	}
	payload, _ := json.Marshal(msg)
	wrapper := shared.MessageWrapper{Type: "Move", Payload: payload}
	out, _ := json.Marshal(wrapper)
	c.conn.WriteToUDP(out, c.serverAddr)
}

// Simulation runs on a fixed timestep, decoupled from the render frame rate.
// Each frame adds its real elapsed time to an accumulator and runs as many
// whole simSteps as fit; the leftover fraction is used only to blend the
// drawn position between the last two steps.
const (
	simStep = time.Second / time.Duration(shared.SimTickHz)
	// maxFrameTime caps how much real time one frame may feed the simulation,
	// so a stall (window drag, breakpoint) doesn't trigger a huge catch-up
	// burst. Time beyond it is dropped rather than simulated.
	maxFrameTime = 250 * time.Millisecond
	// ticksPerNetworkSend sends a ClientMoveMsg every N sim steps, i.e. at
	// shared.NetworkTickRate measured in simulated time (3 steps at 60Hz).
	ticksPerNetworkSend = max(1, (shared.NetworkTickRate*shared.SimTickHz+500)/1000)
)

func (c *Client) localAnimState() uint8 {
	if c.localAnim == nil {
		return shared.AnimIdle
	}
	return c.localAnim.State()
}

func (c *Client) localAnimSeq() uint8 {
	if c.localAnim == nil {
		return 0
	}
	return c.localAnim.Seq()
}

// remoteAnim is a remote player's animator, made the first time it's drawn.
func (c *Client) remoteAnim(id uint64) *CharacterAnimator {
	ca := c.remoteAnims[id]
	if ca == nil {
		ca = NewCharacterAnimator(c.look)
		c.remoteAnims[id] = ca
	}
	return ca
}

// drawnPlayer is one player to draw this frame, snapshotted under the lock
// so drawing doesn't hold it. Players are sorted together with the ysort
// layer's tiles by foot position, so whatever is nearer the bottom of the
// screen is in front.
type drawnPlayer struct {
	pos       shared.Vec2
	direction uint8
	color     rl.Color
	anim      *CharacterAnimator // nil: draw a circle
}

// playerFootY is how far below a player's position their feet are: the
// bottom of their 16x16 box, the y they're sorted by.
const playerFootY = 16

func main() {
	root := flag.String("root", ".", "folder holding world/ and levels/ (the same files the server loads); without them the client runs on a blank grid")
	flag.Parse()
	// The client must predict against the same map the server validates
	// against, so a world that's present but broken is fatal rather than
	// silently replaced by the blank grid.
	m, err := world.LoadMap(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Can't load the world:", err)
		os.Exit(1)
	}

	rl.InitWindow(800, 600, "2D MMO Client")
	defer rl.CloseWindow()
	rl.SetTargetFPS(144)

	var levels *render.Renderer
	if m.World != nil {
		atlas, problems := render.NewAtlas(m.World.Defs, *root)
		for _, p := range problems {
			fmt.Println("Tile problem:", p)
		}
		defer atlas.Unload()
		levels = &render.Renderer{World: m.World, Atlas: atlas}
	}
	camera := render.NewFollowCamera()

	client := NewClient("127.0.0.1:8080", m)
	textures := textureCache{}
	defer textures.unload()
	// Reused every frame, so drawing allocates nothing once warmed up.
	var players []drawnPlayer
	var items []render.Item
	var sprites []anim.Sprite
	lastFrameTime := time.Now()
	var accumulator time.Duration
	var ticksSinceSend uint32
	// animCarry keeps the sub-millisecond remainder the animators whole-ms
	// deltas can't express, so playback speed matches real time.
	var animCarry time.Duration

	for !rl.WindowShouldClose() {
		now := time.Now()
		frameTime := now.Sub(lastFrameTime)
		lastFrameTime = now
		if frameTime > maxFrameTime {
			frameTime = maxFrameTime
		}
		accumulator += frameTime

		// Input
		input := PlayerInput{
			Up:    rl.IsKeyDown(rl.KeyUp) || rl.IsKeyDown(rl.KeyW),
			Down:  rl.IsKeyDown(rl.KeyDown) || rl.IsKeyDown(rl.KeyS),
			Left:  rl.IsKeyDown(rl.KeyLeft) || rl.IsKeyDown(rl.KeyA),
			Right: rl.IsKeyDown(rl.KeyRight) || rl.IsKeyDown(rl.KeyD),
			Jump:  rl.IsKeyPressed(rl.KeySpace),
		}

		client.mutex.Lock()
		for accumulator >= simStep {
			client.controller.UpdatePrediction(&input, simStep)
			accumulator -= simStep

			// Network Send - TimeMs is the simulated time the reported
			// movement covers, so the server's speed check divides distance
			// by exactly the time it was simulated over.
			ticksSinceSend++
			if ticksSinceSend >= ticksPerNetworkSend {
				simulated := time.Duration(ticksSinceSend) * simStep
				client.SendMove(client.controller.PredictedPosition, uint32(simulated.Round(time.Millisecond).Milliseconds()))
				ticksSinceSend = 0
			}
		}
		alpha := float32(accumulator) / float32(simStep)
		localPos := client.controller.RenderPosition(alpha)

		for _, rp := range client.remotePlayers {
			rp.Update(frameTime)
		}

		// Animation - presentation only, so it runs per frame on real time
		// (not in sim steps), with the sub-ms remainder carried over.
		animCarry += frameTime
		animMs := uint32(animCarry / time.Millisecond)
		animCarry -= time.Duration(animMs) * time.Millisecond
		if la := client.localAnim; la != nil {
			moving := client.controller.Velocity.X != 0 || client.controller.Velocity.Y != 0
			la.Step(moving, input.Jump, client.controller.Direction, animMs)
			for id, rp := range client.remotePlayers {
				client.remoteAnim(id).Apply(rp.Animation, rp.AnimSeq, rp.Direction, animMs)
			}
			for id := range client.remoteAnims {
				if client.remotePlayers[id] == nil {
					delete(client.remoteAnims, id) // the player left
				}
			}
		}

		players = append(players[:0], drawnPlayer{
			pos:       localPos,
			direction: client.controller.Direction,
			color:     rl.NewColor(client.colorR, client.colorG, client.colorB, 255),
			anim:      client.localAnim,
		})
		for id, rp := range client.remotePlayers {
			players = append(players, drawnPlayer{
				pos:       rp.CurrentPosition,
				direction: rp.Direction,
				color:     rl.NewColor(rp.ColorR, rp.ColorG, rp.ColorB, 255),
				anim:      client.remoteAnims[id],
			})
		}
		client.mutex.Unlock()

		// Camera: follows the local player's centre; the mouse wheel zooms
		// within the player's limits.
		camera.ZoomBy(rl.GetMouseWheelMove())
		camera.CentreOn(shared.Vec2{X: localPos.X + 8, Y: localPos.Y + 8})
		screenW, screenH := rl.GetScreenWidth(), rl.GetScreenHeight()
		view := camera.Visible(screenW, screenH)

		// Rendering, outside the lock: the snapshot above and the
		// animators are only touched by this goroutine. Outside every
		// level is void, which draws as the black background.
		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		rl.BeginMode2D(camera.Raylib(screenW, screenH))
		items = items[:0]
		if levels != nil {
			levels.DrawUnder(view)
			items = levels.AppendYSort(items, view)
		} else {
			// No level files: the blank fallback grid is open floor.
			size := int32(world.FallbackSize * shared.TileSize)
			rl.DrawRectangle(0, 0, size, size, rl.RayWhite)
		}
		for i, p := range players {
			items = append(items, render.Item{Key: p.pos.Y + playerFootY, Player: i})
		}
		render.SortItems(items)
		for _, it := range items {
			if it.Player < 0 {
				levels.DrawItem(it)
				continue
			}
			drawPlayer(players[it.Player], textures, &sprites)
		}
		if levels != nil {
			levels.DrawOverhead(view)
		}
		rl.EndMode2D()

		rl.DrawText("WASD to move, Space to jump, wheel to zoom", 10, 10, 20, rl.LightGray)

		rl.EndDrawing()
	}
}

// drawPlayer draws one player in world coordinates. sprites is scratch
// space reused across calls.
func drawPlayer(p drawnPlayer, textures textureCache, sprites *[]anim.Sprite) {
	// The player's 16x16 box is at pos; the animation's origin and the
	// fallback circle both sit at its centre.
	centerX, centerY := p.pos.X+8, p.pos.Y+8
	if p.anim == nil {
		drawCircle(int32(centerX), int32(centerY), p.color, p.direction)
		return
	}
	*sprites = p.anim.AppendSprites((*sprites)[:0])
	// A shadow in the player's colour at their feet: it tells identical
	// characters apart, and stays on the ground while they jump.
	look := p.anim.look
	originX := centerX + look.origin.X*look.scale
	originY := centerY + look.origin.Y*look.scale
	footY := originY + groundOffset(*sprites, look.scale)
	rl.DrawEllipse(int32(centerX), int32(footY), 9, 3, rl.Fade(p.color, 0.6))
	textures.drawCharacter(*sprites, originX, originY, look.scale)
}

// drawCircle is how a player is drawn without a character: a circle with
// a line showing its facing.
func drawCircle(centerX, centerY int32, color rl.Color, direction uint8) {
	rl.DrawCircle(centerX, centerY, 8.0, color)
	angle := getAngleForDirection(direction)
	endX := int32(float64(centerX) + math.Cos(angle)*8.0)
	endY := int32(float64(centerY) + math.Sin(angle)*8.0)
	rl.DrawLineEx(rl.NewVector2(float32(centerX), float32(centerY)), rl.NewVector2(float32(endX), float32(endY)), 2.0, rl.Black)
}

// Convert 0=N, 1=NE, etc. to radians (-pi to pi where 0 is East) for drawing
func getAngleForDirection(dir uint8) float64 {
	// N=6, NE=7, E=0, SE=1, S=2, SW=3, W=4, NW=5 in our math octants
	mapping := []float64{
		-math.Pi / 2,     // 0=N
		-math.Pi / 4,     // 1=NE
		0.0,              // 2=E
		math.Pi / 4,      // 3=SE
		math.Pi / 2,      // 4=S
		3 * math.Pi / 4,  // 5=SW
		math.Pi,          // 6=W
		-3 * math.Pi / 4, // 7=NW
	}
	if int(dir) < len(mapping) {
		return mapping[dir]
	}
	return math.Pi / 2 // Default South
}
