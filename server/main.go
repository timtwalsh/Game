package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"game/shared"
	"game/shared/world"
	"net"
	"os"
	"sync"
	"time"
)

type Server struct {
	conn       *net.UDPConn
	clients    map[string]uint64
	players    map[uint64]*shared.PlayerState
	suspicions map[uint64]*SuspicionTracker
	lastSeen   map[uint64]time.Time // when each player's last Move arrived
	board      *StatusBoard
	mutex      sync.Mutex
	validator  MovementValidator
	// spawn is where new players start: the world's spawn object, read
	// from the server's own level files, or shared.SpawnPoint without any.
	spawn  shared.Vec2
	nextID uint64
}

// NewServer makes a server validating movement against m, which the server
// loads from its own files (D32), never from a client.
func NewServer(m *world.Map) *Server {
	return &Server{
		clients:    make(map[string]uint64),
		players:    make(map[uint64]*shared.PlayerState),
		suspicions: make(map[uint64]*SuspicionTracker),
		lastSeen:   make(map[uint64]time.Time),
		board:      NewStatusBoard(),
		validator:  NewMovementValidator(m.Collider),
		spawn:      m.Spawn,
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
	s.board.SetHeader("Server listening on " + addrStr)
	go s.board.Run(os.Stdout, nil)

	go s.tickLoop()

	buf := make([]byte, 2048)
	for {
		n, clientAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			s.board.Event("Error reading UDP: %v", err)
			continue
		}

		s.handlePacket(buf[:n], clientAddr)
	}
}

// playerTimeout is how long a player can go without sending a Move before
// the server drops them. Clients send every NetworkTickRate (50ms), so this
// is ~100 missed sends: a closed client, not a lost packet. There's no
// explicit "leave" message, so this is how everyone else finds out.
const playerTimeout = 5 * time.Second

// dropIdle forgets every player not heard from in playerTimeout. They
// stop appearing in the broadcast, which is how clients learn they left.
// Call with s.mutex held.
func (s *Server) dropIdle(now time.Time) {
	for id, seen := range s.lastSeen {
		if now.Sub(seen) <= playerTimeout {
			continue
		}
		delete(s.players, id)
		delete(s.suspicions, id)
		delete(s.lastSeen, id)
		for addr, cid := range s.clients {
			if cid == id {
				delete(s.clients, addr)
			}
		}
		s.board.Disconnected(id, "timed out")
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
				Position:  s.spawn,
				Animation: moveMsg.Animation,
				Direction: moveMsg.Direction,
				ColorR:    moveMsg.ColorR,
				ColorG:    moveMsg.ColorG,
				ColorB:    moveMsg.ColorB,
			}
			s.players[playerID] = player
			s.suspicions[playerID] = NewSuspicionTracker(playerID)
			s.board.Connected(playerID, addrStr)
		}
		action := describeMove(*player, moveMsg)

		s.lastSeen[playerID] = time.Now()
		tracker := s.suspicions[playerID]

		// Validate
		issues := s.validator.ValidateMovement(player.Position, moveMsg.Position, moveMsg.TimeMs, MovementTypeWalk)
		for _, issue := range issues {
			if issue.TooFast {
				tracker.AddEvent(shared.SuspicionEvent{Type: shared.SuspicionEventTooFast, Speed: issue.Speed})
				action = fmt.Sprintf("flagged: too fast (%.0f px/s)", issue.Speed)
			}
			if issue.WallPhase {
				tracker.AddEvent(shared.SuspicionEvent{Type: shared.SuspicionEventWallPhase, TileX: issue.TileX, TileY: issue.TileY})
				action = fmt.Sprintf("flagged: wall phase at tile (%d, %d)", issue.TileX, issue.TileY)
			}
		}
		if tracker.GetStatus() == SuspicionStatusAutoBan {
			action = "auto-banned (moves ignored)"
		}
		if action != "" {
			s.board.Action(playerID, action)
		}

		if tracker.GetStatus() != SuspicionStatusAutoBan {
			player.Position = moveMsg.Position
			player.Direction = moveMsg.Direction
			player.Animation = moveMsg.Animation
			player.AnimSeq = moveMsg.AnimSeq
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
		s.dropIdle(time.Now())
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
	root := flag.String("root", ".", "folder holding world/ and levels/; without them the server runs on a blank grid")
	flag.Parse()
	m, err := world.LoadMap(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Can't load the world:", err)
		os.Exit(1)
	}
	server := NewServer(m)
	if m.World != nil {
		server.board.Event("Loaded %d levels from %s; spawn at (%.0f, %.0f)", len(m.World.Levels), *root, m.Spawn.X, m.Spawn.Y)
	} else {
		server.board.Event("No world/ and levels/ under %s: running on a blank %dx%d grid", *root, world.FallbackSize, world.FallbackSize)
	}
	if err := server.ListenAndServe("0.0.0.0:8080"); err != nil {
		panic(err)
	}
}
