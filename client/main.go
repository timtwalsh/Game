package main

import (
	"encoding/json"
	"fmt"
	"game/shared"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type Client struct {
	conn            *net.UDPConn
	serverAddr      *net.UDPAddr
	playerID        uint64
	colorR          uint8
	colorG          uint8
	colorB          uint8
	controller      *PlayerController
	remotePlayers   map[uint64]*PlayerInterpolation
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
			for _, state := range msg.States {
				if state.PlayerID == c.playerID {
					c.controller.ServerCorrection(state.Position)
				} else {
					if interp, exists := c.remotePlayers[state.PlayerID]; exists {
						interp.ServerUpdate(state)
					} else {
						rp := NewPlayerInterpolation(state.Position)
						rp.ServerUpdate(state)
						c.remotePlayers[state.PlayerID] = rp
					}
				}
			}
			c.mutex.Unlock()
		}
	}
}

func (c *Client) SendMove(pos shared.Vec2, timeMs uint32) {
	msg := shared.ClientMoveMsg{
		PlayerID:  c.playerID,
		Position:  pos,
		TimeMs:    timeMs,
		Direction: c.controller.Direction,
		ColorR:    c.colorR,
		ColorG:    c.colorG,
		ColorB:    c.colorB,
	}
	payload, _ := json.Marshal(msg)
	wrapper := shared.MessageWrapper{Type: "Move", Payload: payload}
	out, _ := json.Marshal(wrapper)
	c.conn.WriteToUDP(out, c.serverAddr)
}

func main() {
	rl.InitWindow(800, 600, "2D MMO Client")
	defer rl.CloseWindow()
	rl.SetTargetFPS(144)

	client := NewClient("127.0.0.1:8080")
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
		}

		client.mutex.Lock()
		client.controller.UpdatePrediction(&input, deltaMs)
		
		for _, rp := range client.remotePlayers {
			rp.Update(deltaMs)
		}

		// Network Send (10Hz)
		if now.Sub(client.lastNetworkSend).Milliseconds() >= 100 {
			client.SendMove(client.controller.PredictedPosition, 100)
			client.lastNetworkSend = now
		}
		
		// Rendering
		rl.BeginDrawing()
		rl.ClearBackground(rl.RayWhite)

		// Draw remote players
		for _, rp := range client.remotePlayers {
			color := rl.NewColor(rp.ColorR, rp.ColorG, rp.ColorB, 255)
			centerX := int32(rp.CurrentPosition.X + 8)
			centerY := int32(rp.CurrentPosition.Y + 8)
			rl.DrawCircle(centerX, centerY, 8.0, color)
			
			// Draw direction indicator line
			angle := getAngleForDirection(rp.Direction)
			endX := int32(float64(centerX) + math.Cos(angle)*8.0)
			endY := int32(float64(centerY) + math.Sin(angle)*8.0)
			rl.DrawLineEx(rl.NewVector2(float32(centerX), float32(centerY)), rl.NewVector2(float32(endX), float32(endY)), 2.0, rl.Black)
		}
		
		// Draw local player
		localColor := rl.NewColor(client.colorR, client.colorG, client.colorB, 255)
		localCenterX := int32(client.controller.PredictedPosition.X + 8)
		localCenterY := int32(client.controller.PredictedPosition.Y + 8)
		rl.DrawCircle(localCenterX, localCenterY, 8.0, localColor)
		
		// Draw direction indicator line
		localAngle := getAngleForDirection(client.controller.Direction)
		localEndX := int32(float64(localCenterX) + math.Cos(localAngle)*8.0)
		localEndY := int32(float64(localCenterY) + math.Sin(localAngle)*8.0)
		rl.DrawLineEx(rl.NewVector2(float32(localCenterX), float32(localCenterY)), rl.NewVector2(float32(localEndX), float32(localEndY)), 2.0, rl.Black)
		
		rl.DrawText("Use WASD to move", 10, 10, 20, rl.DarkGray)
		
		rl.EndDrawing()
		client.mutex.Unlock()
	}
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
