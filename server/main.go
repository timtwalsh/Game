package main

import (
	"encoding/json"
	"fmt"
	"game/shared"
	"net"
	"sync"
	"time"
)

type Server struct {
	conn       *net.UDPConn
	clients    map[string]uint64
	players    map[uint64]*shared.PlayerState
	suspicions map[uint64]*SuspicionTracker
	mutex      sync.Mutex
	validator  MovementValidator
	nextID     uint64
}

func NewServer() *Server {
	return &Server{
		clients:    make(map[string]uint64),
		players:    make(map[uint64]*shared.PlayerState),
		suspicions: make(map[uint64]*SuspicionTracker),
		validator:  NewMovementValidator(shared.NewCollisionLayer(100, 100)),
		nextID:     1,
	}
}

func (s *Server) ListenAndServe(addrStr string) error {
	addr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return err
	}
	s.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	fmt.Println("Server listening on", addrStr)

	go s.tickLoop()

	buf := make([]byte, 2048)
	for {
		n, clientAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Error reading UDP:", err)
			continue
		}
		
		s.handlePacket(buf[:n], clientAddr)
	}
}

func (s *Server) handlePacket(data []byte, addr *net.UDPAddr) {
	addrStr := addr.String()
	
	var wrapper shared.MessageWrapper
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return
	}

	if wrapper.Type == "Move" {
		var moveMsg shared.ClientMoveMsg
		if err := json.Unmarshal(wrapper.Payload, &moveMsg); err != nil {
			return
		}
		
		s.mutex.Lock()
		
		// MVP Shortcut: Trust client's random PlayerID
		playerID := moveMsg.PlayerID
		s.clients[addrStr] = playerID
		
		player, exists := s.players[playerID]
		if !exists {
			player = &shared.PlayerState{
				PlayerID:  playerID,
				Position:  shared.Vec2{X: 0, Y: 0},
				Animation: 0,
				Direction: moveMsg.Direction,
				ColorR:    moveMsg.ColorR,
				ColorG:    moveMsg.ColorG,
				ColorB:    moveMsg.ColorB,
			}
			s.players[playerID] = player
			s.suspicions[playerID] = NewSuspicionTracker(playerID)
			fmt.Printf("New player connected: %d from %s\n", playerID, addrStr)
		}
		
		tracker := s.suspicions[playerID]
		
		// Validate
		issues := s.validator.ValidateMovement(player.Position, moveMsg.Position, moveMsg.TimeMs, MovementTypeWalk)
		for _, issue := range issues {
			if issue.TooFast {
				tracker.AddEvent(shared.SuspicionEvent{Type: shared.SuspicionEventTooFast, Speed: issue.Speed})
			}
			if issue.WallPhase {
				tracker.AddEvent(shared.SuspicionEvent{Type: shared.SuspicionEventWallPhase, TileX: issue.TileX, TileY: issue.TileY})
			}
		}
		
		if tracker.GetStatus() != SuspicionStatusAutoBan {
			player.Position = moveMsg.Position
			player.Direction = moveMsg.Direction
			player.ColorR = moveMsg.ColorR
			player.ColorG = moveMsg.ColorG
			player.ColorB = moveMsg.ColorB
		}
		s.mutex.Unlock()
	}
}

func (s *Server) tickLoop() {
	ticker := time.NewTicker(time.Duration(shared.NetworkTickRate) * time.Millisecond)
	for range ticker.C {
		s.mutex.Lock()
		var states []shared.PlayerState
		for _, p := range s.players {
			states = append(states, *p)
		}
		
		msg := shared.ServerPlayerStatesMsg{States: states}
		payload, _ := json.Marshal(msg)
		wrapper := shared.MessageWrapper{Type: "PlayerStates", Payload: payload}
		out, _ := json.Marshal(wrapper)
		
		for addrStr := range s.clients {
			addr, _ := net.ResolveUDPAddr("udp", addrStr)
			s.conn.WriteToUDP(out, addr)
		}
		s.mutex.Unlock()
	}
}

func main() {
	server := NewServer()
	if err := server.ListenAndServe("0.0.0.0:8080"); err != nil {
		panic(err)
	}
}
