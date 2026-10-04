package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"game/client/anim"
	"game/shared"
	"math"
	"math/rand"
	"net"
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
	remoteAnims     map[uint64]*CharacterAnimator
	mutex           sync.Mutex
	lastNetworkSend time.Time
}

func NewClient(serverAddrStr string) *Client {
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
		controller:    NewPlayerController(shared.Vec2{X: 100, Y: 100}, shared.NewCollisionLayer(100, 100)),
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
// so drawing doesn't hold it; players are drawn in order of Y so the one
// nearer the bottom of the screen is in front.
type drawnPlayer struct {
	pos       shared.Vec2
	direction uint8
	color     rl.Color
	anim      *CharacterAnimator // nil: draw a circle
}

func main() {
	rl.InitWindow(800, 600, "2D MMO Client")
	defer rl.CloseWindow()
	rl.SetTargetFPS(144)

	client := NewClient("127.0.0.1:8080")
	textures := textureCache{}
	defer textures.unload()
	// Reused every frame, so drawing allocates nothing once warmed up.
	var players []drawnPlayer
	var sprites []anim.Sprite
	var lastFrameTime time.Time = time.Now()

	for !rl.WindowShouldClose() {
		now := time.Now()
		deltaMs := uint32(now.Sub(lastFrameTime).Milliseconds())
		lastFrameTime = now

		// Input
		input := PlayerInput{
			Up:    rl.IsKeyDown(rl.KeyUp) || rl.IsKeyDown(rl.KeyW),
			Down:  rl.IsKeyDown(rl.KeyDown) || rl.IsKeyDown(rl.KeyS),
			Left:  rl.IsKeyDown(rl.KeyLeft) || rl.IsKeyDown(rl.KeyA),
			Right: rl.IsKeyDown(rl.KeyRight) || rl.IsKeyDown(rl.KeyD),
			Jump:  rl.IsKeyPressed(rl.KeySpace),
		}

		client.mutex.Lock()
		client.controller.UpdatePrediction(&input, deltaMs)

		for _, rp := range client.remotePlayers {
			rp.Update(deltaMs)
		}

		// Animation
		if la := client.localAnim; la != nil {
			moving := client.controller.Velocity.X != 0 || client.controller.Velocity.Y != 0
			la.Step(moving, input.Jump, client.controller.Direction, deltaMs)
			for id, rp := range client.remotePlayers {
				client.remoteAnim(id).Apply(rp.Animation, rp.AnimSeq, rp.Direction, deltaMs)
			}
			for id := range client.remoteAnims {
				if client.remotePlayers[id] == nil {
					delete(client.remoteAnims, id) // the player left
				}
			}
		}

		// Network Send
		sinceLastSend := now.Sub(client.lastNetworkSend).Milliseconds()
		if sinceLastSend >= int64(shared.NetworkTickRate) {
			client.SendMove(client.controller.PredictedPosition, uint32(sinceLastSend))
			client.lastNetworkSend = now
		}

		players = append(players[:0], drawnPlayer{
			pos:       client.controller.PredictedPosition,
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

		// Rendering, outside the lock: the snapshot above and the
		// animators are only touched by this goroutine.
		rl.BeginDrawing()
		rl.ClearBackground(rl.RayWhite)
		slices.SortStableFunc(players, func(a, b drawnPlayer) int { return cmp.Compare(a.pos.Y, b.pos.Y) })
		for _, p := range players {
			// The player's 16x16 box is at pos; the animation's origin
			// and the fallback circle both sit at its centre.
			centerX, centerY := p.pos.X+8, p.pos.Y+8
			if p.anim != nil {
				sprites = p.anim.AppendSprites(sprites[:0])
				// A shadow in the player's colour at their feet: it tells
				// identical characters apart, and stays on the ground
				// while they jump.
				look := p.anim.look
				originX := centerX + look.origin.X*look.scale
				originY := centerY + look.origin.Y*look.scale
				footY := originY + groundOffset(sprites, look.scale)
				rl.DrawEllipse(int32(centerX), int32(footY), 9, 3, rl.Fade(p.color, 0.6))
				textures.drawCharacter(sprites, originX, originY, look.scale)
				continue
			}
			drawCircle(int32(centerX), int32(centerY), p.color, p.direction)
		}

		rl.DrawText("WASD to move, Space to jump", 10, 10, 20, rl.DarkGray)

		rl.EndDrawing()
	}
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
